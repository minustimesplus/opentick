import {defineStore} from 'pinia'
import {computed, reactive, ref, shallowRef} from 'vue'
import {api, ApiError, fetchAll, getToken, onUnauthorized, setToken} from '@/api/client'
import type {AppNotification, Attachment, CurrentUser, Label, Project, RelationKind, Task, User} from '@/api/types'
import {addDays, dayDiff, getSchedule, isOverdue, startOfDay} from '@/lib/dates'
import {prefs} from '@/lib/prefs'
import {toast} from '@/lib/toast'
import {signOutOfClerk} from '@/lib/clerk'

const MERGE = 'application/merge-patch+json'
/** How many done-task queries a sync re-runs; older ones are dropped. */
const MAX_DONE_QUERIES = 8

function isNotFound(e: unknown) {
	return e instanceof ApiError && e.status === 404
}

export type TaskPatch = Partial<Omit<Task, 'id' | 'labels' | 'related_tasks' | 'assignees' | 'attachments'>>

/** Embedded collections a task PATCH response may leave out. */
function keepEmbedded(before: Task, updated: Task): Task {
	return {
		...before,
		...updated,
		labels: updated.labels ?? before.labels,
		related_tasks: updated.related_tasks ?? before.related_tasks,
		assignees: updated.assignees ?? before.assignees,
		attachments: updated.attachments ?? before.attachments,
		comment_count: updated.comment_count ?? before.comment_count,
	}
}

export function parentId(t: Task): number | undefined {
	return t.related_tasks?.parenttask?.[0]?.id
}

