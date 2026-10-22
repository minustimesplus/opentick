<script setup lang="ts">
import {computed, nextTick, onMounted, ref} from 'vue'
import {Pause, Pencil, Play, Settings2, SkipForward, Square, Timer, Trash2, X} from 'lucide-vue-next'
import Popover from '@/components/Popover.vue'
import SettingToggle from '@/components/settings/SettingToggle.vue'
import type {FocusSession} from '@/api/types'
import {useData} from '@/stores/data'
import {fmtMinutes, useFocus} from '@/stores/focus'
import {dateKey, fmtRelativeDay, fmtTime, fmtWeekdayNarrow, parseDate} from '@/lib/dates'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {openTaskDrawer} from '@/lib/ui'

const data = useData()
const focus = useFocus()
const state = focus.state

onMounted(() => focus.load())

// --- Timer ---

const RADIUS = 120
const CIRCUMFERENCE = 2 * Math.PI * RADIUS
const dashOffset = computed(() => CIRCUMFERENCE * (1 - focus.progress))

const status = computed(() => {
	if (state.phase === 'break') return t(state.breakKind === 'long' ? 'longBreak' : 'shortBreak')
	if (state.phase === 'focus') return t(state.running ? 'focusing' : 'focusPaused')
	return t(state.mode === 'pomo' ? 'pomo' : 'stopwatch')
})

/** The clock shows the full pomo length before a pomo starts. */
const clock = computed(() => state.phase === 'idle' && state.mode === 'pomo'
	? `${String(prefs.focusPomoMinutes).padStart(2, '0')}:00`
	: focus.display)

const cycleDots = computed(() => Math.max(1, Math.min(12, prefs.focusLongBreakEvery)))

function end() {
	if (state.phase === 'focus' && state.mode === 'pomo' && !confirm(t('abandonPomoConfirm'))) return
	focus.stop()
}

// --- Task picker ---

const taskBtn = ref<HTMLElement | null>(null)
const picking = ref(false)
const query = ref('')
const queryInput = ref<HTMLInputElement | null>(null)

const linkedTask = computed(() => state.taskId ? data.tasks.get(state.taskId) : undefined)

const pickable = computed(() => {
	const q = query.value.trim().toLowerCase()
	return data.openTasks
		.filter(task => !q || task.title.toLowerCase().includes(q))
		.slice(0, 50)
})

async function openPicker() {
	query.value = ''
	picking.value = true
	await nextTick()
	queryInput.value?.focus()
}

function pick(id: number) {
	focus.setTask(id)
	picking.value = false
}

// --- Settings ---

const settingsBtn = ref<HTMLElement | null>(null)
const showSettings = ref(false)

function clampMinutes(value: number, max = 180) {
	return Math.max(1, Math.min(max, Math.round(Number(value) || 1)))
}

// --- Stats ---

const chartDays = computed(() => focus.stats?.history ?? [])
const chartMax = computed(() => Math.max(30, ...chartDays.value.map(d => d.focus_minutes)))

function dayLabel(date: string) {
	const d = parseDate(date + 'T00:00:00')
	if (!d) return ''
	return focus.historyDays <= 7 ? fmtWeekdayNarrow(d) : String(d.getDate())
}

/** Every day is labeled in the week view; the month view labels every fifth day back from today. */
function showDayLabel(i: number) {
	return focus.historyDays <= 7 || (chartDays.value.length - 1 - i) % 5 === 0
}

function dayTitle(date: string, minutes: number, pomos: number) {
	const d = parseDate(date + 'T00:00:00')
	const label = d ? fmtRelativeDay(d) : date
	return `${label}: ${fmtMinutes(minutes)} · ${pomos} ${t('pomosUnit')}`
}

// --- Records ---

interface RecordGroup {
	key: string
	label: string
	sessions: FocusSession[]
}

const groups = computed<RecordGroup[]>(() => {
	const out: RecordGroup[] = []
	for (const s of focus.sessions) {
		const start = parseDate(s.start_time)
		if (!start) continue
		const key = dateKey(start)
		let group = out.find(g => g.key === key)
		if (!group) {
			group = {key, label: fmtRelativeDay(start), sessions: []}
			out.push(group)
		}
		group.sessions.push(s)
	}
	return out
})

function timeRange(s: FocusSession) {
	const start = parseDate(s.start_time)
	const endAt = parseDate(s.end_time)
	return start && endAt ? `${fmtTime(start)} – ${fmtTime(endAt)}` : ''
}

