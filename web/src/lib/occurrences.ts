import type {Task} from '@/api/types'
import {parseDate, toISO} from './dates'

// Projects the upcoming repeats of a task for the calendar, for tasks with
// show_future_occurrences set. This mirrors the server's repeat evaluation
// (pkg/models/task_repeat_rule.go): every occurrence keeps the anchor's
// wall-clock time and all of the task's dates move by the same number of days.

const DAY_MS = 86_400_000
/** Upper bound on periods scanned for one occurrence, like the server's maxRepeatScanPeriods. */
const MAX_SCAN = 1000
/** Upper bound on occurrences projected per task and range. */
const MAX_OCCURRENCES = 1000

type Freq = 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY'

interface ByDay {
	/** nth weekday of the month (negative counts from the end); 0 means every one. */
	n: number
	day: number
}

interface Rule {
	freq: Freq
	interval: number
	byDay: ByDay[]
	byMonthDay: number[]
	/** 0-based months. */
	byMonth: number[]
	weekStart: number
	/** Occurrences left including the current one; 0 means unlimited. */
	count: number
	/** Last day (day number) an occurrence may fall on, or null. */
	untilDay: number | null
	/** Last moment an occurrence may fall on, for UNTIL date-times. */
	untilTime: number | null
}

const DAY_CODES = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']

// Civil-day arithmetic: days since the epoch for a calendar date, independent of DST.
function dayNumber(y: number, m: number, d: number): number {
	return Math.round(Date.UTC(y, m, d) / DAY_MS)
}

function dayOf(date: Date): number {
	return dayNumber(date.getFullYear(), date.getMonth(), date.getDate())
}

function dateOf(day: number): [number, number, number] {
	const d = new Date(day * DAY_MS)
	return [d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()]
}

function weekdayOf(day: number): number {
	return new Date(day * DAY_MS).getUTCDay()
}

function daysInMonth(y: number, m: number): number {
	return new Date(Date.UTC(y, m + 1, 0)).getUTCDate()
}

export function parseRepeatRule(raw: string): Rule | null {
	const rule: Rule = {freq: 'DAILY', interval: 1, byDay: [], byMonthDay: [], byMonth: [], weekStart: 1, count: 0, untilDay: null, untilTime: null}
	let hasFreq = false
	for (const part of raw.trim().replace(/^RRULE:/i, '').toUpperCase().split(';')) {
		if (!part) continue
		const [key, value] = part.split('=')
		if (!value) return null
		switch (key) {
			case 'FREQ':
				if (!['DAILY', 'WEEKLY', 'MONTHLY', 'YEARLY'].includes(value)) return null
				rule.freq = value as Freq
				hasFreq = true
				break
			case 'INTERVAL':
				rule.interval = Math.max(1, parseInt(value, 10) || 1)
				break
			case 'COUNT':
				rule.count = Math.max(0, parseInt(value, 10) || 0)
				break
			case 'UNTIL': {
				const m = value.match(/^(\d{4})(\d{2})(\d{2})(?:T(\d{2})(\d{2})(\d{2})Z?)?$/)
				if (!m) return null
				if (m[4] === undefined) rule.untilDay = dayNumber(+m[1], +m[2] - 1, +m[3])
				else rule.untilTime = Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6])
				break
			}
			case 'WKST':
				if (!DAY_CODES.includes(value)) return null
				rule.weekStart = DAY_CODES.indexOf(value)
				break
			case 'BYDAY':
				for (const item of value.split(',')) {
					const day = DAY_CODES.indexOf(item.slice(-2))
					if (day < 0) return null
					const prefix = item.slice(0, -2)
					rule.byDay.push({n: prefix ? parseInt(prefix, 10) || 0 : 0, day})
				}
				break
			case 'BYMONTHDAY':
				rule.byMonthDay = value.split(',').map(v => parseInt(v, 10)).filter(n => !isNaN(n))
				break
			case 'BYMONTH':
				rule.byMonth = value.split(',').map(v => parseInt(v, 10) - 1).filter(n => n >= 0 && n < 12).sort((a, b) => a - b)
				break
			default:
				return null
		}
	}
	rule.byMonthDay.sort((a, b) => a - b)
	return hasFreq ? rule : null
}

function matchesMonthDay(r: Rule, d: number, n: number): boolean {
	return r.byMonthDay.some(md => md === d || (md < 0 && n + md + 1 === d))
}

