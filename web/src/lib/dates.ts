import {intlLocale, locale, t} from './i18n'
import {prefs, type DateFormat} from './prefs'
import type {Task} from '@/api/types'

export const MINUTE = 60_000
export const HOUR = 60 * MINUTE
export const DAY = 24 * HOUR

export function parseDate(value?: string | null): Date | null {
	if (!value) return null
	const d = new Date(value)
	if (isNaN(d.getTime()) || d.getFullYear() < 1900) return null
	return d
}

export function startOfDay(d: Date): Date {
	const r = new Date(d)
	r.setHours(0, 0, 0, 0)
	return r
}

export function endOfDay(d: Date): Date {
	const r = new Date(d)
	r.setHours(23, 59, 59, 0)
	return r
}

export function addDays(d: Date, n: number): Date {
	const r = new Date(d)
	r.setDate(r.getDate() + n)
	return r
}

export function addMonths(d: Date, n: number): Date {
	const r = new Date(d)
	const day = r.getDate()
	r.setDate(1)
	r.setMonth(r.getMonth() + n)
	const last = new Date(r.getFullYear(), r.getMonth() + 1, 0).getDate()
	r.setDate(Math.min(day, last))
	return r
}

export function addMinutes(d: Date, n: number): Date {
	return new Date(d.getTime() + n * MINUTE)
}

export function isSameDay(a: Date | null, b: Date | null): boolean {
	return !!a && !!b && a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
}

export function dayDiff(a: Date, b: Date): number {
	return Math.round((startOfDay(a).getTime() - startOfDay(b).getTime()) / DAY)
}

export function startOfWeek(d: Date, weekStart: number = prefs.weekStart): Date {
	const r = startOfDay(d)
	const diff = (r.getDay() - weekStart + 7) % 7
	return addDays(r, -diff)
}

export function startOfMonth(d: Date): Date {
	return new Date(d.getFullYear(), d.getMonth(), 1)
}

export function minutesOfDay(d: Date): number {
	return d.getHours() * 60 + d.getMinutes()
}

export function withMinutes(day: Date, minutes: number): Date {
	const r = startOfDay(day)
	r.setMinutes(minutes)
	return r
}

export function toISO(d: Date | null): string | null {
	return d ? d.toISOString() : null
}