function taskTitle(s: FocusSession) {
	return s.task_id ? data.tasks.get(s.task_id)?.title ?? '' : ''
}

const editingNote = ref<number | null>(null)
const noteDraft = ref('')

async function editNote(s: FocusSession) {
	editingNote.value = s.id
	noteDraft.value = s.note ?? ''
	await nextTick()
	document.querySelector<HTMLInputElement>('.note-input')?.focus()
}

async function saveNote(s: FocusSession) {
	if (editingNote.value !== s.id) return
	editingNote.value = null
	const note = noteDraft.value.trim()
	if (note !== (s.note ?? '')) await focus.updateSession(s.id, {note})
}

function removeRecord(s: FocusSession) {
	if (confirm(t('deleteFocusRecordConfirm'))) focus.removeSession(s.id)
}
</script>

<template>
	<div class="focus-view">
		<section class="timer-pane">
			<header class="head">
				<h1>{{ t('focus') }}</h1>
				<button ref="settingsBtn" class="icon-btn" :title="t('focusSettings')" @click="showSettings = true"><Settings2 :size="20" /></button>
			</header>

			<div class="timer">
				<div class="modes" role="tablist">
					<button
						v-for="m in (['pomo', 'stopwatch'] as const)"
						:key="m"
						role="tab"
						class="mode"
						:class="{active: state.mode === m}"
						:aria-selected="state.mode === m"
						:disabled="state.phase !== 'idle' && state.mode !== m"
						@click="focus.setMode(m)"
					>{{ t(m) }}</button>
				</div>

				<div class="task-chip-row">
					<button ref="taskBtn" class="task-chip" :class="{set: !!linkedTask}" @click="openPicker">
						<span class="task-chip-text">{{ linkedTask?.title || t('focusOnTask') }}</span>
					</button>
					<button v-if="linkedTask" class="icon-btn small" :title="t('noFocusTask')" @click="focus.setTask(0)"><X :size="14" /></button>
				</div>

				<div class="ring" :class="{break: state.phase === 'break', paused: state.phase !== 'idle' && !state.running}">
					<svg viewBox="0 0 260 260" aria-hidden="true">
						<circle class="track" cx="130" cy="130" :r="RADIUS" />
						<circle
							class="arc"
							cx="130"
							cy="130"
							:r="RADIUS"
							:stroke-dasharray="CIRCUMFERENCE"
							:stroke-dashoffset="dashOffset"
						/>
					</svg>
					<div class="ring-center">
						<div class="clock" role="timer" aria-live="off">{{ clock }}</div>
						<div class="status">{{ status }}</div>
					</div>
				</div>

				<div v-if="state.mode === 'pomo'" class="cycle" :title="t('longBreakEvery') + ' ' + prefs.focusLongBreakEvery + ' ' + t('pomosUnit')">
					<span v-for="i in cycleDots" :key="i" class="cycle-dot" :class="{done: i <= state.pomoCount}" />
				</div>

				<div class="controls">
					<template v-if="state.phase === 'idle'">
						<button class="btn primary big" @click="focus.startFocus()"><Play :size="16" /> {{ t('focusStart') }}</button>
					</template>
					<template v-else-if="state.phase === 'focus'">
						<button v-if="state.running" class="btn primary big" @click="focus.pause()"><Pause :size="16" /> {{ t('focusPause') }}</button>
						<template v-else>
							<button class="btn primary big" @click="focus.resume()"><Play :size="16" /> {{ t('focusContinue') }}</button>
							<button class="btn big" @click="end"><Square :size="14" /> {{ t('focusEnd') }}</button>
						</template>
					</template>
					<template v-else>
						<button v-if="!state.running" class="btn primary big" @click="focus.resume()"><Play :size="16" /> {{ t('startBreak') }}</button>
						<button class="btn big" @click="focus.stop()"><SkipForward :size="16" /> {{ t('skipBreak') }}</button>
					</template>
				</div>
			</div>
		</section>

		<aside class="stats-pane">
			<h2 class="section">{{ t('focusOverview') }}</h2>
			<div class="cards">
				<div class="card">
					<div class="num">{{ focus.stats?.today_pomos ?? 0 }}</div>
					<div class="label">{{ t('todayPomos') }}</div>
				</div>
				<div class="card">
					<div class="num">{{ fmtMinutes(focus.stats?.today_focus_minutes ?? 0) }}</div>
					<div class="label">{{ t('todayFocus') }}</div>
				</div>
				<div class="card">
					<div class="num">{{ focus.stats?.total_pomos ?? 0 }}</div>
					<div class="label">{{ t('totalPomos') }}</div>
				</div>
				<div class="card">
					<div class="num">{{ fmtMinutes(focus.stats?.total_focus_minutes ?? 0) }}</div>
					<div class="label">{{ t('totalFocus') }}</div>
				</div>
			</div>

			<div class="chart-card">
				<div class="chart-head">
					<h2 class="section inline">{{ t('focusTime') }}</h2>
					<div class="range">
						<button
							v-for="n in [7, 30]"
							:key="n"
							class="range-btn"
							:class="{active: focus.historyDays === n}"
							@click="focus.setHistoryDays(n)"
						>{{ t('lastNDays', {n}) }}</button>
					</div>
				</div>
				<div class="chart" :class="{dense: focus.historyDays > 7}">
					<div
						v-for="(d, i) in chartDays"
						:key="d.date"
						class="bar-col"
						:title="dayTitle(d.date, d.focus_minutes, d.pomos)"
					>
						<div class="bar-track">
							<div class="bar" :style="{height: (d.focus_minutes / chartMax * 100) + '%'}" />
						</div>
						<div class="bar-label">{{ showDayLabel(i) ? dayLabel(d.date) : '' }}</div>
					</div>
				</div>
			</div>

			<h2 class="section">{{ t('focusRecord') }}</h2>
			<div v-if="!focus.sessions.length" class="muted empty">{{ t('noFocusRecords') }}</div>
			<div v-for="g in groups" :key="g.key" class="record-group">
				<div class="group-label">{{ g.label }}</div>
				<div v-for="s in g.sessions" :key="s.id" class="record">
					<span class="record-icon" :class="s.type"><Timer :size="15" /></span>
					<div class="record-body">
						<div class="record-line">
							<span class="record-time">{{ timeRange(s) }}</span>
							<span class="record-duration">{{ fmtMinutes(Math.floor(s.duration / 60)) }}</span>
						</div>
						<button v-if="taskTitle(s)" class="record-task" @click="openTaskDrawer(s.task_id)">{{ taskTitle(s) }}</button>
						<input
							v-if="editingNote === s.id"
							v-model="noteDraft"
							class="input note-input"
							:placeholder="t('focusNotePlaceholder')"
							maxlength="2000"
							@keydown.enter="saveNote(s)"
							@keydown.esc="editingNote = null"
							@blur="saveNote(s)"
						>
						<button v-else-if="s.note" class="record-note" @click="editNote(s)">{{ s.note }}</button>
					</div>
					<button class="icon-btn small hover-action" :title="t('focusNotePlaceholder')" @click="editNote(s)"><Pencil :size="14" /></button>
					<button class="icon-btn small hover-action" :title="t('delete')" @click="removeRecord(s)"><Trash2 :size="14" /></button>
				</div>
			</div>
		</aside>

		<Popover v-if="picking" :anchor="taskBtn" @close="picking = false">
			<div class="menu picker">
				<input ref="queryInput" v-model="query" class="input" :placeholder="t('searchPlaceholder')">
				<div class="picker-list">
					<button class="menu-item" :class="{active: !state.taskId}" @click="pick(0)">{{ t('noFocusTask') }}</button>
					<button
						v-for="task in pickable"
						:key="task.id"
						class="menu-item"
						:class="{active: state.taskId === task.id}"
						@click="pick(task.id)"
					><span class="picker-title">{{ task.title }}</span></button>
				</div>
			</div>
		</Popover>

		<Popover v-if="showSettings" :anchor="settingsBtn" placement="bottom-end" @close="showSettings = false">
			<div class="menu settings">
				<div class="menu-label">{{ t('focusSettings') }}</div>
				<label class="setting">
					<span>{{ t('pomoLength') }}</span>
					<span class="setting-value">
						<input class="input num-input" type="number" min="1" max="180" :value="prefs.focusPomoMinutes" @change="prefs.focusPomoMinutes = clampMinutes(+($event.target as HTMLInputElement).value)">
						{{ t('minutesUnit') }}
					</span>
				</label>
				<label class="setting">
					<span>{{ t('shortBreak') }}</span>
					<span class="setting-value">
						<input class="input num-input" type="number" min="1" max="60" :value="prefs.focusShortBreakMinutes" @change="prefs.focusShortBreakMinutes = clampMinutes(+($event.target as HTMLInputElement).value, 60)">
						{{ t('minutesUnit') }}
					</span>
				</label>
				<label class="setting">
					<span>{{ t('longBreak') }}</span>
					<span class="setting-value">
						<input class="input num-input" type="number" min="1" max="60" :value="prefs.focusLongBreakMinutes" @change="prefs.focusLongBreakMinutes = clampMinutes(+($event.target as HTMLInputElement).value, 60)">
						{{ t('minutesUnit') }}
					</span>
				</label>
				<label class="setting">
					<span>{{ t('longBreakEvery') }}</span>
					<span class="setting-value">
						<input class="input num-input" type="number" min="1" max="12" :value="prefs.focusLongBreakEvery" @change="prefs.focusLongBreakEvery = clampMinutes(+($event.target as HTMLInputElement).value, 12)">
						{{ t('pomosUnit') }}
					</span>
				</label>
				<div class="menu-sep" />
				<div class="setting">
					<span>{{ t('autoStartBreak') }}</span>
					<SettingToggle v-model="prefs.focusAutoStartBreak" />
				</div>
				<div class="setting">
					<span>{{ t('autoStartPomo') }}</span>
					<SettingToggle v-model="prefs.focusAutoStartPomo" />
				</div>
			</div>
		</Popover>
	</div>
