<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import type {Clerk} from '@clerk/clerk-js'
import {useData} from '@/stores/data'
import {t} from '@/lib/i18n'
import {ApiError} from '@/api/client'
import {desktop} from '@/lib/desktop'
import {type AuthInfo, getAuthInfo, loadClerk, SSO_CALLBACK_PATH} from '@/lib/clerk'

type Mode = 'signin' | 'signup' | 'reset'
// form: email + password. code: a one-time code from email or SMS.
// resetCode: code plus new password. totp: this app's own two-factor code.
type Step = 'form' | 'code' | 'resetCode' | 'totp'
type SecondFactor = 'totp' | 'email_code' | 'phone_code' | 'backup_code'

const data = useData()
const route = useRoute()
const router = useRouter()

const mode = ref<Mode>('signin')
const step = ref<Step>('form')
const username = ref('')
const email = ref('')
const password = ref('')
const code = ref('')
const totp = ref('')
const error = ref('')
const notice = ref('')
const busy = ref(false)
const booting = ref(true)
const showMore = ref(false)

const auth = ref<AuthInfo | null>(null)
let clerk: Clerk | null = null
// What the code step verifies: a sign up email, or a sign in second factor.
const codeFor = ref<'signup' | SecondFactor>('signup')
const codeTarget = ref('')

// The desktop app proxies /api to whichever server the user picks here.
const serverUrl = ref('')

const useClerk = computed(() => !!auth.value?.clerk?.enabled && !!auth.value.clerk.publishable_key)
// Clerk production instances only accept their own domain, which the
// desktop app's local origin is not. Development instances allow localhost.
const desktopBlocked = computed(() => !!desktop && useClerk.value && auth.value!.clerk!.publishable_key!.startsWith('pk_live_'))
const canSignUp = computed(() => useClerk.value || !!auth.value?.local.registration_enabled)
const providers = computed(() => useClerk.value ? auth.value!.clerk!.social_providers : [])
const visibleProviders = computed(() => showMore.value ? providers.value : providers.value.slice(0, 2))

const title = computed(() => {
	if (step.value === 'totp' || (step.value === 'code' && codeFor.value !== 'signup')) return t('twoFactor')
	if (step.value === 'code') return t('verifyEmail')
	if (mode.value === 'reset') return t('resetPassword')
	return mode.value === 'signin' ? t('login') : t('register')
})

const codeHint = computed(() => {
	if (step.value === 'totp' || codeFor.value === 'totp') return t('totpPrompt')
	if (codeFor.value === 'backup_code') return t('backupCodePrompt')
	return t('codeSentTo', {target: codeTarget.value})
})

const PROVIDER_NAMES: Record<string, string> = {
	google: 'Google',
	apple: 'Apple',
	github: 'GitHub',
	microsoft: 'Microsoft',
	facebook: 'Facebook',
	discord: 'Discord',
	gitlab: 'GitLab',
	linkedin_oidc: 'LinkedIn',
	x: 'X',
}

function providerName(p: string) {
	return PROVIDER_NAMES[p] ?? p.charAt(0).toUpperCase() + p.slice(1)
}

function switchMode(next: Mode) {
	mode.value = next
	step.value = 'form'
	error.value = ''
	notice.value = ''
	code.value = ''
}

/** Clerk errors carry a readable message per field; show the first one. */
function messageOf(e: unknown): string {
	const errors = (e as {errors?: {longMessage?: string, message?: string}[]}).errors
	if (errors?.length) return errors[0].longMessage || errors[0].message || ''
	return (e as Error).message
}

async function run(fn: () => Promise<void>) {
	error.value = ''
	notice.value = ''
	busy.value = true
	try {
		await fn()
	} catch (e) {
		error.value = messageOf(e)
	} finally {
		busy.value = false
	}
}

// ---------- self hosted ----------

async function submitLocal() {
	try {
		if (mode.value === 'signin') {
			await data.login(username.value.trim(), password.value, totp.value || undefined)
		} else {
			await data.register(username.value.trim(), email.value.trim(), password.value)
		}
	} catch (e) {
		// Vikunja error 1017: TOTP code required.
		if (e instanceof ApiError && (e.code === 1017 || /totp/i.test(e.message))) {
			step.value = 'totp'
			if (!totp.value) return
		}
		throw e
	}
}

// ---------- Clerk ----------

/** Turns the active Clerk session into an app session. */
async function exchange() {
	const token = await clerk!.session?.getToken()
	if (!token) throw new Error(t('authUnavailable'))
	try {
		await data.loginWithClerk(token, totp.value || undefined)
	} catch (e) {
		if (e instanceof ApiError && e.code === 1017) {
			step.value = 'totp'
			if (!totp.value) return
		}
		throw e
	}
	if (route.path === SSO_CALLBACK_PATH) router.replace('/')
}

