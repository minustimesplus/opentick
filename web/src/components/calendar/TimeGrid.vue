<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import TaskCheckbox from '@/components/TaskCheckbox.vue'
import Popover from '@/components/Popover.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {
	addMinutes, dateKey, fmtHourLabel, fmtTime, fmtWeekdayShort, isSameDay, minutesOfDay, schedulePatch, startOfDay,
	withMinutes, type Schedule,
} from '@/lib/dates'
import {projectColor} from '@/lib/tasks'
import {openTaskDrawer, openTaskMenu} from '@/lib/ui'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {allDayFor, layoutDay, minOf, POINT_MINUTES, snap, totalHeight, yOf, type Scale, type TimedItem} from './layout'

const props = defineProps<{days: Date[], tasks: Task[]}>()
const data = useData()

const GUTTER = 72
const NIGHT_H = 44
const DAY_START = 6 * 60
const DAY_END = 21 * 60

const bodyEl = ref<HTMLElement | null>(null)
const innerEl = ref<HTMLElement | null>(null)
const hourH = ref(52)

const scale = computed<Scale>(() => ({
	hourH: hourH.value,
	collapsed: !prefs.calendarExpandNight,
	nightH: NIGHT_H,
	dayStartMin: DAY_START,
	dayEndMin: DAY_END,
}))
const height = computed(() => totalHeight(scale.value))

function fit() {
	const h = bodyEl.value?.clientHeight ?? 0
	if (!h) return
	hourH.value = scale.value.collapsed
		? Math.max(40, Math.floor((h - 2 * NIGHT_H) / ((DAY_END - DAY_START) / 60)))
		: 48
}

let ro: ResizeObserver | null = null
const now = ref(new Date())
let nowTimer: number | undefined

onMounted(() => {
	fit()
	ro = new ResizeObserver(fit)
	if (bodyEl.value) ro.observe(bodyEl.value)
	nowTimer = window.setInterval(() => { now.value = new Date() }, 60_000)
	nextTick(scrollToNow)
})

onBeforeUnmount(() => {
	ro?.disconnect()
	clearInterval(nowTimer)
	detach()
})

watch(() => prefs.calendarExpandNight, () => nextTick(() => {
	fit()
	scrollToNow()
}))

function scrollToNow() {
	if (!bodyEl.value) return
	const target = yOf(Math.max(0, minutesOfDay(new Date()) - 120), scale.value)
	bodyEl.value.scrollTop = scale.value.collapsed ? 0 : target
}

// ---- grid decorations ----
const lines = computed(() => {
	const sc = scale.value
	const out: {y: number, label: string, night?: boolean}[] = []
	if (sc.collapsed) {
		for (let h = DAY_START / 60 + 1; h < DAY_END / 60; h++) {
			out.push({y: yOf(h * 60, sc), label: h === 12 ? t('noon') : fmtHourLabel(h)})
		}
	} else {
		for (let h = 1; h < 24; h++) out.push({y: yOf(h * 60, sc), label: h === 12 ? t('noon') : fmtHourLabel(h)})
	}
	return out
})

const nightBlocks = computed(() => {
	const sc = scale.value
	if (!sc.collapsed) return []
	return [
		{y: 0, h: NIGHT_H, from: fmtHourLabel(0), to: fmtHourLabel(DAY_START / 60)},
		{y: yOf(DAY_END, sc), h: NIGHT_H, from: fmtHourLabel(DAY_END / 60), to: fmtHourLabel(0)},
	]
})

// ---- data ----
const columns = computed(() => props.days.map(day => ({
	day,
	key: dateKey(day),
	today: isSameDay(day, now.value),
	items: layoutDay(props.tasks, day),
	allDay: allDayFor(props.tasks, day),
})))

const allDayExpanded = ref(false)
const maxAllDay = computed(() => Math.max(0, ...columns.value.map(c => c.allDay.length)))
const ALLDAY_VISIBLE = 2

function color(task: Task) {
	return projectColor(data.project(task.project_id))
}

