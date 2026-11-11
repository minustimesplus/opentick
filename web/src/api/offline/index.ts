// Offline mode of the desktop app.
//
// While the server answers, every request goes to it exactly as before, and
// what comes back is copied into a replica of the account: the same database
// the in-browser backend (../local) runs on, filled with the server's data.
// A background pull refreshes the replica every few minutes, so it also picks
// up changes made on other devices.
//
// When the server can't be reached (the desktop proxy marks those failures,
// see web-desktop/main.js), requests are answered from the replica by the
// in-browser backend, and changes are queued in an outbox. Objects created
// offline get ids from TEMP_BASE up, a range server ids never reach. Once the
// server answers again, the outbox is replayed through the regular API in
// order, with offline ids swapped for the ids the server gave those objects.
// Then the app reloads from the server.
//
// The server stays the only source of truth, so the phone app (and any other
// client) sees offline changes as soon as they are replayed. Like the Android
// app, conflicting edits resolve per field: the last write to reach the
// server wins.
//
// Replica, outbox and a cache of other GET responses live in files in the
// desktop app's user data folder (window.ttDesktop.offlineStore). Without that
// bridge (the web app, older desktop builds) this module stays out of the way.

import type {Paginated, RequestOptions} from '../client'
import {ApiError, getToken, NetworkError, onUnauthorized, serverApi} from '../client'
import type {DB, StoredHabit, StoredTask} from '../local/db'
import type {Countdown, CurrentUser, FocusSession, Habit, HabitStats, Label, Project, Task, TaskComment, User} from '../types'
import {applyMergePatch} from '@/lib/viewSync'
import {t} from '@/lib/i18n'

/** Ids of objects created offline start here; safe integers far above any server id. */
export const TEMP_BASE = 2 ** 50
const PROBE_INTERVAL = 15_000
/** How often the replica is refreshed while online. */
const PULL_INTERVAL = 2 * 60_000
/** How often a pull also refreshes per-list data (manual order, kanban columns, habit history). */
const FULL_PULL_INTERVAL = 15 * 60_000
/** Pages of done tasks kept offline (the server caps a page at 50 by default). */
const DONE_PAGES = 20
const CACHE_LIMIT = 200
const CACHE_BLOB_LIMIT = 1_000_000
/** Replays of one change that fail on the server's side before it is given up. */
const MAX_ATTEMPTS = 5
const STEP = 65536

type Query = NonNullable<RequestOptions['query']>
type Json = Record<string, unknown>

interface Op {
	method: string
	path: string
	query?: Query
	body?: unknown
	contentType?: string
	/** What the replica answered, to learn the server ids of objects it created. */
	result?: unknown
	attempts?: number
}

interface ReplicaFile {
	version: 1
	server: string
	userId: number
	pulledAt: number
	fullAt: number
	db: DB
}

interface OutboxFile {
	ops: Op[]
	/** Offline id → server id, for replayed creates. */
	ids: [number, number][]
}

interface CacheEntry {
	at: number
	value?: unknown
	blob?: {type: string, data: string}
}

type Backend = typeof import('../local/backend')

const store = typeof window === 'undefined' ? undefined : window.ttDesktop?.offlineStore

let backend: Backend | null = null
let replica: ReplicaFile | null = null
let outbox: Op[] = []
const idMap = new Map<number, number>()
/**
 * Offline ids replayed in this session. Until the app reloads, it may still
 * use them (an open task, say); requests swap them for the server ids.
 */
const replayedIds = new Map<number, number>()
let cache = new Map<string, CacheEntry>()
let offline = false
/** The session ended unexpectedly: check the next one is the same account before using offline data. */
let needsVerify = false
/** Bumped by every change mirrored into the replica, so a pull can tell it raced one. */
let epoch = 0
let pulling: Promise<void> | null = null
let reconnecting: Promise<void> | null = null
let probeTimer: number | undefined
let loading: Promise<void> | null = null
const reconnectListeners: ((ids: Map<number, number>) => void)[] = []

/** Whether requests go through the offline mode: desktop app, signed in to a server. */
export function offlineEnabled(): boolean {
	return !!store && !!getToken()
}

/** Whether the server is currently unreachable and the replica answers instead. */
export function isOffline(): boolean {
	return offline
}

/**
 * Called after changes made offline were sent to the server, with the server
 * ids of objects that were created offline.
 */
export function onReconnect(fn: (ids: Map<number, number>) => void) {
	reconnectListeners.push(fn)
}

/**
 * Swaps offline ids for server ids anywhere in a JSON value (numbers, and ids
 * inside object keys such as "p<id>"); unknown ones stay.
 */
export function remapIds<T>(value: T, ids: Map<number, number>): T {
	return remap(value, n => ids.get(n) ?? n) as T
}

/** Swaps offline ids inside a string, such as a route path. */
export function remapText(text: string, ids: Map<number, number>): string {
	return text.replace(/\d{16,}/g, m => String(ids.get(Number(m)) ?? m))
}

// ---------- persistence ----------

const timers = new Map<string, number>()

function serialize(name: string): string {
	if (name === 'replica') return JSON.stringify(replica)
	if (name === 'outbox') return JSON.stringify({ops: outbox, ids: [...idMap]} satisfies OutboxFile)
	return JSON.stringify([...cache])
}

