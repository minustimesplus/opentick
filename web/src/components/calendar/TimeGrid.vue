<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import TaskCheckbox from '@/components/TaskCheckbox.vue'
import Popover from '@/components/Popover.vue'
import EventIcon from './EventIcon.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {
	addMinutes, altDayLabel, dateKey, fmtHourLabel, fmtTime, fmtWeekdayShort, isSameDay, minutesOfDay, schedulePatch, startOfDay,
	weekNumber, withMinutes, type Schedule,
} from '@/lib/dates'
import {projectColor} from '@/lib/tasks'
import {openTaskDrawer, openTaskMenu} from '@/lib/ui'
import {prefs} from '@/lib/prefs'
import {isMobile} from '@/lib/viewport'
import {t} from '@/lib/i18n'
import type {CalEvent} from '@/stores/calendars'
import {allDayEventsFor, allDayFor, layoutDay, minOf, POINT_MINUTES, snap, totalHeight, yOf, type Scale, type TimedItem} from './layout'

const props = withDefaults(defineProps<{
	/** The days on screen. */
	days: Date[]
	/** The page before and after `days`, the same length; rendered off screen so they slide in while scrolling sideways. */
	before?: Date[]
	after?: Date[]
	tasks: Task[]
	events?: CalEvent[]
}>(), {before: () => [], after: () => [], events: () => []})
const emit = defineEmits<{
	openEvent: [CalEvent, MouseEvent]
	/** Scrolled a whole page sideways: the parent should move `days` by one page in this direction. */
	page: [1 | -1]
}>()
const data = useData()

// Hour label column; narrower on phones so seven days still fit.
const gutter = computed(() => isMobile.value ? 44 : 72)
const NIGHT_H = 44
const DAY_START = 6 * 60
const DAY_END = 21 * 60

const rootEl = ref<HTMLElement | null>(null)
const bodyEl = ref<HTMLElement | null>(null)
const innerEl = ref<HTMLElement | null>(null)
const viewportEl = ref<HTMLElement | null>(null)

// Every column rendered: the page before, the visible days, the page after.
// Column indices below are indices into this strip.
const strip = computed(() => {
	const n = props.days.length
	const fits = (d: Date[]) => d.length === n
	return {
		days: [...(fits(props.before) ? props.before : []), ...props.days, ...(fits(props.after) ? props.after : [])],
		lead: fits(props.before) ? n : 0,
		canBack: fits(props.before) && n > 0,
		canForward: fits(props.after) && n > 0,
	}
})
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
	rootEl.value?.addEventListener('wheel', onWheel, {passive: false})
	fit()
	ro = new ResizeObserver(fit)
	if (bodyEl.value) ro.observe(bodyEl.value)
	nowTimer = window.setInterval(() => { now.value = new Date() }, 60_000)
	nextTick(scrollToNow)
})

onBeforeUnmount(() => {
	rootEl.value?.removeEventListener('wheel', onWheel)
	stopSnap()
	clearTimeout(idleTimer)
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
const columns = computed(() => strip.value.days.map(day => ({
	day,
	key: dateKey(day),
	today: isSameDay(day, now.value),
	items: layoutDay(props.tasks, day, props.events),
	allDay: [
		...allDayEventsFor(props.events, day).map(event => ({key: `e${event.key}`, event, task: undefined})),
		...allDayFor(props.tasks, day).map(task => ({key: `t${task.id}`, task, event: undefined})),
	],
})))

// The all-day strip has a fixed height the user sets by dragging its bottom
// border; a day with more entries than fit shows "+N more" in the last row.
const AD_ROW = 24
const AD_MIN = AD_ROW
const maxAllDay = computed(() => Math.max(0, ...columns.value.map(c => c.allDay.length)))
const adMax = () => Math.max(AD_ROW * 3, Math.round(window.innerHeight * 0.5))
const adHeight = computed(() => Math.min(adMax(), Math.max(AD_MIN, prefs.calendarAllDayHeight)))
const adRows = computed(() => Math.max(1, Math.floor(adHeight.value / AD_ROW)))

function adVisible<T>(entries: T[]): T[] {
	return entries.length <= adRows.value ? entries : entries.slice(0, adRows.value - 1)
}

function expandAllDay() {
	prefs.calendarAllDayHeight = Math.min(adMax(), maxAllDay.value * AD_ROW)
}

function startAllDayResize(e: PointerEvent) {
	if (e.button !== 0) return
	e.preventDefault()
	const startY = e.clientY
	const startH = adHeight.value
	const move = (ev: PointerEvent) => {
		prefs.calendarAllDayHeight = Math.min(adMax(), Math.max(AD_MIN, startH + ev.clientY - startY))
	}
	const up = () => {
		window.removeEventListener('pointermove', move)
		window.removeEventListener('pointerup', up)
		window.removeEventListener('pointercancel', up)
		document.body.classList.remove('row-resizing')
	}
	document.body.classList.add('row-resizing')
	window.addEventListener('pointermove', move)
	window.addEventListener('pointerup', up)
	window.addEventListener('pointercancel', up)
}

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
		'--c': it.event ? it.event.color : color(it.task!),
	}
}

