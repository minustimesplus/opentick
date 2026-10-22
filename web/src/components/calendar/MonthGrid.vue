<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import TaskCheckbox from '@/components/TaskCheckbox.vue'
import Popover from '@/components/Popover.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {
	addDays, altDayLabel, dateKey, dayDiff, fmtDayMonth, fmtMonthShort, fmtTime, fmtWeekdayShort, getSchedule, isSameDay, minutesOfDay,
	moveSchedule, schedulePatch, startOfDay, startOfMonth, startOfWeek, weekNumber, withMinutes,
} from '@/lib/dates'
import {projectColor} from '@/lib/tasks'
import {openTaskDrawer, openTaskMenu} from '@/lib/ui'
import {prefs} from '@/lib/prefs'
import {isMobile} from '@/lib/viewport'
import {t} from '@/lib/i18n'
import type {CalEvent} from '@/stores/calendars'

const props = withDefaults(defineProps<{month: Date, tasks: Task[], events?: CalEvent[]}>(), {events: () => []})
const emit = defineEmits<{
	day: [Date]
	/** The month currently scrolled into view (first of month). */
	month: [Date]
	/** The span of weeks currently rendered. */
	range: [{start: Date, end: Date}]
	openEvent: [CalEvent, MouseEvent]
}>()
const data = useData()
const MAX = 4
const WEEK_COL = 32

// ---- continuous week scrolling ----
// The grid is an endless list of week rows, 6 per viewport, snapping week by week.
// A window of weeks is rendered and extended (or recentred) as the user nears its edges.
const ROWS = 6
const BUFFER = 26
const EDGE = 6
const MAX_WEEKS = 4 * BUFFER + ROWS

const scroller = ref<HTMLElement | null>(null)
const rowH = ref(0)
const base = ref(addDays(firstWeekOf(props.month), -BUFFER * 7))
const count = ref(2 * BUFFER + ROWS)

function monthKey(d: Date) {
	return `${d.getFullYear()}-${d.getMonth()}`
}

function firstWeekOf(d: Date) {
	return startOfWeek(startOfMonth(d))
}

const shown = ref(monthKey(props.month))
const shownMonth = ref(props.month.getMonth())

const weeks = computed(() => Array.from({length: count.value}, (_, w) => {
	const start = addDays(base.value, w * 7)
	return {
		key: dateKey(start),
		days: Array.from({length: 7}, (_, d) => addDays(start, d))
			.filter(d => prefs.calendarShowWeekends || (d.getDay() !== 0 && d.getDay() !== 6)),
	}
}))

const colCount = computed(() => weeks.value[0]?.days.length ?? 7)
const gridCols = computed(() => ({
	gridTemplateColumns: `${prefs.showWeekNumbers ? `${WEEK_COL}px ` : ''}repeat(${colCount.value}, minmax(0, 1fr))`,
}))

watch([base, count], () => emit('range', {start: base.value, end: addDays(base.value, count.value * 7)}), {immediate: true})

function weekIndexOf(d: Date) {
	return Math.round(dayDiff(startOfWeek(d), base.value) / 7)
}

function topIndex() {
	return rowH.value ? scroller.value!.scrollTop / rowH.value : 0
}

async function setTop(idx: number) {
	await nextTick()
	if (scroller.value) scroller.value.scrollTop = idx * rowH.value
}

function recentre(d: Date) {
	base.value = addDays(firstWeekOf(d), -BUFFER * 7)
	count.value = 2 * BUFFER + ROWS
	return setTop(BUFFER)
}

function extend() {
	const el = scroller.value
	if (!el || !rowH.value || navigating) return
	const top = topIndex()
	if (top < EDGE) {
		base.value = addDays(base.value, -BUFFER * 7)
		count.value = Math.min(MAX_WEEKS, count.value + BUFFER)
		setTop(top + BUFFER)
	} else if (top + ROWS > count.value - EDGE) {
		let next = count.value + BUFFER
		let trim = 0
		if (next > MAX_WEEKS) {
			trim = next - MAX_WEEKS
			next = MAX_WEEKS
		}
		base.value = addDays(base.value, trim * 7)
		count.value = next
		if (trim) setTop(top - trim)
	}
}

