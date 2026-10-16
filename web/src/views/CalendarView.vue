<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {
	AlertCircle, CalendarDays, Check, ChevronDown, ChevronLeft, ChevronRight, ExternalLink, Inbox, ListTodo, MapPin, Menu,
	MoreHorizontal, PanelLeft, Pencil, Plus, Repeat,
} from 'lucide-vue-next'
import TimeGrid from '@/components/calendar/TimeGrid.vue'
import MonthGrid from '@/components/calendar/MonthGrid.vue'
import MiniMonth from '@/components/calendar/MiniMonth.vue'
import Popover from '@/components/Popover.vue'
import QuickAdd from '@/components/QuickAdd.vue'
import TaskCheckbox from '@/components/TaskCheckbox.vue'
import SubscriptionDialog from '@/components/SubscriptionDialog.vue'
import type {CalendarSubscription, Task} from '@/api/types'
import {useData} from '@/stores/data'
import {calendarColor, useCalendars, type CalEvent} from '@/stores/calendars'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {
	addDays, addMonths, dateKey, fmtLongDate, fmtMonthYear, fmtTime, getSchedule, isSameDay, schedulePatch, startOfDay,
	startOfMonth, startOfWeek,
} from '@/lib/dates'
import {hasColor, projectColor, splitEmoji} from '@/lib/tasks'
import {openTaskDrawer} from '@/lib/ui'

const data = useData()
const calendars = useCalendars()
const route = useRoute()
const router = useRouter()

function parseKey(k: unknown): Date | null {
	if (typeof k !== 'string') return null
	const m = k.match(/^(\d{4})-(\d{2})-(\d{2})$/)
	return m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : null
}

const anchor = computed<Date>({
	get: () => parseKey(route.query.d) ?? startOfDay(new Date()),
	set: d => { router.replace({query: {...route.query, d: dateKey(d)}}) },
})

const mode = computed({
	get: () => prefs.calendarMode,
	set: v => { prefs.calendarMode = v },
})

const range = computed(() => {
	if (mode.value === 'day') return {start: startOfDay(anchor.value), end: addDays(startOfDay(anchor.value), 1)}
	if (mode.value === 'week') {
		const s = startOfWeek(anchor.value)
		return {start: s, end: addDays(s, 7)}
	}
	const s = startOfWeek(startOfMonth(anchor.value))
	return {start: s, end: addDays(s, 42)}
})

const days = computed(() => {
	if (mode.value === 'day') return [startOfDay(anchor.value)]
	const out: Date[] = []
	for (let i = 0; i < 7; i++) {
		const d = addDays(range.value.start, i)
		if (prefs.calendarShowWeekends || (d.getDay() !== 0 && d.getDay() !== 6)) out.push(d)
	}
	return out
})

watch(range, r => { data.loadDoneInRange(r.start, r.end).catch(() => {}) }, {immediate: true})

const hidden = computed(() => new Set(prefs.calendarHidden))

// The month grid scrolls continuously and renders more weeks than the anchor month.
const monthSpan = ref<{start: Date, end: Date} | null>(null)
const taskRange = computed(() => mode.value === 'month' && monthSpan.value ? monthSpan.value : range.value)

const visibleTasks = computed<Task[]>(() => data.allTasks.filter(task => {
	if (!data.projectMap.has(task.project_id) || hidden.value.has(task.project_id)) return false
	if (task.done && !prefs.calendarShowCompleted) return false
	const s = getSchedule(task)
	if (!s) return false
	const end = s.end ?? s.date
	return end >= taskRange.value.start && s.start < taskRange.value.end
}))

// ---- subscribed calendars ----
onMounted(() => {
	if (!calendars.loaded) calendars.load()
})

const hiddenSubs = computed(() => new Set(prefs.calendarHiddenSubs))
const visibleSubs = computed(() => calendars.subscriptions.filter(s => !hiddenSubs.value.has(s.id)))

