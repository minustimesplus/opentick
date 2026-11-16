<script setup lang="ts">
import {computed, nextTick, onMounted, ref, watch} from 'vue'
import {storeToRefs} from 'pinia'
import {useRoute, useRouter} from 'vue-router'
import {
	ArrowDownUp,
	ChartPie,
	Check,
	ChevronDown,
	ChevronLeft,
	ChevronRight,
	Hash,
	PanelLeft,
	Pencil,
	Plus,
	Search,
	Tag,
	Trash2,
	Upload,
	Users,
	UserMinus,
	X,
} from 'lucide-vue-next'
import ContactAvatar from '@/components/people/ContactAvatar.vue'
import ContactDetail from '@/components/people/ContactDetail.vue'
import ContactDialog from '@/components/people/ContactDialog.vue'
import GroupDialog from '@/components/people/GroupDialog.vue'
import PeopleImport from '@/components/people/PeopleImport.vue'
import PeopleStats from '@/components/people/PeopleStats.vue'
import Popover from '@/components/Popover.vue'
import type {ContactGroup, ContactListItem} from '@/api/types'
import {usePeople} from '@/stores/people'
import {contactSubtitle, filterContacts, nameColor, sortContacts, type ContactSort} from '@/lib/people'
import {t, type MessageKey} from '@/lib/i18n'
import {prefs} from '@/lib/prefs'
import {clamp, startResize} from '@/lib/resize'
import {toast} from '@/lib/toast'
import {isMobile} from '@/lib/viewport'

const people = usePeople()
const route = useRoute()
const router = useRouter()
const {query, sort, selected} = storeToRefs(people)

const SIDEBAR_RANGE = [200, 420] as const
const DETAIL_RANGE = [320, 720] as const

onMounted(async () => {
	if (!people.loaded) await people.load()
	await nextTick()
	const el = scroller.value
	if (el && people.listScroll.list === list.value) {
		el.scrollTop = people.listScroll.top
		onScroll()
	}
})

// ---------- which list ----------

// While leaving for another view the route changes before this view unmounts;
// holding on to the last list keeps that from resetting the state kept in the store.
const list = computed<string>(prev => route.name === 'people' ? String(route.params.list || 'all') : prev ?? 'all')
const groupId = computed(() => /^g\d+$/.test(list.value) ? Number(list.value.slice(1)) : undefined)
const tagId = computed(() => /^t\d+$/.test(list.value) ? Number(list.value.slice(1)) : undefined)
const group = computed(() => people.groups.find(g => g.id === groupId.value))
const tag = computed(() => people.tags.find(x => x.id === tagId.value))
const pane = computed<'contacts' | 'stats' | 'import'>(() => list.value === 'stats' || list.value === 'import' ? list.value : 'contacts')
const contactId = computed(() => {
	if (route.name !== 'people') return undefined
	const id = Number(route.params.contactId)
	return pane.value !== 'import' && Number.isInteger(id) && id > 0 ? id : undefined
})

const heading = computed(() => {
	if (pane.value === 'stats') return t('peopleStats')
	if (pane.value === 'import') return t('importContacts')
	return group.value?.name ?? (tag.value ? '#' + tag.value.name : t('allContacts'))
})

// A group or tag that was deleted (here or elsewhere) has no list to show.
watch(() => [people.loaded, group.value, tag.value] as const, () => {
	if (!people.loaded) return
	if ((groupId.value !== undefined && !group.value) || (tagId.value !== undefined && !tag.value)) router.replace('/people')
})

function openList(name: string) {
	router.push(name === 'all' ? '/people' : `/people/${name}`)
}

function openContact(id?: number) {
	router.push(id ? `/people/${list.value}/${id}` : (list.value === 'all' ? '/people' : `/people/${list.value}`))
}

// On a phone the sidebar, the list and the contact are one screen each.
const showSidebar = computed(() => isMobile.value ? !route.params.list && !contactId.value : prefs.peopleSidebar)
const showMain = computed(() => !isMobile.value || (!!route.params.list && !contactId.value))