function itemStyle(it: TimedItem) {
	const sc = scale.value
	const top = yOf(it.startMin, sc)
	const bottom = yOf(it.endMin, sc)
	const gap = 2
	const width = 100 / it.cols
	return {
		top: `${top + 1}px`,
		height: `${Math.max(it.point ? 20 : 22, bottom - top - gap)}px`,
		left: `calc(${it.col * width}% + 2px)`,
		width: `calc(${width}% - 6px)`,
		'--c': color(it.task),
	}
}

function timeRange(s: Schedule) {
	return s.end ? `${fmtTime(s.start)}-${fmtTime(s.end)}` : fmtTime(s.start)
}

// ---- pointer interactions ----
interface DragState {
	kind: 'move' | 'resize' | 'create' | 'move-allday'
	task?: Task
	x0: number
	y0: number
	started: boolean
	grabMin: number
	dur: number | null
	dayIdx0: number
	min0: number
}

interface Preview {
	dayIdx: number
	startMin: number
	endMin: number
	allDay: boolean
	task?: Task
}

let drag: DragState | null = null
let justDragged = false
const preview = ref<Preview | null>(null)
const dragging = ref(false)

function pos(e: PointerEvent | DragEvent) {
	const inner = innerEl.value!.getBoundingClientRect()
	const body = bodyEl.value!.getBoundingClientRect()
	const colW = (inner.width - GUTTER) / props.days.length
	const dayIdx = Math.max(0, Math.min(props.days.length - 1, Math.floor((e.clientX - inner.left - GUTTER) / colW)))
	const min = Math.max(0, Math.min(1440, minOf(e.clientY - inner.top, scale.value)))
	return {dayIdx, min, inAllDay: e.clientY < body.top}
}

function attach() {
	window.addEventListener('pointermove', onMove)
	window.addEventListener('pointerup', onUp)
	window.addEventListener('keydown', onEsc)
}

function detach() {
	window.removeEventListener('pointermove', onMove)
	window.removeEventListener('pointerup', onUp)
	window.removeEventListener('keydown', onEsc)
}

function onEsc(e: KeyboardEvent) {
	if (e.key === 'Escape') {
		drag = null
		preview.value = null
		dragging.value = false
		detach()
	}
}

function startOnItem(e: PointerEvent, it: TimedItem, dayIdx: number, kind: 'move' | 'resize') {
	if (e.button !== 0) return
	e.preventDefault()
	const p = pos(e)
	drag = {
		kind,
		task: it.task,
		x0: e.clientX,
		y0: e.clientY,
		started: false,
		grabMin: p.min - it.startMin,
		dur: it.point ? null : it.endMin - it.startMin,
		dayIdx0: dayIdx,
		min0: it.startMin,
	}
	attach()
}

function startOnAllDay(e: PointerEvent, task: Task, dayIdx: number) {
	if (e.button !== 0) return
	e.preventDefault()
	drag = {kind: 'move-allday', task, x0: e.clientX, y0: e.clientY, started: false, grabMin: 0, dur: null, dayIdx0: dayIdx, min0: 0}
	attach()
}

function startCreate(e: PointerEvent) {
	if (e.button !== 0 || (e.target as HTMLElement).closest('.ev')) return
	e.preventDefault()
	const p = pos(e)
	drag = {kind: 'create', x0: e.clientX, y0: e.clientY, started: false, grabMin: 0, dur: null, dayIdx0: p.dayIdx, min0: p.min}
	attach()
}