function matchesWeekday(r: Rule, y: number, m: number, d: number, n: number): boolean {
	const wd = weekdayOf(dayNumber(y, m, d))
	return r.byDay.some(bd => bd.day === wd && (
		bd.n === 0 ||
		(bd.n > 0 && Math.floor((d - 1) / 7) + 1 === bd.n) ||
		(bd.n < 0 && Math.floor((n - d) / 7) + 1 === -bd.n)
	))
}

/** Days of a month a MONTHLY or YEARLY rule hits, in order. */
function monthDays(r: Rule, y: number, m: number, defaultDay: number): number[] {
	const n = daysInMonth(y, m)
	if (!r.byDay.length && !r.byMonthDay.length) return defaultDay > n ? [] : [defaultDay]
	const out: number[] = []
	for (let d = 1; d <= n; d++) {
		if (r.byMonthDay.length && !matchesMonthDay(r, d, n)) continue
		if (r.byDay.length && !matchesWeekday(r, y, m, d, n)) continue
		out.push(d)
	}
	return out
}

/** The first occurrence of the series starting at anchor that lies strictly after `after`. */
export function nextOccurrence(r: Rule, anchor: Date, after: Date): Date | null {
	if (after < anchor) after = anchor
	const at = (y: number, m: number, d: number) =>
		new Date(y, m, d, anchor.getHours(), anchor.getMinutes(), anchor.getSeconds(), anchor.getMilliseconds())
	const ay = anchor.getFullYear(), am = anchor.getMonth(), ad = anchor.getDate()
	const anchorDay = dayOf(anchor)
	const refDay = dayOf(after)
	const iv = r.interval
	// Skip whole intervals up to the period holding the reference date.
	const skip = (periods: number) => periods <= 0 ? 0 : Math.floor(periods / iv) * iv

	if (r.freq === 'DAILY') {
		const start = skip(refDay - anchorDay)
		for (let i = 0; i < MAX_SCAN; i++) {
			const [y, m, d] = dateOf(anchorDay + start + i * iv)
			const n = daysInMonth(y, m)
			if ((r.byMonth.length && !r.byMonth.includes(m)) ||
				(r.byMonthDay.length && !matchesMonthDay(r, d, n)) ||
				(r.byDay.length && !matchesWeekday(r, y, m, d, n))) continue
			const t = at(y, m, d)
			if (t > after) return t
		}
	} else if (r.freq === 'WEEKLY') {
		const weekOf = (day: number) => day - ((weekdayOf(day) - r.weekStart + 7) % 7)
		const anchorWeek = weekOf(anchorDay)
		const start = skip((weekOf(refDay) - anchorWeek) / 7)
		for (let i = 0; i < MAX_SCAN; i++) {
			const weekStart = anchorWeek + (start + i * iv) * 7
			for (let off = 0; off < 7; off++) {
				const day = weekStart + off
				const wd = weekdayOf(day)
				if (!r.byDay.length && wd !== anchor.getDay()) continue
				if (r.byDay.length && !r.byDay.some(bd => bd.day === wd)) continue
				const [y, m, d] = dateOf(day)
				if (r.byMonth.length && !r.byMonth.includes(m)) continue
				const t = at(y, m, d)
				if (t > after) return t
			}
		}
	} else if (r.freq === 'MONTHLY') {
		const anchorMonth = ay * 12 + am
		const start = skip(after.getFullYear() * 12 + after.getMonth() - anchorMonth)
		for (let i = 0; i < MAX_SCAN; i++) {
			const idx = anchorMonth + start + i * iv
			const y = Math.floor(idx / 12), m = idx % 12
			if (r.byMonth.length && !r.byMonth.includes(m)) continue
			for (const d of monthDays(r, y, m, ad)) {
				const t = at(y, m, d)
				if (t > after) return t
			}
		}
	} else {
		const months = r.byMonth.length ? r.byMonth : [am]
		const start = skip(after.getFullYear() - ay)
		for (let i = 0; i < MAX_SCAN; i++) {
			const y = ay + start + i * iv
			for (const m of months) {
				for (const d of monthDays(r, y, m, ad)) {
					const t = at(y, m, d)
					if (t > after) return t
				}
			}
		}
	}
	return null
}

