// View settings shared by every device of an account (the Eisenhower matrix
// setup, list sorting, …). They live in the user's server settings under
// frontend_settings.views and are exchanged as JSON merge patches (RFC 7396),
// so devices only send what they changed.
//
// Each device remembers the last server state it saw (the snapshot). A local
// value that differs from the snapshot is a local edit and gets pushed; any
// other value follows the server. A key the server doesn't have yet is pushed
// as is, so the first device to sync (normally the desktop app) seeds it.
// Android does the same in data/ViewSync.kt.

/** Pref keys stored on the server. Everything else stays per device. */
export const SYNCED_KEYS = [
	'matrixLists',
	'matrixShowDone',
	'matrixView',
	'matrixOrder',
	'listSort',
	'listShowCompleted',
	'listView',
	'timelineScale',
	'smartLists',
	'features',
	'calendarHidden',
	'calendarHiddenSubs',
	'calendarShowWeekends',
] as const

export type SyncedKey = typeof SYNCED_KEYS[number]

/** frontend_settings.views: the synced values plus `rev`, bumped on every push. */
export type Views = Record<string, unknown> & {rev?: number}

type Json = Record<string, unknown>

function isObject(v: unknown): v is Json {
	return typeof v === 'object' && v !== null && !Array.isArray(v)
}

export function deepEqual(a: unknown, b: unknown): boolean {
	if (a === b) return true
	if (Array.isArray(a) && Array.isArray(b)) return a.length === b.length && a.every((x, i) => deepEqual(x, b[i]))
	if (isObject(a) && isObject(b)) {
		const ka = Object.keys(a)
		return ka.length === Object.keys(b).length && ka.every(k => k in b && deepEqual(a[k], b[k]))
	}
	return false
}

/** RFC 7396: objects merge key by key, null removes a key, anything else replaces. */
export function applyMergePatch(target: unknown, patch: unknown): unknown {
	if (!isObject(patch)) return patch
	const out: Json = isObject(target) ? {...target} : {}
	for (const [k, v] of Object.entries(patch)) {
		if (v === null) delete out[k]
		else out[k] = applyMergePatch(out[k], v)
	}
	return out
}

/** The merge patch that turns `from` into `to`. */
export function mergePatchDiff(from: unknown, to: unknown): unknown {
	if (!isObject(from) || !isObject(to)) return to
	const patch: Json = {}
	for (const k of Object.keys(from)) {
		if (!(k in to)) patch[k] = null
	}
	for (const [k, v] of Object.entries(to)) {
		if (!deepEqual(from[k], v)) patch[k] = mergePatchDiff(from[k], v)
	}
	return patch
}

export interface Reconciled {
	/** Values the device should show now. */
	values: Partial<Record<SyncedKey, unknown>>
	/** Merge patch for frontend_settings.views, or null if the server is up to date. */
	patch: Views | null
	/** The server state once the patch went through; the next snapshot. */
	server: Views
}

/**
 * Three-way merge of this device's values with the server's. `remote` is what
 * the server returned (or the snapshot, when only local values changed).
 */
export function reconcile(local: Partial<Record<SyncedKey, unknown>>, snapshot: Views | null, remote: Views, now = Date.now()): Reconciled {
	// A response fetched before our last push went through: ignore what it says.
	const base = snapshot && (remote.rev ?? 0) < (snapshot.rev ?? 0) ? snapshot : remote
	const patch: Views = {}
	for (const key of SYNCED_KEYS) {
		if (!(key in local) || local[key] === undefined) continue
		if (!(key in base)) {
			patch[key] = local[key]
		} else if (snapshot && key in snapshot && !deepEqual(local[key], snapshot[key])) {
			const d = mergePatchDiff(snapshot[key], local[key])
			if (!isObject(d) || Object.keys(d).length) patch[key] = d
		}
	}
	const dirty = Object.keys(patch).length > 0
	if (dirty) patch.rev = Math.max(now, (base.rev ?? 0) + 1)
	const server = (dirty ? applyMergePatch(base, patch) : base) as Views
	const values: Partial<Record<SyncedKey, unknown>> = {}
	for (const key of SYNCED_KEYS) {
		if (key in server) values[key] = server[key]
		else if (key in local) values[key] = local[key]
	}
	return {values, patch: dirty ? patch : null, server}
}