/** The month a viewport belongs to is the one of its third row, mid-week. */
function updateShown() {
	if (!rowH.value) return
	const idx = Math.min(count.value - 1, Math.round(topIndex()) + 2)
	const d = addDays(base.value, idx * 7 + 3)
	shownMonth.value = d.getMonth()
	const key = monthKey(d)
	if (key === shown.value) return
	shown.value = key
	emit('month', startOfMonth(d))
}

function onScroll() {
	extend()
	updateShown()
}

// Programmatic navigation (header arrows, today, mini month): smooth when near, jump when far.
let navigating = false
let navTimer: number | undefined

function endNav() {
	navigating = false
	clearTimeout(navTimer)
	extend()
	updateShown()
}

async function scrollToMonth(d: Date, smooth: boolean) {
	const el = scroller.value
	if (!el || !rowH.value) return
	shown.value = monthKey(d)
	shownMonth.value = d.getMonth()
	const idx = weekIndexOf(firstWeekOf(d))
	if (!smooth || Math.abs(idx - topIndex()) > 12 || idx < EDGE || idx + ROWS > count.value - EDGE) {
		await recentre(d)
		return
	}
	navigating = true
	clearTimeout(navTimer)
	navTimer = window.setTimeout(endNav, 700)
	el.scrollTo({top: idx * rowH.value, behavior: 'smooth'})
}

watch(() => prefs.weekStart, () => recentre(props.month))

watch(() => props.month, m => {
	if (monthKey(m) !== shown.value) scrollToMonth(m, true)
})

function fit() {
	const el = scroller.value
	if (!el?.clientHeight) return
	const top = rowH.value ? topIndex() : weekIndexOf(firstWeekOf(props.month))
	rowH.value = el.clientHeight / ROWS
	setTop(Math.round(top))
}

let ro: ResizeObserver | null = null

onMounted(() => {
	fit()
	ro = new ResizeObserver(fit)
	if (scroller.value) {
		ro.observe(scroller.value)
		scroller.value.addEventListener('scrollend', () => { if (navigating) endNav() })
	}
})

onBeforeUnmount(() => {
	ro?.disconnect()
	clearTimeout(navTimer)
	detach()
})

// ---- tasks ----
interface DayEntry {
	key: string
	task?: Task
	event?: CalEvent
	allDay: boolean
	start: Date
}

const byDay = computed(() => {
	const map = new Map<string, DayEntry[]>()
	const add = (first: Date, last: Date, entry: DayEntry) => {
		for (let d = startOfDay(first); d <= last; d = addDays(d, 1)) {
			const k = dateKey(d)
			if (!map.has(k)) map.set(k, [])
			map.get(k)!.push(entry)
		}
	}
	for (const task of props.tasks) {
		const s = getSchedule(task)
		if (!s) continue
		add(s.start, s.end && s.allDay ? s.end : s.date, {key: `t${task.id}`, task, allDay: s.allDay, start: s.start})
	}
	for (const event of props.events) {
		// The end is exclusive: an event ending at midnight does not show on the next day.
		const last = event.end > event.start ? new Date(event.end.getTime() - 1) : event.start
		add(event.start, last, {key: `e${event.key}`, event, allDay: event.allDay, start: event.start})
	}
	for (const list of map.values()) {
		list.sort((a, b) => Number(!a.allDay) - Number(!b.allDay) || a.start.getTime() - b.start.getTime())
	}
	return map
})

const today = startOfDay(new Date())

function color(task: Task) {
	return projectColor(data.project(task.project_id))
}

