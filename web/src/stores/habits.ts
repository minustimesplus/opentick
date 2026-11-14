import {defineStore} from 'pinia'
import {computed, ref, watch} from 'vue'
import {api, fetchAll} from '@/api/client'
import type {Habit, HabitFrequency, HabitStats} from '@/api/types'
import {useData} from '@/stores/data'
import {dateKey} from '@/lib/dates'
import {toast} from '@/lib/toast'

const MERGE = 'application/merge-patch+json'

export const DEFAULT_HABIT_COLOR = '40C057'
export const ALL_WEEKDAYS = 0b1111111
const POSITION_STEP = 65536

export interface HabitInput {
	title: string
	icon: string
	hex_color: string
	frequency: HabitFrequency
	weekdays: number
	goal: number
	unit: string
	archived?: boolean
	position?: number
}

export function habitColor(h?: Pick<Habit, 'hex_color'> | null): string {
	return '#' + (h?.hex_color?.replace(/^#/, '') || DEFAULT_HABIT_COLOR)
}

export function isDue(h: Pick<Habit, 'weekdays'>, day: Date): boolean {
	return (h.weekdays & (1 << day.getDay())) !== 0
}

/** The client's local date: streaks and "today" follow the user's own clock. */
export function todayKey(): string {
	return dateKey(new Date())
}

export const useHabits = defineStore('habits', () => {
	const data = useData()
	const habits = ref<Habit[]>([])
	const loaded = ref(false)

	const sorted = computed(() => [...habits.value].sort((a, b) => (a.position ?? 0) - (b.position ?? 0) || a.id - b.id))
	const active = computed(() => sorted.value.filter(h => !h.archived))
	const archived = computed(() => sorted.value.filter(h => h.archived))

	watch(() => data.authed, authed => {
		if (!authed) {
			habits.value = []
			loaded.value = false
		}
	})

	function habit(id: number) {
		return habits.value.find(h => h.id === id)
	}

	function replace(next: Habit) {
		const i = habits.value.findIndex(h => h.id === next.id)
		if (i >= 0) habits.value.splice(i, 1, next)
		else habits.value.push(next)
	}

	async function load() {
		try {
			habits.value = await fetchAll<Habit>('/habits', {today: todayKey()})
			loaded.value = true
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	async function create(input: HabitInput): Promise<Habit | null> {
		try {
			const h = await api<Habit>('POST', '/habits', {body: input})
			replace(h)
			return h
		} catch (e) {
			toast(String((e as Error).message))
			return null
		}
	}

	async function update(id: number, patch: Partial<HabitInput>): Promise<Habit | null> {
		try {
			const h = await api<Habit>('PATCH', `/habits/${id}`, {body: patch, contentType: MERGE})
			replace(h)
			return h
		} catch (e) {
			toast(String((e as Error).message))
			return null
		}
	}

	/**
	 * Moves an active habit to `index` among the other active habits (0 is the
	 * top). Only the moved habit's position changes: it takes the mean of its
	 * new neighbours'.
	 */
	async function move(h: Habit, index: number) {
		const others = active.value.filter(x => x.id !== h.id)
		const i = Math.max(0, Math.min(index, others.length))
		const prev = others[i - 1]
		const next = others[i]
		let position: number
		if (prev && next) position = (prev.position + next.position) / 2
		else if (prev) position = prev.position + POSITION_STEP
		else if (next) position = next.position / 2
		else return
		if (position === h.position) return
		const old = h.position
		replace({...h, position})
		if (!await update(h.id, {position})) replace({...h, position: old})
	}

	async function remove(id: number) {
		try {
			await api('DELETE', `/habits/${id}`)
			habits.value = habits.value.filter(h => h.id !== id)
		} catch (e) {
			toast(String((e as Error).message))
		}
	}

	/** Sets the value checked in on a day (YYYY-MM-DD); 0 clears it. */
	async function checkin(id: number, date: string, value: number): Promise<boolean> {
		try {
			const h = await api<Habit>('PUT', `/habits/${id}/checkins/${date}`, {
				body: {value: Math.max(0, value)},
				query: {today: todayKey()},
			})
			replace(h)
			return true
		} catch (e) {
			toast(String((e as Error).message))
			return false
		}
	}

	function valueOn(h: Habit, date: string): number {
		if (date === todayKey()) return h.today_value
		return h.recent_checkins?.find(c => c.date === date)?.value ?? 0
	}

	/**
	 * One tap: a yes/no habit toggles, a counter adds one until its goal is
	 * reached, and a reached goal resets.
	 */
	function tap(h: Habit, date = todayKey()): Promise<boolean> {
		const value = valueOn(h, date)
		const next = value >= h.goal ? 0 : value + 1
		return checkin(h.id, date, h.goal === 1 ? (value ? 0 : 1) : next)
	}

	async function stats(id: number, month?: string): Promise<HabitStats | null> {
		try {
			return await api<HabitStats>('GET', `/habits/${id}/stats`, {query: {today: todayKey(), month}})
		} catch (e) {
			toast(String((e as Error).message))
			return null
		}
	}

	return {habits, loaded, active, archived, habit, load, create, update, move, remove, checkin, valueOn, tap, stats}
})
