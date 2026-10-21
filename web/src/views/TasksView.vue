<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {ArrowUpDown, ChevronDown, ChevronRight, Menu, MoreHorizontal, PanelLeft, Pencil, Trash2, Eye, EyeOff, Check, Tag, Filter} from 'lucide-vue-next'
import ListSidebar from '@/components/ListSidebar.vue'
import TaskRow from '@/components/TaskRow.vue'
import TaskDetail from '@/components/TaskDetail.vue'
import QuickAdd from '@/components/QuickAdd.vue'
import Popover from '@/components/Popover.vue'
import ListEditDialog from '@/components/ListEditDialog.vue'
import type {Task} from '@/api/types'
import {useData, type TaskPatch} from '@/stores/data'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {
	addDays, dateKey, dayDiff, fmtDayMonth, fmtLongDate, fmtWeekdayLong, getSchedule, isOverdue, parseDate,
	schedulePatch, startOfDay,
} from '@/lib/dates'
import {compareTitles, level, splitEmoji} from '@/lib/tasks'
import {openTaskDrawer} from '@/lib/ui'

const data = useData()
const route = useRoute()
const router = useRouter()

const listKey = computed(() => String(route.params.list ?? 'today'))
const selectedId = computed(() => route.params.taskId ? Number(route.params.taskId) : null)

type Kind = 'today' | 'week' | 'inbox' | 'project' | 'label' | 'filter' | 'completed'

const view = computed(() => {
	const k = listKey.value
	if (k === 'today' || k === 'week' || k === 'inbox' || k === 'completed') return {kind: k as Kind, id: 0}
	if (k.startsWith('p')) return {kind: 'project' as Kind, id: Number(k.slice(1))}
	if (k.startsWith('l')) return {kind: 'label' as Kind, id: Number(k.slice(1))}
	if (k.startsWith('f')) return {kind: 'filter' as Kind, id: -Number(k.slice(1))}
	return {kind: 'today' as Kind, id: 0}
})

const project = computed(() => {
	if (view.value.kind === 'project') return data.project(view.value.id)
	if (view.value.kind === 'inbox') return data.project(data.inboxId)
	return undefined
})
const label = computed(() => view.value.kind === 'label' ? data.labels.find(l => l.id === view.value.id) : undefined)
const savedFilter = computed(() => view.value.kind === 'filter' ? data.project(view.value.id) : undefined)

const header = computed(() => {
	switch (view.value.kind) {
		case 'today': return {emoji: null, name: t('today')}
		case 'week': return {emoji: null, name: t('next7')}
		case 'inbox': return {emoji: null, name: t('inbox')}
		case 'completed': return {emoji: null, name: t('completed')}
		case 'label': return {emoji: null, name: label.value?.title ?? ''}
		case 'filter': return {emoji: null, name: savedFilter.value?.title ?? ''}
		default: return project.value ? splitEmoji(project.value.title) : {emoji: null, name: ''}
	}
})

// ---- saved filter tasks are fetched from the server ----
const filterTaskIds = ref<number[]>([])
watch(savedFilter, async f => {
	filterTaskIds.value = []
	if (f) filterTaskIds.value = (await data.loadSavedFilterTasks(f)).map(t => t.id)
}, {immediate: true})

// ---- sorting ----
type SortKey = 'date' | 'priority' | 'title' | 'created'
const sortKey = computed<SortKey>({
	get: () => prefs.listSort[listKey.value] ?? 'date',
	set: v => { prefs.listSort[listKey.value] = v },
})

function dateValue(task: Task) {
	const s = getSchedule(task)
	return s ? s.date.getTime() : Number.MAX_SAFE_INTEGER
}

function sortTasks(list: Task[]): Task[] {
	const by: Record<SortKey, (a: Task, b: Task) => number> = {
		date: (a, b) => dateValue(a) - dateValue(b) || level(b.priority) - level(a.priority) || a.id - b.id,
		priority: (a, b) => level(b.priority) - level(a.priority) || dateValue(a) - dateValue(b),
		title: compareTitles,
		created: (a, b) => (parseDate(b.created)?.getTime() ?? 0) - (parseDate(a.created)?.getTime() ?? 0),
	}
	return [...list].sort(by[sortKey.value])
}

// ---- sections ----
interface Section {
	key: string
	title: string
	tasks: Task[]
}

