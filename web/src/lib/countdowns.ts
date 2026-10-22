import type {Countdown} from '@/api/types'
import {dayDiff, startOfDay} from './dates'

/** Where a countdown stands relative to today, in the viewer's own timezone. */
export interface CountdownState {
	/** The original date, as a local date. */
	date: Date
	/** The date counted to: the next yearly return for repeating countdowns. */
	target: Date
	/** Days from today to target: positive ahead, 0 today, negative passed. */
	days: number
	/** Days since the original date, when it has passed. */
	daysSince: number | null
	/** Full years from the original date to target, for repeating countdowns. */
	years: number | null
}

/** The calendar date of an API date, as local midnight. Only its UTC date counts. */
export function countdownDate(value: string): Date {
	const d = new Date(value)
	return new Date(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate())
}

/** The API value for a local calendar date: midnight UTC of the same date. */
export function countdownDateValue(d: Date): string {
	return new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate())).toISOString()
}

/** The date in the given year; Feb 29 falls on Feb 28 in other years. */
function inYear(d: Date, year: number): Date {
	const last = new Date(year, d.getMonth() + 1, 0).getDate()
	return new Date(year, d.getMonth(), Math.min(d.getDate(), last))
}

export function countdownState(c: Countdown, now = new Date()): CountdownState {
	const today = startOfDay(now)
	const date = countdownDate(c.date)
	const daysSince = date <= today ? dayDiff(today, date) : null
	let target = date
	let years: number | null = null
	if (c.repeat_yearly && date < today) {
		target = inYear(date, today.getFullYear())
		if (target < today) target = inYear(date, today.getFullYear() + 1)
		years = target.getFullYear() - date.getFullYear()
	}
	return {date, target, days: dayDiff(target, today), daysSince, years}
}

/** Upcoming countdowns soonest first, then passed ones most recent first. */
export function compareCountdowns(a: CountdownState, b: CountdownState): number {
	if ((a.days >= 0) !== (b.days >= 0)) return a.days >= 0 ? -1 : 1
	return a.days >= 0 ? a.days - b.days : b.days - a.days
}
