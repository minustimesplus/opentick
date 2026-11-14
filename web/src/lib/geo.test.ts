import {describe, expect, it} from 'vitest'
import * as data from './geoData'
import {buildGazetteer, isLand, landDots, normalize, resolve, stripAffixes} from './geo'

const g = buildGazetteer(data)
const label = (s: string) => resolve(g, s)?.label ?? ''

describe('normalize', () => {
	it('drops case, accents and punctuation', () => {
		expect(normalize('Île-de-France')).toBe('ile de france')
		expect(normalize("  Côte d'Ivoire ")).toBe('cote divoire')
		expect(normalize('Baden-Württemberg')).toBe('baden wurttemberg')
	})
})

describe('stripAffixes', () => {
	it('offers the bare name next to the original', () => {
		expect(stripAffixes('greater oxford area')).toEqual(['greater oxford area', 'oxford'])
		expect(stripAffixes('hong kong sar')).toEqual(['hong kong sar', 'hong kong'])
		expect(stripAffixes('paris')).toEqual(['paris'])
	})
})

describe('resolve', () => {
	it('resolves LinkedIn-style locations', () => {
		expect(label('Reims, Grand Est, France')).toBe('Reims, France')
		expect(label('Paris, Île-de-France, France')).toBe('Paris, France')
		expect(label('Greater Oxford Area')).toBe('Oxford, United Kingdom')
		expect(label('San Francisco Bay Area')).toBe('San Francisco, United States')
		expect(label('New York City Metropolitan Area')).toBe('New York, United States')
		expect(label('Stuttgart Region')).toBe('Stuttgart, Germany')
		expect(label('Hong Kong SAR')).toBe('Hong Kong')
		expect(label('Nerima, Tokyo, Japan')).toBe('Nerima, Japan')
		expect(label('Durham, England, United Kingdom')).toBe('Durham, United Kingdom')
		expect(label('Salem, Baden-Württemberg, Germany')).toBe('Salem, Germany')
		expect(label('Center Sarajevo, Federation of Bosnia and Herzegovina, Bosnia and Herzegovina'))
			.toBe('Sarajevo, Bosnia and Herzegovina')
		expect(label('Washington, District of Columbia, United States')).toBe('Washington, United States')
		expect(label('Tyneside Area')).toBe('Newcastle upon Tyne, United Kingdom')
	})

	it('marks city matches as precise', () => {
		expect(resolve(g, 'Reims, Grand Est, France')?.precise).toBe(true)
		expect(resolve(g, 'Reims, Grand Est, France')?.country).toBe('France')
	})

	it('falls back to the region when the city is unknown', () => {
		expect(resolve(g, 'Some Village, England, United Kingdom')).toMatchObject({
			label: 'England, United Kingdom',
			country: 'United Kingdom',
			precise: false,
		})
	})

	it('puts a bare country on its centroid', () => {
		expect(resolve(g, 'Germany')).toEqual({
			label: 'Germany',
			country: 'Germany',
			lat: 51.2,
			lon: 10.4,
			precise: false,
		})
	})

	it('returns null for unknown places', () => {
		expect(resolve(g, '')).toBeNull()
		expect(resolve(g, ' , ')).toBeNull()
		expect(resolve(g, 'Remote')).toBeNull()
	})
})

describe('land mask', () => {
	it('knows land from sea', () => {
		expect(isLand(data.LAND, 2.35, 48.86)).toBe(true)
		expect(isLand(data.LAND, -100, 40)).toBe(true)
		expect(isLand(data.LAND, 20, 0)).toBe(true)
		expect(isLand(data.LAND, 134, -25)).toBe(true)
		expect(isLand(data.LAND, -30, 30)).toBe(false)
		expect(isLand(data.LAND, 34, 43)).toBe(false)
		expect(isLand(data.LAND, -170, 0)).toBe(false)
	})

	it('dots the continents', () => {
		expect(landDots(data.LAND).length / 2).toBeGreaterThan(1500)
	})
})
