<script setup lang="ts">
import {computed, reactive, ref, watch} from 'vue'
import {MoreHorizontal, Pencil, Plus, Trash2} from 'lucide-vue-next'
import TaskCheckbox from './TaskCheckbox.vue'
import Popover from './Popover.vue'
import {api} from '@/api/client'
import type {Project, Task} from '@/api/types'
import {parentId, useData} from '@/stores/data'
import {isOverdue, schedulePatch, taskDateLabel} from '@/lib/dates'
import {labelColor} from '@/lib/tasks'
import {parseQuickAdd} from '@/lib/parse'
import {openTaskMenu, ui} from '@/lib/ui'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

interface Bucket {
	id: number
	title: string
	position: number
	count?: number
	tasks?: Task[] | null
}

const props = defineProps<{project: Project, selectedId: number | null, showCompleted: boolean}>()
const emit = defineEmits<{select: [number]}>()
const data = useData()

const STEP = 65536

const view = computed(() => props.project.views?.find(v => v.view_kind === 'kanban'))
const base = computed(() => `/projects/${props.project.id}/views/${view.value?.id}`)

const buckets = ref<Bucket[]>([])
/** Bucket and kanban position of each task, as last loaded or changed here. */
const placement = reactive(new Map<number, {bucket: number, position: number}>())
const loading = ref(false)
const defaultBucket = ref(0)
const doneBucket = ref(0)

async function load() {
	const v = view.value
	if (!v) return
	loading.value = true
	try {
		const detail = await api<{default_bucket_id?: number, done_bucket_id?: number}>('GET', base.value)
		defaultBucket.value = detail.default_bucket_id ?? 0
		doneBucket.value = detail.done_bucket_id ?? 0
		// Pages apply to the tasks inside each bucket; keep going while a bucket has more.
		const byId = new Map<number, Bucket>()
		for (let page = 1; page <= 20; page++) {
			const res = await api<{items?: Bucket[] | null}>('GET', `${base.value}/buckets/tasks`, {query: {page, expand: 'comment_count'}})
			const items = res.items ?? []
			for (const b of items) {
				const have = byId.get(b.id)
				if (have) have.tasks = [...(have.tasks ?? []), ...(b.tasks ?? [])]
				else byId.set(b.id, {...b, tasks: [...(b.tasks ?? [])]})
			}
			if (!items.some(b => (b.tasks?.length ?? 0) > 0 && (byId.get(b.id)!.tasks!.length < (b.count ?? 0)))) break
		}
		buckets.value = [...byId.values()].sort((a, b) => a.position - b.position)
		placement.clear()
		for (const b of buckets.value) {
			for (const task of b.tasks ?? []) {
				placement.set(task.id, {bucket: b.id, position: task.position ?? 0})
				if (!data.tasks.has(task.id)) data.tasks.set(task.id, task)
			}
		}
	} catch (e) {
		toast(String((e as Error).message))
	} finally {
		loading.value = false
	}
}

watch(() => props.project.id, load, {immediate: true})
// A full sync may have changed tasks elsewhere.
watch(() => data.loading, (now, before) => {
	if (before && !now) load()
})

function bucketOf(task: Task): number {
	const p = placement.get(task.id)
	if (task.done && doneBucket.value) return doneBucket.value
	if (p && !(p.bucket === doneBucket.value && !task.done)) return p.bucket
	return defaultBucket.value || buckets.value[0]?.id || 0
}

function posOf(task: Task): number {
	return placement.get(task.id)?.position ?? Number.MAX_SAFE_INTEGER
}

const columns = computed(() => {
	const tasks = data.allTasks.filter(task => task.project_id === props.project.id && (props.showCompleted || !task.done))
	return buckets.value.map(b => ({
		bucket: b,
		tasks: tasks.filter(task => bucketOf(task) === b.id).sort((x, y) => posOf(x) - posOf(y) || x.id - y.id),
	}))
})

// ---- moving cards ----
const dropAt = ref<{bucket: number, index: number} | null>(null)

