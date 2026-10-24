// CSV backup in TickTick's backup format, so a file moves freely between
// TickTick and OpenTick. TickTick backups import through the server's TickTick
// migrator; any other CSV goes through the generic CSV migrator.

import {api, fetchAll} from '@/api/client'
import type {Project, Task} from '@/api/types'
import {addDays, getSchedule, parseDate, startOfDay} from './dates'
import {buildRule, parseRepeat} from './repeat'
import {level} from './tasks'

const TICKTICK_HEADERS = ['Folder Name', 'List Name', 'Title', 'Kind', 'Tags', 'Content', 'Is Check list', 'Start Date', 'Due Date', 'Reminder', 'Repeat', 'Priority', 'Status', 'Created Time', 'Completed Time', 'Order', 'Timezone', 'Is All Day', 'Is Floating', 'Column Name', 'Column Order', 'View Mode', 'taskId', 'parentId', 'projectKind']

// TickTick's own priorities for none, low, medium and high.
const TICKTICK_PRIORITY = ['0', '1', '3', '5']

const pad = (n: number) => String(n).padStart(2, '0')

/** TickTick's timestamp format: UTC with a "+0000" offset. */
function tickTickDate(d: Date | null) {
	if (!d) return ''
	return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}T${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}+0000`
}

/** A reminder offset as TickTick writes it: "-PT10M" for timed tasks, "P0DT9H0M0S" for all-day ones. */
function tickTickDuration(ms: number, allDay: boolean) {
	const minutes = Math.round(Math.abs(ms) / 60_000)
	if (!minutes) return 'PT0S'
	const sign = ms < 0 ? '-' : ''
	if (!allDay) return `${sign}PT${minutes}M`
	return `${sign}P${Math.floor(minutes / 1440)}DT${Math.floor(minutes % 1440 / 60)}H${minutes % 60}M0S`
}

function quote(value: string) {
	return `"${value.replace(/"/g, '""')}"`
}

function reminderTimes(task: Task): Date[] {
	const out: Date[] = []
	for (const r of task.reminders ?? []) {
		let at = parseDate(r.reminder)
		if (!at && r.relative_to) {
			const base = parseDate(task[r.relative_to])
			if (base) at = new Date(base.getTime() + (r.relative_period ?? 0) * 1000)
		}
		if (at) out.push(at)
	}
	return out
}

export async function buildCsvBackup(): Promise<Blob> {
	const [projects, tasks] = await Promise.all([
		fetchAll<Project>('/projects', {is_archived: true}),
		fetchAll<Task>('/tasks', {sort_by: ['project_id', 'id'], order_by: ['asc', 'asc']}, 10_000),
	])
	const projectsById = new Map(projects.map(p => [p.id, p]))
	// TickTick ids are just row numbers that parentId refers to.
	const rowIds = new Map(tasks.map((task, i) => [task.id, i + 1]))
	const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
	const orderInList = new Map<number, number>()

	const rows = tasks.map(task => {
		const project = projectsById.get(task.project_id)
		const folder = project?.parent_project_id ? projectsById.get(project.parent_project_id) : undefined

		const s = getSchedule(task)
		let start: Date | null = null
		let due: Date | null = null
		let anchor: Date | null = null
		if (s?.allDay) {
			// All-day dates are local midnight; a range ends on the midnight after its last day.
			start = startOfDay(s.start)
			due = s.end ? addDays(startOfDay(s.end), 1) : start
			anchor = start
		} else if (s) {
			start = s.start
			due = s.end ?? s.start
			anchor = s.start
		}
		const reminders = anchor
			? reminderTimes(task).map(at => tickTickDuration(at.getTime() - anchor.getTime(), !!s?.allDay))
			: []

		let repeat = task.repeat_rule ?? ''
		if (!repeat && s) {
			const spec = parseRepeat(task)
			if (spec && !spec.raw) repeat = buildRule(spec, s.date)
		}

		const parentId = task.related_tasks?.parenttask?.[0]?.id
		const order = (orderInList.get(task.project_id) ?? 0) + 1
		orderInList.set(task.project_id, order)

		const fields = [
			folder?.title ?? '',
			project?.title ?? '',
			task.title,
			'TEXT',
			(task.labels ?? []).map(l => l.title).join(', '),
			// TickTick writes line breaks inside the content as carriage returns.
			(task.description ?? '').replace(/\r?\n/g, '\r'),
			'N',
			tickTickDate(start),
			tickTickDate(due),
			reminders.join('\n'),
			repeat,
			TICKTICK_PRIORITY[level(task.priority)],
			task.done ? (task.wont_do ? '-1' : '2') : '0',
			tickTickDate(parseDate(task.created)),
			task.done ? tickTickDate(parseDate(task.done_at)) : '',
			String(order * 65536),
			timezone,
			s?.allDay ? 'true' : 'false',
			'false',
		].map(quote)
		// TickTick leaves the kanban column cells unquoted when a task has none.
		fields.push('', '')
		fields.push(...[
			'list',
			String(rowIds.get(task.id)),
			parentId && rowIds.has(parentId) ? String(rowIds.get(parentId)) : '',
			'TASK',
		].map(quote))
		return fields.join(',')
	})

	const now = new Date()
	const preamble = [
		quote(`Date: ${now.getUTCFullYear()}-${pad(now.getUTCMonth() + 1)}-${pad(now.getUTCDate())}+0000`),
		quote('Version: 7.2'),
		quote('Status: \n0 Normal\n-1 Abandoned \n2 Completed'),
		TICKTICK_HEADERS.map(quote).join(','),
	]
	// TickTick starts the file with a BOM and ends every line with a bare newline.
	return new Blob(['\ufeff' + [...preamble, ...rows].join('\n') + '\n'], {type: 'text/csv;charset=utf-8'})
}

