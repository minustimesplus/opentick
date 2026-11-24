<script setup lang="ts">
import {computed, nextTick, onMounted, ref} from 'vue'
import {Check, Ellipsis, Expand, Headphones, Minus, Pause, Pencil, PictureInPicture2, Play, Plus, Settings2, SkipForward, Square, Timer, Trash2, X} from 'lucide-vue-next'
import Popover from '@/components/Popover.vue'
import SettingToggle from '@/components/settings/SettingToggle.vue'
import FocusFullscreen from '@/components/focus/FocusFullscreen.vue'
import FocusRecordDialog from '@/components/focus/FocusRecordDialog.vue'
import type {FocusSession} from '@/api/types'
import {useData} from '@/stores/data'
import {fmtMinutes, useFocus} from '@/stores/focus'
import {dateKey, fmtDayMonth, fmtRelativeDay, fmtTime, fmtWeekdayNarrow, parseDate} from '@/lib/dates'
import {FOCUS_SOUNDS, type FocusSound, prefs} from '@/lib/prefs'
import {type MessageKey, t} from '@/lib/i18n'
import {openTaskDrawer} from '@/lib/ui'
import {openPopout, popout, popoutStaysOnTop} from '@/lib/focusPopout'
import {toast} from '@/lib/toast'

const data = useData()
const focus = useFocus()
const state = focus.state

onMounted(() => {
	focus.load()
	focus.loadYear()
})

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
const clock = computed(() => state.phase === 'idle' && focus.phaseLength !== null
	? `${String(Math.round(focus.phaseLength / 60_000)).padStart(2, '0')}:00`
	: focus.display)

/** Countdowns (not the stopwatch) get -/+ buttons beside the clock. */
const ADJUST_STEP = 5
const adjustable = computed(() => focus.phaseLength !== null)

const cycleDots = computed(() => Math.max(1, Math.min(12, prefs.focusLongBreakEvery)))

const fullscreen = ref(false)

function start() {
	focus.startFocus()
	if (prefs.focusAutoFullscreen) fullscreen.value = true
}

// --- More menu ---

const moreBtn = ref<HTMLElement | null>(null)
const showMore = ref(false)
const addingRecord = ref(false)

async function openMini() {
	showMore.value = false
	if (!await openPopout()) toast(t('miniWindowBlocked'))
	else if (!popoutStaysOnTop()) toast(t('miniWindowNotOnTop'))
}

function openFullscreen() {
	showMore.value = false
	fullscreen.value = true
}

// --- White noise ---

