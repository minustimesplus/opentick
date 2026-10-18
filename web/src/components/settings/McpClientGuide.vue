<script setup lang="ts">
import {computed} from 'vue'
import {Copy} from 'lucide-vue-next'
import SettingSelect from './SettingSelect.vue'
import {prefs} from '@/lib/prefs'
import {t, type MessageKey} from '@/lib/i18n'
import {toast} from '@/lib/toast'

const props = defineProps<{endpoint: string, token: string}>()

interface Step {
	text: MessageKey
	value?: string
}

const MCP_HELP = 'https://modelcontextprotocol.io/docs/getting-started/intro'

const clients = computed(() => [
	{value: 'claudeCode', label: 'Claude Code', help: 'https://code.claude.com/docs/en/mcp'},
	{value: 'codex', label: 'Codex', help: 'https://learn.chatgpt.com/docs/extend/mcp?surface=cli'},
	{value: 'claudeDesktop', label: 'Claude Desktop / claude.ai', help: 'https://claude.com/docs/connectors/custom/remote-mcp'},
	{value: 'mistral', label: 'Mistral Vibe', help: 'https://docs.mistral.ai/vibe/work/connectors/mcp-connectors'},
	{value: 'chatgpt', label: 'ChatGPT', help: 'https://developers.openai.com/api/docs/guides/developer-mode'},
	{value: 'other', label: t('mcpClientOther'), help: MCP_HELP},
])

const options = computed(() => [{value: '', label: t('mcpChooseClient')}, ...clients.value])
const selected = computed(() => clients.value.find(c => c.value === prefs.mcpClient))

function shellQuote(value: string) {
	return `'${value.replaceAll('\'', '\'\\\'\'')}'`
}

const steps = computed<Step[]>(() => {
	const {endpoint, token} = props
	switch (prefs.mcpClient) {
	case 'claudeCode':
		return [
			{text: 'mcpClaudeCodeCommand', value: `claude mcp add --transport http opentick ${shellQuote(endpoint)} --header ${shellQuote(`Authorization: Bearer ${token}`)}`},
			{text: 'mcpClaudeCodeVerify'},
		]
	case 'codex':
		return [
			{text: 'mcpCodexEnv', value: `export OPENTICK_MCP_TOKEN=${shellQuote(token)}`},
			{text: 'mcpCodexCommand', value: `codex mcp add opentick --url ${shellQuote(endpoint)} --bearer-token-env-var OPENTICK_MCP_TOKEN`},
			{text: 'mcpCodexConfig', value: `[mcp_servers.opentick]\nurl = ${JSON.stringify(endpoint)}\nbearer_token_env_var = "OPENTICK_MCP_TOKEN"`},
			{text: 'mcpCodexNote'},
		]
	case 'claudeDesktop':
		return [
			{text: 'mcpClaudeDesktopOpen'},
			{text: 'mcpClaudeDesktopUrl', value: endpoint},
			{text: 'mcpClaudeDesktopAuth'},
			{text: 'mcpClaudeDesktopHeader', value: `Bearer ${token}`},
			{text: 'mcpClaudeDesktopBeta'},
		]
	case 'mistral':
		return [
			{text: 'mcpMistralOpen'},
			{text: 'mcpMistralUrl', value: endpoint},
			{text: 'mcpMistralToken', value: token},
		]
	case 'other':
		return [
			{text: 'mcpOtherUrl', value: endpoint},
			{text: 'mcpOtherHeader', value: `Authorization: Bearer ${token}`},
		]
	default:
		return []
	}
})

async function copy(value: string) {
	await navigator.clipboard.writeText(value)
	toast(t('copied'))
}
</script>

<template>
	<div class="card">
		<div class="row">
			<span>{{ t('mcpClient') }}</span>
			<SettingSelect v-model="prefs.mcpClient" :options="options" />
		</div>
	</div>

	<p v-if="prefs.mcpClient === 'chatgpt'" class="note">{{ t('mcpChatgptUnavailable') }}</p>
	<ol v-else-if="steps.length" class="steps">
		<li v-for="step in steps" :key="step.text">
			<p>{{ t(step.text) }}</p>
			<div v-if="step.value" class="copyable">
				<pre><code>{{ step.value }}</code></pre>
				<button class="icon-btn" :title="t('copy')" :aria-label="t('copy')" @click="copy(step.value)">
					<Copy :size="14" :stroke-width="1.8" />
				</button>
			</div>
		</li>
	</ol>
	<a v-if="selected" class="help" :href="selected.help" target="_blank" rel="noopener noreferrer">
		{{ t('mcpClientHelp', {client: selected.label}) }}
	</a>
</template>

<style scoped>
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

.note {
	margin: 0 15px 14px;
	font-size: 13px;
	color: var(--text-2);
}

.steps {
	margin: 0 0 14px;
	padding-left: 32px;
	font-size: 13px;
}

.steps li + li {
	margin-top: 12px;
}

.steps p {
	margin: 0;
}

.copyable {
	display: flex;
	align-items: flex-start;
	gap: 4px;
	margin-top: 6px;
	padding: 4px 4px 4px 10px;
	border-radius: 6px;
	background: var(--bg-input);
}

.copyable pre {
	flex: 1;
	min-width: 0;
	margin: 6px 0;
	font-size: 12px;
	white-space: pre-wrap;
	overflow-wrap: anywhere;
}

.copyable .icon-btn {
	flex-shrink: 0;
	width: 26px;
	height: 26px;
}

.help {
	margin-left: 15px;
	font-size: 13px;
	color: var(--accent);
}

.help:hover {
	text-decoration: underline;
}
</style>
