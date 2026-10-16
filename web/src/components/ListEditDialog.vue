<script setup lang="ts">
import {computed, ref} from 'vue'
import Modal from './Modal.vue'
import type {Project} from '@/api/types'
import {useData} from '@/stores/data'
import {splitEmoji, SWATCHES} from '@/lib/tasks'
import {t} from '@/lib/i18n'

const props = defineProps<{project?: Project | null}>()
const emit = defineEmits<{close: [], saved: [number]}>()
const data = useData()

const split = splitEmoji(props.project?.title ?? '')
const emoji = ref(props.project ? (split.emoji ?? '') : '')
const name = ref(props.project ? split.name : '')
const color = ref((props.project?.hex_color ?? '').replace('#', ''))
const parent = ref<number>(props.project?.parent_project_id ?? 0)

const parents = computed(() => data.listProjects.filter(p => p.id !== props.project?.id && !p.parent_project_id))
const EMOJIS = [...new Set(['', split.emoji ?? '', '📚', '✍️', '💻', '🏃', '🛒', '📅', '💼', '🏠', '💡', '🎯', '📒', '✈️', '❤️', '🎓', '🧪'])]

async function save() {
	const title = (emoji.value ? emoji.value + ' ' : '') + name.value.trim()
	if (!name.value.trim()) return
	const body = {title, hex_color: color.value, parent_project_id: parent.value}
	if (props.project) {
		await data.updateProject(props.project.id, body)
		emit('saved', props.project.id)
	} else {
		const p = await data.createProject(body)
		if (p) emit('saved', p.id)
	}
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

.actions {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	margin-top: 10px;
}
</style>
