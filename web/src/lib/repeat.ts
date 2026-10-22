import type {Task} from '@/api/types'
import {fmtDayMonth, fmtLongDate, fmtWeekdayLong, fmtWeekdayShort} from './dates'
import {t, type MessageKey} from './i18n'

// Repeats are stored as an RFC 5545 RRULE in `repeat_rule` (the server evaluates it);
// repeat_mode 2 makes the next date count from the completion day. Tasks from before
// repeat rules use repeat_after/repeat_mode, which parseRepeat reads too.

export type RepeatFreq = 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY'
export type MonthMode = 'day' | 'nth' | 'last'

export interface RepeatSpec {
	freq: RepeatFreq
	interval: number
	/** Weekdays (0 = Sunday) for weekly rules; empty means the due date's weekday. */
	weekdays: number[]
	/** Monthly: on the due date's day, on its nth weekday, or on the last day. */
	monthMode: MonthMode
	afterCompletion: boolean
	until: Date | null
	count: number | null
	/** Rule parts the custom editor can't show (BYMONTH, several BYMONTHDAYs, ...). */
	raw: string | null
}

const DAY_CODES = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']
export const WORKDAYS = [1, 2, 3, 4, 5]

export function emptySpec(freq: RepeatFreq = 'DAILY'): RepeatSpec {
	return {freq, interval: 1, weekdays: [], monthMode: 'day', afterCompletion: false, until: null, count: null, raw: null}
}

function parseUntil(v: string): Date | null {
	const m = v.match(/^(\d{4})(\d{2})(\d{2})/)
	return m ? new Date(+m[1], +m[2] - 1, +m[3]) : null
}

/** The nth occurrence of anchor's weekday in its month, or -1 when it is the last one. */
export function weekdayOrdinal(anchor: Date): number {
	const n = Math.floor((anchor.getDate() - 1) / 7) + 1
	const daysInMonth = new Date(anchor.getFullYear(), anchor.getMonth() + 1, 0).getDate()
	return n >= 4 && anchor.getDate() + 7 > daysInMonth ? -1 : n
}

export function parseRule(rule: string, afterCompletion = false): RepeatSpec | null {
	const parts = new Map<string, string>()
	for (const p of rule.replace(/^RRULE:/i, '').toUpperCase().split(';')) {
		const [k, v] = p.split('=')
		if (k && v) parts.set(k, v)
	}
	const freq = parts.get('FREQ') as RepeatFreq | undefined
	if (!freq || !['DAILY', 'WEEKLY', 'MONTHLY', 'YEARLY'].includes(freq)) return null
	const spec = emptySpec(freq)
	spec.interval = parseInt(parts.get('INTERVAL') ?? '1', 10) || 1
	spec.afterCompletion = afterCompletion
	if (parts.has('COUNT')) spec.count = parseInt(parts.get('COUNT')!, 10) || null
	if (parts.has('UNTIL')) spec.until = parseUntil(parts.get('UNTIL')!)

	let editable = true
	for (const k of parts.keys()) {
		if (!['FREQ', 'INTERVAL', 'COUNT', 'UNTIL', 'BYDAY', 'BYMONTHDAY', 'WKST'].includes(k)) editable = false
	}
	const byDay = parts.get('BYDAY')
	const byMonthDay = parts.get('BYMONTHDAY')
	if (freq === 'WEEKLY' && byDay) {
		const days = byDay.split(',').map(d => DAY_CODES.indexOf(d))
		if (days.some(d => d < 0)) editable = false
		spec.weekdays = days.filter(d => d >= 0).sort()
	} else if (freq === 'MONTHLY' && byDay && !byMonthDay && /^-?\d[A-Z]{2}$/.test(byDay)) {
		spec.monthMode = 'nth'
	} else if (freq === 'MONTHLY' && byMonthDay === '-1' && !byDay) {
		spec.monthMode = 'last'
	} else if (byDay || byMonthDay) {
		editable = false
	}
	if (!editable) spec.raw = rule
	return spec
}

/** The repeat a task has, or null when it doesn't repeat. */
export function parseRepeat(task?: Task | null): RepeatSpec | null {
	if (!task) return null
	if (task.repeat_rule) return parseRule(task.repeat_rule, task.repeat_mode === 2)
	if (task.repeat_mode === 1) return emptySpec('MONTHLY')
	const after = task.repeat_after ?? 0
	if (!after) return null
	const spec = emptySpec('DAILY')
	spec.afterCompletion = task.repeat_mode === 2
	if (after % (7 * 86400) === 0) {
		spec.freq = 'WEEKLY'
		spec.interval = after / (7 * 86400)
	} else if (after === 365 * 86400) {
		spec.freq = 'YEARLY'
	} else if (after % 86400 === 0) {
		spec.interval = after / 86400
	} else {
		// Sub-day intervals predate repeat rules; keep them as they are.
		spec.raw = `repeat_after=${after}`
	}
	return spec
}

function fmtUntil(d: Date): string {
	return `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`
}

