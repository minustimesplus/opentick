import {reactive} from 'vue'
import type {AgentKind} from '@/api/types'

export const ui = reactive({
	/** Task shown in the floating detail drawer (matrix, calendar, search). */
	drawerTaskId: null as number | null,
	/** Increments to ask the active view to focus its quick-add input. */
	focusQuickAdd: 0,
	/** Right-click task menu; `date` opens straight on the date picker (Tab+D). */
	ctx: null as {taskId: number, x: number, y: number, date?: boolean} | null,
	/** Task row under the mouse, which the task shortcuts act on first. */
	hoverTaskId: null as number | null,
	/** Task open in the tasks view's detail pane. */
	selectedTaskId: null as number | null,
	/** Asks the detail pane of this task to open its subtask input (Shift+Enter). */
	subtaskAdd: null as {taskId: number, n: number} | null,
	/** The command menu (Ctrl+K). */
	commandMenu: false,
	/** Increments to sync now, with the nav rail's spinner. */
	sync: 0,
	/** Task being dragged from a task row, so drop targets can react during dragover. */
	dragTaskId: null as number | null,
	/** Section shown in the settings dialog, null while it is closed. */
	settings: null as SettingsSection | null,
	/** Increments to open the notifications panel on the invitations. */
	invitations: 0,
	/** Bumped when an invitation link was opened, so App offers to join. */
	inviteLink: 0,
	/** Task being handed to an AI agent, with the agent picked first. */
	agentSend: null as {taskId: number, agent: AgentKind} | null,
})

/** Where a dragged task lands relative to a task row. */
export type DropZone = 'before' | 'after' | 'inside'

export type SettingsSection =
	| 'account' | 'premium'
	| 'features' | 'smartList' | 'notifications' | 'dateTime' | 'appearance' | 'aiFeatures' | 'more'
	| 'integrations' | 'collaborate' | 'shortcuts'
	| 'about'

export function openSettings(section: SettingsSection = 'dateTime') {
	ui.settings = section
}

export function openTaskMenu(taskId: number, e: MouseEvent) {
	e.preventDefault()
	ui.ctx = {taskId, x: e.clientX, y: e.clientY}
}

export function openTaskDrawer(id: number) {
	ui.drawerTaskId = id
}

export function closeTaskDrawer() {
	ui.drawerTaskId = null
}

export function openAgentSend(taskId: number, agent: AgentKind) {
	ui.agentSend = {taskId, agent}
}