function onCardOver(e: DragEvent, bucket: number, index: number) {
	if (ui.dragTaskId === null) return
	e.preventDefault()
	const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
	dropAt.value = {bucket, index: e.clientY > rect.top + rect.height / 2 ? index + 1 : index}
}

function onColumnOver(e: DragEvent, bucket: number, length: number) {
	if (ui.dragTaskId === null) return
	e.preventDefault()
	if (dropAt.value?.bucket !== bucket) dropAt.value = {bucket, index: length}
}

async function onDrop(e: DragEvent) {
	const at = dropAt.value
	const id = ui.dragTaskId
	dropAt.value = null
	if (!at || id === null) return
	e.preventDefault()
	const task = data.tasks.get(id)
	if (!task) return
	const col = columns.value.find(c => c.bucket.id === at.bucket)
	if (!col) return
	const others = col.tasks.filter(x => x.id !== id)
	const index = Math.min(at.index - (col.tasks.slice(0, at.index).some(x => x.id === id) ? 1 : 0), others.length)
	const prev = others[index - 1]
	const next = others[index]
	const pp = prev ? posOf(prev) : 0
	const np = next ? posOf(next) : pp + 2 * STEP
	let position = (pp + (np === Number.MAX_SAFE_INTEGER ? pp + 2 * STEP : np)) / 2
	if (!Number.isFinite(position) || position === pp) position = pp + STEP
	await moveTo(task, at.bucket, position)
}

async function moveTo(task: Task, bucket: number, position: number) {
	const before = placement.get(task.id)
	placement.set(task.id, {bucket, position})
	try {
		if (bucketOf(task) !== bucket || before?.bucket !== bucket) {
			await api('PUT', `${base.value}/buckets/${bucket}/tasks`, {body: {task_id: task.id}})
			// Entering or leaving the done bucket changes the task's done state on the server.
			const done = bucket === doneBucket.value
			if (doneBucket.value && done !== task.done) data.patchLocal(task.id, {done, done_at: done ? new Date().toISOString() : null})
		}
		await api('PUT', `/tasks/${task.id}/position`, {body: {position, project_view_id: view.value!.id}})
	} catch (e) {
		if (before) placement.set(task.id, before)
		else placement.delete(task.id)
		toast(String((e as Error).message))
	}
}

// ---- adding tasks ----
const adding = ref<number | null>(null)
const draft = ref('')

async function addTask(bucket: number) {
	const p = draft.value.trim() ? parseQuickAdd(draft.value) : null
	draft.value = ''
	if (!p?.title) return
	const body = {title: p.title, project_id: props.project.id, ...(p.date ? schedulePatch(p.date, p.allDay, p.end) : {}), ...(p.priority !== null ? {priority: p.priority} : {})}
	const created = await data.createTask(body, p.labels)
	if (!created) return
	const col = columns.value.find(c => c.bucket.id === bucket)
	const first = col?.tasks.find(x => x.id !== created.id)
	// New cards go to the top of their column, as in TickTick.
	await moveTo(created, bucket, first ? posOf(first) / 2 : STEP)
}

// ---- sections ----
const menu = ref<{bucket: Bucket, el: HTMLElement} | null>(null)
const renaming = ref<number | null>(null)
const renameDraft = ref('')
const addingSection = ref(false)
const sectionDraft = ref('')

async function addSection() {
	const title = sectionDraft.value.trim()
	sectionDraft.value = ''
	addingSection.value = false
	if (!title) return
	const last = buckets.value[buckets.value.length - 1]
	try {
		const b = await api<Bucket>('POST', `${base.value}/buckets`, {body: {title, position: (last?.position ?? 0) + STEP}})
		buckets.value = [...buckets.value, {...b, tasks: []}]
	} catch (e) {
		toast(String((e as Error).message))
	}
}

function startRename(b: Bucket) {
	menu.value = null
	renaming.value = b.id
	renameDraft.value = b.title
}