function persist(name: 'replica' | 'outbox' | 'cache', delay = 0) {
	if (!store) return
	if (timers.has(name)) {
		if (delay) return
		clearTimeout(timers.get(name))
	}
	timers.set(name, window.setTimeout(() => {
		timers.delete(name)
		store.write(name, serialize(name)).catch(() => {})
	}, delay))
}

function persistPendingNow() {
	if (!store) return
	for (const [name, timer] of timers) {
		clearTimeout(timer)
		store.writeSync(name, serialize(name))
	}
	timers.clear()
}

function parse<T>(text: string | null): T | null {
	if (!text) return null
	try {
		return JSON.parse(text) as T
	} catch {
		return null
	}
}

async function load() {
	if (!store) return
	backend = await import('../local/backend')
	onUnauthorized(() => {
		needsVerify = true
	})
	const [r, o, c, server] = await Promise.all([
		store.read('replica'),
		store.read('outbox'),
		store.read('cache'),
		window.ttDesktop!.getServerUrl().catch(() => ''),
	])
	const file = parse<ReplicaFile>(r)
	const box = parse<OutboxFile>(o)
	if (file && file.version === 1 && file.server === server) {
		replica = file
		outbox = box?.ops ?? []
		for (const [a, b] of box?.ids ?? []) idMap.set(a, b)
		cache = new Map(parse<[string, CacheEntry][]>(c) ?? [])
	} else if (file || box?.ops.length) {
		// Another server than the one signed in to now.
		await store.clear().catch(() => {})
	}
	// Changes left from last time go out before anything else is sent.
	if (outbox.length) {
		offline = true
		scheduleProbe(0)
	}
	window.addEventListener('beforeunload', persistPendingNow)
	window.addEventListener('online', () => {
		if (offline) scheduleProbe(0)
	})
	window.addEventListener('focus', () => {
		if (offline) scheduleProbe(0)
	})
}

function ready(): Promise<void> {
	loading ??= load()
	return loading
}

/** Forgets everything kept for offline use (on sign out). */
export async function clearOfflineData() {
	if (!store) return
	await ready()
	wipe()
	await store.clear().catch(() => {})
}

function wipe() {
	for (const timer of timers.values()) clearTimeout(timer)
	timers.clear()
	replica = null
	outbox = []
	idMap.clear()
	replayedIds.clear()
	cache = new Map()
	offline = false
	needsVerify = false
	epoch++
	clearTimeout(probeTimer)
}

// ---------- requests ----------

function pathOf(path: string): string {
	return path.split('?')[0].replace(/\/+$/, '') || '/'
}

const MODELED = [
	/^\/tasks(\/trash(\/empty|\/\d+(\/restore)?)?)?$/,
	/^\/tasks\/\d+(\/(duplicate|position|labels(\/\d+)?|assignees(\/\d+)?|relations(\/[a-z]+\/\d+)?|comments(\/\d+)?))?$/,
	/^\/projects(\/-?\d+(\/(tasks|views\/\d+(\/(tasks|buckets(\/tasks|\/\d+(\/tasks)?)?))?))?)?$/,
	/^\/filters(\/\d+)?$/,
	/^\/labels(\/\d+)?$/,
	/^\/habits(\/\d+(\/(checkins\/[\d-]+|stats))?)?$/,
	/^\/countdowns(\/\d+)?$/,
	/^\/focus-sessions(\/(stats|\d+))?$/,
	/^\/user(\/(statistics|settings\/general))?$/,
]

/** Requests the replica can answer. Everything else needs the server, or the cache when reading. */
function modeled(path: string): boolean {
	return MODELED.some(re => re.test(path))
}

/** Requests that create an object; the server picks its id. */
function creates(method: string, seg: string[]): boolean {
	if (method !== 'POST') return false
	const [root, , sub] = seg
	if (seg.length === 1) return ['projects', 'labels', 'filters', 'habits', 'countdowns', 'focus-sessions'].includes(root)
	if (root === 'projects') return (seg.length === 3 && sub === 'tasks') || (seg.length === 5 && seg[4] === 'buckets')
	return root === 'tasks' && seg.length === 3 && (sub === 'duplicate' || sub === 'comments')
}

function cacheKey(path: string, query?: Query): string {
	const params = new URLSearchParams()
	for (const [k, v] of Object.entries(query ?? {}).sort(([a], [b]) => a.localeCompare(b))) {
		if (v === undefined || v === null) continue
		if (Array.isArray(v)) v.forEach(x => params.append(k, String(x)))
		else params.append(k, String(v))
	}
	return path + '?' + params.toString()
}

function clone<T>(v: T): T {
	return v === undefined ? v : JSON.parse(JSON.stringify(v)) as T
}

