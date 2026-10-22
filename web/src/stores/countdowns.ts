import {defineStore} from 'pinia'
import {ref, watch} from 'vue'
import {api, fetchAll} from '@/api/client'
import type {Countdown} from '@/api/types'
import {useData} from '@/stores/data'
import {toast} from '@/lib/toast'

const MERGE = 'application/merge-patch+json'

export type CountdownInput = Pick<Countdown, 'title' | 'date' | 'type' | 'repeat_yearly' | 'icon' | 'hex_color'>

export const useCountdowns = defineStore('countdowns', () => {
	const data = useData()
	const countdowns = ref<Countdown[]>([])
	const loaded = ref(false)

	watch(() => data.authed, authed => {
		if (!authed) {
			countdowns.value = []
			loaded.value = false
		}
	})

	async function load() {
		try {
			countdowns.value = await fetchAll<Countdown>('/countdowns')
			loaded.value = true
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	async function create(input: CountdownInput): Promise<Countdown | null> {
		try {
			const created = await api<Countdown>('POST', '/countdowns', {body: input})
			countdowns.value = [...countdowns.value, created]
			return created
		} catch (e) {
			toast(String((e as Error).message))
			return null
		}
	}

	async function update(id: number, patch: Partial<CountdownInput>): Promise<boolean> {
		try {
			const updated = await api<Countdown>('PATCH', `/countdowns/${id}`, {body: patch, contentType: MERGE})
			countdowns.value = countdowns.value.map(c => c.id === id ? updated : c)
			return true
		} catch (e) {
			toast(String((e as Error).message))
			return false
		}
	}

	async function remove(id: number): Promise<boolean> {
		try {
			await api('DELETE', `/countdowns/${id}`)
			countdowns.value = countdowns.value.filter(c => c.id !== id)
			return true
		} catch (e) {
			toast(String((e as Error).message))
			return false
		}
	}

	return {countdowns, loaded, load, create, update, remove}
})