function onMove(e: PointerEvent) {
	if (!drag) return
	if (!drag.started) {
		if (Math.hypot(e.clientX - drag.x0, e.clientY - drag.y0) < 5) return
		drag.started = true
		dragging.value = true
	}
	const p = pos(e)
	if (drag.kind === 'create') {
		const a = snap(Math.min(drag.min0, p.min))
		const b = snap(Math.max(drag.min0, p.min))
		preview.value = {dayIdx: drag.dayIdx0, startMin: a, endMin: Math.max(a + 15, b), allDay: false}
	} else if (drag.kind === 'resize') {
		const end = Math.max(drag.min0 + 15, snap(p.min))
		preview.value = {dayIdx: drag.dayIdx0, startMin: drag.min0, endMin: end, allDay: false, task: drag.task}
	} else {
		if (p.inAllDay) {
			preview.value = {dayIdx: p.dayIdx, startMin: 0, endMin: 0, allDay: true, task: drag.task}
			return
		}
		const len = drag.dur ?? POINT_MINUTES
		let start = snap(p.min - drag.grabMin)
		start = Math.min(start, 1440 - Math.min(len, 1440))
		preview.value = {dayIdx: p.dayIdx, startMin: start, endMin: start + len, allDay: false, task: drag.task}
	}
}

function onUp(e: PointerEvent) {
	detach()
	const d = drag
	drag = null
	dragging.value = false
	const pv = preview.value
	preview.value = null
	if (!d) return
	if (d.started) {
		justDragged = true
		setTimeout(() => { justDragged = false }, 0)
	}

	if (!d.started) {
		if (d.task) {
			openTaskDrawer(d.task.id)
		} else if (d.kind === 'create') {
			const start = Math.floor(d.min0 / 30) * 30
			openCreate(e, {dayIdx: d.dayIdx0, startMin: start, endMin: start, allDay: false}, true)
		}
		return
	}
	if (!pv) return
	const day = props.days[pv.dayIdx]
	if (d.kind === 'create') {
		openCreate(e, pv, false)
		return
	}
	const task = d.task!
	if (pv.allDay) {
		data.updateTask(task.id, schedulePatch(day, true))
	} else if (d.kind === 'resize') {
		const start = withMinutes(day, pv.startMin)
		data.updateTask(task.id, schedulePatch(start, false, withMinutes(day, pv.endMin)))
	} else {
		const start = withMinutes(day, pv.startMin)
		const end = d.dur !== null ? addMinutes(start, d.dur) : null
		data.updateTask(task.id, schedulePatch(start, false, end))
	}
}

// ---- quick create ----
interface Creating {
	anchor: {x: number, y: number}
	day: Date
	start: Date | null
	end: Date | null
	allDay: boolean
}
const creating = ref<Creating | null>(null)
const newTitle = ref('')

function openCreate(e: PointerEvent | MouseEvent, pv: Preview, point: boolean) {
	const day = props.days[pv.dayIdx]
	creating.value = {
		anchor: {x: e.clientX, y: e.clientY},
		day,
		start: pv.allDay ? null : withMinutes(day, pv.startMin),
		end: point || pv.allDay ? null : withMinutes(day, pv.endMin),
		allDay: pv.allDay,
	}
	newTitle.value = ''
	preview.value = point ? null : pv
}

function onAllDayClick(e: MouseEvent, dayIdx: number) {
	if (justDragged || (e.target as HTMLElement).closest('.ad-chip, .ad-more')) return
	openCreate(e, {dayIdx, startMin: 0, endMin: 0, allDay: true}, true)
}

async function submitCreate() {
	const c = creating.value
	const title = newTitle.value.trim()
	creating.value = null
	preview.value = null
	if (!c || !title) return
	const patch = c.allDay ? schedulePatch(c.day, true) : schedulePatch(c.start, false, c.end)
	await data.createTask({title, ...patch, project_id: prefs.defaultListId || data.inboxId})
}

function closeCreate() {
	creating.value = null
	preview.value = null
}

const creatingLabel = computed(() => {
	const c = creating.value
	if (!c) return ''
	if (c.allDay) return `${fmtWeekdayShort(c.day)} ${c.day.getDate()} · ${t('allDay')}`
	return `${fmtWeekdayShort(c.day)} ${c.day.getDate()} · ${fmtTime(c.start!)}${c.end ? ' - ' + fmtTime(c.end) : ''}`
})

// ---- HTML5 drops (from the "arrange" panel or other views) ----
const dropAllDay = ref<number | null>(null)

