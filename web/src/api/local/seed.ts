// Example data for the demo and for "use without an account". Dates are
// relative to today, so the examples always land in Today, Tomorrow, etc.

import type {Project, ProjectView} from '../types'
import type {DB, StoredTask} from './db'

function at(days: number, hour?: number, minute = 0): string {
	const d = new Date()
	d.setDate(d.getDate() + days)
	// All-day tasks are due at the end of their day (dates.ts isAllDayTime).
	if (hour === undefined) d.setHours(23, 59, 59, 0)
	else d.setHours(hour, minute, 0, 0)
	return d.toISOString()
}

function dayKey(days: number): string {
	const d = new Date()
	d.setDate(d.getDate() + days)
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export function seed(): DB {
	let id = 1
	const next = () => id++
	const created = at(-21, 9)
	const user = {id: 1, username: 'demo', name: 'Demo', email: '', settings: {default_project_id: 0}}

	const db: DB = {
		version: 1,
		nextId: 0,
		user,
		projects: [],
		tasks: [],
		labels: [],
		relations: [],
		comments: [],
		positions: {},
		bucketOf: {},
		buckets: [],
		viewBuckets: {},
		filters: [],
		habits: [],
		checkins: {},
		countdowns: [],
		focus: [],
	}

	function project(title: string, hex = '', position = 0): Project {
		const pid = next()
		const views: ProjectView[] = [
			{id: next(), project_id: pid, title: 'List', view_kind: 'list', position: 1},
			{id: next(), project_id: pid, title: 'Kanban', view_kind: 'kanban', position: 2},
		]
		const kanban = views[1].id
		const buckets = ['To do', 'Doing', 'Done'].map((title, i) => ({id: next(), project_view_id: kanban, title, position: (i + 1) * 65536}))
		db.buckets.push(...buckets)
		db.viewBuckets[kanban] = {default_bucket_id: buckets[0].id, done_bucket_id: buckets[2].id}
		const p: Project = {id: pid, title, hex_color: hex, position, views, owner: user, parent_project_id: 0, is_archived: false, is_favorite: false, description: ''}
		db.projects.push(p)
		return p
	}

	const inbox = project('Inbox', '', 0)
	user.settings.default_project_id = inbox.id
	const personal = project('🏡 Personal', '40C057', 65536)
	const work = project('💼 Work', '4772FA', 2 * 65536)
	const shopping = project('🛒 Shopping', 'F76707', 3 * 65536)

	const label = (title: string, hex: string) => {
		const l = {id: next(), title, hex_color: hex, description: ''}
		db.labels.push(l)
		return l
	}
	const family = label('family', 'E64980')
	const errand = label('errand', 'FAB005')
	const deepWork = label('deep-work', '7048E8')

	let index = 1
	function task(projectId: number, title: string, extra: Partial<StoredTask> = {}): StoredTask {
		const t: StoredTask = {
			id: next(),
			title,
			description: '',
			done: false,
			done_at: null,
			wont_do: false,
			due_date: null,
			start_date: null,
			end_date: null,
			priority: 0,
			project_id: projectId,
			label_ids: [],
			reminders: [],
			repeat_after: 0,
			repeat_mode: 0,
			repeat_rule: '',
			hex_color: '',
			is_favorite: false,
			created,
			updated: created,
			index: index++,
			assignees: [],
			attachments: [],
			bucket_id: 0,
			...extra,
		}
		db.tasks.push(t)
		return t
	}

	const welcome = task(inbox.id, 'Welcome to OpenTick 👋 Tap a task to open it', {
		due_date: at(0),
		priority: 3,
		description: 'This is a **demo**: everything you change stays in this browser.\n\n- Check off the subtasks below\n- Drag tasks to reorder them\n- Try the Calendar, Matrix, Habit, Focus and Countdown tabs on the left\n\nWhen you want your tasks on every device, click your avatar at the top left to sign up or self-host.',
	})
	const sub1 = task(inbox.id, 'Add a task with quick add: “Call mum tomorrow 5pm !high #family”', {done: true, done_at: at(0, 8)})
	const sub2 = task(inbox.id, 'Open a task to add notes, subtasks and reminders')
	db.relations.push({task_id: welcome.id, other_task_id: sub1.id, kind: 'subtask'}, {task_id: welcome.id, other_task_id: sub2.id, kind: 'subtask'})
	db.comments.push({id: next(), task_id: welcome.id, comment: 'Comments work too. 💬', author: user, created, updated: created})

	task(inbox.id, 'Press N anywhere to add a task, / to search', {due_date: at(0)})
	task(inbox.id, 'Plan the weekend trip', {due_date: at(2), label_ids: [family.id], priority: 1})

	task(personal.id, 'Water the plants', {due_date: at(0, 18), repeat_rule: 'FREQ=DAILY;INTERVAL=3', reminders: [{reminder: at(0, 18), relative_period: 0, relative_to: ''}]})
	task(personal.id, 'Call mum', {due_date: at(1, 17), label_ids: [family.id], priority: 3})
	task(personal.id, 'Book dentist appointment', {due_date: at(-2), priority: 2})
	task(personal.id, 'Go for a run', {due_date: at(0, 7), done: true, done_at: at(0, 7, 40), repeat_rule: 'FREQ=WEEKLY;BYDAY=MO,WE,FR'})
	task(personal.id, 'Renew passport', {due_date: at(12), priority: 1})
	task(personal.id, 'Read “Deep Work”, chapter 3', {start_date: at(3, 20), due_date: at(3, 20), end_date: at(3, 21)})
	task(personal.id, 'Learn to juggle', {done: true, wont_do: true, done_at: at(-3, 10)})

	task(work.id, 'Review pull requests', {due_date: at(0, 10), priority: 2, label_ids: [deepWork.id]})
	task(work.id, 'Team stand-up', {start_date: at(0, 9, 30), due_date: at(0, 9, 30), end_date: at(0, 9, 45), repeat_rule: 'FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR'})
	task(work.id, 'Write the quarterly report', {due_date: at(-1), priority: 3, label_ids: [deepWork.id]})
	task(work.id, 'Prepare slides for Thursday', {start_date: at(1, 14), due_date: at(1, 14), end_date: at(1, 16), priority: 2})
	task(work.id, '1:1 with Alex', {start_date: at(2, 11), due_date: at(2, 11), end_date: at(2, 11, 30)})
	task(work.id, 'Update the onboarding docs', {due_date: at(6)})
	task(work.id, 'Fix the flaky login test', {done: true, done_at: at(-1, 15)})
	task(work.id, 'Send the invoice to the client', {done: true, done_at: at(-2, 11)})

	task(shopping.id, 'Oat milk', {label_ids: [errand.id]})
	task(shopping.id, 'Coffee beans', {label_ids: [errand.id], priority: 1})
	task(shopping.id, 'Birthday present for Sam', {due_date: at(5), label_ids: [errand.id, family.id], priority: 2})
	task(shopping.id, 'Batteries (AA)', {done: true, done_at: at(-1, 18), label_ids: [errand.id]})

	db.filters.push({id: next(), title: '⚡ High priority', filter: 'done = false && priority in 3, 4, 5', created, updated: created})

	// Habits with a few weeks of history.
	const habits = [
		{title: 'Drink water', icon: '💧', hex_color: '15AABF', goal: 8, unit: 'glasses', frequency: 'daily' as const, weekdays: 127},
		{title: 'Read 20 minutes', icon: '📚', hex_color: '7048E8', goal: 1, unit: '', frequency: 'daily' as const, weekdays: 127},
		{title: 'Workout', icon: '🏃', hex_color: 'F76707', goal: 1, unit: '', frequency: 'weekdays' as const, weekdays: 0b0101010},
	]
	for (const [n, h] of habits.entries()) {
		const hid = next()
		db.habits.push({id: hid, ...h, archived: false, position: (n + 1) * 65536, created, updated: created})
		const days: Record<string, number> = {}
		for (let d = -28; d < 0; d++) {
			const date = new Date()
			date.setDate(date.getDate() + d)
			if (!(h.weekdays & (1 << date.getDay()))) continue
			// A deterministic, mostly-kept streak.
			if ((d * (n + 3)) % 5 === 0 && d < -4) continue
			days[dayKey(d)] = h.goal
		}
		if (n === 0) days[dayKey(0)] = 3
		db.checkins[hid] = days
	}

	const year = new Date().getFullYear()
	const utc = (y: number, m: number, d: number) => new Date(Date.UTC(y, m, d)).toISOString()
	const soon = new Date()
	soon.setDate(soon.getDate() + 24)
	db.countdowns.push(
		{id: next(), title: 'Summer holiday', date: utc(soon.getFullYear(), soon.getMonth(), soon.getDate()), type: 'countdown', repeat_yearly: false, icon: '🏖️', hex_color: '15AABF', created, updated: created},
		{id: next(), title: 'New Year', date: utc(year + 1, 0, 1), type: 'countdown', repeat_yearly: true, icon: '🎉', hex_color: 'FAB005', created, updated: created},
		{id: next(), title: 'Sam’s birthday', date: utc(1994, soon.getMonth(), Math.min(soon.getDate() + 5, 28)), type: 'birthday', repeat_yearly: true, icon: '🎂', hex_color: 'E64980', created, updated: created},
	)

	// Some focus history for the stats on the Focus tab.
	const reviewId = db.tasks.find(t => t.title === 'Review pull requests')!.id
	for (let d = -6; d <= 0; d++) {
		const pomos = d === 0 ? 2 : 1 + ((d + 7) % 4)
		for (let p = 0; p < pomos; p++) {
			const start = new Date(at(d, 9 + p))
			const end = new Date(start.getTime() + 25 * 60_000)
			db.focus.push({id: next(), task_id: p === 0 ? reviewId : 0, type: 'pomo', start_time: start.toISOString(), end_time: end.toISOString(), duration: 25 * 60, note: '', created: end.toISOString(), updated: end.toISOString()})
		}
	}

	db.nextId = id
	return db
}