async function finishSignIn() {
	const signIn = clerk!.client!.signIn
	switch (signIn.status) {
	case 'complete':
		await clerk!.setActive({session: signIn.createdSessionId})
		await exchange()
		return
	case 'needs_second_factor':
	case 'needs_client_trust':
		await startSecondFactor()
		return
	default:
		throw new Error(t('authUnavailable'))
	}
}

async function startSecondFactor() {
	const signIn = clerk!.client!.signIn
	const factors = signIn.supportedSecondFactors ?? []
	const pick = (['totp', 'email_code', 'phone_code', 'backup_code'] as const)
		.map(s => factors.find(f => f.strategy === s))
		.find(f => f)
	if (!pick) throw new Error(t('authUnavailable'))

	codeFor.value = pick.strategy as SecondFactor
	codeTarget.value = 'safeIdentifier' in pick ? pick.safeIdentifier ?? '' : ''
	if (pick.strategy === 'email_code') {
		await signIn.prepareSecondFactor({strategy: 'email_code', emailAddressId: pick.emailAddressId})
	} else if (pick.strategy === 'phone_code') {
		await signIn.prepareSecondFactor({strategy: 'phone_code', phoneNumberId: pick.phoneNumberId})
	}
	code.value = ''
	step.value = 'code'
}

async function submitClerk() {
	if (step.value === 'totp') {
		await exchange()
		return
	}

	if (step.value === 'code') {
		if (codeFor.value === 'signup') {
			const res = await clerk!.client!.signUp.attemptEmailAddressVerification({code: code.value.trim()})
			if (res.status !== 'complete') throw new Error(t('authUnavailable'))
			await clerk!.setActive({session: res.createdSessionId})
			await exchange()
		} else {
			await clerk!.client!.signIn.attemptSecondFactor({strategy: codeFor.value, code: code.value.trim()})
			await finishSignIn()
		}
		return
	}

	if (step.value === 'resetCode') {
		await clerk!.client!.signIn.attemptFirstFactor({
			strategy: 'reset_password_email_code',
			code: code.value.trim(),
			password: password.value,
		})
		await finishSignIn()
		return
	}

	if (mode.value === 'reset') {
		await clerk!.client!.signIn.create({strategy: 'reset_password_email_code', identifier: email.value.trim()})
		codeTarget.value = email.value.trim()
		code.value = ''
		password.value = ''
		step.value = 'resetCode'
		return
	}

	if (mode.value === 'signup') {
		const signUp = await clerk!.client!.signUp.create({emailAddress: email.value.trim(), password: password.value})
		if (signUp.status === 'complete') {
			await clerk!.setActive({session: signUp.createdSessionId})
			await exchange()
			return
		}
		await signUp.prepareEmailAddressVerification({strategy: 'email_code'})
		codeFor.value = 'signup'
		codeTarget.value = email.value.trim()
		code.value = ''
		step.value = 'code'
		return
	}

	await clerk!.client!.signIn.create({identifier: email.value.trim(), password: password.value})
	await finishSignIn()
}

async function resend() {
	await run(async () => {
		if (codeFor.value === 'signup') {
			await clerk!.client!.signUp.prepareEmailAddressVerification({strategy: 'email_code'})
		} else if (step.value === 'resetCode') {
			await clerk!.client!.signIn.create({strategy: 'reset_password_email_code', identifier: codeTarget.value})
		} else {
			await startSecondFactor()
		}
		notice.value = t('codeResent')
	})
}

function social(provider: string) {
	run(async () => {
		const target = window.location.origin + SSO_CALLBACK_PATH
		const resource = mode.value === 'signup' ? clerk!.client!.signUp : clerk!.client!.signIn
		await resource.authenticateWithRedirect({
			strategy: `oauth_${provider}` as `oauth_${string}` as never,
			redirectUrl: target,
			redirectUrlComplete: target,
		})
	})
}

// ---------- setup ----------

function submit() {
	run(async () => {
		if (desktop) serverUrl.value = await desktop.setServerUrl(serverUrl.value.trim())
		if (useClerk.value) await submitClerk()
		else await submitLocal()
	})
}

