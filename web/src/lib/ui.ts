import {reactive} from 'vue'

export const ui = reactive({
	/** Task shown in the floating detail drawer (matrix, calendar, search). */
	drawerTaskId: null as number | null,
	/** Increments to ask the active view to focus its quick-add input. */
	focusQuickAdd: 0,
	/** Right-click task menu. */
	ctx: null as {taskId: number, x: number, y: number} | null,
	/** Task being dragged from a task row, so drop targets can react during dragover. */
	dragTaskId: null as number | null,
	/** Section shown in the settings dialog, null while it is closed. */
	settings: null as SettingsSection | null,
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