function chipTime(entry: DayEntry) {
	return entry.allDay ? '' : fmtTime(entry.start)
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

// ---- drag across days to create ----
interface Selecting {
	from: Date
	to: Date
	x0: number
	y0: number
	started: boolean
}

let sel: Selecting | null = null
const selection = ref<{from: Date, to: Date} | null>(null)
const creating = ref<{from: Date, to: Date, x: number, y: number} | null>(null)
const title = ref('')

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

function dayAt(e: PointerEvent): Date | null {
	const cell = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>('.mg .cell')
	const k = cell?.dataset.day
	if (!k) return null
	const [y, m, d] = k.split('-').map(Number)
	return new Date(y, m - 1, d)
}

function onCellDown(e: PointerEvent, day: Date) {
	if (e.button !== 0 || (e.target as HTMLElement).closest('.chip, .more, .num')) return
	e.preventDefault()
	sel = {from: day, to: day, x0: e.clientX, y0: e.clientY, started: false}
	attach()
}

function onMove(e: PointerEvent) {
	if (!sel) return
	if (!sel.started) {
		if (Math.hypot(e.clientX - sel.x0, e.clientY - sel.y0) < 5) return
		sel.started = true
	}
	sel.to = dayAt(e) ?? sel.to
	selection.value = sel.from <= sel.to ? {from: sel.from, to: sel.to} : {from: sel.to, to: sel.from}
}

function onUp(e: PointerEvent) {
	detach()
	const s = sel
	sel = null
	if (!s?.started || !selection.value) {
		selection.value = null
		return
	}
	title.value = ''
	creating.value = {...selection.value, x: e.clientX, y: e.clientY}
}

function cancel() {
	sel = null
	selection.value = null
	detach()
}

function onEsc(e: KeyboardEvent) {
	if (e.key === 'Escape') cancel()
}

function selected(d: Date) {
	const s = selection.value
	return !!s && d >= s.from && d <= s.to
}

function closeCreate() {
	creating.value = null
	selection.value = null
}

async function submit() {
	const c = creating.value
	closeCreate()
	if (!c || !title.value.trim()) return
	await data.createTask({title: title.value.trim(), ...schedulePatch(c.from, true, c.to), project_id: prefs.defaultListId || data.inboxId})
}

const creatingLabel = computed(() => {
	const c = creating.value
	if (!c) return ''
	return isSameDay(c.from, c.to) ? fmtDayMonth(c.from) : `${fmtDayMonth(c.from)} - ${fmtDayMonth(c.to)}`
})
</script>

<template>
	<div class="mg">
		<div class="mg-head" :style="gridCols">
			<div v-if="prefs.showWeekNumbers" />
			<div v-for="d in weeks[0].days" :key="d.getDay()" class="wd">{{ fmtWeekdayShort(d) }}</div>
		</div>
		<div ref="scroller" class="mg-body" @scroll.passive="onScroll">
			<div
				v-for="week in weeks"
				:key="week.key"
				class="week"
				:style="{...gridCols, height: rowH + 'px'}"
			>
				<div v-if="prefs.showWeekNumbers" class="wk">{{ t('weekShort') }}{{ weekNumber(week.days[0]) }}</div>
				<div
					v-for="d in week.days"
					:key="dateKey(d)"
					class="cell"
					:data-day="dateKey(d)"
					:class="{other: d.getMonth() !== shownMonth, drop: dropKey === dateKey(d), sel: selected(d)}"
					@pointerdown="onCellDown($event, d)"
					@dragover.prevent="dropKey = dateKey(d)"
					@dragleave="dropKey === dateKey(d) && (dropKey = null)"
					@drop.prevent="onDrop($event, d)"
				>
					<div class="num-row">
						<button class="num" :class="{today: isSameDay(d, today), first: d.getDate() === 1}" @click="emit('day', d)">
							{{ d.getDate() === 1 ? `${fmtMonthShort(d)} 1` : d.getDate() }}
						</button>
						<span v-if="prefs.altCalendar !== 'none'" class="alt">{{ altDayLabel(d) }}</span>
					</div>
					<template v-for="entry in (byDay.get(dateKey(d)) ?? []).slice(0, MAX)" :key="entry.key">
						<div
							v-if="entry.event"
							class="chip ext"
							:class="{timed: !!chipTime(entry)}"
							:style="{'--c': entry.event.color}"
							:title="entry.event.title"
							@click="emit('openEvent', entry.event!, $event)"
						>
							<span v-if="chipTime(entry)" class="dot" />
							<span class="title">{{ entry.event.title }}</span>
							<span v-if="chipTime(entry)" class="time">{{ chipTime(entry) }}</span>
						</div>
						<div
							v-else-if="entry.task"
							class="chip"
							:class="{timed: !!chipTime(entry), done: entry.task.done}"
							:style="{'--c': color(entry.task)}"
							:draggable="!isMobile"
							@dragstart="onDragStart($event, entry.task!)"
							@click="openTaskDrawer(entry.task!.id)"
							@contextmenu="openTaskMenu(entry.task!.id, $event)"
						>
							<TaskCheckbox v-if="!chipTime(entry)" :done="entry.task.done" :wont-do="entry.task.wont_do" on-color :size="12" @toggle="data.toggleDone(entry.task!.id)" />
							<span v-else class="dot" />
							<span class="title">{{ entry.task.title }}</span>
							<span v-if="chipTime(entry)" class="time">{{ chipTime(entry) }}</span>
						</div>
					</template>
					<button v-if="(byDay.get(dateKey(d))?.length ?? 0) > MAX" class="more" @click="emit('day', d)">
						{{ t('more', {n: byDay.get(dateKey(d))!.length - MAX}) }}
					</button>
				</div>
			</div>
		</div>
		<Popover v-if="creating" :anchor="{x: creating.x, y: creating.y}" @close="closeCreate">
			<form class="create" @submit.prevent="submit">
				<input v-model="title" class="input" :placeholder="t('titlePlaceholder')" v-focus>
				<div class="create-meta">{{ creatingLabel }} · {{ t('allDay') }}</div>
			</form>
		</Popover>
	</div>
</template>

<style scoped>
.mg {
	display: flex;
	flex-direction: column;
	height: 100%;
	user-select: none;
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
	min-height: 0;
	overflow-y: auto;
	overflow-x: hidden;
	overflow-anchor: none;
	scroll-snap-type: y mandatory;
	scrollbar-width: none;
}

.mg-body::-webkit-scrollbar {
	display: none;
}

.week {
	display: grid;
	scroll-snap-align: start;
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

.num-row {
	display: flex;
	align-items: center;
	gap: 4px;
	min-width: 0;
	flex-shrink: 0;
}

.alt {
	min-width: 0;
	font-size: 11px;
	color: var(--text-3);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}

.cell.other .alt {
	color: var(--text-4);
}

.wk {
	padding-top: 8px;
	text-align: center;
	font-size: 11px;
	color: var(--text-3);
	border-right: 1px solid var(--grid-line);
	border-bottom: 1px solid var(--grid-line);
}

.cell.other .num {
	color: var(--text-4);
}

.cell.drop,
.cell.sel {
	background: var(--accent-soft);
}

.num {
	min-width: 24px;
	height: 24px;
	padding: 0 4px;
	border-radius: 12px;
	font-size: 13px;
	font-weight: 600;
	white-space: nowrap;
	flex-shrink: 0;
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

.chip.ext:not(.timed) {
	background: color-mix(in srgb, var(--c) 22%, var(--bg));
	color: var(--text);
	border-left: 3px solid var(--c);
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

.create-meta {
	margin-top: 8px;
	font-size: 12px;
	color: var(--accent);
}

/* Phones: cells are about 50px wide, so chips drop their times. */
@media (max-width: 767px) {
	.cell {
		padding: 2px;
	}

	.chip {
		gap: 3px;
		padding: 0 3px;
		font-size: 11px;
	}

	.chip .time,
	.alt {
		display: none;
	}
}
</style>
