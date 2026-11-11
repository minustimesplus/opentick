// In-browser stand-in for the /api/v2 server, used by the demo and by
// "use without an account". It answers the requests the app makes with the
// same shapes the server returns, from data kept in localStorage (db.ts).
// Features that need a server (sharing, attachments, calendar feeds, API
// tokens, imports) answer 501 with a hint to sign up or self-host.

import type {Countdown, FocusSession, FocusStats, Habit, HabitStats, Label, Project, RelatedTasks, Task, UserStatistics} from '../types'
import type {RequestOptions} from '../client'
import {ApiError} from '../client'
import {getDB, nextId, now, save, withDB, type DB, type StoredHabit, type StoredTask} from './db'
import {compileFilter} from './filter'
import {parseRepeat} from '@/lib/repeat'
import {t} from '@/lib/i18n'

/** View ids of saved filters' pseudo projects: FILTER_VIEW + filter id. */
const FILTER_VIEW = 1_000_000_000
const STEP = 65536

type Query = NonNullable<RequestOptions['query']>
type Body = Record<string, unknown>

interface Ctx {
	method: string
	parts: string[]
	query: Query
	body: Body
}

function notFound(): never {
	throw new ApiError(404, 'Not found')
}

function unavailable(): never {
	throw new ApiError(501, t('localUnavailable'))
}

function num(v: unknown): number {
	return Number(v)
}

function list<T>(v: unknown): T[] {
	if (v === undefined || v === null) return []
	return (Array.isArray(v) ? v : [v]) as T[]
}

function paginate<T>(items: T[], query: Query) {
	const perPage = Math.max(1, num(query.per_page) || 1000)
	const page = Math.max(1, num(query.page) || 1)
	const total = items.length
	return {
		items: items.slice((page - 1) * perPage, page * perPage),
		page,
		per_page: perPage,
		total,
		total_pages: Math.max(1, Math.ceil(total / perPage)),
	}
}