export const useData = defineStore('data', () => {
	const authed = ref(!!getToken())
	const user = shallowRef<CurrentUser | null>(null)
	const projects = ref<Project[]>([])
	/** Archived lists; loaded on demand, since syncs only fetch active ones. */
	const archivedProjects = ref<Project[]>([])
	const labels = ref<Label[]>([])
	const tasks = reactive(new Map<number, Task>())
	const loading = ref(false)
	const loaded = ref(false)
	const notifications = ref<AppNotification[]>([])
	/** Manual task order per project, from the project's list view: project id → task id → position. */
	const positions = reactive(new Map<number, Map<number, number>>())
	/** Everyone with access to a list (owner, shares, teams), by project id; loaded on demand. */
	const members = reactive(new Map<number, User[]>())
	/** Object URL of the user's uploaded avatar, null when they have none. */
	const avatarUrl = ref<string | null>(null)
	let avatarLoadedFor: string | null = null
	/**
	 * Done tasks come from several scoped queries (per project, per calendar
	 * range, ...). They are kept here, most recent last, so a sync can re-run
	 * them and drop done tasks that were deleted elsewhere.
	 */
	const doneQueries = new Map<string, () => Promise<Task[]>>()
	let inflight: Promise<void> | null = null
	/** Local creates/deletes while a sync runs, which its response predates. */
	const createdDuringSync = new Set<number>()
	const deletedDuringSync = new Set<number>()

	onUnauthorized(() => {
		authed.value = false
		reset()
	})

	function reset() {
		user.value = null
		projects.value = []
		archivedProjects.value = []
		labels.value = []
		tasks.clear()
		loaded.value = false
		doneQueries.clear()
		positions.clear()
		members.clear()
		setAvatarUrl(null)
		avatarLoadedFor = null
	}

	// ---------- auth ----------

	async function login(username: string, password: string, totp?: string) {
		const res = await api<{token: string}>('POST', '/login', {
			auth: false,
			body: {username, password, long_token: true, ...(totp ? {totp_passcode: totp} : {})},
		})
		setToken(res.token)
		authed.value = true
		await loadAll()
	}

	/** Exchanges a Clerk session token for an app session. */
	async function loginWithClerk(clerkToken: string, totp?: string) {
		const res = await api<{token: string}>('POST', '/auth/clerk', {
			auth: false,
			body: {token: clerkToken, ...(totp ? {totp_passcode: totp} : {})},
		})
		setToken(res.token)
		authed.value = true
		await loadAll()
	}

	async function register(username: string, email: string, password: string) {
		await api('POST', '/register', {auth: false, body: {username, email, password}})
		await login(username, password)
	}

	async function logout() {
		try {
			await api('POST', '/user/logout')
		} catch {
			// token may already be invalid
		}
		setToken(null)
		authed.value = false
		reset()
		await signOutOfClerk()
	}

	// ---------- loading ----------

	/** Full sync with the server. Concurrent calls share the running sync. */
	function loadAll(): Promise<void> {
		if (!authed.value) return Promise.resolve()
		inflight ??= syncAll().finally(() => {
			inflight = null
			createdDuringSync.clear()
			deletedDuringSync.clear()
		})
		return inflight
	}

	async function syncAll() {
		loading.value = true
		try {
			const doneSince = addDays(new Date(), -14).toISOString()
			const [u, ps, ls, open, done, scoped] = await Promise.all([
				api<CurrentUser>('GET', '/user'),
				fetchAll<Project>('/projects', {is_archived: false}),
				fetchAll<Label>('/labels'),
				fetchAll<Task>('/tasks', {filter: 'done = false', sort_by: ['due_date', 'id'], order_by: ['asc', 'asc'], expand: 'comment_count'}),
				fetchAll<Task>('/tasks', {filter: `done = true && done_at > "${doneSince}"`, sort_by: ['done_at'], order_by: ['desc']}, 4),
				Promise.all([...doneQueries.values()].map(q => q())),
			])
			user.value = u
			projects.value = ps
			labels.value = ls
			// Replace the task set; anything not returned again was deleted
			// (or moved out of every loaded scope) elsewhere.
			const fresh = [...open, ...done, ...scoped.flat()].filter(t => !deletedDuringSync.has(t.id))
			const ids = new Set(fresh.map(t => t.id))
			for (const id of tasks.keys()) {
				if (!ids.has(id) && !createdDuringSync.has(id)) tasks.delete(id)
			}
			for (const t of fresh) tasks.set(t.id, t)
			loaded.value = true
			loadNotifications()
			loadArchived()
			if (avatarLoadedFor !== u.username) loadAvatar()
		} finally {
			loading.value = false
		}
	}

	async function loadNotifications() {
		try {
			notifications.value = await fetchAll<AppNotification>('/notifications', {}, 1)
		} catch {
			notifications.value = []
		}
	}

	/** Runs a done-task query once and remembers it for later syncs. */
	async function loadDoneQuery(key: string, query: () => Promise<Task[]>) {
		if (doneQueries.has(key)) {
			// Mark as recently used.
			const q = doneQueries.get(key)!
			doneQueries.delete(key)
			doneQueries.set(key, q)
			return
		}
		doneQueries.set(key, query)
		while (doneQueries.size > MAX_DONE_QUERIES) {
			doneQueries.delete(doneQueries.keys().next().value!)
		}
		try {
			const res = await query()
			res.forEach(t => tasks.set(t.id, t))
		} catch (e) {
			doneQueries.delete(key)
			throw e
		}
	}

	function loadDoneForProject(projectId: number) {
		return loadDoneQuery(`project:${projectId}`, () =>
			fetchAll<Task>(`/projects/${projectId}/tasks`, {filter: 'done = true', sort_by: ['done_at'], order_by: ['desc']}, 4))
	}

	function loadDone(pages = 2) {
		return loadDoneQuery('all', () =>
			fetchAll<Task>('/tasks', {filter: 'done = true', sort_by: ['done_at'], order_by: ['desc']}, pages))
	}

	/** Loads done tasks scheduled within [from, to) for the calendar. */
	function loadDoneInRange(from: Date, to: Date) {
		const f = from.toISOString()
		const e = to.toISOString()
		return loadDoneQuery(`range:${f}:${e}`, () => fetchAll<Task>('/tasks', {
			filter: `done = true && ((due_date >= "${f}" && due_date < "${e}") || (start_date >= "${f}" && start_date < "${e}"))`,
		}, 5))
	}

	async function loadSavedFilterTasks(project: Project): Promise<Task[]> {
		const view = project.views?.find(v => v.view_kind === 'list') ?? project.views?.[0]
		if (!view) return []
		const res = await fetchAll<Task>(`/projects/${project.id}/views/${view.id}/tasks`, {}, 5)
		res.forEach(t => tasks.set(t.id, t))
		return res
	}

	function listViewId(projectId: number): number | undefined {
		const views = project(projectId)?.views
		return (views?.find(v => v.view_kind === 'list') ?? views?.[0])?.id
	}

	/** Loads the manual order of a project's open tasks. */
	async function loadPositions(projectId: number) {
		const view = listViewId(projectId)
		if (!view) return
		try {
			const res = await fetchAll<Task>(`/projects/${projectId}/views/${view}/tasks`, {filter: 'done = false'}, 10)
			positions.set(projectId, new Map(res.map(t => [t.id, t.position ?? 0])))
		} catch {
			// Keep the previous order.
		}
	}

	async function setPosition(projectId: number, taskId: number, position: number) {
		const view = listViewId(projectId)
		if (!view) return
		if (!positions.has(projectId)) positions.set(projectId, new Map())
		const map = positions.get(projectId)!
		const before = map.get(taskId)
		map.set(taskId, position)
		try {
			await api('PUT', `/tasks/${taskId}/position`, {body: {position, project_view_id: view}})
		} catch (e) {
			if (before === undefined) map.delete(taskId)
			else map.set(taskId, before)
			toast(String((e as Error).message))
		}
	}

	async function searchRemote(q: string): Promise<Task[]> {
		const res = await fetchAll<Task>('/tasks', {q}, 1)
		res.forEach(t => {
			if (!tasks.has(t.id)) tasks.set(t.id, t)
		})
		return res
	}

	// ---------- derived ----------

	const inboxId = computed(() => {
		const id = user.value?.settings?.default_project_id
		if (id && projects.value.some(p => p.id === id)) return id
		return projects.value.find(p => p.title.toLowerCase() === 'inbox')?.id ?? projects.value.find(p => p.id > 0)?.id ?? 0
	})

	const realProjects = computed(() => projects.value.filter(p => p.id > 0 && !p.is_archived))
	const savedFilters = computed(() => projects.value.filter(p => p.id < -1))
	const projectMap = computed(() => new Map(projects.value.map(p => [p.id, p])))
	const listProjects = computed(() => realProjects.value
		.filter(p => p.id !== inboxId.value)
		.sort((a, b) => (a.position ?? 0) - (b.position ?? 0)))

	const allTasks = computed(() => [...tasks.values()])
	const openTasks = computed(() => allTasks.value.filter(t => !t.done && projectMap.value.has(t.project_id)))

	function isToday(t: Task, now = new Date()) {
		const s = getSchedule(t)
		if (!s) return false
		return dayDiff(s.date, now) === 0 || (s.end != null && s.start <= now && s.end >= startOfDay(now)) || isOverdue(t, now)
	}

	function inNext7(t: Task, now = new Date()) {
		const s = getSchedule(t)
		if (!s) return false
		return dayDiff(s.date, now) < 7
	}

	const counts = computed(() => {
		const now = new Date()
		const byProject = new Map<number, number>()
		const byLabel = new Map<number, number>()
		let today = 0
		let week = 0
		for (const t of openTasks.value) {
			byProject.set(t.project_id, (byProject.get(t.project_id) ?? 0) + 1)
			t.labels?.forEach(l => byLabel.set(l.id, (byLabel.get(l.id) ?? 0) + 1))
			if (isToday(t, now)) today++
			if (inNext7(t, now)) week++
		}
		return {today, week, byProject, byLabel}
	})

	function project(id: number) {
		return projectMap.value.get(id) ?? archivedProjects.value.find(p => p.id === id)
	}

	/** Loaded tasks by their parent task's id. */
	const childIndex = computed(() => {
		const out = new Map<number, Task[]>()
		for (const t of tasks.values()) {
			const p = parentId(t)
			if (p === undefined) continue
			if (!out.has(p)) out.set(p, [])
			out.get(p)!.push(t)
		}
		return out
	})

	/**
	 * A task's subtasks: loaded ones plus the parent's own snapshots of
	 * subtasks that are not loaded (old done ones, say).
	 */
	function subtasksOf(id: number): Task[] {
		const live = childIndex.value.get(id) ?? []
		const ids = new Set(live.map(t => t.id))
		const snaps = (tasks.get(id)?.related_tasks?.subtask ?? []).filter(t => !ids.has(t.id))
		return [...live, ...snaps].sort((a, b) => Number(a.done) - Number(b.done) || a.id - b.id)
	}

	/** Adds tasks that are not loaded yet, such as subtask snapshots, so they can be edited. */
	function adoptTasks(list: Task[]) {
		for (const t of list) if (!tasks.has(t.id)) tasks.set(t.id, t)
	}

	// ---------- task mutations ----------

	async function createTask(input: TaskPatch & {title: string, project_id?: number}, labelTitles: string[] = []): Promise<Task | null> {
		const projectId = input.project_id || prefs.defaultListId || inboxId.value
		try {
			const created = await api<Task>('POST', `/projects/${projectId}/tasks`, {body: {...input, project_id: projectId}, query: {format: 'markdown'}})
			tasks.set(created.id, created)
			if (inflight) createdDuringSync.add(created.id)
			for (const title of labelTitles) {
				await addLabelByTitle(created.id, title)
			}
			return tasks.get(created.id) ?? created
		} catch (e) {
			toast(String((e as Error).message))
			return null
		}
	}

	async function updateTask(id: number, patch: TaskPatch): Promise<Task | null> {
		const before = tasks.get(id)
		if (!before) return null
		tasks.set(id, {...before, ...patch} as Task)
		try {
			const updated = await api<Task>('PATCH', `/tasks/${id}`, {body: patch, contentType: MERGE, query: {format: 'markdown'}})
			// Keep embedded fields the PATCH response omits.
			tasks.set(id, keepEmbedded(before, updated))
			return tasks.get(id)!
		} catch (e) {
			if (isNotFound(e)) {
				// Deleted elsewhere since the last sync.
				tasks.delete(id)
			} else {
				tasks.set(id, before)
			}
			toast(String((e as Error).message))
			return null
		}
	}

	async function toggleDone(id: number) {
		const t = tasks.get(id)
		if (!t) return
		const done = !t.done
		const res = await updateTask(id, {done, wont_do: false, done_at: done ? new Date().toISOString() : null})
		if (res && done && !res.done && res.repeat_after) {
			toast('↻ ' + res.title)
		}
	}

	/** Closes a task as won't do, or re-opens it. The server keeps done and wont_do in step. */
	async function toggleWontDo(id: number) {
		const t = tasks.get(id)
		if (!t) return
		const wontDo = !(t.done && t.wont_do)
		const res = await updateTask(id, {done: wontDo, wont_do: wontDo, done_at: wontDo ? new Date().toISOString() : null})
		if (res && wontDo && !res.done && res.repeat_after) {
			toast('↻ ' + res.title)
		}
	}

	async function deleteTask(id: number) {
		const before = tasks.get(id)
		const parent = before ? parentId(before) : undefined
		if (parent !== undefined) setRelated(parent, 'subtask', l => l.filter(x => x.id !== id))
		tasks.delete(id)
		if (inflight) deletedDuringSync.add(id)
		try {
			await api('DELETE', `/tasks/${id}`)
		} catch (e) {
			// Already deleted elsewhere: the goal is reached.
			if (isNotFound(e)) return
			deletedDuringSync.delete(id)
			if (before) tasks.set(id, before)
			toast(String((e as Error).message))
		}
	}

	async function duplicateTask(id: number): Promise<Task | null> {
		try {
			const res = await api<{duplicated_task?: Task} & Task>('POST', `/tasks/${id}/duplicate`, {query: {format: 'markdown'}})
			const dup = res.duplicated_task ?? res
			if (dup?.id) {
				tasks.set(dup.id, dup)
				if (inflight) createdDuringSync.add(dup.id)
			}
			return dup
		} catch (e) {
			toast(String((e as Error).message))
			return null
		}
	}

	// ---------- relations ----------

	function strip(t: Task): Task {
		// Snapshots inside related_tasks must not nest further.
		const {related_tasks: _r, ...rest} = t
		return rest as Task
	}

	function setRelated(id: number, kind: RelationKind, update: (list: Task[]) => Task[]) {
		const t = tasks.get(id)
		if (!t) return
		const rel = {...(t.related_tasks ?? {})}
		rel[kind] = update(rel[kind] ?? [])
		tasks.set(id, {...t, related_tasks: rel})
	}

	/** Records child as a subtask of parent locally, on both sides. */
	function linkLocal(parent: number, child: number) {
		const p = tasks.get(parent)
		const c = tasks.get(child)
		if (!p || !c) return
		setRelated(parent, 'subtask', l => [...l.filter(x => x.id !== child), strip(c)])
		setRelated(child, 'parenttask', () => [strip(p)])
	}

	function unlinkLocal(parent: number, child: number) {
		setRelated(parent, 'subtask', l => l.filter(x => x.id !== child))
		setRelated(child, 'parenttask', l => l.filter(x => x.id !== parent))
	}

	/** Creates a task as a subtask of `parent`, in the parent's list. */
	async function createSubtask(parent: number, input: TaskPatch & {title: string}): Promise<Task | null> {
		const p = tasks.get(parent)
		if (!p) return null
		const created = await createTask({...input, project_id: p.project_id})
		if (!created) return null
		await setParent(created.id, parent)
		return tasks.get(created.id) ?? created
	}

	/** Moves a task under another one, or to the top level with `null`. */
	async function setParent(child: number, parent: number | null) {
		const c = tasks.get(child)
		if (!c) return
		const old = parentId(c)
		if (old === (parent ?? undefined) || child === parent) return
		try {
			if (old !== undefined) {
				unlinkLocal(old, child)
				await api('DELETE', `/tasks/${child}/relations/parenttask/${old}`)
			}
			if (parent !== null) {
				linkLocal(parent, child)
				await api('POST', `/tasks/${parent}/relations`, {body: {other_task_id: child, relation_kind: 'subtask'}})
			}
		} catch (e) {
			if (parent !== null) unlinkLocal(parent, child)
			if (old !== undefined) linkLocal(old, child)
			toast(String((e as Error).message))
		}
	}

	// ---------- members and assignees ----------

	async function loadMembers(projectId: number, force = false): Promise<User[]> {
		if (!force && members.has(projectId)) return members.get(projectId)!
		try {
			const list = await fetchAll<User>(`/projects/${projectId}/users/search`, {}, 5)
			members.set(projectId, list)
			return list
		} catch {
			return []
		}
	}

	/** A list counts as shared once someone besides the current user can see it. */
	function isShared(projectId: number): boolean {
		return (members.get(projectId) ?? []).some(u => u.id !== user.value?.id)
	}

	async function assign(taskId: number, u: User) {
		const t = tasks.get(taskId)
		if (!t || t.assignees?.some(a => a.id === u.id)) return
		patchLocal(taskId, {assignees: [...(t.assignees ?? []), u]})
		try {
			await api('POST', `/tasks/${taskId}/assignees`, {body: {user_id: u.id}})
		} catch (e) {
			patchLocal(taskId, {assignees: t.assignees})
			toast(String((e as Error).message))
		}
	}

	async function unassign(taskId: number, userId: number) {
		const t = tasks.get(taskId)
		if (!t) return
		patchLocal(taskId, {assignees: (t.assignees ?? []).filter(a => a.id !== userId)})
		try {
			await api('DELETE', `/tasks/${taskId}/assignees/${userId}`)
		} catch (e) {
			if (!isNotFound(e)) {
				patchLocal(taskId, {assignees: t.assignees})
				toast(String((e as Error).message))
			}
		}
	}

	// ---------- attachments ----------

	/** Merges fields into a loaded task without saving them, e.g. after a side request. */
	function patchLocal(id: number, patch: Partial<Task>) {
		const t = tasks.get(id)
		if (t) tasks.set(id, {...t, ...patch})
	}

	async function uploadAttachments(taskId: number, files: File[]) {
		if (!files.length) return
		const body = new FormData()
		files.forEach(f => body.append('files', f))
		try {
			const res = await api<{success?: Attachment[] | null, errors?: {message?: string}[] | null}>('POST', `/tasks/${taskId}/attachments`, {body})
			const added = res.success ?? []
			patchLocal(taskId, {attachments: [...(tasks.get(taskId)?.attachments ?? []), ...added]})
			res.errors?.forEach(e => toast(e.message ?? 'Upload failed'))
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	async function deleteAttachment(taskId: number, attachmentId: number) {
		const before = tasks.get(taskId)?.attachments ?? []
		patchLocal(taskId, {attachments: before.filter(a => a.id !== attachmentId)})
		try {
			await api('DELETE', `/tasks/${taskId}/attachments/${attachmentId}`)
		} catch (e) {
			if (!isNotFound(e)) {
				patchLocal(taskId, {attachments: before})
				toast(String((e as Error).message))
			}
		}
	}

	async function addLabel(taskId: number, label: Label) {
		const t = tasks.get(taskId)
		if (!t || t.labels?.some(l => l.id === label.id)) return
		tasks.set(taskId, {...t, labels: [...(t.labels ?? []), label]})
		try {
			await api('POST', `/tasks/${taskId}/labels`, {body: {label_id: label.id}})
		} catch (e) {
			tasks.set(taskId, t)
			toast(String((e as Error).message))
		}
	}

	async function removeLabel(taskId: number, labelId: number) {
		const t = tasks.get(taskId)
		if (!t) return
		tasks.set(taskId, {...t, labels: (t.labels ?? []).filter(l => l.id !== labelId)})
		try {
			await api('DELETE', `/tasks/${taskId}/labels/${labelId}`)
		} catch (e) {
			tasks.set(taskId, t)
			toast(String((e as Error).message))
		}
	}

	async function ensureLabel(title: string): Promise<Label> {
		const existing = labels.value.find(l => l.title.toLowerCase() === title.toLowerCase())
		if (existing) return existing
		const created = await api<Label>('POST', '/labels', {body: {title}})
		labels.value = [...labels.value, created]
		return created
	}

	async function addLabelByTitle(taskId: number, title: string) {
		try {
			const label = await ensureLabel(title)
			await addLabel(taskId, label)
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	// ---------- projects ----------

	async function createProject(input: Partial<Project> & {title: string}): Promise<Project | null> {
		try {
			const p = await api<Project>('POST', '/projects', {body: input})
			projects.value = [...projects.value, p]
			return p
		} catch (e) {
			toast(String((e as Error).message))
			return null
		}
	}

	async function updateProject(id: number, patch: Partial<Project>) {
		const before = projects.value.find(p => p.id === id)
		if (!before) return
		projects.value = projects.value.map(p => p.id === id ? {...p, ...patch} : p)
		try {
			const updated = await api<Project>('PATCH', `/projects/${id}`, {body: patch, contentType: MERGE})
			projects.value = projects.value.map(p => p.id === id ? {...p, ...updated} : p)
		} catch (e) {
			projects.value = projects.value.map(p => p.id === id ? before : p)
			toast(String((e as Error).message))
		}
	}

	async function loadArchived() {
		try {
			archivedProjects.value = (await fetchAll<Project>('/projects', {is_archived: true})).filter(p => p.is_archived)
		} catch {
			archivedProjects.value = []
		}
	}

	/** Archives or restores a list (and, on the server, its child lists). */
	async function setArchived(id: number, archived: boolean) {
		try {
			const updated = await api<Project>('PATCH', `/projects/${id}`, {body: {is_archived: archived}, contentType: MERGE})
			if (archived) {
				projects.value = projects.value.filter(p => p.id !== id && p.parent_project_id !== id)
				archivedProjects.value = [...archivedProjects.value.filter(p => p.id !== id), updated]
			} else {
				archivedProjects.value = archivedProjects.value.filter(p => p.id !== id)
				projects.value = await fetchAll<Project>('/projects', {is_archived: false})
			}
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	// ---------- saved filters ----------

	/** Creates (no id) or replaces a saved filter, then reloads the sidebar's filters. */
	async function saveFilter(id: number | null, title: string, filter: string): Promise<number | null> {
		const body = {title, filters: {filter, filter_include_nulls: false}}
		try {
			const saved = id === null
				? await api<{id: number}>('POST', '/filters', {body})
				: await api<{id: number}>('PUT', `/filters/${id}`, {body})
			projects.value = await fetchAll<Project>('/projects', {is_archived: false})
			return saved.id
		} catch (e) {
			toast(String((e as Error).message))
			return null
		}
	}

	async function deleteFilter(id: number) {
		try {
			await api('DELETE', `/filters/${id}`)
			projects.value = projects.value.filter(p => p.id !== -id - 1)
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	async function deleteProject(id: number) {
		try {
			await api('DELETE', `/projects/${id}`)
			projects.value = projects.value.filter(p => p.id !== id && p.parent_project_id !== id)
			archivedProjects.value = archivedProjects.value.filter(p => p.id !== id)
			for (const [tid, t] of tasks) if (t.project_id === id) tasks.delete(tid)
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	// ---------- avatar ----------

	function setAvatarUrl(url: string | null) {
		if (avatarUrl.value) URL.revokeObjectURL(avatarUrl.value)
		avatarUrl.value = url
	}

	/** Fetches the avatar image, but only an uploaded one: other providers fall back to the initial. */
	async function loadAvatar() {
		const username = user.value?.username
		if (!username) return
		avatarLoadedFor = username
		try {
			const {avatar_provider: provider} = await api<{avatar_provider: string}>('GET', '/user/settings/avatar/provider')
			if (provider !== 'upload') {
				setAvatarUrl(null)
				return
			}
			const blob = await api<Blob>('GET', `/avatar/${encodeURIComponent(username)}`, {query: {size: 256}, blob: true})
			setAvatarUrl(URL.createObjectURL(blob))
		} catch {
			avatarLoadedFor = null
		}
	}

	async function uploadAvatar(file: File) {
		const body = new FormData()
		body.append('avatar', file)
		await api('PUT', '/user/settings/avatar', {body})
		await loadAvatar()
	}

	async function removeAvatar() {
		await api('PUT', '/user/settings/avatar/provider', {body: {avatar_provider: 'initials'}})
		setAvatarUrl(null)
	}

	async function markNotificationsRead() {
		try {
			await api('POST', '/notifications')
			notifications.value = notifications.value.map(n => ({...n, read_at: new Date().toISOString()}))
		} catch {
			// ignore
		}
	}

	return {
		authed, user, projects, labels, tasks, loading, loaded, notifications, avatarUrl,
		login, loginWithClerk, register, logout, loadAll, loadDone, loadDoneForProject, loadDoneInRange, loadSavedFilterTasks, searchRemote,
		inboxId, realProjects, listProjects, savedFilters, projectMap, allTasks, openTasks, counts, project, isToday, inNext7,
		members, loadMembers, isShared, assign, unassign,
		positions, loadPositions, setPosition, patchLocal, uploadAttachments, deleteAttachment,
		childIndex, subtasksOf, adoptTasks, createSubtask, setParent,
		createTask, updateTask, toggleDone, toggleWontDo, deleteTask, duplicateTask, addLabel, removeLabel, addLabelByTitle, ensureLabel,
		archivedProjects, loadArchived, setArchived,
		createProject, updateProject, deleteProject, saveFilter, deleteFilter, markNotificationsRead, loadNotifications,
		uploadAvatar, removeAvatar,
	}
})
