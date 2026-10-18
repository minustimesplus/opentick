<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {Copy, Trash2} from 'lucide-vue-next'
import McpClientGuide from './McpClientGuide.vue'
import SettingSelect from './SettingSelect.vue'
import {api, fetchAll} from '@/api/client'
import type {ApiPermissions, ApiToken, McpInfo} from '@/api/types'
import {addDays, fmtDayMonth, parseDate} from '@/lib/dates'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

type Preset = 'typed' | 'readOnly' | 'full'

const info = ref<McpInfo | null>(null)
const tokens = ref<ApiToken[]>([])
const loadFailed = ref(false)

const creating = ref(false)
const saving = ref(false)
const title = ref('MCP')
const preset = ref<Preset>('typed')
const expiryDays = ref(90)
/** Cleartext of the token just created; the API never returns it again. */
const newToken = ref('')

const presets = computed(() => [
	{value: 'typed' as const, label: t('mcpPresetTyped')},
	{value: 'readOnly' as const, label: t('mcpPresetReadOnly')},
	{value: 'full' as const, label: t('mcpPresetFull')},
])

const expiries = computed(() => [
	{value: 30, label: t('mcpDays', {n: 30})},
	{value: 90, label: t('mcpDays', {n: 90})},
	{value: 365, label: t('mcpOneYear')},
])

async function load() {
	loadFailed.value = false
	try {
		const [mcpInfo, all] = await Promise.all([
			api<McpInfo>('GET', '/mcp/info'),
			fetchAll<ApiToken>('/tokens'),
		])
		info.value = mcpInfo
		tokens.value = all.filter(token => token.permissions?.mcp?.includes('access'))
	} catch {
		loadFailed.value = true
	}
}

onMounted(load)

/** Resolves a preset against the permissions MCP tools can use; group `*` and permission `*` are wildcards. */
function permissionsFor(which: Preset): ApiPermissions {
	const routes = info.value?.routes ?? {}
	const groups: Record<string, string[] | string> = {
		typed: info.value?.presets.typed ?? {},
		readOnly: info.value?.presets.read_only ?? {},
		full: info.value?.presets.full ?? {},
	}[which]
	const out: ApiPermissions = {}
	for (const [group, wanted] of Object.entries(groups)) {
		for (const target of group === '*' ? Object.keys(routes) : [group]) {
			const available = Object.keys(routes[target] ?? {})
			const picked = wanted === '*' ? available : available.filter(p => wanted.includes(p))
			if (picked.length) out[target] = [...new Set([...(out[target] ?? []), ...picked])]
		}
	}
	out.mcp = ['access']
	return out
}

async function create() {
	if (!title.value.trim() || saving.value) return
	saving.value = true
	try {
		const token = await api<ApiToken>('POST', '/tokens', {body: {
			title: title.value.trim(),
			permissions: permissionsFor(preset.value),
			expires_at: addDays(new Date(), expiryDays.value).toISOString(),
		}})
		newToken.value = token.token ?? ''
		tokens.value = [...tokens.value, token]
		creating.value = false
	} catch (e) {
		toast((e as Error).message)
	} finally {
		saving.value = false
	}
}

async function remove(token: ApiToken) {
	if (!confirm(t('mcpDeleteConfirm', {name: token.title}))) return
	try {
		await api('DELETE', `/tokens/${token.id}`)
		tokens.value = tokens.value.filter(other => other.id !== token.id)
	} catch (e) {
		toast((e as Error).message)
	}
}

function isExpired(token: ApiToken) {
	return (parseDate(token.expires_at)?.getTime() ?? Infinity) < Date.now()
}

function expiryLabel(token: ApiToken) {
	if (isExpired(token)) return t('mcpExpired')
	const date = parseDate(token.expires_at)
	return date ? t('mcpExpiresOn', {date: fmtDayMonth(date, true)}) : ''
}

async function copy(value: string) {
	await navigator.clipboard.writeText(value)
	toast(t('copied'))
}
</script>

