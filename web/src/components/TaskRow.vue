<script setup lang="ts">
import {computed, ref} from 'vue'
import {AlarmClock, AlignLeft, ChevronDown, ChevronRight, MessageSquare, Paperclip, Repeat} from 'lucide-vue-next'
import TaskCheckbox from './TaskCheckbox.vue'
import PersonAvatar from './PersonAvatar.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {isOverdue, taskDateLabel} from '@/lib/dates'
import {labelColor, splitEmoji} from '@/lib/tasks'
import {openTaskMenu, ui, type DropZone} from '@/lib/ui'
import {isMobile} from '@/lib/viewport'
import {t} from '@/lib/i18n'

const props = withDefaults(defineProps<{
	task: Task
	selected?: boolean
	showList?: boolean
	compact?: boolean
	/** Nesting level under parent tasks shown in the same list. */
	depth?: number
	/** Number of subtasks nested below this row; shows a collapse toggle. */
	childCount?: number
	expanded?: boolean
	/** Accept other task rows dropped before/after (reorder) or onto (nest) this one. */
	droppable?: boolean
	/** Whether dropping before/after reorders; nesting works regardless. */
	reorderable?: boolean
}>(), {showList: false, depth: 0, childCount: 0, expanded: true})
const emit = defineEmits<{select: [number], toggle: [number], drop: [dragged: number, target: number, zone: DropZone]}>()
const data = useData()

const project = computed(() => data.project(props.task.project_id))
const listLabel = computed(() => {
	if (!project.value) return null
	if (project.value.id === data.inboxId) return {emoji: null, name: t('inbox')}
	return splitEmoji(project.value.title)
})
const dateLabel = computed(() => taskDateLabel(props.task))
const overdue = computed(() => isOverdue(props.task))
const hasDescription = computed(() => !!props.task.description && props.task.description.replace(/<[^>]*>/g, '').trim() !== '')

const zone = ref<DropZone | null>(null)

function onDragOver(e: DragEvent) {
	const dragged = ui.dragTaskId
	if (!props.droppable || dragged === null || dragged === props.task.id) return
	const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
	const y = (e.clientY - rect.top) / rect.height
	const z: DropZone = !props.reorderable || (y > 0.3 && y < 0.7) ? 'inside' : y <= 0.3 ? 'before' : 'after'
	e.preventDefault()
	if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
	zone.value = z
}

function onDrop(e: DragEvent) {
	const z = zone.value
	zone.value = null
	const dragged = ui.dragTaskId
	if (!z || dragged === null) return
	e.preventDefault()
	e.stopPropagation()
	emit('drop', dragged, props.task.id, z)
}

function onDragStart(e: DragEvent) {
	ui.dragTaskId = props.task.id
	e.dataTransfer?.setData('application/x-task-id', String(props.task.id))
	e.dataTransfer?.setData('text/plain', props.task.title)
	if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}
</script>

