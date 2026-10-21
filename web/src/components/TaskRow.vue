<script setup lang="ts">
import {computed} from 'vue'
import {AlarmClock, AlignLeft, Repeat} from 'lucide-vue-next'
import TaskCheckbox from './TaskCheckbox.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {isOverdue, taskDateLabel} from '@/lib/dates'
import {labelColor, splitEmoji} from '@/lib/tasks'
import {openTaskMenu} from '@/lib/ui'
import {isMobile} from '@/lib/viewport'
import {t} from '@/lib/i18n'

const props = withDefaults(defineProps<{task: Task, selected?: boolean, showList?: boolean, compact?: boolean}>(), {showList: false})
const emit = defineEmits<{select: [number]}>()
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

function onDragStart(e: DragEvent) {
	e.dataTransfer?.setData('application/x-task-id', String(props.task.id))
	e.dataTransfer?.setData('text/plain', props.task.title)
	if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}
</script>

<template>
	<div
		class="row"
		:class="{selected, done: task.done, compact}"
		:draggable="!isMobile"
		@click="emit('select', task.id)"
		@contextmenu="openTaskMenu(task.id, $event)"
		@dragstart="onDragStart"
	>
		<TaskCheckbox :done="task.done" :priority="task.priority" @toggle="data.toggleDone(task.id)" />
		<div class="title-wrap">
			<span class="title">{{ task.title || '—' }}</span>
		</div>
		<div class="meta">
			<span v-for="l in task.labels ?? []" :key="l.id" class="tag" :style="{'--tc': labelColor(l.hex_color)}">{{ l.title }}</span>
			<span v-if="showList && listLabel" class="list">
				<span v-if="listLabel.emoji">{{ listLabel.emoji }}</span>{{ listLabel.name }}
			</span>
			<AlignLeft v-if="hasDescription" :size="13" class="ic" />
			<Repeat v-if="task.repeat_after || task.repeat_mode" :size="13" class="ic" />
			<AlarmClock v-if="task.reminders?.length" :size="13" class="ic" />
			<span v-if="dateLabel" class="date" :class="{overdue}">{{ dateLabel }}</span>
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
