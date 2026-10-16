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
	hex_color?: string
	is_favorite?: boolean
	created?: string
	updated?: string
	identifier?: string
	index?: number
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