// ---------- the list ----------

const sortAnchor = ref<HTMLElement | null>(null)
const sortMenu = ref(false)

const SORTS: [ContactSort, MessageKey][] = [
	['name', 'contactSortName'],
	['company', 'contactSortCompany'],
	['location', 'contactSortLocation'],
	['recent', 'contactSortRecent'],
]

const shown = computed(() => sortContacts(filterContacts(people.indexed, {
	query: query.value,
	groupId: groupId.value,
	tagId: tagId.value,
}), sort.value))

// Only the rows in view are rendered: lists run into the thousands.
const ROW_HEIGHT = 60
const OVERSCAN = 8
const scroller = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const viewHeight = ref(800)

function onScroll() {
	const el = scroller.value
	if (!el) return
	scrollTop.value = el.scrollTop
	viewHeight.value = el.clientHeight
	people.listScroll = {list: list.value, top: el.scrollTop}
}

const firstRow = computed(() => Math.max(0, Math.floor(scrollTop.value / ROW_HEIGHT) - OVERSCAN))
const visible = computed(() => {
	const count = Math.ceil(viewHeight.value / ROW_HEIGHT) + OVERSCAN * 2
	return shown.value.slice(firstRow.value, firstRow.value + count)
})

watch([list, query, sort], () => {
	scroller.value?.scrollTo({top: 0})
	scrollTop.value = 0
	people.listScroll = {list: list.value, top: 0}
})

// ---------- selection ----------

let lastToggled: number | null = null

const selectedIds = computed(() => shown.value.filter(c => selected.value.has(c.id)).map(c => c.id))

watch(list, () => {
	selected.value = new Set()
	lastToggled = null
})

function toggle(c: ContactListItem, e: MouseEvent) {
	const next = new Set(selected.value)
	const from = lastToggled === null ? -1 : shown.value.findIndex(x => x.id === lastToggled)
	const to = shown.value.findIndex(x => x.id === c.id)
	if (e.shiftKey && from >= 0 && to >= 0) {
		for (const x of shown.value.slice(Math.min(from, to), Math.max(from, to) + 1)) next.add(x.id)
	} else if (next.has(c.id)) {
		next.delete(c.id)
	} else {
		next.add(c.id)
	}
	lastToggled = c.id
	selected.value = next
}

function selectAll() {
	selected.value = new Set(shown.value.map(c => c.id))
}

function clearSelection() {
	selected.value = new Set()
}

function onRowClick(c: ContactListItem, e: MouseEvent) {
	if (e.shiftKey || e.metaKey || e.ctrlKey || selected.value.size) toggle(c, e)
	else openContact(c.id)
}

// ---------- bulk actions ----------

const groupAnchor = ref<HTMLElement | null>(null)
const bulkMenu = ref<'group' | 'tag' | null>(null)
const tagDraft = ref('')
const busy = ref(false)

async function bulk(run: (ids: number[]) => Promise<unknown>) {
	const ids = selectedIds.value
	if (!ids.length || busy.value) return
	busy.value = true
	bulkMenu.value = null
	try {
		await run(ids)
	} finally {
		busy.value = false
	}
}

function addToGroup(g: ContactGroup) {
	return bulk(async ids => {
		const res = await people.setGroup(g.id, ids, true)
		if (res) {
			toast(t('contactsAddedToGroup', {n: res.changed, name: g.name}))
			clearSelection()
		}
	})
}

function removeFromGroup() {
	const g = group.value
	if (!g) return
	return bulk(async ids => {
		if (await people.setGroup(g.id, ids, false)) clearSelection()
	})
}

function addTag() {
	const name = tagDraft.value.trim()
	if (!name) return
	return bulk(async ids => {
		const res = await people.setTag(name, ids, true)
		if (res) {
			toast(t('contactsTagged', {n: res.changed, name: res.target.name}))
			tagDraft.value = ''
			clearSelection()
		}
	})
}