</template>

<style scoped>
.focus-view {
	display: flex;
	height: 100%;
	min-height: 0;
}

.timer-pane {
	display: flex;
	flex-direction: column;
	flex: 1;
	min-width: 0;
	padding: 0 24px 24px;
}

.head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	height: 72px;
	padding: 0 4px;
	flex-shrink: 0;
}

h1 {
	margin: 0;
	font-size: 22px;
	font-weight: 700;
}

.timer {
	display: flex;
	flex: 1;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 22px;
	padding-bottom: 40px;
}

.modes {
	display: flex;
	padding: 3px;
	border-radius: 8px;
	background: var(--bg-panel);
}

.mode {
	height: 28px;
	padding: 0 16px;
	border-radius: 6px;
	font-size: 13px;
	color: var(--text-2);
}

.mode.active {
	background: var(--bg-elev);
	color: var(--text);
	font-weight: 600;
	box-shadow: var(--shadow);
}

.mode:disabled {
	opacity: 0.4;
}

.task-chip-row {
	display: flex;
	align-items: center;
	gap: 2px;
	max-width: min(360px, 100%);
}

.task-chip {
	min-width: 0;
	height: 30px;
	padding: 0 12px;
	border-radius: 15px;
	color: var(--text-3);
	font-size: 14px;
}

