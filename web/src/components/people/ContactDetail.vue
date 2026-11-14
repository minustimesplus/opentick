<script setup lang="ts">
import {computed, nextTick, reactive, ref, watch} from 'vue'
import {Briefcase, GraduationCap, Lock, Plus, StickyNote, Trash2, X} from 'lucide-vue-next'
import ContactAvatar from './ContactAvatar.vue'
import Popover from '@/components/Popover.vue'
import type {Contact, ContactEducation, ContactExperience, ContactField, ContactNote} from '@/api/types'
import {usePeople, type EducationInput, type ExperienceInput} from '@/stores/people'
import {contactSubtitle, nameColor} from '@/lib/people'
import {fmtDayMonth, parseDate} from '@/lib/dates'
import {t, type MessageKey} from '@/lib/i18n'
import {toast} from '@/lib/toast'

const props = defineProps<{contactId: number}>()
const emit = defineEmits<{close: [], deleted: []}>()
const people = usePeople()

const contact = ref<Contact | null>(null)
const loading = ref(false)

/** Keeps the pane if the user moved on to another contact while a request ran. */
function show(c: Contact | null) {
	if (c && c.id === props.contactId) contact.value = c
}

// ---------- fields ----------

interface FieldDef {
	key: ContactField
	label: MessageKey
	href?: (value: string) => string
	hint?: MessageKey
	long?: boolean
}

const FIELDS: FieldDef[] = [
	{key: 'headline', label: 'contactHeadline'},
	{key: 'company', label: 'contactCompany'},
	{key: 'title', label: 'contactTitle'},
	{key: 'location', label: 'contactLocation'},
	{key: 'email', label: 'email', href: v => `mailto:${v}`},
	{key: 'phone', label: 'contactPhone', href: v => `tel:${v}`},
	{key: 'linkedin_url', label: 'contactLinkedin', href: v => v},
	{key: 'birthday', label: 'contactBirthday', hint: 'contactBirthdayHint'},
	{key: 'about', label: 'about', long: true},
]

const editing = ref<ContactField | null>(null)
const draft = ref('')

function edit(key: ContactField) {
	if (!contact.value) return
	editing.value = key
	draft.value = contact.value[key]
}

async function commit() {
	const key = editing.value
	const c = contact.value
	if (!key || !c) return
	editing.value = null
	const value = draft.value.trim()
	if (value === c[key]) return
	show(await people.update(c.id, {[key]: value}))
}

function cancelEdit() {
	editing.value = null
}

function locked(key: string) {
	return !!contact.value?.locked_fields.includes(key)
}

async function unlock(key: string) {
	const c = contact.value
	if (!c) return
	show(await people.update(c.id, {locked_fields: c.locked_fields.filter(l => l !== key)}))
}

function shortUrl(url: string) {
	return url.replace(/^https?:\/\/(www\.)?/, '')
}

// ---------- photo ----------

const photoInput = ref<HTMLInputElement | null>(null)

async function pickPhoto(e: Event) {
	const input = e.target as HTMLInputElement
	const file = input.files?.[0]
	input.value = ''
	if (file && contact.value) show(await people.setPhoto(contact.value.id, file))
}

async function removePhoto() {
	if (contact.value) show(await people.setPhoto(contact.value.id, null))
}

// ---------- tags and groups ----------

const groupAnchor = ref<HTMLElement | null>(null)
const groupMenu = ref(false)
const tagDraft = ref('')
const addingTag = ref(false)
const tagInput = ref<HTMLInputElement | null>(null)

const contactTags = computed(() => people.tags.filter(tag => contact.value?.tag_ids.includes(tag.id)))
const contactGroups = computed(() => people.groups.filter(g => contact.value?.group_ids.includes(g.id)))

async function reload() {
	show(await people.read(props.contactId))
}

async function toggleGroup(groupId: number) {
	const c = contact.value
	if (!c) return
	if (await people.setGroup(groupId, [c.id], !c.group_ids.includes(groupId))) await reload()
}

async function startTag() {
	addingTag.value = true
	tagDraft.value = ''
	await nextTick()
	tagInput.value?.focus()
}

