import {describe, expect, it} from 'vitest'
import type {Task} from '@/api/types'
import {futureOccurrences} from './occurrences'

const iso = (y: number, m: number, d: number, h = 0, min = 0) => new Date(y, m - 1, d, h, min).toISOString()

function task(p: Partial<Task>): Task {
	return {id: 1, title: 'x', done: false, project_id: 1, show_future_occurrences: true, ...p} as Task
}

const dates = (ts: Task[]) => ts.map(t => new Date(t.due_date!).getDate())
// The ranges below are in March 2030; "now" is before the series starts.
const now = new Date(2030, 0, 1)

describe('futureOccurrences', () => {
	it('is off unless the task opts in', () => {
		const t = task({due_date: iso(2030, 3, 1, 9), repeat_rule: 'FREQ=DAILY', show_future_occurrences: false})
		expect(futureOccurrences(t, new Date(2030, 2, 1), new Date(2030, 2, 8), now)).toEqual([])
	})

	it('projects daily repeats after the task, keeping the time', () => {
		const t = task({due_date: iso(2030, 3, 1, 9, 30), repeat_rule: 'FREQ=DAILY'})
		const occ = futureOccurrences(t, new Date(2030, 2, 1), new Date(2030, 2, 5), now)
		expect(dates(occ)).toEqual([2, 3, 4])
		expect(new Date(occ[0].due_date!).getHours()).toBe(9)
		expect(occ.map(o => o.occurrence)).toEqual([1, 2, 3])
	})

	it('jumps to a later range', () => {
		const t = task({due_date: iso(2030, 1, 6), repeat_rule: 'FREQ=WEEKLY;BYDAY=MO,FR'})
		expect(dates(futureOccurrences(t, new Date(2030, 2, 1), new Date(2030, 2, 15), now))).toEqual([1, 4, 8, 11])
	})

	it('honours COUNT and UNTIL', () => {
		const c = task({due_date: iso(2030, 3, 1), repeat_rule: 'FREQ=DAILY;COUNT=3'})
		expect(dates(futureOccurrences(c, new Date(2030, 2, 1), new Date(2030, 2, 30), now))).toEqual([2, 3])
		const u = task({due_date: iso(2030, 3, 1), repeat_rule: 'FREQ=DAILY;UNTIL=20300304'})
		expect(dates(futureOccurrences(u, new Date(2030, 2, 1), new Date(2030, 2, 30), now))).toEqual([2, 3, 4])
	})

	it('handles monthly last-day and nth weekday rules', () => {
		const last = task({due_date: iso(2030, 1, 31), repeat_rule: 'FREQ=MONTHLY;BYMONTHDAY=-1'})
		expect(futureOccurrences(last, new Date(2030, 1, 1), new Date(2030, 4, 1), now).map(o => new Date(o.due_date!).getDate())).toEqual([28, 31, 30])
		const nth = task({due_date: iso(2030, 1, 8), repeat_rule: 'FREQ=MONTHLY;BYDAY=2TU'})
		expect(dates(futureOccurrences(nth, new Date(2030, 1, 1), new Date(2030, 3, 1), now))).toEqual([12, 12])
	})

	it('moves start and end with the due date', () => {
		const t = task({start_date: iso(2030, 3, 1, 9), end_date: iso(2030, 3, 1, 10), due_date: iso(2030, 3, 1, 9), repeat_rule: 'FREQ=WEEKLY'})
		const [o] = futureOccurrences(t, new Date(2030, 2, 2), new Date(2030, 2, 9), now)
		expect(new Date(o.start_date!).getDate()).toBe(8)
		expect(new Date(o.end_date!).getHours()).toBe(10)
	})

	it('skips repeats that are already past', () => {
		const t = task({due_date: iso(2030, 3, 1, 9), repeat_rule: 'FREQ=DAILY'})
		expect(dates(futureOccurrences(t, new Date(2030, 2, 1), new Date(2030, 2, 8), new Date(2030, 2, 5, 12)))).toEqual([6, 7])
	})

	it('supports legacy repeat_after', () => {
		const t = task({due_date: iso(2030, 3, 1, 9), repeat_after: 2 * 86400})
		expect(dates(futureOccurrences(t, new Date(2030, 2, 1), new Date(2030, 2, 8), now))).toEqual([3, 5, 7])
	})
})
