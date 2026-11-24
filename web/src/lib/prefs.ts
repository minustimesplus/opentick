import {reactive, watch} from 'vue'
import type {ThemeColor} from './theme'
import type {TimelineScale} from './timeline'
import {isMobile} from './viewport'
import {smartListDefaults, type SmartListKey, type Visibility} from './smartLists'

/** Optional features from Settings > Features. Turning one off only hides it; its data stays. */
export type Feature = 'calendar' | 'matrix' | 'habit' | 'pomo' | 'countdown' | 'people' | 'search' | 'ai'

export type CalendarMode = 'day' | 'week' | 'month' | 'year' | 'agenda'

export type DateFormat = 'mdy' | 'dmy' | 'ymd'
export type AltCalendar = 'none' | 'chinese' | 'hebrew' | 'islamic' | 'persian' | 'indian'

export type MatrixGroup = 'priority' | 'list' | 'date' | 'none'
export type MatrixSort = 'date' | 'priority' | 'title' | 'created' | 'custom'
export interface MatrixQuadrantView {
	group: MatrixGroup
	sort: MatrixSort
}

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
	calendarMode: CalendarMode
	calendarHidden: number[]
	/** Subscribed calendars hidden from the calendar view. */
	calendarHiddenSubs: number[]
	calendarShowWeekends: boolean
	calendarSidebar: boolean
	calendarExpandNight: boolean
	/** Height (px) of the all-day strip in day/week view, set by dragging its bottom border. */
	calendarAllDayHeight: number
	tasksSidebar: boolean
	/** Widths (px) of the list sidebar and the task detail pane, set by dragging their borders. */
	sidebarWidth: number
	detailWidth: number
	/** The People view's panes, like the tasks view's: shown or collapsed, and their widths (px). */
	peopleSidebar: boolean
	peopleDetail: boolean
	peopleSidebarWidth: number
	peopleDetailWidth: number
	peopleGlobeWidth: number
	/** People > Settings: characters of a contact's About, and of other long texts (experience descriptions), shown before "more". 0 shows everything. */
	peopleAboutCutoff: number
	peopleEntryCutoff: number
	/** Width (px) of the habit detail pane, set by dragging its border. */
	habitDetailWidth: number
	/** Whether the weight panel above the habits list is open. */
	habitWeightOpen: boolean
	/** Whether the calendar shows archived events of subscribed calendars (faded) or hides them. */
	calendarShowArchived: boolean
	listSort: Record<string, 'date' | 'priority' | 'title' | 'created' | 'custom'>
	listShowCompleted: Record<string, boolean>
	/** Layout per list key: plain list, kanban columns or timeline. */
	listView: Record<string, 'list' | 'kanban' | 'timeline'>
	timelineScale: TimelineScale
	collapsed: Record<string, boolean>
	matrixLists: number[]
	matrixShowDone: boolean
	/** Grouping and sorting per Eisenhower quadrant (by quadrant id 1-4). */
	matrixView: Record<number, MatrixQuadrantView>
	/** Manual task order per quadrant, used when its sort is 'custom'. */
	matrixOrder: Record<number, number[]>
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
	/** Chime when a pomo or break ends (TickTick's "Pomo Sound"). */
	focusChime: boolean
	/** White noise played while focusing, and its volume (0..1). */
	focusSound: FocusSound
	focusSoundVolume: number
	/** Show the time left in the tab / window title while a session runs. */
	focusTitle: boolean
	/** Show the floating mini timer over other views while a session runs. */
	focusMini: boolean
	/** Where the in-app mini timer was dragged to (px from the bottom-right corner). */
	focusMiniPos: {right: number, bottom: number}
	/** Enter full-screen mode when a session is started from the Focus view. */
	focusAutoFullscreen: boolean
	focusFullscreenStyle: 'digital' | 'flip'
	/** Daily goals; 0 means no goal. */
	focusGoalPomos: number
	focusGoalMinutes: number
	/** Settings > Notifications: desktop notifications when a task reminder is due, on this device. */
	webReminder: boolean
	/** Repeat reminder notifications until the task is opened or completed. */
	constantReminder: boolean
	features: Record<Feature, boolean>
	/** Order of the view icons in the nav rail (keys), set by dragging them. Unknown or missing keys keep their default place. */
	navOrder: string[]
	/** Settings > Smart List: whether each smart list shows in the sidebar. */
	smartLists: Record<SmartListKey, Visibility>
	/** Settings > Appearance > Display: open-task counts next to sidebar lists. 'hideNote' leaves notes out of the counts. */
	sidebarCount: SidebarCount
	/** Show each task's list color as a bar at the left edge of its row. */
	listColor: boolean
	/** Strike through completed tasks' titles instead of only greying them. */
	completedStrike: boolean
	/** Keys the user rebound in Settings > Shortcuts, by shortcut id; missing ones use the defaults. */
	shortcuts: Record<string, string[]>
	/** Whether the tasks view shows the task detail pane next to the list (split view). */
	tasksDetail: boolean
}