function deleteSelected() {
	if (!confirm(t('deleteContactsConfirm', {n: selectedIds.value.length}))) return
	return bulk(async ids => {
		const open = contactId.value
		if (await people.remove(ids)) {
			toast(t('contactsDeleted', {n: ids.length}))
			clearSelection()
			if (open && ids.includes(open) && contactId.value === open) openContact()
		}
	})
}

// ---------- dialogs ----------

const addingContact = ref(false)
const groupDialog = ref<{group: ContactGroup | null} | null>(null)

function toggleSection(key: string) {
	prefs.collapsed[key] = !prefs.collapsed[key]
}

async function deleteTag() {
	const current = tag.value
	if (!current || !confirm(t('deleteTagConfirm', {name: current.name}))) return
	if (await people.removeTag(current.id) && tagId.value === current.id) router.replace('/people')
}
</script>

<template>
	<div class="people-view">
		<aside
			v-if="showSidebar"
			class="sidebar"
			:style="isMobile ? undefined : {width: clamp(prefs.peopleSidebarWidth, SIDEBAR_RANGE) + 'px'}"
		>
			<div class="scroll">
				<button class="item" :class="{active: list === 'all' && !isMobile}" @click="router.push('/people/all')">
					<Users :size="18" />
					<span class="grow">{{ t('allContacts') }}</span>
					<span class="count">{{ people.contacts.length || '' }}</span>
				</button>
				<button class="item" :class="{active: list === 'stats'}" @click="openList('stats')">
					<ChartPie :size="18" />
					<span class="grow">{{ t('peopleStats') }}</span>
				</button>
				<button class="item" :class="{active: list === 'import'}" @click="openList('import')">
					<Upload :size="18" />
					<span class="grow">{{ t('import') }}</span>
				</button>

				<div
					class="section"
					role="button"
					tabindex="0"
					:aria-expanded="!prefs.collapsed.peopleGroups"
					@click="toggleSection('peopleGroups')"
					@keydown.enter.self="toggleSection('peopleGroups')"
				>
					<span class="grow">{{ t('contactGroups') }}</span>
					<button class="icon-btn small" :title="t('newGroup')" @click.stop="groupDialog = {group: null}">
						<Plus :size="16" />
					</button>
					<component :is="prefs.collapsed.peopleGroups ? ChevronRight : ChevronDown" :size="14" class="chev" />
				</div>
				<template v-if="!prefs.collapsed.peopleGroups">
					<button
						v-for="g in people.groups"
						:key="g.id"
						class="item"
						:class="{active: groupId === g.id}"
						@click="openList('g' + g.id)"
					>
						<span class="dot" :style="{background: nameColor(g.name, g.hex_color)}" />
						<span class="grow">{{ g.name }}</span>
						<span class="count">{{ g.members || '' }}</span>
					</button>
				</template>

				<template v-if="people.tags.length">
					<div
						class="section"
						role="button"
						tabindex="0"
						:aria-expanded="!prefs.collapsed.peopleTags"
						@click="toggleSection('peopleTags')"
						@keydown.enter.self="toggleSection('peopleTags')"
					>
						<span class="grow">{{ t('tags') }}</span>
						<component :is="prefs.collapsed.peopleTags ? ChevronRight : ChevronDown" :size="14" class="chev" />
					</div>
					<template v-if="!prefs.collapsed.peopleTags">
						<button
							v-for="x in people.tags"
							:key="x.id"
							class="item"
							:class="{active: tagId === x.id}"
							@click="openList('t' + x.id)"
						>
							<Hash :size="16" class="hash" />
							<span class="grow">{{ x.name }}</span>
							<span class="count">{{ x.contacts || '' }}</span>
						</button>
					</template>
				</template>
			</div>
		</aside>
		<div
			v-if="showSidebar && !isMobile"
			class="resizer"
			@pointerdown="startResize($event, 'peopleSidebarWidth', SIDEBAR_RANGE, 1)"
		/>

		<section v-if="showMain" class="main-pane">
			<header class="head">
				<button v-if="isMobile" class="icon-btn" :title="t('people')" @click="router.push('/people')"><ChevronLeft :size="20" /></button>
				<button
					v-else
					class="icon-btn"
					:title="t('peopleToggleSidebar')"
					@click="prefs.peopleSidebar = !prefs.peopleSidebar"
				>
					<PanelLeft :size="19" />
				</button>
				<h1>{{ heading }}</h1>
				<template v-if="pane === 'contacts'">
					<span class="total">{{ t('contactsCount', {n: shown.length}) }}</span>
					<span class="spacer" />
					<button v-if="group" class="icon-btn" :title="t('editGroup')" @click="groupDialog = {group}"><Pencil :size="17" /></button>
					<button v-if="tag" class="icon-btn" :title="t('delete')" @click="deleteTag"><Trash2 :size="17" /></button>
					<button ref="sortAnchor" class="icon-btn" :title="t('sortBy')" @click="sortMenu = true"><ArrowDownUp :size="17" /></button>
					<button class="icon-btn" :title="t('newContact')" @click="addingContact = true"><Plus :size="20" /></button>
				</template>
			</header>

			<PeopleImport v-if="pane === 'import'" />

			<PeopleStats v-else-if="pane === 'stats'" :open-id="contactId" @open="openContact" />

			<template v-else>
				<div v-if="selectedIds.length" class="bulk">
					<button class="icon-btn" :title="t('clearSelection')" @click="clearSelection"><X :size="17" /></button>
					<span class="bulk-count">{{ t('contactsSelected', {n: selectedIds.length}) }}</span>
					<button class="link" @click="selectAll">{{ t('selectAll') }}</button>
					<span class="spacer" />
					<button ref="groupAnchor" class="btn" :aria-disabled="busy" @click="bulkMenu = 'group'">
						<Users :size="15" /> {{ t('addToGroup') }}
					</button>
					<button class="btn" :aria-disabled="busy" @click="bulkMenu = 'tag'"><Tag :size="15" /> {{ t('addTag') }}</button>
					<button v-if="group" class="btn" :aria-disabled="busy" :title="t('removeFromGroup')" @click="removeFromGroup">
						<UserMinus :size="15" />
					</button>
					<button class="btn danger" :aria-disabled="busy" :title="t('delete')" @click="deleteSelected"><Trash2 :size="15" /></button>
				</div>
				<div v-else class="search">
					<Search :size="16" />
					<input v-model="query" :placeholder="t('searchContacts')" type="search">
				</div>

				<div v-if="people.loaded && !people.contacts.length" class="empty-state">
					<div class="big-emoji">👋</div>
					<div>{{ t('noContacts') }}</div>
					<div class="sub">{{ t('noContactsHint') }}</div>
					<div class="empty-actions">
						<button class="btn primary" @click="addingContact = true"><Plus :size="16" /> {{ t('newContact') }}</button>
						<button class="btn" @click="openList('import')"><Upload :size="15" /> {{ t('import') }}</button>
					</div>
				</div>
				<div v-else-if="people.loaded && !shown.length" class="empty-state">
					<div>{{ t('noContactsFound') }}</div>
				</div>

				<div v-else ref="scroller" class="rows" @scroll.passive="onScroll">
					<div :style="{height: `${shown.length * ROW_HEIGHT}px`, paddingTop: `${firstRow * ROW_HEIGHT}px`}">
						<div
							v-for="c in visible"
							:key="c.id"
							class="row"
							:class="{active: c.id === contactId, selected: selected.has(c.id), selecting: selected.size > 0}"
							@click="onRowClick(c, $event)"
						>
							<button class="pick" :title="t('selectAll')" @click.stop="toggle(c, $event)">
								<ContactAvatar :contact="c" :size="38" />
								<span class="tick"><Check :size="18" :stroke-width="3" /></span>
							</button>
							<div class="info">
								<div class="name">{{ c.name }}</div>
								<div class="sub">{{ contactSubtitle(c) }}</div>
							</div>
							<div class="where">{{ c.location }}</div>
						</div>
					</div>
				</div>
			</template>
		</section>

		<template v-if="contactId">
			<div
				v-if="!isMobile && prefs.peopleDetail"
				class="resizer"
				@pointerdown="startResize($event, 'peopleDetailWidth', DETAIL_RANGE, -1)"
			/>
			<ContactDetail
				:contact-id="contactId"
				:collapsed="!isMobile && !prefs.peopleDetail"
				:style="isMobile || !prefs.peopleDetail ? undefined : {width: clamp(prefs.peopleDetailWidth, DETAIL_RANGE) + 'px'}"
				@close="openContact()"
				@deleted="openContact()"
				@collapse="prefs.peopleDetail = !prefs.peopleDetail"
			/>
		</template>

		<Popover v-if="sortMenu" :anchor="sortAnchor" placement="bottom-end" @close="sortMenu = false">
			<div class="menu">
				<div class="menu-label">{{ t('sortBy') }}</div>
				<button
					v-for="[key, label] in SORTS"
					:key="key"
					class="menu-item"
					:class="{active: sort === key}"
					@click="sort = key; sortMenu = false"
				>
					{{ t(label) }}
					<span v-if="sort === key" class="menu-chevron">✓</span>
				</button>
			</div>
		</Popover>

		<Popover v-if="bulkMenu === 'group'" :anchor="groupAnchor" @close="bulkMenu = null">
			<div class="menu pick-menu">
				<button v-for="g in people.groups" :key="g.id" class="menu-item" @click="addToGroup(g)">
					<span class="dot" :style="{background: nameColor(g.name, g.hex_color)}" />
					{{ g.name }}
				</button>
				<div v-if="people.groups.length" class="menu-sep" />
				<button class="menu-item accent" @click="bulkMenu = null; groupDialog = {group: null}">
					<Plus :size="15" /> {{ t('newGroup') }}
				</button>
			</div>
		</Popover>

		<Popover v-if="bulkMenu === 'tag'" :anchor="groupAnchor" @close="bulkMenu = null">
			<form class="menu tag-form" @submit.prevent="addTag">
				<input v-model="tagDraft" v-focus class="input" :placeholder="t('tagNamePlaceholder')" maxlength="250" list="people-tags">
				<datalist id="people-tags">
					<option v-for="x in people.tags" :key="x.id" :value="x.name" />
				</datalist>
				<button type="submit" class="btn primary">{{ t('addTag') }}</button>
			</form>
		</Popover>

		<ContactDialog
			v-if="addingContact"
			:group-id="groupId"
			@close="addingContact = false"
			@created="openContact($event.id)"
		/>
		<GroupDialog
			v-if="groupDialog"
			:group="groupDialog.group"
			@close="groupDialog = null"
			@saved="selectedIds.length && !groupDialog?.group ? addToGroup($event) : undefined"
		/>
	</div>
