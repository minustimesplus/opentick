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
	/** Closed without being completed. Always comes with done = true. */
	wont_do?: boolean
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
	/** Invitations by username wait for an answer instead of joining right away. */
	confirm_project_invites?: boolean
	notify_shared_task_done?: boolean
	notify_shared_task_created?: boolean
	notify_shared_task_deleted?: boolean
	/** Free-form client settings; `views` holds the prefs synced between devices (lib/viewSync.ts). */
	frontend_settings?: Record<string, unknown> | null
}

export type Permission = 0 | 1 | 2

/**
 * An invitation to a list. kind user: an existing user by username; email: an
 * address, joined through the mailed link; link: a join link for anyone.
 */
export interface ProjectInvitation {
	id: number
	project_id: number
	kind: 'user' | 'email' | 'link'
	permission: Permission
	email?: string
	uses: number
	expires_at?: string | null
	created?: string
	/** accepted: the user joined right away, nothing is pending. */
	status: 'pending' | 'accepted'
	invitee?: User
	inviter?: User
	/** Set on invitations the current user received. */
	project_title?: string
	/** Join links always; email invitations only in the response that created them. */
	invite_url?: string
}

/** What an invitation link leads to, readable without an account. */
export interface InvitationPreview {
	project_title: string
	inviter_name: string
	permission: Permission
	kind: 'email' | 'link'
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

export interface StatsBucket {
	/** First day of the period, YYYY-MM-DD. */
	date: string
	count: number
}

export interface StatsProject {
	project_id: number
	title: string
	hex_color: string
	count: number
}

export interface UserStatistics {
	timezone: string
	total_completed: number
	completed_today: number
	open_overdue: number
	daily: StatsBucket[] | null
	weekly: StatsBucket[] | null
	monthly: StatsBucket[] | null
	completion_rate: {
		on_time: number
		late: number
		no_due_date: number
		/** on_time / (on_time + late), 0–1. */
		rate: number
	}
	projects: StatsProject[] | null
	score: {
		score: number
		level: number
		level_score: number
		next_level_score: number
		completion_points: number
		on_time_points: number
		late_penalty: number
		overdue_penalty: number
	}
}

export type FocusSessionType = 'pomo' | 'stopwatch'

export interface FocusSession {
	id: number
	/** 0 when the session is not linked to a task. */
	task_id: number
	type: FocusSessionType
	start_time: string
	end_time: string
	/** Focused seconds, excluding pauses. */
	duration: number
	note?: string
	created?: string
	updated?: string
}

export interface FocusDay {
	/** YYYY-MM-DD in the requested timezone. */
	date: string
	pomos: number
	focus_minutes: number
}

export interface FocusStats {
	today_pomos: number
	today_focus_minutes: number
	total_pomos: number
	total_focus_minutes: number
	/** Oldest first, ending with today. */
	history: FocusDay[]
}

export type AgentKind = 'claude' | 'instinct'

/** How the user's tasks reach their AI agents (Settings > AI features). */
export interface AgentConfig {
	claude_fire_url: string
	/** Write-only: send to change, never returned. */
	claude_token?: string
	claude_token_set: boolean
	instinct_email: string
	/** Empty means default_system_prompt. */
	system_prompt: string
	default_system_prompt: string
	mailer_enabled: boolean
	/** Private address agents email task commands to; empty until created or while the server reads no mailbox. */
	inbound_address: string
	/** Whether the server reads a mailbox, which the inbound address needs. */
	inbound_enabled: boolean
	/** Extra senders allowed to use the inbound address, comma-separated; "@domain" allows a whole domain. */
	inbound_senders: string
	/** List emailed tasks without a list go to; 0 for the default list. */
	inbound_project_id: number
}

/** AI settings of one list. */
export interface ProjectAgentSettings {
	project_id: number
	/** Context sent with every task of the list, e.g. the language to write in. */
	ai_context: string
	/** Every new task in the list goes to this agent; empty for none. */
	agent: AgentKind | ''
}

export interface AgentDispatch {
	id: number
	task_id: number
	agent: AgentKind
	status: 'sent' | 'failed'
	extra_prompt: string
	/** Claude only: the cloud session working on the task. */
	session_url: string
	error: string
	/** Sent because the task's list belongs to the agent. */
	automatic: boolean
	/** Only on the response of a new dispatch. */
	prompt?: string
	created: string
}

export interface ContactExperience {
	id: number
	contact_id: number
	company: string
	title: string
	location: string
	start_date: string
	end_date: string
	description: string
	sort_order: number
}

export interface ContactEducation {
	id: number
	contact_id: number
	school: string
	degree: string
	field_of_study: string
	start_year: string
	end_year: string
	sort_order: number
}

export interface ContactNote {
	id: number
	contact_id: number
	body: string
	created: string
	updated: string
}

export type ContactSource = 'manual' | 'csv' | 'linkedin' | 'brightdata'

/** The scalar fields of a contact a user edits by hand. */
export type ContactField =
	| 'full_name'
	| 'headline'
	| 'company'
	| 'title'
	| 'location'
	| 'email'
	| 'phone'
	| 'linkedin_url'
	| 'birthday'
	| 'about'

export interface Contact extends Record<ContactField, string> {
	id: number
	name: string
	first_name: string
	last_name: string
	source: ContactSource
	locked_fields: string[]
	has_photo: boolean
	experiences: ContactExperience[]
	educations: ContactEducation[]
	notes: ContactNote[]
	tag_ids: number[]
	group_ids: number[]
	tags: string[]
	groups: string[]
	created: string
	updated: string
}

/** A contact as /contacts/overview lists it. */
export interface ContactListItem {
	id: number
	name: string
	headline: string
	company: string
	title: string
	location: string
	email: string
	phone: string
	linkedin_url: string
	birthday: string
	source: ContactSource
	has_photo: boolean
	tag_ids: number[]
	group_ids: number[]
	schools: string[]
	notes_count: number
	has_experience: boolean
	has_education: boolean
	search_text: string
	created: string
	updated: string
}

export interface ContactGroup {
	id: number
	name: string
	hex_color: string
	description: string
	members: number
}

export interface ContactTag {
	id: number
	name: string
	hex_color: string
	contacts: number
}

export interface ContactOverview {
	contacts: ContactListItem[] | null
	tags: ContactTag[] | null
	groups: ContactGroup[] | null
}

export interface ContactMembershipResult {
	dry_run: boolean
	target: {
		id: number
		name: string
		created: boolean
	}
	changed: number
	unchanged: number
	contacts: {id: number, name: string}[]
	unknown_ids: number[]
}

export interface ContactImportResult {
	created: number
	updated: number
	skipped: number
}
