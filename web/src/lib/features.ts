import type {RouteRecordNameGeneric} from 'vue-router'
import {prefs, type Feature} from './prefs'

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
