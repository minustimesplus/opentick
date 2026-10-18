import {reactive, watch} from 'vue'

export type DateFormat = 'mdy' | 'dmy' | 'ymd'
export type AltCalendar = 'none' | 'chinese' | 'hebrew' | 'islamic' | 'persian' | 'indian'

export interface Prefs {
	language: 'auto' | 'en' | 'fr'
	theme: 'dark' | 'light' | 'system'
	weekStart: 0 | 1 | 6
	hour12: boolean
	/** Order of numeric dates: 12/31/2026, 31/12/2026 or 2026/12/31. */
	dateFormat: DateFormat
	/** Secondary calendar shown next to day numbers. */
	altCalendar: AltCalendar
	showWeekNumbers: boolean
	defaultListId: number | null
	calendarMode: 'day' | 'week' | 'month'
	calendarHidden: number[]
	/** Subscribed calendars hidden from the calendar view. */
	calendarHiddenSubs: number[]
	calendarShowCompleted: boolean
	calendarShowWeekends: boolean
	calendarSidebar: boolean
	calendarExpandNight: boolean
	tasksSidebar: boolean
	listSort: Record<string, 'date' | 'priority' | 'title' | 'created'>
	listShowCompleted: Record<string, boolean>
	collapsed: Record<string, boolean>
	matrixLists: number[]
	matrixShowDone: boolean
}

const KEY = 'tt.prefs'

const defaults: Prefs = {
	language: 'auto',
	theme: 'dark',
	weekStart: 1,
	hour12: false,
	dateFormat: navigator.language === 'en-US' ? 'mdy' : 'dmy',
	altCalendar: 'none',
	showWeekNumbers: false,
	defaultListId: null,
	calendarMode: 'week',
	calendarHidden: [],
	calendarHiddenSubs: [],
	calendarShowCompleted: true,
	calendarShowWeekends: true,
	calendarSidebar: false,
	calendarExpandNight: false,
	tasksSidebar: true,
	listSort: {},
	listShowCompleted: {},
	collapsed: {},
	matrixLists: [],
	matrixShowDone: false,
}

function load(): Prefs {
	try {
		const raw = localStorage.getItem(KEY)
		if (raw) return {...defaults, ...JSON.parse(raw)}
	} catch {
		// storage unavailable or corrupt
	}
	return {...defaults}
}

export const prefs = reactive<Prefs>(load())

watch(prefs, value => {
	try {
		localStorage.setItem(KEY, JSON.stringify(value))
	} catch {
		// ignore
	}
}, {deep: true})
