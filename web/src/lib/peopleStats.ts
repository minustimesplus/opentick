import type {ContactGroup, ContactListItem, ContactSource} from '@/api/types'
import type {Place} from './geo'

export interface PlacePoint {
	label: string
	lat: number
	lon: number
	precise: boolean
	contactIds: number[]
}

export interface Ranked {
	name: string
	count: number
}

export interface CountryCount extends Ranked {
	lat: number
	lon: number
}

export interface GroupCount extends Ranked {
	id: number
	hexColor: string
}

export type CompletenessField =
	| 'location'
	| 'company'
	| 'photo'
	| 'linkedin'
	| 'experience'
	| 'education'
	| 'email'
	| 'phone'
	| 'birthday'

export interface PeopleStats {
	total: number
	/** Sorted biggest first, so smaller markers are drawn (and hit-tested) on top. */
	places: PlacePoint[]
	mapped: number
	unmapped: Ranked[]
	noLocation: number
	countries: CountryCount[]
	companiesTotal: number
	companies: Ranked[]
	schools: Ranked[]
	groups: GroupCount[]
	sources: {source: ContactSource, count: number}[]
	completeness: {field: CompletenessField, count: number}[]
	/** The last 12 months, oldest first, keyed "YYYY-MM" in UTC. */
	months: {key: string, count: number}[]
	notes: number
}

const TOP = 8

function byCount<T extends {count: number, name: string}>(a: T, b: T): number {
	return b.count - a.count || a.name.localeCompare(b.name)
}

function top(counts: Map<string, number>, n = TOP): Ranked[] {
	return [...counts].map(([name, count]) => ({name, count})).sort(byCount).slice(0, n)
}

function bump(m: Map<string, number>, key: string) {
	m.set(key, (m.get(key) ?? 0) + 1)
}

export function computeStats(
	contacts: ContactListItem[],
	groups: ContactGroup[],
	resolve: (location: string) => Place | null,
	now = new Date(),
): PeopleStats {
	const places = new Map<string, PlacePoint>()
	const unmapped = new Map<string, number>()
	const countries = new Map<string, number>()
	const companies = new Map<string, number>()
	const schools = new Map<string, number>()
	const sources = new Map<ContactSource, number>()
	const perMonth = new Map<string, number>()
	let mapped = 0
	let noLocation = 0
	let notes = 0

	for (const c of contacts) {
		notes += c.notes_count
		const loc = c.location.trim()
		if (!loc) {
			noLocation++
		} else {
			const place = resolve(loc)
			if (!place) {
				bump(unmapped, loc)
			} else {
				mapped++
				let point = places.get(place.label)
				if (!point) {
					places.set(place.label, point = {
						label: place.label,
						lat: place.lat,
						lon: place.lon,
						precise: place.precise,
						contactIds: [],
					})
				}
				point.contactIds.push(c.id)
				bump(countries, place.country)
			}
		}
		const company = c.company.trim()
		if (company) bump(companies, company)
		for (const s of new Set(c.schools.map(s => s.trim()).filter(Boolean))) bump(schools, s)
		sources.set(c.source, (sources.get(c.source) ?? 0) + 1)
		if (c.created.length >= 7) bump(perMonth, c.created.slice(0, 7))
	}

	const has = (s: string) => !!s.trim()
	const count = (f: (c: ContactListItem) => boolean) => contacts.filter(f).length

	const months = []
	for (let i = 11; i >= 0; i--) {
		const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - i, 1))
		const key = `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`
		months.push({key, count: perMonth.get(key) ?? 0})
	}

	const groupCounts = new Map<number, number>()
	for (const c of contacts) for (const id of c.group_ids) groupCounts.set(id, (groupCounts.get(id) ?? 0) + 1)

	return {
		total: contacts.length,
		places: [...places.values()]
			.sort((a, b) => b.contactIds.length - a.contactIds.length || a.label.localeCompare(b.label)),
		mapped,
		unmapped: top(unmapped, Infinity),
		noLocation,
		// Country markers sit on the country's centroid, not on the first city seen.
		countries: [...countries].map(([name, count]) => {
			const p = resolve(name)
			return {name, count, lat: p?.lat ?? 0, lon: p?.lon ?? 0}
		}).sort(byCount),
		companiesTotal: companies.size,
		companies: top(companies),
		schools: top(schools),
		groups: groups
			.map(g => ({id: g.id, name: g.name, hexColor: g.hex_color, count: groupCounts.get(g.id) ?? 0}))
			.sort(byCount)
			.slice(0, TOP),
		sources: [...sources].map(([source, count]) => ({source, count})).sort((a, b) => b.count - a.count),
		completeness: [
			{field: 'location', count: count(c => has(c.location))},
			{field: 'company', count: count(c => has(c.company))},
			{field: 'photo', count: count(c => c.has_photo)},
			{field: 'linkedin', count: count(c => has(c.linkedin_url))},
			{field: 'experience', count: count(c => c.has_experience)},
			{field: 'education', count: count(c => c.has_education)},
			{field: 'email', count: count(c => has(c.email))},
			{field: 'phone', count: count(c => has(c.phone))},
			{field: 'birthday', count: count(c => has(c.birthday))},
		],
		months,
		notes,
	}
}

/** Center of mass of all placed contacts, as [lon, lat]. */
export function centroid(places: PlacePoint[]): [number, number] | null {
	let x = 0
	let y = 0
	let z = 0
	for (const p of places) {
		const l = p.lon * Math.PI / 180
		const b = p.lat * Math.PI / 180
		const w = p.contactIds.length
		x += w * Math.cos(b) * Math.cos(l)
		y += w * Math.cos(b) * Math.sin(l)
		z += w * Math.sin(b)
	}
	const n = Math.hypot(x, y, z)
	if (n === 0) return null
	return [Math.atan2(y, x) * 180 / Math.PI, Math.asin(z / n) * 180 / Math.PI]
}

/** Orthographic projection onto the unit disc: x right, y up, z > 0 on the visible side. */
export function project(lon: number, lat: number, lon0: number, lat0: number): [number, number, number] {
	const r = Math.PI / 180
	const l = lon * r
	const b = lat * r
	const b0 = lat0 * r
	const dl = l - lon0 * r
	const cb = Math.cos(b)
	const cdl = Math.cos(dl)
	return [
		cb * Math.sin(dl),
		Math.cos(b0) * Math.sin(b) - Math.sin(b0) * cb * cdl,
		Math.sin(b0) * Math.sin(b) + Math.cos(b0) * cb * cdl,
	]
}

export function wrapLon(l: number): number {
	return ((l + 180) % 360 + 360) % 360 - 180
}
