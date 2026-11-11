import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import type {Paginated} from '../client'
import type {DB} from '../local/db'
import type {Label, Project, Task} from '../types'

// The "server" is the in-browser backend running on its own database, behind
// a fake fetch() that can be switched off to simulate losing the connection.

type Client = typeof import('../client')
type Offline = typeof import('./index')

let server: DB
let down: 'unreachable' | 'lost' | 'gateway' | false
let files: Record<string, string>
let client: Client
let offline: Offline
let requests: string[]

function toQuery(params: URLSearchParams) {
	const out: Record<string, string | string[]> = {}
	for (const key of new Set(params.keys())) {
		const all = params.getAll(key)
		out[key] = all.length > 1 ? all : all[0]
	}
	return out
}

beforeEach(async () => {
	vi.resetModules()
	localStorage.clear()
	files = {}
	down = false
	requests = []
	window.ttDesktop = {
		getServerUrl: async () => 'https://tasks.example.com',
		setServerUrl: async (url: string) => url,
		onUpdateReady: () => {},
		installUpdate: async () => {},
		offlineStore: {
			read: async name => files[name] ?? null,
			write: async (name, text) => { files[name] = text },
			writeSync: (name, text) => { files[name] = text },
			clear: async () => { files = {} },
		},
	}

	const {seed} = await import('../local/seed')
	server = seed()
	const backend = await import('../local/backend')
	const {ApiError} = await import('../client')

	vi.stubGlobal('fetch', async (input: string, init: RequestInit = {}) => {
		const url = new URL(input, 'http://127.0.0.1')
		const method = (init.method ?? 'GET').toUpperCase()
		const path = url.pathname.replace(/^\/api\/v2/, '')
		requests.push(`${method} ${path}`)
		if (down === 'gateway') {
			return new Response('<html>502 Bad Gateway</html>', {status: 502, headers: {'Content-Type': 'text/html'}})
		}
		if (down) {
			return new Response(JSON.stringify({message: 'Could not reach the server'}), {status: 502, headers: {'X-OpenTick-Offline': down}})
		}
		try {
			const body = typeof init.body === 'string' ? JSON.parse(init.body) : undefined
			const res = backend.answer(server, method, path, {query: toQuery(url.searchParams), body})
			return new Response(JSON.stringify(res ?? {}), {status: 200, headers: {'Content-Type': 'application/json'}})
		} catch (e) {
			if (e instanceof ApiError) return new Response(JSON.stringify({message: e.message}), {status: e.status})
			throw e
		}
	})

	client = await import('../client')
	offline = await import('./index')
	client.setToken('token')
})

afterEach(() => {
	vi.unstubAllGlobals()
	delete window.ttDesktop
})

/** Lets queued file writes run. */
const settle = () => new Promise(resolve => setTimeout(resolve, 20))

async function openTasks(): Promise<Task[]> {
	return client.fetchAll<Task>('/tasks', {filter: 'done = false'})
}

function serverTask(title: string) {
	return server.tasks.find(t => t.title === title && !t.deleted_at)
}

