<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {Bot, ChevronDown, ChevronRight, Send} from 'lucide-vue-next'
import Modal from './Modal.vue'
import type {AgentKind} from '@/api/types'
import {AGENTS, agentName, useAgents} from '@/stores/agents'
import {useData} from '@/stores/data'
import {openSettings, ui} from '@/lib/ui'
import {t} from '@/lib/i18n'

const props = defineProps<{taskId: number, agent: AgentKind}>()
const emit = defineEmits<{close: []}>()
const data = useData()
const agents = useAgents()

const task = computed(() => data.tasks.get(props.taskId))
const agent = ref<AgentKind>(props.agent)
const extra = ref('')
const sending = ref(false)
const showPreview = ref(false)
const preview = ref('')
const previewError = ref(false)
const ready = computed(() => agents.ready(agent.value))
const listContext = computed(() => task.value ? agents.listSettings(task.value.project_id).ai_context : '')

let timer: number | undefined
async function loadPreview() {
	if (!showPreview.value) return
	try {
		preview.value = await agents.preview(props.taskId, extra.value)
		previewError.value = false
	} catch {
		previewError.value = true
	}
}
watch(showPreview, loadPreview)
watch(extra, () => {
	clearTimeout(timer)
	timer = window.setTimeout(loadPreview, 400)
})

function setUp() {
	emit('close')
	ui.ctx = null
	openSettings('aiFeatures')
}

async function send() {
	if (!ready.value || sending.value) return
	sending.value = true
	const d = await agents.send(props.taskId, agent.value, extra.value.trim())
	sending.value = false
	if (d) emit('close')
}
</script>

<template>
	<Modal :width="480" @close="emit('close')">
		<form class="form" @submit.prevent="send">
			<h3><Bot :size="18" /> {{ t('agentSendTitle') }}</h3>
			<p class="task-title">{{ task?.title }}</p>

			<div class="seg" role="radiogroup">
				<button
					v-for="a in AGENTS"
					:key="a"
					type="button"
					role="radio"
					:aria-checked="agent === a"
					:class="{on: agent === a}"
					@click="agent = a"
				>{{ agentName(a) }}</button>
			</div>

			<p v-if="!ready" class="note warn">
				{{ agent === 'instinct' && agents.config?.instinct_email && !agents.config?.mailer_enabled ? t('agentNoMailer') : t('agentNotSetUp', {agent: agentName(agent)}) }}
				<button type="button" class="link" @click="setUp">{{ t('agentSetUp') }}</button>
			</p>

			<label class="lbl" for="agent-extra">{{ t('agentExtra') }}</label>
			<textarea
				id="agent-extra"
				v-model="extra"
				class="input extra"
				rows="4"
				:placeholder="t('agentExtraPlaceholder')"
				v-focus
				@keydown.meta.enter.prevent="send"
				@keydown.ctrl.enter.prevent="send"
			/>
			<p v-if="listContext" class="note">{{ t('agentListContextIncluded') }}</p>

			<button type="button" class="toggle" @click="showPreview = !showPreview">
				<component :is="showPreview ? ChevronDown : ChevronRight" :size="14" /> {{ t('agentPreview') }}
			</button>
			<pre v-if="showPreview" class="preview">{{ previewError ? t('agentPreviewError') : preview || '…' }}</pre>

			<div class="actions">
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button type="submit" class="btn primary" :disabled="!ready || sending">
					<Send :size="14" /> {{ t('agentSendTo', {agent: agentName(agent)}) }}
				</button>
			</div>
		</form>
	</Modal>
</template>

<style scoped>
.form {
	display: flex;
	flex-direction: column;
	gap: 10px;
}

h3 {
	display: flex;
	align-items: center;
	gap: 8px;
	margin: 0;
	font-size: 17px;
}

.task-title {
	margin: -4px 0 4px;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	color: var(--text-2);
	font-size: 13px;
}

.seg {
	display: flex;
	gap: 4px;
	padding: 3px;
	border-radius: 8px;
	background: var(--bg-hover);
}

.seg button {
	flex: 1;
	height: 28px;
	border-radius: 6px;
	font-size: 13px;
	color: var(--text-2);
}

.seg button.on {
	background: var(--bg-elev);
	color: var(--text);
	font-weight: 600;
	box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.lbl {
	font-size: 12px;
	color: var(--text-3);
	margin-top: 4px;
}

.extra {
	height: auto;
	min-height: 84px;
	padding: 8px 10px;
	resize: vertical;
	font: inherit;
	font-size: 13px;
	line-height: 1.45;
}

.note {
	margin: 0;
	font-size: 12px;
	line-height: 1.45;
	color: var(--text-3);
}

.note.warn {
	color: var(--text-2);
}

.link {
	color: var(--accent);
	font-size: 12px;
}

.toggle {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	align-self: flex-start;
	font-size: 12px;
	color: var(--text-2);
}

.preview {
	max-height: 220px;
	margin: 0;
	padding: 10px;
	overflow: auto;
	border-radius: 6px;
	background: var(--bg-input);
	border: 1px solid var(--border);
	font-size: 11.5px;
	line-height: 1.45;
	white-space: pre-wrap;
	word-break: break-word;
}

.actions {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	margin-top: 6px;
}

.actions .btn.primary {
	display: inline-flex;
	align-items: center;
	gap: 6px;
}

.btn:disabled {
	opacity: 0.5;
	cursor: default;
}
</style>
