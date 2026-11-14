import {describe, expect, it} from 'vitest'
import type {ContactListItem} from '@/api/types'
import {contactSubtitle, filterContacts, fold, indexContacts, initials, nameColor, sortContacts} from './people'

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

const ada = contact({id: 1, name: 'Ada Lovelace', company: 'Analytical Engines', location: 'London', group_ids: [1]})
const grace = contact({id: 2, name: 'Grace Hopper', company: 'US Navy', schools: ['Yale University'], tag_ids: [7], updated: '2026-11-01T08:00:00Z'})
const jean = contact({id: 3, name: 'Jean Dupont', headline: 'Étudiant à SciencesPo', search_text: 'Master Affaires européennes'})

describe('fold', () => {
	it('ignores case, accents, spaces and punctuation', () => {
		expect(fold('Sciences Po')).toBe('sciencespo')
		expect(fold('sciences-po')).toBe('sciencespo')
		expect(fold('Étudiant à SciencesPo')).toBe('etudiantasciencespo')
		expect(fold('Straße')).toBe('strasse')
		expect(fold('Łódź')).toBe('lodz')
	})
})

describe('initials', () => {
	it('takes the first and last name', () => {
		expect(initials('Ada Lovelace')).toBe('AL')
		expect(initials('Jean de la Fontaine')).toBe('JF')
		expect(initials('Cher')).toBe('CH')
		expect(initials('  ')).toBe('?')
	})
})

describe('nameColor', () => {
	it('prefers a given color and is stable otherwise', () => {
		expect(nameColor('Pioneers', '4772fa')).toBe('#4772fa')
		expect(nameColor('Pioneers', '#4772fa')).toBe('#4772fa')
		expect(nameColor('Pioneers')).toBe(nameColor('Pioneers'))
		expect(nameColor('Pioneers')).toMatch(/^#[0-9A-F]{6}$/)
	})
})

describe('filterContacts', () => {
	const indexed = indexContacts([ada, grace, jean], new Map([[7, 'friends']]), new Map([[1, 'Pioneers']]))
	const ids = (query: string, rest = {}) => filterContacts(indexed, {query, ...rest}).map(c => c.id)

	it('needs every word, anywhere on the contact', () => {
		expect(ids('')).toEqual([1, 2, 3])
		expect(ids('ada engines')).toEqual([1])
		expect(ids('ada navy')).toEqual([])
	})

	it('finds schools, tags, groups and history whatever the spelling', () => {
		expect(ids('yale')).toEqual([2])
		expect(ids('FRIENDS')).toEqual([2])
		expect(ids('pioneers')).toEqual([1])
		expect(ids('sciences po')).toEqual([3])
		expect(ids('europeennes')).toEqual([3])
	})

	it('narrows to a group or tag', () => {
		expect(ids('', {groupId: 1})).toEqual([1])
		expect(ids('', {tagId: 7})).toEqual([2])
		expect(ids('grace', {groupId: 1})).toEqual([])
	})
})

describe('sortContacts', () => {
	it('sorts by name, and puts contacts without the field last', () => {
		expect(sortContacts([jean, grace, ada], 'name').map(c => c.id)).toEqual([1, 2, 3])
		expect(sortContacts([jean, grace, ada], 'company').map(c => c.id)).toEqual([1, 2, 3])
		expect(sortContacts([jean, grace, ada], 'location').map(c => c.id)).toEqual([1, 2, 3])
		expect(sortContacts([jean, ada, grace], 'recent').map(c => c.id)).toEqual([2, 1, 3])
	})
})

describe('contactSubtitle', () => {
	it('falls back to the headline', () => {
		expect(contactSubtitle({title: 'Consultant', company: 'BCG', headline: 'x'})).toBe('Consultant · BCG')
		expect(contactSubtitle({title: '', company: 'BCG', headline: 'x'})).toBe('BCG')
		expect(contactSubtitle({title: '', company: '', headline: 'Student'})).toBe('Student')
	})
})
