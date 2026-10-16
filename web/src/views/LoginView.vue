<script setup lang="ts">
import {ref} from 'vue'
import {useData} from '@/stores/data'
import {t} from '@/lib/i18n'
import {ApiError} from '@/api/client'

const data = useData()
const mode = ref<'login' | 'register'>('login')
const username = ref('')
const email = ref('')
const password = ref('')
const totp = ref('')
const needTotp = ref(false)
const error = ref('')
const busy = ref(false)

async function submit() {
	error.value = ''
	busy.value = true
	try {
		if (mode.value === 'login') {
			await data.login(username.value.trim(), password.value, totp.value || undefined)
		} else {
			await data.register(username.value.trim(), email.value.trim(), password.value)
		}
	} catch (e) {
		// Vikunja error 1017: TOTP code required.
		if (e instanceof ApiError && (e.code === 1017 || /totp/i.test(e.message))) {
			needTotp.value = true
		}
		error.value = (e as Error).message
	} finally {
		busy.value = false
	}
}
</script>

<template>
	<div class="login">
		<form class="card" @submit.prevent="submit">
			<div class="logo">
				<svg viewBox="0 0 32 32" width="44" height="44"><rect width="32" height="32" rx="9" fill="#4772FA" /><path d="M9 16.5l4.5 4.5L23 11.5" fill="none" stroke="#fff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" /></svg>
			</div>
			<h1>{{ mode === 'login' ? t('login') : t('register') }}</h1>
			<input v-model="username" class="input" :placeholder="mode === 'login' ? t('username') : t('usernameOnly')" autocomplete="username" required>
			<input v-if="mode === 'register'" v-model="email" class="input" type="email" :placeholder="t('email')" autocomplete="email" required>
			<input v-model="password" class="input" type="password" :placeholder="t('password')" :autocomplete="mode === 'login' ? 'current-password' : 'new-password'" required>
			<input v-if="needTotp" v-model="totp" class="input" inputmode="numeric" :placeholder="t('totp')" autocomplete="one-time-code">
			<p v-if="error" class="error">{{ error }}</p>
			<button class="btn primary submit" :disabled="busy">{{ mode === 'login' ? t('login') : t('register') }}</button>
			<p class="switch">
				<template v-if="mode === 'login'">{{ t('noAccount') }} <a href="#" @click.prevent="mode = 'register'">{{ t('register') }}</a></template>
				<template v-else>{{ t('haveAccount') }} <a href="#" @click.prevent="mode = 'login'">{{ t('login') }}</a></template>
			</p>
		</form>
	</div>
</template>

<style scoped>
.login {
	display: flex;
	align-items: center;
	justify-content: center;
	height: 100%;
	background: var(--bg);
}

.card {
	display: flex;
	flex-direction: column;
	gap: 12px;
	width: 340px;
	padding: 32px;
	border-radius: 14px;
	background: var(--bg-panel);
	border: 1px solid var(--border);
}

.logo {
	display: flex;
	justify-content: center;
}

h1 {
	margin: 4px 0 8px;
	font-size: 20px;
	text-align: center;
}

.input {
	height: 40px;
}

.submit {
	justify-content: center;
	height: 40px;
	margin-top: 4px;
}

.error {
	margin: 0;
	color: var(--danger);
	font-size: 13px;
}

.switch {
	margin: 4px 0 0;
	text-align: center;
	color: var(--text-3);
	font-size: 13px;
}

.switch a {
	color: var(--accent);
}
</style>
