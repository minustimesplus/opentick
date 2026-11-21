<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {Copy, KeyRound, Upload} from 'lucide-vue-next'
import {usePeople} from '@/stores/people'
import {api} from '@/api/client'
import type {ApiPermissions, ApiToken} from '@/api/types'
import {addDays} from '@/lib/dates'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

const people = usePeople()
const fileInput = ref<HTMLInputElement | null>(null)
const importing = ref(false)
const result = ref('')

const COMMAND = 'opentick import-personalcrm --url ' + window.location.origin + ' --token <token>'

/** Exactly the routes the LinkedIn extension calls (pkg/routes/api/v2/contacts_linkedin.go). */
const EXTENSION_PERMISSIONS: ApiPermissions = {
	contacts: ['linkedin_profiles', 'linkedin_failures', 'linkedin_queue', 'linkedin_lookup'],
}

const serverUrl = ref(window.location.origin)
const creatingToken = ref(false)
/** Cleartext of the token just created; the API never returns it again. */
const extensionToken = ref('')

onMounted(async () => {
	// The desktop app runs on a local origin; the server knows its public address.
	try {
		const info = await api<{frontend_url?: string}>('GET', '/info', {auth: false})
		const url = info.frontend_url?.replace(/\/+$/, '')
		if (url) serverUrl.value = url
	} catch {
		// Keep the page's own origin.
	}
})

async function pick(e: Event) {
	const input = e.target as HTMLInputElement
	const file = input.files?.[0]
	input.value = ''
	if (!file || importing.value) return
	importing.value = true
	result.value = ''
	const res = await people.importLinkedinCsv(await file.text())
	importing.value = false
	if (res) result.value = t('importResult', {created: res.created, updated: res.updated})
}

async function createExtensionToken() {
	if (creatingToken.value) return
	creatingToken.value = true
	try {
		const token = await api<ApiToken>('POST', '/tokens', {body: {
			title: t('importExtensionTokenTitle'),
			permissions: EXTENSION_PERMISSIONS,
			expires_at: addDays(new Date(), 365).toISOString(),
		}})
		extensionToken.value = token.token ?? ''
	} catch (e) {
		toast((e as Error).message)
	} finally {
		creatingToken.value = false
	}
}

async function copy(value: string) {
	await navigator.clipboard.writeText(value)
	toast(t('copied'))
}
</script>

<template>
	<div class="import">
		<section class="card">
			<h2>{{ t('importLinkedinCsv') }}</h2>
			<p>{{ t('importLinkedinCsvHint') }}</p>
			<input ref="fileInput" type="file" accept=".csv,text/csv" hidden @change="pick">
			<div class="row">
				<button class="btn primary" :aria-disabled="importing" @click="!importing && fileInput?.click()">
					<Upload :size="15" /> {{ importing ? t('importing') : t('importChooseCsv') }}
				</button>
				<span v-if="result" class="result">{{ result }}</span>
			</div>
		</section>

		<section class="card">
			<h2>{{ t('importExtension') }}</h2>
			<p>{{ t('importExtensionHint') }}</p>
			<ol>
				<li>{{ t('importExtensionStep1') }}</li>
				<li>{{ t('importExtensionStep2') }}</li>
				<li>{{ t('importExtensionStep3') }}</li>
			</ol>
			<div v-if="extensionToken" class="secret">
				<div class="secret-row">
					<span class="label">{{ t('importExtensionServer') }}</span>
					<code>{{ serverUrl }}</code>
					<button class="icon-btn" :title="t('copy')" :aria-label="t('copy')" @click="copy(serverUrl)">
						<Copy :size="14" :stroke-width="1.8" />
					</button>
				</div>
				<div class="secret-row">
					<span class="label">{{ t('importExtensionToken') }}</span>
					<code>{{ extensionToken }}</code>
					<button class="icon-btn" :title="t('copy')" :aria-label="t('copy')" @click="copy(extensionToken)">
						<Copy :size="14" :stroke-width="1.8" />
					</button>
				</div>
				<p class="note">{{ t('importExtensionTokenOnce') }}</p>
			</div>
			<div v-else class="row">
				<button class="btn primary" :aria-disabled="creatingToken" @click="createExtensionToken">
					<KeyRound :size="15" /> {{ creatingToken ? t('importExtensionCreating') : t('importExtensionCreate') }}
				</button>
			</div>
		</section>

		<section class="card">
			<h2>{{ t('importPersonalCrm') }}</h2>
			<p>{{ t('importPersonalCrmHint') }}</p>
			<pre>{{ COMMAND }}</pre>
		</section>
	</div>
</template>

<style scoped>
.import {
	display: flex;
	flex-direction: column;
	gap: 14px;
	max-width: 640px;
	padding: 4px 8px 24px;
}

.card {
	padding: 18px 20px;
	border-radius: 12px;
	background: var(--bg-panel);
}

h2 {
	margin: 0 0 6px;
	font-size: 15px;
	font-weight: 600;
}

p {
	margin: 0 0 14px;
	font-size: 13px;
	line-height: 1.5;
	color: var(--text-2);
}

ol {
	margin: 0 0 14px;
	padding-left: 20px;
	font-size: 13px;
	line-height: 1.5;
	color: var(--text-2);
}

li + li {
	margin-top: 4px;
}

.row {
	display: flex;
	align-items: center;
	gap: 12px;
}

.result {
	font-size: 13px;
	color: var(--text-2);
}

pre {
	margin: 0;
	padding: 10px 12px;
	overflow-x: auto;
	border-radius: 6px;
	background: var(--bg-input);
	font-size: 12px;
	user-select: all;
}

.secret {
	display: flex;
	flex-direction: column;
	gap: 8px;
}

.secret-row {
	display: flex;
	align-items: center;
	gap: 10px;
	min-width: 0;
}

.secret-row .label {
	flex: none;
	width: 56px;
	font-size: 13px;
	color: var(--text-2);
}

.secret-row code {
	flex: 1;
	min-width: 0;
	padding: 6px 10px;
	overflow-x: auto;
	border-radius: 6px;
	background: var(--bg-input);
	font-size: 12px;
	white-space: nowrap;
	user-select: all;
}

.note {
	margin: 0;
}

.icon-btn {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 28px;
	height: 28px;
	border: 0;
	border-radius: 6px;
	background: transparent;
	color: var(--text-2);
	cursor: pointer;
}

.icon-btn:hover {
	background: var(--bg-hover);
	color: var(--text);
}

.btn[aria-disabled="true"] {
	opacity: 0.5;
	cursor: default;
}
</style>
