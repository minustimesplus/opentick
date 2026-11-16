/**
 * Offline geography for the People stats globe: a small gazetteer that turns
 * LinkedIn-style locations ("Reims, Grand Est, France", "Greater Oxford Area",
 * "Germany") into coordinates, and a coarse land mask for the dotted continents.
 * Nothing leaves the browser. The tables live in geoData.ts, loaded on demand.
 */

export interface Place {
	/** "Reims, France" or "England, United Kingdom". */
	label: string
	country: string
	lat: number
	lon: number
	/** A city match, as opposed to a region or country fallback. */
	precise: boolean
}

export interface GeoData {
	COUNTRIES: string
	REGIONS: string
	CITIES: string
	LAND: number[][]
}

interface Entry {
	display: string
	cc: string
	lat: number
	lon: number
}

export interface Gazetteer {
	/** Normalized name → country code. */
	countries: Map<string, string>
	countryInfo: Map<string, {name: string, lat: number, lon: number}>
	regions: Map<string, Entry[]>
	cities: Map<string, Entry[]>
}

const FOLD: Record<string, string> = {}
for (const [to, from] of [
	['a', 'àáâãäåāăą'],
	['c', 'çćč'],
	['e', 'èéêëēęě'],
	['i', 'ìíîïı'],
	['n', 'ñńň'],
	['o', 'òóôõöøōő'],
	['u', 'ùúûüūůű'],
	['y', 'ýÿ'],
	['s', 'śšşșß'],
	['z', 'źżž'],
	['l', 'ł'],
	['r', 'ř'],
	['t', 'ťţț'],
	['d', 'ď'],
	[' ', '-_/.'],
	['', '\'’()'],
]) {
	for (const ch of from) FOLD[ch] = to
}

/** Lowercases and strips accents and punctuation, so "Île-de-France" equals "ile de france". */
export function normalize(s: string): string {
	let out = ''
	for (const ch of s.toLowerCase()) out += FOLD[ch] ?? ch
	return out.split(/\s+/).filter(Boolean).join(' ')
}

const PREFIXES = ['greater ', 'city of ', 'commune de ', 'center ', 'centre ', 'metropolitan ']
const SUFFIXES = [
	' metropolitan area',
	' metropolitan region',
	' metro area',
	' metropolitan',
	' capital region',
	' area',
	' region',
	' sar',
	' city',
]

/** The segment itself and, if different, the segment without "Greater …" / "… Area"-style wrapping. */
export function stripAffixes(seg: string): string[] {
	let s = seg
	for (const pre of PREFIXES) if (s.startsWith(pre)) s = s.slice(pre.length)
	for (const suf of SUFFIXES) if (s.endsWith(suf)) s = s.slice(0, -suf.length)
	return s !== seg && s ? [seg, s] : [seg]
}

function parseRows(data: string): {names: string[], cc: string, lat: number, lon: number}[] {
	const rows = []
	for (const line of data.split('\n')) {
		if (!line.trim()) continue
		const parts = line.split(';')
		if (parts.length !== 3 && parts.length !== 4) continue
		const [names, cc, lat, lon] = parts.length === 4 ? parts : [parts[0], '', parts[1], parts[2]]
		const latN = Number(lat.trim())
		const lonN = Number(lon.trim())
		if (Number.isNaN(latN) || Number.isNaN(lonN)) continue
		rows.push({names: names.split('|'), cc, lat: latN, lon: lonN})
	}
	return rows
}

export function buildGazetteer(data: GeoData): Gazetteer {
	const g: Gazetteer = {
		countries: new Map(),
		countryInfo: new Map(),
		regions: new Map(),
		cities: new Map(),
	}
	for (const {names, lat, lon} of parseRows(data.COUNTRIES)) {
		const [cc, ...aliases] = names
		g.countryInfo.set(cc, {name: aliases[0], lat, lon})
		for (const n of aliases) g.countries.set(normalize(n), cc)
	}
	for (const [table, rows] of [[g.regions, data.REGIONS], [g.cities, data.CITIES]] as const) {
		for (const {names, cc, lat, lon} of parseRows(rows)) {
			// "Cambridge MA" style names disambiguate in the data only.
			const display = (names[0].replace(/[A-Z ]+$/, '').trim() || names[0]).replace(/( State)+$/, '')
			const base = normalize(display)
			for (const n of names) {
				// Keys like "durham nc" also answer to plain "durham".
				for (const key of new Set([normalize(n), base])) {
					let entries = table.get(key)
					if (!entries) table.set(key, entries = [])
					if (!entries.some(e => e.cc === cc && e.display === display)) entries.push({display, cc, lat, lon})
				}
			}
		}
	}
	return g
}