// The scrolling month grid can show a few weeks before and after the anchor
// month (whose month is the one of its third row), so load a wider window there.
// Day and week views load whole weeks, so stepping through days stays in cache.
function eventRangeFor(m: typeof mode.value, at: Date) {
	if (m === 'month') {
		const s = startOfWeek(startOfMonth(at))
		return {start: addDays(s, -21), end: addDays(s, 70)}
	}
	const s = startOfWeek(at)
	return {start: s, end: addDays(s, 7)}
}

const eventRange = computed(() => eventRangeFor(mode.value, anchor.value))

// Keyed on the url too, so editing a subscription's address reloads it.
watch(
	() => [eventRange.value.start.getTime(), eventRange.value.end.getTime(), visibleSubs.value.map(s => `${s.id}:${s.url}`).join(',')],
	async () => {
		const ids = visibleSubs.value.map(s => s.id)
		await calendars.loadEvents(eventRange.value.start, eventRange.value.end, ids)
		// Prefetch the neighbouring ranges (the next two weeks, or the adjacent
		// months) so navigating shows events immediately.
		const neighbours = mode.value === 'month'
			? [addMonths(startOfMonth(anchor.value), 1), addMonths(startOfMonth(anchor.value), -1)]
			: [addDays(anchor.value, 7), addDays(anchor.value, 14), addDays(anchor.value, -7)]
		for (const at of neighbours) {
			const r = eventRangeFor(mode.value, at)
			calendars.prefetchEvents(r.start, r.end, ids)
		}
	},
	{immediate: true},
)

const visibleEvents = computed<CalEvent[]>(() => visibleSubs.value.flatMap(sub => {
	const color = calendarColor(sub)
	return (calendars.events.get(sub.id) ?? []).map(e => ({...e, color}))
}))

function toggleSub(id: number) {
	prefs.calendarHiddenSubs = hiddenSubs.value.has(id)
		? prefs.calendarHiddenSubs.filter(x => x !== id)
		: [...prefs.calendarHiddenSubs, id]
}

const subDialog = ref<{subscription: CalendarSubscription | null} | null>(null)

function openSubscribe() {
	pop.value = null
	subDialog.value = {subscription: null}
}

// ---- event details ----
const openedEvent = ref<{event: CalEvent, x: number, y: number} | null>(null)

function openEvent(event: CalEvent, e: MouseEvent) {
	openedEvent.value = {event, x: e.clientX, y: e.clientY}
}

function eventWhen(e: CalEvent): string {
	if (e.allDay) {
		const last = addDays(e.end, -1)
		return isSameDay(e.start, last) || last < e.start
			? `${fmtLongDate(e.start)} · ${t('allDay')}`
			: `${fmtLongDate(e.start)} – ${fmtLongDate(last)}`
	}
	if (isSameDay(e.start, e.end) || e.end <= e.start) {
		return `${fmtLongDate(e.start)} · ${fmtTime(e.start)}${e.end > e.start ? ' – ' + fmtTime(e.end) : ''}`
	}
	return `${fmtLongDate(e.start)} ${fmtTime(e.start)} – ${fmtLongDate(e.end)} ${fmtTime(e.end)}`
}

function safeLink(url: string): string | null {
	return /^https?:\/\//i.test(url.trim()) ? url.trim() : null
}

const title = computed(() => fmtMonthYear(mode.value === 'month' ? anchor.value : days.value[0] ?? anchor.value))

function step(dir: number) {
	if (mode.value === 'day') anchor.value = addDays(anchor.value, dir)
	else if (mode.value === 'week') anchor.value = addDays(anchor.value, 7 * dir)
	else anchor.value = addMonths(startOfMonth(anchor.value), dir)
}

function goToday() {
	anchor.value = startOfDay(new Date())
}

function onMonthScrolled(d: Date) {
	if (d.getFullYear() !== anchor.value.getFullYear() || d.getMonth() !== anchor.value.getMonth()) anchor.value = d
}

function openDay(d: Date) {
	mode.value = 'day'
	anchor.value = d
}