</template>

<style scoped>
.people-view {
	display: flex;
	height: 100%;
	min-width: 0;
}

.sidebar {
	display: flex;
	flex-direction: column;
	flex-shrink: 0;
	border-right: 1px solid var(--border);
	background: var(--bg);
}

.scroll {
	flex: 1;
	overflow: auto;
	padding: 14px 10px;
}

.item {
	display: flex;
	align-items: center;
	gap: 12px;
	width: 100%;
	height: 40px;
	padding: 0 12px;
	border-radius: 8px;
	text-align: left;
	color: var(--text);
	font-size: 15px;
}

.item:hover {
	background: var(--bg-hover);
}

.item.active {
	background: var(--bg-active);
}

.item svg {
	flex-shrink: 0;
	color: var(--text-2);
}

.grow {
	flex: 1;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.count {
	font-size: 13px;
	color: var(--text-3);
	min-width: 18px;
	text-align: right;
}

.dot {
	width: 10px;
	height: 10px;
	margin: 0 4px;
	border-radius: 50%;
	flex-shrink: 0;
}

.item .hash {
	margin: 0 1px;
	color: var(--text-3);
}

.section {
	display: flex;
	align-items: center;
	gap: 4px;
	height: 30px;
	margin-top: 14px;
	padding: 0 8px 0 12px;
	border-radius: 6px;
	font-size: 12px;
	color: var(--text-3);
	cursor: pointer;
	user-select: none;
}

.section:hover {
	background: var(--bg-hover);
}

.section .chev {
	flex-shrink: 0;
	color: var(--text-4);
}

.icon-btn.small {
	width: 26px;
	height: 26px;
}

.main-pane {
	display: flex;
	flex-direction: column;
	flex: 1;
	min-width: 0;
	padding: 0 16px 0;
}

.head {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 72px;
	padding: 0 8px;
	flex-shrink: 0;
}

h1 {
	margin: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 22px;
	font-weight: 700;
}

.total {
	margin-left: 8px;
	font-size: 13px;
	color: var(--text-3);
	white-space: nowrap;
}

.spacer {
	flex: 1;
}

.search,
.bulk {
	display: flex;
	align-items: center;
	gap: 8px;
	height: 40px;
	margin: 0 8px 8px;
	flex-shrink: 0;
}

.search {
	padding: 0 12px;
	border-radius: 8px;
	background: var(--bg-input);
	color: var(--text-3);
}

.search input {
	flex: 1;
	min-width: 0;
	height: 100%;
	border: none;
	background: none;
	outline: none;
	color: var(--text);
}

.bulk-count {
	font-weight: 600;
}

.link {
	color: var(--accent);
	font-size: 13px;
}

.btn.danger {
	color: var(--danger);
}

.btn[aria-disabled="true"] {
	opacity: 0.5;
	pointer-events: none;
}

.rows {
	flex: 1;
	overflow: auto;
	min-height: 0;
}

.row {
	display: flex;
	align-items: center;
	gap: 12px;
	height: 60px;
	padding: 0 10px;
	border-radius: 8px;
	cursor: pointer;
}

.row:hover {
	background: var(--bg-hover);
}

.row.active,
.row.selected {
	background: var(--bg-active);
}

.pick {
	position: relative;
	display: flex;
	flex-shrink: 0;
	border-radius: 50%;
}

.tick {
	position: absolute;
	inset: 0;
	display: none;
	align-items: center;
	justify-content: center;
	border-radius: 50%;
	background: var(--accent);
	color: #fff;
}

.row.selected .tick {
	display: flex;
}

/* Hovering the avatar offers to select; while selecting, every row does. */
.pick:hover .tick,
.row.selecting:not(.selected) .pick:hover .tick {
	display: flex;
	opacity: 0.85;
}

.info {
	flex: 1;
	min-width: 0;
}

.name {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-weight: 500;
}

.sub {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 13px;
	color: var(--text-3);
}

.where {
	flex-shrink: 0;
	max-width: 32%;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 13px;
	color: var(--text-3);
}

.big-emoji {
	font-size: 40px;
}

.empty-actions {
	display: flex;
	gap: 8px;
	margin-top: 10px;
}

.pick-menu {
	max-height: 320px;
	overflow: auto;
}

.tag-form {
	display: flex;
	gap: 6px;
	padding: 8px;
}

.tag-form .input {
	width: 180px;
}

@media (max-width: 767px) {
	.sidebar {
		width: 100%;
		border-right: none;
	}

	.main-pane {
		padding: 0 6px;
	}

	.head {
		height: 56px;
	}

	.where {
		display: none;
	}
}
</style>