async function addTag() {
	const name = tagDraft.value.trim()
	const c = contact.value
	addingTag.value = false
	if (!name || !c) return
	if (await people.setTag(name, [c.id], true)) await reload()
}

async function removeTag(tagId: number) {
	const c = contact.value
	if (c && await people.setTag(tagId, [c.id], false)) await reload()
}

// ---------- experience, education ----------

type EntryKind = 'experience' | 'education'

const entry = ref<{kind: EntryKind, id?: number} | null>(null)
const exp = reactive<ExperienceInput>({company: '', title: '', location: '', start_date: '', end_date: '', description: ''})
const edu = reactive<EducationInput>({school: '', degree: '', field_of_study: '', start_year: '', end_year: ''})

function editExperience(e?: ContactExperience) {
	Object.assign(exp, {
		company: e?.company ?? '',
		title: e?.title ?? '',
		location: e?.location ?? '',
		start_date: e?.start_date ?? '',
		end_date: e?.end_date ?? '',
		description: e?.description ?? '',
	})
	entry.value = {kind: 'experience', id: e?.id}
}

function editEducation(e?: ContactEducation) {
	Object.assign(edu, {
		school: e?.school ?? '',
		degree: e?.degree ?? '',
		field_of_study: e?.field_of_study ?? '',
		start_year: e?.start_year ?? '',
		end_year: e?.end_year ?? '',
	})
	entry.value = {kind: 'education', id: e?.id}
}

async function saveEntry() {
	const current = entry.value
	const c = contact.value
	if (!current || !c) return
	if (current.kind === 'experience') {
		if (!exp.company.trim() && !exp.title.trim()) return
		const saved = await people.saveExperience(c.id, {...exp}, current.id)
		if (!saved) return
		show(saved)
	} else {
		if (!edu.school.trim()) return
		const saved = await people.saveEducation(c.id, {...edu}, current.id)
		if (!saved) return
		show(saved)
	}
	if (entry.value === current) entry.value = null
}

async function removeEntry() {
	const current = entry.value
	const c = contact.value
	if (!current?.id || !c) return
	const saved = current.kind === 'experience'
		? await people.removeExperience(c.id, current.id)
		: await people.removeEducation(c.id, current.id)
	if (!saved) return
	show(saved)
	if (entry.value === current) entry.value = null
}

function years(a: string, b: string, open = '') {
	if (a && b) return `${a} – ${b}`
	if (a) return `${a} – ${open}`.trim()
	return b
}

// ---------- notes ----------

const noteDraft = ref('')
const editingNote = ref<ContactNote | null>(null)
const editNoteDraft = ref('')

async function addNote() {
	const body = noteDraft.value.trim()
	const c = contact.value
	if (!body || !c) return
	const saved = await people.saveNote(c.id, body)
	if (!saved) return
	noteDraft.value = ''
	show(saved)
}

function startNote(n: ContactNote) {
	editingNote.value = n
	editNoteDraft.value = n.body
}

async function saveNote() {
	const note = editingNote.value
	const c = contact.value
	if (!note || !c) return
	const body = editNoteDraft.value.trim()
	const saved = body ? await people.saveNote(c.id, body, note) : await people.removeNote(c.id, note.id)
	if (!saved) return
	show(saved)
	if (editingNote.value === note) editingNote.value = null
}

async function removeNote(n: ContactNote) {
	if (contact.value) show(await people.removeNote(contact.value.id, n.id))
}

function noteDate(n: ContactNote) {
	const d = parseDate(n.created)
	return d ? fmtDayMonth(d) : ''
}

// ---------- contact ----------

const SOURCES: Record<string, MessageKey> = {
	manual: 'contactSourceManual',
	csv: 'contactSourceCsv',
	linkedin: 'contactSourceLinkedin',
	brightdata: 'contactSourceBrightdata',
}

async function remove() {
	const c = contact.value
	if (!c || !confirm(t('deleteContactConfirm', {name: c.name}))) return
	if (await people.remove([c.id])) {
		toast(t('contactDeleted'))
		emit('deleted')
	}
}