const showCompleted = computed({
	get: () => prefs.listShowCompleted[listKey.value] ?? true,
	set: v => { prefs.listShowCompleted[listKey.value] = v },
})

function matches(task: Task): boolean {
	const v = view.value
	switch (v.kind) {
		case 'today': return data.isToday(task)
		case 'week': return data.inNext7(task)
		case 'inbox': return task.project_id === data.inboxId
		case 'project': return task.project_id === v.id
		case 'label': return !!task.labels?.some(l => l.id === v.id)
		case 'filter': return filterTaskIds.value.includes(task.id)
		default: return false
	}
}

const openTasks = computed(() => sortTasks(data.openTasks.filter(matches)))

const doneTasks = computed(() => {
	const v = view.value
	const now = new Date()
	let list = data.allTasks.filter(task => task.done)
	if (v.kind === 'today') {
		list = list.filter(task => {
			const s = getSchedule(task)
			const d = parseDate(task.done_at)
			return (s && dayDiff(s.date, now) === 0) || (d && dayDiff(d, now) === 0)
		})
	} else if (v.kind === 'week') {
		list = list.filter(task => {
			const s = getSchedule(task)
			return s && dayDiff(s.date, now) >= 0 && dayDiff(s.date, now) < 7
		})
	} else if (v.kind !== 'completed') {
		list = list.filter(matches)
	}
	return list.sort((a, b) => (parseDate(b.done_at)?.getTime() ?? 0) - (parseDate(a.done_at)?.getTime() ?? 0))
})

const sections = computed<Section[]>(() => {
	const v = view.value
	const now = new Date()
	if (v.kind === 'completed') {
		const groups = new Map<string, Section>()
		for (const task of doneTasks.value) {
			const d = parseDate(task.done_at) ?? new Date(0)
			const key = dateKey(d)
			if (!groups.has(key)) {
				const diff = dayDiff(d, now)
				const title = diff === 0 ? t('today') : diff === -1 ? t('yesterday') : fmtLongDate(d)
				groups.set(key, {key, title, tasks: []})
			}
			groups.get(key)!.tasks.push(task)
		}
		return [...groups.values()]
	}
	if (v.kind === 'today' || v.kind === 'week') {
		const overdue = openTasks.value.filter(task => isOverdue(task, now) && dayDiff(getSchedule(task)!.date, now) < 0)
		const rest = openTasks.value.filter(task => !overdue.includes(task))
		const out: Section[] = []
		if (overdue.length) out.push({key: 'overdue', title: t('overdue'), tasks: overdue})
		if (v.kind === 'today') {
			out.push({key: 'today', title: t('today'), tasks: rest})
		} else {
			for (let i = 0; i < 7; i++) {
				const day = addDays(startOfDay(now), i)
				const tasks = rest.filter(task => dayDiff(getSchedule(task)!.date, day) === 0)
				const title = i === 0 ? t('today') : i === 1 ? t('tomorrow') : `${fmtWeekdayLong(day)}`
				out.push({key: `d${i}`, title: `${title}${i > 1 ? ', ' + fmtDayMonth(day) : ''}`, tasks})
			}
		}
		return out.filter(s => s.tasks.length || (v.kind === 'today' && s.key === 'today'))
	}
	return [{key: 'all', title: '', tasks: openTasks.value}]
})

const completedSection = computed(() => view.value.kind === 'completed' ? null : doneTasks.value)

const completedOpen = computed({
	get: () => !prefs.collapsed[`done:${listKey.value}`],
	set: v => {
		prefs.collapsed[`done:${listKey.value}`] = !v
	},
})

watch([view, completedOpen], () => {
	const v = view.value
	if (!completedOpen.value && v.kind !== 'completed') return
	if (v.kind === 'project') data.loadDoneForProject(v.id)
	else if (v.kind === 'inbox') data.loadDoneForProject(data.inboxId)
	else if (v.kind === 'completed') data.loadDone(4)
}, {immediate: true})

const isEmpty = computed(() => openTasks.value.length === 0)

// ---- quick add defaults ----
const quickDefaults = computed<TaskPatch & {project_id?: number}>(() => {
	const v = view.value
	if (v.kind === 'today' || v.kind === 'week') return {...schedulePatch(startOfDay(new Date()), true)}
	if (v.kind === 'project') return {project_id: v.id}
	if (v.kind === 'inbox') return {project_id: data.inboxId}
	return {}
})
const quickLabels = computed(() => label.value ? [label.value.title] : [])
const placeholder = computed(() => view.value.kind === 'completed' ? '' : t('addTaskTo', {list: header.value.name}))

