<script setup lang="ts">
import {computed, ref} from 'vue'
import {AlarmClock, CalendarPlus, ChevronLeft, ChevronRight, Clock, Moon, Repeat, Sun, Sunrise, X} from 'lucide-vue-next'
import type {Reminder, Task} from '@/api/types'
import {
	addDays, addMonths, dateKey, fmtMonthYear, fmtWeekdayNarrow, getSchedule, isSameDay, minutesOfDay,
	schedulePatch, startOfDay, startOfMonth, startOfWeek, withMinutes,
} from '@/lib/dates'
import {t, type MessageKey} from '@/lib/i18n'
import type {TaskPatch} from '@/stores/data'

const props = defineProps<{task?: Task | null}>()
const emit = defineEmits<{apply: [TaskPatch], close: []}>()

const s = props.task ? getSchedule(props.task) : null
const today = startOfDay(new Date())

const tab = ref<'date' | 'duration'>(s?.end && !s.allDay ? 'duration' : 'date')
const day = ref<Date | null>(s ? startOfDay(s.date) : null)
const month = ref<Date>(startOfMonth(s?.date ?? today))
const toHHMM = (d: Date) => `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
const time = ref(s && !s.allDay && !s.end ? toHHMM(s.date) : '')
const startTime = ref(s && !s.allDay ? toHHMM(s.start) : nextHalfHour())
const endTime = ref(s?.end && !s.allDay ? toHHMM(s.end) : addHour(startTime.value))
const endDay = ref<Date | null>(s?.end ? startOfDay(s.end) : null)

function nextHalfHour() {
	const now = new Date()
	const m = Math.ceil((minutesOfDay(now) + 1) / 30) * 30
	return `${String(Math.floor(m / 60) % 24).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
}

