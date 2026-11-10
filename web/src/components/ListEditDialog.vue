<script setup lang="ts">
import {computed, ref} from 'vue'
import Modal from './Modal.vue'
import type {AgentKind, Project} from '@/api/types'
import {useData} from '@/stores/data'
import {AGENTS, agentName, useAgents} from '@/stores/agents'
import {splitEmoji, SWATCHES} from '@/lib/tasks'
import {t} from '@/lib/i18n'

const props = defineProps<{project?: Project | null}>()
const emit = defineEmits<{close: [], saved: [number]}>()
const data = useData()
const agents = useAgents()

const split = splitEmoji(props.project?.title ?? '')
const emoji = ref(props.project ? (split.emoji ?? '') : '')
const name = ref(props.project ? split.name : '')
const color = ref((props.project?.hex_color ?? '').replace('#', ''))
const parent = ref<number>(props.project?.parent_project_id ?? 0)
const aiSettings = props.project ? agents.listSettings(props.project.id) : null
const aiContext = ref(aiSettings?.ai_context ?? '')
const agent = ref<AgentKind | ''>(aiSettings?.agent ?? '')

const parents = computed(() => data.listProjects.filter(p => p.id !== props.project?.id && !p.parent_project_id))
const EMOJIS = [...new Set(['', split.emoji ?? '', '📚', '✍️', '💻', '🏃', '🛒', '📅', '💼', '🏠', '💡', '🎯', '📒', '✈️', '❤️', '🎓', '🧪'])]

async function save() {
	const title = (emoji.value ? emoji.value + ' ' : '') + name.value.trim()
	if (!name.value.trim()) return
	const body = {title, hex_color: color.value, parent_project_id: parent.value}
	let id: number | undefined
	if (props.project) {
		await data.updateProject(props.project.id, body)
		id = props.project.id
	} else {
		id = (await data.createProject(body))?.id
	}
	if (id && agents.enabled) await agents.saveListSettings(id, {ai_context: aiContext.value, agent: agent.value})
	if (id) emit('saved', id)
	emit('close')
}
</script>

<template>
	<Modal @close="emit('close')">
		<form class="form" @submit.prevent="save">
			<h3>{{ project ? t('editList') : t('newList') }}</h3>
			<div class="name-row">
				<select v-model="emoji" class="emoji-sel">
					<option v-for="e in EMOJIS" :key="e" :value="e">{{ e || '☰' }}</option>
				</select>
				<input v-model="name" class="input" :placeholder="t('name')" v-focus>
			</div>
			<label class="lbl">{{ t('color') }}</label>
			<div class="swatches">
				<button
					v-for="s in SWATCHES"
					:key="s"
					type="button"
					class="sw"
					:class="{on: color.toLowerCase() === s.toLowerCase(), none: !s}"
					:style="s ? {background: '#' + s} : {}"
					@click="color = s"
				/>
			</div>
			<label class="lbl">{{ t('folder') }}</label>
			<select v-model="parent" class="input">
				<option :value="0">{{ t('noParent') }}</option>
				<option v-for="p in parents" :key="p.id" :value="p.id">{{ p.title }}</option>
			</select>
			<template v-if="agents.enabled">
				<label class="lbl" for="list-ai-context">{{ t('aiContext') }}</label>
				<textarea
					id="list-ai-context"
					v-model="aiContext"
					class="input ai-context"
					rows="3"
					maxlength="10000"
					:placeholder="t('aiContextPlaceholder')"
				/>
				<label class="lbl" for="list-agent">{{ t('agentList') }}</label>
				<select id="list-agent" v-model="agent" class="input">
					<option value="">{{ t('agentListNone') }}</option>
					<option v-for="a in AGENTS" :key="a" :value="a">{{ t('agentListSendTo', {agent: agentName(a)}) }}</option>
				</select>
				<p v-if="agent && !agents.ready(agent)" class="hint warn">{{ t('agentNotSetUp', {agent: agentName(agent)}) }}</p>
				<p v-else-if="agent" class="hint">{{ t('agentListHint', {agent: agentName(agent)}) }}</p>
			</template>
			<div class="actions">
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button type="submit" class="btn primary">{{ t('save') }}</button>
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
	margin: 0 0 6px;
	font-size: 17px;
}

.name-row {
	display: flex;
	gap: 8px;
}

.emoji-sel {
	width: 56px;
	height: 34px;
	border-radius: 6px;
	border: 1px solid var(--border-strong);
	background: var(--bg-input);
	font-size: 16px;
	text-align: center;
}

.lbl {
	font-size: 12px;
	color: var(--text-3);
	margin-top: 4px;
}

.swatches {
	display: flex;
	flex-wrap: wrap;
	gap: 8px;
}

.sw {
	width: 22px;
	height: 22px;
	border-radius: 50%;
	outline-offset: 2px;
}

.sw.none {
	border: 1.5px dashed var(--text-3);
}

.sw.on {
	outline: 2px solid var(--text);
}

.ai-context {
	height: auto;
	min-height: 64px;
	padding: 8px 10px;
	resize: vertical;
	font: inherit;
	font-size: 13px;
	line-height: 1.45;
}

.hint {
	margin: 0;
	font-size: 12px;
	line-height: 1.4;
	color: var(--text-3);
}

.hint.warn {
	color: var(--danger);
}

.actions {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	margin-top: 10px;
}
</style>
