<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {CalendarDays, ChevronLeft, Copy, Flag, Inbox, ListTree, Menu, MoreHorizontal, Trash2, Unlink, X, Tag} from 'lucide-vue-next'
import Popover from './Popover.vue'
import TaskCheckbox from './TaskCheckbox.vue'
import DatePicker from './DatePicker.vue'
import PriorityMenu from './PriorityMenu.vue'
import ListPicker from './ListPicker.vue'
import MarkdownEditor from './MarkdownEditor.vue'
import SubtaskList from './SubtaskList.vue'
import {parentId, useData} from '@/stores/data'
import {fmtDayMonth, fmtTime, getSchedule, isOverdue, parseDate, scheduleLabel} from '@/lib/dates'
import {labelColor, level, PRIORITY_COLORS, splitEmoji} from '@/lib/tasks'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

const props = defineProps<{taskId: number, closable?: boolean}>()
const emit = defineEmits<{close: [], deleted: [], open: [number]}>()
const data = useData()

const task = computed(() => data.tasks.get(props.taskId))
const schedule = computed(() => task.value ? getSchedule(task.value) : null)
const overdue = computed(() => task.value ? isOverdue(task.value) : false)

const title = ref(task.value?.title ?? '')
const description = ref(task.value?.description ?? '')
const titleFocused = ref(false)
const descFocused = ref(false)

watch(() => task.value?.title, v => {
	if (!titleFocused.value && v !== undefined) title.value = v
})
watch(() => task.value?.description, v => {
	if (!descFocused.value && v !== undefined) description.value = v
})

let titleTimer: number | undefined
let descTimer: number | undefined

function saveTitle() {
	clearTimeout(titleTimer)
	const v = title.value.trim()
	if (task.value && v && v !== task.value.title) data.updateTask(props.taskId, {title: v})
}

function saveDescription() {
	clearTimeout(descTimer)
	if (task.value && description.value !== (task.value.description ?? '')) {
		data.updateTask(props.taskId, {description: description.value})
	}
}

watch(title, () => {
	clearTimeout(titleTimer)
	titleTimer = window.setTimeout(saveTitle, 800)
})
watch(description, () => {
	clearTimeout(descTimer)
	descTimer = window.setTimeout(saveDescription, 1000)
})

onBeforeUnmount(() => {
	saveTitle()
	saveDescription()
})

const titleEl = ref<HTMLTextAreaElement | null>(null)
const descEl = ref<InstanceType<typeof MarkdownEditor> | null>(null)

function autosize() {
	const el = titleEl.value
	if (!el) return
	el.style.height = 'auto'
	el.style.height = el.scrollHeight + 'px'
}

onMounted(() => nextTick(autosize))
watch(title, () => nextTick(autosize))

// popovers
const dateEl = ref<HTMLElement | null>(null)
const prioEl = ref<HTMLElement | null>(null)
const listEl = ref<HTMLElement | null>(null)
const moreEl = ref<HTMLElement | null>(null)
const tagEl = ref<HTMLElement | null>(null)
const pop = ref<'date' | 'prio' | 'list' | 'more' | 'tag' | null>(null)

const project = computed(() => task.value ? data.project(task.value.project_id) : undefined)

// subtasks
const subtaskEl = ref<InstanceType<typeof SubtaskList> | null>(null)
const parent = computed(() => {
	const id = task.value ? parentId(task.value) : undefined
	if (id === undefined) return null
	return data.tasks.get(id) ?? task.value?.related_tasks?.parenttask?.[0] ?? null
})

function addSubtask() {
	pop.value = null
	subtaskEl.value?.focusAdd()
}

function detach() {
	pop.value = null
	data.setParent(props.taskId, null)
}
const listLabel = computed(() => {
	if (!project.value) return {emoji: null, name: '—'}
	if (project.value.id === data.inboxId) return {emoji: null, name: t('inbox'), inbox: true}
	return splitEmoji(project.value.title)
})

// tags
const tagQuery = ref('')
const tagSuggestions = computed(() => {
	const have = new Set(task.value?.labels?.map(l => l.id))
	const q = tagQuery.value.trim().toLowerCase()
	return data.labels.filter(l => !have.has(l.id) && (!q || l.title.toLowerCase().includes(q))).slice(0, 8)
})
const canCreateTag = computed(() => {
	const q = tagQuery.value.trim()
	return q && !data.labels.some(l => l.title.toLowerCase() === q.toLowerCase())
})