export function downloadBlob(blob: Blob, filename: string) {
	const url = URL.createObjectURL(blob)
	const a = document.createElement('a')
	a.href = url
	a.download = filename
	a.click()
	setTimeout(() => URL.revokeObjectURL(url), 1000)
}

interface ColumnMapping {
	column_index: number
	column_name: string
	attribute: string
}

interface CsvDetection {
	delimiter: string
	quote_char: string
	date_format: string
	suggested_mapping: ColumnMapping[]
}

export interface MigrationStatus {
	started_at: string
	finished_at: string
	error_kind: string
	error_message: string
}

export class NoTitleColumnError extends Error {}

/** Whether a CSV file is a TickTick backup (its header sits below a few metadata lines). */
async function isTickTickBackup(file: File) {
	const head = await file.slice(0, 4096).text()
	return head.includes('"Folder Name"') && head.includes('"List Name"') && head.includes('"taskId"')
}

/** Starts a background import of a CSV file; the server reports progress via {@link importStatus}. Returns the migrator used. */
export async function startCsvImport(file: File): Promise<string> {
	if (await isTickTickBackup(file)) {
		const body = new FormData()
		body.append('import', file)
		await api('POST', '/migration/ticktick/migrate', {body})
		return 'ticktick'
	}

	const detectBody = new FormData()
	detectBody.append('import', file)
	const detected = await api<CsvDetection>('POST', '/migration/csv/detect', {body: detectBody})
	if (!detected.suggested_mapping.some(m => m.attribute === 'title')) throw new NoTitleColumnError()

	const body = new FormData()
	body.append('import', file)
	body.append('config', JSON.stringify({
		delimiter: detected.delimiter,
		quote_char: detected.quote_char,
		date_format: detected.date_format,
		skip_rows: 0,
		mapping: detected.suggested_mapping,
	}))
	await api('POST', '/migration/csv/migrate', {body})
	return 'csv'
}

export function importStatus(migrator: string) {
	return api<MigrationStatus>('GET', `/migration/${migrator}/status`)
}

/** Polls until the running import finishes; resolves with its final status, or null on timeout. */
export async function waitForImport(migrator: string, timeoutMs = 5 * 60_000): Promise<MigrationStatus | null> {
	const deadline = Date.now() + timeoutMs
	while (Date.now() < deadline) {
		await new Promise(resolve => setTimeout(resolve, 2000))
		const status = await importStatus(migrator)
		if (parseDate(status.finished_at) || status.error_kind) return status
	}
	return null
}