async function loadAuth(force = false) {
	auth.value = await getAuthInfo(force)
	if (!auth.value) {
		if (!desktop || serverUrl.value) error.value = t('authUnavailable')
		return
	}
	if (!useClerk.value || desktopBlocked.value) return

	clerk = await loadClerk(auth.value.clerk!.publishable_key!)
	if (route.path === SSO_CALLBACK_PATH || window.location.pathname === SSO_CALLBACK_PATH) {
		// Back from Google or Apple. Clerk would navigate to its own pages
		// for further steps; this screen renders them instead.
		await clerk.handleRedirectCallback({
			signInFallbackRedirectUrl: SSO_CALLBACK_PATH,
			signUpFallbackRedirectUrl: SSO_CALLBACK_PATH,
		}, async () => {})
		const signIn = clerk.client?.signIn
		if (!clerk.session && signIn && ['needs_second_factor', 'needs_client_trust'].includes(signIn.status ?? '')) {
			await startSecondFactor()
			return
		}
	}
	// A live Clerk session (the app token expired, or back from a provider)
	// signs straight in.
	if (clerk.session) await exchange()
}

async function changeServer() {
	if (!desktop) return
	await run(async () => {
		serverUrl.value = await desktop!.setServerUrl(serverUrl.value.trim())
		await loadAuth(true)
	})
}

onMounted(async () => {
	if (desktop) serverUrl.value ||= await desktop.getServerUrl()
	await run(() => loadAuth())
	booting.value = false
})
</script>

