import type {ContactListItem} from '@/api/types'

const FOLD: Record<string, string> = {ß: 'ss', æ: 'ae', œ: 'oe', ø: 'o', đ: 'd', ł: 'l'}

/**
 * Lowercases text, strips accents and keeps only letters and digits, so that
 * "Sciences Po", "SciencesPo" and "sciences-po" compare equal. Mirrors
 * FoldContactText on the server.
 */
export function fold(text: string): string {
	return text
		.toLowerCase()
		.replace(/[ßæœøđł]/g, ch => FOLD[ch])
		.normalize('NFD')
		.replace(/[^\p{L}\p{N}]/gu, '')
}

export function initials(name: string): string {
	const parts = name.trim().split(/\s+/).filter(Boolean)
	if (!parts.length) return '?'
	if (parts.length === 1) return [...parts[0]].slice(0, 2).join('').toUpperCase()
	return ([...parts[0]][0] + [...parts[parts.length - 1]][0]).toUpperCase()
}

const AVATAR_COLORS = ['E03131', 'F76707', 'F59F00', '40C057', '12B886', '15AABF', '4772FA', '7048E8', 'BE4BDB', 'E64980']

/** A stable color for a name, for avatars without a photo and chips without a color. */
export function nameColor(name: string, hex?: string | null): string {
	const given = hex?.replace(/^#/, '')
	if (given) return '#' + given
	let hash = 0
	for (const ch of name) hash = (hash * 31 + ch.codePointAt(0)!) >>> 0
	return '#' + AVATAR_COLORS[hash % AVATAR_COLORS.length]
}

export type ContactSort = 'name' | 'company' | 'location' | 'recent'

export interface ContactFilter {
	query: string
	groupId?: number
	tagId?: number
}

/** A list row with its folded search text, built once per load. */
export interface IndexedContact {
	contact: ContactListItem
	haystack: string
}

export function indexContacts(
	contacts: ContactListItem[],
	tagNames: Map<number, string>,
	groupNames: Map<number, string>,
): IndexedContact[] {
	return contacts.map(contact => ({
		contact,
		haystack: fold([
			contact.name,
			contact.headline,
			contact.company,
			contact.title,
			contact.location,
			contact.email,
			contact.phone,
			contact.search_text,
			...contact.schools,
			...contact.tag_ids.map(id => tagNames.get(id) ?? ''),
			...contact.group_ids.map(id => groupNames.get(id) ?? ''),
		].join(' ')),
	}))
}

/** Every word of the query has to appear somewhere on the contact. */
export function filterContacts(indexed: IndexedContact[], filter: ContactFilter): ContactListItem[] {
	const words = filter.query.split(/\s+/).map(fold).filter(Boolean)
	const out: ContactListItem[] = []
	for (const {contact, haystack} of indexed) {
		if (filter.groupId !== undefined && !contact.group_ids.includes(filter.groupId)) continue
		if (filter.tagId !== undefined && !contact.tag_ids.includes(filter.tagId)) continue
		if (words.every(w => haystack.includes(w))) out.push(contact)
	}
	return out
}

function compareText(a: string, b: string): number {
	// Contacts without the sorted field go last.
	if (!a || !b) return a ? -1 : b ? 1 : 0
	return a.localeCompare(b, undefined, {sensitivity: 'base'})
}

export function sortContacts(contacts: ContactListItem[], sort: ContactSort): ContactListItem[] {
	const byName = (a: ContactListItem, b: ContactListItem) => compareText(a.name, b.name)
	const sorted = [...contacts]
	switch (sort) {
	case 'company': return sorted.sort((a, b) => compareText(a.company, b.company) || byName(a, b))
	case 'location': return sorted.sort((a, b) => compareText(a.location, b.location) || byName(a, b))
	case 'recent': return sorted.sort((a, b) => b.updated.localeCompare(a.updated) || byName(a, b))
	default: return sorted.sort(byName)
	}
}

/** "Consultant · BCG", or whichever of the two there is. */
export function contactSubtitle(c: Pick<ContactListItem, 'title' | 'company' | 'headline'>): string {
	return [c.title, c.company].filter(Boolean).join(' · ') || c.headline
}