// ---- list visibility ----
const allLists = computed(() => data.realProjects)
const allVisible = computed(() => prefs.calendarHidden.length === 0)

function toggleList(id: number) {
	prefs.calendarHidden = hidden.value.has(id) ? prefs.calendarHidden.filter(x => x !== id) : [...prefs.calendarHidden, id]
}

function toggleAll() {
	prefs.calendarHidden = allVisible.value ? allLists.value.map(p => p.id) : []
}

function listName(id: number) {
	const p = data.project(id)
	if (!p) return {emoji: null, name: ''}
	if (p.id === data.inboxId) return {emoji: null, name: t('inbox'), inbox: true}
	return splitEmoji(p.title)
}

// ---- arrange panel: unscheduled tasks to drag on the grid ----
const arrangeOpen = ref(false)
const unscheduled = computed(() => data.openTasks
	.filter(task => !getSchedule(task) && !hidden.value.has(task.project_id))
	.sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0) || a.id - b.id))

function onArrangeDragStart(e: DragEvent, task: Task) {
	e.dataTransfer?.setData('application/x-task-id', String(task.id))
	if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onArrangeDrop(e: DragEvent) {
	const id = Number(e.dataTransfer?.getData('application/x-task-id'))
	if (id) data.updateTask(id, schedulePatch(null, false))
}

// ---- header popovers ----
const modeEl = ref<HTMLElement | null>(null)
const moreEl = ref<HTMLElement | null>(null)
const addEl = ref<HTMLElement | null>(null)
const pop = ref<'mode' | 'more' | 'add' | null>(null)
const modes = [
	{key: 'day', label: 'day'},
	{key: 'week', label: 'week'},
	{key: 'month', label: 'month'},
] as const

const addDefaults = computed(() => {
	const now = new Date()
	const day = now >= range.value.start && now < range.value.end ? now : range.value.start
	return {...schedulePatch(startOfDay(day), true), project_id: prefs.defaultListId || data.inboxId}
})
</script>

<template>
	<div class="cal-view">
		<aside v-if="prefs.calendarSidebar" class="cal-side">
			<MiniMonth :selected="anchor" :tasks="data.openTasks" :range-start="range.start" :range-end="mode === 'month' ? range.start : range.end" @pick="d => anchor = d" />
			<div class="side-sep" />
			<button class="side-row strong" @click="toggleAll">
				<span class="name">{{ t('all') }}</span>
				<span class="box" :class="{on: allVisible}"><Check v-if="allVisible" :size="12" /></span>
			</button>
			<div class="side-head">{{ t('lists') }}</div>
			<button v-for="p in allLists" :key="p.id" class="side-row" @click="toggleList(p.id)">
				<Inbox v-if="'inbox' in listName(p.id)" :size="15" class="ic" />
				<span v-else-if="listName(p.id).emoji" class="ic">{{ listName(p.id).emoji }}</span>
				<Menu v-else :size="15" class="ic" />
				<span class="name">{{ listName(p.id).name }}</span>
				<span v-if="hasColor(p)" class="cdot" :style="{background: projectColor(p)}" />
				<span class="box" :class="{on: !hidden.has(p.id)}"><Check v-if="!hidden.has(p.id)" :size="12" /></span>
			</button>
			<div class="side-head with-action">
				{{ t('calendars') }}
				<button class="side-add" :title="t('subscribeCalendar')" @click="openSubscribe"><Plus :size="15" /></button>
			</div>
			<div v-for="sub in calendars.subscriptions" :key="'s' + sub.id" class="side-row sub-row" role="button" tabindex="0" @click="toggleSub(sub.id)" @keydown.enter="toggleSub(sub.id)">
				<CalendarDays :size="15" class="ic" />
				<span class="name" :title="sub.title">{{ sub.title }}</span>
				<span v-if="calendars.errors.has(sub.id)" class="sub-error" :title="calendars.errors.get(sub.id) || t('calendarError')"><AlertCircle :size="15" /></span>
				<button class="sub-edit" :title="t('editCalendar')" @click.stop="subDialog = {subscription: sub}"><Pencil :size="13" /></button>
				<span class="cdot" :style="{background: calendarColor(sub)}" />
				<span class="box" :class="{on: !hiddenSubs.has(sub.id)}"><Check v-if="!hiddenSubs.has(sub.id)" :size="12" /></span>
			</div>
			<button v-if="!calendars.subscriptions.length" class="side-row add-sub" @click="openSubscribe">
				<Plus :size="15" class="ic" />
				<span class="name">{{ t('subscribeCalendar') }}</span>
			</button>
		</aside>

		<section class="cal-main">
			<header class="head">
				<button class="icon-btn" @click="prefs.calendarSidebar = !prefs.calendarSidebar"><PanelLeft :size="19" /></button>
				<h1>{{ title }}</h1>
				<div class="spacer" />
				<button ref="addEl" class="hbtn square" :title="t('addTask')" @click="pop = 'add'"><Plus :size="18" /></button>
				<button ref="modeEl" class="hbtn" @click="pop = 'mode'">{{ t(mode) }} <ChevronDown :size="14" /></button>
				<div class="nav-group">
					<button class="nav" @click="step(-1)"><ChevronLeft :size="16" /></button>
					<button class="nav today" @click="goToday">{{ t('today') }}</button>
					<button class="nav" @click="step(1)"><ChevronRight :size="16" /></button>
				</div>
				<button class="icon-btn" :class="{active: arrangeOpen}" :title="t('arrange')" @click="arrangeOpen = !arrangeOpen"><ListTodo :size="19" /></button>
				<button ref="moreEl" class="icon-btn" @click="pop = 'more'"><MoreHorizontal :size="19" /></button>
			</header>

			<div class="cal-body">
				<TimeGrid v-if="mode !== 'month'" :days="days" :tasks="visibleTasks" :events="visibleEvents" @open-event="openEvent" />
				<MonthGrid
					v-else
					:month="anchor"
					:tasks="visibleTasks"
					:events="visibleEvents"
					@day="openDay"
					@month="onMonthScrolled"
					@range="r => monthSpan = r"
					@open-event="openEvent"
				/>
			</div>
		</section>

		<aside v-if="arrangeOpen" class="arrange" @dragover.prevent @drop.prevent="onArrangeDrop">
			<div class="ar-head">{{ t('unscheduled') }} <span class="muted">{{ unscheduled.length }}</span></div>
			<div class="ar-hint">{{ t('dragHint') }}</div>
			<div class="ar-list">
				<div
					v-for="task in unscheduled"
					:key="task.id"
					class="ar-row"
					draggable="true"
					@dragstart="onArrangeDragStart($event, task)"
					@click="openTaskDrawer(task.id)"
				>
					<TaskCheckbox :done="task.done" :priority="task.priority" @toggle="data.toggleDone(task.id)" />
					<div class="ar-text">
						<div class="ar-title">{{ task.title }}</div>
						<div class="ar-list-name">{{ listName(task.project_id).emoji }} {{ listName(task.project_id).name }}</div>
					</div>
				</div>
			</div>
		</aside>

		<Popover v-if="pop === 'mode'" :anchor="modeEl" placement="bottom-end" @close="pop = null">
			<div class="menu">
				<button v-for="m in modes" :key="m.key" class="menu-item" @click="mode = m.key; pop = null">
					<Check :size="15" :style="{visibility: mode === m.key ? 'visible' : 'hidden'}" /> {{ t(m.label) }}
				</button>
			</div>
		</Popover>
		<Popover v-if="pop === 'more'" :anchor="moreEl" placement="bottom-end" @close="pop = null">
			<div class="menu">
				<button class="menu-item" @click="prefs.calendarShowCompleted = !prefs.calendarShowCompleted">
					<Check :size="15" :style="{visibility: prefs.calendarShowCompleted ? 'visible' : 'hidden'}" /> {{ t('showCompleted') }}
				</button>
				<button class="menu-item" @click="prefs.calendarShowWeekends = !prefs.calendarShowWeekends">
					<Check :size="15" :style="{visibility: prefs.calendarShowWeekends ? 'visible' : 'hidden'}" /> {{ t('showWeekends') }}
				</button>
				<button class="menu-item" @click="openSubscribe">
					<CalendarDays :size="15" /> {{ t('subscribeCalendar') }}
				</button>
			</div>
		</Popover>
		<Popover v-if="pop === 'add'" :anchor="addEl" placement="bottom-end" @close="pop = null">
			<div class="add-pop">
				<QuickAdd :defaults="addDefaults" @created="pop = null" />
			</div>
		</Popover>
		<Popover v-if="openedEvent" :anchor="{x: openedEvent.x, y: openedEvent.y}" @close="openedEvent = null">
			<div class="evp">
				<div class="evp-cal">
					<span class="cdot" :style="{background: openedEvent.event.color}" />
					{{ calendars.subscription(openedEvent.event.subscriptionId)?.title }}
				</div>
				<div class="evp-title">{{ openedEvent.event.title }}</div>
				<div class="evp-when">
					{{ eventWhen(openedEvent.event) }}
					<span v-if="openedEvent.event.recurring" class="evp-rec" :title="t('recurringEvent')"><Repeat :size="13" /></span>
				</div>
				<div v-if="openedEvent.event.location" class="evp-row"><MapPin :size="14" /> <span>{{ openedEvent.event.location }}</span></div>
				<div v-if="openedEvent.event.description" class="evp-desc">{{ openedEvent.event.description }}</div>
				<a v-if="safeLink(openedEvent.event.url)" class="evp-link" :href="safeLink(openedEvent.event.url)!" target="_blank" rel="noopener noreferrer">
					<ExternalLink :size="13" /> {{ t('openLink') }}
				</a>
			</div>
		</Popover>
		<SubscriptionDialog v-if="subDialog" :subscription="subDialog.subscription" @close="subDialog = null" />
	</div>
</template>

<style scoped>
.cal-view {
	display: flex;
	height: 100%;
}

.cal-side {
	width: 300px;
	flex-shrink: 0;
	padding: 16px 12px;
	border-right: 1px solid var(--border);
	overflow: auto;
}

.side-sep {
	height: 1px;
	background: var(--border);
	margin: 8px 4px 10px;
}

.side-head {
	padding: 10px 8px 4px;
	font-size: 13px;
	color: var(--text-3);
}

.side-head.with-action {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding-top: 16px;
}

.side-add {
	display: inline-flex;
	padding: 2px;
	border-radius: 4px;
	color: var(--text-3);
}

.side-add:hover {
	background: var(--bg-hover);
	color: var(--text);
}

.sub-row {
	cursor: pointer;
}

.sub-edit {
	display: none;
	padding: 2px;
	color: var(--text-3);
}

.sub-row:hover .sub-edit,
.sub-row:focus-within .sub-edit {
	display: inline-flex;
}

.sub-edit:hover {
	color: var(--text);
}

.sub-error {
	display: inline-flex;
	color: var(--danger);
	flex-shrink: 0;
}

.evp-rec {
	display: inline-flex;
}

.add-sub {
	color: var(--text-3);
}

.side-row {
	display: flex;
	align-items: center;
	gap: 10px;
	width: 100%;
	height: 36px;
	padding: 0 8px;
	border-radius: 6px;
	text-align: left;
}

.side-row:hover {
	background: var(--bg-hover);
}

.side-row.strong {
	font-weight: 600;
}

.side-row .ic {
	width: 16px;
	display: inline-flex;
	justify-content: center;
	color: var(--text-2);
}

.side-row .name {
	flex: 1;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.cdot {
	width: 8px;
	height: 8px;
	border-radius: 50%;
}

.box {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 16px;
	height: 16px;
	border-radius: 4px;
	border: 1.5px solid var(--text-4);
	color: #fff;
}

.box.on {
	background: var(--accent);
	border-color: var(--accent);
}

.cal-main {
	flex: 1;
	min-width: 0;
	display: flex;
	flex-direction: column;
}

.head {
	display: flex;
	align-items: center;
	gap: 8px;
	height: 64px;
	padding: 0 14px;
	flex-shrink: 0;
}

h1 {
	margin: 0 0 0 4px;
	font-size: 22px;
	font-weight: 700;
	white-space: nowrap;
}

.spacer {
	flex: 1;
}

.hbtn {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	height: 34px;
	padding: 0 14px;
	border-radius: 7px;
	border: 1px solid var(--border-strong);
	font-size: 14px;
}

.hbtn.square {
	width: 34px;
	padding: 0;
	justify-content: center;
}

.hbtn:hover {
	background: var(--bg-hover);
}

.nav-group {
	display: flex;
	height: 34px;
	border-radius: 7px;
	border: 1px solid var(--border-strong);
	overflow: hidden;
}

.nav {
	display: flex;
	align-items: center;
	justify-content: center;
	min-width: 34px;
	padding: 0 6px;
}

.nav.today {
	padding: 0 14px;
	border-left: 1px solid var(--border-strong);
	border-right: 1px solid var(--border-strong);
}

.nav:hover {
	background: var(--bg-hover);
}

.icon-btn.active {
	color: var(--accent);
}

.cal-body {
	flex: 1;
	min-height: 0;
}

.arrange {
	width: 280px;
	flex-shrink: 0;
	display: flex;
	flex-direction: column;
	border-left: 1px solid var(--border);
}

.ar-head {
	display: flex;
	gap: 8px;
	padding: 20px 16px 4px;
	font-weight: 600;
}

.ar-hint {
	padding: 0 16px 10px;
	font-size: 12px;
	color: var(--text-3);
}

.ar-list {
	flex: 1;
	overflow: auto;
	padding: 0 8px 12px;
}

.ar-row {
	display: flex;
	align-items: flex-start;
	gap: 10px;
	padding: 9px 8px;
	border-radius: 6px;
	cursor: grab;
}

.ar-row:hover {
	background: var(--bg-hover);
}

.ar-row :deep(.cb) {
	margin-top: 2px;
}

.ar-text {
	min-width: 0;
}

.ar-title {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.ar-list-name {
	font-size: 12px;
	color: var(--text-3);
}

.evp {
	width: 320px;
	padding: 14px 16px;
	background: var(--bg-elev);
	border-radius: 10px;
	box-shadow: var(--shadow);
	display: flex;
	flex-direction: column;
	gap: 8px;
	user-select: text;
}

.evp-cal {
	display: flex;
	align-items: center;
	gap: 8px;
	font-size: 12px;
	color: var(--text-3);
}

.evp-title {
	font-size: 16px;
	font-weight: 600;
	overflow-wrap: anywhere;
}

.evp-when {
	display: flex;
	align-items: center;
	gap: 6px;
	font-size: 13px;
	color: var(--text-2);
}

.evp-row {
	display: flex;
	align-items: flex-start;
	gap: 6px;
	font-size: 13px;
	color: var(--text-2);
	overflow-wrap: anywhere;
}

.evp-row svg {
	flex-shrink: 0;
	margin-top: 2px;
}

.evp-desc {
	max-height: 200px;
	overflow: auto;
	font-size: 13px;
	line-height: 1.45;
	white-space: pre-wrap;
	overflow-wrap: anywhere;
	color: var(--text-2);
	border-top: 1px solid var(--border);
	padding-top: 8px;
}

.evp-link {
	display: inline-flex;
	align-items: center;
	gap: 6px;
	font-size: 13px;
	color: var(--accent);
}

.add-pop {
	width: 380px;
	padding: 8px;
	background: var(--bg-elev);
	border-radius: 10px;
	box-shadow: var(--shadow);
}
</style>