export async function offlineApi<T>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
	await ready()
	const m = method.toUpperCase()
	if (needsVerify && opts.auth !== false) await verify()
	if (!offline || opts.auth === false && m !== 'GET') {
		if (replayedIds.size) {
			const fn = (n: number) => n < 0 ? -(replayedIds.get(-n - 1) ?? -n - 1) - 1 : replayedIds.get(n) ?? n
			path = remapPath(path, fn)
			opts = {...opts, query: remap(opts.query, fn) as Query | undefined, body: opts.body instanceof FormData ? opts.body : remap(opts.body, fn)}
		}
		try {
			const res = await serverApi<T>(m, path, opts)
			observe(m, path, opts, res)
			return res
		} catch (e) {
			if (!(e instanceof NetworkError) || !replica || opts.auth === false && m !== 'GET') throw e
			goOffline()
			// It may have reached the server: applying it again offline could duplicate it.
			if (m !== 'GET' && e.delivered) throw e
		}
	}
	return answerOffline<T>(m, path, opts)
}

function unavailable(): ApiError {
	return new ApiError(503, t('offlineUnavailable'))
}

async function answerOffline<T>(m: string, path: string, opts: RequestOptions): Promise<T> {
	const db = replica?.db
	if (!db || !backend) throw new NetworkError(t('authUnavailable'), false)
	const p = pathOf(path)
	if (m === 'GET') {
		if (!modeled(p)) {
			const hit = cache.get(cacheKey(p, opts.query))
			if (hit?.blob) {
				const bin = atob(hit.blob.data)
				const bytes = new Uint8Array(bin.length)
				for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
				return new Blob([bytes], {type: hit.blob.type}) as T
			}
			if (hit) return clone(hit.value) as T
			if (p === '/info') throw new NetworkError(t('authUnavailable'), false)
		}
		try {
			return backend.answer<T>(db, m, path, opts)
		} catch (e) {
			throw e instanceof ApiError && e.status === 501 ? unavailable() : e
		}
	}
	if (opts.auth === false || opts.body instanceof FormData || !modeled(p)) throw unavailable()
	let result: unknown
	try {
		result = applyLocal(db, m, p, opts)
	} catch (e) {
		throw e instanceof ApiError && e.status === 501 ? unavailable() : e
	}
	outbox.push({method: m, path, query: clone(opts.query), body: clone(opts.body), contentType: opts.contentType, result: clone(result)})
	persist('outbox')
	persist('replica', 300)
	return clone(result) as T
}

/** Applies a change to the replica the way the server would. */
function applyLocal(db: DB, m: string, p: string, opts: RequestOptions): unknown {
	const seg = p.split('/').filter(Boolean)
	const body = (opts.body ?? {}) as Json
	if (p === '/user/settings/general') {
		patchSettings(db, body)
		return {}
	}
	// The in-browser backend only knows its single local user.
	if (seg[0] === 'tasks' && seg[2] === 'assignees' && seg.length === 3 && m === 'POST') {
		assignLocal(db, Number(seg[1]), Number(body.user_id))
		return {}
	}
	return backend!.answer(db, m, p, opts)
}

function patchSettings(db: DB, body: Json) {
	if (typeof body.name === 'string') db.user.name = body.name
	db.user.settings = applyMergePatch(db.user.settings, body) as DB['user']['settings']
}

function assignLocal(db: DB, taskId: number, userId: number) {
	const task = db.tasks.find(x => x.id === taskId && !x.deleted_at)
	if (!task) throw new ApiError(404, 'Not found')
	if (task.assignees?.some(u => u.id === userId)) return
	task.assignees = [...(task.assignees ?? []), findUser(db, userId)]
}

/** A user the app has seen, from the members lists it loaded. */
function findUser(db: DB, id: number): User {
	if (db.user.id === id) return {id, username: db.user.username, name: db.user.name}
	for (const [key, entry] of cache) {
		if (!/^\/projects\/\d+\/users/.test(key)) continue
		const items = (entry.value as Paginated<User> | undefined)?.items ?? []
		const u = items.find(x => x.id === id)
		if (u) return u
	}
	return {id, username: ''}
}

// ---------- mirroring online responses ----------

function observe(m: string, path: string, opts: RequestOptions, res: unknown) {
	const p = pathOf(path)
	if (m === 'GET') {
		if (p === '/user') {
			const user = res as CurrentUser
			if (replica && replica.userId !== user.id) {
				// Signed in to another account: nothing kept so far belongs to it.
				wipe()
				store?.clear().catch(() => {})
			}
			if (replica) {
				replica.db.user = {...user, settings: {...user.settings, default_project_id: user.settings?.default_project_id ?? 0}}
				persist('replica', 1000)
			}
			schedulePull()
		}
		if (!modeled(p)) cachePut(cacheKey(p, opts.query), res)
		else if (replica) {
			try {
				mirrorRead(replica.db, p, opts, res)
			} catch (e) {
				console.warn('offline: could not copy', p, e)
			}
		}
		return
	}
	epoch++
	if (!replica) return
	try {
		mirrorWrite(replica.db, m, p, opts, res)
	} catch {
		// The replica disagrees (e.g. the object isn't in it yet); the next pull fixes it.
	}
	persist('replica', 1000)
}

/** Images worth keeping offline; attachments can be large and stay online only. */
const CACHED_BLOBS = /^\/(avatar\/|user\/settings\/background)/

