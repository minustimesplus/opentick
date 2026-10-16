import type {Task} from '@/api/types'
import type {CalEvent} from '@/stores/calendars'
import {addDays, getSchedule, minutesOfDay, startOfDay, type Schedule} from '@/lib/dates'

/** Visual length used for tasks that have a time but no duration. */
export const POINT_MINUTES = 25

/** A block on the grid: either a task or a read-only event from a subscribed calendar. */
export interface TimedItem {
	key: string
	task?: Task
	event?: CalEvent
	schedule: Schedule
	/** Minutes from the start of the column day (clamped to 0..1440). */
	startMin: number
	endMin: number
	point: boolean
	col: number
	cols: number
	/** The block continues from the previous / into the next day. */
	clippedStart: boolean
	clippedEnd: boolean
}

function eventSchedule(e: CalEvent): Schedule {
	return {date: e.start, allDay: e.allDay, start: e.start, end: e.end > e.start ? e.end : null}
}

/** Timed tasks and events intersecting `day`, with overlap columns assigned. */
export function layoutDay(tasks: Task[], day: Date, events: CalEvent[] = []): TimedItem[] {
	const dayStart = startOfDay(day)
	const dayEnd = addDays(dayStart, 1)
	const items: TimedItem[] = []
	const add = (s: Schedule, item: Pick<TimedItem, 'key' | 'task' | 'event'>) => {
		const point = !s.end
		const end = s.end ?? new Date(s.start.getTime() + POINT_MINUTES * 60_000)
		if (end <= dayStart || s.start >= dayEnd) return
		const clippedStart = s.start < dayStart
		const clippedEnd = end > dayEnd
		const startMin = clippedStart ? 0 : minutesOfDay(s.start)
		let endMin = clippedEnd ? 1440 : minutesOfDay(end) || 1440
		if (endMin <= startMin) endMin = Math.min(1440, startMin + POINT_MINUTES)
		items.push({...item, schedule: s, startMin, endMin, point, col: 0, cols: 1, clippedStart, clippedEnd})
	}
	for (const task of tasks) {
		const s = getSchedule(task)
		if (s && !s.allDay) add(s, {key: `t${task.id}`, task})
	}
	for (const event of events) {
		if (!event.allDay) add(eventSchedule(event), {key: `e${event.key}`, event})
	}
	items.sort((a, b) => a.startMin - b.startMin || (b.endMin - b.startMin) - (a.endMin - a.startMin) || a.key.localeCompare(b.key))

	// Group into clusters of transitively overlapping items, then pack columns.
	let cluster: TimedItem[] = []
	let clusterEnd = -1
	const flush = () => {
		const colEnds: number[] = []
		for (const it of cluster) {
			let c = colEnds.findIndex(e => e <= it.startMin)
			if (c === -1) {
				c = colEnds.length
				colEnds.push(0)
			}
			colEnds[c] = it.endMin
			it.col = c
		}
		cluster.forEach(it => { it.cols = colEnds.length })
		cluster = []
	}
	for (const it of items) {
		if (cluster.length && it.startMin >= clusterEnd) {
			flush()
			clusterEnd = -1
		}
		cluster.push(it)
		clusterEnd = Math.max(clusterEnd, it.endMin)
	}
	if (cluster.length) flush()
	return items
}

/** All-day events covering `day`. Their end is exclusive. */
export function allDayEventsFor(events: CalEvent[], day: Date): CalEvent[] {
	const d0 = startOfDay(day)
	const d1 = addDays(d0, 1)
	return events.filter(e => e.allDay && e.start < d1 && e.end > d0)
}

/** All-day tasks covering `day`. */
export function allDayFor(tasks: Task[], day: Date): Task[] {
	const d0 = startOfDay(day).getTime()
	return tasks.filter(task => {
		const s = getSchedule(task)
		if (!s || !s.allDay) return false
		const start = startOfDay(s.start).getTime()
		const end = startOfDay(s.end ?? s.date).getTime()
		return d0 >= start && d0 <= end
	})
}

/** Maps minutes of the day to a y offset, with optionally collapsed night hours. */
export interface Scale {
	hourH: number
	collapsed: boolean
	nightH: number
	dayStartMin: number
	dayEndMin: number
}

export function totalHeight(sc: Scale): number {
	if (!sc.collapsed) return 24 * sc.hourH
	return 2 * sc.nightH + ((sc.dayEndMin - sc.dayStartMin) / 60) * sc.hourH
}

export function yOf(min: number, sc: Scale): number {
	if (!sc.collapsed) return (min / 60) * sc.hourH
	if (min <= sc.dayStartMin) return (min / sc.dayStartMin) * sc.nightH
	if (min <= sc.dayEndMin) return sc.nightH + ((min - sc.dayStartMin) / 60) * sc.hourH
	return sc.nightH + ((sc.dayEndMin - sc.dayStartMin) / 60) * sc.hourH + ((min - sc.dayEndMin) / (1440 - sc.dayEndMin)) * sc.nightH
}

export function minOf(y: number, sc: Scale): number {
	if (!sc.collapsed) return (y / sc.hourH) * 60
	const mid = ((sc.dayEndMin - sc.dayStartMin) / 60) * sc.hourH
	if (y <= sc.nightH) return (y / sc.nightH) * sc.dayStartMin
	if (y <= sc.nightH + mid) return sc.dayStartMin + ((y - sc.nightH) / sc.hourH) * 60
	return sc.dayEndMin + ((y - sc.nightH - mid) / sc.nightH) * (1440 - sc.dayEndMin)
}

export function snap(min: number, step = 15): number {
	return Math.max(0, Math.min(1440, Math.round(min / step) * step))
}
