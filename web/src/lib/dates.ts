import {intlLocale, t} from './i18n'
import {prefs} from './prefs'
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

export function fmtTime(d: Date): string {
	return new Intl.DateTimeFormat(intlLocale.value, {hour: '2-digit', minute: '2-digit', hour12: prefs.hour12}).format(d)
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

export function fmtDayMonth(d: Date, withYear = d.getFullYear() !== new Date().getFullYear()): string {
	return new Intl.DateTimeFormat(intlLocale.value, {day: 'numeric', month: 'short', ...(withYear ? {year: 'numeric'} : {})}).format(d)
}

export function fmtLongDate(d: Date): string {
	return new Intl.DateTimeFormat(intlLocale.value, {weekday: 'short', day: 'numeric', month: 'short', ...(d.getFullYear() !== new Date().getFullYear() ? {year: 'numeric'} : {})}).format(d)
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