.task-chip:hover {
	background: var(--bg-hover);
}

.task-chip.set {
	color: var(--text);
}

.task-chip-text {
	display: block;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.icon-btn.small {
	width: 24px;
	height: 24px;
}

.ring {
	position: relative;
	width: min(300px, 70vw);
	aspect-ratio: 1;
	--ring-color: var(--accent);
}

.ring.break {
	--ring-color: var(--success, #2fb36f);
}

.ring svg {
	width: 100%;
	height: 100%;
	transform: rotate(-90deg);
}

.track {
	fill: none;
	stroke: var(--bg-panel);
	stroke-width: 8;
}

.arc {
	fill: none;
	stroke: var(--ring-color);
	stroke-width: 8;
	stroke-linecap: round;
	transition: stroke-dashoffset 0.9s linear;
}

.ring.paused .arc {
	opacity: 0.5;
}

.ring-center {
	position: absolute;
	inset: 0;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 6px;
}

.clock {
	font-size: clamp(44px, 12vw, 60px);
	font-weight: 300;
	font-variant-numeric: tabular-nums;
	letter-spacing: 1px;
}

.status {
	font-size: 14px;
	color: var(--text-3);
}

.cycle {
	display: flex;
	gap: 8px;
	margin-top: -6px;
}

.cycle-dot {
	width: 8px;
	height: 8px;
	border-radius: 50%;
	background: var(--border-strong);
}

.cycle-dot.done {
	background: var(--accent);
}

.controls {
	display: flex;
	gap: 12px;
}

.btn.big {
	height: 40px;
	min-width: 132px;
	justify-content: center;
	border-radius: 20px;
	font-size: 15px;
}

.stats-pane {
	width: 360px;
	flex-shrink: 0;
	overflow: auto;
	padding: 20px 20px 24px;
	border-left: 1px solid var(--border);
}

.section {
	margin: 0 0 10px;
	font-size: 13px;
	font-weight: 600;
	color: var(--text-3);
}

.section.inline {
	margin: 0;
}

.cards {
	display: grid;
	grid-template-columns: repeat(2, 1fr);
	gap: 10px;
	margin-bottom: 20px;
}

.card,
.chart-card {
	padding: 14px;
	border-radius: 10px;
	background: var(--bg-panel);
}

.num {
	font-size: 22px;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}

.label {
	margin-top: 2px;
	font-size: 12px;
	color: var(--text-3);
}

.chart-card {
	margin-bottom: 20px;
}

.chart-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	margin-bottom: 12px;
}

.range {
	display: flex;
	gap: 2px;
}

.range-btn {
	height: 24px;
	padding: 0 8px;
	border-radius: 5px;
	font-size: 12px;
	color: var(--text-3);
}

.range-btn.active {
	background: var(--bg-hover);
	color: var(--text);
}

.chart {
	display: flex;
	align-items: stretch;
	gap: 6px;
	height: 120px;
}

.chart.dense {
	gap: 2px;
}

.bar-col {
	display: flex;
	flex: 1;
	flex-direction: column;
	min-width: 0;
}

.bar-track {
	display: flex;
	flex: 1;
	align-items: flex-end;
	justify-content: center;
	border-bottom: 1px solid var(--border);
}

.bar {
	width: min(100%, 18px);
	min-height: 0;
	border-radius: 4px 4px 0 0;
	background: var(--accent);
}

.bar-col:hover .bar {
	filter: brightness(1.15);
}

.bar-label {
	height: 18px;
	padding-top: 4px;
	font-size: 11px;
	color: var(--text-3);
	text-align: center;
	white-space: nowrap;
}

.empty {
	padding: 8px 2px;
	font-size: 13px;
}

.record-group {
	margin-bottom: 14px;
}

.group-label {
	margin-bottom: 4px;
	font-size: 12px;
	color: var(--text-3);
}

.record {
	display: flex;
	align-items: flex-start;
	gap: 10px;
	padding: 8px 6px;
	border-radius: 8px;
}

.record:hover {
	background: var(--bg-hover);
}

.record-icon {
	display: flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	width: 26px;
	height: 26px;
	border-radius: 50%;
	color: var(--accent);
	background: var(--accent-soft);
}

.record-icon.stopwatch {
	color: var(--text-2);
	background: var(--bg-panel);
}

.record-body {
	display: flex;
	flex: 1;
	flex-direction: column;
	align-items: flex-start;
	gap: 2px;
	min-width: 0;
}

.record-line {
	display: flex;
	justify-content: space-between;
	width: 100%;
	font-size: 13px;
}

.record-duration {
	font-variant-numeric: tabular-nums;
	color: var(--text-2);
}

.record-task {
	max-width: 100%;
	overflow: hidden;
	font-size: 13px;
	color: var(--accent);
	text-align: left;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.record-note {
	max-width: 100%;
	font-size: 12px;
	color: var(--text-2);
	text-align: left;
	overflow-wrap: anywhere;
}

.note-input {
	height: 28px;
	font-size: 12px;
}

.record .hover-action {
	flex-shrink: 0;
	opacity: 0;
}

.record:hover .hover-action,
.record .hover-action:focus-visible {
	opacity: 1;
}

.picker {
	width: min(320px, calc(100vw - 16px));
}

.picker-list {
	max-height: 320px;
	margin-top: 6px;
	overflow: auto;
}

.picker-title {
	overflow: hidden;
	text-overflow: ellipsis;
}

.settings {
	width: 300px;
}

.setting {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	padding: 5px 10px;
	font-size: 13px;
}

.setting-value {
	display: flex;
	align-items: center;
	gap: 6px;
	color: var(--text-3);
	white-space: nowrap;
}

.num-input {
	width: 60px;
	height: 28px;
	color: var(--text);
}

@media (hover: none) {
	.record .hover-action {
		opacity: 1;
	}
}

@media (max-width: 1000px) {
	.focus-view {
		flex-direction: column;
		overflow: auto;
	}

	.timer-pane {
		flex: none;
	}

	.stats-pane {
		width: auto;
		overflow: visible;
		border-left: none;
		border-top: 1px solid var(--border);
	}
}

@media (max-width: 767px) {
	.timer-pane {
		padding: 0 10px 10px;
	}

	.head {
		height: 56px;
	}

	.timer {
		gap: 18px;
		padding-bottom: 16px;
	}

	.stats-pane {
		padding: 16px 12px 20px;
	}
}
</style>
