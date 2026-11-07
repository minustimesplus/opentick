import {beforeEach, describe, expect, it} from 'vitest'
import type {Paginated} from '../client'
import type {Habit, Project, Task} from '../types'
import {localApi} from './backend'
import {resetDB} from './db'
import {compileFilter, resolveDate} from './filter'

function task(extra: Partial<Task>): Task {
	return {id: 1, title: 'x', done: false, project_id: 1, ...extra}
}

describe('filter', () => {
	it('evaluates the filters the app writes', () => {
		const today = new Date()
		today.setHours(12, 0, 0, 0)
		const t = task({due_date: today.toISOString(), priority: 4, labels: [{id: 7, title: 'a'}], title: 'Buy Milk'})
		expect(compileFilter('done = false && due_date >= now/d && due_date < now/d+1d')(t)).toBe(true)
		expect(compileFilter('done = true || priority in 3, 4, 5')(t)).toBe(true)
		expect(compileFilter('done = false && labels in 7 && project in 1, 2')(t)).toBe(true)
		expect(compileFilter('labels in 8')(t)).toBe(false)
		expect(compileFilter('done = false && (title like \'%milk%\' || description like \'%milk%\')')(t)).toBe(true)
		expect(compileFilter('due_date < now/d')(t)).toBe(false)
		// Tasks without the date never match a date comparison.
		expect(compileFilter('due_date < now/d+30d')(task({}))).toBe(false)
		expect(compileFilter('done = true && done_at > "2020-01-01T00:00:00Z"')(task({done: true, done_at: '2021-05-01T00:00:00Z'}))).toBe(true)
	})

	it('resolves date math', () => {
		const base = new Date(2026, 10, 7, 15, 30)
		expect(new Date(resolveDate('now/d+1d', base)!)).toEqual(new Date(2026, 10, 8))
		expect(new Date(resolveDate('now-7d', base)!)).toEqual(new Date(2026, 9, 31, 15, 30))
	})
})

describe('local backend', () => {
	beforeEach(() => {
		localStorage.clear()
		resetDB()
	})

	it('serves the example data', async () => {
		const user = await localApi<{settings: {default_project_id: number}}>('GET', '/user')
		const projects = await localApi<Paginated<Project>>('GET', '/projects', {query: {is_archived: false}})
		expect(projects.items!.some(p => p.id === user.settings.default_project_id)).toBe(true)
		// The saved filter shows up as a pseudo project with a negative id.
		expect(projects.items!.some(p => p.id < -1)).toBe(true)
		const open = await localApi<Paginated<Task>>('GET', '/tasks', {query: {filter: 'done = false'}})
		expect(open.items!.length).toBeGreaterThan(5)
		expect(open.items!.every(t => !t.done)).toBe(true)
		const welcome = open.items!.find(t => t.title.startsWith('Welcome'))!
		expect(welcome.related_tasks?.subtask?.length).toBe(2)
	})

	it('creates, completes, trashes and restores tasks', async () => {
		const user = await localApi<{settings: {default_project_id: number}}>('GET', '/user')
		const inbox = user.settings.default_project_id
		const created = await localApi<Task>('POST', `/projects/${inbox}/tasks`, {body: {title: 'New', priority: 2}})
		expect(created.title).toBe('New')
		const done = await localApi<Task>('PATCH', `/tasks/${created.id}`, {body: {done: true}})
		expect(done.done).toBe(true)
		expect(done.done_at).toBeTruthy()
		await localApi('DELETE', `/tasks/${created.id}`)
		await expect(localApi('GET', `/tasks/${created.id}`)).rejects.toMatchObject({status: 404})
		const trash = await localApi<Paginated<Task>>('GET', '/tasks/trash')
		expect(trash.items!.map(t => t.id)).toContain(created.id)
		await localApi('POST', `/tasks/trash/${created.id}/restore`)
		expect((await localApi<Task>('GET', `/tasks/${created.id}`)).id).toBe(created.id)
	})

	it('moves a repeating task on instead of closing it', async () => {
		const user = await localApi<{settings: {default_project_id: number}}>('GET', '/user')
		const due = new Date(2026, 0, 5, 9, 0)
		const t = await localApi<Task>('POST', `/projects/${user.settings.default_project_id}/tasks`, {
			body: {title: 'Daily', due_date: due.toISOString(), repeat_rule: 'FREQ=DAILY;INTERVAL=2'},
		})
		const next = await localApi<Task>('PATCH', `/tasks/${t.id}`, {body: {done: true}})
		expect(next.done).toBe(false)
		expect(new Date(next.due_date!)).toEqual(new Date(2026, 0, 7, 9, 0))
	})

	it('tracks habit check-ins', async () => {
		const today = '2026-11-07'
		const h = await localApi<Habit>('POST', '/habits', {body: {title: 'Stretch', goal: 1}, query: {today}})
		const checked = await localApi<Habit>('PUT', `/habits/${h.id}/checkins/${today}`, {body: {value: 1}, query: {today}})
		expect(checked.today_value).toBe(1)
		expect(checked.current_streak).toBe(1)
	})

	it('persists across reloads', async () => {
		const labels = await localApi<{id: number}>('POST', '/labels', {body: {title: 'kept'}})
		const raw = JSON.parse(localStorage.getItem('tt.local.db')!) as {labels: {id: number}[]}
		expect(raw.labels.some(l => l.id === labels.id)).toBe(true)
	})

	it('refuses features that need a server', async () => {
		await expect(localApi('POST', '/tokens', {body: {}})).rejects.toMatchObject({status: 501})
	})
})

describe('local kanban', () => {
	beforeEach(() => {
		localStorage.clear()
		resetDB()
	})

	it('lists buckets and moves tasks into the done column', async () => {
		const projects = await localApi<Paginated<Project>>('GET', '/projects', {query: {is_archived: false}})
		const work = projects.items!.find(p => p.title.includes('Work'))!
		const view = work.views!.find(v => v.view_kind === 'kanban')!
		const base = `/projects/${work.id}/views/${view.id}`
		const detail = await localApi<{default_bucket_id: number, done_bucket_id: number}>('GET', base)
		const buckets = await localApi<{items: {id: number, tasks: Task[]}[]}>('GET', `${base}/buckets/tasks`)
		const todo = buckets.items.find(b => b.id === detail.default_bucket_id)!
		const task = todo.tasks.find(t => !t.done)!
		await localApi('PUT', `${base}/buckets/${detail.done_bucket_id}/tasks`, {body: {task_id: task.id}})
		expect((await localApi<Task>('GET', `/tasks/${task.id}`)).done).toBe(true)
	})
})
