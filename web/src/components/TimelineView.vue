<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import TaskCheckbox from './TaskCheckbox.vue'
import type {Project, Task} from '@/api/types'
import {useData} from '@/stores/data'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {addDays, dayDiff, fmtDayMonth, fmtMonthShort, fmtMonthYear, fmtWeekdayNarrow, schedulePatch, startOfDay} from '@/lib/dates'
import {projectColor} from '@/lib/tasks'
import {openTaskMenu, ui} from '@/lib/ui'
import {
	DAY_WIDTH, dragSpan, groups, spanPatch, taskSpan, timelineRange,
	type DragMode, type GroupUnit, type Span, type TimelineScale,
} from '@/lib/timeline'

const props = defineProps<{project: Project, selectedId: number | null, showCompleted: boolean}>()
const emit = defineEmits<{select: [number]}>()
const data = useData()

const ROW = 36

const scales: {key: TimelineScale, label: 'day' | 'week' | 'month'}[] = [
	{key: 'day', label: 'day'},
	{key: 'week', label: 'week'},
	{key: 'month', label: 'month'},
]
const dw = computed(() => DAY_WIDTH[prefs.timelineScale])
const today = startOfDay(new Date())

const tasks = computed(() => data.allTasks.filter(task => task.project_id === props.project.id && (props.showCompleted || !task.done)))

interface Bar {
	task: Task
	span: Span
}

const bars = computed<Bar[]>(() => tasks.value
	.flatMap(task => {
		const span = taskSpan(task)
		return span ? [{task, span}] : []
	})
	.sort((a, b) => a.span.start.getTime() - b.span.start.getTime() || a.span.end.getTime() - b.span.end.getTime() || a.task.id - b.task.id))

const undated = computed(() => tasks.value.filter(task => !taskSpan(task)).sort((a, b) => Number(a.done) - Number(b.done) || a.id - b.id))

const range = computed(() => timelineRange(bars.value.map(b => b.span), prefs.timelineScale, today))
const width = computed(() => range.value.days * dw.value)
const x = (d: Date) => dayDiff(d, range.value.start) * dw.value

const units = computed<[GroupUnit, GroupUnit]>(() => prefs.timelineScale === 'month' ? ['year', 'month'] : prefs.timelineScale === 'week' ? ['month', 'week'] : ['month', 'day'])
const topRow = computed(() => groups(range.value.start, range.value.days, units.value[0]))
const bottomRow = computed(() => groups(range.value.start, range.value.days, units.value[1]))

function topLabel(d: Date): string {
	return units.value[0] === 'year' ? String(d.getFullYear()) : fmtMonthYear(d)
}

function bottomLabel(d: Date): string {
	switch (units.value[1]) {
		case 'day': return String(d.getDate())
		case 'week': return fmtDayMonth(d, false)
		default: return fmtMonthShort(d)
	}
}

/** Weekend columns, shaded at the day scale only. */
const weekends = computed(() => prefs.timelineScale === 'day'
	? bottomRow.value.filter(g => g.date.getDay() === 0 || g.date.getDay() === 6)
	: [])

// ---- scrolling ----
const scroller = ref<HTMLElement | null>(null)

function scrollToToday() {
	const el = scroller.value
	if (el) el.scrollLeft = x(today) - el.clientWidth / 3
}

onMounted(scrollToToday)
watch(() => prefs.timelineScale, () => nextTick(scrollToToday))
// Keep the same days on screen when the timeline grows to the left.
watch(() => range.value.start, (now, before) => {
	if (scroller.value && before) scroller.value.scrollLeft += dayDiff(before, now) * dw.value
}, {flush: 'post'})

// ---- dragging bars ----
interface Drag {
	task: Task
	span: Span
	mode: DragMode
	x0: number
	days: number
	moved: boolean
	/** Over the undated list: dropping there clears the dates. */
	unschedule: boolean
}

const drag = ref<Drag | null>(null)

function shown(bar: Bar): Span {
	const d = drag.value
	return d && d.task.id === bar.task.id && !d.unschedule ? dragSpan(d.span, d.mode, d.days) : bar.span
}

function onBarDown(e: PointerEvent, bar: Bar, mode: DragMode) {
	if (e.button !== 0) return
	e.preventDefault()
	e.stopPropagation()
	drag.value = {task: bar.task, span: bar.span, mode, x0: e.clientX, days: 0, moved: false, unschedule: false}
	window.addEventListener('pointermove', onMove)
	window.addEventListener('pointerup', onUp)
	window.addEventListener('pointercancel', cancel)
	window.addEventListener('keydown', onEsc)
}