function addHour(hhmm: string) {
	const [h, m] = hhmm.split(':').map(Number)
	return `${String(Math.min(23, h + 1)).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}

function parseHHMM(v: string): number | null {
	const m = v.match(/^(\d{1,2}):(\d{2})$/)
	return m ? parseInt(m[1], 10) * 60 + parseInt(m[2], 10) : null
}

// ---- reminders ----
type RemKey = 'none' | 'ontime' | 'm5' | 'm30' | 'h1' | 'd1' | 'day9' | 'dayBefore9'
const ALL_DAY_9 = -(14 * 3600 + 59 * 60)
const REM: Record<RemKey, {period: number, label: MessageKey}> = {
	none: {period: NaN, label: 'remNone'},
	ontime: {period: 0, label: 'remOnTime'},
	m5: {period: -300, label: 'remBefore5m'},
	m30: {period: -1800, label: 'remBefore30m'},
	h1: {period: -3600, label: 'remBefore1h'},
	d1: {period: -86400, label: 'remBefore1d'},
	day9: {period: ALL_DAY_9, label: 'remDay9'},
	dayBefore9: {period: ALL_DAY_9 - 86400, label: 'remDayBefore9'},
}

const isAllDay = computed(() => tab.value === 'date' && !time.value)
const remOptions = computed<RemKey[]>(() => isAllDay.value ? ['none', 'day9', 'dayBefore9'] : ['none', 'ontime', 'm5', 'm30', 'h1', 'd1'])

function initialReminder(): RemKey {
	const r = props.task?.reminders?.[0]
	if (!r || r.relative_to !== 'due_date' && r.relative_to !== 'start_date') return 'none'
	const found = (Object.keys(REM) as RemKey[]).find(k => REM[k].period === r.relative_period)
	return found ?? 'none'
}
const reminder = ref<RemKey>(initialReminder())
const extraReminders: Reminder[] = (props.task?.reminders ?? []).slice(1)

// ---- repeat ----
type RepKey = 'none' | 'daily' | 'weekly' | 'biweekly' | 'monthly' | 'yearly'
const REP: Record<RepKey, {after: number, mode: number, label: MessageKey}> = {
	none: {after: 0, mode: 0, label: 'repNone'},
	daily: {after: 86400, mode: 0, label: 'repDaily'},
	weekly: {after: 604800, mode: 0, label: 'repWeekly'},
	biweekly: {after: 1209600, mode: 0, label: 'repBiweekly'},
	monthly: {after: 0, mode: 1, label: 'repMonthly'},
	yearly: {after: 31536000, mode: 0, label: 'repYearly'},
}
function initialRepeat(): RepKey {
	if (props.task?.repeat_mode === 1) return 'monthly'
	const after = props.task?.repeat_after ?? 0
	return (Object.keys(REP) as RepKey[]).find(k => k !== 'monthly' && REP[k].after === after) ?? 'none'
}
const repeat = ref<RepKey>(initialRepeat())

// ---- calendar grid ----
const weeks = computed(() => {
	const first = startOfWeek(month.value)
	return Array.from({length: 6}, (_, w) => Array.from({length: 7}, (_, d) => addDays(first, w * 7 + d)))
})
const weekdayLabels = computed(() => weeks.value[0].map(d => fmtWeekdayNarrow(d)))

function pick(d: Date) {
	day.value = d
	endDay.value = null
	if (d.getMonth() !== month.value.getMonth()) month.value = startOfMonth(d)
}

function quick(kind: 'today' | 'tomorrow' | 'week' | 'month' | 'tonight') {
	if (kind === 'today') day.value = today
	if (kind === 'tomorrow') day.value = addDays(today, 1)
	if (kind === 'week') day.value = addDays(startOfWeek(today, 1), 7)
	if (kind === 'month') day.value = addMonths(today, 1)
	if (kind === 'tonight') {
		day.value = today
		tab.value = 'date'
		time.value = '20:00'
	}
	month.value = startOfMonth(day.value!)
	if (kind !== 'tonight') apply()
}

function buildPatch(): TaskPatch {
	const d = day.value
	let sched
	if (!d) {
		sched = schedulePatch(null, false)
	} else if (tab.value === 'duration') {
		const sm = parseHHMM(startTime.value) ?? 9 * 60
		let em = parseHHMM(endTime.value) ?? sm + 60
		const start = withMinutes(d, sm)
		let end = withMinutes(endDay.value ?? d, em)
		if (end <= start) {
			em = sm + 60
			end = withMinutes(d, em)
		}
		sched = schedulePatch(start, false, end)
	} else if (time.value) {
		sched = schedulePatch(withMinutes(d, parseHHMM(time.value) ?? 0), false)
	} else {
		sched = schedulePatch(d, true)
	}
	const rem = REM[reminder.value]
	let reminders: Reminder[] = extraReminders
	if (d && reminder.value !== 'none' && remOptions.value.includes(reminder.value)) {
		reminders = [{relative_to: 'due_date', relative_period: rem.period}, ...extraReminders]
	} else if (!d) {
		reminders = []
	}
	const rep = d ? REP[repeat.value] : REP.none
	return {...sched, reminders, repeat_after: rep.after, repeat_mode: rep.mode}
}

function apply() {
	emit('apply', buildPatch())
	emit('close')
}

function clear() {
	emit('apply', {...schedulePatch(null, false), reminders: [], repeat_after: 0, repeat_mode: 0})
	emit('close')
}
</script>

<template>
	<div class="dp" @keydown.enter.prevent="apply">
		<div class="tabs">
			<button :class="{on: tab === 'date'}" @click="tab = 'date'">{{ t('date') }}</button>
			<button :class="{on: tab === 'duration'}" @click="tab = 'duration'">{{ t('duration') }}</button>
		</div>

		<div class="quick">
			<button :title="t('today')" @click="quick('today')"><Sun :size="18" /></button>
			<button :title="t('tomorrow')" @click="quick('tomorrow')"><Sunrise :size="18" /></button>
			<button :title="t('nextWeek')" @click="quick('week')"><span class="plus7">+7</span></button>
			<button :title="t('nextMonth')" @click="quick('month')"><CalendarPlus :size="18" /></button>
			<button title="20:00" @click="quick('tonight')"><Moon :size="18" /></button>
		</div>

		<div class="cal-head">
			<span class="month">{{ fmtMonthYear(month) }}</span>
			<div>
				<button class="icon-btn sm" @click="month = addMonths(month, -1)"><ChevronLeft :size="16" /></button>
				<button class="icon-btn sm" @click="month = startOfMonth(today)">•</button>
				<button class="icon-btn sm" @click="month = addMonths(month, 1)"><ChevronRight :size="16" /></button>
			</div>
		</div>
		<div class="grid">
			<span v-for="(w, i) in weekdayLabels" :key="'w' + i" class="wd">{{ w }}</span>
			<template v-for="week in weeks" :key="dateKey(week[0])">
				<button
					v-for="d in week"
					:key="dateKey(d)"
					class="day"
					:class="{other: d.getMonth() !== month.getMonth(), today: isSameDay(d, today), sel: isSameDay(d, day)}"
					@click="pick(d)"
					@dblclick="pick(d); apply()"
				>{{ d.getDate() }}</button>
			</template>
		</div>

		<div class="rows">
			<template v-if="tab === 'date'">
				<label class="row">
					<Clock :size="16" />
					<span class="row-label">{{ t('time') }}</span>
					<input v-model="time" type="time" class="time" step="300">
					<button v-if="time" class="x" @click.prevent="time = ''"><X :size="14" /></button>
				</label>
			</template>
			<template v-else>
				<label class="row">
					<Clock :size="16" />
					<span class="row-label">{{ t('start') }}</span>
					<input v-model="startTime" type="time" class="time" step="300" @change="parseHHMM(endTime)! <= parseHHMM(startTime)! && (endTime = addHour(startTime))">
				</label>
				<label class="row">
					<span style="width:16px" />
					<span class="row-label">{{ t('end') }}</span>
					<input v-model="endTime" type="time" class="time" step="300">
				</label>
			</template>
			<label class="row">
				<AlarmClock :size="16" />
				<span class="row-label">{{ t('reminder') }}</span>
				<select v-model="reminder" class="sel">
					<option v-for="k in remOptions" :key="k" :value="k">{{ t(REM[k].label) }}</option>
				</select>
			</label>
			<label class="row">
				<Repeat :size="16" />
				<span class="row-label">{{ t('repeat') }}</span>
				<select v-model="repeat" class="sel">
					<option v-for="(v, k) in REP" :key="k" :value="k">{{ t(v.label) }}</option>
				</select>
			</label>
		</div>

		<div class="actions">
			<button class="btn" @click="clear">{{ t('clear') }}</button>
			<button class="btn primary" @click="apply">{{ t('ok') }}</button>
		</div>
	</div>
</template>

<style scoped>
.dp {
	width: 272px;
	padding: 12px;
	background: var(--bg-elev);
	border-radius: 10px;
	box-shadow: var(--shadow);
	user-select: none;
}

.tabs {
	display: flex;
	padding: 3px;
	border-radius: 7px;
	background: var(--bg-input);
}

.tabs button {
	flex: 1;
	height: 26px;
	border-radius: 5px;
	color: var(--text-3);
	font-size: 13px;
}

.tabs button.on {
	background: var(--bg-active);
	color: var(--text);
}

.quick {
	display: flex;
	justify-content: space-around;
	margin: 10px 0 4px;
}

.quick button {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 36px;
	height: 32px;
	border-radius: 6px;
	color: var(--text-2);
}

.quick button:hover {
	background: var(--bg-hover);
	color: var(--accent);
}

.plus7 {
	font-size: 12px;
	font-weight: 700;
	border: 1.5px solid currentColor;
	border-radius: 4px;
	padding: 0 2px;
}

.cal-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	margin: 6px 2px;
}

.month {
	font-weight: 600;
	text-transform: capitalize;
}

.icon-btn.sm {
	width: 24px;
	height: 24px;
}

.grid {
	display: grid;
	grid-template-columns: repeat(7, 1fr);
	row-gap: 2px;
	text-align: center;
}

.wd {
	font-size: 11px;
	color: var(--text-3);
	padding: 4px 0;
	text-transform: uppercase;
}

.day {
	height: 30px;
	width: 30px;
	margin: 0 auto;
	border-radius: 50%;
	font-size: 13px;
}

.day:hover {
	background: var(--bg-hover);
}

.day.other {
	color: var(--text-4);
}

.day.today {
	color: var(--accent);
	font-weight: 700;
}

.day.sel {
	background: var(--accent);
	color: #fff;
}

.rows {
	margin-top: 8px;
	border-top: 1px solid var(--border);
	padding-top: 6px;
}

.row {
	display: flex;
	align-items: center;
	gap: 10px;
	height: 34px;
	color: var(--text-2);
}

.row-label {
	flex: 1;
}

.time,
.sel {
	height: 28px;
	padding: 0 6px;
	border-radius: 5px;
	border: 1px solid var(--border-strong);
	background: var(--bg-input);
	color: var(--text);
	outline: none;
	max-width: 150px;
}

.x {
	color: var(--text-3);
	display: flex;
}

.actions {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	margin-top: 10px;
}

.actions .btn {
	height: 30px;
	padding: 0 18px;
}
</style>