// ---- selection ----
function select(id: number) {
	// The inline detail pane is hidden on narrow windows; use the drawer there.
	if (window.innerWidth <= 1100) {
		openTaskDrawer(id)
		return
	}
	router.push(`/tasks/${listKey.value}/${id}`)
}

function onCreated(id: number) {
	if (selectedId.value) select(id)
}

const collapsed = ref<Record<string, boolean>>({})

// ---- header menus ----
const sortEl = ref<HTMLElement | null>(null)
const moreEl = ref<HTMLElement | null>(null)
const pop = ref<'sort' | 'more' | null>(null)
const editing = ref(false)

async function deleteList() {
	pop.value = null
	if (!project.value || !confirm(t('deleteListConfirm', {name: project.value.title}))) return
	await data.deleteProject(project.value.id)
	router.push('/tasks/today')
}

const sorts: {key: SortKey, label: Parameters<typeof t>[0]}[] = [
	{key: 'date', label: 'sortDate'},
	{key: 'priority', label: 'sortPriority'},
	{key: 'title', label: 'sortTitle'},
	{key: 'created', label: 'sortCreated'},
]
</script>

<template>
	<div class="tasks-view">
		<ListSidebar v-if="prefs.tasksSidebar" />

		<section class="list-pane">
			<header class="head">
				<button class="icon-btn" @click="prefs.tasksSidebar = !prefs.tasksSidebar"><PanelLeft :size="19" /></button>
				<h1>
					<Tag v-if="view.kind === 'label'" :size="19" class="h-ic" />
					<Filter v-else-if="view.kind === 'filter'" :size="19" class="h-ic" />
					<span v-else-if="header.emoji" class="h-emoji">{{ header.emoji }}</span>
					{{ header.name }}
				</h1>
				<div class="spacer" />
				<button ref="sortEl" class="icon-btn" :title="t('sortBy')" @click="pop = 'sort'"><ArrowUpDown :size="18" /></button>
				<button ref="moreEl" class="icon-btn" @click="pop = 'more'"><MoreHorizontal :size="18" /></button>
			</header>

			<div class="content">
				<QuickAdd
					v-if="view.kind !== 'completed' && view.kind !== 'filter'"
					:key="listKey"
					class="qa"
					:placeholder="placeholder"
					:defaults="quickDefaults"
					:default-labels="quickLabels"
					@created="onCreated"
				/>

				<div class="sections">
					<template v-for="s in sections" :key="s.key">
						<div v-if="s.title" class="sec-head" @click="collapsed[s.key] = !collapsed[s.key]">
							<component :is="collapsed[s.key] ? ChevronRight : ChevronDown" :size="14" />
							<span :class="{overdue: s.key === 'overdue'}">{{ s.title }}</span>
							<span class="sec-count">{{ s.tasks.length }}</span>
						</div>
						<template v-if="!collapsed[s.key]">
							<TaskRow
								v-for="task in s.tasks"
								:key="task.id"
								:task="task"
								:selected="task.id === selectedId"
								:show-list="view.kind !== 'project' && view.kind !== 'inbox'"
								@select="select"
							/>
						</template>
					</template>

					<div v-if="isEmpty && view.kind !== 'completed' && !(completedSection?.length)" class="empty-state">
						<svg width="150" height="120" viewBox="0 0 150 120" fill="none">
							<path d="M30 70c-6-26 18-48 44-46 30 2 50 18 48 42-2 26-30 40-58 36-18-3-30-14-34-32z" fill="var(--bg-hover)" />
							<path d="M58 66l12 12 26-28" stroke="var(--accent)" stroke-width="7" stroke-linecap="round" stroke-linejoin="round" />
							<circle cx="104" cy="30" r="4" fill="var(--accent)" />
							<circle cx="40" cy="44" r="2.5" fill="var(--text-3)" />
							<circle cx="116" cy="82" r="2.5" fill="var(--text-3)" />
						</svg>
						<div>{{ t('allDoneTitle') }}</div>
						<div class="sub">{{ t('allDoneSub') }}</div>
					</div>

					<template v-if="completedSection && completedSection.length && showCompleted">
						<div class="sec-head" @click="completedOpen = !completedOpen">
							<component :is="completedOpen ? ChevronDown : ChevronRight" :size="14" />
							<span>{{ t('completed') }}</span>
							<span class="sec-count">{{ completedSection.length }}</span>
						</div>
						<template v-if="completedOpen">
							<TaskRow
								v-for="task in completedSection"
								:key="task.id"
								:task="task"
								:selected="task.id === selectedId"
								:show-list="view.kind !== 'project' && view.kind !== 'inbox'"
								@select="select"
							/>
						</template>
					</template>
				</div>
			</div>
		</section>

		<section class="detail-pane">
			<TaskDetail v-if="selectedId && data.tasks.has(selectedId)" :key="selectedId" :task-id="selectedId" @deleted="router.push(`/tasks/${listKey}`)" />
			<div v-else class="empty-state detail-empty">
				<svg width="200" height="150" viewBox="0 0 200 150" fill="none" stroke="var(--text-4)" stroke-width="2">
					<rect x="40" y="40" width="90" height="70" rx="6" transform="rotate(-12 85 75)" />
					<path d="M62 62l8 8 16-16" stroke="var(--text-3)" />
					<rect x="100" y="60" width="70" height="80" rx="6" />
					<path d="M112 80h44M112 94h44M112 108h30" />
					<circle cx="60" cy="125" r="12" />
				</svg>
				<div class="sub">{{ t('noTaskSelected') }}</div>
			</div>
		</section>

		<Popover v-if="pop === 'sort'" :anchor="sortEl" placement="bottom-end" @close="pop = null">
			<div class="menu">
				<div class="menu-label">{{ t('sortBy') }}</div>
				<button v-for="s in sorts" :key="s.key" class="menu-item" @click="sortKey = s.key; pop = null">
					<Check :size="15" :style="{visibility: sortKey === s.key ? 'visible' : 'hidden'}" /> {{ t(s.label) }}
				</button>
			</div>
		</Popover>
		<Popover v-if="pop === 'more'" :anchor="moreEl" placement="bottom-end" @close="pop = null">
			<div class="menu">
				<button class="menu-item" @click="showCompleted = !showCompleted; pop = null">
					<component :is="showCompleted ? EyeOff : Eye" :size="15" /> {{ showCompleted ? t('hideCompleted') : t('showCompleted') }}
				</button>
				<template v-if="view.kind === 'project' && project">
					<div class="menu-sep" />
					<button class="menu-item" @click="editing = true; pop = null"><Pencil :size="15" /> {{ t('editList') }}</button>
					<button class="menu-item danger" @click="deleteList"><Trash2 :size="15" /> {{ t('delete') }}</button>
				</template>
			</div>
		</Popover>
		<ListEditDialog v-if="editing && project" :project="project" @close="editing = false" />
	</div>
