<script setup lang="ts">
import {computed} from 'vue'
import {Bot} from 'lucide-vue-next'
import {agentName, useAgents} from '@/stores/agents'
import {fmtDayMonth, fmtTime, parseDate} from '@/lib/dates'
import {t} from '@/lib/i18n'

/** Shows that a task was handed to an AI agent, and whether that worked. */
const props = defineProps<{taskId: number}>()
const agents = useAgents()

const dispatch = computed(() => agents.enabled ? agents.latest.get(props.taskId) : undefined)
const title = computed(() => {
	const d = dispatch.value
	if (!d) return ''
	const when = parseDate(d.created)
	const at = when ? `${fmtDayMonth(when)} ${fmtTime(when)}` : ''
	return d.status === 'failed'
		? t('agentBadgeFailed', {agent: agentName(d.agent), error: d.error})
		: t('agentBadgeSent', {agent: agentName(d.agent), date: at})
})
</script>

<template>
	<span v-if="dispatch" class="agent-badge" :class="[dispatch.agent, dispatch.status]" :title="title">
		<Bot :size="12" :stroke-width="2" />{{ agentName(dispatch.agent) }}
	</span>
</template>

<style scoped>
.agent-badge {
	display: inline-flex;
	align-items: center;
	gap: 3px;
	padding: 1px 7px 1px 5px;
	border-radius: 10px;
	font-size: 11.5px;
	white-space: nowrap;
	--ac: #d97757;
	color: color-mix(in srgb, var(--ac) 75%, var(--text));
	background: color-mix(in srgb, var(--ac) 18%, transparent);
}

.agent-badge.instinct {
	--ac: #8b5cf6;
}

.agent-badge.failed {
	--ac: var(--danger);
}
</style>
