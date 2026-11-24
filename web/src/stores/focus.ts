import {defineStore} from 'pinia'
import {computed, reactive, ref, watch} from 'vue'
import {api, ApiError, fetchAll} from '@/api/client'
import type {FocusSession, FocusSessionType, FocusStats} from '@/api/types'
import {useData} from '@/stores/data'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'
import {playAmbient, stopAmbient} from '@/lib/focusSound'

const KEY = 'tt.focus'

/** Stopwatch sessions shorter than this are not worth a record. */
const MIN_STOPWATCH_MS = 60_000
/** A pomo ended early is only worth a record after this long (TickTick's limit). */
export const MIN_EARLY_POMO_MS = 5 * 60_000
/** Shortest and longest a countdown can be adjusted to with the -/+ buttons. */
const MIN_ADJUSTED_MS = 5 * 60_000
const MAX_ADJUSTED_MS = 180 * 60_000

export type FocusPhase = 'idle' | 'focus' | 'break'

/** What ending a pomo before its time offers: TickTick's three confirmations. */
export type EarlyEnd = 'complete' | 'saveOrAbandon' | 'abandon'

/** The pop-up shown when a phase ends on its own. */
export type FocusPrompt = 'pomoDone' | 'breakDone'

/** The record of the pomo that just finished, so the pop-up can amend it. */
interface LastRecord {
	/** Server id, 0 while the record is still queued. */
	id: number
	start_time: string
	end_time: string
	duration: number
}

interface TimerState {
	mode: FocusSessionType
	phase: FocusPhase
	breakKind: 'short' | 'long'
	running: boolean
	/** When the current focus session started (ms). */
	startedAt: number | null
	/** When the current running stretch started (ms); null while paused. */
	segmentStart: number | null
	/** Time spent in the current phase before the running stretch (ms). */
	accumulated: number
	taskId: number
	/** Pomos finished since the last long break. */
	pomoCount: number
	/** Minutes added to or taken from the current countdown with -/+ (ms). */
	adjustMs: number
	/** Focus note typed during the session; saved with its record. */
	note: string
	/** Pop-up to show for the phase that just ended on its own. */
	prompt: FocusPrompt | null
	/** When that phase ended (ms). */
	promptAt: number
	last: LastRecord | null
	/** Sessions that could not be saved yet; retried on the next load. */
	pending: Omit<FocusSession, 'id'>[]
}

const initial: TimerState = {
	mode: 'pomo',
	phase: 'idle',
	breakKind: 'short',
	running: false,
	startedAt: null,
	segmentStart: null,
	accumulated: 0,
	taskId: 0,
	pomoCount: 0,
	adjustMs: 0,
	note: '',
	prompt: null,
	promptAt: 0,
	last: null,
	pending: [],
}

function loadState(): TimerState {
	try {
		const raw = localStorage.getItem(KEY)
		if (raw) return {...initial, pending: [], ...JSON.parse(raw)}
	} catch {
		// storage unavailable or corrupt
	}
	// A fresh array: pushing to initial.pending would leak records into every later reset.
	return {...initial, pending: []}
}

function chime() {
	try {
		const ctx = new AudioContext()
		;[0, 0.25, 0.5].forEach((at, i) => {
			const osc = ctx.createOscillator()
			const gain = ctx.createGain()
			osc.frequency.value = i === 2 ? 1046 : 784
			gain.gain.setValueAtTime(0.0001, ctx.currentTime + at)
			gain.gain.exponentialRampToValueAtTime(0.25, ctx.currentTime + at + 0.02)
			gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + at + 0.4)
			osc.connect(gain).connect(ctx.destination)
			osc.start(ctx.currentTime + at)
			osc.stop(ctx.currentTime + at + 0.45)
		})
		setTimeout(() => ctx.close(), 1500)
	} catch {
		// no audio output
	}
}

function notify(title: string, body: string) {
	if (!prefs.features.pomo) return
	if (prefs.focusChime) chime()
	if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return
	try {
		const n = new Notification(title, {body})
		n.onclick = () => {
			window.focus()
			window.ttDesktop?.focusWindow?.()
			n.close()
		}
	} catch {
		// some platforms only allow notifications from a service worker
	}
}