<template>
	<div class="login">
		<header class="top">
			<div class="brand">
				<svg viewBox="0 0 32 32" width="30" height="30" aria-hidden="true"><rect width="32" height="32" rx="9" fill="#4772FA" /><path d="M9 16.5l4.5 4.5L23 11.5" fill="none" stroke="#fff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" /></svg>
				<span class="wordmark">OpenTick</span>
			</div>
			<div v-if="canSignUp && step === 'form'" class="top-switch">
				<span>{{ mode === 'signup' ? t('haveAccount') : t('noAccount') }}</span>
				<button class="top-btn" type="button" @click="switchMode(mode === 'signup' ? 'signin' : 'signup')">
					{{ mode === 'signup' ? t('login') : t('register') }}
				</button>
			</div>
		</header>

		<main class="center">
			<div v-if="booting" class="card loading">
				<div class="spinner" />
			</div>

			<form v-else class="card" @submit.prevent="submit">
				<h1>{{ title }}</h1>

				<input v-if="desktop && step === 'form'" v-model="serverUrl" class="field" inputmode="url" spellcheck="false" :placeholder="t('serverUrl')" autocomplete="url" required @change="changeServer">

				<p v-if="desktopBlocked" class="hint">{{ t('desktopHostedSignIn') }}</p>

				<template v-else-if="step === 'form'">
					<p v-if="mode === 'reset'" class="hint">{{ t('resetPasswordHint') }}</p>
					<template v-if="useClerk">
						<input v-model="email" class="field" type="email" :placeholder="t('email')" autocomplete="email" required autofocus>
					</template>
					<template v-else>
						<input v-model="username" class="field" :placeholder="mode === 'signin' ? t('username') : t('usernameOnly')" autocomplete="username" required autofocus>
						<input v-if="mode === 'signup'" v-model="email" class="field" type="email" :placeholder="t('email')" autocomplete="email" required>
					</template>
					<input v-if="mode !== 'reset'" v-model="password" class="field" type="password" :placeholder="t('password')" :autocomplete="mode === 'signin' ? 'current-password' : 'new-password'" required>
				</template>

				<template v-else>
					<p class="hint">{{ step === 'totp' ? t('totpPrompt') : codeHint }}</p>
					<input v-if="step === 'totp'" v-model="totp" class="field" inputmode="numeric" :placeholder="t('totp')" autocomplete="one-time-code" required autofocus>
					<input v-else v-model="code" class="field" :inputmode="codeFor === 'backup_code' ? 'text' : 'numeric'" :placeholder="t('verificationCode')" autocomplete="one-time-code" required autofocus>
					<input v-if="step === 'resetCode'" v-model="password" class="field" type="password" :placeholder="t('newPassword')" autocomplete="new-password" required>
				</template>

				<!-- Clerk mounts its bot protection challenge here during sign up. -->
				<div v-if="useClerk && mode === 'signup'" id="clerk-captcha" />

				<p v-if="error" class="error">{{ error }}</p>
				<p v-if="notice" class="notice">{{ notice }}</p>

				<button v-if="!desktopBlocked" class="submit" :disabled="busy">
					<span v-if="busy" class="spinner small" />
					<template v-else-if="step !== 'form'">{{ t('verify') }}</template>
					<template v-else-if="mode === 'reset'">{{ t('sendCode') }}</template>
					<template v-else>{{ mode === 'signin' ? t('login') : t('register') }}</template>
				</button>

				<button v-if="mode === 'signin' && step === 'form' && useClerk && !desktopBlocked" class="link" type="button" @click="switchMode('reset')">
					{{ t('forgotPassword') }}
				</button>
				<button v-if="step === 'code' || step === 'resetCode'" class="link" type="button" :disabled="busy" @click="resend">
					{{ t('resendCode') }}
				</button>
				<button v-if="mode === 'reset' || step !== 'form'" class="link" type="button" @click="switchMode('signin')">
					{{ t('backToSignIn') }}
				</button>

				<template v-if="providers.length && step === 'form' && mode !== 'reset' && !desktopBlocked">
					<div class="divider" />
					<button
						v-for="p in visibleProviders"
						:key="p"
						type="button"
						class="social"
						:disabled="busy"
						@click="social(p)"
					>
						<svg v-if="p === 'google'" viewBox="0 0 48 48" width="18" height="18" aria-hidden="true">
							<path fill="#FFC107" d="M43.6 20.5H42V20H24v8h11.3C33.7 32.7 29.2 36 24 36c-6.6 0-12-5.4-12-12s5.4-12 12-12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 12.9 4 4 12.9 4 24s8.9 20 20 20 20-8.9 20-20c0-1.3-.1-2.4-.4-3.5z" />
							<path fill="#FF3D00" d="M6.3 14.7l6.6 4.8C14.7 15.1 19 12 24 12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 16.3 4 9.7 8.3 6.3 14.7z" />
							<path fill="#4CAF50" d="M24 44c5.2 0 9.9-2 13.4-5.2l-6.2-5.2C29.2 35.1 26.7 36 24 36c-5.2 0-9.6-3.3-11.3-7.9l-6.5 5C9.5 39.6 16.2 44 24 44z" />
							<path fill="#1976D2" d="M43.6 20.5H42V20H24v8h11.3c-.8 2.2-2.2 4.2-4.1 5.6l6.2 5.2C37 39.2 44 34 44 24c0-1.3-.1-2.4-.4-3.5z" />
						</svg>
						<svg v-else-if="p === 'apple'" viewBox="0 0 384 512" width="16" height="16" aria-hidden="true">
							<path fill="currentColor" d="M318.7 268.7c-.2-36.7 16.4-64.4 50-84.8-18.8-26.9-47.2-41.7-84.7-44.6-35.5-2.8-74.3 20.7-88.5 20.7-15 0-49.4-19.7-76.4-19.7C63.3 141.2 4 184.8 4 273.5q0 39.3 14.4 81.2c12.8 36.7 59 126.7 107.2 125.2 25.2-.6 43-17.9 75.8-17.9 31.8 0 48.3 17.9 76.4 17.9 48.6-.7 90.4-82.5 102.6-119.3-65.2-30.7-61.7-90-61.7-91.9zm-56.6-164.2c27.3-32.4 24.8-61.9 24-72.5-24.1 1.4-52 16.4-67.9 34.9-17.5 19.8-27.8 44.3-25.6 71.9 26.1 2 49.9-11.4 69.5-34.3z" />
						</svg>
						<svg v-else-if="p === 'github'" viewBox="0 0 16 16" width="17" height="17" aria-hidden="true">
							<path fill="currentColor" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
						</svg>
						<svg v-else-if="p === 'microsoft'" viewBox="0 0 21 21" width="16" height="16" aria-hidden="true">
							<path fill="#F25022" d="M1 1h9v9H1z" /><path fill="#7FBA00" d="M11 1h9v9h-9z" /><path fill="#00A4EF" d="M1 11h9v9H1z" /><path fill="#FFB900" d="M11 11h9v9h-9z" />
						</svg>
						<span v-else class="initial" aria-hidden="true">{{ providerName(p).charAt(0) }}</span>
						<span>{{ t('continueWith', {provider: providerName(p)}) }}</span>
					</button>
					<button v-if="providers.length > 2 && !showMore" type="button" class="link" @click="showMore = true">
						{{ t('moreProviders') }}
					</button>
				</template>
			</form>

			<p v-if="canSignUp && step === 'form' && !booting" class="bottom-switch">
				<template v-if="mode === 'signup'">{{ t('haveAccount') }} <a href="#" @click.prevent="switchMode('signin')">{{ t('login') }}</a></template>
				<template v-else>{{ t('noAccount') }} <a href="#" @click.prevent="switchMode('signup')">{{ t('register') }}</a></template>
			</p>
		</main>
	</div>
</template>

