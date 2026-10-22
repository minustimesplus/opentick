export interface User {
	id: number
	username: string
	name?: string
	email?: string
}

export interface Label {
	id: number
	title: string
	hex_color?: string
	description?: string
}

export interface Reminder {
	reminder?: string | null
	relative_period?: number
	relative_to?: '' | 'due_date' | 'start_date' | 'end_date'
}

export interface Task {
	id: number
	title: string
	description?: string
	done: boolean
	done_at?: string | null
	due_date?: string | null
	start_date?: string | null
	end_date?: string | null
	priority?: number
	project_id: number
	labels?: Label[] | null
	reminders?: Reminder[] | null
	repeat_after?: number
	repeat_mode?: number
	/** RFC 5545 RRULE value; takes precedence over repeat_after. */
	repeat_rule?: string
	hex_color?: string
	is_favorite?: boolean
	created?: string
	updated?: string
	identifier?: string
	index?: number
	/** Snapshots of related tasks, keyed by relation kind. */
	related_tasks?: RelatedTasks | null
	assignees?: User[] | null
	attachments?: Attachment[] | null
	/** Only set when the list was requested with `expand=comment_count`. */
	comment_count?: number
	/** Position in the view the task was fetched through; 0 elsewhere. */
	position?: number
	bucket_id?: number
	created_by?: User | null
	/** Only set on tasks in the trash. */
	deleted_at?: string | null
}

export type RelationKind = 'subtask' | 'parenttask' | 'related' | 'duplicateof' | 'duplicates' | 'blocking' | 'blocked' | 'precedes' | 'follows' | 'copiedfrom' | 'copiedto'

export type RelatedTasks = Partial<Record<RelationKind, Task[] | null>>

export interface FileInfo {
	id: number
	name: string
	mime: string
	size: number
	created?: string
}

export interface Attachment {
	id: number
	task_id: number
	created_by?: User | null
	file: FileInfo
	created?: string
}

export interface TaskComment {
	id: number
	comment: string
	author: User
	created: string
	updated: string
}

export interface ProjectView {
	id: number
	project_id: number
	title: string
	view_kind: string | number
	position?: number
}

export interface Project {
	id: number
	title: string
	description?: string
	hex_color?: string
	parent_project_id?: number | null
	position?: number
	is_archived?: boolean
	is_favorite?: boolean
	views?: ProjectView[] | null
	owner?: User | null
}

/** A user a list is shared with directly. 0 = read only, 1 = read & write, 2 = admin. */
export interface ProjectMember extends User {
	permission: 0 | 1 | 2
}

export interface UserSettings {
	default_project_id?: number
	week_start?: number
	timezone?: string
	name?: string
	language?: string
}

export interface CurrentUser extends User {
	settings?: UserSettings
}

export interface AppNotification {
	id: number
	name: string
	notification: Record<string, unknown>
	read_at?: string | null
	created?: string
}

export interface CalendarSubscription {
	id: number
	title: string
	url: string
	hex_color?: string
	created?: string
	updated?: string
}

export type HabitFrequency = 'daily' | 'weekdays'

export interface HabitCheckin {
	habit_id: number
	/** YYYY-MM-DD */
	date: string
	value: number
}

export interface Habit {
	id: number
	title: string
	icon: string
	hex_color?: string
	frequency: HabitFrequency
	/** Bitmask of due days: bit 0 is Sunday, bit 6 Saturday. */
	weekdays: number
	goal: number
	unit: string
	archived: boolean
	today_value: number
	current_streak: number
	/** The last 7 days, today included. */
	recent_checkins: HabitCheckin[] | null
	created?: string
	updated?: string
}

export type CountdownType = 'countdown' | 'anniversary' | 'birthday'

export interface Countdown {
	id: number
	title: string
	/** Midnight UTC of the counted date; only its UTC calendar date matters. */
	date: string
	type: CountdownType
	repeat_yearly: boolean
	icon?: string
	hex_color?: string
	created?: string
	updated?: string
}

export interface HabitStatsDay {
	date: string
	value: number
	due: boolean
	completed: boolean
}

export interface HabitStats {
	habit_id: number
	today: string
	current_streak: number
	longest_streak: number
	completed_days: number
	due_days: number
	completion_rate: number
	total_checkins: number
	total_value: number
	month: string
	days: HabitStatsDay[]
}

/** One occurrence of an event from a subscribed calendar, as the API returns it. */
export interface ExternalEvent {
	id: string
	uid: string
	title: string
	description: string
	location: string
	url: string
	start: string
	/** Exclusive. For all-day events: midnight after the last day. */
	end: string
	all_day: boolean
	recurring: boolean
}

export interface CalendarSubscriptionEvents {
	subscription_id: number
	calendar_name: string
	events: ExternalEvent[] | null
}

export type ApiPermissions = Record<string, string[]>

export interface ApiToken {
	id: number
	title: string
	token?: string
	permissions: ApiPermissions
	expires_at: string
	created?: string
}

export interface McpInfo {
	endpoint: string
	/** Token permission groups usable by MCP tools, keyed group → permission → route. */
	routes: Record<string, Record<string, unknown>>
	presets: {
		/** Wildcard group `*` expands against routes. */
		read_only: Record<string, string[]>
		typed: ApiPermissions
		full: Record<string, string>
	}
}