function timeRange(s: Schedule) {
	return s.end ? `${fmtTime(s.start)}-${fmtTime(s.end)}` : fmtTime(s.start)
}

// ---- pointer interactions ----
interface DragState {
	kind: 'move' | 'resize' | 'create' | 'create-allday' | 'move-allday'
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
	/** Last day of an all-day range being created. */
	endDayIdx?: number
	startMin: number
	endMin: number
	allDay: boolean
	task?: Task
}

let drag: DragState | null = null
const preview = ref<Preview | null>(null)
const dragging = ref(false)

function pos(e: PointerEvent | DragEvent) {
	const inner = innerEl.value!.getBoundingClientRect()
	const body = bodyEl.value!.getBoundingClientRect()
	const vp = viewportEl.value!.getBoundingClientRect()
	const colW = vp.width / Math.max(1, props.days.length)
	const x = e.clientX - vp.left - sx + strip.value.lead * colW
	const dayIdx = Math.max(0, Math.min(strip.value.days.length - 1, Math.floor(x / colW)))
	const min = Math.max(0, Math.min(1440, minOf(e.clientY - inner.top, scale.value)))
	return {dayIdx, min, inAllDay: e.clientY < body.top}
}

// ---- sideways scrolling ----
// Trackpad swipes and shift+wheel slide the columns with the fingers, using
// the neighbouring pages rendered off screen. Passing a whole page commits it
// (the parent moves `days`), and once the gesture ends the grid eases to the
// nearest page, leaning toward the direction it was scrolled in. The offset is
// written straight to a CSS variable so scrolling never re-renders the grid.

/** Horizontal offset of the strip from its resting place, in px; negative shows later days. */
let sx = 0
/** Pages the parent was asked to move that have not reached the props yet. */
let pendingPages = 0
let lastDir = 0
let gestureAxis: 'x' | 'y' | null = null
let idleTimer: number | undefined
let snapFrame = 0
const SNAP_LEAN = 0.12
const GESTURE_GAP = 160

function pageW() {
	return viewportEl.value?.clientWidth ?? 0
}

function applyOffset() {
	rootEl.value?.style.setProperty('--sx', `${sx}px`)
}

function clampOffset() {
	const w = pageW()
	const min = strip.value.canForward ? -w : 0
	const max = strip.value.canBack ? w : 0
	sx = Math.max(min, Math.min(max, sx))
}

/** Commits a page once the strip has slid a whole page; the props catch up in `watch(days)`. */
function maybeTurnPage() {
	const w = pageW()
	if (!w || pendingPages) return
	if (sx <= -w && strip.value.canForward) {
		pendingPages = 1
		emit('page', 1)
	} else if (sx >= w && strip.value.canBack) {
		pendingPages = -1
		emit('page', -1)
	}
}

watch(() => props.days.map(dateKey).join(), () => {
	if (pendingPages) {
		// The strip moved one page under the offset: shift the offset back so
		// the same days stay under the fingers.
		sx += pendingPages * pageW()
		pendingPages = 0
	} else {
		// Navigated some other way (buttons, mini calendar): start at rest.
		stopSnap()
		sx = 0
	}
	clampOffset()
	applyOffset()
}, {flush: 'post'})

function wheelPx(delta: number, e: WheelEvent) {
	if (e.deltaMode === 1) return delta * 16
	if (e.deltaMode === 2) return delta * pageW()
	return delta
}

function onWheel(e: WheelEvent) {
	if (e.ctrlKey || drag || (!strip.value.canBack && !strip.value.canForward)) return
	// Shift+wheel scrolls sideways; some platforms already report it as deltaX.
	const dx = e.shiftKey && !e.deltaX ? e.deltaY : e.deltaX
	const dy = e.shiftKey ? 0 : e.deltaY
	// Lock each gesture to the axis it started on, so a vertical scroll that
	// drifts sideways does not wobble the columns.
	if (!gestureAxis) {
		if (!dx && !dy) return
		gestureAxis = Math.abs(dx) > Math.abs(dy) ? 'x' : 'y'
	}
	clearTimeout(idleTimer)
	idleTimer = window.setTimeout(endGesture, GESTURE_GAP)
	if (gestureAxis !== 'x') return
	e.preventDefault()
	stopSnap()
	const d = wheelPx(dx, e)
	if (!d) return
	lastDir = Math.sign(d)
	sx -= d
	clampOffset()
	maybeTurnPage()
	applyOffset()
}