export const useFocus = defineStore('focus', () => {
	const data = useData()
	const state = reactive<TimerState>(loadState())
	const now = ref(Date.now())
	const sessions = ref<FocusSession[]>([])
	const stats = ref<FocusStats | null>(null)

	watch(state, value => {
		try {
			localStorage.setItem(KEY, JSON.stringify(value))
		} catch {
			// ignore
		}
	}, {deep: true})

	watch(() => data.authed, authed => {
		if (!authed) {
			Object.assign(state, {...initial, pending: []})
			sessions.value = []
			stats.value = null
		}
	})

	/** Length of the current phase in ms, or null for an open-ended stopwatch. */
	const baseLength = computed<number | null>(() => {
		if (state.phase === 'break') {
			return (state.breakKind === 'long' ? prefs.focusLongBreakMinutes : prefs.focusShortBreakMinutes) * 60_000
		}
		if (state.mode === 'stopwatch') return null
		return prefs.focusPomoMinutes * 60_000
	})
	const phaseLength = computed<number | null>(() => baseLength.value === null ? null : Math.max(60_000, baseLength.value + state.adjustMs))

	const elapsed = computed(() => state.accumulated + (state.running && state.segmentStart ? Math.max(0, now.value - state.segmentStart) : 0))
	const remaining = computed(() => phaseLength.value === null ? null : Math.max(0, phaseLength.value - elapsed.value))
	/** 0..1 through the current phase; stopwatches go round once an hour. */
	const progress = computed(() => phaseLength.value === null
		? (elapsed.value % 3_600_000) / 3_600_000
		: Math.min(1, elapsed.value / phaseLength.value))
	/** What the clock shows: time left for countdowns, time spent for the stopwatch. */
	const display = computed(() => remaining.value === null
		? fmtClock(Math.floor(elapsed.value / 1000))
		: fmtClock(Math.ceil(remaining.value / 1000)))

	// Turning the Pomo feature off holds a running timer where it is; it can
	// be resumed once the feature is back on.
	watch(() => prefs.features.pomo, on => {
		if (!on) pause()
	}, {immediate: true})

	// White noise plays while focusing (not during breaks or pauses).
	watch(() => prefs.features.pomo && state.phase === 'focus' && state.running && prefs.focusSound !== 'none'
		? prefs.focusSound
		: 'none', sound => {
		if (sound === 'none') stopAmbient()
		else playAmbient(sound, prefs.focusSoundVolume)
	}, {immediate: true})
	watch(() => prefs.focusSoundVolume, v => {
		if (state.phase === 'focus' && state.running && prefs.focusSound !== 'none') playAmbient(prefs.focusSound, v)
	})

	// The time left shows in the tab (and window) title, like TickTick's "Focus Time in Tab Title".
	const baseTitle = typeof document !== 'undefined' ? document.title : ''
	watch(() => prefs.features.pomo && prefs.focusTitle && state.phase !== 'idle'
		? `${state.running ? '' : '⏸ '}${display.value} · ${t(state.phase === 'break' ? 'focusRelaxing' : 'focusing')} – ${baseTitle}`
		: baseTitle, title => {
		if (typeof document !== 'undefined') document.title = title
	}, {immediate: true})

	let interval: number | undefined
	watch(() => state.phase !== 'idle', active => {
		clearInterval(interval)
		interval = undefined
		if (active) {
			interval = window.setInterval(tick, 1000)
			tick()
		}
	}, {immediate: true})

	function tick() {
		now.value = Date.now()
		// A tab left in the background (or closed) may have missed several phase ends.
		for (let i = 0; i < 3 && state.running && remaining.value === 0; i++) complete()
	}

	function startFocus(at = Date.now()) {
		requestNotificationPermission()
		// A pomo started by hand after a break keeps a -/+ set while idle.
		const adjustMs = state.phase === 'idle' ? state.adjustMs : 0
		Object.assign(state, {phase: 'focus', running: true, startedAt: at, segmentStart: at, accumulated: 0, adjustMs, prompt: null})
		now.value = Date.now()
	}

	function pause() {
		if (!state.running || state.segmentStart === null) return
		state.accumulated += Math.max(0, Date.now() - state.segmentStart)
		state.segmentStart = null
		state.running = false
	}

	function resume() {
		if (state.running || state.phase === 'idle') return
		state.segmentStart = Date.now()
		state.running = true
		state.prompt = null
		now.value = Date.now()
	}

	function reset() {
		Object.assign(state, {phase: 'idle', running: false, startedAt: null, segmentStart: null, accumulated: 0, adjustMs: 0})
	}

	/** Starts, pauses or resumes, whichever fits (the Start/Pause Focus shortcut). */
	function toggle() {
		if (state.phase === 'idle') startFocus()
		else if (state.running) pause()
		else resume()
	}

	/**
	 * Lengthens or shortens the current countdown (or the next pomo while
	 * idle) by `minutes`, never below what has already run plus a minute.
	 */
	function adjust(minutes: number) {
		if (phaseLength.value === null || baseLength.value === null) return
		const floor = Math.max(MIN_ADJUSTED_MS, state.phase === 'idle' ? 0 : elapsed.value + 60_000)
		const target = Math.min(MAX_ADJUSTED_MS, Math.max(floor, phaseLength.value + minutes * 60_000))
		state.adjustMs = target - baseLength.value
	}

	/** Whether the -/+ buttons can still move the countdown. */
	function canAdjust(minutes: number): boolean {
		if (phaseLength.value === null) return false
		if (minutes > 0) return phaseLength.value < MAX_ADJUSTED_MS
		const floor = Math.max(MIN_ADJUSTED_MS, state.phase === 'idle' ? 0 : elapsed.value + 60_000)
		return phaseLength.value > floor
	}

	/** Records a finished pomo and moves on to the break; shared by on-time and early completion. */
	function finishPomo(endAt: number, focusedMs: number) {
		record('pomo', state.startedAt ?? endAt - focusedMs, endAt, focusedMs)
		state.pomoCount++
		const long = state.pomoCount >= Math.max(1, prefs.focusLongBreakEvery)
		if (long) state.pomoCount = 0
		notify(t('pomoDone'), t(long ? 'pomoDoneLongBreak' : 'pomoDoneBreak'))
		Object.assign(state, {
			phase: 'break',
			breakKind: long ? 'long' : 'short',
			accumulated: 0,
			adjustMs: 0,
			running: prefs.focusAutoStartBreak,
			segmentStart: prefs.focusAutoStartBreak ? endAt : null,
			prompt: 'pomoDone',
			promptAt: endAt,
		})
	}

	/** Ends the current phase on time: records a finished pomo and moves on to the break or the next pomo. */
	function complete() {
		if (state.segmentStart === null || phaseLength.value === null) return
		const endAt = state.segmentStart + (phaseLength.value - state.accumulated)
		if (state.phase === 'focus') {
			finishPomo(endAt, phaseLength.value)
		} else if (state.phase === 'break') {
			const auto = prefs.focusAutoStartPomo && state.mode === 'pomo'
			notify(t('breakOver'), t(auto ? 'breakOverAutoBody' : 'breakOverBody'))
			if (auto) {
				startFocus(endAt)
			} else {
				reset()
			}
			Object.assign(state, {prompt: 'breakDone', promptAt: endAt})
		}
	}

	/** What ending the running pomo now would offer, or null when it is not a pomo. */
	function earlyEnd(): EarlyEnd | null {
		if (state.phase !== 'focus' || state.mode !== 'pomo' || remaining.value === null) return null
		if (remaining.value < MIN_EARLY_POMO_MS) return 'complete'
		return elapsed.value >= MIN_EARLY_POMO_MS ? 'saveOrAbandon' : 'abandon'
	}

	/**
	 * Ends a pomo before its time. 'complete' counts it as a full pomo (less
	 * than five minutes were left); 'save' keeps the focused time as a record
	 * without counting a pomo; 'abandon' drops it.
	 */
	function endEarly(how: 'complete' | 'save' | 'abandon') {
		if (state.phase !== 'focus' || state.startedAt === null) return
		pause()
		const focused = state.accumulated
		if (how === 'complete') {
			finishPomo(Date.now(), focused)
			state.prompt = null
			return
		}
		if (how === 'save' && focused >= MIN_EARLY_POMO_MS) {
			record('stopwatch', state.startedAt, Date.now(), focused)
		}
		reset()
	}

	/**
	 * Ends the current phase early. A stopwatch session is recorded; an
	 * unfinished pomo is abandoned, like TickTick; a break is skipped.
	 */
	function stop() {
		if (state.phase === 'focus' && state.mode === 'stopwatch' && state.startedAt !== null) {
			pause()
			if (state.accumulated >= MIN_STOPWATCH_MS) {
				record('stopwatch', state.startedAt, Date.now(), state.accumulated)
			}
		}
		state.prompt = null
		reset()
	}

	function dismissPrompt() {
		state.prompt = null
	}

	/**
	 * Starts focusing on a task (task menu > Start Focus). With a session
	 * already running it offers to move that session to the task instead.
	 */
	function startForTask(taskId: number, mode: FocusSessionType) {
		if (state.phase === 'idle') {
			state.mode = mode
			state.taskId = taskId
			startFocus()
			return
		}
		if (state.taskId === taskId) {
			toast(t('taskIsFocused'))
			return
		}
		const title = data.tasks.get(taskId)?.title ?? ''
		const kind = t(state.mode === 'pomo' ? 'pomo' : 'stopwatch')
		if (confirm(t('focusSwitchTask', {kind, task: title}))) state.taskId = taskId
	}

	function setMode(mode: FocusSessionType) {
		if (state.phase !== 'idle') return
		state.mode = mode
	}

	function setTask(id: number) {
		state.taskId = id
	}

	function record(type: FocusSessionType, start: number, end: number, focusedMs: number) {
		const session: Omit<FocusSession, 'id'> = {
			task_id: state.taskId,
			type,
			// Whole seconds, widened outwards, so the stored span always holds the duration.
			start_time: new Date(Math.floor(start / 1000) * 1000).toISOString(),
			end_time: new Date(Math.ceil(end / 1000) * 1000).toISOString(),
			duration: Math.floor(focusedMs / 1000),
		}
		const note = state.note.trim()
		if (note) session.note = note
		state.note = ''
		state.last = {id: 0, start_time: session.start_time, end_time: session.end_time, duration: session.duration}
		state.pending.push(session)
		flushPending()
	}

	/** Changes the record of the pomo that just finished, queued or saved. */
	async function amendLast(patch: Pick<FocusSession, 'end_time' | 'duration'>): Promise<boolean> {
		const last = state.last
		if (!last) return false
		const queued = state.pending.find(p => p.start_time === last.start_time)
		if (queued) {
			Object.assign(queued, patch)
		} else if (last.id) {
			if (!await updateSession(last.id, patch)) return false
			await loadStats()
		} else {
			return false
		}
		Object.assign(last, patch)
		return true
	}

	/**
	 * Adds the time since the pomo ended to its record ("Add Extra Time"):
	 * the user kept working, so a break that started meanwhile starts over.
	 */
	async function addExtraTime(): Promise<boolean> {
		const last = state.last
		if (!last) return false
		const endAt = new Date(last.end_time).getTime()
		const extra = Math.floor((Date.now() - endAt) / 1000)
		if (extra < 60) return false
		const ok = await amendLast({
			end_time: new Date(endAt + extra * 1000).toISOString(),
			duration: last.duration + extra,
		})
		if (ok && state.phase === 'break') {
			state.accumulated = 0
			if (state.running) state.segmentStart = Date.now()
		}
		return ok
	}

	/** Sets the focused minutes of the pomo that just finished ("Edit Focus Duration"); it ends that much earlier. */
	async function editLastDuration(minutes: number): Promise<boolean> {
		const last = state.last
		if (!last) return false
		const duration = Math.max(60, Math.min(last.duration, Math.round(minutes * 60)))
		const end = new Date(last.end_time).getTime() - (last.duration - duration) * 1000
		return amendLast({end_time: new Date(end).toISOString(), duration})
	}

	/** Records a session by hand (Focus Record > +). */
	async function addRecord(session: Omit<FocusSession, 'id'>): Promise<boolean> {
		try {
			const created = await api<FocusSession>('POST', '/focus-sessions', {body: session})
			sessions.value = [created, ...sessions.value].sort((a, b) => b.start_time.localeCompare(a.start_time))
			await Promise.all([loadStats(), loadRange()])
			return true
		} catch (e) {
			toast(String((e as Error).message))
			return false
		}
	}

	let flushing = false
	async function flushPending() {
		if (flushing || !data.authed) return
		flushing = true
		try {
			while (state.pending.length) {
				const next = state.pending[0]
				try {
					const created = await api<FocusSession>('POST', '/focus-sessions', {body: next})
					sessions.value = [created, ...sessions.value]
					if (state.last && state.last.start_time === next.start_time) state.last.id = created.id
				} catch (e) {
					// A linked task that was deleted (or unshared) must not block the record.
					if (next.task_id && e instanceof ApiError && (e.status === 403 || e.status === 404)) {
						next.task_id = 0
						continue
					}
					toast(String((e as Error).message))
					return
				}
				state.pending.shift()
			}
		} finally {
			flushing = false
		}
		// Outside the lock: a record queued while the stats load must still be sent.
		await Promise.all([loadStats(), loadRange()])
	}

	const historyDays = ref(7)

	async function loadStats() {
		try {
			stats.value = await api<FocusStats>('GET', '/focus-sessions/stats', {
				query: {days: historyDays.value, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone},
			})
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	/** Sessions of the stats range, for the focus ranking. */
	const rangeSessions = ref<FocusSession[]>([])
	/** Per-day history of the last year, for the year grid. */
	const yearHistory = ref<FocusStats['history']>([])

	async function loadRange() {
		const from = new Date()
		from.setHours(0, 0, 0, 0)
		from.setDate(from.getDate() - (historyDays.value - 1))
		try {
			rangeSessions.value = await fetchAll<FocusSession>('/focus-sessions', {from: from.toISOString(), per_page: 250})
		} catch {
			// the ranking is optional; the main list already reported errors
		}
	}

	async function loadYear() {
		try {
			const res = await api<FocusStats>('GET', '/focus-sessions/stats', {
				query: {days: 366, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone},
			})
			yearHistory.value = res?.history ?? []
		} catch {
			// optional
		}
	}

	async function load() {
		flushPending()
		loadRange()
		try {
			const res = await api<{items?: FocusSession[] | null}>('GET', '/focus-sessions', {query: {per_page: 50}})
			sessions.value = res?.items ?? []
		} catch (e) {
			toast(String((e as Error).message))
		}
		await loadStats()
	}

	async function setHistoryDays(days: number) {
		historyDays.value = days
		await Promise.all([loadStats(), loadRange()])
	}

	async function updateSession(id: number, patch: Partial<Pick<FocusSession, 'note' | 'task_id' | 'end_time' | 'duration'>>): Promise<boolean> {
		try {
			const updated = await api<FocusSession>('PATCH', `/focus-sessions/${id}`, {body: patch, contentType: 'application/merge-patch+json'})
			sessions.value = sessions.value.map(s => s.id === id ? updated : s)
			return true
		} catch (e) {
			toast(String((e as Error).message))
			return false
		}
	}

	async function removeSession(id: number): Promise<boolean> {
		try {
			await api('DELETE', `/focus-sessions/${id}`)
			sessions.value = sessions.value.filter(s => s.id !== id)
			rangeSessions.value = rangeSessions.value.filter(s => s.id !== id)
			await loadStats()
			return true
		} catch (e) {
			toast(String((e as Error).message))
			return false
		}
	}

	return {
		state, sessions, stats, historyDays, rangeSessions, yearHistory, phaseLength, elapsed, remaining, progress, display,
		startFocus, pause, resume, stop, toggle, adjust, canAdjust, earlyEnd, endEarly, dismissPrompt, startForTask,
		setMode, setTask, load, loadStats, loadYear, setHistoryDays, updateSession, removeSession,
		addExtraTime, editLastDuration, addRecord,
	}
})

function requestNotificationPermission() {
	if (typeof Notification !== 'undefined' && Notification.permission === 'default') {
		Notification.requestPermission().catch(() => {})
	}
}

/** mm:ss, or h:mm:ss past an hour. */
export function fmtClock(seconds: number): string {
	const h = Math.floor(seconds / 3600)
	const mm = String(Math.floor((seconds % 3600) / 60)).padStart(2, '0')
	const ss = String(seconds % 60).padStart(2, '0')
	return h ? `${h}:${mm}:${ss}` : `${mm}:${ss}`
}

/** 1h 25m / 25m for a number of minutes. */
export function fmtMinutes(minutes: number): string {
	const h = Math.floor(minutes / 60)
	const m = minutes % 60
	if (!h) return t('minutesShort', {n: m})
	if (!m) return t('hoursShort', {n: h})
	return t('hoursMinutesShort', {h, m})
}
