// Desktop notifications for task reminders and the daily summary, like
// TickTick's "Web Reminder". They use the Notification API, which the
// Electron app forwards to the OS (libnotify on Linux, Notification Center
// on macOS, toasts on Windows). They only show while the app is open; mail
// reminders and the daily summary mail come from the server.

import {ref} from 'vue'
import type {Task} from '@/api/types'
import {router} from '@/router'
import {useData} from '@/stores/data'
import {desktop} from './desktop'
import {t} from './i18n'
import {prefs} from './prefs'
import {openTaskDrawer} from './ui'

const TICK = 15_000
/** How often a constant reminder shows again until it is dealt with. */
export const CONSTANT_REPEAT = 5 * 60_000
/** Day (YYYY-MM-DD) the daily summary was last shown, shared between tabs. */
const DAILY_KEY = 'tt.dailyNotified'
const SUMMARY_TITLES = 5

export const DEFAULT_DAILY_TIME = '09:00'

export const notificationsSupported = typeof Notification !== 'undefined'

/** 'unsupported' when the browser has no Notification API. */
export const notificationPermission = ref<NotificationPermission | 'unsupported'>(
	notificationsSupported ? Notification.permission : 'unsupported')

export async function requestNotificationPermission(): Promise<boolean> {
	if (!notificationsSupported) return false
	if (Notification.permission === 'default') {
		try {
			await Notification.requestPermission()
		} catch {
			// old Safari only has the callback form
		}
	}
	notificationPermission.value = Notification.permission
	return Notification.permission === 'granted'
}

function canNotify() {
	return notificationsSupported && Notification.permission === 'granted'
}

function bringToFront() {
	window.focus()
	desktop?.focusWindow?.()
}

function show(title: string, options: NotificationOptions, onClick?: () => void): Notification | null {
	if (!canNotify()) return null
	try {
		const n = new Notification(title, {icon: '/favicon.svg', ...options})
		n.onclick = () => {
			bringToFront()
			onClick?.()
			n.close()
		}
		return n
	} catch {
		// some platforms only allow notifications from a service worker
		return null
	}
}

/** Today's date as YYYY-MM-DD in local time. */
function dayKey(d: Date) {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

/** The time today at "HH:MM", or null when it doesn't parse. */
export function timeToday(value: string, now: Date): Date | null {
	const m = /^(\d{1,2}):(\d{2})$/.exec(value)
	if (!m) return null
	const at = new Date(now)
	at.setHours(Number(m[1]), Number(m[2]), 0, 0)
	return at
}

/** Absolute reminder times (ms) of a task that fall in (from, to]. */
export function remindersBetween(task: Task, from: number, to: number): number[] {
	const due: number[] = []
	for (const r of task.reminders ?? []) {
		if (!r.reminder) continue
		const at = Date.parse(r.reminder)
		if (!Number.isNaN(at) && at > from && at <= to) due.push(at)
	}
	return due
}

interface Constant {
	taskId: number
	at: number
	next: number
	notification: Notification | null
}

/**
 * Starts checking for due reminders and the daily summary. Returns a stop
 * function. Only reminders that come due while the app runs are shown.
 */
export function startReminderNotifier(): () => void {
	const data = useData()
	let last = Date.now()
	/** Constant reminders waiting for the task to be opened or completed, by tag. */
	const constant = new Map<string, Constant>()

	function stopConstant(tag: string) {
		const c = constant.get(tag)
		if (!c) return
		constant.delete(tag)
		c.notification?.close()
	}

	function remind(task: Task, at: number) {
		const tag = `reminder-${task.id}-${at}`
		const notification = show(task.title, {
			body: t('reminder'),
			tag,
			requireInteraction: prefs.constantReminder,
		}, () => {
			stopConstant(tag)
			openTaskDrawer(task.id)
		})
		if (prefs.constantReminder) {
			constant.set(tag, {taskId: task.id, at, next: Date.now() + CONSTANT_REPEAT, notification})
		}
	}

	function repeatConstant(now: number) {
		for (const [tag, c] of constant) {
			const task = data.tasks.get(c.taskId)
			// Dealt with: done, deleted, or the reminder was moved.
			if (!prefs.webReminder || !prefs.constantReminder || !task || task.done || !task.reminders?.some(r => r.reminder && Date.parse(r.reminder) === c.at)) {
				stopConstant(tag)
				continue
			}
			if (now < c.next) continue
			c.notification?.close()
			c.next = now + CONSTANT_REPEAT
			c.notification = show(task.title, {
				body: t('reminder'),
				tag,
				requireInteraction: true,
			}, () => {
				stopConstant(tag)
				openTaskDrawer(task.id)
			})
		}
	}

	function dailySummary(now: Date) {
		const settings = data.user?.settings
		if (!settings?.overdue_tasks_reminders_enabled) return
		const at = timeToday(settings.overdue_tasks_reminders_time || DEFAULT_DAILY_TIME, now)
		if (!at || at.getTime() <= last || at > now) return

		const today = dayKey(now)
		try {
			if (localStorage.getItem(DAILY_KEY) === today) return
			localStorage.setItem(DAILY_KEY, today)
		} catch {
			// storage unavailable: show it anyway
		}

		const tasks = data.openTasks.filter(task => data.isToday(task, now))
		if (tasks.length === 0) return
		const lines = tasks.slice(0, SUMMARY_TITLES).map(task => `• ${task.title}`)
		if (tasks.length > SUMMARY_TITLES) lines.push(t('dailySummaryMore', {count: tasks.length - SUMMARY_TITLES}))
		show(tasks.length === 1 ? t('dailySummaryTitleOne') : t('dailySummaryTitle', {count: tasks.length}), {body: lines.join('\n'), tag: `daily-${today}`}, () => {
			router.push('/tasks/today').catch(() => {})
		})
	}

	function tick() {
		const now = Date.now()
		if (data.authed && data.loaded && canNotify()) {
			if (prefs.webReminder) {
				for (const task of data.openTasks) {
					for (const at of remindersBetween(task, last, now)) remind(task, at)
				}
			}
			repeatConstant(now)
			dailySummary(new Date(now))
		}
		last = now
	}

	const timer = window.setInterval(tick, TICK)
	return () => {
		clearInterval(timer)
		for (const tag of [...constant.keys()]) stopConstant(tag)
	}
}