function cachePut(key: string, value: unknown) {
	if (value instanceof Blob) {
		if (value.size > CACHE_BLOB_LIMIT || !CACHED_BLOBS.test(key)) return
		const type = value.type || 'application/octet-stream'
		value.arrayBuffer().then(buf => {
			let bin = ''
			const bytes = new Uint8Array(buf)
			for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
			setCache(key, {at: Date.now(), blob: {type, data: btoa(bin)}})
		}).catch(() => {})
		return
	}
	setCache(key, {at: Date.now(), value: clone(value)})
}

function setCache(key: string, entry: CacheEntry) {
	cache.delete(key)
	cache.set(key, entry)
	while (cache.size > CACHE_LIMIT) cache.delete(cache.keys().next().value!)
	persist('cache', 2000)
}

function mirrorRead(db: DB, p: string, opts: RequestOptions, res: unknown) {
	const seg = p.split('/').filter(Boolean)
	const page = res as Paginated<unknown> | undefined
	const items = page?.items ?? []
	if (p === '/tasks' || (seg[0] === 'projects' && seg[2] === 'tasks')) {
		upsertTasks(db, items as Task[])
	} else if (seg[0] === 'projects' && seg[2] === 'views' && seg[4] === 'tasks' && Number(seg[1]) > 0) {
		const view = Number(seg[3])
		upsertTasks(db, items as Task[])
		const positions = db.positions[view] ??= {}
		for (const task of items as Task[]) if (task.position) positions[task.id] = task.position
	} else if (seg[0] === 'projects' && seg[4] === 'buckets' && seg[5] === 'tasks') {
		mirrorBuckets(db, Number(seg[3]), items as KanbanBucket[], Number(opts.query?.page ?? 1) <= 1)
	} else if (p === '/projects') {
		for (const project of items as Project[]) upsertProject(db, project)
	} else if (p === '/labels') {
		for (const label of items as Label[]) upsertById(db.labels, label)
	} else if (seg[0] === 'tasks' && seg[2] === 'comments' && seg.length === 3) {
		const taskId = Number(seg[1])
		for (const c of items as TaskComment[]) upsertComment(db, taskId, c)
		const complete = (page?.page ?? 1) === 1 && (page?.total_pages ?? 1) <= 1
		if (complete) {
			const ids = new Set((items as TaskComment[]).map(c => c.id))
			db.comments = db.comments.filter(c => c.task_id !== taskId || ids.has(c.id))
			const task = db.tasks.find(x => x.id === taskId)
			// All comments are stored now; count them instead of keeping the server's count.
			if (task) delete task.comment_count
		}
	}
}

function mirrorWrite(db: DB, m: string, p: string, opts: RequestOptions, res: unknown) {
	const seg = p.split('/').filter(Boolean)
	const r = (res ?? {}) as Json
	const body = (opts.body ?? {}) as Json
	const [root] = seg
	if (root === 'projects' && seg[2] === 'tasks' && seg.length === 3 && m === 'POST') return upsertTasks(db, [r as unknown as Task])
	if (root === 'tasks' && seg.length === 2 && m !== 'DELETE') return upsertTasks(db, [r as unknown as Task], false)
	if (root === 'tasks' && seg[2] === 'duplicate') return upsertTasks(db, [((r.duplicated_task ?? r) as Task)])
	if (root === 'tasks' && seg[2] === 'comments' && m !== 'DELETE') return upsertComment(db, Number(seg[1]), r as unknown as TaskComment)
	if (root === 'tasks' && seg[2] === 'assignees' && seg.length === 3 && m === 'POST') return assignLocal(db, Number(seg[1]), Number(body.user_id))
	if (root === 'projects' && seg.length <= 2 && m !== 'DELETE') {
		upsertProject(db, r as unknown as Project)
		// Kanban columns of a new list come with the next full pull.
		if (seg.length === 1) schedulePull(true)
		return
	}
	if (root === 'projects' && seg[4] === 'buckets' && seg.length <= 6 && m !== 'DELETE') return upsertBucket(db, Number(seg[3]), r as unknown as KanbanBucket)
	if (root === 'labels' && m !== 'DELETE') return upsertById(db.labels, r as unknown as Label)
	if (root === 'filters' && m !== 'DELETE') {
		upsertFilter(db, Number(seg[1] ?? r.id), body)
		schedulePull(true)
		return
	}
	if (root === 'habits' && seg.length <= 2 && m !== 'DELETE') return upsertHabit(db, r as unknown as Habit)
	if (root === 'countdowns' && m !== 'DELETE') return upsertById(db.countdowns, r as unknown as Countdown)
	if (root === 'focus-sessions' && m !== 'DELETE') return upsertById(db.focus, r as unknown as FocusSession)
	if (p === '/user/settings/general') return patchSettings(db, body)
	// Never let the replica invent ids for objects the server created.
	if (creates(m, seg) || !modeled(p)) return
	backend!.answer(db, m, p, opts)
}

function upsertById<T extends {id: number}>(list: T[], item: T) {
	if (!item || typeof item.id !== 'number') return
	const i = list.findIndex(x => x.id === item.id)
	if (i >= 0) list[i] = {...list[i], ...item}
	else list.push(item)
}