watch(() => props.contactId, async id => {
	contact.value = null
	editing.value = null
	entry.value = null
	loading.value = true
	const c = await people.read(id)
	// Another contact may have been opened while this one loaded.
	if (props.contactId !== id) return
	contact.value = c
	loading.value = false
	if (!c) emit('close')
}, {immediate: true})
</script>

<template>
	<aside class="detail">
		<div v-if="!contact" class="empty-state muted">{{ loading ? '' : t('contactSelect') }}</div>
		<template v-else>
			<header class="top">
				<button class="icon-btn" :title="t('close')" @click="emit('close')"><X :size="18" /></button>
				<div class="spacer" />
				<button class="icon-btn danger" :title="t('delete')" @click="remove"><Trash2 :size="17" /></button>
			</header>

			<div class="scroll">
				<div class="identity">
					<button class="photo" :title="t('contactChangePhoto')" @click="photoInput?.click()">
						<ContactAvatar :contact="contact" :size="72" />
					</button>
					<input ref="photoInput" type="file" accept="image/png,image/jpeg,image/gif" hidden @change="pickPhoto">
					<div class="who">
						<input
							v-if="editing === 'full_name'"
							v-focus
							v-model="draft"
							class="input name-input"
							maxlength="250"
							@keydown.enter.prevent="commit"
							@keydown.esc.stop="cancelEdit"
							@blur="commit"
						>
						<button v-else class="name" @click="edit('full_name')">{{ contact.name }}</button>
						<div v-if="contactSubtitle(contact)" class="sub">{{ contactSubtitle(contact) }}</div>
						<button v-if="contact.has_photo" class="link small" @click="removePhoto">{{ t('contactRemovePhoto') }}</button>
					</div>
				</div>

				<div class="chips">
					<span
						v-for="g in contactGroups"
						:key="'g' + g.id"
						class="chip"
						:style="{'--chip': nameColor(g.name, g.hex_color)}"
					>
						{{ g.name }}
						<button :title="t('remove')" @click="toggleGroup(g.id)"><X :size="12" /></button>
					</span>
					<span v-for="tag in contactTags" :key="'t' + tag.id" class="chip tag">
						#{{ tag.name }}
						<button :title="t('remove')" @click="removeTag(tag.id)"><X :size="12" /></button>
					</span>
					<button ref="groupAnchor" class="chip add" @click="groupMenu = true"><Plus :size="12" /> {{ t('contactGroups') }}</button>
					<input
						v-if="addingTag"
						ref="tagInput"
						v-model="tagDraft"
						class="chip-input"
						:placeholder="t('tagNamePlaceholder')"
						maxlength="250"
						@keydown.enter.prevent="addTag"
						@keydown.esc.stop="addingTag = false"
						@blur="addTag"
					>
					<button v-else class="chip add" @click="startTag"><Plus :size="12" /> {{ t('addTag') }}</button>
				</div>

				<section class="fields">
					<div v-for="f in FIELDS" :key="f.key" class="field" :class="{long: f.long}">
						<span class="label">
							{{ t(f.label) }}
							<button v-if="locked(f.key)" class="lock" :title="t('contactLocked')" @click="unlock(f.key)">
								<Lock :size="11" />
							</button>
						</span>
						<template v-if="editing === f.key">
							<textarea
								v-if="f.long"
								v-focus
								v-model="draft"
								class="input area"
								rows="5"
								@keydown.esc.stop="cancelEdit"
								@blur="commit"
							/>
							<input
								v-else
								v-focus
								v-model="draft"
								class="input"
								:placeholder="f.hint ? t(f.hint) : ''"
								@keydown.enter.prevent="commit"
								@keydown.esc.stop="cancelEdit"
								@blur="commit"
							>
						</template>
						<div v-else class="value-row">
							<button class="value" :class="{empty: !contact[f.key]}" @click="edit(f.key)">
								{{ contact[f.key] ? (f.key === 'linkedin_url' ? shortUrl(contact[f.key]) : contact[f.key]) : t('contactAddField', {field: t(f.label).toLowerCase()}) }}
							</button>
							<a
								v-if="f.href && contact[f.key]"
								class="open"
								:href="f.href(contact[f.key])"
								target="_blank"
								rel="noopener noreferrer"
							>↗</a>
						</div>
					</div>
				</section>

				<section class="block">
					<h4>
						<Briefcase :size="15" /> {{ t('contactExperience') }}
						<button v-if="locked('experiences')" class="lock" :title="t('contactLocked')" @click="unlock('experiences')">
							<Lock :size="11" />
						</button>
						<span class="spacer" />
						<button class="icon-btn small" :title="t('contactAddExperience')" @click="editExperience()"><Plus :size="16" /></button>
					</h4>
					<form v-if="entry?.kind === 'experience'" class="entry-form" @submit.prevent="saveEntry">
						<input v-model="exp.title" v-focus class="input" :placeholder="t('contactTitle')" maxlength="250">
						<input v-model="exp.company" class="input" :placeholder="t('contactCompany')" maxlength="250">
						<input v-model="exp.location" class="input" :placeholder="t('contactLocation')" maxlength="250">
						<div class="pair">
							<input v-model="exp.start_date" class="input" :placeholder="t('contactStart')" maxlength="50">
							<input v-model="exp.end_date" class="input" :placeholder="t('contactEnd')" maxlength="50">
						</div>
						<textarea v-model="exp.description" class="input area" rows="3" :placeholder="t('description')" />
						<div class="entry-actions">
							<button v-if="entry.id" type="button" class="btn danger" @click="removeEntry">{{ t('delete') }}</button>
							<span class="spacer" />
							<button type="button" class="btn" @click="entry = null">{{ t('cancel') }}</button>
							<button type="submit" class="btn primary">{{ t('save') }}</button>
						</div>
					</form>
					<div v-if="!contact.experiences.length && entry?.kind !== 'experience'" class="none">{{ t('contactNoHistory') }}</div>
					<button
						v-for="e in contact.experiences"
						v-show="!(entry?.kind === 'experience' && entry.id === e.id)"
						:key="e.id"
						class="entry"
						@click="editExperience(e)"
					>
						<span class="entry-title">{{ e.title || e.company }}</span>
						<span v-if="e.title && e.company" class="entry-sub">{{ e.company }}</span>
						<span class="entry-meta">
							{{ [years(e.start_date, e.end_date, t('contactPresent')), e.location].filter(Boolean).join(' · ') }}
						</span>
						<span v-if="e.description" class="entry-desc">{{ e.description }}</span>
					</button>
				</section>

				<section class="block">
					<h4>
						<GraduationCap :size="15" /> {{ t('contactEducation') }}
						<button v-if="locked('educations')" class="lock" :title="t('contactLocked')" @click="unlock('educations')">
							<Lock :size="11" />
						</button>
						<span class="spacer" />
						<button class="icon-btn small" :title="t('contactAddEducation')" @click="editEducation()"><Plus :size="16" /></button>
					</h4>
					<form v-if="entry?.kind === 'education'" class="entry-form" @submit.prevent="saveEntry">
						<input v-model="edu.school" v-focus class="input" :placeholder="t('contactSchool')" maxlength="250">
						<input v-model="edu.degree" class="input" :placeholder="t('contactDegree')" maxlength="250">
						<input v-model="edu.field_of_study" class="input" :placeholder="t('contactFieldOfStudy')" maxlength="250">
						<div class="pair">
							<input v-model="edu.start_year" class="input" :placeholder="t('contactStart')" maxlength="20">
							<input v-model="edu.end_year" class="input" :placeholder="t('contactEnd')" maxlength="20">
						</div>
						<div class="entry-actions">
							<button v-if="entry.id" type="button" class="btn danger" @click="removeEntry">{{ t('delete') }}</button>
							<span class="spacer" />
							<button type="button" class="btn" @click="entry = null">{{ t('cancel') }}</button>
							<button type="submit" class="btn primary">{{ t('save') }}</button>
						</div>
					</form>
					<div v-if="!contact.educations.length && entry?.kind !== 'education'" class="none">{{ t('contactNoHistory') }}</div>
					<button
						v-for="e in contact.educations"
						v-show="!(entry?.kind === 'education' && entry.id === e.id)"
						:key="e.id"
						class="entry"
						@click="editEducation(e)"
					>
						<span class="entry-title">{{ e.school }}</span>
						<span v-if="e.degree || e.field_of_study" class="entry-sub">
							{{ [e.degree, e.field_of_study].filter(Boolean).join(', ') }}
						</span>
						<span v-if="e.start_year || e.end_year" class="entry-meta">{{ years(e.start_year, e.end_year) }}</span>
					</button>
				</section>

				<section class="block">
					<h4><StickyNote :size="15" /> {{ t('contactNotes') }}</h4>
					<form class="note-form" @submit.prevent="addNote">
						<textarea
							v-model="noteDraft"
							class="input area"
							rows="2"
							:placeholder="t('contactNotePlaceholder')"
							@keydown.enter.ctrl.prevent="addNote"
							@keydown.enter.meta.prevent="addNote"
						/>
						<button v-if="noteDraft.trim()" type="submit" class="btn primary">{{ t('contactAddNote') }}</button>
					</form>
					<div v-for="n in contact.notes" :key="n.id" class="note">
						<template v-if="editingNote?.id === n.id">
							<textarea
								v-model="editNoteDraft"
								v-focus
								class="input area"
								rows="3"
								@keydown.esc.stop="editingNote = null"
							/>
							<div class="entry-actions">
								<span class="spacer" />
								<button type="button" class="btn" @click="editingNote = null">{{ t('cancel') }}</button>
								<button type="button" class="btn primary" @click="saveNote">{{ t('save') }}</button>
							</div>
						</template>
						<template v-else>
							<button class="note-body" @click="startNote(n)">{{ n.body }}</button>
							<div class="note-foot">
								<span>{{ noteDate(n) }}</span>
								<button class="link" @click="removeNote(n)">{{ t('delete') }}</button>
							</div>
						</template>
					</div>
				</section>

				<div class="source">{{ t('contactSource', {source: t(SOURCES[contact.source] ?? 'contactSourceManual')}) }}</div>
			</div>

			<Popover v-if="groupMenu" :anchor="groupAnchor" @close="groupMenu = false">
				<div class="menu group-menu">
					<div v-if="!people.groups.length" class="menu-label">{{ t('newGroup') }}…</div>
					<button
						v-for="g in people.groups"
						:key="g.id"
						class="menu-item"
						:class="{active: contact.group_ids.includes(g.id)}"
						@click="toggleGroup(g.id)"
					>
						<span class="dot" :style="{background: nameColor(g.name, g.hex_color)}" />
						{{ g.name }}
						<span v-if="contact.group_ids.includes(g.id)" class="menu-chevron">✓</span>
					</button>
				</div>
			</Popover>
		</template>
	</aside>