describe('offline mode', () => {
	it('keeps working from the offline copy and replays changes when back online', async () => {
		const before = await openTasks()
		await offline.refreshOfflineCopy()
		await settle()
		expect(files.replica).toBeTruthy()

		down = 'unreachable'
		// Reads come from the offline copy.
		const offlineOpen = await openTasks()
		expect(offlineOpen.map(t => t.id).sort()).toEqual(before.map(t => t.id).sort())
		expect(offline.isOffline()).toBe(true)

		// Changes made offline: a new list with a labelled task in it, an edit,
		// a delete and a subtask relation.
		const [existing, gone, child] = before
		const list = await client.api<Project>('POST', '/projects', {body: {title: 'Made offline'}})
		expect(list.id).toBeGreaterThanOrEqual(offline.TEMP_BASE)
		const task = await client.api<Task>('POST', `/projects/${list.id}/tasks`, {body: {title: 'Offline task'}})
		const label = await client.api<Label>('POST', '/labels', {body: {title: 'offline-label'}})
		await client.api('POST', `/tasks/${task.id}/labels`, {body: {label_id: label.id}})
		await client.api('PATCH', `/tasks/${existing.id}`, {body: {title: 'Renamed offline'}, contentType: 'application/merge-patch+json'})
		await client.api('DELETE', `/tasks/${gone.id}`)
		await client.api('POST', `/tasks/${task.id}/relations`, {body: {other_task_id: child.id, relation_kind: 'subtask'}})
		await client.api('PATCH', '/user/settings/general', {body: {frontend_settings: {views: {listSort: {[`p${list.id}`]: 'title'}}}}, contentType: 'application/merge-patch+json'})

		// The app sees its changes while offline...
		const now = await openTasks()
		const shown = now.find(t => t.id === task.id)!
		expect(shown.labels?.map(l => l.title)).toEqual(['offline-label'])
		expect(shown.related_tasks?.subtask?.map(t => t.id)).toEqual([child.id])
		expect(now.find(t => t.id === existing.id)?.title).toBe('Renamed offline')
		expect(now.some(t => t.id === gone.id)).toBe(false)
		// ...and they survive a restart.
		await settle()
		expect(JSON.parse(files.outbox).ops).toHaveLength(8)
		// The server has none of them yet.
		expect(server.projects.some(p => p.title === 'Made offline')).toBe(false)

		const reconnected = vi.fn()
		offline.onReconnect(reconnected)
		down = false
		expect(await offline.checkConnection()).toBe(true)

		const serverList = server.projects.find(p => p.title === 'Made offline')!
		const created = serverTask('Offline task')!
		expect(created.project_id).toBe(serverList.id)
		const serverLabel = server.labels.find(l => l.title === 'offline-label')!
		expect(created.label_ids).toEqual([serverLabel.id])
		expect(server.relations).toContainEqual({task_id: created.id, other_task_id: child.id, kind: 'subtask'})
		expect(serverTask('Renamed offline')?.id).toBe(existing.id)
		expect(server.tasks.find(t => t.id === gone.id)?.deleted_at).toBeTruthy()
		await settle()
		expect(JSON.parse(files.outbox).ops).toHaveLength(0)

		const ids: Map<number, number> = reconnected.mock.calls[0][0]
		expect(ids.get(list.id)).toBe(serverList.id)
		expect(ids.get(task.id)).toBe(created.id)
		expect(offline.remapText(`/tasks/p${list.id}/${task.id}`, ids)).toBe(`/tasks/p${serverList.id}/${created.id}`)

		// Back online, requests go to the server again.
		requests = []
		await openTasks()
		expect(requests).toContain('GET /tasks')
		// The app may still use an offline id until it reloaded.
		await client.api('PATCH', `/tasks/${task.id}`, {body: {title: 'Edited right after'}, contentType: 'application/merge-patch+json'})
		expect(server.tasks.find(t => t.id === created.id)?.title).toBe('Edited right after')
	})

	it('sends changes left from a previous run before anything else', async () => {
		await offline.refreshOfflineCopy()
		down = 'unreachable'
		const inbox = server.user.settings.default_project_id
		await client.api('POST', `/projects/${inbox}/tasks`, {body: {title: 'Before quitting'}})
		await settle()

		// Restart with the connection back.
		vi.resetModules()
		down = false
		client = await import('../client')
		offline = await import('./index')
		const tasks = await openTasks()
		// Answered from the offline copy until the queued change went out...
		expect(tasks.some(t => t.title === 'Before quitting')).toBe(true)
		expect(await offline.checkConnection()).toBe(true)
		// ...then from the server, which has it now.
		expect(serverTask('Before quitting')).toBeTruthy()
		expect((await openTasks()).filter(t => t.title === 'Before quitting')).toHaveLength(1)
	})

	it('answers with the server data when online and follows changes from other devices', async () => {
		await offline.refreshOfflineCopy()
		// Another device adds a task.
		const {answer} = await import('../local/backend')
		const inbox = server.user.settings.default_project_id
		answer(server, 'POST', `/projects/${inbox}/tasks`, {body: {title: 'From the phone'}})
		expect((await openTasks()).some(t => t.title === 'From the phone')).toBe(true)

		// Seen online, so it is there offline too, even before the next pull.
		down = 'unreachable'
		expect((await openTasks()).some(t => t.title === 'From the phone')).toBe(true)
	})

	it('does not queue a change that may have reached the server', async () => {
		await offline.refreshOfflineCopy()
		const inbox = server.user.settings.default_project_id
		down = 'lost'
		await expect(client.api('POST', `/projects/${inbox}/tasks`, {body: {title: 'Maybe sent'}})).rejects.toThrow()
		await settle()
		expect(JSON.parse(files.outbox ?? '{"ops":[]}').ops).toHaveLength(0)
		// Later changes are queued as usual.
		const task = await client.api<Task>('POST', `/projects/${inbox}/tasks`, {body: {title: 'Queued'}})
		expect(task.id).toBeGreaterThanOrEqual(offline.TEMP_BASE)
	})

	it('explains that server-only features need a connection', async () => {
		await offline.refreshOfflineCopy()
		down = 'unreachable'
		await client.fetchAll('/tasks', {}, 1)
		await expect(client.api('POST', '/tokens', {body: {title: 'x'}})).rejects.toMatchObject({status: 503})
	})

	it('drops changes made offline by another account', async () => {
		await offline.refreshOfflineCopy()
		down = 'unreachable'
		const inbox = server.user.settings.default_project_id
		await client.api('POST', `/projects/${inbox}/tasks`, {body: {title: 'Not yours'}})
		server.user = {...server.user, id: server.user.id + 1}
		down = false
		expect(await offline.checkConnection()).toBe(true)
		expect(serverTask('Not yours')).toBeUndefined()
	})

	it('treats a reverse proxy in front of a server that is down as offline', async () => {
		await offline.refreshOfflineCopy()
		down = 'gateway'
		expect((await openTasks()).length).toBeGreaterThan(0)
		expect(offline.isOffline()).toBe(true)
	})

	it('keeps the API\'s own gateway errors', async () => {
		await offline.refreshOfflineCopy()
		vi.stubGlobal('fetch', async () => new Response(JSON.stringify({title: 'Bad Gateway', detail: 'Could not load the calendar', code: 20003}), {status: 502}))
		await expect(client.api('GET', '/calendar-subscriptions/1/events')).rejects.toMatchObject({status: 502, code: 20003})
		expect(offline.isOffline()).toBe(false)
	})

	it('serves a cached copy of server-only reads', async () => {
		await offline.refreshOfflineCopy()
		const online = await client.api<Paginated<unknown>>('GET', '/notifications')
		down = 'unreachable'
		await client.fetchAll('/tasks', {}, 1)
		expect(await client.api('GET', '/notifications')).toEqual(online)
	})
})