function dayKey(d: Date): string {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function parseDay(key: string): Date {
	const [y, m, d] = key.split('-').map(Number)
	return new Date(y, m - 1, d)
}

function validDate(v?: string | null): Date | null {
	if (!v) return null
	const d = new Date(v)
	return isNaN(d.getTime()) || d.getFullYear() < 1900 ? null : d
}

// ---------- tasks ----------

function live(db: DB): StoredTask[] {
	return db.tasks.filter(t => !t.deleted_at)
}

function findTask(db: DB, id: number, trashed = false): StoredTask {
	const task = db.tasks.find(t => t.id === id && !!t.deleted_at === trashed)
	if (!task) notFound()
	return task
}

function snapshot(db: DB, s: StoredTask): Task {
	const {label_ids, ...rest} = s
	return {...rest, labels: db.labels.filter(l => label_ids.includes(l.id))}
}

function listViewOf(db: DB, projectId: number): number | undefined {
	return db.projects.find(p => p.id === projectId)?.views?.find(v => v.view_kind === 'list')?.id
}

interface Index {
	byId: Map<number, StoredTask>
	subs: Map<number, number[]>
	parents: Map<number, number[]>
	comments: Map<number, number>
}

/**
 * Lookups for present(), built once per request: large accounts (the offline
 * copy of a server account, see src/api/offline) have thousands of tasks.
 * Requests change the data before they present tasks, never after.
 */
let index: Index | null = null

function indexOf(db: DB): Index {
	if (index) return index
	const push = (m: Map<number, number[]>, k: number, v: number) => {
		const list = m.get(k)
		if (list) list.push(v)
		else m.set(k, [v])
	}
	const out: Index = {byId: new Map(), subs: new Map(), parents: new Map(), comments: new Map()}
	for (const t of db.tasks) if (!t.deleted_at) out.byId.set(t.id, t)
	for (const r of db.relations) {
		push(out.subs, r.task_id, r.other_task_id)
		push(out.parents, r.other_task_id, r.task_id)
	}
	for (const c of db.comments) out.comments.set(c.task_id, (out.comments.get(c.task_id) ?? 0) + 1)
	index = out
	return out
}

function present(db: DB, s: StoredTask, viewId?: number): Task {
	const related: RelatedTasks = {}
	const idx = indexOf(db)
	const subs = (idx.subs.get(s.id) ?? []).map(id => idx.byId.get(id)).filter(Boolean) as StoredTask[]
	const parents = (idx.parents.get(s.id) ?? []).map(id => idx.byId.get(id)).filter(Boolean) as StoredTask[]
	if (subs.length) related.subtask = subs.map(x => snapshot(db, x))
	if (parents.length) related.parenttask = parents.map(x => snapshot(db, x))
	const view = viewId ?? listViewOf(db, s.project_id)
	return {
		...snapshot(db, s),
		related_tasks: related,
		comment_count: s.comment_count ?? idx.comments.get(s.id) ?? 0,
		created_by: s.created_by ?? db.user,
		identifier: s.identifier || `#${s.index ?? s.id}`,
		position: (view !== undefined ? db.positions[view]?.[s.id] : undefined) ?? (s.index ?? s.id) * STEP,
	}
}

function sortTasks(tasks: Task[], query: Query): Task[] {
	const by = list<string>(query.sort_by)
	const order = list<string>(query.order_by)
	if (!by.length) by.push('id')
	const value = (task: Task, field: string): number | string | null => {
		const v = (task as unknown as Record<string, unknown>)[field]
		if (field.endsWith('_date') || field === 'done_at' || field === 'created' || field === 'updated') {
			return validDate(v as string)?.getTime() ?? null
		}
		if (typeof v === 'string') return v.toLowerCase()
		return typeof v === 'number' ? v : v ? 1 : 0
	}
	return [...tasks].sort((a, b) => {
		for (const [i, field] of by.entries()) {
			const desc = order[i] === 'desc'
			const x = value(a, field)
			const y = value(b, field)
			if (x === y) continue
			// Empty values last, whatever the order.
			if (x === null) return 1
			if (y === null) return -1
			return (x < y ? -1 : 1) * (desc ? -1 : 1)
		}
		return 0
	})
}

function queryTasks(db: DB, source: StoredTask[], query: Query, viewId?: number) {
	const match = compileFilter(query.filter as string | undefined)
	const q = String(query.q ?? '').trim().toLowerCase()
	let tasks = source.map(s => present(db, s, viewId)).filter(match)
	if (q) tasks = tasks.filter(x => x.title.toLowerCase().includes(q) || (x.description ?? '').toLowerCase().includes(q))
	return paginate(sortTasks(tasks, query), query)
}

const TASK_FIELDS = [
	'title', 'description', 'done', 'done_at', 'wont_do', 'due_date', 'start_date', 'end_date', 'priority', 'project_id',
	'reminders', 'repeat_after', 'repeat_mode', 'repeat_rule', 'hex_color', 'is_favorite', 'percent_done', 'bucket_id',
] as const

function applyTaskFields(task: StoredTask, body: Body) {
	for (const key of TASK_FIELDS) {
		if (key in body) (task as unknown as Record<string, unknown>)[key] = body[key]
	}
}

/** The date a repeating task moves to, or null when the repeat has ended. */
function nextOccurrence(task: StoredTask, anchor: Date): Date | null {
	const spec = parseRepeat(task as unknown as Task)
	const fromNow = task.repeat_mode === 2 || !!spec?.afterCompletion
	const base = new Date(anchor)
	if (fromNow) {
		const today = new Date()
		base.setFullYear(today.getFullYear(), today.getMonth(), today.getDate())
	}
	if (!spec) {
		if (!task.repeat_after) return null
		return new Date(base.getTime() + task.repeat_after * 1000)
	}
	const next = new Date(base)
	const n = spec.interval
	switch (spec.freq) {
	case 'DAILY':
		next.setDate(next.getDate() + n)
		break
	case 'WEEKLY':
		if (spec.weekdays.length && !fromNow) {
			do {
				next.setDate(next.getDate() + 1)
				// Starting a new week skips the weeks in between.
				if (next.getDay() === 1 && n > 1) next.setDate(next.getDate() + 7 * (n - 1))
			} while (!spec.weekdays.includes(next.getDay()))
		} else {
			next.setDate(next.getDate() + 7 * n)
		}
		break
	case 'MONTHLY': {
		const day = base.getDate()
		next.setDate(1)
		next.setMonth(next.getMonth() + n)
		const last = new Date(next.getFullYear(), next.getMonth() + 1, 0).getDate()
		if (spec.monthMode === 'last') next.setDate(last)
		else if (spec.monthMode === 'nth') {
			const ordinal = Math.floor((day - 1) / 7)
			const first = (base.getDay() - next.getDay() + 7) % 7 + 1
			next.setDate(Math.min(first + 7 * ordinal, last))
		} else next.setDate(Math.min(day, last))
		break
	}
	case 'YEARLY':
		next.setFullYear(next.getFullYear() + n)
		break
	}
	if (spec.until && next > new Date(spec.until.getTime() + 86_400_000)) return null
	return next
}

/** Completing a repeating task moves its dates on instead of closing it. */
function completeRepeat(task: StoredTask): boolean {
	if (!task.repeat_rule && !task.repeat_after) return false
	const anchor = validDate(task.due_date) ?? validDate(task.start_date) ?? validDate(task.end_date)
	if (!anchor) return false
	const next = nextOccurrence(task, anchor)
	if (!next) return false
	const delta = next.getTime() - anchor.getTime()
	const shift = (v?: string | null) => {
		const d = validDate(v)
		return d ? new Date(d.getTime() + delta).toISOString() : v
	}
	task.due_date = shift(task.due_date)
	task.start_date = shift(task.start_date)
	task.end_date = shift(task.end_date)
	task.reminders = (task.reminders ?? []).map(r => r.reminder ? {...r, reminder: shift(r.reminder)} : r)
	task.done = false
	task.wont_do = false
	task.done_at = null
	return true
}

function newTask(db: DB, projectId: number, body: Body): StoredTask {
	if (!db.projects.some(p => p.id === projectId)) notFound()
	const stamp = now()
	const index = Math.max(0, ...db.tasks.filter(x => x.project_id === projectId).map(x => x.index ?? 0)) + 1
	const task: StoredTask = {
		id: nextId(),
		title: '',
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
		created: stamp,
		updated: stamp,
		index,
		assignees: [],
		attachments: [],
		bucket_id: 0,
	}
	applyTaskFields(task, body)
	task.project_id = projectId
	if (task.done && !task.done_at) task.done_at = stamp
	db.tasks.push(task)
	return task
}

function trashTask(db: DB, task: StoredTask, stamp: string) {
	task.deleted_at = stamp
	// Subtasks go with their parent.
	for (const r of db.relations.filter(r => r.task_id === task.id)) {
		const sub = db.tasks.find(x => x.id === r.other_task_id && !x.deleted_at)
		if (sub) trashTask(db, sub, stamp)
	}
}

function purgeTask(db: DB, id: number) {
	db.tasks = db.tasks.filter(x => x.id !== id)
	db.relations = db.relations.filter(r => r.task_id !== id && r.other_task_id !== id)
	db.comments = db.comments.filter(c => c.task_id !== id)
	db.focus = db.focus.map(f => f.task_id === id ? {...f, task_id: 0} : f)
}

function tasksRoute(db: DB, c: Ctx): unknown {
	const [, a, b, c2, d] = c.parts
	const m = c.method

	if (a === undefined) {
		if (m === 'GET') return queryTasks(db, live(db), c.query)
		unavailable()
	}

	if (a === 'trash') {
		if (b === undefined && m === 'GET') {
			const trashed = db.tasks.filter(x => x.deleted_at).map(x => ({...present(db, x), deleted_at: x.deleted_at}))
			return paginate(sortTasks(trashed, {sort_by: 'deleted_at', order_by: 'desc'}), c.query)
		}
		if (b === 'empty' && m === 'POST') {
			db.tasks.filter(x => x.deleted_at).forEach(x => purgeTask(db, x.id))
			return {}
		}
		const task = findTask(db, num(b), true)
		if (c2 === 'restore' && m === 'POST') {
			const stamp = task.deleted_at
			for (const x of db.tasks) if (x.deleted_at === stamp) x.deleted_at = null
			if (!db.projects.some(p => p.id === task.project_id)) task.project_id = db.user.settings.default_project_id
			return {}
		}
		if (m === 'DELETE') {
			purgeTask(db, task.id)
			return {}
		}
		notFound()
	}

	const task = findTask(db, num(a))

	if (b === undefined) {
		if (m === 'GET') return present(db, task)
		if (m === 'PATCH' || m === 'POST') {
			const wasDone = task.done
			const projectBefore = task.project_id
			applyTaskFields(task, c.body)
			if ('project_id' in c.body && !db.projects.some(p => p.id === task.project_id)) task.project_id = projectBefore
			if (task.done && !wasDone) {
				if (!completeRepeat(task)) task.done_at ||= now()
			}
			if (!task.done) {
				task.done_at = null
				task.wont_do = false
			}
			task.updated = now()
			return present(db, task)
		}
		if (m === 'DELETE') {
			trashTask(db, task, now())
			return {}
		}
	}

	if (b === 'duplicate' && m === 'POST') {
		const copy = newTask(db, task.project_id, {...task, done: false, done_at: null, wont_do: false})
		copy.label_ids = [...task.label_ids]
		const parent = db.relations.find(r => r.other_task_id === task.id)
		if (parent) db.relations.push({task_id: parent.task_id, other_task_id: copy.id, kind: 'subtask'})
		return {duplicated_task: present(db, copy)}
	}

	if (b === 'position' && m === 'PUT') {
		const view = num(c.body.project_view_id)
		;(db.positions[view] ??= {})[task.id] = num(c.body.position)
		return {}
	}

	if (b === 'labels') {
		if (m === 'POST') {
			const id = num(c.body.label_id)
			if (!db.labels.some(l => l.id === id)) notFound()
			if (!task.label_ids.includes(id)) task.label_ids.push(id)
			return {label_id: id}
		}
		if (m === 'DELETE') {
			task.label_ids = task.label_ids.filter(l => l !== num(c2))
			return {}
		}
	}

	if (b === 'assignees') {
		if (m === 'POST') {
			if (num(c.body.user_id) !== db.user.id) notFound()
			if (!task.assignees?.some(u => u.id === db.user.id)) task.assignees = [...(task.assignees ?? []), {id: db.user.id, username: db.user.username, name: db.user.name}]
			return {}
		}
		if (m === 'DELETE') {
			task.assignees = (task.assignees ?? []).filter(u => u.id !== num(c2))
			return {}
		}
	}

	if (b === 'relations') {
		if (m === 'POST' && c.body.relation_kind === 'subtask') {
			const child = num(c.body.other_task_id)
			findTask(db, child)
			db.relations = db.relations.filter(r => r.other_task_id !== child)
			db.relations.push({task_id: task.id, other_task_id: child, kind: 'subtask'})
			return {}
		}
		if (m === 'DELETE' && c2 === 'parenttask') {
			db.relations = db.relations.filter(r => !(r.task_id === num(d) && r.other_task_id === task.id))
			return {}
		}
		if (m === 'DELETE' && c2 === 'subtask') {
			db.relations = db.relations.filter(r => !(r.task_id === task.id && r.other_task_id === num(d)))
			return {}
		}
	}

	if (b === 'comments') {
		if (c2 === undefined) {
			if (m === 'GET') return paginate(db.comments.filter(x => x.task_id === task.id), c.query)
			if (m === 'POST') {
				const stamp = now()
				const comment = {id: nextId(), task_id: task.id, comment: String(c.body.comment ?? ''), author: db.user, created: stamp, updated: stamp}
				db.comments.push(comment)
				if (task.comment_count !== undefined) task.comment_count++
				return comment
			}
		}
		const comment = db.comments.find(x => x.id === num(c2) && x.task_id === task.id) ?? notFound()
		if (m === 'PUT' || m === 'PATCH') {
			comment.comment = String(c.body.comment ?? '')
			comment.updated = now()
			return comment
		}
		if (m === 'DELETE') {
			db.comments = db.comments.filter(x => x !== comment)
			if (task.comment_count) task.comment_count--
			return {}
		}
	}

	if (b === 'attachments') {
		if (m === 'GET' && c2 === undefined) return paginate([], c.query)
		if (m === 'GET') notFound()
		unavailable()
	}

	notFound()
}

// ---------- projects ----------

function filterProject(f: DB['filters'][number]): Project {
	if (f.project) return {...f.project, title: f.title}
	const id = -f.id - 1
	return {
		id,
		title: f.title,
		position: 0,
		views: [{id: FILTER_VIEW + f.id, project_id: id, title: 'List', view_kind: 'list', position: 1}],
		is_archived: false,
	}
}

function newProject(db: DB, body: Body): Project {
	const id = nextId()
	const views = [
		{id: nextId(), project_id: id, title: 'List', view_kind: 'list', position: 1},
		{id: nextId(), project_id: id, title: 'Kanban', view_kind: 'kanban', position: 2},
	]
	const buckets = ['To do', 'Doing', 'Done'].map((title, i) => ({id: nextId(), project_view_id: views[1].id, title, position: (i + 1) * STEP}))
	db.buckets.push(...buckets)
	db.viewBuckets[views[1].id] = {default_bucket_id: buckets[0].id, done_bucket_id: buckets[2].id}
	const project: Project = {
		id,
		title: String(body.title ?? ''),
		description: String(body.description ?? ''),
		hex_color: String(body.hex_color ?? ''),
		parent_project_id: num(body.parent_project_id) || 0,
		position: num(body.position) || (Math.max(0, ...db.projects.map(p => p.position ?? 0)) + STEP),
		is_archived: false,
		is_favorite: !!body.is_favorite,
		views,
		owner: db.user,
	}
	db.projects.push(project)
	return project
}

function deleteProject(db: DB, id: number) {
	for (const child of db.projects.filter(p => p.parent_project_id === id)) deleteProject(db, child.id)
	db.tasks.filter(x => x.project_id === id).forEach(x => purgeTask(db, x.id))
	db.projects = db.projects.filter(p => p.id !== id)
}

function viewTasks(db: DB, projectId: number, viewId: number, query: Query) {
	if (projectId < -1) {
		const filter = db.filters.find(f => f.id === -projectId - 1) ?? notFound()
		const saved = compileFilter(filter.filter)
		const source = live(db).filter(s => saved(present(db, s, viewId)))
		return queryTasks(db, source, query, viewId)
	}
	return queryTasks(db, live(db).filter(x => x.project_id === projectId), query, viewId)
}

function kanbanBucket(db: DB, viewId: number, task: StoredTask): number {
	const ids = db.viewBuckets[viewId]
	if (task.done && ids?.done_bucket_id) return ids.done_bucket_id
	const placed = db.bucketOf[viewId]?.[task.id]
	if (placed && db.buckets.some(b => b.id === placed) && placed !== ids?.done_bucket_id) return placed
	return ids?.default_bucket_id ?? 0
}

function kanbanRoute(db: DB, c: Ctx, project: Project, viewId: number): unknown {
	const [, , , , what, bucketId, sub] = c.parts
	const m = c.method
	const ids = db.viewBuckets[viewId] ?? notFound()
	const buckets = db.buckets.filter(b => b.project_view_id === viewId)

	if (what === undefined && m === 'GET') {
		const view = project.views?.find(v => v.id === viewId)
		return {...view, ...ids}
	}
	if (what !== 'buckets') notFound()

	if (bucketId === 'tasks' && m === 'GET') {
		if ((num(c.query.page) || 1) > 1) return {items: []}
		const tasks = live(db).filter(x => x.project_id === project.id)
		return {
			items: buckets.map(b => {
				const inside = tasks.filter(x => kanbanBucket(db, viewId, x) === b.id).map(x => present(db, x, viewId))
				return {...b, count: inside.length, tasks: sortTasks(inside, {sort_by: 'position'})}
			}),
		}
	}
	if (bucketId === undefined && m === 'POST') {
		const bucket = {id: nextId(), project_view_id: viewId, title: String(c.body.title ?? ''), position: num(c.body.position) || STEP}
		db.buckets.push(bucket)
		return {...bucket, count: 0, tasks: []}
	}
	const bucket = buckets.find(b => b.id === num(bucketId)) ?? notFound()
	if (sub === 'tasks' && m === 'PUT') {
		const task = findTask(db, num(c.body.task_id))
		;(db.bucketOf[viewId] ??= {})[task.id] = bucket.id
		if (bucket.id === ids.done_bucket_id && !task.done) {
			task.done = true
			task.done_at = now()
		} else if (bucket.id !== ids.done_bucket_id && task.done) {
			task.done = false
			task.done_at = null
		}
		return {}
	}
	if (sub === undefined && (m === 'PUT' || m === 'POST')) {
		if ('title' in c.body) bucket.title = String(c.body.title)
		if ('position' in c.body) bucket.position = num(c.body.position)
		return bucket
	}
	if (sub === undefined && m === 'DELETE') {
		if (buckets.length <= 1) throw new ApiError(412, 'A board needs at least one column.')
		db.buckets = db.buckets.filter(b => b !== bucket)
		const rest = db.buckets.filter(b => b.project_view_id === viewId)
		if (ids.default_bucket_id === bucket.id) ids.default_bucket_id = rest[0].id
		if (ids.done_bucket_id === bucket.id) ids.done_bucket_id = 0
		return {}
	}
	notFound()
}

function projectsRoute(db: DB, c: Ctx): unknown {
	const [, a, b, view, sub] = c.parts
	const m = c.method

	if (a === undefined) {
		if (m === 'GET') {
			const archived = c.query.is_archived === true || c.query.is_archived === 'true'
			const projects = archived ? db.projects : db.projects.filter(p => !p.is_archived)
			return paginate([...projects, ...(archived ? [] : db.filters.map(filterProject))], c.query)
		}
		if (m === 'POST' || m === 'PUT') return newProject(db, c.body)
	}

	const id = num(a)
	if (id < -1) {
		if (b === 'views' && sub === 'tasks' && m === 'GET') return viewTasks(db, id, num(view), c.query)
		notFound()
	}
	const project = db.projects.find(p => p.id === id) ?? notFound()

	if (b === undefined) {
		if (m === 'GET') return project
		if (m === 'PATCH' || m === 'POST') {
			for (const key of ['title', 'description', 'hex_color', 'parent_project_id', 'position', 'is_archived', 'is_favorite'] as const) {
				if (key in c.body) (project as unknown as Record<string, unknown>)[key] = c.body[key]
			}
			return project
		}
		if (m === 'DELETE') {
			if (project.id === db.user.settings.default_project_id) throw new ApiError(412, 'The inbox cannot be deleted.')
			deleteProject(db, project.id)
			return {}
		}
	}

	if (b === 'tasks') {
		if (m === 'GET') return queryTasks(db, live(db).filter(x => x.project_id === id), c.query)
		if (m === 'POST' || m === 'PUT') return present(db, newTask(db, id, c.body))
	}

	if (b === 'views') {
		const viewId = num(view)
		const kind = project.views?.find(v => v.id === viewId)?.view_kind ?? notFound()
		if (sub === 'tasks' && m === 'GET') return viewTasks(db, id, viewId, c.query)
		if (kind === 'kanban') return kanbanRoute(db, c, project, viewId)
		notFound()
	}

	if (b === 'users') {
		if (m === 'GET' && view === 'search') return paginate([db.user], c.query)
		if (m === 'GET') return paginate([], c.query)
		unavailable()
	}

	notFound()
}

function filtersRoute(db: DB, c: Ctx): unknown {
	const [, a] = c.parts
	const body = c.body as {title?: string, filters?: {filter?: string}}
	if (a === undefined && (c.method === 'POST' || c.method === 'PUT')) {
		const stamp = now()
		const f = {id: nextId(), title: body.title ?? '', filter: body.filters?.filter ?? '', created: stamp, updated: stamp}
		db.filters.push(f)
		return {id: f.id, title: f.title, filters: {filter: f.filter, filter_include_nulls: false}}
	}
	const f = db.filters.find(x => x.id === num(a)) ?? notFound()
	if (c.method === 'GET') return {id: f.id, title: f.title, filters: {filter: f.filter, filter_include_nulls: false}}
	if (c.method === 'PUT' || c.method === 'POST' || c.method === 'PATCH') {
		if (body.title !== undefined) f.title = body.title
		if (body.filters?.filter !== undefined) f.filter = body.filters.filter
		f.updated = now()
		return {id: f.id, title: f.title, filters: {filter: f.filter, filter_include_nulls: false}}
	}
	if (c.method === 'DELETE') {
		db.filters = db.filters.filter(x => x !== f)
		return {}
	}
	notFound()
}

function labelsRoute(db: DB, c: Ctx): unknown {
	const [, a] = c.parts
	if (a === undefined) {
		if (c.method === 'GET') return paginate(db.labels, c.query)
		if (c.method === 'POST' || c.method === 'PUT') {
			const label: Label = {id: nextId(), title: String(c.body.title ?? ''), hex_color: String(c.body.hex_color ?? ''), description: ''}
			db.labels.push(label)
			return label
		}
	}
	const label = db.labels.find(l => l.id === num(a)) ?? notFound()
	if (c.method === 'GET') return label
	if (c.method === 'PUT' || c.method === 'POST' || c.method === 'PATCH') {
		if ('title' in c.body) label.title = String(c.body.title)
		if ('hex_color' in c.body) label.hex_color = String(c.body.hex_color)
		return label
	}
	if (c.method === 'DELETE') {
		db.labels = db.labels.filter(l => l !== label)
		for (const x of db.tasks) x.label_ids = x.label_ids.filter(l => l !== label.id)
		return {}
	}
	notFound()
}

// ---------- habits ----------

function isDue(h: StoredHabit, d: Date): boolean {
	return !!(h.weekdays & (1 << d.getDay()))
}

function streak(h: StoredHabit, days: Record<string, number>, today: Date): number {
	let n = 0
	const d = new Date(today)
	// An unfinished today does not break the streak yet.
	if (!((days[dayKey(d)] ?? 0) >= h.goal)) d.setDate(d.getDate() - 1)
	for (let i = 0; i < 3650; i++) {
		if (isDue(h, d)) {
			if ((days[dayKey(d)] ?? 0) >= h.goal) n++
			else break
		}
		d.setDate(d.getDate() - 1)
		if (h.created && d < new Date(h.created) && n === 0) break
	}
	return n
}

function presentHabit(db: DB, h: StoredHabit, todayKey: string): Habit {
	const days = db.checkins[h.id] ?? {}
	const today = parseDay(todayKey)
	const recent = []
	for (let i = 6; i >= 0; i--) {
		const d = new Date(today)
		d.setDate(d.getDate() - i)
		const key = dayKey(d)
		if (days[key]) recent.push({habit_id: h.id, date: key, value: days[key]})
	}
	return {...h, today_value: days[todayKey] ?? 0, current_streak: streak(h, days, today), recent_checkins: recent}
}

function habitStats(db: DB, h: StoredHabit, todayKey: string, monthKey?: string): HabitStats {
	const days = db.checkins[h.id] ?? {}
	const today = parseDay(todayKey)
	const month = monthKey ?? todayKey.slice(0, 7)
	const [y, mo] = month.split('-').map(Number)
	const monthDays = []
	for (let d = new Date(y, mo - 1, 1); d.getMonth() === mo - 1; d.setDate(d.getDate() + 1)) {
		const key = dayKey(d)
		const value = days[key] ?? 0
		monthDays.push({date: key, value, due: isDue(h, d), completed: value >= h.goal})
	}
	const keys = Object.keys(days).filter(k => days[k] > 0).sort()
	let longest = 0
	let run = 0
	let dueDays = 0
	let completed = 0
	if (keys.length) {
		for (let d = parseDay(keys[0]); d <= today; d.setDate(d.getDate() + 1)) {
			if (!isDue(h, d)) continue
			dueDays++
			if ((days[dayKey(d)] ?? 0) >= h.goal) {
				completed++
				longest = Math.max(longest, ++run)
			} else if (dayKey(d) !== todayKey) run = 0
		}
	}
	return {
		habit_id: h.id,
		today: todayKey,
		current_streak: streak(h, days, today),
		longest_streak: longest,
		completed_days: completed,
		due_days: dueDays,
		completion_rate: dueDays ? completed / dueDays : 0,
		total_checkins: keys.length,
		total_value: keys.reduce((s, k) => s + days[k], 0),
		month,
		days: monthDays,
	}
}

const HABIT_FIELDS = ['title', 'icon', 'hex_color', 'frequency', 'weekdays', 'goal', 'unit', 'archived'] as const

function habitsRoute(db: DB, c: Ctx): unknown {
	const [, a, b, date] = c.parts
	const todayKey = String(c.query.today ?? dayKey(new Date()))
	if (a === undefined) {
		if (c.method === 'GET') return paginate(db.habits.map(h => presentHabit(db, h, todayKey)), c.query)
		if (c.method === 'POST') {
			const stamp = now()
			const h: StoredHabit = {id: nextId(), title: '', icon: '✅', hex_color: '', frequency: 'daily', weekdays: 127, goal: 1, unit: '', archived: false, created: stamp, updated: stamp}
			for (const k of HABIT_FIELDS) if (k in c.body) (h as unknown as Record<string, unknown>)[k] = c.body[k]
			if (h.frequency === 'daily') h.weekdays = 127
			db.habits.push(h)
			return presentHabit(db, h, todayKey)
		}
	}
	const h = db.habits.find(x => x.id === num(a)) ?? notFound()
	if (b === undefined) {
		if (c.method === 'GET') return presentHabit(db, h, todayKey)
		if (c.method === 'PATCH') {
			for (const k of HABIT_FIELDS) if (k in c.body) (h as unknown as Record<string, unknown>)[k] = c.body[k]
			if (h.frequency === 'daily') h.weekdays = 127
			h.updated = now()
			return presentHabit(db, h, todayKey)
		}
		if (c.method === 'DELETE') {
			db.habits = db.habits.filter(x => x !== h)
			delete db.checkins[h.id]
			return {}
		}
	}
	if (b === 'checkins' && c.method === 'PUT' && date) {
		const days = db.checkins[h.id] ??= {}
		const value = Math.max(0, num(c.body.value) || 0)
		if (value) days[date] = value
		else delete days[date]
		return presentHabit(db, h, todayKey)
	}
	if (b === 'stats' && c.method === 'GET') return habitStats(db, h, todayKey, c.query.month as string | undefined)
	notFound()
}

// ---------- countdowns, focus ----------

function countdownsRoute(db: DB, c: Ctx): unknown {
	const [, a] = c.parts
	const fields = ['title', 'date', 'type', 'repeat_yearly', 'icon', 'hex_color'] as const
	if (a === undefined) {
		if (c.method === 'GET') return paginate(db.countdowns, c.query)
		if (c.method === 'POST') {
			const stamp = now()
			const cd: Countdown = {id: nextId(), title: '', date: stamp, type: 'countdown', repeat_yearly: false, icon: '', hex_color: '', created: stamp, updated: stamp}
			for (const k of fields) if (k in c.body) (cd as unknown as Record<string, unknown>)[k] = c.body[k]
			db.countdowns.push(cd)
			return cd
		}
	}
	const cd = db.countdowns.find(x => x.id === num(a)) ?? notFound()
	if (c.method === 'GET') return cd
	if (c.method === 'PATCH') {
		for (const k of fields) if (k in c.body) (cd as unknown as Record<string, unknown>)[k] = c.body[k]
		cd.updated = now()
		return cd
	}
	if (c.method === 'DELETE') {
		db.countdowns = db.countdowns.filter(x => x !== cd)
		return {}
	}
	notFound()
}

function focusStats(db: DB, days: number): FocusStats {
	const byDay = new Map<string, {pomos: number, seconds: number}>()
	for (const s of db.focus) {
		const key = dayKey(new Date(s.start_time))
		const e = byDay.get(key) ?? {pomos: 0, seconds: 0}
		if (s.type === 'pomo') e.pomos++
		e.seconds += s.duration
		byDay.set(key, e)
	}
	const history = []
	for (let i = days - 1; i >= 0; i--) {
		const d = new Date()
		d.setDate(d.getDate() - i)
		const e = byDay.get(dayKey(d))
		history.push({date: dayKey(d), pomos: e?.pomos ?? 0, focus_minutes: Math.round((e?.seconds ?? 0) / 60)})
	}
	const today = byDay.get(dayKey(new Date()))
	return {
		today_pomos: today?.pomos ?? 0,
		today_focus_minutes: Math.round((today?.seconds ?? 0) / 60),
		total_pomos: db.focus.filter(s => s.type === 'pomo').length,
		total_focus_minutes: Math.round(db.focus.reduce((s, x) => s + x.duration, 0) / 60),
		history,
	}
}

function focusRoute(db: DB, c: Ctx): unknown {
	const [, a] = c.parts
	const fields = ['task_id', 'type', 'start_time', 'end_time', 'duration', 'note'] as const
	if (a === 'stats') return focusStats(db, Math.min(366, Math.max(1, num(c.query.days) || 7)))
	if (a === undefined) {
		if (c.method === 'GET') return paginate([...db.focus].sort((x, y) => y.start_time.localeCompare(x.start_time)), c.query)
		if (c.method === 'POST') {
			const stamp = now()
			const s: FocusSession = {id: nextId(), task_id: 0, type: 'pomo', start_time: stamp, end_time: stamp, duration: 0, note: '', created: stamp, updated: stamp}
			for (const k of fields) if (k in c.body) (s as unknown as Record<string, unknown>)[k] = c.body[k]
			if (s.task_id) findTask(db, s.task_id)
			db.focus.push(s)
			return s
		}
	}
	const s = db.focus.find(x => x.id === num(a)) ?? notFound()
	if (c.method === 'PATCH') {
		for (const k of fields) if (k in c.body) (s as unknown as Record<string, unknown>)[k] = c.body[k]
		s.updated = now()
		return s
	}
	if (c.method === 'DELETE') {
		db.focus = db.focus.filter(x => x !== s)
		return {}
	}
	notFound()
}

// ---------- user ----------

function statistics(db: DB, days: number): UserStatistics {
	const done = live(db).filter(x => x.done && !x.wont_do && validDate(x.done_at))
	const count = (from: Date, to: Date) => done.filter(x => {
		const d = validDate(x.done_at)!
		return d >= from && d < to
	}).length
	const today = new Date()
	today.setHours(0, 0, 0, 0)
	const daily = []
	for (let i = days - 1; i >= 0; i--) {
		const from = new Date(today)
		from.setDate(from.getDate() - i)
		const to = new Date(from)
		to.setDate(to.getDate() + 1)
		daily.push({date: dayKey(from), count: count(from, to)})
	}
	const weekly = []
	const monday = new Date(today)
	monday.setDate(monday.getDate() - ((monday.getDay() + 6) % 7))
	for (let i = 11; i >= 0; i--) {
		const from = new Date(monday)
		from.setDate(from.getDate() - 7 * i)
		const to = new Date(from)
		to.setDate(to.getDate() + 7)
		weekly.push({date: dayKey(from), count: count(from, to)})
	}
	const monthly = []
	for (let i = 11; i >= 0; i--) {
		const from = new Date(today.getFullYear(), today.getMonth() - i, 1)
		const to = new Date(today.getFullYear(), today.getMonth() - i + 1, 1)
		monthly.push({date: dayKey(from), count: count(from, to)})
	}
	let onTime = 0
	let late = 0
	let noDue = 0
	for (const x of done) {
		const due = validDate(x.due_date)
		if (!due) noDue++
		else if (validDate(x.done_at)! <= new Date(due.getTime() + 86_399_000)) onTime++
		else late++
	}
	const overdue = live(db).filter(x => !x.done && validDate(x.due_date) && validDate(x.due_date)! < today).length
	const completionPoints = done.length * 10
	const onTimePoints = onTime * 5
	const latePenalty = late * 2
	const overduePenalty = overdue * 3
	const score = Math.max(0, completionPoints + onTimePoints - latePenalty - overduePenalty)
	const level = Math.floor(score / 100) + 1
	const projects = db.projects.map(p => ({project_id: p.id, title: p.title, hex_color: p.hex_color ?? '', count: done.filter(x => x.project_id === p.id).length})).filter(p => p.count)
	return {
		timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
		total_completed: done.length,
		completed_today: count(today, new Date(today.getTime() + 86_400_000)),
		open_overdue: overdue,
		daily,
		weekly,
		monthly,
		completion_rate: {on_time: onTime, late, no_due_date: noDue, rate: onTime + late ? onTime / (onTime + late) : 0},
		projects,
		score: {
			score,
			level,
			level_score: (level - 1) * 100,
			next_level_score: level * 100,
			completion_points: completionPoints,
			on_time_points: onTimePoints,
			late_penalty: latePenalty,
			overdue_penalty: overduePenalty,
		},
	}
}

function userRoute(db: DB, c: Ctx): unknown {
	const rest = c.parts.slice(1).join('/')
	if (rest === '' && c.method === 'GET') return db.user
	if (rest === 'logout') return {}
	if (rest === 'statistics') return statistics(db, Math.min(366, Math.max(1, num(c.query.days) || 30)))
	if (rest === 'settings/avatar/provider' && c.method === 'GET') return {avatar_provider: 'initials'}
	if (rest === 'settings/avatar/provider') return {}
	if (rest === 'settings/background' && c.method === 'GET') notFound()
	if (rest === 'settings/general' && (c.method === 'POST' || c.method === 'PUT' || c.method === 'PATCH')) {
		if (typeof c.body.name === 'string') db.user.name = c.body.name
		return {}
	}
	if (rest === 'token/refresh') return {access_token: 'local'}
	unavailable()
}

function info() {
	return {
		version: 'local',
		frontend_url: '',
		motd: '',
		registration_enabled: false,
		task_attachments_enabled: false,
		enabled_background_providers: null,
		trash_retention_days: null,
		auth: {local: {enabled: true, registration_enabled: false}, openid_connect: {enabled: false, providers: null}},
	}
}

// ---------- entry point ----------

function route(db: DB, c: Ctx): unknown {
	const [root] = c.parts
	switch (root) {
	case 'info': return info()
	case 'login':
	case 'register': return {token: 'local'}
	case 'tasks': return tasksRoute(db, c)
	case 'projects': return projectsRoute(db, c)
	case 'filters': return filtersRoute(db, c)
	case 'labels': return labelsRoute(db, c)
	case 'habits': return habitsRoute(db, c)
	case 'countdowns': return countdownsRoute(db, c)
	case 'focus-sessions': return focusRoute(db, c)
	case 'user': return userRoute(db, c)
	case 'notifications': return c.method === 'GET' ? paginate([], c.query) : {}
	case 'calendar-subscriptions':
	case 'tokens':
		// Nothing to list; creating one needs a server.
		if (c.method === 'GET' && c.parts.length === 1) return paginate([], c.query)
		break
	case 'avatar':
		notFound()
	}
	unavailable()
}

/**
 * Answers a request from `db`. Synchronous: the offline mode
 * (src/api/offline) relies on nothing else touching `db` meanwhile.
 */
export function answer<T>(db: DB, method: string, path: string, opts: RequestOptions = {}): T {
	const url = new URL(path, 'http://local')
	const parts = url.pathname.split('/').filter(Boolean).map(decodeURIComponent)
	// JSON round trips also unwrap the reactive proxies callers may pass.
	const body = opts.body instanceof FormData || opts.body === undefined ? {} : JSON.parse(JSON.stringify(opts.body)) as Body
	try {
		const result = withDB(db, () => route(db, {method: method.toUpperCase(), parts, query: opts.query ?? {}, body}))
		// Callers mutate what they get back; never hand out the stored objects.
		return (result === undefined ? undefined : JSON.parse(JSON.stringify(result))) as T
	} finally {
		index = null
	}
}

export async function localApi<T>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
	const db = getDB()
	try {
		return answer<T>(db, method, path, opts)
	} finally {
		if (method.toUpperCase() !== 'GET') save()
	}
}
