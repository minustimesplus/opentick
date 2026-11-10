import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {createPinia, setActivePinia} from 'pinia'
import type {CalendarSubscription} from '@/api/types'

const client = vi.hoisted(() => ({
	api: vi.fn(),
	fetchAll: vi.fn(),
}))
vi.mock('@/api/client', async importOriginal => ({...await importOriginal<object>(), ...client}))

const toast = vi.hoisted(() => vi.fn())
vi.mock('@/lib/toast', () => ({toast}))

const {eventRangeFor, useCalendars} = await import('./calendars')
const {prefs} = await import('@/lib/prefs')
const {addDays, startOfDay} = await import('@/lib/dates')

const work = {id: 1, title: 'Work', url: 'https://example.com/work.ics', hex_color: ''} as CalendarSubscription
const hidden = {id: 2, title: 'Hidden', url: 'https://example.com/hidden.ics', hex_color: ''} as CalendarSubscription

function eventsResponse(title: string) {
	return {calendar_name: 'Work', events: [{
		id: 'e1', title, description: '', location: '', url: '',
		start: '2026-11-10T09:00:00Z', end: '2026-11-10T10:00:00Z', all_day: false, recurring: false,
	}]}
}

function requestedRanges() {
	return client.api.mock.calls.map(([, path, opts]) => `${path} ${opts.query.from} ${opts.query.to}`)
}

const flush = () => new Promise(r => setTimeout(r, 0))

describe('useCalendars preload', () => {
	beforeEach(() => {
		setActivePinia(createPinia())
		vi.clearAllMocks()
		prefs.calendarMode = 'week'
		prefs.calendarHiddenSubs = [2]
		client.fetchAll.mockResolvedValue([work, hidden])
		client.api.mockResolvedValue(eventsResponse('Standup'))
	})

	afterEach(() => {
		vi.useRealTimers()
		prefs.calendarHiddenSubs = []
	})

	it('loads this week and next week of every visible subscription', async () => {
		const store = useCalendars()
		await store.preload()
		await flush()
		const today = startOfDay(new Date())
		const expected = [today, addDays(today, 7)].map(at => {
			const r = eventRangeFor('week', at)
			return `/calendar-subscriptions/1/events ${r.start.toISOString()} ${r.end.toISOString()}`
		})
		expect(requestedRanges()).toEqual(expected)
	})

	it('lets the calendar show preloaded events without another request', async () => {
		const store = useCalendars()
		await store.preload()
		await flush()
		client.api.mockClear()
		const r = eventRangeFor('week', new Date())
		const pending = store.loadEvents(r.start, r.end, [1])
		// Swapped in synchronously, before any await.
		expect(store.events.get(1)?.map(e => e.title)).toEqual(['Standup'])
		await pending
		expect(client.api).not.toHaveBeenCalled()
	})

	it('does not toast when the background load fails', async () => {
		client.fetchAll.mockRejectedValue(new Error('offline'))
		const store = useCalendars()
		await store.preload()
		expect(toast).not.toHaveBeenCalled()
		expect(client.api).not.toHaveBeenCalled()
	})

	it('shows expired events at once and replaces them when fresh ones arrive', async () => {
		vi.useFakeTimers({toFake: ['Date']})
		const store = useCalendars()
		await store.preload()
		await flush()
		const r = eventRangeFor('week', new Date())
		vi.setSystemTime(Date.now() + 10 * 60_000)
		client.api.mockResolvedValue(eventsResponse('Moved standup'))
		const pending = store.loadEvents(r.start, r.end, [1])
		expect(store.events.get(1)?.map(e => e.title)).toEqual(['Standup'])
		await pending
		expect(store.events.get(1)?.map(e => e.title)).toEqual(['Moved standup'])
	})
})
