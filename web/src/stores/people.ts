import {defineStore} from 'pinia'
import {computed, ref, watch} from 'vue'
import {api} from '@/api/client'
import type {
	Contact,
	ContactEducation,
	ContactExperience,
	ContactField,
	ContactGroup,
	ContactImportResult,
	ContactListItem,
	ContactMembershipResult,
	ContactNote,
	ContactOverview,
	ContactTag,
} from '@/api/types'
import {useData} from '@/stores/data'
import {indexContacts} from '@/lib/people'
import {toast} from '@/lib/toast'

const MERGE = 'application/merge-patch+json'

export type ExperienceInput = Pick<ContactExperience, 'company' | 'title' | 'location' | 'start_date' | 'end_date' | 'description'>
export type EducationInput = Pick<ContactEducation, 'school' | 'degree' | 'field_of_study' | 'start_year' | 'end_year'>
export type NewContactInput = Partial<Record<ContactField, string>>

export interface GroupInput {
	name: string
	hex_color: string
	description: string
}

function message(e: unknown): string {
	return String((e as Error).message)
}

export const usePeople = defineStore('people', () => {
	const data = useData()
	const contacts = ref<ContactListItem[]>([])
	const groups = ref<ContactGroup[]>([])
	const tags = ref<ContactTag[]>([])
	const loaded = ref(false)
	const loading = ref(false)
	/** Bumped per contact when its photo changes, so cached images are refetched. */
	const photoVersions = ref<Record<number, number>>({})

	const tagNames = computed(() => new Map(tags.value.map(t => [t.id, t.name])))
	const groupNames = computed(() => new Map(groups.value.map(g => [g.id, g.name])))
	const indexed = computed(() => indexContacts(contacts.value, tagNames.value, groupNames.value))

	watch(() => data.authed, authed => {
		if (!authed) {
			contacts.value = []
			groups.value = []
			tags.value = []
			loaded.value = false
		}
	})

	async function load() {
		loading.value = true
		try {
			const overview = await api<ContactOverview>('GET', '/contacts/overview')
			contacts.value = overview.contacts ?? []
			groups.value = overview.groups ?? []
			tags.value = overview.tags ?? []
			loaded.value = true
		} catch (e) {
			toast(message(e))
		} finally {
			loading.value = false
		}
	}

	async function read(id: number): Promise<Contact | null> {
		try {
			return await api<Contact>('GET', `/contacts/${id}`)
		} catch (e) {
			toast(message(e))
			return null
		}
	}

	/** Copies what the list shows from a full contact into its row. */
	function applyToRow(c: Contact) {
		const row: ContactListItem = {
			id: c.id,
			name: c.name,
			headline: c.headline,
			company: c.company,
			title: c.title,
			location: c.location,
			email: c.email,
			phone: c.phone,
			linkedin_url: c.linkedin_url,
			birthday: c.birthday,
			source: c.source,
			has_photo: c.has_photo,
			tag_ids: c.tag_ids ?? [],
			group_ids: c.group_ids ?? [],
			schools: [...new Set((c.educations ?? []).map(e => e.school).filter(Boolean))],
			notes_count: c.notes?.length ?? 0,
			has_experience: !!c.experiences?.length,
			has_education: !!c.educations?.length,
			search_text: [
				c.about,
				...(c.educations ?? []).flatMap(e => [e.degree, e.field_of_study]),
				...(c.experiences ?? []).flatMap(e => [e.company, e.title]),
			].filter(Boolean).join(' '),
			created: c.created,
			updated: c.updated,
		}
		const i = contacts.value.findIndex(x => x.id === c.id)
		if (i >= 0) contacts.value.splice(i, 1, row)
		else contacts.value.push(row)
	}

	/** Throws the ApiError, so the dialog can offer to add a duplicate anyway (409). */
	async function create(input: NewContactInput, allowDuplicate = false): Promise<Contact> {
		const c = await api<Contact>('POST', '/contacts', {
			body: input,
			query: {allow_duplicate: allowDuplicate || undefined},
		})
		applyToRow(c)
		return c
	}

	async function update(id: number, patch: Partial<Record<ContactField, string>> & {locked_fields?: string[]}): Promise<Contact | null> {
		try {
			const c = await api<Contact>('PATCH', `/contacts/${id}`, {body: patch, contentType: MERGE})
			applyToRow(c)
			return c
		} catch (e) {
			toast(message(e))
			return null
		}
	}

	async function remove(ids: number[]): Promise<boolean> {
		try {
			if (ids.length === 1) await api('DELETE', `/contacts/${ids[0]}`)
			else await api('POST', '/contacts/bulk/delete', {body: {contact_ids: ids}})
			const gone = new Set(ids)
			contacts.value = contacts.value.filter(c => !gone.has(c.id))
			// Member counts changed.
			load()
			return true
		} catch (e) {
			toast(message(e))
			return false
		}
	}

	async function setGroup(group: string | number, ids: number[], on: boolean, createGroup = false): Promise<ContactMembershipResult | null> {
		try {
			const res = await api<ContactMembershipResult>('POST', `/contacts/bulk/${on ? 'add-to-group' : 'remove-from-group'}`, {
				body: {group: String(group), contact_ids: ids, create: createGroup || undefined},
			})
			await load()
			return res
		} catch (e) {
			toast(message(e))
			return null
		}
	}

	async function setTag(tag: string | number, ids: number[], on: boolean): Promise<ContactMembershipResult | null> {
		try {
			const res = await api<ContactMembershipResult>('POST', `/contacts/bulk/${on ? 'tag' : 'untag'}`, {
				body: {tag: String(tag), contact_ids: ids},
			})
			await load()
			return res
		} catch (e) {
			toast(message(e))
			return null
		}
	}

	async function saveGroup(input: GroupInput, id?: number): Promise<ContactGroup | null> {
		try {
			const g = id
				? await api<ContactGroup>('PUT', `/contact-groups/${id}`, {body: input})
				: await api<ContactGroup>('POST', '/contact-groups', {body: input})
			const i = groups.value.findIndex(x => x.id === g.id)
			if (i >= 0) groups.value.splice(i, 1, g)
			else groups.value.push(g)
			groups.value.sort((a, b) => a.name.localeCompare(b.name, undefined, {sensitivity: 'base'}))
			return g
		} catch (e) {
			toast(message(e))
			return null
		}
	}

	async function removeGroup(id: number): Promise<boolean> {
		try {
			await api('DELETE', `/contact-groups/${id}`)
			await load()
			return true
		} catch (e) {
			toast(message(e))
			return false
		}
	}

	async function removeTag(id: number): Promise<boolean> {
		try {
			await api('DELETE', `/contact-tags/${id}`)
			await load()
			return true
		} catch (e) {
			toast(message(e))
			return false
		}
	}

	/** Runs a change to a contact's history, notes or photo, then returns the contact as it is now. */
	async function changeEntry(id: number, method: string, path: string, body?: unknown): Promise<Contact | null> {
		try {
			await api(method, `/contacts/${id}/${path}`, {body})
			const c = await api<Contact>('GET', `/contacts/${id}`)
			applyToRow(c)
			return c
		} catch (e) {
			toast(message(e))
			return null
		}
	}

	const saveExperience = (id: number, input: ExperienceInput, entry?: number) =>
		changeEntry(id, entry ? 'PUT' : 'POST', entry ? `experiences/${entry}` : 'experiences', input)
	const removeExperience = (id: number, entry: number) => changeEntry(id, 'DELETE', `experiences/${entry}`)
	const saveEducation = (id: number, input: EducationInput, entry?: number) =>
		changeEntry(id, entry ? 'PUT' : 'POST', entry ? `educations/${entry}` : 'educations', input)
	const removeEducation = (id: number, entry: number) => changeEntry(id, 'DELETE', `educations/${entry}`)
	const saveNote = (id: number, body: string, note?: ContactNote) =>
		changeEntry(id, note ? 'PUT' : 'POST', note ? `notes/${note.id}` : 'notes', {body})
	const removeNote = (id: number, entry: number) => changeEntry(id, 'DELETE', `notes/${entry}`)

	async function setPhoto(id: number, file: File | null): Promise<Contact | null> {
		let body: FormData | undefined
		if (file) {
			body = new FormData()
			body.append('photo', file)
		}
		const c = await changeEntry(id, file ? 'POST' : 'DELETE', 'photo', body)
		if (c) photoVersions.value = {...photoVersions.value, [id]: (photoVersions.value[id] ?? 0) + 1}
		return c
	}

	async function importLinkedinCsv(csv: string): Promise<ContactImportResult | null> {
		try {
			const res = await api<ContactImportResult>('POST', '/contacts/import/linkedin-csv', {body: {csv}})
			await load()
			return res
		} catch (e) {
			toast(message(e))
			return null
		}
	}

	return {
		contacts,
		groups,
		tags,
		loaded,
		loading,
		photoVersions,
		indexed,
		tagNames,
		groupNames,
		load,
		read,
		create,
		update,
		remove,
		setGroup,
		setTag,
		saveGroup,
		removeGroup,
		removeTag,
		saveExperience,
		removeExperience,
		saveEducation,
		removeEducation,
		saveNote,
		removeNote,
		setPhoto,
		importLinkedinCsv,
	}
})
