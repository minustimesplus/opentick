<script setup lang="ts">
import {computed, ref} from 'vue'
import TaskCheckbox from '@/components/TaskCheckbox.vue'
import Popover from '@/components/Popover.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {
	addDays, dateKey, fmtTime, fmtWeekdayShort, getSchedule, isSameDay, minutesOfDay, moveSchedule, schedulePatch,
	startOfDay, startOfMonth, startOfWeek, withMinutes,
} from '@/lib/dates'
import {projectColor} from '@/lib/tasks'
import {openTaskDrawer, openTaskMenu} from '@/lib/ui'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'

const props = defineProps<{month: Date, tasks: Task[]}>()
const emit = defineEmits<{day: [Date]}>()
const data = useData()
const MAX = 4

const weeks = computed(() => {
	const first = startOfWeek(startOfMonth(props.month))
	return Array.from({length: 6}, (_, w) => Array.from({length: 7}, (_, d) => addDays(first, w * 7 + d))
		.filter(d => prefs.calendarShowWeekends || (d.getDay() !== 0 && d.getDay() !== 6)))
})

const byDay = computed(() => {
	const map = new Map<string, Task[]>()
	for (const task of props.tasks) {
		const s = getSchedule(task)
		if (!s) continue
		const last = s.end && s.allDay ? s.end : s.date
		for (let d = startOfDay(s.start); d <= last; d = addDays(d, 1)) {
			const k = dateKey(d)
			if (!map.has(k)) map.set(k, [])
			map.get(k)!.push(task)
		}
	}
	for (const list of map.values()) {
		list.sort((a, b) => {
			const sa = getSchedule(a)!
			const sb = getSchedule(b)!
			return Number(!sa.allDay) - Number(!sb.allDay) || sa.start.getTime() - sb.start.getTime()
		})
	}
	return map
})

const today = startOfDay(new Date())

function color(task: Task) {
	return projectColor(data.project(task.project_id))
}

function chipTime(task: Task) {
	const s = getSchedule(task)
	return s && !s.allDay ? fmtTime(s.start) : ''
}

function onDragStart(e: DragEvent, task: Task) {
	e.dataTransfer?.setData('application/x-task-id', String(task.id))
}

const dropKey = ref<string | null>(null)

function onDrop(e: DragEvent, day: Date) {
	dropKey.value = null
	const id = Number(e.dataTransfer?.getData('application/x-task-id'))
	const task = id ? data.tasks.get(id) : undefined
	if (!task) return
	const s = getSchedule(task)
	data.updateTask(id, s && !s.allDay ? moveSchedule(s, withMinutes(day, minutesOfDay(s.start)), false) : schedulePatch(day, true))
}

const creating = ref<{day: Date, x: number, y: number} | null>(null)
const title = ref('')

function onCellClick(e: MouseEvent, day: Date) {
	if ((e.target as HTMLElement).closest('.chip, .more, .num')) return
	title.value = ''
	creating.value = {day, x: e.clientX, y: e.clientY}
}

async function submit() {
	const c = creating.value
	creating.value = null
	if (!c || !title.value.trim()) return
	await data.createTask({title: title.value.trim(), ...schedulePatch(c.day, true), project_id: prefs.defaultListId || data.inboxId})
}
</script>