function allows(r: Rule, t: Date): boolean {
	if (r.untilDay !== null) return dayOf(t) <= r.untilDay
	if (r.untilTime !== null) return t.getTime() <= r.untilTime
	return true
}

function shiftDays(iso: string | null | undefined, days: number): string | null | undefined {
	const d = parseDate(iso)
	if (!d) return iso
	return toISO(new Date(d.getFullYear(), d.getMonth(), d.getDate() + days, d.getHours(), d.getMinutes(), d.getSeconds(), d.getMilliseconds()))
}

function shiftMs(iso: string | null | undefined, ms: number): string | null | undefined {
	const d = parseDate(iso)
	return d ? toISO(new Date(d.getTime() + ms)) : iso
}

/** The rule a task repeats by, including the pre-RRULE repeat_after/repeat_mode forms that are whole days. */
function ruleOf(task: Task): Rule | null {
	if (task.repeat_rule) return parseRepeatRule(task.repeat_rule)
	if (task.repeat_mode === 1) return parseRepeatRule('FREQ=MONTHLY')
	const after = task.repeat_after ?? 0
	if (after > 0 && after % 86400 === 0) return parseRepeatRule(`FREQ=DAILY;INTERVAL=${after / 86400}`)
	return null
}

/**
 * Projected copies of a repeating task whose schedule intersects [from, to), excluding
 * the task itself. Each copy has `occurrence` set and its dates moved to that repeat;
 * it shares the task's id, so opening it opens the real task.
 */
export function futureOccurrences(task: Task, from: Date, to: Date, now = new Date()): Task[] {
	if (!task.show_future_occurrences || task.done) return []
	const due = parseDate(task.due_date)
	const start = parseDate(task.start_date)
	const end = parseDate(task.end_date)
	let anchor = due ?? start ?? end
	if (!anchor) return []
	// How far the task's dates reach before and after the anchor.
	const times = [due, start, end].filter((d): d is Date => !!d).map(d => d.getTime())
	const reachBefore = anchor.getTime() - Math.min(...times)
	const reachAfter = Math.max(...times) - anchor.getTime()

	const out: Task[] = []
	const push = (occurrence: number, shift: (iso: string | null | undefined) => string | null | undefined) => {
		out.push({...task, occurrence, due_date: shift(task.due_date), start_date: shift(task.start_date), end_date: shift(task.end_date)})
	}
	const afterCompletion = task.repeat_mode === 2
	const rule = ruleOf(task)

	if (!rule) {
		// Sub-day repeat_after intervals step in plain seconds.
		const step = (task.repeat_after ?? 0) * 1000
		if (step <= 0) return []
		const first = Math.max(1, Math.ceil((from.getTime() - reachAfter - anchor.getTime()) / step))
		for (let k = first; out.length < MAX_OCCURRENCES; k++) {
			const at = anchor.getTime() + k * step
			if (at - reachBefore >= to.getTime()) break
			push(k, iso => shiftMs(iso, k * step))
		}
		return out
	}

	// The series continues from the anchor, but occurrences that are already past are
	// skipped on completion. Repeats after completion count from the completion day,
	// so an overdue task's next repeats count from today.
	const origin = anchor
	let after = anchor
	if (now > anchor) {
		if (afterCompletion) {
			anchor = new Date(now.getFullYear(), now.getMonth(), now.getDate(), anchor.getHours(), anchor.getMinutes(), anchor.getSeconds(), anchor.getMilliseconds())
			after = anchor
		} else {
			after = now
		}
	}
	// COUNT limits the occurrences left, so they have to be walked from the start;
	// otherwise jump straight to the range.
	let left = rule.count ? rule.count - 1 : Infinity
	if (!rule.count) {
		const jump = new Date(from.getTime() - reachAfter - DAY_MS)
		if (jump > after) after = jump
	}
	for (let i = 0; left > 0 && out.length < MAX_OCCURRENCES && i < 100_000; i++) {
		const next = nextOccurrence(rule, anchor, after)
		if (!next || !allows(rule, next)) break
		after = next
		left--
		if (next.getTime() - reachBefore >= to.getTime()) break
		if (next.getTime() + reachAfter < from.getTime()) continue
		const days = dayOf(next) - dayOf(origin)
		if (days <= 0) continue
		push(days, iso => shiftDays(iso, days))
	}
	return out
}
