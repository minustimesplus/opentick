import {describe, expect, it} from 'vitest'
import type {Countdown} from '@/api/types'
import {compareCountdowns, countdownDate, countdownDateValue, countdownState} from './countdowns'

const day = (y: number, m: number, d: number, h = 0) => new Date(y, m - 1, d, h)
const utc = (y: number, m: number, d: number) => new Date(Date.UTC(y, m - 1, d)).toISOString()

function countdown(patch: Partial<Countdown>): Countdown {
	return {id: 1, title: 'C', date: utc(2026, 12, 24), type: 'countdown', repeat_yearly: false, ...patch}
}

describe('countdownDate', () => {
	it('takes the UTC calendar date as a local date', () => {
		expect(countdownDate('2026-12-24T00:00:00Z')).toEqual(day(2026, 12, 24))
	})

	it('ignores the time of day', () => {
		expect(countdownDate('2026-12-24T23:30:00Z')).toEqual(day(2026, 12, 24))
	})
})

describe('countdownDateValue', () => {
	it('sends a local date as midnight UTC of the same date', () => {
		expect(countdownDateValue(day(2026, 12, 24))).toBe('2026-12-24T00:00:00.000Z')
		expect(countdownDateValue(day(2026, 12, 24, 23))).toBe('2026-12-24T00:00:00.000Z')
	})

	it('round-trips through countdownDate', () => {
		for (const d of [day(2026, 1, 1), day(2000, 2, 29), day(1990, 6, 15), day(2026, 3, 29), day(2026, 10, 25)]) {
			expect(countdownDate(countdownDateValue(d))).toEqual(d)
		}
	})
})

describe('countdownState', () => {
	const now = day(2026, 10, 22, 15)

	describe('one-off', () => {
		it('counts the days left to a future date', () => {
			const s = countdownState(countdown({date: utc(2026, 12, 24)}), now)
			expect(s.days).toBe(63)
			expect(s.target).toEqual(day(2026, 12, 24))
			expect(s.daysSince).toBeNull()
			expect(s.years).toBeNull()
		})

		it('is 0 on the day itself, whatever the time', () => {
			expect(countdownState(countdown({date: utc(2026, 10, 22)}), day(2026, 10, 22, 0)).days).toBe(0)
			expect(countdownState(countdown({date: utc(2026, 10, 22)}), day(2026, 10, 22, 23)).days).toBe(0)
		})

		it('is 1 the day before', () => {
			expect(countdownState(countdown({date: utc(2026, 10, 23)}), day(2026, 10, 22, 23)).days).toBe(1)
		})

		it('goes negative once the date has passed', () => {
			const s = countdownState(countdown({date: utc(2026, 9, 1)}), now)
			expect(s.days).toBe(-51)
			expect(s.daysSince).toBe(51)
			expect(s.target).toEqual(day(2026, 9, 1))
		})

		it('counts whole days across a DST change', () => {
			// Europe and the US both leave summer time between these dates.
			expect(countdownState(countdown({date: utc(2026, 11, 10)}), day(2026, 10, 20, 12)).days).toBe(21)
		})
	})

	describe('repeating yearly', () => {
		const birthday = (date: string) => countdown({date, type: 'birthday', repeat_yearly: true})

		it('counts to the return later this year', () => {
			const s = countdownState(birthday(utc(1990, 10, 23)), now)
			expect(s.target).toEqual(day(2026, 10, 23))
			expect(s.days).toBe(1)
			expect(s.years).toBe(36)
		})

		it('counts to next year once this year’s date has passed', () => {
			const s = countdownState(birthday(utc(2018, 6, 2)), now)
			expect(s.target).toEqual(day(2027, 6, 2))
			expect(s.days).toBe(223)
			expect(s.years).toBe(9)
		})

		it('is today on the anniversary itself', () => {
			const s = countdownState(birthday(utc(1990, 10, 22)), now)
			expect(s.target).toEqual(day(2026, 10, 22))
			expect(s.days).toBe(0)
			expect(s.years).toBe(36)
		})

		it('counts the days since the original date', () => {
			expect(countdownState(birthday(utc(2018, 6, 2)), now).daysSince).toBe(3064)
		})

		it('moves Feb 29 to Feb 28 in common years', () => {
			const s = countdownState(birthday(utc(2000, 2, 29)), now)
			expect(s.target).toEqual(day(2027, 2, 28))
			expect(s.years).toBe(27)
		})

		it('keeps Feb 29 in leap years', () => {
			const s = countdownState(birthday(utc(2000, 2, 29)), day(2027, 10, 1))
			expect(s.target).toEqual(day(2028, 2, 29))
			expect(s.years).toBe(28)
		})

		it('counts to a future original date like a one-off', () => {
			const s = countdownState(birthday(utc(2027, 3, 1)), now)
			expect(s.target).toEqual(day(2027, 3, 1))
			expect(s.days).toBe(130)
			expect(s.years).toBeNull()
			expect(s.daysSince).toBeNull()
		})

		it('never goes negative', () => {
			for (let d = 1; d <= 365; d += 7) {
				expect(countdownState(birthday(utc(2001, 1, 1)), day(2026, 1, d)).days).toBeGreaterThanOrEqual(0)
			}
		})
	})
})

describe('compareCountdowns', () => {
	const now = day(2026, 10, 22)
	const state = (date: string) => countdownState(countdown({date}), now)

	it('puts upcoming countdowns soonest first, then passed ones most recent first', () => {
		const states = [
			state(utc(2026, 1, 1)),
			state(utc(2026, 12, 24)),
			state(utc(2026, 10, 22)),
			state(utc(2026, 10, 1)),
			state(utc(2026, 11, 1)),
		]
		expect(states.sort(compareCountdowns).map(s => s.days)).toEqual([0, 10, 63, -21, -294])
	})
})
