import {defineStore} from 'pinia'
import {reactive, ref, shallowRef, watch} from 'vue'
import {api, fetchAll} from '@/api/client'
import type {CalendarSubscription, CalendarSubscriptionEvents} from '@/api/types'
import {useData} from '@/stores/data'
import {toast} from '@/lib/toast'

const MERGE = 'application/merge-patch+json'
/** The server caches feeds for 15 minutes; this only avoids refetching while navigating. */
const CLIENT_TTL = 5 * 60_000

export const DEFAULT_CALENDAR_COLOR = '12B886'

/** A read-only event occurrence from a subscribed calendar. */
export interface CalEvent {
	key: string
	subscriptionId: number
	title: string
	description: string
	location: string
	url: string
	start: Date
	/** Exclusive end. All-day events end at midnight after their last day. */
	end: Date
	allDay: boolean
	recurring: boolean
	color: string
}

export function calendarColor(sub?: CalendarSubscription | null): string {
	return '#' + (sub?.hex_color?.replace(/^#/, '') || DEFAULT_CALENDAR_COLOR)
}

function hostnameOf(url: string): string {
	try {
		return new URL(url.trim().replace(/^webcals?:/i, 'https:')).hostname.replace(/^www\./, '')
	} catch {
		return url
	}
}

const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone

export const useCalendars = defineStore('calendars', () => {
	const data = useData()
	const subscriptions = ref<CalendarSubscription[]>([])
	const loaded = ref(false)
	/** Why a subscription's feed could not be loaded, by subscription id. */
	const errors = reactive(new Map<number, string>())
	/** Events of the most recently requested range, by subscription id. */
	const events = shallowRef(new Map<number, CalEvent[]>())
	const cache = new Map<string, {at: number, events: CalEvent[], name: string}>()
	/** Subscriptions created without a name: they take the feed's own name once known. */
	const autoNamed = new Set<number>()
	let latestRequest = 0

	function reset() {
		subscriptions.value = []
		loaded.value = false
		errors.clear()
		events.value = new Map()
		cache.clear()
		inflight.clear()
		autoNamed.clear()
	}

	watch(() => data.authed, authed => {
		if (!authed) reset()
	})

	async function load() {
		try {
			subscriptions.value = await fetchAll<CalendarSubscription>('/calendar-subscriptions')
			loaded.value = true
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	function subscription(id: number) {
		return subscriptions.value.find(s => s.id === id)
	}

	function cacheKey(sub: CalendarSubscription, start: Date, end: Date) {
		return `${sub.id}|${sub.url}|${start.toISOString()}|${end.toISOString()}`
	}

	function cached(sub: CalendarSubscription, start: Date, end: Date): CalEvent[] | undefined {
		const hit = cache.get(cacheKey(sub, start, end))
		return hit && Date.now() - hit.at < CLIENT_TTL ? hit.events : undefined
	}

	/** Requests in flight by cache key, so a prefetch and a navigation share one request. */
	const inflight = new Map<string, Promise<CalEvent[]>>()

	function fetchEvents(sub: CalendarSubscription, start: Date, end: Date): Promise<CalEvent[]> {
		const hit = cached(sub, start, end)
		if (hit) return Promise.resolve(hit)
		const key = cacheKey(sub, start, end)
		let pending = inflight.get(key)
		if (!pending) {
			pending = requestEvents(sub, start, end, key).finally(() => inflight.delete(key))
			inflight.set(key, pending)
		}
		return pending
	}

	async function requestEvents(sub: CalendarSubscription, start: Date, end: Date, key: string): Promise<CalEvent[]> {
		const res = await api<CalendarSubscriptionEvents>('GET', `/calendar-subscriptions/${sub.id}/events`, {
			query: {from: start.toISOString(), to: end.toISOString(), timezone},
		})
		const list = (res?.events ?? []).map(e => ({
			key: `${sub.id}:${e.id}`,
			subscriptionId: sub.id,
			title: e.title,
			description: e.description,
			location: e.location,
			url: e.url,
			start: new Date(e.start),
			end: new Date(e.end),
			allDay: e.all_day,
			recurring: e.recurring,
			color: '',
		}))
		cache.set(key, {at: Date.now(), events: list, name: res?.calendar_name ?? ''})
		if (autoNamed.has(sub.id) && res?.calendar_name) {
			autoNamed.delete(sub.id)
			update(sub.id, {title: res.calendar_name})
		}
		return list
	}

	/** Loads the events of the given subscriptions overlapping [start, end). */
	async function loadEvents(start: Date, end: Date, ids: number[]) {
		const request = ++latestRequest
		const subs = subscriptions.value.filter(s => ids.includes(s.id))
		// Everything already cached (e.g. prefetched): swap synchronously so the
		// grid never renders a frame without its events.
		const hits = subs.map(sub => [sub.id, cached(sub, start, end)] as const)
		if (hits.every(([, list]) => list)) {
			for (const sub of subs) errors.delete(sub.id)
			events.value = new Map(hits as [number, CalEvent[]][])
			return
		}
		const results = await Promise.all(subs.map(async sub => {
			try {
				const list = await fetchEvents(sub, start, end)
				errors.delete(sub.id)
				return [sub.id, list] as const
			} catch (e) {
				errors.set(sub.id, String((e as Error).message))
				return [sub.id, [] as CalEvent[]] as const
			}
		}))
		// Navigating quickly fires overlapping requests; only the latest may land.
		if (request !== latestRequest) return
		events.value = new Map(results)
	}

	/** Warms the cache for a range the user is likely to navigate to next. */
	function prefetchEvents(start: Date, end: Date, ids: number[]) {
		for (const sub of subscriptions.value) {
			if (ids.includes(sub.id)) fetchEvents(sub, start, end).catch(() => {})
		}
	}

	async function create(input: {title: string, url: string, hex_color: string}): Promise<CalendarSubscription | null> {
		const title = input.title.trim() || hostnameOf(input.url)
		try {
			const sub = await api<CalendarSubscription>('POST', '/calendar-subscriptions', {body: {...input, title}})
			subscriptions.value = [...subscriptions.value, sub]
			if (!input.title.trim()) autoNamed.add(sub.id)
			return sub
		} catch (e) {
			toast(String((e as Error).message))
			return null
		}
	}

	async function update(id: number, patch: Partial<CalendarSubscription>): Promise<boolean> {
		try {
			const updated = await api<CalendarSubscription>('PATCH', `/calendar-subscriptions/${id}`, {body: patch, contentType: MERGE})
			subscriptions.value = subscriptions.value.map(s => s.id === id ? updated : s)
			return true
		} catch (e) {
			toast(String((e as Error).message))
			return false
		}
	}

	async function remove(id: number) {
		try {
			await api('DELETE', `/calendar-subscriptions/${id}`)
			subscriptions.value = subscriptions.value.filter(s => s.id !== id)
			errors.delete(id)
			const next = new Map(events.value)
			next.delete(id)
			events.value = next
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	return {subscriptions, loaded, errors, events, load, subscription, loadEvents, prefetchEvents, create, update, remove}
})