async function addTag(title?: string) {
	const name = (title ?? tagQuery.value).trim()
	if (!name) return
	tagQuery.value = ''
	await data.addLabelByTitle(props.taskId, name)
}

async function remove() {
	const tk = task.value
	if (!tk) return
	pop.value = null
	emit('deleted')
	await data.deleteTask(tk.id)
	toast(`🗑 ${tk.title}`)
}

async function duplicate() {
	pop.value = null
	await data.duplicateTask(props.taskId)
}

const meta = computed(() => {
	const tk = task.value
	if (!tk) return ''
	if (tk.done) {
		const d = parseDate(tk.done_at)
		return d ? t('completedOn', {date: `${fmtDayMonth(d)} ${fmtTime(d)}`}) : ''
	}
	const c = parseDate(tk.created)
	return c ? `${t('created')} ${fmtDayMonth(c)}` : ''
})
</script>

<template>
	<div v-if="task" class="detail">
		<header class="head">
			<TaskCheckbox :done="task.done" :priority="task.priority" :size="18" @toggle="data.toggleDone(task.id)" />
			<div class="vsep" />
			<button ref="dateEl" class="date-btn" :class="{set: !!schedule, overdue}" @click="pop = 'date'">
				<CalendarDays :size="16" />
				<span>{{ schedule ? scheduleLabel(schedule) : t('date') }}</span>
			</button>
			<div class="spacer" />
			<button ref="prioEl" class="icon-btn" :title="t('priority')" @click="pop = 'prio'">
				<Flag :size="18" :color="PRIORITY_COLORS[level(task.priority)]" :fill="level(task.priority) ? PRIORITY_COLORS[level(task.priority)] : 'none'" />
			</button>
			<button v-if="closable" class="icon-btn" @click="emit('close')"><X :size="18" /></button>
		</header>

		<div class="body">
			<button v-if="parent" class="parent-link" @click="emit('open', parent.id)">
				<ChevronLeft :size="14" /> <span>{{ parent.title }}</span>
			</button>
			<textarea
				ref="titleEl"
				v-model="title"
				class="title"
				:class="{done: task.done}"
				rows="1"
				:placeholder="t('titlePlaceholder')"
				@focus="titleFocused = true"
				@blur="titleFocused = false; saveTitle()"
				@keydown.enter.prevent="($event.target as HTMLTextAreaElement).blur(); descEl?.focus()"
			/>
			<MarkdownEditor
					ref="descEl"
					v-model="description"
					:placeholder="t('description')"
					@focus="descFocused = true"
					@blur="descFocused = false; saveDescription()"
				/>

			<SubtaskList ref="subtaskEl" :parent-id="task.id" @open="id => emit('open', id)" />

			<div class="tags">
				<span v-for="l in task.labels ?? []" :key="l.id" class="tag" :style="{'--tc': labelColor(l.hex_color)}">
					{{ l.title }}
					<button @click="data.removeLabel(task.id, l.id)"><X :size="11" /></button>
				</span>
				<button ref="tagEl" class="add-tag" @click="pop = 'tag'"><Tag :size="13" /> {{ t('tags_add') }}</button>
			</div>
		</div>

		<footer class="foot">
			<button ref="listEl" class="list-btn" @click="pop = 'list'">
				<Inbox v-if="'inbox' in listLabel" :size="15" />
				<span v-else-if="listLabel.emoji">{{ listLabel.emoji }}</span>
				<Menu v-else :size="15" />
				{{ listLabel.name }}
			</button>
			<span class="muted small">{{ meta }}</span>
			<button ref="moreEl" class="icon-btn" @click="pop = 'more'"><MoreHorizontal :size="18" /></button>
		</footer>

		<Popover v-if="pop === 'date'" :anchor="dateEl" @close="pop = null">
			<DatePicker :task="task" @apply="p => data.updateTask(task!.id, p)" @close="pop = null" />
		</Popover>
		<Popover v-if="pop === 'prio'" :anchor="prioEl" placement="bottom-end" @close="pop = null">
			<PriorityMenu :value="task.priority" @select="l => { data.updateTask(task!.id, {priority: l}); pop = null }" />
		</Popover>
		<Popover v-if="pop === 'list'" :anchor="listEl" placement="top-start" @close="pop = null">
			<ListPicker :value="task.project_id" @select="id => { data.updateTask(task!.id, {project_id: id}); pop = null }" />
		</Popover>
		<Popover v-if="pop === 'more'" :anchor="moreEl" placement="top-start" @close="pop = null">
			<div class="menu">
				<button class="menu-item" @click="addSubtask"><ListTree :size="16" /> {{ t('addSubtask') }}</button>
				<button v-if="parent" class="menu-item" @click="detach"><Unlink :size="16" /> {{ t('detachSubtask') }}</button>
				<div class="menu-sep" />
				<button class="menu-item" @click="duplicate"><Copy :size="16" /> {{ t('duplicate') }}</button>
				<button class="menu-item danger" @click="remove"><Trash2 :size="16" /> {{ t('delete') }}</button>
			</div>
		</Popover>
		<Popover v-if="pop === 'tag'" :anchor="tagEl" @close="pop = null">
			<div class="menu tag-menu">
				<input v-model="tagQuery" class="input" :placeholder="t('tags_add')" v-focus @keydown.enter.prevent="addTag()">
				<button v-for="l in tagSuggestions" :key="l.id" class="menu-item" @click="data.addLabel(task.id, l)">
					<span class="dot" :style="{background: labelColor(l.hex_color)}" /> {{ l.title }}
				</button>
				<button v-if="canCreateTag" class="menu-item" @click="addTag()">+ {{ t('createTag', {name: tagQuery.trim()}) }}</button>
			</div>
		</Popover>
	</div>
