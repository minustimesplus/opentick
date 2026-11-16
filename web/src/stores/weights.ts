import {defineStore} from 'pinia'
import {computed, ref, watch} from 'vue'
import {api} from '@/api/client'
import type {WeightEntry} from '@/api/types'
import {useData} from '@/stores/data'
import {todayKey, useHabits} from '@/stores/habits'
import {addDays, dateKey} from '@/lib/dates'
import {toast} from '@/lib/toast'

/** How far back the weight chart and changes look. */
const HISTORY_DAYS = 365

/** The body weight log (kg) shown with the weigh-in habit and the weight panel. */
export const useWeights = defineStore('weights', () => {
	const data = useData()
	const entries = ref<WeightEntry[]>([])
	const loaded = ref(false)
	let loading: Promise<void> | null = null

	watch(() => data.authed, authed => {
		if (!authed) {
			entries.value = []
			loaded.value = false
		}
	})

	const latest = computed(() => entries.value.at(-1) ?? null)
	const today = computed(() => entries.value.find(e => e.date === todayKey()) ?? null)

	function load(force = false): Promise<void> {
		if (loaded.value && !force) return Promise.resolve()
		loading ??= (async () => {
			try {
				entries.value = await api<WeightEntry[]>('GET', '/weights', {query: {
					from: dateKey(addDays(new Date(), -HISTORY_DAYS)),
					to: todayKey(),
				}})
				loaded.value = true
			} catch (e) {
				toast(String((e as Error).message))
			} finally {
				loading = null
			}
		})()
		return loading
	}

	/** The entries of the last `days` days, oldest first. */
	function since(days: number): WeightEntry[] {
		const from = dateKey(addDays(new Date(), -days))
		return entries.value.filter(e => e.date >= from)
	}

	/** Latest weight minus the first one within the last `days` days; null without two entries. */
	function change(days: number): number | null {
		const recent = since(days)
		if (recent.length < 2) return null
		return Math.round((recent.at(-1)!.weight - recent[0].weight) * 10) / 10
	}

	/** Logs today's (or `date`'s) weight. The server also checks off weigh-in habits. */
	async function log(weight: number, date = todayKey()): Promise<boolean> {
		try {
			const saved = await api<WeightEntry>('PUT', `/weights/${date}`, {body: {weight}, query: {today: todayKey()}})
			entries.value = [...entries.value.filter(e => e.date !== date), saved].sort((a, b) => a.date.localeCompare(b.date))
			await useHabits().load()
			return true
		} catch (e) {
			toast(String((e as Error).message))
			return false
		}
	}

	return {entries, loaded, latest, today, load, since, change, log}
})

/** "72.4", or "+0.3"/"−0.3" with sign; "–" for nothing. */
export function fmtWeight(n: number | null | undefined, signed = false): string {
	if (n === null || n === undefined) return '–'
	const s = n.toFixed(1)
	if (n < 0) return '−' + s.slice(1)
	return signed && n > 0 ? '+' + s : s
}
