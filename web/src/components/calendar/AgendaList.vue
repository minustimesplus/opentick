<script setup lang="ts">
import {computed} from 'vue'
import type {Task} from '@/api/types'
import {AGENDA_DAYS, type CalEvent} from '@/stores/calendars'
import {useData} from '@/stores/data'
import TaskCheckbox from '@/components/TaskCheckbox.vue'
import {addDays, dayDiff, fmtLongDate, fmtTime, fmtWeekdayLong, getSchedule, startOfDay} from '@/lib/dates'
import {t} from '@/lib/i18n'
import {openTaskMenu} from '@/lib/ui'

/** The agenda: the coming days' tasks and events as a list, skipping empty days. */
const props = defineProps<{start: Date, tasks: Task[], events: CalEvent[]}>()
const emit = defineEmits<{openTask: [number], openEvent: [CalEvent, MouseEvent]}>()
const data = useData()

type Item =
	| {kind: 'task', key: string, task: Task, time: string, sort: number}
	| {kind: 'event', key: string, event: CalEvent, time: string, sort: number}

const days = computed(() => {
	const first = startOfDay(props.start)
	const out: {day: Date, items: Item[]}[] = Array.from({length: AGENDA_DAYS}, (_, i) => ({day: addDays(first, i), items: []}))
	for (const task of props.tasks) {
		const s = getSchedule(task)
		if (!s) continue
		const i = dayDiff(s.date, first)
		if (i < 0 || i >= AGENDA_DAYS) continue
		out[i].items.push({kind: 'task', key: `t${task.id}`, task, time: s.allDay ? '' : fmtTime(s.start), sort: s.allDay ? -1 : s.start.getTime()})
	}
	for (const event of props.events) {
		// Every day an event covers within the window.
		if (event.start < first && event.end <= first) continue
		const from = Math.max(0, dayDiff(event.start, first))
		for (let i = from; i < AGENDA_DAYS; i++) {
			const day = out[i].day
			if (event.start >= addDays(day, 1) || (event.end <= day && i > from)) break
			const startsToday = i === dayDiff(event.start, first)
			out[i].items.push({
				kind: 'event', key: `e${event.key}:${i}`, event,
				time: event.allDay || !startsToday ? '' : fmtTime(event.start),
				sort: event.allDay || !startsToday ? -1 : event.start.getTime(),
			})
		}
	}
	for (const d of out) d.items.sort((a, b) => a.sort - b.sort)
	return out.filter(d => d.items.length)
})

const today = startOfDay(new Date())

function dayTitle(d: Date): string {
	const diff = dayDiff(d, today)
	const name = diff === 0 ? t('today') : diff === 1 ? t('tomorrow') : fmtWeekdayLong(d)
	return `${name} · ${fmtLongDate(d)}`
}
</script>

<template>
	<div class="agenda">
		<section v-for="d in days" :key="d.day.getTime()" class="day">
			<h3 :class="{today: dayDiff(d.day, today) === 0}">{{ dayTitle(d.day) }}</h3>
			<template v-for="item in d.items" :key="item.key">
				<div
					v-if="item.kind === 'task'"
					class="item"
					:data-task-id="item.task.id"
					@click="emit('openTask', item.task.id)"
					@contextmenu="openTaskMenu(item.task.id, $event)"
				>
					<TaskCheckbox :done="item.task.done" :wont-do="item.task.wont_do" :priority="item.task.priority" @toggle="data.toggleDone(item.task.id)" />
					<span class="time">{{ item.time || t('allDay') }}</span>
					<span class="title">{{ item.task.title }}</span>
				</div>
				<div v-else class="item" @click="emit('openEvent', item.event, $event)">
					<span class="swatch" :style="{background: item.event.color}" />
					<span class="time">{{ item.time || t('allDay') }}</span>
					<span class="title" :class="{archived: item.event.archived}">{{ item.event.title }}</span>
				</div>
			</template>
		</section>
		<div v-if="!days.length" class="empty-state">{{ t('agendaEmpty') }}</div>
	</div>
</template>

<style scoped>
.agenda {
	height: 100%;
	padding: 8px 24px 24px;
	overflow-y: auto;
}

.day {
	margin-bottom: 14px;
}

h3 {
	margin: 10px 0 4px;
	font-size: 13px;
	font-weight: 600;
	color: var(--text-2);
}

h3.today {
	color: var(--accent);
}

.item {
	display: flex;
	align-items: center;
	gap: 12px;
	min-height: 38px;
	padding: 0 10px;
	border-radius: 8px;
	cursor: pointer;
}

.item:hover {
	background: var(--bg-hover);
}

.swatch {
	width: 18px;
	height: 18px;
	flex-shrink: 0;
	border-radius: 5px;
}

.time {
	width: 64px;
	flex-shrink: 0;
	font-size: 12px;
	color: var(--text-3);
}

.title {
	overflow: hidden;
	white-space: nowrap;
	text-overflow: ellipsis;
}

.title.archived {
	opacity: 0.5;
	text-decoration: line-through;
}
</style>
