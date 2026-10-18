// CSV backup: export every task client-side, import through the server's
// generic CSV migrator. The headers are chosen so the migrator's column
// detection maps each one back to the right task attribute.

import {api, fetchAll} from '@/api/client'
import type {Project, Task} from '@/api/types'
import {parseDate} from './dates'

const HEADERS = ['Title', 'Description', 'Due Date', 'Start Date', 'End Date', 'Done', 'Priority', 'Labels', 'Project', 'Reminder']

/** RFC 3339 without milliseconds, which the migrator parses with the offset intact. */
function isoDate(value?: string | null) {
	const d = parseDate(value)
	return d ? d.toISOString().replace(/\.\d{3}Z$/, 'Z') : ''
}

function csvField(value: string) {
	return /[",\r\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value
}

export async function buildCsvBackup(): Promise<Blob> {
	const [projects, tasks] = await Promise.all([
		fetchAll<Project>('/projects', {is_archived: true}),
		fetchAll<Task>('/tasks', {sort_by: ['project_id', 'id'], order_by: ['asc', 'asc']}, 10_000),
	])
	const projectTitles = new Map(projects.map(p => [p.id, p.title]))
	const rows = tasks.map(task => [
		task.title,
		task.description ?? '',
		isoDate(task.due_date),
		isoDate(task.start_date),
		isoDate(task.end_date),
		task.done ? 'true' : 'false',
		String(task.priority ?? 0),
		(task.labels ?? []).map(l => l.title).join(', '),
		projectTitles.get(task.project_id) ?? '',
		isoDate(task.reminders?.find(r => parseDate(r.reminder))?.reminder),
	])
	const lines = [HEADERS, ...rows].map(row => row.map(csvField).join(','))
	// The BOM makes Excel read the file as UTF-8; the importer strips it.
	return new Blob(['﻿' + lines.join('\r\n') + '\r\n'], {type: 'text/csv;charset=utf-8'})
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

/** Starts a background import of a CSV file; the server reports progress via {@link csvImportStatus}. */
export async function startCsvImport(file: File) {
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
}

export function csvImportStatus() {
	return api<MigrationStatus>('GET', '/migration/csv/status')
}

/** Polls until the running import finishes; resolves with its final status, or null on timeout. */
export async function waitForCsvImport(timeoutMs = 5 * 60_000): Promise<MigrationStatus | null> {
	const deadline = Date.now() + timeoutMs
	while (Date.now() < deadline) {
		await new Promise(resolve => setTimeout(resolve, 2000))
		const status = await csvImportStatus()
		if (parseDate(status.finished_at) || status.error_kind) return status
	}
	return null
}
