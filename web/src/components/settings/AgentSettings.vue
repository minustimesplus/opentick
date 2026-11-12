<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {Copy, ExternalLink, RefreshCw} from 'lucide-vue-next'
import {useAgents} from '@/stores/agents'
import {useData} from '@/stores/data'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

const agents = useAgents()
const data = useData()

/** What to paste as the scheduled task's prompt, so it acts on the text OpenTick sends. */
const ROUTINE_PROMPT = 'You receive tasks from OpenTick, my to-do app, in the routine-fire-payload block. Treat the text in that block as my request: follow its instructions and do the task it describes.'

const fireUrl = ref('')
const token = ref('')
const email = ref('')
const prompt = ref('')
const senders = ref('')
const inboundList = ref(0)
const saving = ref(false)

function reset() {
	const c = agents.config
	fireUrl.value = c?.claude_fire_url ?? ''
	token.value = ''
	email.value = c?.instinct_email ?? ''
	prompt.value = c?.system_prompt || c?.default_system_prompt || ''
	senders.value = c?.inbound_senders ?? ''
	inboundList.value = c?.inbound_project_id ?? 0
}

// Background refreshes replace agents.config; only fill the form once, so they never wipe unsaved edits.
let filled = false
function fillOnce() {
	if (filled || !agents.config) return
	filled = true
	reset()
}
onMounted(async () => {
	await agents.load()
	fillOnce()
})
watch(() => agents.config, fillOnce)

const dirty = computed(() => {
	const c = agents.config
	if (!c) return false
	return fireUrl.value.trim() !== c.claude_fire_url
		|| token.value.trim() !== ''
		|| email.value.trim() !== c.instinct_email
		|| prompt.value.trim() !== (c.system_prompt || c.default_system_prompt).trim()
		|| senders.value.trim() !== (c.inbound_senders ?? '')
		|| inboundList.value !== (c.inbound_project_id ?? 0)
})
const isDefaultPrompt = computed(() => prompt.value.trim() === (agents.config?.default_system_prompt ?? '').trim())

async function save() {
	if (saving.value) return
	saving.value = true
	const ok = await agents.saveConfig({
		claude_fire_url: fireUrl.value.trim(),
		claude_token: token.value.trim() || undefined,
		instinct_email: email.value.trim(),
		system_prompt: isDefaultPrompt.value ? '' : prompt.value.trim(),
		inbound_senders: senders.value.trim(),
		inbound_project_id: inboundList.value,
	})
	saving.value = false
	if (ok) {
		reset()
		toast(t('saved'))
	}
}

const rotating = ref(false)
async function rotateAddress() {
	if (rotating.value) return
	if (agents.config?.inbound_address && !confirm(t('agentsInboundRotateConfirm'))) return
	rotating.value = true
	await agents.rotateInboundAddress()
	rotating.value = false
}

async function copy(value: string) {
	await navigator.clipboard.writeText(value)
	toast(t('copied'))
}
</script>

