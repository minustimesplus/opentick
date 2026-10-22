import type {Task} from '@/api/types'
import {
	addDays, dayDiff, getSchedule, minutesOfDay, schedulePatch, startOfDay, startOfMonth, startOfWeek, withMinutes,
	type SchedulePatch,
} from './dates'

// The timeline (gantt) view works in whole days: every dated task is a bar
// from its first to its last day, whatever the times of day in it.

export type TimelineScale = 'day' | 'week' | 'month'

/** Width of one day in pixels at each scale. */
export const DAY_WIDTH: Record<TimelineScale, number> = {day: 56, week: 22, month: 7}

/** Days shown before and after today when no task reaches further. */
const PAD: Record<TimelineScale, [number, number]> = {day: [14, 42], week: [35, 120], month: [90, 365]}
/** Tasks further away than this from today don't stretch the timeline. */
const MAX_REACH = 730

/** First and last day (both at midnight, inclusive) a task covers. */
export interface Span {
	start: Date
	end: Date
}

function atMidnight(d: Date): boolean {
	return d.getTime() === startOfDay(d).getTime()
}

export function taskSpan(task: Task): Span | null {
	const s = getSchedule(task)
	if (!s) return null
	const start = startOfDay(s.start)
	if (!s.end) return {start, end: start}
	// A timed block ending at midnight does not cover the next day.
	const end = startOfDay(!s.allDay && atMidnight(s.end) ? new Date(s.end.getTime() - 1) : s.end)
	return {start, end: end < start ? start : end}
}

export type DragMode = 'move' | 'start' | 'end'

/** The span after dragging the bar (or one of its edges) by `days`. An edge never crosses the other. */
export function dragSpan(span: Span, mode: DragMode, days: number): Span {
	if (mode === 'move') return {start: addDays(span.start, days), end: addDays(span.end, days)}
	if (mode === 'start') {
		const start = addDays(span.start, days)
		return {start: start > span.end ? span.end : start, end: span.end}
	}
	const end = addDays(span.end, days)
	return {start: span.start, end: end < span.start ? span.start : end}
}

/** Dates that put `task` on `span`, keeping the times of day of a timed task. */
export function spanPatch(task: Task, span: Span): SchedulePatch {
	const s = getSchedule(task)
	if (!s || s.allDay) return schedulePatch(span.start, true, span.end)
	const start = withMinutes(span.start, minutesOfDay(s.start))
	if (!s.end && dayDiff(span.end, span.start) === 0) return schedulePatch(start, false)
	const end = s.end && atMidnight(s.end)
		? addDays(span.end, 1)
		: withMinutes(span.end, minutesOfDay(s.end ?? s.start))
	// Shrunk to one day with the end time before the start time: keep just the start.
	return end > start ? schedulePatch(start, false, end) : schedulePatch(start, false)
}

/** First day and number of days the timeline shows: around today, stretched to cover the tasks. */
export function timelineRange(spans: Span[], scale: TimelineScale, today = new Date()): {start: Date, days: number} {
	const now = startOfDay(today)
	const [before, after] = PAD[scale]
	let start = addDays(now, -before)
	let end = addDays(now, after)
	for (const s of spans) {
		if (s.start < start && dayDiff(now, s.start) <= MAX_REACH) start = s.start
		if (s.end > end && dayDiff(s.end, now) <= MAX_REACH) end = s.end
	}
	start = scale === 'month' ? startOfMonth(addDays(start, -7)) : startOfWeek(addDays(start, -7))
	end = scale === 'month' ? addDays(startOfMonth(addDays(end, 40)), -1) : addDays(startOfWeek(addDays(end, 14)), -1)
	return {start, days: dayDiff(end, start) + 1}
}

export type GroupUnit = 'day' | 'week' | 'month' | 'year'

/** A header cell: `days` days starting `offset` days after the timeline's first day. */
export interface Group {
	date: Date
	offset: number
	days: number
}

function nextBoundary(d: Date, unit: GroupUnit): Date {
	switch (unit) {
		case 'day': return addDays(d, 1)
		case 'week': return addDays(startOfWeek(d), 7)
		case 'month': return new Date(d.getFullYear(), d.getMonth() + 1, 1)
		case 'year': return new Date(d.getFullYear() + 1, 0, 1)
	}
}

/** Splits `days` days from `start` at day, week, month or year boundaries. Edge groups may be partial. */
export function groups(start: Date, days: number, unit: GroupUnit): Group[] {
	const out: Group[] = []
	let offset = 0
	let d = startOfDay(start)
	while (offset < days) {
		const len = Math.min(dayDiff(nextBoundary(d, unit), d), days - offset)
		out.push({date: d, offset, days: len})
		offset += len
		d = addDays(d, len)
	}
	return out
}