<template>
	<div
		class="row"
		:class="{selected, done: task.done, 'wont-do': task.done && task.wont_do, compact}"
		:style="depth ? {paddingLeft: `${16 + depth * 28}px`} : undefined"
		:draggable="!isMobile"
		@click="emit('select', task.id)"
		@contextmenu="openTaskMenu(task.id, $event)"
		:data-drop="zone"
		@dragstart="onDragStart"
		@dragend="ui.dragTaskId = null"
		@dragover="onDragOver"
		@dragleave="zone = null"
		@drop="onDrop"
	>
		<button v-if="childCount" class="fold" :style="{left: `${depth * 28}px`}" @click.stop="emit('toggle', task.id)">
			<component :is="expanded ? ChevronDown : ChevronRight" :size="14" />
		</button>
		<TaskCheckbox :done="task.done" :wont-do="task.wont_do" :priority="task.priority" @toggle="data.toggleDone(task.id)" />
		<div class="title-wrap">
			<span class="title">{{ task.title || '—' }}</span>
		</div>
		<span v-if="childCount && !expanded" class="sub-count">{{ childCount }}</span>
		<div class="meta">
			<span v-for="l in task.labels ?? []" :key="l.id" class="tag" :style="{'--tc': labelColor(l.hex_color)}">{{ l.title }}</span>
			<span v-if="showList && listLabel" class="list">
				<span v-if="listLabel.emoji">{{ listLabel.emoji }}</span>{{ listLabel.name }}
			</span>
			<AlignLeft v-if="hasDescription" :size="13" class="ic" />
			<Paperclip v-if="task.attachments?.length" :size="13" class="ic" />
			<MessageSquare v-if="task.comment_count" :size="13" class="ic" />
			<Repeat v-if="task.repeat_after || task.repeat_mode" :size="13" class="ic" />
			<AlarmClock v-if="task.reminders?.length" :size="13" class="ic" />
			<span v-if="dateLabel" class="date" :class="{overdue}">{{ dateLabel }}</span>
			<PersonAvatar v-for="u in (task.assignees ?? []).slice(0, 2)" :key="u.id" :user="u" :size="20" />
		</div>
	</div>
</template>

<style scoped>
.row {
	display: flex;
	align-items: center;
	gap: 12px;
	min-height: 44px;
	padding: 0 12px 0 16px;
	border-radius: 6px;
	cursor: default;
	position: relative;
}

.row::after {
	content: "";
	position: absolute;
	left: 44px;
	right: 12px;
	bottom: 0;
	height: 1px;
	background: var(--border);
}

.fold {
	position: absolute;
	top: 0;
	bottom: 0;
	width: 16px;
	display: flex;
	align-items: center;
	justify-content: center;
	color: var(--text-3);
	opacity: 0;
}

.row:hover .fold,
.fold:focus-visible {
	opacity: 1;
}

.sub-count {
	font-size: 12px;
	color: var(--text-3);
}

.row[data-drop="inside"] {
	box-shadow: inset 0 0 0 2px var(--accent);
}

.row[data-drop="before"]::before,
.row[data-drop="after"]::before {
	content: "";
	position: absolute;
	left: 40px;
	right: 12px;
	height: 2px;
	background: var(--accent);
}

.row[data-drop="before"]::before {
	top: -1px;
}

.row[data-drop="after"]::before {
	bottom: -1px;
}

.row.compact {
	min-height: 40px;
}

@media (hover: hover) {
	.row:hover {
		background: var(--bg-hover);
	}
}

.row.selected {
	background: var(--bg-active);
}

.title-wrap {
	flex: 1;
	min-width: 0;
}

.title {
	display: block;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 15px;
}

.done .title {
	color: var(--text-3);
	text-decoration: line-through;
}

.wont-do .title {
	color: var(--text-4);
}

.meta {
	display: flex;
	align-items: center;
	gap: 8px;
	flex-shrink: 0;
	font-size: 13px;
	color: var(--text-3);
}

.tag {
	padding: 1px 8px;
	border-radius: 10px;
	font-size: 12px;
	color: color-mix(in srgb, var(--tc) 60%, var(--text));
	background: color-mix(in srgb, var(--tc) 30%, transparent);
}

.list {
	display: inline-flex;
	gap: 3px;
	max-width: 160px;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.ic {
	color: var(--text-3);
}

.date {
	color: var(--accent);
	white-space: nowrap;
}

.date.overdue {
	color: var(--danger);
}

.done .date {
	color: var(--text-3);
}

/* Phones: tags move out of the row so the title keeps its room. */
@media (max-width: 767px) {
	.row {
		min-height: 48px;
		padding: 0 8px 0 10px;
		-webkit-touch-callout: none;
		user-select: none;
	}

	.row::after {
		left: 38px;
		right: 8px;
	}

	.meta {
		flex-shrink: 1;
		min-width: 0;
		gap: 6px;
	}

	.tag {
		display: none;
	}

	.list {
		max-width: 88px;
	}
}
</style>
