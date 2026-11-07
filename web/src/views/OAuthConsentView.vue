<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useRoute} from 'vue-router'
import {api} from '@/api/client'
import {t, type MessageKey} from '@/lib/i18n'
import {takeOAuthReturn} from '@/lib/oauthReturn'

// The OAuth authorization endpoint for clients registered via dynamic client
// registration, e.g. Claude connecting to the MCP server. The backend checks
// everything again; this page only asks the user.

interface ClientInfo {
	client_id: string
	client_name: string
	redirect_uris: string[]
}

type Preset = 'typed' | 'read_only' | 'full'

const route = useRoute()

const query = computed(() => {
	const q = route.query
	const str = (k: string) => typeof q[k] === 'string' ? q[k] as string : ''
	return {
		responseType: str('response_type'),
		clientId: str('client_id'),
		redirectUri: str('redirect_uri'),
		state: str('state'),
		codeChallenge: str('code_challenge'),
		codeChallengeMethod: str('code_challenge_method'),
		resource: str('resource'),
	}
})

const client = ref<ClientInfo | null>(null)
const error = ref('')
const loading = ref(true)
const busy = ref(false)
const preset = ref<Preset>('typed')

const presets: {value: Preset, label: MessageKey, hint: MessageKey}[] = [
	{value: 'typed', label: 'mcpPresetTyped', hint: 'oauthPresetTypedHint'},
	{value: 'read_only', label: 'mcpPresetReadOnly', hint: 'oauthPresetReadOnlyHint'},
	{value: 'full', label: 'mcpPresetFull', hint: 'oauthPresetFullHint'},
]

const redirectHost = computed(() => {
	try {
		return new URL(query.value.redirectUri).host
	} catch {
		return ''
	}
})

onMounted(async () => {
	// Signed in now, so the page no longer needs to be restored after login.
	takeOAuthReturn()
	const q = query.value
	try {
		if (q.responseType !== 'code' || !q.clientId || !q.redirectUri || !q.codeChallenge || q.codeChallengeMethod !== 'S256') {
			throw new Error(t('oauthInvalidRequest'))
		}
		const info = await api<ClientInfo>('GET', `/oauth/clients/${encodeURIComponent(q.clientId)}`)
		// Never redirect to a URI the client did not register.
		if (!info.redirect_uris.includes(q.redirectUri)) throw new Error(t('oauthInvalidRequest'))
		client.value = info
	} catch (e) {
		error.value = (e as Error).message || t('oauthInvalidRequest')
	} finally {
		loading.value = false
	}
})

function redirect(params: Record<string, string>) {
	const url = new URL(query.value.redirectUri)
	for (const [k, v] of Object.entries(params)) url.searchParams.set(k, v)
	if (query.value.state) url.searchParams.set('state', query.value.state)
	window.location.assign(url.toString())
}

async function allow() {
	if (busy.value) return
	busy.value = true
	error.value = ''
	const q = query.value
	try {
		const resp = await api<{code: string}>('POST', '/oauth/authorize', {body: {
			response_type: q.responseType,
			client_id: q.clientId,
			redirect_uri: q.redirectUri,
			state: q.state,
			code_challenge: q.codeChallenge,
			code_challenge_method: q.codeChallengeMethod,
			resource: q.resource || undefined,
			permissions: preset.value,
		}})
		redirect({code: resp.code})
	} catch (e) {
		error.value = (e as Error).message
		busy.value = false
	}
}

function deny() {
	redirect({error: 'access_denied'})
}
</script>

<template>
	<div class="consent">
		<header class="top">
			<div class="brand">
				<svg viewBox="0 0 32 32" width="30" height="30" aria-hidden="true"><rect width="32" height="32" rx="9" fill="#4772FA" /><path d="M9 16.5l4.5 4.5L23 11.5" fill="none" stroke="#fff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" /></svg>
				<span class="wordmark">OpenTick</span>
			</div>
		</header>

		<main class="center">
			<div v-if="loading" class="card loading">
				<div class="spinner" />
			</div>

			<div v-else-if="!client" class="card">
				<h1>{{ t('oauthTitle') }}</h1>
				<p class="error">{{ error }}</p>
			</div>

			<form v-else class="card" @submit.prevent="allow">
				<h1>{{ t('oauthTitle') }}</h1>
				<p class="hint">{{ t('oauthRequest', {client: client.client_name}) }}</p>
				<p class="host">{{ t('oauthRedirectsTo', {host: redirectHost}) }}</p>

				<fieldset class="presets">
					<legend>{{ t('mcpAccess') }}</legend>
					<label v-for="p in presets" :key="p.value" class="preset" :class="{active: preset === p.value}">
						<input v-model="preset" type="radio" name="preset" :value="p.value">
						<span class="label">{{ t(p.label) }}</span>
						<span class="sub">{{ t(p.hint) }}</span>
					</label>
				</fieldset>

				<p class="hint small">{{ t('oauthRevokeHint') }}</p>
				<p v-if="error" class="error">{{ error }}</p>

				<button class="submit" type="submit" :disabled="busy">{{ t('oauthAllow') }}</button>
				<button class="link" type="button" :disabled="busy" @click="deny">{{ t('cancel') }}</button>
			</form>
		</main>
	</div>
</template>

<style scoped>
/* Matches the sign in page, which keeps its light look whatever the app theme is. */
.consent {
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
	flex-shrink: 0;
	padding: 26px 40px 0;
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
	max-width: 420px;
	margin-top: 4vh;
	padding: 40px 42px 36px;
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
	margin: 0 0 4px;
	font-size: 24px;
	font-weight: 700;
}

.hint {
	margin: 0;
	color: var(--l-text);
	font-size: 14px;
	line-height: 1.5;
}

.hint.small,
.host {
	margin: 0;
	color: var(--l-muted);
	font-size: 13px;
	line-height: 1.5;
	overflow-wrap: anywhere;
}

.presets {
	display: flex;
	flex-direction: column;
	gap: 8px;
	margin: 0;
	padding: 0;
	border: 0;
}

.presets legend {
	margin-bottom: 8px;
	padding: 0;
	font-size: 13px;
	font-weight: 600;
}

.preset {
	display: grid;
	grid-template-columns: auto 1fr;
	column-gap: 10px;
	padding: 10px 12px;
	border: 1px solid var(--l-border);
	border-radius: 8px;
	cursor: pointer;
}

.preset.active {
	border-color: var(--l-accent);
}

.preset input {
	grid-row: span 2;
	margin: 3px 0 0;
	accent-color: var(--l-accent);
}

.preset .label {
	font-size: 14px;
}

.preset .sub {
	color: var(--l-muted);
	font-size: 12px;
	line-height: 1.4;
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

.submit:hover:not(:disabled) {
	background: var(--l-accent-hover);
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

.error {
	margin: 0;
	color: #e5484d;
	font-size: 13px;
}

.spinner {
	width: 26px;
	height: 26px;
	border: 2.5px solid var(--l-border);
	border-top-color: var(--l-accent);
	border-radius: 50%;
	animation: spin 0.8s linear infinite;
}

@keyframes spin {
	to {
		transform: rotate(360deg);
	}
}

@media (max-width: 480px) {
	.top {
		padding: 20px 16px 0;
	}

	.card {
		padding: 28px 20px;
	}
}
</style>