</template>

<style scoped>
.detail {
	display: flex;
	flex-direction: column;
	width: 400px;
	flex-shrink: 0;
	min-height: 0;
	border-left: 1px solid var(--border);
	background: var(--bg);
}

.top {
	display: flex;
	align-items: center;
	height: 48px;
	padding: 0 10px;
	flex-shrink: 0;
}

.spacer {
	flex: 1;
}

.icon-btn.danger:hover {
	color: var(--danger);
}

.icon-btn.small {
	width: 26px;
	height: 26px;
}

.scroll {
	flex: 1;
	overflow: auto;
	padding: 0 20px 28px;
}

.identity {
	display: flex;
	align-items: center;
	gap: 16px;
}

.photo {
	display: flex;
	border-radius: 50%;
}

.photo:hover {
	filter: brightness(0.92);
}

.who {
	display: flex;
	flex-direction: column;
	align-items: flex-start;
	gap: 3px;
	min-width: 0;
	flex: 1;
}

.name {
	max-width: 100%;
	overflow: hidden;
	text-overflow: ellipsis;
	font-size: 20px;
	font-weight: 700;
	text-align: left;
}

.name-input {
	font-size: 17px;
	font-weight: 600;
}

.sub {
	font-size: 13px;
	color: var(--text-2);
}

