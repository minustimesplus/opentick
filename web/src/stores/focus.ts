import {defineStore} from 'pinia'
import {computed, reactive, ref, watch} from 'vue'
import {api, ApiError} from '@/api/client'
import type {FocusSession, FocusSessionType, FocusStats} from '@/api/types'
import {useData} from '@/stores/data'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

const KEY = 'tt.focus'

/** Stopwatch sessions shorter than this are not worth a record. */
const MIN_STOPWATCH_MS = 60_000

export type FocusPhase = 'idle' | 'focus' | 'break'

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
	pending: [],
}

function loadState(): TimerState {
	try {
		const raw = localStorage.getItem(KEY)
		if (raw) return {...initial, ...JSON.parse(raw)}
	} catch {
		// storage unavailable or corrupt
	}
	return {...initial}
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
	chime()
	if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return
	try {
		new Notification(title, {body})
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
	const phaseLength = computed<number | null>(() => {
		if (state.phase === 'break') {
			return (state.breakKind === 'long' ? prefs.focusLongBreakMinutes : prefs.focusShortBreakMinutes) * 60_000
		}
		if (state.mode === 'stopwatch') return null
		return prefs.focusPomoMinutes * 60_000
	})

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
		Object.assign(state, {phase: 'focus', running: true, startedAt: at, segmentStart: at, accumulated: 0})
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
		now.value = Date.now()
	}

	function reset() {
		Object.assign(state, {phase: 'idle', running: false, startedAt: null, segmentStart: null, accumulated: 0})
	}

	/** Ends the current phase on time: records a finished pomo and moves on to the break or the next pomo. */
	function complete() {
		if (state.segmentStart === null || phaseLength.value === null) return
		const endAt = state.segmentStart + (phaseLength.value - state.accumulated)
		if (state.phase === 'focus') {
			record('pomo', state.startedAt ?? endAt - phaseLength.value, endAt, phaseLength.value)
			state.pomoCount++
			const long = state.pomoCount >= Math.max(1, prefs.focusLongBreakEvery)
			if (long) state.pomoCount = 0
			notify(t('pomoDone'), t(long ? 'pomoDoneLongBreak' : 'pomoDoneBreak'))
			Object.assign(state, {
				phase: 'break',
				breakKind: long ? 'long' : 'short',
				accumulated: 0,
				running: prefs.focusAutoStartBreak,
				segmentStart: prefs.focusAutoStartBreak ? endAt : null,
			})
		} else if (state.phase === 'break') {
			notify(t('breakOver'), t('breakOverBody'))
			if (prefs.focusAutoStartPomo && state.mode === 'pomo') {
				startFocus(endAt)
			} else {
				reset()
			}
		}
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
		reset()
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
		state.pending.push(session)
		flushPending()
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
			await loadStats()
		} finally {
			flushing = false
		}
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

	async function load() {
		flushPending()
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
		await loadStats()
	}

	async function updateSession(id: number, patch: Partial<Pick<FocusSession, 'note' | 'task_id'>>): Promise<boolean> {
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
			await loadStats()
			return true
		} catch (e) {
			toast(String((e as Error).message))
			return false
		}
	}

	return {
		state, sessions, stats, historyDays, phaseLength, elapsed, remaining, progress, display,
		startFocus, pause, resume, stop, setMode, setTask, load, loadStats, setHistoryDays, updateSession, removeSession,
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