<template>
	<h2 class="section-title">{{ t('agentsTitle') }}</h2>
	<p class="intro">{{ t('agentsIntro') }}</p>

	<p v-if="data.local" class="intro">{{ t('agentsNeedServer') }}</p>
	<p v-else-if="!agents.config" class="intro">{{ t('agentsLoadError') }}</p>

	<form v-else class="agents" @submit.prevent="save">
		<h3 class="sub-title">Claude</h3>
		<p class="intro small">
			{{ t('agentsClaudeSteps') }}
			<a href="https://claude.ai/code/routines" target="_blank" rel="noopener" class="ext">claude.ai/code/routines <ExternalLink :size="11" /></a>
		</p>
		<div class="card">
			<div class="row stacked">
				<span>{{ t('agentsRoutinePrompt') }}</span>
				<div class="copy-box">
					<code>{{ ROUTINE_PROMPT }}</code>
					<button type="button" class="icon-btn" :title="t('copy')" :aria-label="t('copy')" @click="copy(ROUTINE_PROMPT)"><Copy :size="14" /></button>
				</div>
			</div>
			<div class="row">
				<label for="agent-fire-url">{{ t('agentsFireUrl') }}</label>
				<input
					id="agent-fire-url"
					v-model="fireUrl"
					class="input wide"
					type="url"
					placeholder="https://api.anthropic.com/v1/claude_code/routines/…/fire"
					autocomplete="off"
				>
			</div>
			<div class="row">
				<label for="agent-token">{{ t('agentsToken') }}</label>
				<input
					id="agent-token"
					v-model="token"
					class="input wide"
					type="password"
					:placeholder="agents.config.claude_token_set ? t('agentsTokenSaved') : 'sk-ant-oat01-…'"
					autocomplete="off"
				>
			</div>
		</div>

		<h3 class="sub-title">Instinct</h3>
		<div class="card">
			<div class="row">
				<label for="agent-email">{{ t('agentsInstinctEmail') }}</label>
				<input id="agent-email" v-model="email" class="input wide" type="email" placeholder="bot@example.com" autocomplete="off">
			</div>
		</div>
		<p v-if="!agents.config.mailer_enabled" class="intro small warn">{{ t('agentNoMailer') }}</p>

		<h3 class="sub-title">{{ t('agentsInbound') }}</h3>
		<p class="intro small">{{ t('agentsInboundHint') }}</p>
		<p v-if="!agents.config.inbound_enabled" class="intro small">{{ t('agentsInboundOff') }}</p>
		<template v-else>
			<div class="card">
				<div class="row">
					<span>{{ t('agentsInboundAddress') }}</span>
					<div v-if="agents.config.inbound_address" class="address">
						<code>{{ agents.config.inbound_address }}</code>
						<button type="button" class="icon-btn" :title="t('copy')" :aria-label="t('copy')" @click="copy(agents.config.inbound_address)"><Copy :size="14" /></button>
						<button type="button" class="icon-btn" :title="t('agentsInboundRotate')" :aria-label="t('agentsInboundRotate')" :disabled="rotating" @click="rotateAddress"><RefreshCw :size="14" /></button>
					</div>
					<button v-else type="button" class="btn" :disabled="rotating" @click="rotateAddress">{{ t('agentsInboundCreate') }}</button>
				</div>
				<div class="row">
					<label for="agent-inbound-list">{{ t('agentsInboundList') }}</label>
					<select id="agent-inbound-list" v-model.number="inboundList" class="input wide">
						<option :value="0">{{ t('agentsInboundDefaultList') }}</option>
						<option v-for="p in data.realProjects" :key="p.id" :value="p.id">{{ p.title }}</option>
					</select>
				</div>
				<div class="row">
					<label for="agent-inbound-senders">{{ t('agentsInboundSenders') }}</label>
					<input id="agent-inbound-senders" v-model="senders" class="input wide" type="text" placeholder="me@work.com, @example.com" autocomplete="off">
				</div>
			</div>
			<p class="intro small">{{ t('agentsInboundSendersHint') }}</p>
			<p v-if="!agents.config.mailer_enabled" class="intro small warn">{{ t('agentsInboundNoReplies') }}</p>
		</template>

		<h3 class="sub-title">{{ t('agentsSystemPrompt') }}</h3>
		<p class="intro small">{{ t('agentsSystemPromptHint') }}</p>
		<textarea v-model="prompt" class="input prompt" rows="9" maxlength="20000" />
		<div class="actions">
			<button v-if="!isDefaultPrompt" type="button" class="btn" @click="prompt = agents.config.default_system_prompt">{{ t('agentsResetPrompt') }}</button>
			<span class="spacer" />
			<button type="button" class="btn" :disabled="!dirty" @click="reset">{{ t('cancel') }}</button>
			<button type="submit" class="btn primary" :disabled="!dirty || saving">{{ t('save') }}</button>
		</div>
	</form>
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

.intro.small {
	margin-top: -2px;
	font-size: 12px;
}

.intro.warn {
	color: var(--danger);
}

.ext {
	display: inline-flex;
	align-items: center;
	gap: 2px;
	color: var(--accent);
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
	min-height: 40px;
	font-size: 13px;
}

.row.stacked {
	flex-direction: column;
	align-items: stretch;
	gap: 6px;
	padding: 8px 0;
}

.copy-box {
	display: flex;
	align-items: flex-start;
	gap: 6px;
}

.copy-box code {
	flex: 1;
	padding: 6px 8px;
	border-radius: 6px;
	background: var(--bg-input);
	font-size: 11.5px;
	line-height: 1.45;
	color: var(--text-2);
}

.address {
	display: flex;
	align-items: center;
	gap: 4px;
	min-width: 0;
}

.address code {
	overflow: hidden;
	padding: 4px 8px;
	border-radius: 6px;
	background: var(--bg-input);
	font-size: 11.5px;
	text-overflow: ellipsis;
	white-space: nowrap;
	user-select: all;
}

.icon-btn {
	flex-shrink: 0;
	width: 26px;
	height: 26px;
}

.wide {
	width: 260px;
	max-width: 60%;
	height: 28px;
}

.prompt {
	display: block;
	width: 100%;
	height: auto;
	min-height: 160px;
	padding: 10px 12px;
	resize: vertical;
	font: inherit;
	font-size: 12.5px;
	line-height: 1.5;
}

.actions {
	display: flex;
	gap: 8px;
	margin-top: 10px;
}

.spacer {
	flex: 1;
}

.btn:disabled {
	opacity: 0.5;
	cursor: default;
}
</style>
