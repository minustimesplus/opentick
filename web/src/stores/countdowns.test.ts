import {beforeEach, describe, expect, it, vi} from 'vitest'
import {createPinia, setActivePinia} from 'pinia'
import {nextTick} from 'vue'
import type {Countdown} from '@/api/types'

const client = vi.hoisted(() => ({
	api: vi.fn(),
	fetchAll: vi.fn(),
}))
vi.mock('@/api/client', async importOriginal => ({...await importOriginal<object>(), ...client}))

const toast = vi.hoisted(() => vi.fn())
vi.mock('@/lib/toast', () => ({toast}))

const {useCountdowns} = await import('./countdowns')
const {useData} = await import('./data')

const trip: Countdown = {id: 1, title: 'Trip', date: '2026-12-20T00:00:00Z', type: 'countdown', repeat_yearly: false}
const birthday: Countdown = {id: 2, title: 'Anna', date: '1990-10-23T00:00:00Z', type: 'birthday', repeat_yearly: true}

describe('useCountdowns', () => {
	beforeEach(() => {
		setActivePinia(createPinia())
		vi.clearAllMocks()
	})

	it('loads every countdown', async () => {
		client.fetchAll.mockResolvedValue([trip, birthday])
		const store = useCountdowns()
		await store.load()
		expect(client.fetchAll).toHaveBeenCalledWith('/countdowns')
		expect(store.countdowns).toEqual([trip, birthday])
		expect(store.loaded).toBe(true)
	})

	it('toasts a failed load and stays unloaded', async () => {
		client.fetchAll.mockRejectedValue(new Error('offline'))
		const store = useCountdowns()
		await store.load()
		expect(toast).toHaveBeenCalledWith('offline')
		expect(store.loaded).toBe(false)
	})

	it('creates with POST and appends the result', async () => {
		client.api.mockResolvedValue(birthday)
		const store = useCountdowns()
		store.countdowns = [trip]
		const input = {title: 'Anna', date: birthday.date, type: 'birthday' as const, repeat_yearly: true, icon: '', hex_color: 'E64980'}
		expect(await store.create(input)).toEqual(birthday)
		expect(client.api).toHaveBeenCalledWith('POST', '/countdowns', {body: input})
		expect(store.countdowns).toEqual([trip, birthday])
	})

	it('returns null and keeps the list when create fails', async () => {
		client.api.mockRejectedValue(new Error('The countdown type must be countdown, anniversary or birthday.'))
		const store = useCountdowns()
		store.countdowns = [trip]
		expect(await store.create({title: 'x', date: trip.date, type: 'countdown', repeat_yearly: false})).toBeNull()
		expect(store.countdowns).toEqual([trip])
		expect(toast).toHaveBeenCalled()
	})

	it('updates with a merge PATCH and replaces the countdown', async () => {
		const renamed = {...trip, title: 'Trip to Kyoto'}
		client.api.mockResolvedValue(renamed)
		const store = useCountdowns()
		store.countdowns = [trip, birthday]
		expect(await store.update(1, {title: 'Trip to Kyoto'})).toBe(true)
		expect(client.api).toHaveBeenCalledWith('PATCH', '/countdowns/1', {body: {title: 'Trip to Kyoto'}, contentType: 'application/merge-patch+json'})
		expect(store.countdowns).toEqual([renamed, birthday])
	})

	it('reports a failed update', async () => {
		client.api.mockRejectedValue(new Error('forbidden'))
		const store = useCountdowns()
		store.countdowns = [trip]
		expect(await store.update(1, {title: 'x'})).toBe(false)
		expect(store.countdowns).toEqual([trip])
	})

	it('removes with DELETE', async () => {
		client.api.mockResolvedValue(undefined)
		const store = useCountdowns()
		store.countdowns = [trip, birthday]
		expect(await store.remove(1)).toBe(true)
		expect(client.api).toHaveBeenCalledWith('DELETE', '/countdowns/1')
		expect(store.countdowns).toEqual([birthday])
	})

	it('keeps the countdown when delete fails', async () => {
		client.api.mockRejectedValue(new Error('not found'))
		const store = useCountdowns()
		store.countdowns = [trip]
		expect(await store.remove(1)).toBe(false)
		expect(store.countdowns).toEqual([trip])
	})

	it('forgets everything on logout', async () => {
		client.fetchAll.mockResolvedValue([trip])
		const data = useData()
		data.authed = true
		const store = useCountdowns()
		await store.load()
		data.authed = false
		await nextTick()
		expect(store.countdowns).toEqual([])
		expect(store.loaded).toBe(false)
	})
})
