<script setup lang="ts">
import {computed, ref} from 'vue'
import {AlignLeft, ChevronDown, ChevronRight, MoreHorizontal, Plus, Eye, EyeOff, Check} from 'lucide-vue-next'
import TaskCheckbox from '@/components/TaskCheckbox.vue'
import Popover from '@/components/Popover.vue'
import QuickAdd from '@/components/QuickAdd.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {getSchedule, isOverdue, parseDate, taskDateLabel} from '@/lib/dates'
import {level, priorityName, splitEmoji, type Level} from '@/lib/tasks'
import {t, type MessageKey} from '@/lib/i18n'
import {openTaskDrawer, openTaskMenu, ui} from '@/lib/ui'
import {prefs} from '@/lib/prefs'
import {isMobile} from '@/lib/viewport'

const data = useData()

interface Quadrant {
	id: number
	roman: string
	title: MessageKey
	color: string
	priority: Level
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

function sortKey(task: Task) {
	const s = getSchedule(task)
	return s ? s.date.getTime() : Number.MAX_SAFE_INTEGER
}

function tasksFor(q: Quadrant): {priority: number, tasks: Task[]}[] {
	const source = data.allTasks.filter(task =>
		(!task.done || (showDone.value && isRecentDone(task)))
		&& data.projectMap.has(task.project_id)
		&& level(task.priority) === q.priority
		&& (!listFilter.value.length || listFilter.value.includes(task.project_id)))
	const groups = new Map<number, Task[]>()
	for (const task of source) {
		const p = task.priority ?? 0
		const g = q.priority === 3 ? 3 : p
		if (!groups.has(g)) groups.set(g, [])
		groups.get(g)!.push(task)
	}
	return [...groups.entries()]
		.sort((a, b) => b[0] - a[0])
		.map(([priority, tasks]) => ({
			priority,
			tasks: tasks.sort((a, b) => Number(a.done) - Number(b.done) || sortKey(a) - sortKey(b) || a.id - b.id),
		}))
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

// ---- drag & drop between quadrants ----
const dropQ = ref<number | null>(null)

function onDragStart(e: DragEvent, task: Task) {
	e.dataTransfer?.setData('application/x-task-id', String(task.id))
	if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragOver(e: DragEvent, q: Quadrant) {
	if (!e.dataTransfer?.types.includes('application/x-task-id')) return
	e.preventDefault()
	dropQ.value = q.id
}

function onDrop(e: DragEvent, q: Quadrant) {
	dropQ.value = null
	const id = Number(e.dataTransfer?.getData('application/x-task-id'))
	const task = id ? data.tasks.get(id) : undefined
	if (!task || level(task.priority) === q.priority) return
	data.updateTask(id, {priority: q.priority})
}

// ---- add per quadrant ----
const adding = ref<number | null>(null)

// ---- menu ----
const moreEl = ref<HTMLElement | null>(null)
const menuOpen = ref(false)

function toggleList(id: number) {
	const cur = listFilter.value
	listFilter.value = cur.includes(id) ? cur.filter(x => x !== id) : [...cur, id]
}

function groupTitle(p: number) {
	return priorityName(level(p))
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
				:class="{drop: dropQ === q.id}"
				:style="{'--q-order': q.id}"
				@dragover="onDragOver($event, q)"
				@dragleave="dropQ === q.id && (dropQ = null)"
				@drop="onDrop($event, q)"
			>
				<div class="q-head" :style="{color: q.color}">
					<span class="roman" :style="{background: q.color}">{{ q.roman }}</span>
					<span class="q-title">{{ t(q.title) }}</span>
					<button class="q-add" :title="t('addTask')" @click="adding = adding === q.id ? null : q.id"><Plus :size="16" /></button>
				</div>

				<QuickAdd
					v-if="adding === q.id"
					:key="'qa' + q.id + ui.focusQuickAdd"
					class="q-qa"
					:defaults="{priority: q.priority}"
					@created="adding = null"
				/>

				<div class="q-body">
					<template v-for="g in groupsByQuadrant.get(q.id)" :key="g.priority">
						<div v-if="q.priority !== 0" class="g-head" @click="collapsed[`${q.id}:${g.priority}`] = !collapsed[`${q.id}:${g.priority}`]">
							<component :is="collapsed[`${q.id}:${g.priority}`] ? ChevronRight : ChevronDown" :size="14" />
							<span>{{ groupTitle(g.priority) }}</span>
							<span class="g-count">{{ g.tasks.length }}</span>
						</div>
						<template v-if="!collapsed[`${q.id}:${g.priority}`]">
							<div
								v-for="task in g.tasks"
								:key="task.id"
								class="m-row"
								:class="{done: task.done}"
								:draggable="!isMobile"
								@dragstart="onDragStart($event, task)"
								@click="openTaskDrawer(task.id)"
								@contextmenu="openTaskMenu(task.id, $event)"
							>
								<TaskCheckbox :done="task.done" :priority="task.priority" @toggle="data.toggleDone(task.id)" />
								<span class="m-title">{{ task.title }}</span>
								<span v-if="listLabel(task)" class="m-list">
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
	background: var(--bg-panel);
	border-radius: 12px;
	padding: 14px 10px 10px 14px;
	border: 1.5px solid transparent;
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

.q-add {
	display: none;
	color: var(--text-3);
	border-radius: 5px;
	padding: 3px;
}

.quad:hover .q-add {
	display: flex;
}

/* Touch screens can't hover, so the add button always shows. */
@media (hover: none) {
	.q-add {
		display: flex;
	}
}

.q-add:hover {
	background: var(--bg-hover);
	color: var(--text);
}

.q-qa {
	margin: 0 8px 8px 0;
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

	.q-add {
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
