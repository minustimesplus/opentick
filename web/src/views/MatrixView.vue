<script setup lang="ts">
import {computed, ref} from 'vue'
import {AlignLeft, ChevronDown, ChevronRight, MoreHorizontal, Plus, Eye, EyeOff, Check, Layers, ArrowUpDown, RotateCcw} from 'lucide-vue-next'
import TaskCheckbox from '@/components/TaskCheckbox.vue'
import Popover from '@/components/Popover.vue'
import TaskCreatePopup from '@/components/TaskCreatePopup.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {addDays, getSchedule, isOverdue, parseDate, startOfDay, taskDateLabel} from '@/lib/dates'
import {level, priorityName, splitEmoji, type Level} from '@/lib/tasks'
import {t, type MessageKey} from '@/lib/i18n'
import {openTaskDrawer, openTaskMenu} from '@/lib/ui'
import {prefs, type MatrixGroup, type MatrixQuadrantView, type MatrixSort} from '@/lib/prefs'
import {isMobile} from '@/lib/viewport'

const data = useData()

interface Quadrant {
	id: number
	roman: string
	title: MessageKey
	color: string
	priority: Level
}

interface Group {
	key: string
	title: string
	emoji?: string | null
	header: boolean
	tasks: Task[]
}

// Same arrangement as TickTick: I | III on top, II | IV below.
const quadrants: Quadrant[] = [
	{id: 1, roman: 'I', title: 'q1', color: 'var(--q1)', priority: 3},
	{id: 3, roman: 'III', title: 'q3', color: 'var(--q3)', priority: 1},
	{id: 2, roman: 'II', title: 'q2', color: 'var(--q2)', priority: 2},
	{id: 4, roman: 'IV', title: 'q4', color: 'var(--q4)', priority: 0},
]

const showDone = computed({
	get: () => prefs.matrixShowDone,
	set: v => { prefs.matrixShowDone = v },
})
const listFilter = computed<number[]>({
	get: () => prefs.matrixLists,
	set: v => { prefs.matrixLists = v },
})

// ---- per-quadrant grouping and sorting ----
const DEFAULT_VIEW: MatrixQuadrantView = {group: 'priority', sort: 'date'}

const groupOptions: {key: MatrixGroup, label: MessageKey}[] = [
	{key: 'priority', label: 'priority'},
	{key: 'list', label: 'groupList'},
	{key: 'date', label: 'date'},
	{key: 'none', label: 'none'},
]
const sortOptions: {key: MatrixSort, label: MessageKey}[] = [
	{key: 'date', label: 'sortDate'},
	{key: 'priority', label: 'sortPriority'},
	{key: 'title', label: 'sortTitle'},
	{key: 'created', label: 'sortCreated'},
	{key: 'custom', label: 'sortCustom'},
]

function viewOf(qid: number): MatrixQuadrantView {
	return {...DEFAULT_VIEW, ...prefs.matrixView[qid]}
}

function setView(qid: number, patch: Partial<MatrixQuadrantView>) {
	prefs.matrixView = {...prefs.matrixView, [qid]: {...viewOf(qid), ...patch}}
}

function resetView(qid: number) {
	const {[qid]: _v, ...views} = prefs.matrixView
	const {[qid]: _o, ...orders} = prefs.matrixOrder
	prefs.matrixView = views
	prefs.matrixOrder = orders
}

function isDefault(qid: number) {
	const v = viewOf(qid)
	return v.group === DEFAULT_VIEW.group && v.sort === DEFAULT_VIEW.sort && !prefs.matrixOrder[qid]
}

function dateKey(task: Task) {
	const s = getSchedule(task)
	return s ? s.date.getTime() : Number.MAX_SAFE_INTEGER
}