function endGesture() {
	const axis = gestureAxis
	gestureAxis = null
	if (axis === 'x') snapToPage()
}

function snapToPage() {
	const w = pageW()
	if (!w) return
	const frac = sx / w
	let target = 0
	if (lastDir > 0 && frac < -SNAP_LEAN) target = -w
	else if (lastDir < 0 && frac > SNAP_LEAN) target = w
	else if (Math.abs(frac) > 0.5) target = Math.sign(frac) * w
	if (target < 0 && !strip.value.canForward) target = 0
	if (target > 0 && !strip.value.canBack) target = 0
	animateTo(target)
}

function animateTo(target: number) {
	stopSnap()
	const from = sx
	if (from === target) return
	const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
	const duration = reduce ? 0 : Math.min(320, 160 + Math.abs(target - from) * 0.25)
	const t0 = performance.now()
	const tick = (now: number) => {
		const p = duration ? Math.min(1, (now - t0) / duration) : 1
		const eased = 1 - Math.pow(1 - p, 3)
		sx = from + (target - from) * eased
		if (p >= 1) {
			snapFrame = 0
			sx = target
			maybeTurnPage()
		} else {
			snapFrame = requestAnimationFrame(tick)
		}
		applyOffset()
	}
	snapFrame = requestAnimationFrame(tick)
}

function stopSnap() {
	if (snapFrame) cancelAnimationFrame(snapFrame)
	snapFrame = 0
}

// A touch that turns into a scroll ends with pointercancel, not pointerup.
function attach() {
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
	drag = null
	preview.value = null
	dragging.value = false
	detach()
}

