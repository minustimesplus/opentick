import {reactive, watch} from 'vue'
import type {ThemeColor} from './theme'
import type {TimelineScale} from './timeline'
import {isMobile} from './viewport'

/** Optional features from Settings > Features. Turning one off only hides it; its data stays. */
export type Feature = 'calendar' | 'matrix' | 'habit' | 'pomo' | 'countdown' | 'ai'

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
	/** Height (px) of the all-day strip in day/week view, set by dragging its bottom border. */
	calendarAllDayHeight: number
	tasksSidebar: boolean
	/** Widths (px) of the list sidebar and the task detail pane, set by dragging their borders. */
	sidebarWidth: number
	detailWidth: number
	listSort: Record<string, 'date' | 'priority' | 'title' | 'created' | 'custom'>
	listShowCompleted: Record<string, boolean>
	/** Layout per list key: plain list, kanban columns or timeline. */
	listView: Record<string, 'list' | 'kanban' | 'timeline'>
	timelineScale: TimelineScale
	collapsed: Record<string, boolean>
	matrixLists: number[]
	matrixShowDone: boolean
	/** Client last picked in the MCP setup guide. */
	mcpClient: string
	/** Focus timer lengths in minutes; a long break follows every focusLongBreakEvery pomos. */
	focusPomoMinutes: number
	focusShortBreakMinutes: number
	focusLongBreakMinutes: number
	focusLongBreakEvery: number
	/** Start the break right after a pomo ends, and the next pomo after a break. */
	focusAutoStartBreak: boolean
	focusAutoStartPomo: boolean
	features: Record<Feature, boolean>
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
	// Seven columns are too narrow to read on a phone.
	calendarMode: isMobile.value ? 'day' : 'week',
	calendarHidden: [],
	calendarHiddenSubs: [],
	calendarShowCompleted: true,
	calendarShowWeekends: true,
	calendarSidebar: false,
	calendarExpandNight: false,
	calendarAllDayHeight: 72,
	tasksSidebar: true,
	sidebarWidth: 260,
	detailWidth: 420,
	listSort: {},
	listShowCompleted: {},
	listView: {},
	timelineScale: 'week',
	collapsed: {},
	matrixLists: [],
	matrixShowDone: false,
	mcpClient: '',
	focusPomoMinutes: 25,
	focusShortBreakMinutes: 5,
	focusLongBreakMinutes: 15,
	focusLongBreakEvery: 4,
	focusAutoStartBreak: true,
	focusAutoStartPomo: false,
	features: {calendar: true, matrix: true, habit: true, pomo: true, countdown: true, ai: true},
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
		if (raw) {
			const stored = JSON.parse(raw)
			// Features added later default to on for existing users.
			return migrate({...defaults, ...stored, features: {...defaults.features, ...stored.features}})
		}
	} catch {
		// storage unavailable or corrupt
	}
	return {...defaults, features: {...defaults.features}}
}

export const prefs = reactive<Prefs>(load())

watch(prefs, value => {
	try {
		localStorage.setItem(KEY, JSON.stringify(value))
	} catch {
		// ignore
	}
}, {deep: true})