<style scoped>
/* TickTick's sign in page keeps its light look whatever the app theme is. */
.login {
	--l-bg: #f8f8f8;
	--l-card: #ffffff;
	--l-text: #191919;
	--l-muted: #757575;
	--l-border: #e3e3e3;
	--l-accent: #4772fa;
	--l-accent-hover: #3a63e6;
	display: flex;
	flex-direction: column;
	height: 100%;
	overflow: auto;
	background: var(--l-bg);
	color: var(--l-text);
	color-scheme: light;
}

.top {
	display: flex;
	align-items: center;
	justify-content: space-between;
	flex-shrink: 0;
	padding: 26px 40px 0;
}

html.desktop .top {
	padding-top: 30px;
}

.brand {
	display: flex;
	align-items: center;
	gap: 8px;
}

.wordmark {
	color: var(--l-accent);
	font-size: 26px;
	font-weight: 500;
	letter-spacing: -0.2px;
}

.top-switch {
	display: flex;
	align-items: center;
	gap: 16px;
	color: var(--l-muted);
	font-size: 13px;
}

.top-btn {
	height: 34px;
	padding: 0 17px;
	border-radius: 8px;
	background: var(--l-accent);
	color: #fff;
	font-size: 13px;
}

.top-btn:hover,
.submit:hover:not(:disabled) {
	background: var(--l-accent-hover);
}

.center {
	display: flex;
	flex: 1;
	flex-direction: column;
	align-items: center;
	padding: 6vh 16px 40px;
}

.card {
	display: flex;
	flex-direction: column;
	gap: 14px;
	width: 100%;
	max-width: 382px;
	margin-top: 8vh;
	padding: 44px 46px 46px;
	border-radius: 14px;
	background: var(--l-card);
	box-shadow: 0 4px 24px rgba(0, 0, 0, 0.04);
}

.card.loading {
	align-items: center;
	justify-content: center;
	min-height: 220px;
}

h1 {
	margin: 0 0 10px;
	font-size: 26px;
	font-weight: 700;
}

.field {
	height: 40px;
	padding: 0 10px;
	border: 1px solid var(--l-border);
	border-radius: 8px;
	background: #fff;
	color: var(--l-text);
	font-size: 14px;
	outline: none;
}

.field::placeholder {
	color: #a3a3a3;
}

.field:focus {
	border-color: var(--l-accent);
}

.hint {
	margin: -4px 0 2px;
	color: var(--l-muted);
	font-size: 13px;
	line-height: 1.5;
}

.submit {
	display: flex;
	align-items: center;
	justify-content: center;
	height: 40px;
	border-radius: 8px;
	background: var(--l-accent);
	color: #fff;
	font-size: 14px;
}

.submit:disabled {
	opacity: 0.8;
}

.link {
	align-self: center;
	color: var(--l-muted);
	font-size: 13px;
}

.link:hover {
	color: var(--l-text);
}

.divider {
	height: 1px;
	margin: 4px 0 10px;
	background: #f0f0f0;
}

.social {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 8px;
	height: 40px;
	border: 1px solid var(--l-border);
	border-radius: 8px;
	background: #fff;
	color: var(--l-text);
	font-size: 14px;
}

.social:hover:not(:disabled) {
	background: #fafafa;
}

.initial {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 18px;
	height: 18px;
	border-radius: 50%;
	background: #e9e9e9;
	font-size: 11px;
	font-weight: 600;
}

/* Takes no room until Clerk mounts a challenge into it. */
#clerk-captcha:empty {
	display: none;
}

.error {
	margin: 0;
	color: #e5484d;
	font-size: 13px;
}

.notice {
	margin: 0;
	color: var(--l-muted);
	font-size: 13px;
}

.bottom-switch {
	margin: 44px 0 0;
	color: var(--l-muted);
	font-size: 13px;
}

.bottom-switch a {
	color: var(--l-accent);
	text-decoration: none;
}

.spinner {
	width: 24px;
	height: 24px;
	border: 3px solid #e3e3e3;
	border-top-color: var(--l-accent);
	border-radius: 50%;
	animation: spin 0.8s linear infinite;
}

.spinner.small {
	width: 16px;
	height: 16px;
	border-width: 2px;
	border-color: rgba(255, 255, 255, 0.4);
	border-top-color: #fff;
}

@keyframes spin {
	to {
		transform: rotate(360deg);
	}
}

@media (max-width: 520px) {
	.top {
		padding: 18px 16px 0;
	}

	.top-switch > span {
		display: none;
	}

	.card {
		margin-top: 2vh;
		padding: 32px 22px;
	}
}
</style>