function comparator(qid: number): (a: Task, b: Task) => number {
	const sort = viewOf(qid).sort
	const byDate = (a: Task, b: Task) => dateKey(a) - dateKey(b) || a.id - b.id
	switch (sort) {
		case 'priority':
			return (a, b) => (b.priority ?? 0) - (a.priority ?? 0) || byDate(a, b)
		case 'title':
			return (a, b) => a.title.localeCompare(b.title) || a.id - b.id
		case 'created':
			return (a, b) => (parseDate(b.created)?.getTime() ?? 0) - (parseDate(a.created)?.getTime() ?? 0) || b.id - a.id
		case 'custom': {
			const order = new Map((prefs.matrixOrder[qid] ?? []).map((id, i) => [id, i]))
			// Tasks not placed yet (new ones) go on top, newest first.
			return (a, b) => (order.get(a.id) ?? -1 - a.id) - (order.get(b.id) ?? -1 - b.id)
		}
		default:
			return byDate
	}
}

type DateBucket = 'overdue' | 'today' | 'tomorrow' | 'next7' | 'later' | 'noDate'
const DATE_BUCKETS: DateBucket[] = ['overdue', 'today', 'tomorrow', 'next7', 'later', 'noDate']

function dateBucket(task: Task): DateBucket {
	const s = getSchedule(task)
	if (!s) return 'noDate'
	const today = startOfDay(new Date())
	const day = startOfDay(s.date)
	if (day < today) return task.done ? 'today' : 'overdue'
	if (day.getTime() === today.getTime()) return 'today'
	if (day.getTime() === addDays(today, 1).getTime()) return 'tomorrow'
	if (day < addDays(today, 7)) return 'next7'
	return 'later'
}

function tasksFor(q: Quadrant): Group[] {
	const view = viewOf(q.id)
	const source = data.allTasks.filter(task =>
		(!task.done || (showDone.value && isRecentDone(task)))
		&& data.projectMap.has(task.project_id)
		&& level(task.priority) === q.priority
		&& (!listFilter.value.length || listFilter.value.includes(task.project_id)))
	const groups = new Map<string, Group>()
	const add = (key: string, make: () => Omit<Group, 'tasks' | 'key'>, task: Task) => {
		if (!groups.has(key)) groups.set(key, {key, ...make(), tasks: []})
		groups.get(key)!.tasks.push(task)
	}
	for (const task of source) {
		if (view.group === 'priority') {
			const p = q.priority === 3 ? 3 : task.priority ?? 0
			add(`p${p}`, () => ({title: priorityName(level(p)), header: q.priority !== 0}), task)
		} else if (view.group === 'list') {
			const l = listLabel(task)
			add(`l${task.project_id}`, () => ({title: l?.name ?? '', emoji: l?.emoji, header: true}), task)
		} else if (view.group === 'date') {
			const b = dateBucket(task)
			add(`d${b}`, () => ({title: t(b), header: true}), task)
		} else {
			add('all', () => ({title: '', header: false}), task)
		}
	}
	const projectIndex = new Map(data.realProjects.map((p, i) => [p.id, i]))
	const rank = (g: Group) => {
		if (view.group === 'priority') return -Number(g.key.slice(1))
		if (view.group === 'list') return projectIndex.get(Number(g.key.slice(1))) ?? 0
		if (view.group === 'date') return DATE_BUCKETS.indexOf(g.key.slice(1) as DateBucket)
		return 0
	}
	const cmp = comparator(q.id)
	return [...groups.values()]
		.sort((a, b) => rank(a) - rank(b))
		.map(g => ({...g, tasks: g.tasks.sort((a, b) => Number(a.done) - Number(b.done) || cmp(a, b))}))
}

function isRecentDone(task: Task) {
	const d = parseDate(task.done_at)
	return !!d && Date.now() - d.getTime() < 7 * 86400_000
}

const groupsByQuadrant = computed(() => new Map(quadrants.map(q => [q.id, tasksFor(q)])))

function listLabel(task: Task) {
	const p = data.project(task.project_id)
	if (!p) return null
	if (p.id === data.inboxId) return {emoji: null, name: t('inbox')}
	return splitEmoji(p.title)
}

const collapsed = ref<Record<string, boolean>>({})

