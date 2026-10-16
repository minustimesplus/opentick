import {defineStore} from 'pinia'
import {computed, reactive, ref, shallowRef} from 'vue'
import {api, fetchAll, getToken, onUnauthorized, setToken} from '@/api/client'
import type {AppNotification, CurrentUser, Label, Project, Task} from '@/api/types'
import {addDays, dayDiff, getSchedule, isOverdue, startOfDay} from '@/lib/dates'
import {prefs} from '@/lib/prefs'
import {toast} from '@/lib/toast'

const MERGE = 'application/merge-patch+json'

export type TaskPatch = Partial<Omit<Task, 'id' | 'labels'>>

export const useData = defineStore('data', () => {
	const authed = ref(!!getToken())
	const user = shallowRef<CurrentUser | null>(null)
	const projects = ref<Project[]>([])
	const labels = ref<Label[]>([])
	const tasks = reactive(new Map<number, Task>())
	const loading = ref(false)
	const loaded = ref(false)
	const notifications = ref<AppNotification[]>([])
	/** Projects whose done tasks were fully fetched. */
	const doneLoaded = new Set<number>()
	const doneRanges = new Set<string>()

	onUnauthorized(() => {
		authed.value = false
		reset()
	})

	function reset() {
		user.value = null
		projects.value = []
		labels.value = []
		tasks.clear()
		loaded.value = false
		doneLoaded.clear()
		doneRanges.clear()
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
	}

	// ---------- loading ----------

	async function loadAll() {
		if (!authed.value) return
		loading.value = true
		try {
			const doneSince = addDays(new Date(), -14).toISOString()
			const [u, ps, ls, open, done] = await Promise.all([
				api<CurrentUser>('GET', '/user'),
				fetchAll<Project>('/projects', {is_archived: false}),
				fetchAll<Label>('/labels'),
				fetchAll<Task>('/tasks', {filter: 'done = false', sort_by: ['due_date', 'id'], order_by: ['asc', 'asc']}),
				fetchAll<Task>('/tasks', {filter: `done = true && done_at > "${doneSince}"`, sort_by: ['done_at'], order_by: ['desc']}, 4),
			])
			user.value = u
			projects.value = ps
			labels.value = ls
			// Replace open tasks; tasks that disappeared were done or deleted elsewhere.
			const openIds = new Set(open.map(t => t.id))
			for (const [id, t] of tasks) {
				if (!t.done && !openIds.has(id)) tasks.delete(id)
			}
			for (const t of [...open, ...done]) tasks.set(t.id, t)
			loaded.value = true
			loadNotifications()
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

	async function loadDoneForProject(projectId: number) {
		if (doneLoaded.has(projectId)) return
		doneLoaded.add(projectId)
		const done = await fetchAll<Task>(`/projects/${projectId}/tasks`, {filter: 'done = true', sort_by: ['done_at'], order_by: ['desc']}, 4)
		done.forEach(t => tasks.set(t.id, t))
	}

	async function loadDone(pages = 2) {
		if (doneLoaded.has(0)) return
		doneLoaded.add(0)
		const done = await fetchAll<Task>('/tasks', {filter: 'done = true', sort_by: ['done_at'], order_by: ['desc']}, pages)
		done.forEach(t => tasks.set(t.id, t))
	}

	/** Loads done tasks scheduled within [from, to) for the calendar. */
	async function loadDoneInRange(from: Date, to: Date) {
		const key = from.toISOString() + to.toISOString()
		if (doneRanges.has(key)) return
		doneRanges.add(key)
		const f = from.toISOString()
		const e = to.toISOString()
		const done = await fetchAll<Task>('/tasks', {
			filter: `done = true && ((due_date >= "${f}" && due_date < "${e}") || (start_date >= "${f}" && start_date < "${e}"))`,
		}, 5)
		done.forEach(t => tasks.set(t.id, t))
	}

	async function loadSavedFilterTasks(project: Project): Promise<Task[]> {
		const view = project.views?.find(v => v.view_kind === 'list') ?? project.views?.[0]
		if (!view) return []
		const res = await fetchAll<Task>(`/projects/${project.id}/views/${view.id}/tasks`, {}, 5)
		res.forEach(t => tasks.set(t.id, t))
		return res
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
		return projectMap.value.get(id)
	}

	// ---------- task mutations ----------

	async function createTask(input: TaskPatch & {title: string, project_id?: number}, labelTitles: string[] = []): Promise<Task | null> {
		const projectId = input.project_id || prefs.defaultListId || inboxId.value
		try {
			const created = await api<Task>('POST', `/projects/${projectId}/tasks`, {body: {...input, project_id: projectId}, query: {format: 'markdown'}})
			tasks.set(created.id, created)
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
			tasks.set(id, {...before, ...updated, labels: updated.labels ?? before.labels})
			return tasks.get(id)!
		} catch (e) {
			tasks.set(id, before)
			toast(String((e as Error).message))
			return null
		}
	}

	async function toggleDone(id: number) {
		const t = tasks.get(id)
		if (!t) return
		const done = !t.done
		const res = await updateTask(id, {done, done_at: done ? new Date().toISOString() : null})
		if (res && done && !res.done && res.repeat_after) {
			toast('↻ ' + res.title)
		}
	}

	async function deleteTask(id: number) {
		const before = tasks.get(id)
		tasks.delete(id)
		try {
			await api('DELETE', `/tasks/${id}`)
		} catch (e) {
			if (before) tasks.set(id, before)
			toast(String((e as Error).message))
		}
	}

	async function duplicateTask(id: number): Promise<Task | null> {
		try {
			const res = await api<{duplicated_task?: Task} & Task>('POST', `/tasks/${id}/duplicate`, {query: {format: 'markdown'}})
			const dup = res.duplicated_task ?? res
			if (dup?.id) tasks.set(dup.id, dup)
			return dup
		} catch (e) {
			toast(String((e as Error).message))
			return null
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

	async function deleteProject(id: number) {
		try {
			await api('DELETE', `/projects/${id}`)
			projects.value = projects.value.filter(p => p.id !== id && p.parent_project_id !== id)
			for (const [tid, t] of tasks) if (t.project_id === id) tasks.delete(tid)
		} catch (e) {
			toast(String((e as Error).message))
		}
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
		authed, user, projects, labels, tasks, loading, loaded, notifications,
		login, register, logout, loadAll, loadDone, loadDoneForProject, loadDoneInRange, loadSavedFilterTasks, searchRemote,
		inboxId, realProjects, listProjects, savedFilters, projectMap, allTasks, openTasks, counts, project, isToday, inNext7,
		createTask, updateTask, toggleDone, deleteTask, duplicateTask, addLabel, removeLabel, addLabelByTitle, ensureLabel,
		createProject, updateProject, deleteProject, markNotificationsRead, loadNotifications,
	}
})
