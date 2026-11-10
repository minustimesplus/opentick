<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {Bot, ExternalLink, RotateCw} from 'lucide-vue-next'
import type {AgentDispatch} from '@/api/types'
import {agentName, useAgents} from '@/stores/agents'
import {fmtDayMonth, fmtTime, parseDate} from '@/lib/dates'
import {openAgentSend} from '@/lib/ui'
import {t} from '@/lib/i18n'

/** The hand-offs of a task to AI agents, shown in the task detail. */
const props = defineProps<{taskId: number}>()
const agents = useAgents()
const items = ref<AgentDispatch[]>([])
const expanded = ref(false)

const latest = computed(() => agents.latest.get(props.taskId))
const shown = computed(() => expanded.value ? items.value : items.value.slice(0, 1))

async function load() {
	if (!agents.enabled) return
	try {
		items.value = await agents.history(props.taskId)
	} catch {
		items.value = []
	}
}

watch(() => props.taskId, load, {immediate: true})
// A send from the dialog or the list agent updates the latest dispatch.
watch(() => latest.value?.id, id => {
	if (id && !items.value.some(d => d.id === id)) load()
})

function when(d: AgentDispatch) {
	const date = parseDate(d.created)
	return date ? `${fmtDayMonth(date)} ${fmtTime(date)}` : ''
}
</script>

<template>
	<div v-if="agents.enabled && items.length" class="agent-panel">
		<div v-for="d in shown" :key="d.id" class="entry" :class="d.status">
			<Bot :size="14" class="ic" />
			<div class="text">
				<div class="line">
					<span>{{ d.status === 'failed' ? t('agentFailedTo', {agent: agentName(d.agent)}) : t('agentSentTo', {agent: agentName(d.agent)}) }}</span>
					<span class="muted">· {{ when(d) }}<template v-if="d.automatic"> · {{ t('agentAutomatic') }}</template></span>
				</div>
				<div v-if="d.status === 'failed' && d.error" class="error">{{ d.error }}</div>
				<div v-if="d.extra_prompt" class="extra">“{{ d.extra_prompt }}”</div>
			</div>
			<a v-if="d.session_url" class="icon-btn" :href="d.session_url" target="_blank" rel="noopener" :title="t('agentOpenSession')">
				<ExternalLink :size="14" />
			</a>
			<button class="icon-btn" :title="t('agentSendAgain')" @click="openAgentSend(taskId, d.agent)">
				<RotateCw :size="14" />
			</button>
		</div>
		<button v-if="items.length > 1" class="more" @click="expanded = !expanded">
			{{ expanded ? t('agentShowLess') : t('agentShowAll', {n: items.length}) }}
		</button>
	</div>
</template>

<style scoped>
.agent-panel {
	margin: 10px 0 4px;
	padding: 6px 8px;
	border-radius: 8px;
	background: var(--bg-hover);
}

.entry {
	display: flex;
	align-items: flex-start;
	gap: 8px;
	padding: 4px 0;
	font-size: 12.5px;
}

.ic {
	flex-shrink: 0;
	margin-top: 2px;
	color: #d97757;
}

.entry.failed .ic,
.error {
	color: var(--danger);
}

.text {
	flex: 1;
	min-width: 0;
}

.line {
	display: flex;
	flex-wrap: wrap;
	gap: 4px;
}

.muted {
	color: var(--text-3);
}

.error,
.extra {
	margin-top: 2px;
	font-size: 12px;
	overflow-wrap: anywhere;
}

.extra {
	color: var(--text-2);
}

.icon-btn {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	width: 24px;
	height: 24px;
	color: var(--text-2);
}

.more {
	font-size: 12px;
	color: var(--text-3);
	padding: 2px 0 2px 22px;
}
</style>