// ---- drag & drop: between quadrants changes priority, within one reorders ----
const dropQ = ref<number | null>(null)
const dragId = ref<number | null>(null)
const dropAt = ref<{q: number, id: number, after: boolean} | null>(null)

function onDragStart(e: DragEvent, task: Task) {
	dragId.value = task.id
	e.dataTransfer?.setData('application/x-task-id', String(task.id))
	if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragEnd() {
	dragId.value = null
	dropAt.value = null
	dropQ.value = null
}

function onDragOver(e: DragEvent, q: Quadrant) {
	if (!e.dataTransfer?.types.includes('application/x-task-id')) return
	e.preventDefault()
	dropQ.value = q.id
	const row = (e.target as HTMLElement).closest?.('.m-row[data-id]') as HTMLElement | null
	const id = Number(row?.dataset.id)
	if (!row || !id || id === dragId.value) {
		if (!row) dropAt.value = null
		return
	}
	const r = row.getBoundingClientRect()
	dropAt.value = {q: q.id, id, after: e.clientY > r.top + r.height / 2}
}

function onDragLeave(e: DragEvent, q: Quadrant) {
	if ((e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) return
	if (dropQ.value === q.id) dropQ.value = null
	if (dropAt.value?.q === q.id) dropAt.value = null
}

function onDrop(e: DragEvent, q: Quadrant) {
	const target = dropAt.value?.q === q.id ? dropAt.value : null
	const id = dragId.value ?? Number(e.dataTransfer?.getData('application/x-task-id'))
	onDragEnd()
	const task = id ? data.tasks.get(id) : undefined
	if (!task) return
	if (level(task.priority) !== q.priority) data.updateTask(id, {priority: q.priority})
	if (target && target.id !== id) reorder(q.id, id, target.id, target.after)
}

/** Moves a task before or after another one and switches the quadrant to custom order. */
function reorder(qid: number, id: number, targetId: number, after: boolean) {
	const shown = (groupsByQuadrant.value.get(qid) ?? []).flatMap(g => g.tasks.map(task => task.id)).filter(x => x !== id)
	let i = shown.indexOf(targetId)
	if (i < 0) return
	if (after) i++
	shown.splice(i, 0, id)
	// Keep tasks hidden by the filters where they were, after the visible ones.
	const hidden = (prefs.matrixOrder[qid] ?? []).filter(x => !shown.includes(x))
	prefs.matrixOrder = {...prefs.matrixOrder, [qid]: [...shown, ...hidden]}
	if (viewOf(qid).sort !== 'custom') setView(qid, {sort: 'custom'})
}

// ---- add per quadrant ----
const adding = ref<{q: Quadrant, el: HTMLElement} | null>(null)

function startAdd(q: Quadrant, e: MouseEvent) {
	adding.value = {q, el: e.currentTarget as HTMLElement}
}

// ---- quadrant menu ----
const qMenu = ref<{q: Quadrant, el: HTMLElement} | null>(null)
const qSub = ref<'group' | 'sort' | null>(null)
const groupEl = ref<HTMLElement | null>(null)
const sortEl = ref<HTMLElement | null>(null)

function openQMenu(q: Quadrant, e: MouseEvent) {
	qSub.value = null
	qMenu.value = {q, el: e.currentTarget as HTMLElement}
}

function closeQMenu() {
	qSub.value = null
	qMenu.value = null
}

function pickGroup(g: MatrixGroup) {
	if (qMenu.value) setView(qMenu.value.q.id, {group: g})
	closeQMenu()
}

function pickSort(s: MatrixSort) {
	if (qMenu.value) setView(qMenu.value.q.id, {sort: s})
	closeQMenu()
}

function optionLabel<K extends string>(opts: {key: K, label: MessageKey}[], key: K) {
	return t(opts.find(o => o.key === key)?.label ?? opts[0].label)
}

// ---- view menu ----
const moreEl = ref<HTMLElement | null>(null)
const menuOpen = ref(false)

function toggleList(id: number) {
	const cur = listFilter.value
	listFilter.value = cur.includes(id) ? cur.filter(x => x !== id) : [...cur, id]
}
</script>

<template>
	<div class="matrix-view">
		<header class="head">
			<h1>{{ t('matrix') }}</h1>
			<button ref="moreEl" class="icon-btn" @click="menuOpen = true"><MoreHorizontal :size="20" /></button>
		</header>

		<div class="grid">
			<section
				v-for="q in quadrants"
				:key="q.id"
				class="quad"
				:class="{drop: dropQ === q.id, active: adding?.q.id === q.id || qMenu?.q.id === q.id}"
				:style="{'--q-order': q.id}"
				@dragover="onDragOver($event, q)"
				@dragleave="onDragLeave($event, q)"
				@drop="onDrop($event, q)"
			>
				<div class="q-head" :style="{color: q.color}">
					<span class="roman" :style="{background: q.color}">{{ q.roman }}</span>
					<span class="q-title">{{ t(q.title) }}</span>
					<button class="q-btn" :title="t('addTask')" @click="startAdd(q, $event)"><Plus :size="16" /></button>
					<button class="q-btn" @click="openQMenu(q, $event)"><MoreHorizontal :size="16" /></button>
				</div>

				<div class="q-body">
					<template v-for="g in groupsByQuadrant.get(q.id)" :key="g.key">
						<div v-if="g.header" class="g-head" @click="collapsed[`${q.id}:${g.key}`] = !collapsed[`${q.id}:${g.key}`]">
							<component :is="collapsed[`${q.id}:${g.key}`] ? ChevronRight : ChevronDown" :size="14" />
							<span v-if="g.emoji">{{ g.emoji }}</span>
							<span>{{ g.title }}</span>
							<span class="g-count">{{ g.tasks.length }}</span>
						</div>
						<template v-if="!g.header || !collapsed[`${q.id}:${g.key}`]">
							<div
								v-for="task in g.tasks"
								:key="task.id"
								class="m-row"
								:data-id="task.id"
								:class="{
									done: task.done,
									dragging: dragId === task.id,
									'drop-before': dropAt?.q === q.id && dropAt.id === task.id && !dropAt.after,
									'drop-after': dropAt?.q === q.id && dropAt.id === task.id && dropAt.after,
								}"
								:draggable="!isMobile"
								@dragstart="onDragStart($event, task)"
								@dragend="onDragEnd"
								@click="openTaskDrawer(task.id)"
								@contextmenu="openTaskMenu(task.id, $event)"
							>
								<TaskCheckbox :done="task.done" :wont-do="task.wont_do" :priority="task.priority" @toggle="data.toggleDone(task.id)" />
								<span class="m-title">{{ task.title }}</span>
								<span v-if="viewOf(q.id).group !== 'list' && listLabel(task)" class="m-list">
									<span v-if="listLabel(task)!.emoji">{{ listLabel(task)!.emoji }}</span>{{ listLabel(task)!.name }}
								</span>
								<AlignLeft v-if="task.description && task.description.replace(/<[^>]*>/g, '').trim()" :size="13" class="m-ic" />
								<span v-if="taskDateLabel(task)" class="m-date" :class="{overdue: isOverdue(task)}">{{ taskDateLabel(task) }}</span>
							</div>
						</template>
					</template>
					<div v-if="!groupsByQuadrant.get(q.id)?.length" class="q-empty">—</div>
				</div>
			</section>
		</div>

		<TaskCreatePopup
			v-if="adding"
			:key="adding.q.id"
			:anchor="adding.el"
			:priority="adding.q.priority"
			:project-id="listFilter.length === 1 ? listFilter[0] : undefined"
			@close="adding = null"
		/>

		<Popover v-if="qMenu" :anchor="qMenu.el" placement="bottom-end" @close="closeQMenu">
			<div class="menu q-menu">
				<button ref="groupEl" class="menu-item" :class="{active: qSub === 'group'}" @click="qSub = 'group'">
					<Layers :size="15" /> {{ t('groupBy') }}
					<span class="q-menu-val">{{ optionLabel(groupOptions, viewOf(qMenu.q.id).group) }}</span>
					<ChevronRight :size="14" />
				</button>
				<button ref="sortEl" class="menu-item" :class="{active: qSub === 'sort'}" @click="qSub = 'sort'">
					<ArrowUpDown :size="15" /> {{ t('sortBy') }}
					<span class="q-menu-val">{{ optionLabel(sortOptions, viewOf(qMenu.q.id).sort) }}</span>
					<ChevronRight :size="14" />
				</button>
				<div class="menu-sep" />
				<button class="menu-item" :disabled="isDefault(qMenu.q.id)" @click="resetView(qMenu.q.id); closeQMenu()">
					<RotateCcw :size="15" /> {{ t('resetMatrixSort') }}
				</button>
			</div>
		</Popover>
		<Popover v-if="qMenu && qSub === 'group'" :anchor="groupEl" placement="right-start" @close="qSub = null">
			<div class="menu">
				<button v-for="o in groupOptions" :key="o.key" class="menu-item" @click="pickGroup(o.key)">
					<Check :size="15" :style="{visibility: viewOf(qMenu.q.id).group === o.key ? 'visible' : 'hidden'}" /> {{ t(o.label) }}
				</button>
			</div>
		</Popover>
		<Popover v-if="qMenu && qSub === 'sort'" :anchor="sortEl" placement="right-start" @close="qSub = null">
			<div class="menu">
				<button v-for="o in sortOptions" :key="o.key" class="menu-item" @click="pickSort(o.key)">
					<Check :size="15" :style="{visibility: viewOf(qMenu.q.id).sort === o.key ? 'visible' : 'hidden'}" /> {{ t(o.label) }}
				</button>
			</div>
		</Popover>

		<Popover v-if="menuOpen" :anchor="moreEl" placement="bottom-end" @close="menuOpen = false">
			<div class="menu lists-menu">
				<button class="menu-item" @click="showDone = !showDone">
					<component :is="showDone ? EyeOff : Eye" :size="15" /> {{ showDone ? t('hideCompleted') : t('showCompleted') }}
				</button>
				<div class="menu-sep" />
				<div class="menu-label">{{ t('lists') }}</div>
				<button class="menu-item" @click="listFilter = []">
					<Check :size="15" :style="{visibility: listFilter.length ? 'hidden' : 'visible'}" /> {{ t('allLists') }}
				</button>
				<button v-for="p in data.realProjects" :key="p.id" class="menu-item" @click="toggleList(p.id)">
					<Check :size="15" :style="{visibility: listFilter.includes(p.id) ? 'visible' : 'hidden'}" />
					{{ p.id === data.inboxId ? t('inbox') : p.title }}
				</button>
			</div>
		</Popover>
	</div>
</template>

<style scoped>
.matrix-view {
	display: flex;
	flex-direction: column;
	height: 100%;
	padding: 0 24px 24px;
}

.head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	height: 72px;
	padding: 0 14px;
	flex-shrink: 0;
}