</template>

<style scoped>
.detail {
	display: flex;
	flex-direction: column;
	height: 100%;
	min-height: 0;
}

.head {
	display: flex;
	align-items: center;
	gap: 10px;
	height: 56px;
	padding: 0 16px 0 20px;
	border-bottom: 1px solid var(--border);
	flex-shrink: 0;
}

.vsep {
	width: 1px;
	height: 18px;
	background: var(--border-strong);
}

.date-btn {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 30px;
	padding: 0 8px;
	border-radius: 6px;
	color: var(--text-3);
}

.date-btn:hover {
	background: var(--bg-hover);
}

.date-btn.set {
	color: var(--accent);
}

.date-btn.overdue {
	color: var(--danger);
}

.spacer {
	flex: 1;
}

.body {
	flex: 1;
	overflow: auto;
	padding: 16px 20px;
}

textarea {
	width: 100%;
	border: 0;
	outline: 0;
	resize: none;
	background: transparent;
	overflow: hidden;
}

.parent-link {
	display: flex;
	align-items: center;
	gap: 2px;
	max-width: 100%;
	margin: -4px 0 6px -4px;
	padding: 2px 4px;
	border-radius: 4px;
	font-size: 13px;
	color: var(--text-3);
}

.parent-link span {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.parent-link:hover {
	color: var(--text);
	background: var(--bg-hover);
}

.title {
	font-size: 20px;
	font-weight: 600;
	line-height: 1.35;
	margin-bottom: 8px;
}

.title.done {
	color: var(--text-3);
}

textarea::placeholder {
	color: var(--text-4);
}

.tags {
	display: flex;
	flex-wrap: wrap;
	gap: 6px;
	margin-top: 12px;
}

.tag {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	padding: 2px 6px 2px 10px;
	border-radius: 12px;
	font-size: 12px;
	color: color-mix(in srgb, var(--tc) 60%, var(--text));
	background: color-mix(in srgb, var(--tc) 30%, transparent);
}

.tag button {
	display: flex;
	opacity: 0.7;
}

.add-tag {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	padding: 2px 8px;
	border-radius: 12px;
	font-size: 12px;
	color: var(--text-3);
	border: 1px dashed var(--border-strong);
}

.foot {
	display: flex;
	align-items: center;
	gap: 10px;
	height: 48px;
	padding: 0 12px 0 16px;
	border-top: 1px solid var(--border);
	flex-shrink: 0;
}

.list-btn {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 30px;
	padding: 0 8px;
	border-radius: 6px;
	color: var(--text-2);
	max-width: 50%;
	overflow: hidden;
	white-space: nowrap;
	text-overflow: ellipsis;
}

.list-btn:hover {
	background: var(--bg-hover);
}

.small {
	flex: 1;
	text-align: right;
	font-size: 12px;
}

.tag-menu {
	width: 220px;
}

.tag-menu .input {
	height: 30px;
	margin-bottom: 4px;
}

.dot {
	width: 8px;
	height: 8px;
	border-radius: 50%;
}
</style>
