import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {nextTick, reactive} from 'vue'
import {createPinia, setActivePinia} from 'pinia'
import {api, ApiError} from '@/api/client'
import {prefs} from '@/lib/prefs'
import {fmtClock, fmtMinutes, useFocus} from './focus'

vi.mock('@/api/client', () => ({
	api: vi.fn(),
	fetchAll: vi.fn(async () => []),
	ApiError: class ApiError extends Error {
		status: number
		constructor(status: number, message: string) {
			super(message)
			this.status = status
		}
	},
}))
vi.mock('@/stores/data', () => {
	const data = reactive({authed: true, tasks: new Map([[7, {id: 7, title: 'Write report'}], [8, {id: 8, title: 'Read'}]])})
	return {useData: () => data}
})
vi.mock('@/lib/toast', () => ({toast: vi.fn()}))

const apiMock = vi.mocked(api)
const T0 = new Date('2026-10-22T09:00:00Z').getTime()
const MIN = 60_000

function posted() {
	return apiMock.mock.calls
		.filter(([method, path]) => method === 'POST' && path === '/focus-sessions')
		.map(([, , opts]) => opts?.body as {type: string, task_id: number, start_time: string, end_time: string, duration: number})
}

async function start(focus: ReturnType<typeof useFocus>) {
	focus.startFocus()
	await nextTick() // the ticking interval starts in a watcher
}

beforeEach(() => {
	localStorage.clear()
	vi.useFakeTimers()
	vi.setSystemTime(T0)
	apiMock.mockReset()
	apiMock.mockImplementation(async (method, path, opts) => {
		if (method === 'POST') return {id: 1, ...(opts?.body as object)}
		if (path === '/focus-sessions/stats') return {today_pomos: 0, today_focus_minutes: 0, total_pomos: 0, total_focus_minutes: 0, history: []}
		return {items: []}
	})
	Object.assign(prefs, {
		focusPomoMinutes: 25,
		focusShortBreakMinutes: 5,
		focusLongBreakMinutes: 15,
		focusLongBreakEvery: 4,
		focusAutoStartBreak: true,
		focusAutoStartPomo: false,
		focusChime: false,
		focusSound: 'none',
		focusTitle: true,
	})
	setActivePinia(createPinia())
})

afterEach(() => {
	vi.useRealTimers()
})

describe('fmtClock', () => {
	it('formats minutes and seconds, and hours past an hour', () => {
		expect(fmtClock(0)).toBe('00:00')
		expect(fmtClock(25 * 60)).toBe('25:00')
		expect(fmtClock(65)).toBe('01:05')
		expect(fmtClock(3600 + 61)).toBe('1:01:01')
	})
})

describe('fmtMinutes', () => {
	it('uses the largest units that fit', () => {
		expect(fmtMinutes(25)).toBe('25m')
		expect(fmtMinutes(120)).toBe('2h')
		expect(fmtMinutes(85)).toBe('1h 25m')
	})
})

describe('pomo', () => {
	it('records a finished pomo and starts the short break', async () => {
		const focus = useFocus()
		focus.setTask(7)
		await start(focus)
		expect(focus.display).toBe('25:00')

		vi.advanceTimersByTime(10 * MIN)
		expect(focus.display).toBe('15:00')

		vi.advanceTimersByTime(15 * MIN)
		expect(posted()).toEqual([{
			task_id: 7,
			type: 'pomo',
			start_time: '2026-10-22T09:00:00.000Z',
			end_time: '2026-10-22T09:25:00.000Z',
			duration: 1500,
		}])
		expect(focus.state.phase).toBe('break')
		expect(focus.state.breakKind).toBe('short')
		expect(focus.state.running).toBe(true)
		expect(focus.state.pomoCount).toBe(1)
	})

	it('does not count paused time', async () => {
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(5 * MIN)
		focus.pause()
		vi.advanceTimersByTime(10 * MIN)
		expect(focus.display).toBe('20:00')
		focus.resume()
		vi.advanceTimersByTime(20 * MIN)

		const [session] = posted()
		expect(session.duration).toBe(1500)
		expect(session.end_time).toBe('2026-10-22T09:35:00.000Z')
	})

	it('takes a long break after every fourth pomo, then waits for the next pomo', async () => {
		const focus = useFocus()
		for (let i = 0; i < 4; i++) {
			await start(focus)
			vi.advanceTimersByTime(25 * MIN)
			if (i < 3) {
				expect(focus.state.breakKind).toBe('short')
				focus.stop() // skip the break
				await nextTick()
			}
		}
		expect(focus.state.breakKind).toBe('long')
		expect(focus.state.pomoCount).toBe(0)

		vi.advanceTimersByTime(15 * MIN)
		expect(focus.state.phase).toBe('idle')
		expect(posted()).toHaveLength(4)
	})

	it('catches up on phases that ended while the app was closed', async () => {
		const focus = useFocus()
		await start(focus)
		vi.setSystemTime(T0 + 2 * 60 * MIN)
		vi.advanceTimersByTime(1000)

		expect(posted()[0].end_time).toBe('2026-10-22T09:25:00.000Z')
		expect(focus.state.phase).toBe('idle') // the break ended at 09:30 too
	})

	it('does not record an abandoned pomo', async () => {
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(20 * MIN)
		focus.pause()
		focus.stop()
		expect(posted()).toHaveLength(0)
		expect(focus.state.phase).toBe('idle')
	})

	it('restores a running timer after a reload', async () => {
		await start(useFocus())
		vi.advanceTimersByTime(5 * MIN)
		await nextTick() // persist

		setActivePinia(createPinia())
		const focus = useFocus()
		await nextTick()
		expect(focus.state.phase).toBe('focus')
		expect(focus.display).toBe('20:00')
	})
})