.link {
	color: var(--text-3);
	font-size: 12px;
}

.link:hover {
	color: var(--accent);
}

.chips {
	display: flex;
	flex-wrap: wrap;
	gap: 6px;
	margin: 16px 0 6px;
}

.chip {
	--chip: var(--text-3);
	display: inline-flex;
	align-items: center;
	gap: 4px;
	height: 24px;
	padding: 0 8px;
	border-radius: 12px;
	background: color-mix(in srgb, var(--chip) 16%, transparent);
	color: color-mix(in srgb, var(--chip) 70%, var(--text));
	font-size: 12px;
	white-space: nowrap;
}

.chip button {
	display: flex;
	opacity: 0.6;
}

.chip button:hover {
	opacity: 1;
}

.chip.add {
	border: 1px dashed var(--border-strong);
	background: none;
	color: var(--text-3);
}

.chip.add:hover {
	color: var(--text);
}

.chip-input {
	width: 120px;
	height: 24px;
	padding: 0 8px;
	border: 1px solid var(--accent);
	border-radius: 12px;
	background: none;
	outline: none;
	font-size: 12px;
}

.fields {
	display: flex;
	flex-direction: column;
	margin-top: 12px;
}

.field {
	display: grid;
	grid-template-columns: 92px 1fr;
	align-items: center;
	min-height: 34px;
	column-gap: 8px;
}