</template>

<style scoped>
.tasks-view {
	display: flex;
	height: 100%;
}

.list-pane {
	flex: 1.7;
	min-width: 360px;
	display: flex;
	flex-direction: column;
	border-right: 1px solid var(--border);
}

.head {
	display: flex;
	align-items: center;
	gap: 8px;
	height: 64px;
	padding: 0 20px 0 14px;
	flex-shrink: 0;
}

h1 {
	display: flex;
	align-items: center;
	gap: 8px;
	margin: 0;
	font-size: 22px;
	font-weight: 700;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}

.h-ic {
	color: var(--text-2);
}

.h-emoji {
	font-size: 20px;
}

.spacer {
	flex: 1;
}

.content {
	flex: 1;
	overflow: auto;
	padding: 0 20px 40px;
}

.qa {
	margin-bottom: 12px;
}

.sections {
	min-height: calc(100% - 60px);
}

.sec-head {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 38px;
	padding: 0 4px;
	margin-top: 6px;
	font-size: 13px;
	font-weight: 600;
	color: var(--text-2);
	cursor: pointer;
	user-select: none;
}

.sec-head .overdue {
	color: var(--danger);
}

.sec-count {
	color: var(--text-3);
	font-weight: 400;
}

.empty-state {
	height: 60vh;
}

.detail-pane {
	flex: 1;
	min-width: 340px;
	max-width: 640px;
	position: relative;
}

.detail-empty {
	height: 100%;
	padding: 40px;
}

@media (max-width: 1100px) {
	.detail-pane {
		display: none;
	}
}
</style>
