import {reactive, watch} from 'vue'
import type {ThemeColor} from './theme'

export type DateFormat = 'mdy' | 'dmy' | 'ymd'
export type AltCalendar = 'none' | 'chinese' | 'hebrew' | 'islamic' | 'persian' | 'indian'

export interface Prefs {
	language: 'auto' | 'en' | 'fr'
	/** TickTick-style color series; 'default' is plain light, 'dark' is dark mode. */
	themeColor: ThemeColor
	/** Switch to dark while the OS is in dark mode, otherwise use themeColor. */
	followSystemDark: boolean
	/** Blur the background image behind the task list and matrix. */
	backgroundBlur: boolean
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
	/** Client last picked in the MCP setup guide. */
	mcpClient: string
}

const KEY = 'tt.prefs'

const defaults: Prefs = {
	language: 'auto',
	themeColor: 'dark',
	followSystemDark: false,
	backgroundBlur: true,
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
	mcpClient: '',
}

// Before color series, `theme` was 'dark' | 'light' | 'system'.
function migrate(stored: Prefs & {theme?: string}): Prefs {
	const {theme, ...rest} = stored
	if (theme === 'light') rest.themeColor = 'default'
	if (theme === 'dark') rest.themeColor = 'dark'
	if (theme === 'system') {
		rest.themeColor = 'default'
		rest.followSystemDark = true
	}
	return rest
}

function load(): Prefs {
	try {
		const raw = localStorage.getItem(KEY)
		if (raw) return migrate({...defaults, ...JSON.parse(raw)})
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