function onDragOver(e: DragEvent) {
	if (!e.dataTransfer?.types.includes('application/x-task-id')) return
	e.preventDefault()
	const p = pos(e)
	const start = snap(p.min, 15)
	preview.value = {dayIdx: p.dayIdx, startMin: start, endMin: start + 60, allDay: false}
}

function onDrop(e: DragEvent) {
	const id = Number(e.dataTransfer?.getData('application/x-task-id'))
	const pv = preview.value
	preview.value = null
	if (!id || !pv) return
	e.preventDefault()
	const start = withMinutes(props.days[pv.dayIdx], pv.startMin)
	data.updateTask(id, schedulePatch(start, false, addMinutes(start, 60)))
}

function onAllDayDragOver(e: DragEvent, idx: number) {
	if (!e.dataTransfer?.types.includes('application/x-task-id')) return
	e.preventDefault()
	dropAllDay.value = idx
}

function onAllDayDrop(e: DragEvent, idx: number) {
	dropAllDay.value = null
	const id = Number(e.dataTransfer?.getData('application/x-task-id'))
	if (id) data.updateTask(id, schedulePatch(props.days[idx], true))
}

const nowY = computed(() => yOf(minutesOfDay(now.value), scale.value))
const gridCols = computed(() => ({gridTemplateColumns: `${GUTTER}px repeat(${props.days.length}, minmax(0, 1fr))`}))
const today0 = computed(() => startOfDay(now.value))
</script>

