import type {RouteRecordNameGeneric} from 'vue-router'
import {prefs, type Feature} from './prefs'
import type {MessageKey} from './i18n'

/** The optional views, in the order Settings and onboarding list them. */
export const OPTIONAL_FEATURES: {key: Feature, title: MessageKey, desc: MessageKey}[] = [
	{key: 'calendar', title: 'featCalendar', desc: 'featCalendarDesc'},
	{key: 'matrix', title: 'matrix', desc: 'featMatrixDesc'},
	{key: 'habit', title: 'featHabit', desc: 'featHabitDesc'},
	{key: 'pomo', title: 'featPomo', desc: 'featPomoDesc'},
	{key: 'countdown', title: 'featCountdown', desc: 'featCountdownDesc'},
	{key: 'ai', title: 'featAi', desc: 'featAiDesc'},
]

/** Backdrop color of each feature's miniature (FeaturePreview). */
export const FEATURE_TINTS: Record<Feature, string> = {
	calendar: '#7d95f7',
	matrix: '#7fb2f2',
	habit: '#9fdd9c',
	pomo: '#f09a8a',
	countdown: '#8fa6f6',
	ai: '#8ab4f4',
}

/** Route name of each optional view and the feature that gates it. */
const routeFeatures: Record<string, Feature> = {
	matrix: 'matrix',
	calendar: 'calendar',
	habits: 'habit',
	focus: 'pomo',
	countdowns: 'countdown',
}

export function featureOn(feature: Feature): boolean {
	return prefs.features[feature]
}

/** False when the route belongs to a feature that is turned off. */
export function routeAllowed(route: {name?: RouteRecordNameGeneric | null}): boolean {
	const feature = routeFeatures[String(route.name ?? '')]
	return !feature || featureOn(feature)
}
