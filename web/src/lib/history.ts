/**
 * Undo and redo for task changes (Ctrl+Z, Ctrl+Shift+Z). The data store
 * records each change with the calls that revert and repeat it; replaying an
 * entry goes through the same store calls, which are not recorded again.
 */

export interface Entry {
	undo: () => unknown
	redo: () => unknown
	/** Changes with the same key in quick succession merge into one step (typing a title). */
	key?: string
	at?: number
}

const MAX = 100
const MERGE_MS = 1500

const undoStack: Entry[] = []
let redoStack: Entry[] = []
let replaying = false

export function record(entry: Entry) {
	if (replaying) return
	const top = undoStack[undoStack.length - 1]
	const now = Date.now()
	if (entry.key && top?.key === entry.key && top.at && now - top.at < MERGE_MS) {
		// Keep the oldest undo, take the newest redo.
		top.redo = entry.redo
		top.at = now
	} else {
		undoStack.push({...entry, at: now})
		if (undoStack.length > MAX) undoStack.shift()
	}
	redoStack = []
}

/** Runs `fn` without recording what it changes. Store calls record synchronously, before their first await. */
function replay(fn: () => unknown): Promise<unknown> {
	replaying = true
	try {
		return Promise.resolve(fn())
	} finally {
		replaying = false
	}
}

export async function undo(): Promise<boolean> {
	const entry = undoStack.pop()
	if (!entry) return false
	redoStack.push(entry)
	await replay(entry.undo)
	return true
}

export async function redo(): Promise<boolean> {
	const entry = redoStack.pop()
	if (!entry) return false
	undoStack.push({...entry, key: undefined})
	await replay(entry.redo)
	return true
}

export function canUndo() {
	return undoStack.length > 0
}

export function canRedo() {
	return redoStack.length > 0
}

export function clearHistory() {
	undoStack.length = 0
	redoStack = []
}
