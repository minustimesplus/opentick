import {describe, expect, it} from 'vitest'
import {parseQuickAdd} from './parse'

const now = new Date(2026, 10, 20, 9, 0)

describe('parseQuickAdd', () => {
	it('reads @username as an assignee', () => {
		const p = parseQuickAdd('Review draft @alice', now)
		expect(p.assignees).toEqual(['alice'])
		expect(p.title).toBe('Review draft')
	})

	it('keeps @ followed by a digit as a time', () => {
		const p = parseQuickAdd('Call @3pm', now)
		expect(p.assignees).toEqual([])
		expect(p.date?.getHours()).toBe(15)
	})

	it('reads a date picked after *', () => {
		const p = parseQuickAdd('Pay rent *tomorrow', now)
		expect(p.title).toBe('Pay rent')
		expect(p.date?.getDate()).toBe(21)
	})
})