function detach() {
	window.removeEventListener('pointermove', onMove)
	window.removeEventListener('pointerup', onUp)
	window.removeEventListener('pointercancel', cancel)
	window.removeEventListener('keydown', onEsc)
}

function cancel() {
	detach()
	drag.value = null
}

function onEsc(e: KeyboardEvent) {
	if (e.key === 'Escape') cancel()
}

function onMove(e: PointerEvent) {
	const d = drag.value
	if (!d) return
	const dx = e.clientX - d.x0
	if (!d.moved && Math.abs(dx) < 4) return
	d.moved = true
	d.days = Math.round(dx / dw.value)
	d.unschedule = d.mode === 'move' && !!document.elementFromPoint(e.clientX, e.clientY)?.closest('.tl-undated')
}

function onUp() {
	const d = drag.value
	detach()
	drag.value = null
	if (!d) return
	if (!d.moved) {
		emit('select', d.task.id)
	} else if (d.unschedule) {
		data.updateTask(d.task.id, schedulePatch(null, true))
	} else if (d.days) {
		data.updateTask(d.task.id, spanPatch(d.task, dragSpan(d.span, d.mode, d.days)))
	}
}

onBeforeUnmount(detach)

// ---- scheduling undated tasks by dropping them on a day ----
const canvas = ref<HTMLElement | null>(null)
const dropDay = ref<number | null>(null)