const soundBtn = ref<HTMLElement | null>(null)
const showSound = ref(false)
const SOUND_LABELS: Record<FocusSound, MessageKey> = {
	none: 'none',
	white: 'soundWhite',
	pink: 'soundPink',
	brown: 'soundBrown',
	rain: 'soundRain',
	ocean: 'soundOcean',
	ticking: 'soundTicking',
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

/** Goals may be 0 (no goal). */
function clampGoal(value: number, max: number) {
	return Math.max(0, Math.min(max, Math.round(Number(value) || 0)))
}

// --- Goals ---

function goalPercent(done: number, goal: number) {
	return goal > 0 ? Math.min(100, Math.round(done / goal * 100)) : 0
}

// --- Focus ranking (by task, over the chart's range) ---

const ranking = computed(() => {
	const byTask = new Map<number, number>()
	for (const s of focus.rangeSessions) byTask.set(s.task_id, (byTask.get(s.task_id) ?? 0) + s.duration)
	const rows = [...byTask.entries()]
		.map(([id, seconds]) => ({
			id,
			title: id ? data.tasks.get(id)?.title ?? '…' : t('noFocusTask'),
			minutes: Math.floor(seconds / 60),
		}))
		.filter(r => r.minutes > 0)
		.sort((a, b) => b.minutes - a.minutes)
		.slice(0, 8)
	const max = Math.max(1, ...rows.map(r => r.minutes))
	return rows.map(r => ({...r, share: r.minutes / max}))
})

// --- Year grid: one cell per day, a column per week ---

const yearCells = computed(() => {
	const days = focus.yearHistory
	if (!days.length) return []
	const first = parseDate(days[0].date + 'T00:00:00')
	const lead = first ? (first.getDay() - prefs.weekStart + 7) % 7 : 0
	return [
		...Array.from({length: lead}, (_, i) => ({key: `pad${i}`, level: -1, title: ''})),
		...days.map(d => {
			const date = parseDate(d.date + 'T00:00:00')
			const m = d.focus_minutes
			return {
				key: d.date,
				level: m <= 0 ? 0 : m < 30 ? 1 : m < 60 ? 2 : m < 120 ? 3 : 4,
				title: t('yearGridDay', {date: date ? fmtDayMonth(date) : d.date, time: fmtMinutes(m)}),
			}
		}),
	]
})

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
				<div class="head-actions">
					<button ref="soundBtn" class="icon-btn" :class="{on: prefs.focusSound !== 'none'}" :title="t('whiteNoise')" @click="showSound = true"><Headphones :size="19" /></button>
					<button ref="moreBtn" class="icon-btn" :title="t('focusMoreActions')" @click="showMore = true"><Ellipsis :size="20" /></button>
					<button ref="settingsBtn" class="icon-btn" :title="t('focusSettings')" @click="showSettings = true"><Settings2 :size="20" /></button>
				</div>
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
						<div class="clock-row">
							<button
								v-if="adjustable"
								class="adjust"
								:disabled="!focus.canAdjust(-ADJUST_STEP)"
								:title="t('focusRemoveMinutes', {n: ADJUST_STEP})"
								@click="focus.adjust(-ADJUST_STEP)"
							><Minus :size="16" /></button>
							<div class="clock" role="timer" aria-live="off">{{ clock }}</div>
							<button
								v-if="adjustable"
								class="adjust"
								:disabled="!focus.canAdjust(ADJUST_STEP)"
								:title="t('focusAddMinutes', {n: ADJUST_STEP})"
								@click="focus.adjust(ADJUST_STEP)"
							><Plus :size="16" /></button>
						</div>
						<div class="status">{{ status }}</div>
					</div>
				</div>

				<div v-if="state.mode === 'pomo'" class="cycle" :title="t('longBreakEvery') + ' ' + prefs.focusLongBreakEvery + ' ' + t('pomosUnit')">
					<span v-for="i in cycleDots" :key="i" class="cycle-dot" :class="{done: i <= state.pomoCount}" />
				</div>

				<div class="controls">
					<template v-if="state.phase === 'idle'">
						<button class="btn primary big" @click="start"><Play :size="16" /> {{ t('focusStart') }}</button>
					</template>
					<template v-else-if="state.phase === 'focus'">
						<button v-if="state.running" class="btn primary big" @click="focus.pause()"><Pause :size="16" /> {{ t('focusPause') }}</button>
						<template v-else>
							<button class="btn primary big" @click="focus.resume()"><Play :size="16" /> {{ t('focusContinue') }}</button>
							<button class="btn big" @click="focus.requestEnd()"><Square :size="14" /> {{ t('focusEnd') }}</button>
						</template>
					</template>
					<template v-else>
						<button v-if="!state.running" class="btn primary big" @click="focus.resume()"><Play :size="16" /> {{ t('startBreak') }}</button>
						<button class="btn big" @click="focus.stop()"><SkipForward :size="16" /> {{ t('skipBreak') }}</button>
					</template>
				</div>

				<label v-if="state.phase === 'focus'" class="note">
					<span class="note-label">{{ t('focusNote') }}</span>
					<textarea v-model="state.note" class="input note-area" rows="2" maxlength="2000" :placeholder="t('focusNoteHint')" />
				</label>
			</div>
		</section>

		<aside class="stats-pane">
			<h2 class="section">{{ t('focusOverview') }}</h2>
			<div class="cards">
				<div class="card">
					<div class="num">{{ focus.stats?.today_pomos ?? 0 }}<span v-if="prefs.focusGoalPomos" class="goal">{{ t('goalOf', {goal: prefs.focusGoalPomos}) }}</span></div>
					<div class="label">{{ t('todayPomos') }}</div>
					<div v-if="prefs.focusGoalPomos" class="goal-bar"><div :style="{width: goalPercent(focus.stats?.today_pomos ?? 0, prefs.focusGoalPomos) + '%'}" /></div>
				</div>
				<div class="card">
					<div class="num">{{ fmtMinutes(focus.stats?.today_focus_minutes ?? 0) }}<span v-if="prefs.focusGoalMinutes" class="goal">{{ t('goalOf', {goal: fmtMinutes(prefs.focusGoalMinutes)}) }}</span></div>
					<div class="label">{{ t('todayFocus') }}</div>
					<div v-if="prefs.focusGoalMinutes" class="goal-bar"><div :style="{width: goalPercent(focus.stats?.today_focus_minutes ?? 0, prefs.focusGoalMinutes) + '%'}" /></div>
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

			<div class="chart-card">
				<h2 class="section">{{ t('focusRanking') }} · {{ t('lastNDays', {n: focus.historyDays}) }}</h2>
				<div v-if="!ranking.length" class="muted empty">{{ t('noFocusInRange') }}</div>
				<div v-for="r in ranking" :key="r.id" class="rank">
					<button v-if="r.id" class="rank-title" @click="openTaskDrawer(r.id)">{{ r.title }}</button>
					<span v-else class="rank-title muted">{{ r.title }}</span>
					<span class="rank-minutes">{{ fmtMinutes(r.minutes) }}</span>
					<div class="rank-bar"><div :style="{width: (r.share * 100) + '%'}" /></div>
				</div>
			</div>

			<div v-if="yearCells.length" class="chart-card">
				<h2 class="section">{{ t('yearGrid') }}</h2>
				<div class="year-grid">
					<span v-for="c in yearCells" :key="c.key" class="cell" :class="'l' + c.level" :title="c.title" />
				</div>
			</div>

			<div class="records-head">
				<h2 class="section inline">{{ t('focusRecord') }}</h2>
				<button class="icon-btn small" :title="t('addFocusRecord')" @click="addingRecord = true"><Plus :size="16" /></button>
			</div>
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
				<div class="menu-sep" />
				<div class="setting" :title="t('focusChimeDesc')">
					<span>{{ t('focusChime') }}</span>
					<SettingToggle v-model="prefs.focusChime" />
				</div>
				<div class="setting">
					<span>{{ t('focusTitleSetting') }}</span>
					<SettingToggle v-model="prefs.focusTitle" />
				</div>
				<div class="setting">
					<span>{{ t('focusMiniSetting') }}</span>
					<SettingToggle v-model="prefs.focusMini" />
				</div>
				<label class="setting">
					<span>{{ t('miniTimerSize') }}</span>
					<span class="setting-value">
						<input v-model.number="prefs.focusMiniScale" class="slider" type="range" min="0.5" max="3" step="0.1">
						<span class="slider-value">{{ Math.round(prefs.focusMiniScale * 100) }}%</span>
					</span>
				</label>
				<label class="setting" :title="t('miniTimerOpacityHint')">
					<span>{{ t('miniTimerOpacity') }}</span>
					<span class="setting-value">
						<input v-model.number="prefs.focusMiniOpacity" class="slider" type="range" min="0.1" max="1" step="0.05">
						<span class="slider-value">{{ Math.round(prefs.focusMiniOpacity * 100) }}%</span>
					</span>
				</label>
				<div class="setting">
					<span>{{ t('focusAutoFullscreen') }}</span>
					<SettingToggle v-model="prefs.focusAutoFullscreen" />
				</div>
				<div class="menu-sep" />
				<div class="menu-label">{{ t('dailyGoals') }}</div>
				<label class="setting">
					<span>{{ t('goalPomos') }}</span>
					<span class="setting-value">
						<input class="input num-input" type="number" min="0" max="50" :value="prefs.focusGoalPomos" @change="prefs.focusGoalPomos = clampGoal(+($event.target as HTMLInputElement).value, 50)">
						{{ t('pomosUnit') }}
					</span>
				</label>
				<label class="setting">
					<span>{{ t('goalFocus') }}</span>
					<span class="setting-value">
						<input class="input num-input" type="number" min="0" max="1440" step="5" :value="prefs.focusGoalMinutes" @change="prefs.focusGoalMinutes = clampGoal(+($event.target as HTMLInputElement).value, 1440)">
						{{ t('minutesUnit') }}
					</span>
				</label>
			</div>
		</Popover>

		<Popover v-if="showMore" :anchor="moreBtn" placement="bottom-end" @close="showMore = false">
			<div class="menu">
				<button class="menu-item" @click="openFullscreen"><Expand :size="16" /> {{ t('fullScreenMode') }}</button>
				<button class="menu-item" @click="openMini"><PictureInPicture2 :size="16" /> {{ popout ? t('closeMiniWindow') : t('openMiniWindow') }}</button>
				<button class="menu-item" @click="addingRecord = true; showMore = false"><Plus :size="16" /> {{ t('addFocusRecord') }}</button>
			</div>
		</Popover>

		<Popover v-if="showSound" :anchor="soundBtn" placement="bottom-end" @close="showSound = false">
			<div class="menu sounds">
				<div class="menu-label">{{ t('whiteNoise') }}</div>
				<button
					v-for="snd in FOCUS_SOUNDS"
					:key="snd"
					class="menu-item"
					:class="{active: prefs.focusSound === snd}"
					@click="prefs.focusSound = snd"
				>{{ t(SOUND_LABELS[snd]) }}<Check v-if="prefs.focusSound === snd" :size="15" class="menu-chevron" /></button>
				<div class="menu-sep" />
				<label class="setting">
					<span>{{ t('volume') }}</span>
					<input v-model.number="prefs.focusSoundVolume" class="volume" type="range" min="0" max="1" step="0.05">
				</label>
			</div>
		</Popover>

		<FocusRecordDialog v-if="addingRecord" @close="addingRecord = false" />
		<FocusFullscreen v-if="fullscreen" @close="fullscreen = false" />
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

.slider {
	width: 96px;
	accent-color: var(--accent);
}

.slider-value {
	width: 36px;
	font-variant-numeric: tabular-nums;
	text-align: right;
}

.num-input {
	width: 60px;
	height: 28px;
	color: var(--text);
}

/* --- header, sound, adjust, note --- */

.head-actions {
	display: flex;
	gap: 2px;
}

.icon-btn.on {
	color: var(--accent);
}

.clock-row {
	display: flex;
	align-items: center;
	gap: 6px;
}

.adjust {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 28px;
	height: 28px;
	border-radius: 50%;
	color: var(--text-3);
	opacity: 0;
	transition: opacity 0.15s;
}

.ring:hover .adjust,
.adjust:focus-visible {
	opacity: 1;
}

.adjust:hover:not(:disabled) {
	background: var(--bg-hover);
	color: var(--text);
}

.adjust:disabled {
	cursor: default;
	opacity: 0 !important;
}

.note {
	display: flex;
	flex-direction: column;
	gap: 4px;
	width: min(360px, 100%);
}

.note-label {
	font-size: 12px;
	color: var(--text-3);
}

.note-area {
	height: auto;
	min-height: 52px;
	padding: 8px 10px;
	resize: vertical;
	font: inherit;
	font-size: 13px;
	color: var(--text);
}

.sounds {
	width: 240px;
}

.volume {
	width: 120px;
	accent-color: var(--accent);
}

/* --- goals, ranking, year grid --- */

.goal {
	margin-left: 4px;
	font-size: 12px;
	font-weight: 400;
	color: var(--text-3);
}

.goal-bar {
	height: 4px;
	margin-top: 8px;
	border-radius: 2px;
	background: var(--border);
	overflow: hidden;
}

.goal-bar > div {
	height: 100%;
	background: var(--accent);
}

.rank {
	display: grid;
	grid-template-columns: 1fr auto;
	gap: 2px 8px;
	padding: 4px 0;
	font-size: 13px;
}

.rank-title {
	overflow: hidden;
	text-align: left;
	text-overflow: ellipsis;
	white-space: nowrap;
}

button.rank-title:hover {
	color: var(--accent);
}

.rank-minutes {
	font-variant-numeric: tabular-nums;
	color: var(--text-2);
}

.rank-bar {
	grid-column: 1 / -1;
	height: 4px;
	border-radius: 2px;
	background: var(--border);
	overflow: hidden;
}

.rank-bar > div {
	height: 100%;
	border-radius: 2px;
	background: var(--accent);
}

.year-grid {
	display: grid;
	grid-auto-columns: 1fr;
	grid-auto-flow: column;
	grid-template-rows: repeat(7, auto);
	gap: 2px;
}

.cell {
	aspect-ratio: 1;
	border-radius: 2px;
	background: var(--border);
}

.cell.l-1 {
	background: none;
}

.cell.l1 {
	background: color-mix(in srgb, var(--accent) 30%, transparent);
}

.cell.l2 {
	background: color-mix(in srgb, var(--accent) 55%, transparent);
}

.cell.l3 {
	background: color-mix(in srgb, var(--accent) 80%, transparent);
}

.cell.l4 {
	background: var(--accent);
}

.records-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	margin-bottom: 6px;
}

@media (hover: none) {
	.record .hover-action,
	.adjust {
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
