import {onBeforeUnmount, onMounted} from 'vue'
import {prefs} from './prefs'
import {t, type MessageKey} from './i18n'

/**
 * Keyboard shortcuts: their default keys, the user's rebindings, and one
 * window-level dispatcher that runs whichever handler the views currently
 * on screen registered for a binding.
 *
 * A binding is a list of alternatives; each alternative is a sequence of key
 * tokens separated by spaces ("g c": G, then C). A token is a key with its
 * modifiers, lowercase: "ctrl+shift+z", "alt+1", "enter", "?". Tab followed
 * by a key ("tab m") is shown as Tab+M, as in TickTick.
 */

export const SHORTCUTS = {
	// general
	save: ['ctrl+s'],
	sync: ['.'],
	undo: ['ctrl+z'],
	redo: ['ctrl+shift+z'],
	print: ['ctrl+p'],
	commandMenu: ['ctrl+k'],
	shortcuts: ['?'],
	// task list
	addTask: ['n'],
	addBelow: ['enter'],
	addSubtask: ['shift+enter'],
	toggleGroups: [],
	toggleSubtasks: ['tab e'],
	listView: ['v l'],
	kanbanView: ['v k'],
	timelineView: ['v t'],
	// the task under the cursor, or the open one
	complete: ['tab m'],
	pin: ['tab p'],
	delete: ['ctrl+delete', 'ctrl+backspace'],
	setDate: ['tab d'],
	clearTime: ['tab 0'],
	setToday: ['tab 1'],
	setTomorrow: ['tab 2'],
	setNextWeek: ['tab 3'],
	priority0: ['alt+0'],
	priority1: ['alt+1'],
	priority2: ['alt+2'],
	priority3: ['alt+3'],
	// navigation
	goCalendar: ['g c'],
	goMatrix: ['g m'],
	goAi: [],
	splitView: [],
	goSearch: ['/'],
	goSettings: ['g s'],
	goAll: ['g a'],
	goToday: ['g t'],
	goTomorrow: ['g r'],
	goNext7: ['g w'],
	goAssigned: [],
	goInbox: ['g i'],
	goCompleted: [],
	goWontDo: [],
	goSummary: ['g b'],
	goTrash: ['g g'],
	goHabits: ['g h'],
	goFocus: ['g f'],
	goCountdowns: ['g d'],
	goPeople: ['g p'],
	// focus timer: start, pause or resume from anywhere
	toggleFocus: ['alt+shift+f'],
	// calendar
	dayView: ['d', '1'],
	weekView: ['w', '2'],
	monthView: ['m', '3'],
	yearView: ['y', '4'],
	agendaView: ['a', '5'],
	backToToday: ['t'],
	goToDate: [],
} satisfies Record<string, string[]>

export type ShortcutId = keyof typeof SHORTCUTS

/** Shortcuts that also fire while typing in a text field. */
const IN_INPUTS = new Set<ShortcutId>(['save', 'print', 'commandMenu'])

export function binding(id: ShortcutId): string[] {
	return prefs.shortcuts[id] ?? SHORTCUTS[id]
}

export function isDefault(id: ShortcutId): boolean {
	return !prefs.shortcuts[id]
}

/** Binds `id` to `seq` alone; another shortcut using `seq` loses it. Returns that shortcut. */
export function rebind(id: ShortcutId, seq: string | null): ShortcutId | null {
	let taken: ShortcutId | null = null
	if (seq) {
		for (const other of Object.keys(SHORTCUTS) as ShortcutId[]) {
			if (other !== id && binding(other).includes(seq)) {
				taken = other
				prefs.shortcuts[other] = binding(other).filter(s => s !== seq)
			}
		}
	}
	prefs.shortcuts[id] = seq ? [seq] : []
	return taken
}

export function resetShortcuts() {
	prefs.shortcuts = {}
}

/** Rows of Settings > Shortcuts. A row with `keys` shows fixed text instead of a binding. */
export interface ShortcutRow {
	label: MessageKey
	id?: ShortcutId
	keys?: string
}