/**
 * Turns a free-text location into coordinates, preferring the most specific
 * match (city, then region, then country) that agrees with the country named
 * in the text, if any.
 */
export function resolve(g: Gazetteer, location: string): Place | null {
	const segs = location.split(',').map(normalize).filter(Boolean).map(stripAffixes)
	if (!segs.length) return null
	const countryName = (cc: string) => g.countryInfo.get(cc)?.name ?? cc

	let cc: string | undefined
	for (const key of [...segs].reverse().flat()) {
		cc = g.countries.get(key)
		if (cc) break
	}
	// A region can pin down the country too ("England" → GB).
	if (!cc) {
		for (const key of segs.slice(1).flat()) {
			const v = g.regions.get(key)
			if (v?.length === 1) {
				cc = v[0].cc
				break
			}
		}
	}
	const pick = (v: Entry[] | undefined) => cc ? v?.find(e => e.cc === cc) : v?.[0]

	for (const key of segs.flat()) {
		const e = pick(g.cities.get(key))
		if (e) {
			const country = countryName(e.cc)
			const label = e.display === country ? e.display : `${e.display}, ${country}`
			return {label, country, lat: e.lat, lon: e.lon, precise: true}
		}
	}
	for (const key of segs.flat()) {
		if (g.countries.has(key)) continue
		const e = pick(g.regions.get(key))
		if (e) {
			const country = countryName(e.cc)
			return {label: `${e.display}, ${country}`, country, lat: e.lat, lon: e.lon, precise: false}
		}
	}
	const info = cc ? g.countryInfo.get(cc) : undefined
	if (!info) return null
	return {label: info.name, country: info.name, lat: info.lat, lon: info.lon, precise: false}
}

function inRing(ring: number[], lon: number, lat: number): boolean {
	let inside = false
	const n = ring.length / 2
	for (let i = 0, j = n - 1; i < n; j = i++) {
		const xi = ring[2 * i]
		const yi = ring[2 * i + 1]
		const xj = ring[2 * j]
		const yj = ring[2 * j + 1]
		if ((yi > lat) !== (yj > lat) && lon < (xj - xi) * (lat - yi) / (yj - yi) + xi) inside = !inside
	}
	return inside
}

/** Even-odd over all rings, so the Black Sea and Caspian cut holes into Eurasia. */
export function isLand(land: number[][], lon: number, lat: number): boolean {
	let n = 0
	for (const ring of land) if (inRing(ring, lon, lat)) n++
	return n % 2 === 1
}

/** Evenly spaced points on land, as flat [lon, lat, lon, lat, …]. */
export function landDots(land: number[][]): Float32Array {
	const step = 1.8
	const out: number[] = []
	for (let lat = -88; lat <= 84; lat += step) {
		const lonStep = Math.min(step / Math.max(Math.cos(lat * Math.PI / 180), 0.05), 60)
		const n = Math.max(Math.floor(360 / lonStep), 1)
		const off = Math.trunc(lat / step) % 2 === 0 ? 0 : 0.5
		for (let i = 0; i < n; i++) {
			const lon = -180 + (i + off) * 360 / n
			if (isLand(land, lon, lat)) out.push(lon, lat)
		}
	}
	return new Float32Array(out)
}

export interface Geo {
	resolve: (location: string) => Place | null
	dots: Float32Array
}

let geo: Promise<Geo> | null = null

/** Loads the tables (a separate chunk) once, and caches resolved locations. */
export function loadGeo(): Promise<Geo> {
	geo ??= import('./geoData').then(data => {
		const g = buildGazetteer(data)
		const cache = new Map<string, Place | null>()
		return {
			resolve: (location: string) => {
				let place = cache.get(location)
				if (place === undefined) cache.set(location, place = resolve(g, location))
				return place
			},
			dots: landDots(data.LAND),
		}
	}).catch(e => {
		geo = null
		throw e
	})
	return geo
}
