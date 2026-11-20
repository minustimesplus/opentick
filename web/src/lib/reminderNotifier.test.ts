import {describe, expect, it} from 'vitest'
import type {Task} from '@/api/types'
import {remindersBetween, timeToday} from './reminderNotifier'

function task(reminders: Task['reminders']): Task {
	return {id: 1, title: 'T', done: false, project_id: 1, reminders}
}

describe('remindersBetween', () => {
	const at = Date.parse('2026-11-20T09:00:00Z')

	it('returns reminders in the half-open window (from, to]', () => {
		const t = task([{reminder: '2026-11-20T09:00:00Z'}])
		expect(remindersBetween(t, at - 15_000, at)).toEqual([at])
		expect(remindersBetween(t, at, at + 15_000)).toEqual([])
	})

	it('skips empty and unparsable reminders', () => {
		const t = task([{reminder: null}, {reminder: 'nope'}, {}])
		expect(remindersBetween(t, 0, Number.MAX_SAFE_INTEGER)).toEqual([])
	})

	it('handles tasks without reminders', () => {
		expect(remindersBetween(task(null), 0, at)).toEqual([])
	})
})

describe('timeToday', () => {
	const now = new Date(2026, 10, 20, 14, 30)

	it('parses HH:MM and H:MM as local time today', () => {
		expect(timeToday('08:00', now)).toEqual(new Date(2026, 10, 20, 8, 0))
		expect(timeToday('9:30', now)).toEqual(new Date(2026, 10, 20, 9, 30))
	})

	it('rejects anything else', () => {
		expect(timeToday('', now)).toBeNull()
		expect(timeToday('8am', now)).toBeNull()
	})
})