/** The RRULE for a spec. Monthly nth-weekday rules are pinned to the anchor (the due date). */
export function buildRule(spec: RepeatSpec, anchor: Date): string {
	const parts = [`FREQ=${spec.freq}`]
	if (spec.interval > 1) parts.push(`INTERVAL=${spec.interval}`)
	if (!spec.afterCompletion) {
		if (spec.freq === 'WEEKLY' && spec.weekdays.length) {
			const ordered = [1, 2, 3, 4, 5, 6, 0].filter(d => spec.weekdays.includes(d))
			parts.push(`BYDAY=${ordered.map(d => DAY_CODES[d]).join(',')}`)
		}
		if (spec.freq === 'MONTHLY' && spec.monthMode === 'nth') parts.push(`BYDAY=${weekdayOrdinal(anchor)}${DAY_CODES[anchor.getDay()]}`)
		if (spec.freq === 'MONTHLY' && spec.monthMode === 'last') parts.push('BYMONTHDAY=-1')
	}
	if (spec.count) parts.push(`COUNT=${spec.count}`)
	else if (spec.until) parts.push(`UNTIL=${fmtUntil(spec.until)}`)
	return parts.join(';')
}

/** Task fields for a repeat; null clears it. */
export function repeatPatch(spec: RepeatSpec | null, anchor: Date): Pick<Task, 'repeat_rule' | 'repeat_after' | 'repeat_mode'> {
	if (!spec) return {repeat_rule: '', repeat_after: 0, repeat_mode: 0}
	return {repeat_rule: buildRule(spec, anchor), repeat_after: 0, repeat_mode: spec.afterCompletion ? 2 : 0}
}

// ---- presets ----

export type RepeatPreset = 'none' | 'daily' | 'workdays' | 'weekly' | 'biweekly' | 'monthly' | 'yearly'

export const PRESETS: RepeatPreset[] = ['none', 'daily', 'workdays', 'weekly', 'biweekly', 'monthly', 'yearly']

export function presetSpec(p: RepeatPreset): RepeatSpec | null {
	switch (p) {
		case 'none': return null
		case 'daily': return emptySpec('DAILY')
		case 'workdays': return {...emptySpec('WEEKLY'), weekdays: [...WORKDAYS]}
		case 'weekly': return emptySpec('WEEKLY')
		case 'biweekly': return {...emptySpec('WEEKLY'), interval: 2}
		case 'monthly': return emptySpec('MONTHLY')
		case 'yearly': return emptySpec('YEARLY')
	}
}

/** The preset a spec is, or null for anything custom. */
export function matchPreset(spec: RepeatSpec | null): RepeatPreset | null {
	if (!spec) return 'none'
	const ref = new Date(2000, 0, 3)
	for (const p of PRESETS) {
		const ps = presetSpec(p)
		if (ps && !spec.raw && buildRule(ps, ref) === buildRule({...spec, afterCompletion: false}, ref) && !spec.afterCompletion) return p
	}
	return null
}

export function presetLabel(p: RepeatPreset, anchor: Date): string {
	switch (p) {
		case 'none': return t('repNone')
		case 'daily': return t('repDaily')
		case 'workdays': return t('repWorkdays')
		case 'weekly': return t('repWeeklyOn', {day: fmtWeekdayLong(anchor)})
		case 'biweekly': return t('repBiweekly')
		case 'monthly': return t('repMonthlyOn', {day: anchor.getDate()})
		case 'yearly': return t('repYearlyOn', {date: fmtDayMonth(anchor, false)})
	}
}

const UNIT: Record<RepeatFreq, [MessageKey, MessageKey]> = {
	DAILY: ['unitDay', 'unitDays'],
	WEEKLY: ['unitWeek', 'unitWeeks'],
	MONTHLY: ['unitMonth', 'unitMonths'],
	YEARLY: ['unitYear', 'unitYears'],
}

export function unitLabel(freq: RepeatFreq, n: number): string {
	return t(UNIT[freq][n === 1 ? 0 : 1])
}

function weekdayName(d: number): string {
	// 2000-01-02 was a Sunday.
	return fmtWeekdayShort(new Date(2000, 0, 2 + d))
}

function ordinalLabel(n: number): string {
	return t(n === -1 ? 'ordLast' : (['ord1', 'ord2', 'ord3', 'ord4'] as MessageKey[])[n - 1] ?? 'ord4')
}

/** A one-line, human description of a repeat, e.g. "Every 2 weeks on Mon, Fri, 5 times". */
export function describeRepeat(spec: RepeatSpec | null, anchor: Date | null): string {
	if (!spec) return t('repNone')
	const a = anchor ?? new Date()
	const preset = matchPreset(spec)
	let s: string
	if (preset && preset !== 'none' && anchor) {
		s = presetLabel(preset, a)
	} else if (spec.raw) {
		s = t('repCustom')
	} else {
		s = spec.interval === 1
			? t('repEveryUnit', {unit: unitLabel(spec.freq, 1)})
			: t('repEveryN', {n: spec.interval, unit: unitLabel(spec.freq, spec.interval)})
		if (!spec.afterCompletion) {
			if (spec.freq === 'WEEKLY' && spec.weekdays.length) {
				s += ' · ' + [1, 2, 3, 4, 5, 6, 0].filter(d => spec.weekdays.includes(d)).map(weekdayName).join(', ')
			}
			if (spec.freq === 'MONTHLY') {
				s += ' · ' + (spec.monthMode === 'last'
					? t('repLastDay')
					: spec.monthMode === 'nth'
						? `${ordinalLabel(weekdayOrdinal(a))} ${fmtWeekdayLong(a)}`
						: t('repOnDay', {day: a.getDate()}))
			}
		}
	}
	if (spec.afterCompletion) s += ' · ' + t('repAfterCompletion')
	if (spec.count) s += ' · ' + t('repTimes', {n: spec.count})
	else if (spec.until) s += ' · ' + t('repUntil', {date: fmtLongDate(spec.until)})
	return s
}
