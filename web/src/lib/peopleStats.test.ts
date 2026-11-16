import {describe, expect, it} from 'vitest'
import type {ContactGroup, ContactListItem} from '@/api/types'
import * as data from './geoData'
import {buildGazetteer, resolve} from './geo'
import {centroid, computeStats, project, wrapLon} from './peopleStats'

const g = buildGazetteer(data)
const where = (s: string) => resolve(g, s)

function contact(patch: Partial<ContactListItem>): ContactListItem {
	return {
		id: 1,
		name: '',
		headline: '',
		company: '',
		title: '',
		location: '',
		email: '',
		phone: '',
		linkedin_url: '',
		birthday: '',
		source: 'manual',
		has_photo: false,
		tag_ids: [],
		group_ids: [],
		schools: [],
		notes_count: 0,
		has_experience: false,
		has_education: false,
		search_text: '',
		created: '2026-10-01T08:00:00Z',
		updated: '2026-10-01T08:00:00Z',
		...patch,
	}
}

const contacts = [
	contact({
		id: 1,
		location: 'Reims, Grand Est, France',
		company: 'Acme',
		schools: ['Sciences Po', 'Sciences Po'],
		has_photo: true,
		notes_count: 2,
		group_ids: [10],
		source: 'csv',
	}),
	contact({
		id: 2,
		location: 'Reims, Grand Est, France',
		company: 'Acme',
		email: 'a@b.c',
		group_ids: [10, 11],
		created: '2026-11-03T08:00:00Z',
	}),
	contact({
		id: 3,
		location: 'Germany',
		company: ' Initech ',
		schools: ['Sciences Po'],
		source: 'csv',
		created: '2025-01-01T08:00:00Z',
	}),
	contact({id: 4, location: 'Remote', notes_count: 1}),
	contact({id: 5, location: '  '}),
]

const groups: ContactGroup[] = [
	{id: 10, name: 'Friends', hex_color: 'ff0000', description: '', members: 2},
	{id: 11, name: 'Work', hex_color: '', description: '', members: 1},
	{id: 12, name: 'Empty', hex_color: '', description: '', members: 0},
]

describe('computeStats', () => {
	const s = computeStats(contacts, groups, where, new Date('2026-11-14T12:00:00Z'))

	it('counts totals', () => {
		expect(s.total).toBe(5)
		expect(s.mapped).toBe(3)
		expect(s.noLocation).toBe(1)
		expect(s.notes).toBe(3)
		expect(s.companiesTotal).toBe(2)
	})

	it('groups contacts by place, biggest first', () => {
		expect(s.places.map(p => [p.label, p.contactIds, p.precise])).toEqual([
			['Reims, France', [1, 2], true],
			['Germany', [3], false],
		])
		expect(s.unmapped).toEqual([{name: 'Remote', count: 1}])
	})

	it('puts countries on their centroid', () => {
		expect(s.countries).toEqual([
			{name: 'France', count: 2, lat: 46.6, lon: 2.2},
			{name: 'Germany', count: 1, lat: 51.2, lon: 10.4},
		])
	})

	it('ranks companies, schools, groups and sources', () => {
		expect(s.companies).toEqual([{name: 'Acme', count: 2}, {name: 'Initech', count: 1}])
		expect(s.schools).toEqual([{name: 'Sciences Po', count: 2}])
		expect(s.groups.map(x => [x.name, x.count])).toEqual([['Friends', 2], ['Work', 1], ['Empty', 0]])
		expect(s.sources).toEqual([{source: 'manual', count: 3}, {source: 'csv', count: 2}])
	})

	it('measures completeness', () => {
		const by = Object.fromEntries(s.completeness.map(c => [c.field, c.count]))
		expect(by).toMatchObject({location: 4, company: 3, photo: 1, email: 1, phone: 0})
	})

	it('buckets the last 12 months', () => {
		expect(s.months).toHaveLength(12)
		expect(s.months[0].key).toBe('2025-12')
		expect(s.months[11]).toEqual({key: '2026-11', count: 1})
		expect(s.months[10]).toEqual({key: '2026-10', count: 3})
	})
})

describe('globe math', () => {
	it('finds the centroid of places', () => {
		const [lon, lat] = centroid([
			{label: 'a', lat: 0, lon: 10, precise: true, contactIds: [1]},
			{label: 'b', lat: 0, lon: 30, precise: true, contactIds: [2]},
		])!
		expect(lon).toBeCloseTo(20)
		expect(lat).toBeCloseTo(0)
		expect(centroid([])).toBeNull()
	})

	it('projects the view center to the middle of the disc', () => {
		const [x, y, z] = project(2, 48, 2, 48)
		expect(x).toBeCloseTo(0)
		expect(y).toBeCloseTo(0)
		expect(z).toBeCloseTo(1)
		expect(project(182, -48, 2, 48)[2]).toBeLessThan(0)
	})

	it('wraps longitudes', () => {
		expect(wrapLon(190)).toBe(-170)
		expect(wrapLon(-190)).toBe(170)
		expect(wrapLon(45)).toBe(45)
	})
})