<template>
	<div class="tg" :class="{dragging}">
		<div class="tg-head" :style="gridCols">
			<div class="g-head" />
			<div
				v-for="(col, i) in columns"
				:key="col.key"
				class="col-head"
				:class="{drop: dropAllDay === i, 'drop-preview': preview?.allDay && preview.dayIdx === i}"
				@click="onAllDayClick($event, i)"
				@dragover="onAllDayDragOver($event, i)"
				@dragleave="dropAllDay = null"
				@drop="onAllDayDrop($event, i)"
			>
				<div class="wd">{{ fmtWeekdayShort(col.day) }}</div>
				<div class="dn"><span :class="{today: col.today, past: col.day < today0}">{{ col.day.getDate() }}</span></div>
				<div class="ad-list">
					<div
						v-for="task in (allDayExpanded ? col.allDay : col.allDay.slice(0, maxAllDay > ALLDAY_VISIBLE + 1 ? ALLDAY_VISIBLE : ALLDAY_VISIBLE + 1))"
						:key="task.id"
						class="ad-chip"
						:class="{done: task.done}"
						:style="{'--c': color(task)}"
						@pointerdown="startOnAllDay($event, task, i)"
						@contextmenu="openTaskMenu(task.id, $event)"
					>
						<TaskCheckbox :done="task.done" on-color :size="13" @toggle="data.toggleDone(task.id)" />
						<span class="ad-title">{{ task.title }}</span>
					</div>
					<button
						v-if="!allDayExpanded && maxAllDay > ALLDAY_VISIBLE + 1 && col.allDay.length > ALLDAY_VISIBLE"
						class="ad-more"
						@click.stop="allDayExpanded = true"
					>{{ t('more', {n: col.allDay.length - ALLDAY_VISIBLE}) }}</button>
					<button v-if="allDayExpanded && i === 0" class="ad-more" @click.stop="allDayExpanded = false">▲</button>
				</div>
			</div>
		</div>

		<div ref="bodyEl" class="tg-body" @dragover="onDragOver" @dragleave="preview = drag ? preview : null" @drop="onDrop">
			<div ref="innerEl" class="tg-inner" :style="{...gridCols, height: height + 'px'}" @pointerdown="startCreate">
				<div class="gutter">
					<div v-for="l in lines" :key="l.y" class="hour-label" :style="{top: l.y + 'px'}">{{ l.label }}</div>
					<button
						v-for="(n, i) in nightBlocks"
						:key="'n' + i"
						class="night-label"
						:style="{top: n.y + 'px', height: n.h + 'px'}"
						@pointerdown.stop
						@click="prefs.calendarExpandNight = true"
					>{{ n.from }}<br>- {{ n.to }}</button>
					<button
						v-if="!scale.collapsed"
						class="collapse-night"
						@pointerdown.stop
						@click="prefs.calendarExpandNight = false"
					>⌃</button>
				</div>

				<div class="lines">
					<div v-for="l in lines" :key="l.y" class="line" :style="{top: l.y + 'px'}" />
					<div v-for="(n, i) in nightBlocks" :key="'nb' + i" class="night" :style="{top: n.y + 'px', height: n.h + 'px'}" />
				</div>

				<div v-for="(col, i) in columns" :key="col.key" class="col" :class="{today: col.today}" :data-idx="i">
					<div
						v-for="it in col.items"
						:key="it.task.id"
						class="ev"
						:class="{point: it.point, done: it.task.done, ghost: preview?.task?.id === it.task.id, short: yOf(it.endMin, scale) - yOf(it.startMin, scale) < 40}"
						:style="itemStyle(it)"
						@pointerdown.stop="startOnItem($event, it, i, 'move')"
						@contextmenu="openTaskMenu(it.task.id, $event)"
					>
						<div class="ev-top">
							<TaskCheckbox :done="it.task.done" on-color :size="13" @toggle="data.toggleDone(it.task.id)" />
							<span class="ev-title">{{ it.task.title }}</span>
						</div>
						<div v-if="!it.point" class="ev-time">{{ timeRange(it.schedule) }}</div>
						<div class="resize" @pointerdown.stop="startOnItem($event, it, i, 'resize')" />
					</div>

					<div
						v-if="preview && !preview.allDay && preview.dayIdx === i"
						class="ev preview"
						:style="{
							top: yOf(preview.startMin, scale) + 1 + 'px',
							height: Math.max(20, yOf(preview.endMin, scale) - yOf(preview.startMin, scale) - 2) + 'px',
							left: '2px',
							width: 'calc(100% - 6px)',
							'--c': preview.task ? color(preview.task) : 'var(--accent)',
						}"
					>
						<div class="ev-top"><span class="ev-title">{{ preview.task?.title ?? t('newTask') }}</span></div>
						<div class="ev-time">{{ fmtTime(withMinutes(props.days[i], preview.startMin)) }}-{{ fmtTime(withMinutes(props.days[i], preview.endMin)) }}</div>
					</div>

					<div v-if="col.today" class="now" :style="{top: nowY + 'px'}" />
				</div>
			</div>
		</div>

		<Popover v-if="creating" :anchor="creating.anchor" @close="closeCreate">
			<form class="create" @submit.prevent="submitCreate">
				<input v-model="newTitle" class="input" :placeholder="t('titlePlaceholder')" v-focus>
				<div class="create-meta">{{ creatingLabel }}</div>
			</form>
		</Popover>
	</div>
</template>

<style scoped>
.tg {
	display: flex;
	flex-direction: column;
	height: 100%;
	min-height: 0;
	user-select: none;
}

.tg.dragging {
	cursor: grabbing;
}

.tg-head,
.tg-body {
	scrollbar-gutter: stable;
}

.tg-head {
	display: grid;
	overflow-y: hidden;
	border-bottom: 1px solid var(--border);
	flex-shrink: 0;
}

.col-head {
	padding: 0 6px 6px;
	border-left: 1px solid var(--grid-line);
	min-width: 0;
	cursor: default;
}

.col-head.drop,
.col-head.drop-preview {
	background: var(--accent-soft);
}

.wd {
	text-align: center;
	font-size: 13px;
	color: var(--text-3);
	padding: 8px 0 14px;
}

.dn {
	height: 30px;
	display: flex;
	align-items: center;
	padding-left: 4px;
	font-size: 16px;
	font-weight: 600;
}

.dn span {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	min-width: 28px;
	height: 28px;
	border-radius: 14px;
}

.dn span.today {
	background: var(--accent);
	color: #fff;
}

