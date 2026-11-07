// State of the in-browser backend, kept as one JSON document in localStorage.

import type {Countdown, FocusSession, Habit, Label, Project, Task, TaskComment, User} from '../types'
import {seed} from './seed'

const DB_KEY = 'tt.local.db'
const VERSION = 1

/** A task as stored: embeds are rebuilt on every read. */
export type StoredTask = Omit<Task, 'labels' | 'related_tasks' | 'comment_count' | 'position' | 'created_by'> & {
	label_ids: number[]
}

export interface StoredBucket {
	id: number
	project_view_id: number
	title: string
	position: number
}

export interface StoredFilter {
	id: number
	title: string
	filter: string
	created: string
	updated: string
}

export type StoredHabit = Omit<Habit, 'today_value' | 'current_streak' | 'recent_checkins'>

export interface Relation {
	/** The parent, for subtask relations. */
	task_id: number
	other_task_id: number
	kind: 'subtask'
}

export interface DB {
	version: number
	nextId: number
	user: User & {settings: {default_project_id: number}}
	projects: Project[]
	tasks: StoredTask[]
	labels: Label[]
	relations: Relation[]
	comments: (TaskComment & {task_id: number})[]
	/** view id → task id → position */
	positions: Record<number, Record<number, number>>
	/** view id → task id → bucket id */
	bucketOf: Record<number, Record<number, number>>
	buckets: StoredBucket[]
	/** kanban view id → bucket ids */
	viewBuckets: Record<number, {default_bucket_id: number, done_bucket_id: number}>
	filters: StoredFilter[]
	habits: StoredHabit[]
	/** habit id → YYYY-MM-DD → value */
	checkins: Record<number, Record<string, number>>
	countdowns: Countdown[]
	focus: FocusSession[]
}

let db: DB | null = null

export function getDB(): DB {
	if (db) return db
	try {
		const raw = localStorage.getItem(DB_KEY)
		const parsed = raw ? JSON.parse(raw) as DB : null
		if (parsed?.version === VERSION) db = parsed
	} catch {
		// Corrupt or blocked storage: start over from the examples.
	}
	db ??= seed()
	save()
	return db
}

export function save() {
	if (!db) return
	try {
		localStorage.setItem(DB_KEY, JSON.stringify(db))
	} catch {
		// Full or blocked storage: changes last for this page load.
	}
}

/** Replaces everything with the example data. */
export function resetDB() {
	db = seed()
	save()
}

export function nextId(): number {
	return getDB().nextId++
}

export function now(): string {
	return new Date().toISOString()
}