<template>
	<h2 class="section-title">{{ t('mcp') }}</h2>
	<p class="intro">{{ t('mcpIntro') }}</p>

	<p v-if="loadFailed" class="intro error">{{ t('mcpLoadError') }}</p>

	<template v-if="info">
		<div class="card">
			<div class="row">
				<span>{{ t('mcpEndpoint') }}</span>
				<span class="endpoint">
					<code>{{ info.endpoint }}</code>
					<button class="icon-btn" :title="t('copy')" :aria-label="t('copy')" @click="copy(info.endpoint)">
						<Copy :size="14" :stroke-width="1.8" />
					</button>
				</span>
			</div>
		</div>

		<template v-if="newToken">
			<p class="intro success">{{ t('mcpTokenReady') }}</p>
			<McpClientGuide :endpoint="info.endpoint" :token="newToken" />
			<div class="actions">
				<button class="btn primary" @click="newToken = ''">{{ t('done') }}</button>
			</div>
		</template>

		<template v-else>
			<h3 class="sub-title">{{ t('mcpTokens') }}</h3>
			<div class="card">
				<div v-for="token in tokens" :key="token.id" class="row" :class="{expired: isExpired(token)}">
					<span class="token-title">{{ token.title }}</span>
					<span class="token-meta">
						<span>{{ expiryLabel(token) }}</span>
						<button class="icon-btn" :title="t('delete')" :aria-label="t('delete')" @click="remove(token)">
							<Trash2 :size="14" :stroke-width="1.8" />
						</button>
					</span>
				</div>
				<div v-if="!tokens.length" class="row muted">{{ t('mcpNoTokens') }}</div>
			</div>

			<form v-if="creating" class="card form" @submit.prevent="create">
				<div class="row">
					<label for="mcp-token-title">{{ t('mcpTokenName') }}</label>
					<input id="mcp-token-title" v-model="title" class="input title-input" required>
				</div>
				<div class="row">
					<span>{{ t('mcpAccess') }}</span>
					<SettingSelect v-model="preset" :options="presets" />
				</div>
				<div class="row">
					<span>{{ t('mcpExpiry') }}</span>
					<SettingSelect v-model="expiryDays" :options="expiries" />
				</div>
				<div class="actions">
					<button type="button" class="btn" @click="creating = false">{{ t('cancel') }}</button>
					<button type="submit" class="btn primary" :disabled="saving || !title.trim()">{{ t('mcpCreateToken') }}</button>
				</div>
			</form>
			<div v-else class="actions start">
				<button class="btn" @click="creating = true">{{ t('mcpCreateToken') }}</button>
			</div>
		</template>
	</template>
</template>

<style scoped>
.section-title {
	margin: 0 0 8px 15px;
	font-size: 14px;
	font-weight: 600;
}

.sub-title {
	margin: 18px 0 8px 15px;
	font-size: 13px;
	font-weight: 600;
}

.intro {
	margin: 0 15px 14px;
	font-size: 13px;
	line-height: 1.45;
	color: var(--text-2);
}

.intro.error {
	color: var(--danger);
}

.intro.success {
	color: var(--text);
}

.card {
	margin-bottom: 14px;
	padding: 2px 15px;
	border-radius: 8px;
	background: var(--settings-card);
}

.row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 16px;
	min-height: 36px;
	font-size: 13px;
}

.row.muted {
	color: var(--text-3);
}

.row.expired .token-title {
	color: var(--text-3);
	text-decoration: line-through;
}

.endpoint,
.token-meta {
	display: flex;
	align-items: center;
	gap: 4px;
	min-width: 0;
	margin-right: -6px;
	color: var(--text-2);
}

.endpoint code {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 12px;
}

.icon-btn {
	flex-shrink: 0;
	width: 26px;
	height: 26px;
}

.token-title {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.form {
	padding-bottom: 12px;
}

.title-input {
	width: 220px;
	height: 28px;
}

.actions {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	margin-top: 8px;
}

.actions.start {
	justify-content: flex-start;
	margin-left: 15px;
}

.btn:disabled {
	opacity: 0.5;
	cursor: default;
}
</style>