/** Converts server tasks for the replica. `full` responses carry their relations. */
function upsertTasks(db: DB, tasks: Task[], full = true) {
	if (!tasks.length) return
	const byId = new Map(db.tasks.map((x, i) => [x.id, i]))
	const labels = new Set(db.labels.map(l => l.id))
	for (const t of tasks) {
		if (!t || typeof t.id !== 'number' || typeof t.project_id !== 'number') continue
		const i = byId.get(t.id)
		const prev = i === undefined ? undefined : db.tasks[i]
		const stored = toStored(t, prev)
		for (const l of t.labels ?? []) {
			if (!labels.has(l.id)) {
				labels.add(l.id)
				db.labels.push(l)
			}
		}
		if (i === undefined) {
			byId.set(t.id, db.tasks.length)
			db.tasks.push(stored)
		} else db.tasks[i] = stored
		if (full && t.related_tasks) setRelations(db, t)
	}
}

function toStored(t: Task, prev?: StoredTask): StoredTask {
	const {labels, related_tasks: _r, position: _p, ...rest} = t
	return {
		...(prev ?? {}),
		...rest,
		label_ids: labels ? labels.map(l => l.id) : prev?.label_ids ?? [],
		deleted_at: t.deleted_at ?? null,
	} as StoredTask
}

function setRelations(db: DB, t: Task) {
	const subs = (t.related_tasks?.subtask ?? []).map(s => s.id)
	const parents = (t.related_tasks?.parenttask ?? []).map(s => s.id)
	db.relations = db.relations.filter(r => r.task_id !== t.id && r.other_task_id !== t.id)
	for (const id of subs) db.relations.push({task_id: t.id, other_task_id: id, kind: 'subtask'})
	for (const id of parents) db.relations.push({task_id: id, other_task_id: t.id, kind: 'subtask'})
}

function upsertComment(db: DB, taskId: number, c: TaskComment) {
	if (!c || typeof c.id !== 'number') return
	const i = db.comments.findIndex(x => x.id === c.id)
	if (i >= 0) db.comments[i] = {...db.comments[i], ...c, task_id: taskId}
	else {
		db.comments.push({...c, task_id: taskId})
		const task = db.tasks.find(x => x.id === taskId)
		if (task?.comment_count !== undefined) task.comment_count++
	}
}

interface KanbanView {
	id: number
	view_kind: string | number
	default_bucket_id?: number
	done_bucket_id?: number
}

interface KanbanBucket {
	id: number
	title: string
	position?: number
	count?: number
	tasks?: Task[] | null
}

function upsertProject(db: DB, p: Project) {
	if (!p || typeof p.id !== 'number') return
	if (p.id < -1) {
		const f = db.filters.find(x => x.id === -p.id - 1)
		if (f) f.project = p
		return
	}
	upsertById(db.projects, p)
	for (const v of (p.views ?? []) as KanbanView[]) {
		if (v.view_kind !== 'kanban') continue
		const prev = db.viewBuckets[v.id]
		db.viewBuckets[v.id] = {
			default_bucket_id: v.default_bucket_id || prev?.default_bucket_id || 0,
			done_bucket_id: v.done_bucket_id ?? prev?.done_bucket_id ?? 0,
		}
	}
}

function upsertBucket(db: DB, viewId: number, b: KanbanBucket) {
	if (!b || typeof b.id !== 'number') return
	const stored = {id: b.id, project_view_id: viewId, title: b.title, position: b.position ?? STEP}
	upsertById(db.buckets, stored)
}

function mirrorBuckets(db: DB, viewId: number, buckets: KanbanBucket[], firstPage: boolean) {
	if (firstPage) {
		const ids = new Set(buckets.map(b => b.id))
		db.buckets = db.buckets.filter(b => b.project_view_id !== viewId || ids.has(b.id))
	}
	const placed = db.bucketOf[viewId] ??= {}
	const positions = db.positions[viewId] ??= {}
	for (const b of buckets) {
		upsertBucket(db, viewId, b)
		upsertTasks(db, b.tasks ?? [])
		for (const task of b.tasks ?? []) {
			placed[task.id] = b.id
			if (task.position) positions[task.id] = task.position
		}
	}
}

function upsertFilter(db: DB, id: number, body: Json) {
	if (!id) return
	const def = body as {title?: string, filters?: {filter?: string}}
	const now = new Date().toISOString()
	const f = db.filters.find(x => x.id === id)
	if (f) {
		if (def.title !== undefined) f.title = def.title
		if (def.filters?.filter !== undefined) f.filter = def.filters.filter
		f.updated = now
	} else db.filters.push({id, title: def.title ?? '', filter: def.filters?.filter ?? '', created: now, updated: now})
}

function toStoredHabit(h: Habit): StoredHabit {
	const {today_value: _t, current_streak: _c, recent_checkins: _r, ...rest} = h
	return rest
}

function upsertHabit(db: DB, h: Habit) {
	if (!h || typeof h.id !== 'number') return
	upsertById(db.habits, toStoredHabit(h))
	mergeRecentCheckins(db, h)
}