h1 {
	margin: 0;
	font-size: 22px;
	font-weight: 700;
}

.grid {
	flex: 1;
	min-height: 0;
	display: grid;
	grid-template-columns: 1fr 1fr;
	grid-template-rows: 1fr 1fr;
	gap: 16px;
}

.quad {
	display: flex;
	flex-direction: column;
	min-height: 0;
	/* Like TickTick: white cards set off by a hairline and a faint shadow. */
	background: var(--bg-elev);
	border-radius: 12px;
	padding: 14px 10px 10px 14px;
	border: 1px solid color-mix(in srgb, var(--border) 45%, var(--bg-elev));
	box-shadow: 0 0 6px rgb(0 0 0 / 0.03);
}

.quad.drop {
	border-color: var(--accent);
}

.q-head {
	display: flex;
	align-items: center;
	gap: 8px;
	height: 28px;
	margin-bottom: 8px;
	font-size: 15px;
	font-weight: 500;
}

.roman {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	min-width: 20px;
	height: 20px;
	padding: 0 4px;
	border-radius: 10px;
	color: #fff;
	font-size: 10px;
	font-weight: 700;
	font-family: Georgia, serif;
}

.q-title {
	flex: 1;
}

.q-btn {
	display: none;
	color: var(--text-3);
	border-radius: 5px;
	padding: 3px;
}