function onEsc(e: KeyboardEvent) {
	if (e.key === 'Escape') cancel()
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

function startCreateAllDay(e: PointerEvent, dayIdx: number) {
	if (e.button !== 0 || (e.target as HTMLElement).closest('.ad-chip, .ad-more')) return
	e.preventDefault()
	drag = {kind: 'create-allday', x0: e.clientX, y0: e.clientY, started: false, grabMin: 0, dur: null, dayIdx0: dayIdx, min0: 0}
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
	if (drag.kind === 'create-allday') {
		const a = Math.min(drag.dayIdx0, p.dayIdx)
		preview.value = {dayIdx: a, endDayIdx: Math.max(drag.dayIdx0, p.dayIdx), startMin: 0, endMin: 0, allDay: true}
	} else if (drag.kind === 'create') {
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
	// A plain click on empty space does nothing; creating needs a drag.
	if (!d.started) {
		if (d.task) openTaskDrawer(d.task.id)
		return
	}
	if (!pv) return
	const day = strip.value.days[pv.dayIdx]
	if (d.kind === 'create' || d.kind === 'create-allday') {
		openCreate(e, pv)
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
	endDay: Date
	start: Date | null
	end: Date | null
	allDay: boolean
}
const creating = ref<Creating | null>(null)
const newTitle = ref('')

function openCreate(e: PointerEvent, pv: Preview) {
	const day = strip.value.days[pv.dayIdx]
	creating.value = {
		anchor: {x: e.clientX, y: e.clientY},
		day,
		endDay: strip.value.days[pv.endDayIdx ?? pv.dayIdx],
		start: pv.allDay ? null : withMinutes(day, pv.startMin),
		end: pv.allDay ? null : withMinutes(day, pv.endMin),
		allDay: pv.allDay,
	}
	newTitle.value = ''
	preview.value = pv
}

function inAllDayPreview(i: number) {
	const pv = preview.value
	return !!pv?.allDay && i >= pv.dayIdx && i <= (pv.endDayIdx ?? pv.dayIdx)
}

async function submitCreate() {
	const c = creating.value
	const title = newTitle.value.trim()
	creating.value = null
	preview.value = null
	if (!c || !title) return
	const patch = c.allDay ? schedulePatch(c.day, true, c.endDay) : schedulePatch(c.start, false, c.end)
	await data.createTask({title, ...patch, project_id: prefs.defaultListId || data.inboxId})
}

function closeCreate() {
	creating.value = null
	preview.value = null
}

const creatingLabel = computed(() => {
	const c = creating.value
	if (!c) return ''
	if (c.allDay) {
		const end = isSameDay(c.day, c.endDay) ? '' : ` - ${fmtWeekdayShort(c.endDay)} ${c.endDay.getDate()}`
		return `${fmtWeekdayShort(c.day)} ${c.day.getDate()}${end} · ${t('allDay')}`
	}
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
	const start = withMinutes(strip.value.days[pv.dayIdx], pv.startMin)
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
	if (id) data.updateTask(id, schedulePatch(strip.value.days[idx], true))
}

const nowY = computed(() => yOf(minutesOfDay(now.value), scale.value))
const gutterStyle = computed(() => ({width: `${gutter.value}px`}))
// The strip is as many pages wide as it has columns, parked on the visible page.
const trackStyle = computed(() => {
	const n = Math.max(1, props.days.length)
	const total = strip.value.days.length
	return {
		width: `${(total / n) * 100}%`,
		gridTemplateColumns: `repeat(${total}, minmax(0, 1fr))`,
		'--lead': `${(-strip.value.lead / Math.max(1, total)) * 100}%`,
	}
})
const today0 = computed(() => startOfDay(now.value))
</script>

<template>
	<div ref="rootEl" class="tg" :class="{dragging}">
		<div class="tg-head">
			<div class="g-head" :style="gutterStyle">
				<div class="wk"><template v-if="prefs.showWeekNumbers && days.length">{{ t('weekShort') }}{{ weekNumber(days[0]) }}</template></div>
			</div>
			<div class="hvp">
				<div class="track" :style="trackStyle">
					<div
						v-for="(col, i) in columns"
						:key="col.key"
						class="col-head"
						:class="{drop: dropAllDay === i, 'drop-preview': inAllDayPreview(i)}"
						@pointerdown="startCreateAllDay($event, i)"
						@dragover="onAllDayDragOver($event, i)"
						@dragleave="dropAllDay = null"
						@drop="onAllDayDrop($event, i)"
					>
						<div class="wd">{{ fmtWeekdayShort(col.day) }}</div>
						<div class="dn">
							<span :class="{today: col.today, past: col.day < today0}">{{ col.day.getDate() }}</span>
							<small v-if="prefs.altCalendar !== 'none'" class="alt">{{ altDayLabel(col.day) }}</small>
						</div>
						<div class="ad-list" :style="{height: adHeight + 'px'}">
							<template v-for="entry in adVisible(col.allDay)" :key="entry.key">
								<div
									v-if="entry.event"
									class="ad-chip ext"
									:class="{done: entry.event.archived}"
									:style="{'--c': entry.event.color}"
									:title="entry.event.title"
									@click.stop="emit('openEvent', entry.event!, $event)"
								>
									<EventIcon />
									<span class="ad-title">{{ entry.event.title }}</span>
								</div>
								<div
									v-else-if="entry.task"
									class="ad-chip"
									:class="{done: entry.task.done}"
									:style="{'--c': color(entry.task)}"
									@pointerdown="startOnAllDay($event, entry.task!, i)"
									@contextmenu="openTaskMenu(entry.task!.id, $event)"
								>
									<TaskCheckbox :done="entry.task.done" :wont-do="entry.task.wont_do" on-color :size="13" @toggle="data.toggleDone(entry.task!.id)" />
									<span class="ad-title">{{ entry.task.title }}</span>
								</div>
							</template>
							<button
								v-if="col.allDay.length > adRows"
								class="ad-more"
								@click.stop="expandAllDay"
							>{{ t('more', {n: col.allDay.length - (adRows - 1)}) }}</button>
						</div>
					</div>
				</div>
			</div>
			<div class="ad-resizer" @pointerdown="startAllDayResize" />
		</div>

		<div ref="bodyEl" class="tg-body" @dragover="onDragOver" @dragleave="preview = drag ? preview : null" @drop="onDrop">
			<div ref="innerEl" class="tg-inner" :style="{height: height + 'px'}" @pointerdown="startCreate">
				<div class="gutter" :style="gutterStyle">
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

				<div ref="viewportEl" class="bvp">
					<div class="lines">
						<div v-for="l in lines" :key="l.y" class="line" :style="{top: l.y + 'px'}" />
						<div v-for="(n, i) in nightBlocks" :key="'nb' + i" class="night" :style="{top: n.y + 'px', height: n.h + 'px'}" />
					</div>

					<div class="track" :style="trackStyle">
						<div v-for="(col, i) in columns" :key="col.key" class="col" :class="{today: col.today}" :data-idx="i">
							<template v-for="it in col.items" :key="it.key">
								<div
									v-if="it.event"
									class="ev ext"
									:class="{point: it.point, short: yOf(it.endMin, scale) - yOf(it.startMin, scale) < 40, done: it.event.archived}"
									:style="itemStyle(it)"
									:title="it.event.title"
									@pointerdown.stop
									@click="emit('openEvent', it.event!, $event)"
								>
									<div class="ev-top">
										<EventIcon />
										<span class="ev-title">{{ it.event.title }}</span>
									</div>
									<div v-if="!it.point" class="ev-time">{{ timeRange(it.schedule) }}</div>
								</div>
								<div
									v-else-if="it.task"
									class="ev"
									:class="{point: it.point, done: it.task.done, ghost: preview?.task?.id === it.task.id, short: yOf(it.endMin, scale) - yOf(it.startMin, scale) < 40}"
									:style="itemStyle(it)"
									@pointerdown.stop="startOnItem($event, it, i, 'move')"
									@contextmenu="openTaskMenu(it.task!.id, $event)"
								>
									<div class="ev-top">
										<TaskCheckbox :done="it.task.done" :wont-do="it.task.wont_do" on-color :size="13" @toggle="data.toggleDone(it.task!.id)" />
										<span class="ev-title">{{ it.task.title }}</span>
									</div>
									<div v-if="!it.point" class="ev-time">{{ timeRange(it.schedule) }}</div>
									<div class="resize" @pointerdown.stop="startOnItem($event, it, i, 'resize')" />
								</div>
							</template>

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
								<div class="ev-time">{{ fmtTime(withMinutes(strip.days[i], preview.startMin)) }}-{{ fmtTime(withMinutes(strip.days[i], preview.endMin)) }}</div>
							</div>

							<div v-if="col.today" class="now" :style="{top: nowY + 'px'}" />
						</div>
					</div>
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
	--sx: 0px;
	display: flex;
	flex-direction: column;
	height: 100%;
	min-height: 0;
	user-select: none;
	/* Sideways swipes scroll the days, not the browser history. */
	overscroll-behavior-x: none;
}

.tg.dragging {
	cursor: grabbing;
}

.tg-head,
.tg-body {
	scrollbar-gutter: stable;
}

.tg-head {
	position: relative;
	display: flex;
	overflow-y: hidden;
	border-bottom: 1px solid var(--border);
	flex-shrink: 0;
}

.g-head,
.gutter {
	flex-shrink: 0;
}

/* Viewports clip the strip of columns to the visible page. `clip` rather than
   `hidden`, so focusing something off screen never scrolls them. */
.hvp,
.bvp {
	flex: 1;
	min-width: 0;
	overflow: clip;
}

.bvp {
	position: relative;
}

.track {
	display: grid;
	transform: translateX(calc(var(--lead) + var(--sx)));
	will-change: transform;
}

.bvp > .track {
	position: relative;
	height: 100%;
}

.col-head {
	padding: 0 6px 3px;
	border-left: 1px solid var(--grid-line);
	min-width: 0;
	cursor: default;
}

.col-head.drop,
.col-head.drop-preview {
	background: var(--accent-soft);
}

.wd,
.wk {
	height: 26px;
	line-height: 26px;
	font-size: 12px;
	color: var(--text-3);
	border-bottom: 1px solid var(--grid-line);
}

/* The divider under the weekday runs edge to edge across the column. */
.wd {
	margin: 0 -6px;
	text-align: center;
}

.dn {
	height: 32px;
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

.dn .alt {
	margin-left: 6px;
	font-size: 11px;
	font-weight: 400;
	color: var(--text-3);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}

.wk {
	padding-right: 10px;
	text-align: right;
}

.dn span.today {
	background: var(--accent);
	color: #fff;
}

.ad-list {
	display: flex;
	flex-direction: column;
	gap: 2px;
	overflow: hidden;
}

/* Drag handle on the strip's bottom border; the head clips overflow, so the
   hit area sits just above the line. */
.ad-resizer {
	position: absolute;
	left: 0;
	right: 0;
	bottom: 0;
	height: 6px;
	cursor: row-resize;
	touch-action: none;
	z-index: 2;
}

.ad-resizer:hover {
	background: var(--accent);
	opacity: 0.35;
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
	display: flex;
	position: relative;
}

.gutter {
	position: relative;
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

.ev.ext,
.ad-chip.ext {
	/* Read-only calendar events share the task fill; the calendar icon in
	   place of the checkbox marks them as context. */
	cursor: pointer;
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

@media (max-width: 767px) {
	.hour-label {
		right: 6px;
		font-size: 11px;
	}

	.col-head {
		padding: 0 2px 3px;
	}

	.wd {
		margin: 0 -2px;
	}

	.dn {
		justify-content: center;
		padding-left: 0;
		font-size: 15px;
	}

	.dn .alt {
		display: none;
	}
}
</style>
