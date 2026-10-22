<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue'
import {Plus} from 'lucide-vue-next'
import TaskCheckbox from './TaskCheckbox.vue'
import {useData} from '@/stores/data'
import {isOverdue, taskDateLabel} from '@/lib/dates'
import {openTaskMenu} from '@/lib/ui'
import {t} from '@/lib/i18n'

const props = defineProps<{parentId: number}>()
const emit = defineEmits<{open: [number]}>()
const data = useData()

const subtasks = computed(() => data.subtasksOf(props.parentId))
const doneCount = computed(() => subtasks.value.filter(s => s.done).length)

// Snapshots of subtasks that are not loaded become real tasks so they can be toggled.
watch(subtasks, list => data.adoptTasks(list), {immediate: true})

const adding = ref(false)
const draft = ref('')
const inputEl = ref<HTMLInputElement | null>(null)

async function focusAdd() {
	adding.value = true
	await nextTick()
	inputEl.value?.focus()
}

async function add() {
	const title = draft.value.trim()
	if (!title) return
	draft.value = ''
	await data.createSubtask(props.parentId, {title})
}

function onBlur() {
	if (!draft.value.trim()) adding.value = false
}

defineExpose({focusAdd})
</script>

<template>
	<div v-if="subtasks.length || adding" class="subtasks">
		<div v-if="subtasks.length" class="sub-head">
			<span>{{ t('subtasks') }}</span>
			<span class="muted">{{ doneCount }}/{{ subtasks.length }}</span>
		</div>
		<div
			v-for="s in subtasks"
			:key="s.id"
			class="sub-row"
			:class="{done: s.done}"
			@click="emit('open', s.id)"
			@contextmenu="openTaskMenu(s.id, $event)"
		>
			<TaskCheckbox :done="s.done" :wont-do="s.wont_do" :priority="s.priority" :size="15" @toggle="data.toggleDone(s.id)" />
			<span class="sub-title">{{ s.title }}</span>
			<span v-if="taskDateLabel(s)" class="sub-date" :class="{overdue: isOverdue(s)}">{{ taskDateLabel(s) }}</span>
		</div>
		<div class="sub-row add" @click="focusAdd">
			<Plus :size="15" class="plus" />
			<input
				v-if="adding"
				ref="inputEl"
				v-model="draft"
				class="add-input"
				:placeholder="t('addSubtask')"
				@keydown.enter.prevent="add"
				@keydown.esc.stop="draft = ''; ($event.target as HTMLInputElement).blur()"
				@blur="onBlur"
			>
			<span v-else class="muted">{{ t('addSubtask') }}</span>
		</div>
	</div>
</template>

<style scoped>
.subtasks {
	margin-top: 14px;
}

.sub-head {
	display: flex;
	justify-content: space-between;
	padding: 0 4px 4px;
	font-size: 12px;
	font-weight: 600;
	color: var(--text-2);
}

.sub-row {
	display: flex;
	align-items: center;
	gap: 10px;
	min-height: 34px;
	padding: 0 6px;
	border-radius: 6px;
	cursor: default;
}

.sub-row:hover {
	background: var(--bg-hover);
}

.sub-title {
	flex: 1;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 14px;
}

.done .sub-title {
	color: var(--text-3);
	text-decoration: line-through;
}

.sub-date {
	font-size: 12px;
	color: var(--accent);
}

.sub-date.overdue {
	color: var(--danger);
}

.add .plus {
	color: var(--text-3);
	margin: 0 1px;
}

.add-input {
	flex: 1;
	border: 0;
	outline: 0;
	background: transparent;
	font-size: 14px;
}
</style>