export function dateKey(d: Date): string {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// ---- formatting ----

export function fmtTime(d: Date, hour12: boolean = prefs.hour12): string {
	return new Intl.DateTimeFormat(intlLocale.value, {hour: '2-digit', minute: '2-digit', hour12}).format(d)
}

export function fmtHourLabel(hour: number): string {
	const d = new Date(2000, 0, 1, hour % 24, 0)
	if (prefs.hour12) {
		return new Intl.DateTimeFormat(intlLocale.value, {hour: 'numeric', hour12: true}).format(d)
	}
	return `${String(hour % 24).padStart(2, '0')}:00`
}

export function fmtMonthYear(d: Date): string {
	return new Intl.DateTimeFormat(intlLocale.value, {month: 'long', year: 'numeric'}).format(d)
}

export function fmtMonthShort(d: Date): string {
	return new Intl.DateTimeFormat(intlLocale.value, {month: 'short'}).format(d)
}

export function fmtWeekdayShort(d: Date): string {
	return new Intl.DateTimeFormat(intlLocale.value, {weekday: 'short'}).format(d)
}

export function fmtWeekdayNarrow(d: Date): string {
	return new Intl.DateTimeFormat(intlLocale.value, {weekday: 'narrow'}).format(d)
}

export function fmtWeekdayLong(d: Date): string {
	const s = new Intl.DateTimeFormat(intlLocale.value, {weekday: 'long'}).format(d)
	return s.charAt(0).toUpperCase() + s.slice(1)
}

function pad2(n: number): string {
	return String(n).padStart(2, '0')
}

/** Numeric date in the user's date format: 12/31/2026, 31/12/2026 or 2026/12/31. */
export function fmtNumericDate(d: Date, format: DateFormat = prefs.dateFormat): string {
	const y = d.getFullYear()
	const m = pad2(d.getMonth() + 1)
	const day = pad2(d.getDate())
	switch (format) {
		case 'mdy': return `${m}/${day}/${y}`
		case 'dmy': return `${day}/${m}/${y}`
		default: return `${y}/${m}/${day}`
	}
}

/** "Oct 26" / "26 Oct", or the numeric date when the year is needed. */
export function fmtDayMonth(d: Date, withYear = d.getFullYear() !== new Date().getFullYear()): string {
	if (withYear) return fmtNumericDate(d)
	const month = fmtMonthShort(d)
	return prefs.dateFormat === 'dmy' ? `${d.getDate()} ${month}` : `${month} ${d.getDate()}`
}

export function fmtLongDate(d: Date): string {
	return `${fmtWeekdayShort(d)}${locale.value === 'fr' ? '' : ','} ${fmtDayMonth(d)}`
}

/** Week of the year: ISO 8601 when weeks start on Monday, otherwise the week holding Jan 1 is week 1. */
export function weekNumber(d: Date, weekStart: number = prefs.weekStart): number {
	if (weekStart === 1) {
		const thursday = addDays(startOfWeek(d, 1), 3)
		const firstThursday = addDays(startOfWeek(new Date(thursday.getFullYear(), 0, 4), 1), 3)
		return Math.round(dayDiff(thursday, firstThursday) / 7) + 1
	}
	const start = startOfWeek(d, weekStart)
	const firstWeek = startOfWeek(new Date(addDays(start, 6).getFullYear(), 0, 1), weekStart)
	return Math.round(dayDiff(start, firstWeek) / 7) + 1
}

// ---- additional calendar ----

const CHINESE_DAYS = [
	'初一', '初二', '初三', '初四', '初五', '初六', '初七', '初八', '初九', '初十',
	'十一', '十二', '十三', '十四', '十五', '十六', '十七', '十八', '十九', '二十',
	'廿一', '廿二', '廿三', '廿四', '廿五', '廿六', '廿七', '廿八', '廿九', '三十',
]

const altFormatters = new Map<string, Intl.DateTimeFormat>()

function altFormatter(tag: string, month: 'long' | 'short'): Intl.DateTimeFormat {
	const key = `${tag}|${month}`
	let f = altFormatters.get(key)
	if (!f) {
		f = new Intl.DateTimeFormat(tag, {day: 'numeric', month})
		altFormatters.set(key, f)
	}
	return f
}

/** The day in the user's additional calendar ("初五", "12"), with the month on its first day. */
export function altDayLabel(d: Date): string {
	const cal = prefs.altCalendar
	if (cal === 'none') return ''
	try {
		const chinese = cal === 'chinese'
		const parts = altFormatter(chinese ? 'zh-CN-u-ca-chinese' : `${intlLocale.value}-u-ca-${cal}`, chinese ? 'long' : 'short').formatToParts(d)
		const day = Number(parts.find(p => p.type === 'day')?.value)
		const month = parts.find(p => p.type === 'month')?.value ?? ''
		if (!day) return ''
		if (chinese) return day === 1 ? month : CHINESE_DAYS[day - 1] ?? ''
		return day === 1 ? `${day} ${month}` : String(day)
	} catch {
		// calendar unsupported by this runtime
		return ''
	}
}

/** Relative day name: Today / Tomorrow / Yesterday / weekday (within a week) / "26 Oct". */
export function fmtRelativeDay(d: Date, now = new Date()): string {
	const diff = dayDiff(d, now)
	if (diff === 0) return t('today')
	if (diff === 1) return t('tomorrow')
	if (diff === -1) return t('yesterday')
	if (diff > 1 && diff < 7) return fmtWeekdayShort(d)
	return fmtDayMonth(d)
}

// ---- task scheduling model ----
//
// A task's "date" is its due_date. All-day tasks are stored at 23:59 local
// time (so Vikunja's own overdue logic stays correct). Timed tasks carry the
// exact time in due_date. A duration is expressed through start_date/end_date,
// with due_date equal to the start.

export interface Schedule {
	date: Date
	allDay: boolean
	/** Start of the block (equals date for timed tasks). */
	start: Date
	/** End of the block, null when the task has no duration. */
	end: Date | null
}

export function isAllDayTime(d: Date): boolean {
	return d.getHours() === 23 && d.getMinutes() === 59
}

export function getSchedule(task: Task): Schedule | null {
	const due = parseDate(task.due_date)
	const start = parseDate(task.start_date)
	const end = parseDate(task.end_date)
	if (start && end && end > start) {
		const allDay = start.getHours() === 0 && start.getMinutes() === 0 && isAllDayTime(end)
		return {date: start, allDay, start, end}
	}
	const date = due ?? start
	if (!date) return null
	const allDay = isAllDayTime(date)
	return {date, allDay, start: allDay ? startOfDay(date) : date, end: null}
}

export interface SchedulePatch {
	due_date: string | null
	start_date: string | null
	end_date: string | null
}

export function schedulePatch(date: Date | null, allDay: boolean, end: Date | null = null): SchedulePatch {
	if (!date) return {due_date: null, start_date: null, end_date: null}
	if (allDay) {
		if (end && dayDiff(end, date) > 0) {
			return {due_date: toISO(endOfDay(date)), start_date: toISO(startOfDay(date)), end_date: toISO(endOfDay(end))}
		}
		return {due_date: toISO(endOfDay(date)), start_date: null, end_date: null}
	}
	if (end && end > date) {
		return {due_date: toISO(date), start_date: toISO(date), end_date: toISO(end)}
	}
	return {due_date: toISO(date), start_date: null, end_date: null}
}

/** Moves a task's schedule to a new start keeping its duration. */
export function moveSchedule(s: Schedule, newStart: Date, allDay: boolean): SchedulePatch {
	if (allDay) return schedulePatch(newStart, true)
	if (s.end && !s.allDay) {
		const dur = s.end.getTime() - s.start.getTime()
		return schedulePatch(newStart, false, new Date(newStart.getTime() + dur))
	}
	return schedulePatch(newStart, false)
}

export function isOverdue(task: Task, now = new Date()): boolean {
	if (task.done) return false
	const s = getSchedule(task)
	if (!s) return false
	if (s.allDay) return dayDiff(s.date, now) < 0
	return (s.end ?? s.date) < now
}

/** The compact date label shown at the right of a task row. */
export function taskDateLabel(task: Task, now = new Date()): string {
	const s = getSchedule(task)
	if (!s) return ''
	if (!s.allDay && isSameDay(s.date, now)) return fmtTime(s.date)
	return fmtRelativeDay(s.date, now)
}

/** Detailed label for the detail pane: "Tomorrow, 10:00 - 11:00". */
export function scheduleLabel(s: Schedule | null): string {
	if (!s) return ''
	const day = fmtRelativeDay(s.date)
	if (s.allDay) {
		if (s.end && dayDiff(s.end, s.date) > 0) return `${fmtDayMonth(s.date)} - ${fmtDayMonth(s.end)}`
		return day
	}
	if (s.end) {
		if (isSameDay(s.end, s.date)) return `${day}, ${fmtTime(s.start)} - ${fmtTime(s.end)}`
		return `${day}, ${fmtTime(s.start)} - ${fmtRelativeDay(s.end)}, ${fmtTime(s.end)}`
	}
	return `${day}, ${fmtTime(s.date)}`
}