export const SHORTCUT_SECTIONS: {title: MessageKey, desc?: MessageKey, rows: ShortcutRow[]}[] = [
	{title: 'general', rows: [
		{label: 'save', id: 'save'},
		{label: 'sync', id: 'sync'},
		{label: 'cancel', keys: 'Esc'},
		{label: 'undo', id: 'undo'},
		{label: 'scRedo', id: 'redo'},
		{label: 'scPrint', id: 'print'},
		{label: 'scCommandMenu', id: 'commandMenu'},
		{label: 'shortcutsShort', id: 'shortcuts'},
	]},
	{title: 'scTask', rows: [
		{label: 'addTask', id: 'addTask'},
		{label: 'scAddBelow', id: 'addBelow'},
		{label: 'scAddSubtask', id: 'addSubtask'},
		{label: 'scToggleGroups', id: 'toggleGroups'},
		{label: 'scToggleSubtasks', id: 'toggleSubtasks'},
		{label: 'scListView', id: 'listView'},
		{label: 'scKanbanView', id: 'kanbanView'},
		{label: 'scTimelineView', id: 'timelineView'},
	]},
	{title: 'scQuickAdd', desc: 'scQuickAddDesc', rows: [
		{label: 'scSetDueDate', keys: '*'},
		{label: 'scSetPriority', keys: '!'},
		{label: 'scSetTag', keys: '#'},
		{label: 'scMoveToList', keys: '~ / ^'},
		{label: 'scAssign', keys: '@'},
	]},
	{title: 'scEditTasks', desc: 'scEditTasksDesc', rows: [
		{label: 'scComplete', id: 'complete'},
		{label: 'scPin', id: 'pin'},
		{label: 'scDelete', id: 'delete'},
		{label: 'scSetDate', id: 'setDate'},
		{label: 'scClearTime', id: 'clearTime'},
		{label: 'scSetToday', id: 'setToday'},
		{label: 'scSetTomorrow', id: 'setTomorrow'},
		{label: 'scSetNextWeek', id: 'setNextWeek'},
		{label: 'scSetPriorityNone', id: 'priority0'},
		{label: 'scSetPriorityLow', id: 'priority1'},
		{label: 'scSetPriorityMedium', id: 'priority2'},
		{label: 'scSetPriorityHigh', id: 'priority3'},
	]},
	{title: 'scNavigation', desc: 'scNavigationDesc', rows: [
		{label: 'scGoCalendar', id: 'goCalendar'},
		{label: 'scGoMatrix', id: 'goMatrix'},
		{label: 'scGoAi', id: 'goAi'},
		{label: 'scSplitView', id: 'splitView'},
		{label: 'scGoSearch', id: 'goSearch'},
		{label: 'scGoSettings', id: 'goSettings'},
		{label: 'scGoAll', id: 'goAll'},
		{label: 'scGoToday', id: 'goToday'},
		{label: 'scGoTomorrow', id: 'goTomorrow'},
		{label: 'scGoNext7', id: 'goNext7'},
		{label: 'scGoAssigned', id: 'goAssigned'},
		{label: 'scGoInbox', id: 'goInbox'},
		{label: 'scGoCompleted', id: 'goCompleted'},
		{label: 'scGoWontDo', id: 'goWontDo'},
		{label: 'scGoSummary', id: 'goSummary'},
		{label: 'scGoTrash', id: 'goTrash'},
		{label: 'scGoHabits', id: 'goHabits'},
		{label: 'scGoFocus', id: 'goFocus'},
		{label: 'scGoCountdowns', id: 'goCountdowns'},
		{label: 'scGoPeople', id: 'goPeople'},
	]},
	{title: 'featPomo', rows: [
		{label: 'scToggleFocus', id: 'toggleFocus'},
	]},
	{title: 'featCalendar', rows: [
		{label: 'scDayView', id: 'dayView'},
		{label: 'scWeekView', id: 'weekView'},
		{label: 'scMonthView', id: 'monthView'},
		{label: 'scYearView', id: 'yearView'},
		{label: 'scAgenda', id: 'agendaView'},
		{label: 'scBackToToday', id: 'backToToday'},
		{label: 'scGoToDate', id: 'goToDate'},
	]},
]

// ---- keys ----

const NAMED: Record<string, string> = {
	' ': 'space', Escape: 'esc', Enter: 'enter', Tab: 'tab', Delete: 'delete', Backspace: 'backspace',
	ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right',
	Home: 'home', End: 'end', PageUp: 'pageup', PageDown: 'pagedown', Insert: 'insert',
}

/** The token for a key press, or null for a lone modifier. */
export function keyToken(e: Pick<KeyboardEvent, 'key' | 'code' | 'ctrlKey' | 'metaKey' | 'altKey' | 'shiftKey'>): string | null {
	if (['Control', 'Shift', 'Alt', 'Meta', 'AltGraph', 'CapsLock', 'Dead', 'Unidentified'].includes(e.key)) return null
	let key = e.key
	// With modifiers held, e.key turns into other characters (Alt+1 is ¡ on a Mac).
	if (e.ctrlKey || e.metaKey || e.altKey) {
		if (/^Key[A-Z]$/.test(e.code)) key = e.code.slice(3)
		else if (/^Digit\d$/.test(e.code)) key = e.code.slice(5)
	}
	key = NAMED[key] ?? key.toLowerCase()
	const mods: string[] = []
	if (e.ctrlKey || e.metaKey) mods.push('ctrl')
	if (e.altKey) mods.push('alt')
	// Shift already shows in symbols ("?"), so it only counts for letters, digits and named keys.
	if (e.shiftKey && (key.length > 1 || /[a-z0-9]/.test(key))) mods.push('shift')
	return [...mods, key].join('+')
}