function dayKey(d: Date): string {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

/** A habit lists its check-ins of the last 7 days; days it leaves out have none. */
function mergeRecentCheckins(db: DB, h: Habit) {
	if (!h.recent_checkins) return
	const days = db.checkins[h.id] ??= {}
	const values = new Map(h.recent_checkins.map(c => [c.date, c.value]))
	const d = new Date()
	for (let i = 0; i < 7; i++) {
		const key = dayKey(d)
		const value = values.get(key) ?? (i === 0 ? h.today_value : 0)
		if (value) days[key] = value
		else delete days[key]
		d.setDate(d.getDate() - 1)
	}
}

// ---------- pulling the replica ----------

async function fetchAllServer<T>(path: string, query: Query = {}, maxPages = 50): Promise<T[]> {
	const out: T[] = []
	for (let page = 1; page <= maxPages; page++) {
		const res = await serverApi<Paginated<T>>('GET', path, {query: {...query, page}})
		out.push(...(res?.items ?? []))
		if (page >= (res?.total_pages ?? 1) || !res?.items?.length) break
	}
	return out
}

/** Runs fn over items, a few at a time. */
async function each<T>(items: T[], fn: (item: T) => Promise<void>, parallel = 4) {
	let next = 0
	await Promise.all(Array.from({length: Math.min(parallel, items.length)}, async () => {
		while (next < items.length) await fn(items[next++])
	}))
}

let lastPull = 0

/** Refreshes the replica soon; `force` skips the interval, e.g. after reconnecting. */
function schedulePull(force = false) {
	if (!store || offline || outbox.length || !getToken()) return
	if (!force && Date.now() - lastPull < PULL_INTERVAL) return
	if (pulling) return
	lastPull = Date.now()
	const full = force || !replica || Date.now() - replica.fullAt > FULL_PULL_INTERVAL
	// Let the app's own requests go first.
	pulling = new Promise<void>(resolve => setTimeout(resolve, 1500))
		.then(() => pull(full))
		.catch(e => {
			if (!(e instanceof NetworkError)) console.warn('offline: pull failed', e)
		})
		.finally(() => {
			pulling = null
		})
}

async function pull(full: boolean) {
	for (let attempt = 0; ; attempt++) {
		const started = epoch
		const prev = replica
		const next = await download(full, prev)
		// Offline (or signed out) meanwhile: the replica may hold changes the download doesn't.
		if (offline || outbox.length || !getToken() || replica !== prev) return
		// An edit went through while downloading and may be missing from it.
		if (started !== epoch && attempt < 2) continue
		replica = next
		persist('replica')
		return
	}
}

async function download(full: boolean, prev: ReplicaFile | null): Promise<ReplicaFile> {
	const old = prev?.db
	const today = dayKey(new Date())
	const [server, user, info, active, all, labels, open, done, trash, habits, countdowns, focus] = await Promise.all([
		window.ttDesktop!.getServerUrl(),
		serverApi<CurrentUser>('GET', '/user'),
		serverApi('GET', '/info', {auth: false}).catch(() => null),
		fetchAllServer<Project>('/projects', {is_archived: false}),
		fetchAllServer<Project>('/projects', {is_archived: true}),
		fetchAllServer<Label>('/labels'),
		fetchAllServer<Task>('/tasks', {filter: 'done = false', sort_by: ['due_date', 'id'], order_by: ['asc', 'asc'], expand: 'comment_count'}),
		fetchAllServer<Task>('/tasks', {filter: 'done = true', sort_by: ['done_at'], order_by: ['desc'], expand: 'comment_count'}, DONE_PAGES),
		fetchAllServer<Task>('/tasks/trash', {format: 'markdown'}, 10),
		fetchAllServer<Habit>('/habits', {today}).catch(() => null),
		fetchAllServer<Countdown>('/countdowns').catch(() => null),
		fetchAllServer<FocusSession>('/focus-sessions', {}, 20).catch(() => null),
	])
	if (info) cachePut(cacheKey('/info'), info)

	const db: DB = {
		version: 1,
		nextId: Math.max(old?.nextId ?? 0, TEMP_BASE),
		user: {...user, settings: {...user.settings, default_project_id: user.settings?.default_project_id ?? 0}},
		projects: [],
		tasks: [],
		labels: [],
		relations: [],
		comments: [],
		positions: full ? {} : clone(old?.positions ?? {}),
		bucketOf: full ? {} : clone(old?.bucketOf ?? {}),
		buckets: full ? [] : clone(old?.buckets ?? []),
		viewBuckets: {},
		filters: [],
		habits: [],
		checkins: clone(old?.checkins ?? {}),
		countdowns: countdowns ?? clone(old?.countdowns ?? []),
		focus: focus ?? clone(old?.focus ?? []),
	}

	const projects = new Map<number, Project>()
	for (const p of [...all, ...active]) projects.set(p.id, p)
	const filterProjects = [...projects.values()].filter(p => p.id < -1)
	for (const p of projects.values()) if (p.id >= -1) upsertProject(db, p)
	await each(filterProjects, async p => {
		const id = -p.id - 1
		try {
			const f = await serverApi<{id: number, title: string, filters?: {filter?: string}, created?: string, updated?: string}>('GET', `/filters/${id}`)
			const stamp = new Date().toISOString()
			db.filters.push({id, title: f.title ?? p.title, filter: f.filters?.filter ?? '', created: f.created ?? stamp, updated: f.updated ?? stamp, project: p})
		} catch (e) {
			if (e instanceof NetworkError) throw e
		}
	})

	db.labels = labels
	const tasks = new Map<number, Task>()
	for (const x of [...done, ...open]) tasks.set(x.id, x)
	upsertTasks(db, [...tasks.values()])
	const stamp = new Date().toISOString()
	upsertTasks(db, trash.filter(x => !tasks.has(x.id)).map(x => ({...x, deleted_at: x.deleted_at || stamp})), false)

	// Comments of tasks that still exist, as loaded when they were opened.
	// Tasks keep the server's comment count, which covers the ones never loaded.
	const ids = new Set(db.tasks.map(x => x.id))
	db.comments = (old?.comments ?? []).filter(c => ids.has(c.task_id))

	if (habits) {
		db.habits = habits.map(toStoredHabit)
		const live = new Set(habits.map(h => h.id))
		for (const id of Object.keys(db.checkins)) if (!live.has(Number(id))) delete db.checkins[Number(id)]
		for (const h of habits) mergeRecentCheckins(db, h)
	} else db.habits = clone(old?.habits ?? [])

	if (full) {
		await each(db.projects.filter(p => p.id > 0 && !p.is_archived), async p => {
			for (const v of (p.views ?? []) as KanbanView[]) {
				try {
					if (v.view_kind === 'list') {
						const items = await fetchAllServer<Task>(`/projects/${p.id}/views/${v.id}/tasks`, {filter: 'done = false'}, 10)
						db.positions[v.id] = Object.fromEntries(items.filter(x => x.position).map(x => [x.id, x.position!]))
					} else if (v.view_kind === 'kanban') {
						for (let page = 1; page <= 20; page++) {
							const res = await serverApi<{items?: KanbanBucket[] | null}>('GET', `/projects/${p.id}/views/${v.id}/buckets/tasks`, {query: {page}})
							const items = res?.items ?? []
							mirrorBuckets(db, v.id, items, page === 1)
							if (!items.some(b => (b.tasks?.length ?? 0) > 0 && (b.tasks?.length ?? 0) < (b.count ?? 0))) break
						}
					}
				} catch (e) {
					if (e instanceof NetworkError) throw e
				}
			}
		})
		if (habits) {
			const months = [0, 1, 2].map(i => {
				const d = new Date()
				d.setDate(1)
				d.setMonth(d.getMonth() - i)
				return dayKey(d).slice(0, 7)
			})
			await each(habits.flatMap(h => months.map(month => ({h, month}))), async ({h, month}) => {
				try {
					const stats = await serverApi<HabitStats>('GET', `/habits/${h.id}/stats`, {query: {month, today}})
					const days = db.checkins[h.id] ??= {}
					for (const d of stats.days ?? []) {
						if (d.value) days[d.date] = d.value
						else delete days[d.date]
					}
				} catch (e) {
					if (e instanceof NetworkError) throw e
				}
			})
		}
	}

	const now = Date.now()
	return {version: 1, server, userId: user.id, pulledAt: now, fullAt: full ? now : prev?.fullAt ?? 0, db}
}

/** Refreshes the offline copy right away (normally it happens every few minutes). */
export async function refreshOfflineCopy() {
	await ready()
	await pulling
	if (!store || offline || outbox.length || !getToken()) return
	lastPull = Date.now()
	await pull(true)
}

/** Checks whether the server is back, and if so sends the changes made offline. True when online. */
export async function checkConnection(): Promise<boolean> {
	await ready()
	if (offline) await probe()
	return !offline
}

// ---------- going offline and back ----------

function goOffline() {
	if (offline) return
	offline = true
	scheduleProbe(PROBE_INTERVAL)
}

function scheduleProbe(delay: number) {
	clearTimeout(probeTimer)
	probeTimer = window.setTimeout(probe, delay)
}

async function probe() {
	if (!offline) return
	if (getToken() && !reconnecting) {
		try {
			await serverApi('GET', '/info', {auth: false})
			await reconnect()
		} catch {
			// Still offline.
		}
	}
	if (offline) scheduleProbe(PROBE_INTERVAL)
}

/** Before using offline data with a new session, make sure it belongs to this account. */
async function verify() {
	try {
		const [user, server] = await Promise.all([serverApi<CurrentUser>('GET', '/user'), window.ttDesktop!.getServerUrl()])
		needsVerify = false
		if (replica && (replica.userId !== user.id || replica.server !== server)) {
			if (outbox.length) {
				const {toast} = await import('@/lib/toast')
				toast(t('offlineDiscarded'))
			}
			wipe()
			await store?.clear().catch(() => {})
		}
	} catch {
		// Can't tell yet; ask again with the next request.
	}
}

function reconnect(): Promise<void> {
	reconnecting ??= (async () => {
		try {
			if (outbox.length && !await flush()) return
			// Nothing queued anymore: from here on requests go to the server.
			offline = false
			clearTimeout(probeTimer)
			const ids = new Map(idMap)
			for (const [a, b] of idMap) replayedIds.set(a, b)
			idMap.clear()
			persist('outbox')
			reconnectListeners.forEach(fn => fn(ids))
			schedulePull(true)
		} finally {
			reconnecting = null
		}
	})()
	return reconnecting
}

/** Status codes that mean the server will never take this change. */
function rejected(status: number) {
	return status >= 400 && status < 500 && ![401, 408, 429].includes(status)
}

/** Replays the outbox. False when it has to wait for a later try. */
async function flush(): Promise<boolean> {
	const user = await serverApi<CurrentUser>('GET', '/user')
	if (replica && replica.userId !== user.id) {
		const {toast} = await import('@/lib/toast')
		toast(t('offlineDiscarded'))
		outbox = []
		persist('outbox')
		return true
	}
	while (outbox.length) {
		const op = outbox[0]
		await mapBucketsFor(op)
		const missing = {count: 0}
		const map = (n: number) => {
			const out = mapNumber(n)
			if (out === null) {
				missing.count++
				return n
			}
			return out
		}
		const path = remapPath(op.path, map)
		const query = remap(op.query, map) as Query | undefined
		const body = remap(op.body, map)
		if (missing.count) {
			// Refers to something created offline that the server refused.
			console.warn('offline: dropping change to an object that was never created', op.method, op.path)
			outbox.shift()
			persist('outbox')
			continue
		}
		try {
			const res = await serverApi(op.method, path, {query, body, contentType: op.contentType})
			learnIds(op, res)
		} catch (e) {
			if (e instanceof NetworkError) return false
			if (!(e instanceof ApiError)) throw e
			if (e.status === 401) return false
			if (!rejected(e.status) && (op.attempts = (op.attempts ?? 0) + 1) < MAX_ATTEMPTS) {
				persist('outbox')
				return false
			}
			console.warn('offline: the server refused a change made offline', op.method, op.path, e.message)
		}
		outbox.shift()
		persist('outbox')
	}
	return true
}

/** Offline id → server id; null for offline ids the server never got. */
function mapNumber(n: number): number | null {
	if (n >= TEMP_BASE) return idMap.get(n) ?? null
	// Saved filters' pseudo projects: -(filter id) - 1.
	if (n <= -TEMP_BASE - 1) {
		const id = idMap.get(-n - 1)
		return id === undefined ? null : -id - 1
	}
	return n
}

const BIG_INT = /^-?\d{16,}$/

function remapPath(path: string, fn: (n: number) => number): string {
	return path.split('/').map(s => BIG_INT.test(s) ? String(remap(Number(s), fn)) : s).join('/')
}

/** Applies fn to every offline id in a JSON value: numbers, and numeric object keys. */
function remap(value: unknown, fn: (n: number) => number): unknown {
	if (typeof value === 'number') return Math.abs(value) >= TEMP_BASE ? fn(value) : value
	if (Array.isArray(value)) return value.map(v => remap(v, fn))
	if (value && typeof value === 'object') {
		return Object.fromEntries(Object.entries(value).map(([k, v]) => [k.replace(/-?\d{16,}/g, m => String(remap(Number(m), fn))), remap(v, fn)]))
	}
	return value
}

function learnIds(op: Op, res: unknown) {
	const local = op.result as Json | undefined
	const server = res as Json | undefined
	if (!local || !server) return
	const pair = (a: unknown, b: unknown) => {
		if (typeof a === 'number' && a >= TEMP_BASE && typeof b === 'number') idMap.set(a, b)
	}
	pair(local.id, server.id)
	const dup = local.duplicated_task as Json | undefined
	if (dup) pair(dup.id, ((server.duplicated_task ?? server) as Json).id)
	if (Array.isArray(local.views) && Array.isArray(server.views)) {
		for (const v of local.views as KanbanView[]) pair(v.id, (server.views as KanbanView[]).find(s => s.view_kind === v.view_kind)?.id)
	}
	persist('outbox')
}

/**
 * Kanban columns of a list created offline: the server made its own, so match
 * them up by order the first time a change refers to one.
 */
async function mapBucketsFor(op: Op) {
	const db = replica?.db
	if (!db) return
	const ids = [...JSON.stringify([op.path, op.body]).matchAll(/\d{16,}/g)].map(m => Number(m[0]))
	for (const id of ids) {
		if (idMap.has(id)) continue
		const bucket = db.buckets.find(b => b.id === id)
		if (!bucket) continue
		const project = db.projects.find(p => p.views?.some(v => v.id === bucket.project_view_id))
		const projectId = project && mapNumber(project.id)
		const viewId = mapNumber(bucket.project_view_id)
		if (!projectId || !viewId) continue
		try {
			const res = await serverApi<{items?: KanbanBucket[] | null}>('GET', `/projects/${projectId}/views/${viewId}/buckets/tasks`, {query: {page: 1}})
			const theirs = [...res?.items ?? []].sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
			const ours = db.buckets.filter(b => b.project_view_id === bucket.project_view_id).sort((a, b) => a.position - b.position)
			ours.forEach((b, i) => {
				if (b.id >= TEMP_BASE && theirs[i]) idMap.set(b.id, theirs[i].id)
			})
		} catch {
			// Left unmapped: the change is dropped.
		}
	}
}
