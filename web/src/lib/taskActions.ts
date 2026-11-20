import {useData} from '@/stores/data'
import {addDays, getSchedule, minutesOfDay, moveSchedule, schedulePatch, startOfDay, startOfWeek, withMinutes} from './dates'
import {t} from './i18n'
import {toast} from './toast'
import {ui} from './ui'
import type {Level} from './tasks'

/** Task actions shared by the context menu, the detail pane and the keyboard shortcuts. */

/** The task the editing shortcuts act on: the row under the mouse, else the open one. */
export function targetTaskId(): number | null {
	const data = useData()
	for (const id of [ui.hoverTaskId, ui.drawerTaskId, ui.selectedTaskId]) {
		if (id !== null && data.tasks.has(id)) return id
	}
	return null
}

/** Moves a task to day `d`, keeping its time of day if it has one. */
export function setTaskDay(id: number, d: Date) {
	const data = useData()
	const tk = data.tasks.get(id)
	if (!tk) return
	const s = getSchedule(tk)
	const patch = s && !s.allDay
		? moveSchedule(s, withMinutes(d, minutesOfDay(s.start)), false)
		: schedulePatch(d, true)
	data.updateTask(id, patch)
}

export function today(): Date {
	return startOfDay(new Date())
}

export function tomorrow(): Date {
	return addDays(today(), 1)
}

/** Monday (or the first day of the week) of next week. */
export function nextWeek(): Date {
	return addDays(startOfWeek(today(), 1), 7)
}

/** Keeps a task's day but drops its time, making it an all-day task. */
export function clearTaskTime(id: number) {
	const data = useData()
	const tk = data.tasks.get(id)
	const s = tk ? getSchedule(tk) : null
	if (!s || s.allDay) return
	data.updateTask(id, schedulePatch(startOfDay(s.date), true))
}

export function setTaskPriority(id: number, l: Level) {
	useData().updateTask(id, {priority: l})
}

/** Pinned tasks (favorites on the server) stay at the top of their lists. */
export function togglePin(id: number) {
	const data = useData()
	const tk = data.tasks.get(id)
	if (tk) data.updateTask(id, {is_favorite: !tk.is_favorite})
}

/** Moves a task to the trash, with an undo button. */
export function trashTask(id: number) {
	const data = useData()
	const tk = data.tasks.get(id)
	if (!tk) return
	if (ui.drawerTaskId === id) ui.drawerTaskId = null
	data.deleteTask(id)
	toast(t('movedToTrash', {name: tk.title}), {label: t('undo'), run: () => data.restoreTask(id)})
}

/** Opens the date picker for a task, next to its row when it is on screen. */
export function pickTaskDate(id: number) {
	const row = document.querySelector<HTMLElement>(`[data-task-id="${id}"]`)
	const r = row?.getBoundingClientRect()
	const x = r ? r.left + Math.min(r.width / 2, 260) : window.innerWidth / 2 - 130
	const y = r ? r.bottom : window.innerHeight / 3
	ui.ctx = {taskId: id, x, y, date: true}
}