export type SidebarCount = 'all' | 'hideNote' | 'none'

/** White noise for the focus timer, generated in the browser. */
export const FOCUS_SOUNDS = ['none', 'white', 'pink', 'brown', 'rain', 'ocean', 'ticking'] as const
export type FocusSound = typeof FOCUS_SOUNDS[number]

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
	calendarShowWeekends: true,
	calendarSidebar: false,
	calendarExpandNight: false,
	calendarAllDayHeight: 72,
	tasksSidebar: true,
	sidebarWidth: 260,
	detailWidth: 420,
	peopleSidebar: true,
	peopleDetail: true,
	peopleSidebarWidth: 260,
	peopleDetailWidth: 400,
	peopleGlobeWidth: 560,
	// About three lines each in the default-width contact pane.
	peopleAboutCutoff: 80,
	peopleEntryCutoff: 120,
	habitDetailWidth: 460,
	habitWeightOpen: false,
	calendarShowArchived: false,
	listSort: {},
	listShowCompleted: {},
	listView: {},
	timelineScale: 'week',
	collapsed: {},
	matrixLists: [],
	matrixShowDone: false,
	matrixView: {},
	matrixOrder: {},
	mcpClient: '',
	focusPomoMinutes: 25,
	focusShortBreakMinutes: 5,
	focusLongBreakMinutes: 15,
	focusLongBreakEvery: 4,
	focusAutoStartBreak: true,
	focusAutoStartPomo: false,
	focusChime: true,
	focusSound: 'none',
	focusSoundVolume: 0.5,
	focusTitle: true,
	focusMini: true,
	focusMiniPos: {right: 24, bottom: 24},
	focusAutoFullscreen: false,
	focusFullscreenStyle: 'digital',
	focusGoalPomos: 4,
	focusGoalMinutes: 0,
	webReminder: true,
	constantReminder: false,
	navOrder: [],
	features: {calendar: true, matrix: true, habit: true, pomo: true, countdown: true, people: true, search: true, ai: true},
	smartLists: {...smartListDefaults},
	sidebarCount: 'all',
	listColor: true,
	completedStrike: false,
	shortcuts: {},
	tasksDetail: true,
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
			// Features and smart lists added later get their defaults for existing users.
			return migrate({
				...defaults,
				...stored,
				features: {...defaults.features, ...stored.features},
				smartLists: {...defaults.smartLists, ...stored.smartLists},
			})
		}
	} catch {
		// storage unavailable or corrupt
	}
	return {...defaults, features: {...defaults.features}, smartLists: {...defaults.smartLists}}
}

export const prefs = reactive<Prefs>(load())

watch(prefs, value => {
	try {
		localStorage.setItem(KEY, JSON.stringify(value))
	} catch {
		// ignore
	}
}, {deep: true})