describe('stopwatch', () => {
	it('records the focused time when ended', async () => {
		const focus = useFocus()
		focus.setMode('stopwatch')
		await start(focus)
		vi.advanceTimersByTime(42 * MIN)
		expect(focus.display).toBe('42:00')
		focus.stop()

		expect(posted()).toEqual([expect.objectContaining({type: 'stopwatch', duration: 42 * 60})])
	})

	it('drops sessions shorter than a minute', async () => {
		const focus = useFocus()
		focus.setMode('stopwatch')
		await start(focus)
		vi.advanceTimersByTime(30_000)
		focus.stop()
		expect(posted()).toHaveLength(0)
	})

	it('cannot switch modes while running', async () => {
		const focus = useFocus()
		await start(focus)
		focus.setMode('stopwatch')
		expect(focus.state.mode).toBe('pomo')
	})
})

describe('saving', () => {
	it('keeps the record without its task when the task is gone', async () => {
		apiMock.mockImplementation(async (method, _path, opts) => {
			const body = opts?.body as {task_id?: number} | undefined
			if (method === 'POST' && body?.task_id) throw new ApiError(404, 'gone')
			if (method === 'POST') return {id: 1, ...body}
			return {history: []}
		})
		const focus = useFocus()
		focus.setTask(99)
		focus.setMode('stopwatch')
		await start(focus)
		vi.advanceTimersByTime(5 * MIN)
		focus.stop()
		await vi.runOnlyPendingTimersAsync()

		const sessions = posted()
		expect(sessions).toHaveLength(2)
		expect(sessions[1].task_id).toBe(0)
		expect(focus.state.pending).toHaveLength(0)
	})

	it('keeps unsaved records for later', async () => {
		apiMock.mockImplementation(async method => {
			if (method === 'POST') throw new ApiError(500, 'offline')
			return {history: []}
		})
		const focus = useFocus()
		focus.setMode('stopwatch')
		await start(focus)
		vi.advanceTimersByTime(5 * MIN)
		focus.stop()
		await vi.runOnlyPendingTimersAsync()

		expect(focus.state.pending).toHaveLength(1)
	})
})

describe('adjusting the time', () => {
	it('lengthens and shortens the running pomo in five-minute steps', async () => {
		const focus = useFocus()
		await start(focus)
		focus.adjust(5)
		expect(focus.display).toBe('30:00')
		focus.adjust(-5)
		focus.adjust(-5)
		expect(focus.display).toBe('20:00')

		vi.advanceTimersByTime(20 * MIN)
		expect(posted()).toEqual([expect.objectContaining({type: 'pomo', duration: 20 * 60})])
		// the break is not affected
		expect(focus.display).toBe('05:00')
	})

	it('never goes below the time already focused plus a minute', async () => {
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(18 * MIN)
		focus.adjust(-5)
		focus.adjust(-5)
		expect(focus.phaseLength).toBe(19 * MIN)
		expect(focus.canAdjust(-5)).toBe(false)
	})

	it('changes the next pomo while idle', () => {
		const focus = useFocus()
		focus.adjust(10)
		expect(focus.phaseLength).toBe(35 * MIN)
		focus.startFocus()
		expect(focus.phaseLength).toBe(35 * MIN)
	})

	it('has no -/+ for the stopwatch', () => {
		const focus = useFocus()
		focus.setMode('stopwatch')
		expect(focus.canAdjust(5)).toBe(false)
	})
})