.quad:hover .q-btn,
.quad.active .q-btn {
	display: flex;
}

/* Touch screens can't hover, so the buttons always show. */
@media (hover: none) {
	.q-btn {
		display: flex;
	}
}

.q-btn:hover {
	background: var(--bg-hover);
	color: var(--text);
}

.q-menu {
	min-width: 240px;
}

.q-menu-val {
	margin-left: auto;
	color: var(--text-3);
}

.q-body {
	flex: 1;
	overflow: auto;
	padding-right: 4px;
}

.g-head {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 32px;
	padding-left: 12px;
	font-weight: 600;
	color: var(--text);
	cursor: pointer;
	user-select: none;
}

.g-count {
	color: var(--text-3);
	font-weight: 400;
	font-size: 12px;
}

.m-row {
	display: flex;
	align-items: center;
	gap: 12px;
	min-height: 46px;
	padding: 0 12px 0 30px;
	border-radius: 6px;
	cursor: default;
	position: relative;
}

.m-row.dragging {
	opacity: 0.4;
}

/* Where a dragged task will land. */
.m-row.drop-before::before,
.m-row.drop-after::before {
	content: "";
	position: absolute;
	left: 30px;
	right: 12px;
	height: 2px;
	border-radius: 1px;
	background: var(--accent);
	z-index: 1;
}

