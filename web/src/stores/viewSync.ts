// Keeps the synced view prefs (lib/viewSync.ts) in step with the server: every
// GET /user (sync on start, on focus and every minute) brings the server state,
// and local edits go out as a merge patch shortly after they happen.

import {watch} from 'vue'
import {api} from '@/api/client'
import {prefs, type Prefs} from '@/lib/prefs'
import {deepEqual, reconcile, SYNCED_KEYS, type SyncedKey, type Views} from '@/lib/viewSync'
import {useData} from './data'

const SNAPSHOT_KEY = 'tt.prefs.synced'
const PUSH_DELAY = 600

interface Stored {
	user: number
	views: Views
}

function loadSnapshot(user: number): Views | null {
	try {
		const s = JSON.parse(localStorage.getItem(SNAPSHOT_KEY) ?? 'null') as Stored | null
		return s?.user === user ? s.views : null
	} catch {
		return null
	}
}

function saveSnapshot(user: number, views: Views) {
	try {
		localStorage.setItem(SNAPSHOT_KEY, JSON.stringify({user, views} satisfies Stored))
	} catch {
		// storage unavailable
	}
}

function localValues(): Partial<Record<SyncedKey, unknown>> {
	// Plain copies, not reactive proxies.
	return JSON.parse(JSON.stringify(Object.fromEntries(SYNCED_KEYS.map(k => [k, prefs[k]]))))
}

function remoteViews(settings: unknown): Views {
	const views = (settings as {frontend_settings?: {views?: unknown}} | undefined)?.frontend_settings?.views
	return typeof views === 'object' && views !== null && !Array.isArray(views) ? views as Views : {}
}

let started = false

export function startViewSync() {
	if (started) return
	started = true
	const data = useData()
	let userId: number | null = null
	let snapshot: Views | null = null
	let chain = Promise.resolve()
	let timer: number | undefined

	function run(remote: () => Views) {
		chain = chain.then(async () => {
			const user = userId
			if (user === null || !data.authed || data.local) return
			const r = reconcile(localValues(), snapshot, remote())
			for (const key of SYNCED_KEYS) {
				if (key in r.values && !deepEqual(prefs[key], r.values[key])) {
					(prefs as Record<SyncedKey, unknown>)[key] = structuredClone(r.values[key]) as Prefs[SyncedKey]
				}
			}
			if (r.patch) {
				try {
					await api('PATCH', '/user/settings/general', {
						body: {frontend_settings: {views: r.patch}},
						contentType: 'application/merge-patch+json',
					})
				} catch {
					// Offline: the edit stays local and goes out with the next sync.
					return
				}
			}
			if (user !== userId) return
			snapshot = r.server
			saveSnapshot(user, snapshot)
		})
	}

	// Every sync replaces data.user with a fresh copy from the server.
	watch(() => data.user, u => {
		if (!u || data.local) {
			userId = null
			return
		}
		if (u.id !== userId) {
			userId = u.id
			snapshot = loadSnapshot(u.id)
		}
		run(() => remoteViews(u.settings))
	})

	watch(() => SYNCED_KEYS.map(k => prefs[k]), () => {
		clearTimeout(timer)
		timer = window.setTimeout(() => {
			// Nothing to compare against before the first server answer.
			if (snapshot) run(() => snapshot!)
		}, PUSH_DELAY)
	}, {deep: true})
}