async function rename(b: Bucket) {
	const title = renameDraft.value.trim()
	renaming.value = null
	if (!title || title === b.title) return
	const before = b.title
	b.title = title
	try {
		await api('PUT', `${base.value}/buckets/${b.id}`, {body: {title, position: b.position}})
	} catch (e) {
		b.title = before
		toast(String((e as Error).message))
	}
}

async function removeSection(b: Bucket) {
	menu.value = null
	if (!confirm(t('deleteSectionConfirm', {name: b.title}))) return
	try {
		await api('DELETE', `${base.value}/buckets/${b.id}`)
		await load()
	} catch (e) {
		toast(String((e as Error).message))
	}
}

function onDragStart(e: DragEvent, task: Task) {
	ui.dragTaskId = task.id
	e.dataTransfer?.setData('application/x-task-id', String(task.id))
	e.dataTransfer?.setData('text/plain', task.title)
	if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragEnd() {
	ui.dragTaskId = null
	dropAt.value = null
}
</script>

<template>
	<div v-if="!view" class="empty-state">{{ t('noKanbanView') }}</div>
	<div v-else class="board">
		<section
			v-for="col in columns"
			:key="col.bucket.id"
			class="column"
			@dragover="onColumnOver($event, col.bucket.id, col.tasks.length)"
			@drop="onDrop"
		>
			<header class="col-head">
				<input
					v-if="renaming === col.bucket.id"
					v-model="renameDraft"
					v-focus
					class="input rename"
					@keydown.enter.prevent="rename(col.bucket)"
					@keydown.esc.stop="renaming = null"
					@blur="rename(col.bucket)"
				>
				<template v-else>
					<span class="col-title" @dblclick="startRename(col.bucket)">{{ col.bucket.title }}</span>
					<span class="col-count">{{ col.tasks.length }}</span>
				</template>
				<span class="spacer" />
				<button class="icon-btn small" :title="t('addTask')" @click="adding = col.bucket.id"><Plus :size="16" /></button>
				<button class="icon-btn small" @click="menu = {bucket: col.bucket, el: $event.currentTarget as HTMLElement}"><MoreHorizontal :size="16" /></button>
			</header>

			<div class="cards">
				<div v-if="adding === col.bucket.id" class="card add-card">
					<input
						v-model="draft"
						v-focus
						class="add-input"
						:placeholder="t('addTask')"
						@keydown.enter.prevent="addTask(col.bucket.id)"
						@keydown.esc.stop="draft = ''; adding = null"
						@blur="!draft.trim() && (adding = null)"
					>
				</div>
				<template v-for="(task, i) in col.tasks" :key="task.id">
					<div v-if="dropAt?.bucket === col.bucket.id && dropAt.index === i" class="drop-line" />
					<div
						class="card"
						:class="{selected: task.id === selectedId, done: task.done, dragging: ui.dragTaskId === task.id}"
						draggable="true"
						@click="emit('select', task.id)"
						@contextmenu="openTaskMenu(task.id, $event)"
						@dragstart="onDragStart($event, task)"
						@dragend="onDragEnd"
						@dragover.stop="onCardOver($event, col.bucket.id, i)"
					>
						<TaskCheckbox :done="task.done" :wont-do="task.wont_do" :priority="task.priority" @toggle="data.toggleDone(task.id)" />
						<div class="card-main">
							<div v-if="parentId(task) !== undefined" class="card-parent muted">{{ data.tasks.get(parentId(task)!)?.title ?? task.related_tasks?.parenttask?.[0]?.title }}</div>
							<div class="card-title">{{ task.title }}</div>
							<div v-if="taskDateLabel(task) || task.labels?.length" class="card-meta">
								<span v-if="taskDateLabel(task)" class="date" :class="{overdue: isOverdue(task)}">{{ taskDateLabel(task) }}</span>
								<span v-for="l in task.labels ?? []" :key="l.id" class="tag" :style="{'--tc': labelColor(l.hex_color)}">{{ l.title }}</span>
							</div>
						</div>
					</div>
				</template>
				<div v-if="dropAt?.bucket === col.bucket.id && dropAt.index >= col.tasks.length" class="drop-line" />
				<button v-if="adding !== col.bucket.id" class="add-row" @click="adding = col.bucket.id"><Plus :size="15" /> {{ t('addTask') }}</button>
			</div>
		</section>

		<section class="column new-col">
			<input
				v-if="addingSection"
				v-model="sectionDraft"
				v-focus
				class="input"
				:placeholder="t('sectionName')"
				@keydown.enter.prevent="addSection"
				@keydown.esc.stop="addingSection = false"
				@blur="addSection"
			>
			<button v-else class="add-row" @click="addingSection = true"><Plus :size="15" /> {{ t('addSection') }}</button>
		</section>

		<Popover v-if="menu" :anchor="menu.el" placement="bottom-end" @close="menu = null">
			<div class="menu">
				<button class="menu-item" @click="startRename(menu!.bucket)"><Pencil :size="15" /> {{ t('rename') }}</button>
				<button class="menu-item danger" :disabled="buckets.length <= 1" @click="removeSection(menu!.bucket)"><Trash2 :size="15" /> {{ t('delete') }}</button>
			</div>
		</Popover>
	</div>
</template>

<style scoped>
.board {
	display: flex;
	align-items: flex-start;
	gap: 14px;
	height: 100%;
	overflow-x: auto;
	padding: 0 20px 20px;
}

.column {
	display: flex;
	flex-direction: column;
	width: 272px;
	flex-shrink: 0;
	max-height: 100%;
	border-radius: 10px;
	background: var(--bg-hover);
}

.col-head {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 44px;
	padding: 0 6px 0 12px;
	flex-shrink: 0;
}

.col-title {
	font-weight: 600;
	font-size: 14px;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.col-count {
	font-size: 13px;
	color: var(--text-3);
}

.rename {
	height: 30px;
}

.spacer {
	flex: 1;
}

.icon-btn.small {
	width: 26px;
	height: 26px;
}

.cards {
	display: flex;
	flex-direction: column;
	gap: 6px;
	min-height: 40px;
	padding: 0 8px 8px;
	overflow-y: auto;
}

.card {
	display: flex;
	align-items: flex-start;
	gap: 10px;
	padding: 10px;
	border-radius: 8px;
	background: var(--bg);
	box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
	cursor: default;
}

.card :deep(.checkbox),
.card > :first-child {
	margin-top: 2px;
}

.card:hover {
	box-shadow: 0 0 0 1px var(--border-strong);
}

.card.selected {
	box-shadow: 0 0 0 1.5px var(--accent);
}

.card.dragging {
	opacity: 0.4;
}

.card-main {
	flex: 1;
	min-width: 0;
}

.card-parent {
	font-size: 11px;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.card-title {
	font-size: 14px;
	line-height: 1.4;
	overflow-wrap: anywhere;
}

.done .card-title {
	color: var(--text-3);
	text-decoration: line-through;
}

.card-meta {
	display: flex;
	flex-wrap: wrap;
	gap: 6px;
	margin-top: 6px;
	font-size: 12px;
}

.date {
	color: var(--accent);
}

.date.overdue {
	color: var(--danger);
}

.tag {
	padding: 0 7px;
	border-radius: 9px;
	color: color-mix(in srgb, var(--tc) 60%, var(--text));
	background: color-mix(in srgb, var(--tc) 30%, transparent);
}

.add-card {
	padding: 8px 10px;
}

.add-input {
	width: 100%;
	border: 0;
	outline: 0;
	background: transparent;
	font-size: 14px;
}

.add-row {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 34px;
	padding: 0 6px;
	border-radius: 6px;
	color: var(--text-3);
	font-size: 14px;
}

.add-row:hover {
	color: var(--text);
}

.drop-line {
	height: 2px;
	margin: -4px 4px;
	border-radius: 1px;
	background: var(--accent);
}

.new-col {
	padding: 6px;
	background: transparent;
	border: 1px dashed var(--border-strong);
}
</style>
