import {describe, expect, it} from 'vitest'
import type {Task} from '@/api/types'
import {getSchedule, schedulePatch} from './dates'
import {dragSpan, groups, spanPatch, taskSpan, timelineRange} from './timeline'

const day = (m: number, d: number, h = 0, min = 0) => new Date(2026, m - 1, d, h, min)

function task(patch: Partial<Task>): Task {
	return {id: 1, title: 'T', done: false, project_id: 1, ...patch}
}

function allDay(from: Date, to?: Date): Task {
	return task(schedulePatch(from, true, to ?? null))
}

function timed(from: Date, to?: Date): Task {
	return task(schedulePatch(from, false, to ?? null))
}

describe('taskSpan', () => {
	it('is null for undated tasks', () => {
		expect(taskSpan(task({}))).toBeNull()
	})

	it('covers one day for a task with only a due date', () => {
		expect(taskSpan(allDay(day(10, 5)))).toEqual({start: day(10, 5), end: day(10, 5)})
		expect(taskSpan(timed(day(10, 5, 14, 30)))).toEqual({start: day(10, 5), end: day(10, 5)})
	})

	it('covers every day of a multi-day task', () => {
		expect(taskSpan(allDay(day(10, 5), day(10, 8)))).toEqual({start: day(10, 5), end: day(10, 8)})
		expect(taskSpan(timed(day(10, 5, 22), day(10, 7, 2)))).toEqual({start: day(10, 5), end: day(10, 7)})
	})

	it('does not count the day a timed block ends at midnight', () => {
		expect(taskSpan(timed(day(10, 5, 9), day(10, 7)))).toEqual({start: day(10, 5), end: day(10, 6)})
	})
})

describe('dragSpan', () => {
	const span = {start: day(10, 5), end: day(10, 8)}

	it('moves both ends', () => {
		expect(dragSpan(span, 'move', 3)).toEqual({start: day(10, 8), end: day(10, 11)})
		expect(dragSpan(span, 'move', -5)).toEqual({start: day(9, 30), end: day(10, 3)})
	})

	it('resizes one end without crossing the other', () => {
		expect(dragSpan(span, 'start', -2)).toEqual({start: day(10, 3), end: day(10, 8)})
		expect(dragSpan(span, 'start', 9)).toEqual({start: day(10, 8), end: day(10, 8)})
		expect(dragSpan(span, 'end', 2)).toEqual({start: day(10, 5), end: day(10, 10)})
		expect(dragSpan(span, 'end', -9)).toEqual({start: day(10, 5), end: day(10, 5)})
	})
})

describe('spanPatch', () => {
	it('round-trips through taskSpan', () => {
		const cases = [allDay(day(10, 5)), allDay(day(10, 5), day(10, 8)), timed(day(10, 5, 9)), timed(day(10, 5, 9), day(10, 6, 17))]
		const target = {start: day(11, 2), end: day(11, 4)}
		for (const t of cases) {
			expect(taskSpan(task(spanPatch(t, target)))).toEqual(target)
		}
	})

	it('turns a one-day all-day task into a multi-day one', () => {
		const patch = spanPatch(allDay(day(10, 5)), {start: day(10, 5), end: day(10, 7)})
		const s = getSchedule(task(patch))!
		expect(s.allDay).toBe(true)
		expect(s.start).toEqual(day(10, 5))
		expect(s.end).toEqual(new Date(2026, 9, 7, 23, 59, 59))
	})

	it('shrinks a multi-day all-day task back to a plain due date', () => {
		const patch = spanPatch(allDay(day(10, 5), day(10, 8)), {start: day(10, 6), end: day(10, 6)})
		expect(patch.start_date).toBeNull()
		expect(patch.end_date).toBeNull()
		expect(getSchedule(task(patch))).toMatchObject({allDay: true, date: new Date(2026, 9, 6, 23, 59, 59)})
	})

	it('keeps the times of day of a timed task', () => {
		const patch = spanPatch(timed(day(10, 5, 9, 15), day(10, 6, 17, 45)), {start: day(10, 12), end: day(10, 14)})
		expect(getSchedule(task(patch))).toMatchObject({allDay: false, start: day(10, 12, 9, 15), end: day(10, 14, 17, 45)})
	})

	it('drops the end when one day is left and the end time is before the start time', () => {
		const patch = spanPatch(timed(day(10, 5, 22), day(10, 6, 2)), {start: day(10, 6), end: day(10, 6)})
		expect(patch).toEqual(schedulePatch(day(10, 6, 22), false))
	})
})

describe('timelineRange', () => {
	const today = day(10, 22, 15)

	it('pads around today', () => {
		const {start, days} = timelineRange([], 'day', today)
		expect(start <= day(10, 8)).toBe(true)
		expect(days).toBeGreaterThan(56)
	})

	it('stretches to cover tasks, but not without bound', () => {
		const near = {start: day(1, 3), end: day(1, 3)}
		expect(timelineRange([near], 'week', today).start <= day(1, 3)).toBe(true)
		const far = {start: new Date(2010, 0, 1), end: new Date(2010, 0, 1)}
		expect(timelineRange([far], 'week', today).start.getFullYear()).toBe(2026)
	})

	it('starts on a month boundary at month scale', () => {
		expect(timelineRange([], 'month', today).start.getDate()).toBe(1)
	})
})

describe('groups', () => {
	it('splits at month boundaries with partial edges', () => {
		const g = groups(day(9, 25), 40, 'month')
		expect(g.map(x => [x.offset, x.days])).toEqual([[0, 6], [6, 31], [37, 3]])
		expect(g[1].date).toEqual(day(10, 1))
	})

	it('covers every day exactly once', () => {
		for (const unit of ['day', 'week', 'month', 'year'] as const) {
			const g = groups(day(3, 17), 500, unit)
			expect(g.reduce((n, x) => n + x.days, 0)).toBe(500)
			g.forEach((x, i) => expect(x.offset).toBe(i ? g[i - 1].offset + g[i - 1].days : 0))
		}
	})
})