describe('ending a pomo early', () => {
	it('offers abandoning only in the first five minutes', async () => {
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(4 * MIN)
		expect(focus.earlyEnd()).toBe('abandon')
		focus.endEarly('save')
		expect(posted()).toHaveLength(0)
		expect(focus.state.phase).toBe('idle')
	})

	it('saves the time focused as a stopwatch record, not a pomo', async () => {
		const focus = useFocus()
		focus.setTask(7)
		await start(focus)
		vi.advanceTimersByTime(12 * MIN)
		expect(focus.earlyEnd()).toBe('saveOrAbandon')
		focus.endEarly('save')
		expect(posted()).toEqual([expect.objectContaining({type: 'stopwatch', task_id: 7, duration: 12 * 60})])
		expect(focus.state.pomoCount).toBe(0)
		expect(focus.state.phase).toBe('idle')
	})

	it('counts a full pomo when less than five minutes are left', async () => {
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(22 * MIN)
		expect(focus.earlyEnd()).toBe('complete')
		focus.endEarly('complete')
		expect(posted()).toEqual([expect.objectContaining({type: 'pomo', duration: 22 * 60})])
		expect(focus.state.pomoCount).toBe(1)
		expect(focus.state.phase).toBe('break')
	})

	it('is not asked for the stopwatch or a break', async () => {
		const focus = useFocus()
		focus.setMode('stopwatch')
		await start(focus)
		expect(focus.earlyEnd()).toBeNull()
	})
})

describe('focus note', () => {
	it('is saved with the record and cleared for the next session', async () => {
		const focus = useFocus()
		await start(focus)
		focus.state.note = '  drafted the intro '
		vi.advanceTimersByTime(25 * MIN)
		expect(posted()[0]).toEqual(expect.objectContaining({note: 'drafted the intro'}))
		expect(focus.state.note).toBe('')
	})
})

describe('completion pop-up', () => {
	it('opens when a pomo ends and closes when the user moves on', async () => {
		prefs.focusAutoStartBreak = false
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(25 * MIN)
		expect(focus.state.prompt).toBe('pomoDone')
		focus.resume() // start break
		expect(focus.state.prompt).toBeNull()
	})

	it('opens when a break ends', async () => {
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(30 * MIN)
		expect(focus.state.phase).toBe('idle')
		expect(focus.state.prompt).toBe('breakDone')
	})

	it('adds the time worked past the end to the record and restarts the break', async () => {
		prefs.focusAutoStartBreak = false
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(25 * MIN)
		await vi.runOnlyPendingTimersAsync() // saved, id 1
		vi.advanceTimersByTime(7 * MIN)

		expect(await focus.addExtraTime()).toBe(true)
		const patch = apiMock.mock.calls.find(([method]) => method === 'PATCH')
		expect(patch?.[1]).toBe('/focus-sessions/1')
		// 7 minutes and the second the save took
		expect(patch?.[2]?.body).toEqual({end_time: '2026-10-22T09:32:01.000Z', duration: 32 * 60 + 1})
		expect(focus.state.last?.duration).toBe(32 * 60 + 1)
	})

	it('changes a queued record in place when editing the duration', async () => {
		apiMock.mockImplementation(async method => {
			if (method === 'POST') throw new ApiError(500, 'offline')
			return {history: []}
		})
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(25 * MIN)
		await vi.runOnlyPendingTimersAsync()

		expect(await focus.editLastDuration(20)).toBe(true)
		expect(focus.state.pending[0]).toEqual(expect.objectContaining({duration: 20 * 60, end_time: '2026-10-22T09:20:00.000Z'}))
	})
})

describe('starting from a task', () => {
	it('starts the chosen mode on the task', () => {
		const focus = useFocus()
		focus.startForTask(7, 'stopwatch')
		expect(focus.state).toEqual(expect.objectContaining({phase: 'focus', mode: 'stopwatch', taskId: 7}))
	})

	it('moves a running session to another task after asking', () => {
		const focus = useFocus()
		focus.startForTask(7, 'pomo')
		const ask = vi.fn(() => true)
		vi.stubGlobal('confirm', ask)
		focus.startForTask(8, 'stopwatch')
		expect(ask).toHaveBeenCalledWith(expect.stringContaining('Read'))
		expect(focus.state).toEqual(expect.objectContaining({mode: 'pomo', taskId: 8}))
		vi.unstubAllGlobals()
	})
})

describe('tab title', () => {
	it('shows the time left while a session runs', async () => {
		document.title = 'OpenTick'
		const focus = useFocus()
		await start(focus)
		vi.advanceTimersByTime(MIN)
		await nextTick()
		expect(document.title).toBe('24:00 · Focusing – OpenTick')
		focus.stop()
		await nextTick()
		expect(document.title).toBe('OpenTick')
	})
})
