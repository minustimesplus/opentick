import type {Component} from 'vue'
import {CalendarRange, FileText, Inbox, Layers, ListFilter, SquareCheckBig, SquareX, Sunrise, Tag, Trash2, UserCheck} from 'lucide-vue-next'
import type {MessageKey} from './i18n'

/**
 * Smart lists: the built-in views of the task sidebar, configured in Settings > Smart List.
 * This module only describes them; `useSmartLists` resolves their visibility at runtime.
 */

/** 'auto' is "Show if not empty". */
export type Visibility = 'show' | 'hide' | 'auto'

export type SmartListKey = 'all' | 'today' | 'tomorrow' | 'next7' | 'assigned' | 'inbox' | 'summary'
	| 'tags' | 'filters' | 'completed' | 'wontDo' | 'trash'

export interface SmartListDef {
	key: SmartListKey
	label: MessageKey
	/** Lucide icon; Today and Next 7 Days draw a calendar page (DayIcon) instead. */
	icon?: Component
	/** The `:list` segment of /tasks/:list; sections (tags, filters) have none. */
	route?: string
	/** Settings group: top views, sidebar sections, closed tasks at the bottom. */
	group: 'views' | 'sections' | 'closed'
	modes: Visibility[]
}

const ALL_MODES: Visibility[] = ['show', 'hide', 'auto']

export const SMART_LISTS: SmartListDef[] = [
	{key: 'all', label: 'all', icon: Layers, route: 'all', group: 'views', modes: ALL_MODES},
	{key: 'today', label: 'today', route: 'today', group: 'views', modes: ALL_MODES},
	{key: 'tomorrow', label: 'tomorrow', icon: Sunrise, route: 'tomorrow', group: 'views', modes: ALL_MODES},
	{key: 'next7', label: 'next7', icon: CalendarRange, route: 'week', group: 'views', modes: ALL_MODES},
	{key: 'assigned', label: 'assignedToMe', icon: UserCheck, route: 'assigned', group: 'views', modes: ALL_MODES},
	{key: 'inbox', label: 'inbox', icon: Inbox, route: 'inbox', group: 'views', modes: ALL_MODES},
	// A summary is a report over a date range, so it is never "empty".
	{key: 'summary', label: 'summary', icon: FileText, route: 'summary', group: 'views', modes: ['show', 'hide']},
	{key: 'tags', label: 'tags', icon: Tag, group: 'sections', modes: ALL_MODES},
	{key: 'filters', label: 'filters', icon: ListFilter, group: 'sections', modes: ALL_MODES},
	{key: 'completed', label: 'completed', icon: SquareCheckBig, route: 'completed', group: 'closed', modes: ALL_MODES},
	{key: 'wontDo', label: 'wontDo', icon: SquareX, route: 'wontdo', group: 'closed', modes: ALL_MODES},
	{key: 'trash', label: 'trash', icon: Trash2, route: 'trash', group: 'closed', modes: ALL_MODES},
]

/** TickTick's defaults. */
export const smartListDefaults: Record<SmartListKey, Visibility> = {
	all: 'hide',
	today: 'show',
	tomorrow: 'hide',
	next7: 'show',
	assigned: 'auto',
	inbox: 'show',
	summary: 'hide',
	tags: 'show',
	filters: 'show',
	completed: 'show',
	wontDo: 'auto',
	trash: 'show',
}

export function smartList(key: SmartListKey): SmartListDef {
	return SMART_LISTS.find(d => d.key === key)!
}