.m-row.drop-before::before {
	top: -1px;
}

.m-row.drop-after::before {
	bottom: -1px;
}

.m-row::after {
	content: "";
	position: absolute;
	left: 58px;
	right: 12px;
	bottom: 0;
	height: 1px;
	background: var(--border);
}

@media (hover: hover) {
	.m-row:hover {
		background: var(--bg-hover);
	}
}

.m-title {
	flex: 1;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 15px;
}

.done .m-title {
	color: var(--text-3);
	text-decoration: line-through;
}

.m-list {
	display: inline-flex;
	gap: 3px;
	font-size: 13px;
	color: var(--text-3);
	white-space: nowrap;
	max-width: 160px;
	overflow: hidden;
}

.m-ic {
	color: var(--text-3);
	flex-shrink: 0;
}

.m-date {
	font-size: 13px;
	color: var(--accent);
	white-space: nowrap;
}

.m-date.overdue {
	color: var(--danger);
}

.q-empty {
	color: var(--text-4);
	padding: 12px 30px;
}

.lists-menu {
	max-height: 70vh;
	overflow: auto;
}

@media (max-width: 900px) {
	.grid {
		grid-template-columns: 1fr;
		grid-template-rows: none;
		overflow: auto;
	}

	.quad {
		min-height: 300px;
	}
}

@media (max-width: 767px) {
	.matrix-view {
		padding: 0 10px 10px;
	}

	.head {
		height: 56px;
		padding: 0 4px 0 8px;
	}

	h1 {
		font-size: 20px;
	}

	/* minmax(0, …) stops long titles from widening the column. */
	.grid {
		grid-template-columns: minmax(0, 1fr);
		gap: 10px;
	}

	/* Quadrants grow with their tasks; the grid scrolls as a whole. Stacked,
	   they read I to IV instead of the 2x2 grid's urgent row first. */
	.quad {
		order: var(--q-order);
		min-height: 0;
		padding: 10px 6px 6px 10px;
	}

	.q-body {
		flex: none;
		overflow: visible;
		padding-right: 0;
	}

	.q-btn {
		padding: 6px;
	}

	.g-head {
		padding-left: 4px;
	}

	.m-row {
		min-height: 48px;
		padding: 0 8px 0 10px;
		-webkit-touch-callout: none;
		user-select: none;
	}

	.m-row::after {
		left: 38px;
		right: 8px;
	}

	.m-list {
		max-width: 88px;
	}

	.q-empty {
		padding: 8px 10px;
	}
}
</style>