function onUndatedDragStart(e: DragEvent, task: Task) {
	ui.dragTaskId = task.id
	e.dataTransfer?.setData('application/x-task-id', String(task.id))
	e.dataTransfer?.setData('text/plain', task.title)
	if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragEnd() {
	ui.dragTaskId = null
	dropDay.value = null
}

function onCanvasOver(e: DragEvent) {
	if (ui.dragTaskId === null || !canvas.value) return
	e.preventDefault()
	const left = canvas.value.getBoundingClientRect().left
	dropDay.value = Math.max(0, Math.min(range.value.days - 1, Math.floor((e.clientX - left) / dw.value)))
}

function onCanvasDrop(e: DragEvent) {
	const day = dropDay.value
	const id = Number(e.dataTransfer?.getData('application/x-task-id')) || ui.dragTaskId
	onDragEnd()
	if (day === null || !id || !data.tasks.has(id)) return
	e.preventDefault()
	data.updateTask(id, schedulePatch(addDays(range.value.start, day), true))
}

/** Bars narrower than this show their title beside them. */
const MIN_LABELLED = 72
/** Bars narrower than this have no room for a checkbox. */
const MIN_CHECKBOX = 24

function barWidth(span: Span): number {
	return (dayDiff(span.end, span.start) + 1) * dw.value - 2
}

function barColor(task: Task): string {
	return task.hex_color ? '#' + task.hex_color.replace(/^#/, '') : projectColor(props.project)
}
</script>

<template>
	<div class="timeline">
		<div class="tl-main">
			<div class="tl-toolbar">
				<button class="btn" @click="scrollToToday">{{ t('today') }}</button>
				<div class="spacer" />
				<div class="seg">
					<button v-for="s in scales" :key="s.key" :class="{on: prefs.timelineScale === s.key}" @click="prefs.timelineScale = s.key">{{ t(s.label) }}</button>
				</div>
			</div>
			<div ref="scroller" class="tl-scroll">
				<div
					ref="canvas"
					class="tl-canvas"
					:style="{width: width + 'px', '--row': ROW + 'px'}"
					@dragover="onCanvasOver"
					@dragleave.self="dropDay = null"
					@drop="onCanvasDrop"
				>
					<header class="tl-head">
						<div class="tl-hrow">
							<div v-for="g in topRow" :key="g.offset" class="tl-hcell top" :style="{left: g.offset * dw + 'px', width: g.days * dw + 'px'}">
								<span>{{ topLabel(g.date) }}</span>
							</div>
						</div>
						<div class="tl-hrow">
							<div
								v-for="g in bottomRow"
								:key="g.offset"
								class="tl-hcell"
								:class="{today: prefs.timelineScale === 'day' && g.date.getTime() === today.getTime()}"
								:style="{left: g.offset * dw + 'px', width: g.days * dw + 'px'}"
							>
								<span v-if="prefs.timelineScale === 'day'" class="wd">{{ fmtWeekdayNarrow(g.date) }}</span>
								<span>{{ bottomLabel(g.date) }}</span>
							</div>
						</div>
					</header>

					<div class="tl-rows" :style="{height: Math.max(bars.length, 1) * ROW + 'px'}">
						<div v-for="w in weekends" :key="'w' + w.offset" class="tl-weekend" :style="{left: w.offset * dw + 'px', width: dw + 'px'}" />
						<div v-for="g in bottomRow" :key="'l' + g.offset" class="tl-line" :style="{left: g.offset * dw + 'px'}" />
						<div class="tl-today" :style="{left: x(today) + dw / 2 + 'px'}" />
						<div v-if="dropDay !== null" class="tl-drop" :style="{left: dropDay * dw + 'px', width: dw + 'px'}" />

						<template v-for="(bar, i) in bars" :key="bar.task.id">
							<div
								class="tl-bar"
								:class="{done: bar.task.done, selected: bar.task.id === selectedId, dragging: drag?.task.id === bar.task.id && drag.moved, leaving: drag?.task.id === bar.task.id && drag.unschedule}"
								:style="{
									top: i * ROW + 6 + 'px',
									left: x(shown(bar).start) + 'px',
									width: barWidth(shown(bar)) + 'px',
									'--c': barColor(bar.task),
								}"
								:title="bar.task.title"
								@pointerdown="onBarDown($event, bar, 'move')"
								@contextmenu="openTaskMenu(bar.task.id, $event)"
							>
								<span class="grip start" @pointerdown="onBarDown($event, bar, 'start')" />
								<span class="bar-in">
									<TaskCheckbox v-if="barWidth(shown(bar)) >= MIN_CHECKBOX" :done="bar.task.done" :priority="bar.task.priority" :size="13" on-color @toggle="data.toggleDone(bar.task.id)" />
									<span v-if="barWidth(shown(bar)) >= MIN_LABELLED" class="label">{{ bar.task.title }}</span>
								</span>
								<span class="grip end" @pointerdown="onBarDown($event, bar, 'end')" />
							</div>
							<span
								v-if="barWidth(shown(bar)) < MIN_LABELLED"
								class="tl-outside"
								:class="{done: bar.task.done}"
								:style="{top: i * ROW + 'px', left: x(shown(bar).start) + barWidth(shown(bar)) + 6 + 'px'}"
							>{{ bar.task.title }}</span>
						</template>
						<div v-if="!bars.length" class="tl-empty muted">{{ t('timelineEmpty') }}</div>
					</div>
				</div>
			</div>
		</div>

		<aside class="tl-undated" :class="{target: drag?.unschedule}">
			<div class="ud-head">
				<span>{{ t('noDate') }}</span>
				<span class="count">{{ undated.length }}</span>
			</div>
			<div class="ud-list">
				<div
					v-for="task in undated"
					:key="task.id"
					class="ud-item"
					:class="{selected: task.id === selectedId, done: task.done, dragging: ui.dragTaskId === task.id}"
					draggable="true"
					@click="emit('select', task.id)"
					@contextmenu="openTaskMenu(task.id, $event)"
					@dragstart="onUndatedDragStart($event, task)"
					@dragend="onDragEnd"
				>
					<TaskCheckbox :done="task.done" :priority="task.priority" @toggle="data.toggleDone(task.id)" />
					<span class="ud-title">{{ task.title }}</span>
				</div>
				<div class="ud-hint muted">{{ drag?.moved && drag.mode === 'move' ? t('timelineUnscheduleHint') : t('timelineScheduleHint') }}</div>
			</div>
		</aside>
	</div>
</template>

<style scoped>
.timeline {
	display: flex;
	min-height: 0;
}

.tl-main {
	flex: 1;
	min-width: 0;
	display: flex;
	flex-direction: column;
}

.tl-toolbar {
	display: flex;
	align-items: center;
	gap: 8px;
	padding: 0 20px 10px;
}

.tl-toolbar .btn {
	height: 28px;
}

.spacer {
	flex: 1;
}

.seg {
	display: flex;
	padding: 2px;
	border-radius: 7px;
	background: var(--bg-hover);
}

.seg button {
	height: 24px;
	padding: 0 12px;
	border-radius: 5px;
	font-size: 13px;
	color: var(--text-2);
}

.seg button.on {
	color: var(--text);
	background: var(--bg);
	box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.tl-scroll {
	flex: 1;
	overflow: auto;
	border-top: 1px solid var(--border);
}

.tl-canvas {
	position: relative;
	min-height: 100%;
}

.tl-head {
	position: sticky;
	top: 0;
	z-index: 3;
	background: var(--bg);
	border-bottom: 1px solid var(--border);
}

.tl-hrow {
	position: relative;
	height: 26px;
}

.tl-hcell {
	position: absolute;
	top: 0;
	bottom: 0;
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 3px;
	/* clip, not hidden: hidden would stop the month label from sticking. */
	overflow: clip;
	font-size: 12px;
	color: var(--text-3);
	white-space: nowrap;
	border-left: 1px solid var(--border);
}

.tl-hcell.top {
	justify-content: flex-start;
	padding-left: 8px;
	font-weight: 600;
	color: var(--text-2);
}

/* Keep a month's name in view while it is partly scrolled past. */
.tl-hcell.top span {
	position: sticky;
	left: 8px;
}

.tl-hcell .wd {
	color: var(--text-4);
}

.tl-hcell.today {
	color: var(--accent);
	font-weight: 600;
}

.tl-rows {
	position: relative;
	min-height: calc(100% - 53px);
	padding-bottom: 40px;
	box-sizing: content-box;
}

.tl-line {
	position: absolute;
	top: 0;
	bottom: 0;
	width: 1px;
	background: var(--border);
	opacity: 0.6;
}

.tl-weekend {
	position: absolute;
	top: 0;
	bottom: 0;
	background: var(--bg-hover);
	opacity: 0.6;
}

.tl-today {
	position: absolute;
	top: 0;
	bottom: 0;
	width: 2px;
	margin-left: -1px;
	background: var(--accent);
	opacity: 0.7;
	z-index: 1;
}

.tl-drop {
	position: absolute;
	top: 0;
	bottom: 0;
	background: color-mix(in srgb, var(--accent) 18%, transparent);
}

.tl-bar {
	position: absolute;
	z-index: 2;
	display: flex;
	align-items: center;
	height: calc(var(--row) - 12px);
	min-width: 4px;
	border-radius: 6px;
	color: #fff;
	background: var(--c);
	font-size: 12.5px;
	cursor: grab;
	user-select: none;
	touch-action: none;
}

.tl-bar.done {
	opacity: 0.45;
}

.tl-bar.done .label {
	text-decoration: line-through;
}

.tl-bar.selected {
	box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--c);
}

.tl-bar.dragging {
	cursor: grabbing;
	box-shadow: var(--shadow);
}

.tl-bar.leaving {
	opacity: 0.3;
}

.tl-outside {
	position: absolute;
	z-index: 2;
	height: var(--row);
	line-height: var(--row);
	font-size: 12.5px;
	color: var(--text-2);
	white-space: nowrap;
	pointer-events: none;
}

.tl-outside.done {
	color: var(--text-3);
	text-decoration: line-through;
}

/* Sticks to the left edge while the bar's start is scrolled out of view. */
.bar-in {
	position: sticky;
	left: 0;
	padding: 0 8px;
	display: flex;
	align-items: center;
	gap: 6px;
	min-width: 0;
}

.tl-bar .label {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

/* Mostly outside the bar, so a one-day bar still has room to grab. */
.grip {
	position: absolute;
	top: 0;
	bottom: 0;
	width: 8px;
	cursor: ew-resize;
}

.grip.start {
	left: -6px;
}

.grip.end {
	right: -6px;
}

.tl-empty {
	position: absolute;
	top: 40px;
	left: 0;
	right: 0;
	text-align: center;
	font-size: 13px;
}

.tl-undated {
	flex: none;
	display: flex;
	flex-direction: column;
	width: 240px;
	border-left: 1px solid var(--border);
}

.tl-undated.target {
	background: color-mix(in srgb, var(--accent) 8%, transparent);
}

.ud-head {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 38px;
	padding: 0 14px;
	font-size: 13px;
	font-weight: 600;
	color: var(--text-2);
}

.count {
	color: var(--text-3);
	font-weight: 400;
}

.ud-list {
	flex: 1;
	overflow: auto;
	padding: 0 8px 16px;
}

.ud-item {
	display: flex;
	align-items: center;
	gap: 10px;
	min-height: 36px;
	padding: 0 8px;
	border-radius: 6px;
	font-size: 14px;
	cursor: grab;
}

.ud-item:hover {
	background: var(--bg-hover);
}

.ud-item.selected {
	background: color-mix(in srgb, var(--accent) 12%, transparent);
}

.ud-item.done .ud-title {
	color: var(--text-3);
	text-decoration: line-through;
}

.ud-item.dragging {
	opacity: 0.4;
}

.ud-title {
	flex: 1;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.ud-hint {
	padding: 10px 8px;
	font-size: 12px;
	line-height: 1.4;
}

@media (max-width: 767px) {
	.timeline {
		flex-direction: column;
	}

	.tl-toolbar {
		padding: 0 10px 8px;
	}

	.tl-undated {
		width: auto;
		max-height: 32%;
		border-left: 0;
		border-top: 1px solid var(--border);
	}
}
</style>