.field.long {
	align-items: start;
	padding-top: 7px;
}

.label {
	display: flex;
	align-items: center;
	gap: 5px;
	font-size: 12px;
	color: var(--text-3);
}

.lock {
	display: flex;
	color: var(--text-4);
}

.lock:hover {
	color: var(--accent);
}

.value-row {
	display: flex;
	align-items: center;
	min-width: 0;
}

.value {
	flex: 1;
	min-width: 0;
	padding: 6px 8px;
	border-radius: 6px;
	text-align: left;
	white-space: pre-wrap;
	overflow-wrap: anywhere;
}

.value:hover {
	background: var(--bg-hover);
}

.value.empty {
	color: var(--text-4);
}

.open {
	padding: 4px 6px;
	color: var(--text-3);
	text-decoration: none;
}

.open:hover {
	color: var(--accent);
}

.area {
	height: auto;
	padding: 8px 10px;
	resize: vertical;
	font: inherit;
}

.block {
	margin-top: 22px;
}

h4 {
	display: flex;
	align-items: center;
	gap: 7px;
	margin: 0 0 6px;
	font-size: 13px;
	font-weight: 600;
	color: var(--text-2);
}

.none {
	padding: 4px 0;
	font-size: 13px;
	color: var(--text-4);
}

.entry {
	display: flex;
	flex-direction: column;
	gap: 2px;
	width: 100%;
	padding: 8px;
	margin: 0 -8px;
	border-radius: 8px;
	text-align: left;
}

.entry:hover {
	background: var(--bg-hover);
}

.entry-title {
	font-weight: 600;
}

.entry-sub {
	font-size: 13px;
	color: var(--text-2);
}

.entry-meta {
	font-size: 12px;
	color: var(--text-3);
}

.entry-desc {
	margin-top: 3px;
	font-size: 13px;
	color: var(--text-2);
	white-space: pre-wrap;
}

.entry-form,
.note-form {
	display: flex;
	flex-direction: column;
	gap: 6px;
	margin-bottom: 8px;
}

.note-form .btn {
	align-self: flex-end;
}

.pair {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 6px;
}

.entry-actions {
	display: flex;
	gap: 8px;
	margin-top: 2px;
}

.btn.danger {
	border-color: transparent;
	color: var(--danger);
}

.note {
	padding: 10px 12px;
	margin-top: 8px;
	border-radius: 8px;
	background: var(--bg-panel);
}

.note-body {
	width: 100%;
	text-align: left;
	white-space: pre-wrap;
	overflow-wrap: anywhere;
}

.note-foot {
	display: flex;
	justify-content: space-between;
	margin-top: 6px;
	font-size: 12px;
	color: var(--text-3);
}

.source {
	margin-top: 24px;
	font-size: 12px;
	color: var(--text-4);
}

.group-menu {
	max-height: 320px;
	overflow: auto;
}

.dot {
	width: 10px;
	height: 10px;
	border-radius: 50%;
	flex-shrink: 0;
}

@media (max-width: 767px) {
	.detail {
		width: 100%;
		border-left: none;
	}
}
</style>