<template>
	<div class="mg">
		<div class="mg-head" :style="{gridTemplateColumns: `repeat(${weeks[0].length}, 1fr)`}">
			<div v-for="d in weeks[0]" :key="d.getDay()" class="wd">{{ fmtWeekdayShort(d) }}</div>
		</div>
		<div class="mg-body" :style="{gridTemplateColumns: `repeat(${weeks[0].length}, 1fr)`}">
			<template v-for="week in weeks" :key="dateKey(week[0])">
				<div
					v-for="d in week"
					:key="dateKey(d)"
					class="cell"
					:class="{other: d.getMonth() !== month.getMonth(), drop: dropKey === dateKey(d)}"
					@click="onCellClick($event, d)"
					@dragover.prevent="dropKey = dateKey(d)"
					@dragleave="dropKey === dateKey(d) && (dropKey = null)"
					@drop.prevent="onDrop($event, d)"
				>
					<button class="num" :class="{today: isSameDay(d, today)}" @click="emit('day', d)">{{ d.getDate() }}</button>
					<div
						v-for="task in (byDay.get(dateKey(d)) ?? []).slice(0, MAX)"
						:key="task.id"
						class="chip"
						:class="{timed: !!chipTime(task), done: task.done}"
						:style="{'--c': color(task)}"
						draggable="true"
						@dragstart="onDragStart($event, task)"
						@click="openTaskDrawer(task.id)"
						@contextmenu="openTaskMenu(task.id, $event)"
					>
						<TaskCheckbox v-if="!chipTime(task)" :done="task.done" on-color :size="12" @toggle="data.toggleDone(task.id)" />
						<span v-else class="dot" />
						<span class="title">{{ task.title }}</span>
						<span v-if="chipTime(task)" class="time">{{ chipTime(task) }}</span>
					</div>
					<button v-if="(byDay.get(dateKey(d))?.length ?? 0) > MAX" class="more" @click="emit('day', d)">
						{{ t('more', {n: byDay.get(dateKey(d))!.length - MAX}) }}
					</button>
				</div>
			</template>
		</div>
		<Popover v-if="creating" :anchor="{x: creating.x, y: creating.y}" @close="creating = null">
			<form class="create" @submit.prevent="submit">
				<input v-model="title" class="input" :placeholder="t('titlePlaceholder')" v-focus>
			</form>
		</Popover>
	</div>
</template>

<style scoped>
.mg {
	display: flex;
	flex-direction: column;
	height: 100%;
}

.mg-head {
	display: grid;
	border-bottom: 1px solid var(--border);
}

.wd {
	padding: 8px 0;
	text-align: center;
	font-size: 13px;
	color: var(--text-3);
}

.mg-body {
	flex: 1;
	display: grid;
	grid-template-rows: repeat(6, minmax(0, 1fr));
	min-height: 0;
}

.cell {
	display: flex;
	flex-direction: column;
	gap: 2px;
	padding: 4px;
	border-right: 1px solid var(--grid-line);
	border-bottom: 1px solid var(--grid-line);
	overflow: hidden;
	min-height: 0;
}

.cell.other .num {
	color: var(--text-4);
}

.cell.drop {
	background: var(--accent-soft);
}

.num {
	align-self: flex-start;
	min-width: 24px;
	height: 24px;
	border-radius: 12px;
	font-size: 13px;
	font-weight: 600;
}

.num.today {
	background: var(--accent);
	color: #fff;
}

.chip {
	display: flex;
	align-items: center;
	gap: 5px;
	height: 20px;
	padding: 0 5px;
	border-radius: 4px;
	font-size: 12px;
	flex-shrink: 0;
	background: color-mix(in srgb, var(--c) var(--event-mix), var(--bg));
	color: var(--event-text);
	cursor: pointer;
	overflow: hidden;
}

.chip.timed {
	background: transparent;
	color: var(--text);
}

.chip.timed:hover {
	background: var(--bg-hover);
}

.chip.done {
	opacity: 0.45;
	text-decoration: line-through;
}

.dot {
	width: 7px;
	height: 7px;
	border-radius: 50%;
	background: var(--c);
	flex-shrink: 0;
}

.title {
	flex: 1;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.time {
	font-size: 11px;
	color: var(--text-3);
}

.more {
	font-size: 12px;
	color: var(--text-3);
	text-align: left;
	padding-left: 5px;
}

.create {
	width: 260px;
	padding: 10px;
	background: var(--bg-elev);
	border-radius: 10px;
	box-shadow: var(--shadow);
}
</style>