const KEY_NAMES: Record<string, string> = {
	ctrl: 'Ctrl', alt: 'Alt', shift: 'Shift', esc: 'Esc', enter: 'Enter', tab: 'Tab', delete: 'Del',
	backspace: 'Backspace', space: 'Space', up: '↑', down: '↓', left: '←', right: '→',
	home: 'Home', end: 'End', pageup: 'PgUp', pagedown: 'PgDn', insert: 'Ins',
}

function formatToken(token: string): string {
	// "ctrl++" is Ctrl and the plus key.
	const parts = token.endsWith('++') ? [...token.slice(0, -2).split('+'), '+'] : token.split('+')
	return parts.map(p => KEY_NAMES[p] ?? p.toUpperCase()).join('+')
}

export function formatSequence(seq: string): string {
	const steps = seq.split(' ').map(formatToken)
	if (steps.length === 2 && steps[0] === 'Tab') return `Tab+${steps[1]}`
	return steps.join(` ${t('scThen')} `)
}

/** How a shortcut's keys read in the UI, or '' when it has none. */
export function formatBinding(id: ShortcutId): string {
	return binding(id).map(formatSequence).join(' / ')
}

// ---- handlers ----

export type Handler = () => boolean | void
export interface HandlerSpec {
	run: Handler
	/** Whether the shortcut applies right now; when not, its keys pass through. */
	when?: () => boolean
}
type Layer = Partial<Record<ShortcutId, Handler | HandlerSpec>>

const layers: Layer[] = []

/** Registers handlers while a component is mounted. Later layers win; a handler returning false passes on. */
export function useShortcuts(layer: Layer) {
	onMounted(() => layers.push(layer))
	onBeforeUnmount(() => {
		const i = layers.indexOf(layer)
		if (i >= 0) layers.splice(i, 1)
	})
}

/** For tests and for code outside components. Returns the remover. */
export function addShortcuts(layer: Layer): () => void {
	layers.push(layer)
	return () => {
		const i = layers.indexOf(layer)
		if (i >= 0) layers.splice(i, 1)
	}
}

function spec(h: Handler | HandlerSpec): HandlerSpec {
	return typeof h === 'function' ? {run: h} : h
}

function available(id: ShortcutId): HandlerSpec[] {
	const out: HandlerSpec[] = []
	for (let i = layers.length - 1; i >= 0; i--) {
		const h = layers[i][id]
		if (!h) continue
		const s = spec(h)
		if (!s.when || s.when()) out.push(s)
	}
	return out
}

/** Whether some view on screen handles `id` right now. */
export function canRun(id: ShortcutId): boolean {
	return available(id).length > 0
}

/** Runs a shortcut's action as if its keys were pressed (the command menu). */
export function runShortcut(id: ShortcutId): boolean {
	return run(id)
}

function run(id: ShortcutId): boolean {
	for (const s of available(id)) {
		if (s.run() !== false) return true
	}
	return false
}

let pending: string[] = []
let pendingTimer: number | undefined

function clearPending() {
	pending = []
	clearTimeout(pendingTimer)
}

/** Whether a key press happens in a text field, where plain keys type text. */
function typing(e: KeyboardEvent): boolean {
	const el = e.target as HTMLElement | null
	return !!el?.closest?.('input, textarea, select, [contenteditable="true"], [contenteditable=""]')
}

/**
 * Handles a key press; returns whether a shortcut used it. `blocked` turns
 * shortcuts off while a dialog has the keyboard.
 */
export function dispatchKey(e: KeyboardEvent, blocked = false): boolean {
	if (e.isComposing) return false
	const token = keyToken(e)
	if (!token) return false
	if (blocked) {
		clearPending()
		return false
	}
	// Enter and Space on a focused button press the button.
	if ((token === 'enter' || token === 'space') && (e.target as HTMLElement | null)?.closest?.('button, a, [role="button"]')) return false
	const inField = typing(e)
	const ids = (Object.keys(SHORTCUTS) as ShortcutId[]).filter(id => !inField || IN_INPUTS.has(id))
	const attempt = (seq: string[]): 'done' | 'prefix' | 'none' => {
		const joined = seq.join(' ')
		let prefix = false
		for (const id of ids) {
			for (const b of binding(id)) {
				if (b === joined) {
					if (run(id)) return 'done'
				} else if (b.startsWith(joined + ' ') && available(id).length) {
					prefix = true
				}
			}
		}
		return prefix ? 'prefix' : 'none'
	}
	let result = attempt([...pending, token])
	if (result === 'none' && pending.length) {
		clearPending()
		result = attempt([token])
	}
	if (result === 'done') {
		clearPending()
		e.preventDefault()
		return true
	}
	if (result === 'prefix') {
		pending = [...pending, token]
		clearTimeout(pendingTimer)
		pendingTimer = window.setTimeout(clearPending, 1500)
		e.preventDefault()
		return true
	}
	clearPending()
	return false
}