.ad-list {
	display: flex;
	flex-direction: column;
	gap: 2px;
	min-height: 22px;
	padding-bottom: 2px;
}

.ad-chip,
.ev {
	--bgc: color-mix(in srgb, var(--c) var(--event-mix), var(--bg));
	background: var(--bgc);
	color: var(--event-text);
}

.ad-chip {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 22px;
	padding: 0 6px;
	border-radius: 4px;
	font-size: 13px;
	cursor: grab;
	overflow: hidden;
}

.ad-title,
.ev-title {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.ad-chip.done,
.ev.done {
	opacity: 0.45;
}

.ad-chip.done .ad-title,
.ev.done .ev-title {
	text-decoration: line-through;
}

.ad-more {
	font-size: 12px;
	color: var(--text-3);
	text-align: left;
	padding-left: 4px;
}

.tg-body {
	flex: 1;
	overflow-y: auto;
	overflow-x: hidden;
	position: relative;
}

.tg-inner {
	display: grid;
	position: relative;
}

.gutter {
	position: relative;
	grid-column: 1;
	grid-row: 1;
}

.hour-label {
	position: absolute;
	right: 10px;
	transform: translateY(-50%);
	font-size: 12px;
	color: var(--text-3);
}

.night-label {
	position: absolute;
	left: 0;
	right: 0;
	display: flex;
	flex-direction: column;
	align-items: flex-end;
	justify-content: center;
	padding-right: 10px;
	font-size: 12px;
	line-height: 1.5;
	color: var(--text-3);
	background: var(--bg-panel);
	border-bottom: 1px solid var(--grid-line);
	border-top: 1px solid var(--grid-line);
}

.collapse-night {
	position: absolute;
	top: 4px;
	left: 8px;
	color: var(--text-3);
}

.lines {
	position: absolute;
	inset: 0;
	left: 72px;
	pointer-events: none;
}

.line {
	position: absolute;
	left: 0;
	right: 0;
	border-top: 1px solid var(--grid-line);
}

.night {
	position: absolute;
	left: 0;
	right: 0;
	background: color-mix(in srgb, var(--bg-panel) 60%, transparent);
	border-top: 1px solid var(--grid-line);
	border-bottom: 1px solid var(--grid-line);
}

.col {
	position: relative;
	grid-row: 1;
	border-left: 1px solid var(--grid-line);
	min-width: 0;
}

.ev {
	position: absolute;
	padding: 4px 6px;
	border-radius: 5px;
	font-size: 13px;
	overflow: hidden;
	cursor: grab;
	z-index: 1;
}

.ev:hover {
	z-index: 3;
	filter: brightness(1.08);
}

.ev.point,
.ev.short {
	padding: 2px 6px;
}

.ev.ghost {
	opacity: 0.35;
}

.ev.preview {
	z-index: 4;
	opacity: 0.9;
	box-shadow: 0 2px 10px rgba(0, 0, 0, 0.35);
	pointer-events: none;
}

.ev-top {
	display: flex;
	align-items: center;
	gap: 6px;
	font-weight: 500;
}

.ev-time {
	margin-top: 2px;
	padding-left: 19px;
	font-size: 11px;
	opacity: 0.75;
}

.short .ev-time {
	display: none;
}

.resize {
	position: absolute;
	left: 0;
	right: 0;
	bottom: 0;
	height: 6px;
	cursor: ns-resize;
}

.now {
	position: absolute;
	left: -1px;
	right: 0;
	height: 2px;
	background: var(--now);
	z-index: 5;
	pointer-events: none;
}

.now::before {
	content: "";
	position: absolute;
	left: -5px;
	top: -4px;
	width: 10px;
	height: 10px;
	border-radius: 50%;
	background: var(--now);
}

.create {
	width: 280px;
	padding: 10px;
	background: var(--bg-elev);
	border-radius: 10px;
	box-shadow: var(--shadow);
}

.create-meta {
	margin-top: 8px;
	font-size: 12px;
	color: var(--accent);
}
</style>
