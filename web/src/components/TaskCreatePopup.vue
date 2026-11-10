<script setup lang="ts">
import {computed, ref} from 'vue'
import {CalendarDays, Flag, Inbox, Menu} from 'lucide-vue-next'
import Popover, {type Anchor} from './Popover.vue'
import DatePicker from './DatePicker.vue'
import PriorityMenu from './PriorityMenu.vue'
import ListPicker from './ListPicker.vue'
import MarkdownEditor from './MarkdownEditor.vue'
import type {Task} from '@/api/types'
import {useData, type TaskPatch} from '@/stores/data'
import {parseQuickAdd} from '@/lib/parse'
import {getSchedule, scheduleLabel, schedulePatch} from '@/lib/dates'
import {PRIORITY_COLORS, splitEmoji, type Level} from '@/lib/tasks'
import {t} from '@/lib/i18n'
import {prefs} from '@/lib/prefs'

/**
 * TickTick-style "new task" card: date and priority on top, title and
 * description in the middle, list at the bottom. Saves on Enter, or when
 * closed with a title typed in.
 */
const props = withDefaults(defineProps<{
	anchor: Anchor | null
	placement?: 'bottom-start' | 'bottom-end' | 'right-start' | 'left-start' | 'top-start'
	priority?: Level
	projectId?: number
}>(), {placement: 'right-start', priority: 0, projectId: undefined})
const emit = defineEmits<{close: [], created: [number]}>()

const data = useData()
const title = ref('')
const description = ref('')
const priority = ref<Level>(props.priority)
const projectId = ref(props.projectId || prefs.defaultListId || data.inboxId)
const datePatch = ref<TaskPatch | null>(null)
const pop = ref<'date' | 'prio' | 'list' | null>(null)
const dateEl = ref<HTMLElement | null>(null)
const prioEl = ref<HTMLElement | null>(null)
const listEl = ref<HTMLElement | null>(null)
let saving = false

/** A stand-in task so the date picker can show what's been picked so far. */
const draft = computed(() => ({...(datePatch.value ?? {})}) as Task)
const schedule = computed(() => datePatch.value ? getSchedule(draft.value) : null)
const dateLabel = computed(() => schedule.value ? scheduleLabel(schedule.value) : t('date'))

const listLabel = computed(() => {
	if (projectId.value === data.inboxId) return {inbox: true, emoji: null, name: t('inbox')}
	const p = data.project(projectId.value)
	return {inbox: false, ...splitEmoji(p?.title ?? '')}
})

async function save() {
	if (saving) return
	const text = title.value.trim()
	if (!text) {
		emit('close')
		return
	}
	saving = true
	const listNames = data.realProjects.map(p => splitEmoji(p.title).name)
	const parsed = parseQuickAdd(text, new Date(), listNames)
	const body: TaskPatch & {title: string, project_id?: number} = {
		...(datePatch.value ?? {}),
		title: parsed.title || text,
		priority: priority.value,
		project_id: projectId.value,
	}
	if (description.value.trim()) body.description = description.value
	if (parsed.date) Object.assign(body, schedulePatch(parsed.date, parsed.allDay, parsed.end))
	if (parsed.priority !== null) body.priority = parsed.priority
	const name = parsed.list?.toLowerCase()
	const list = name ? data.realProjects.find(p => splitEmoji(p.title).name.toLowerCase().startsWith(name)) : null
	if (list) body.project_id = list.id
	emit('close')
	const created = await data.createTask(body, parsed.labels)
	if (created) emit('created', created.id)
}

function onClose() {
	// Clicking away keeps what was typed, like TickTick.
	if (pop.value) return
	save()
}
</script>

<template>
	<Popover :anchor="anchor" :placement="placement" @close="onClose">
		<div class="tcp">
			<header class="tcp-head">
				<button ref="dateEl" class="tcp-date" :class="{set: !!schedule}" @click="pop = 'date'">
					<CalendarDays :size="16" /> {{ dateLabel }}
				</button>
				<button ref="prioEl" class="icon-btn" :title="t('priority')" @click="pop = 'prio'">
					<Flag :size="18" :color="PRIORITY_COLORS[priority]" :fill="priority ? PRIORITY_COLORS[priority] : 'none'" />
				</button>
			</header>
			<div class="tcp-body">
				<input
					v-model="title"
					v-focus
					class="tcp-title"
					:placeholder="t('titlePlaceholder')"
					@keydown.enter.prevent="save"
				>
				<MarkdownEditor v-model="description" class="tcp-desc" :placeholder="t('descriptionPlaceholder')" />
			</div>
			<footer class="tcp-foot">
				<button ref="listEl" class="tcp-list" @click="pop = 'list'">
					<Inbox v-if="listLabel.inbox" :size="15" />
					<span v-else-if="listLabel.emoji">{{ listLabel.emoji }}</span>
					<Menu v-else :size="15" />
					{{ listLabel.name }}
				</button>
			</footer>
		</div>
	</Popover>

	<Popover v-if="pop === 'date'" :anchor="dateEl" @close="pop = null">
		<DatePicker :task="draft" @apply="p => datePatch = {...(datePatch ?? {}), ...p}" @close="pop = null" />
	</Popover>
	<Popover v-if="pop === 'prio'" :anchor="prioEl" placement="bottom-end" @close="pop = null">
		<PriorityMenu :value="priority" @select="l => { priority = l; pop = null }" />
	</Popover>
	<Popover v-if="pop === 'list'" :anchor="listEl" placement="top-start" @close="pop = null">
		<ListPicker :value="projectId" @select="id => { projectId = id; pop = null }" />
	</Popover>
</template>

<style scoped>
.tcp {
	display: flex;
	flex-direction: column;
	width: 420px;
	max-width: calc(100vw - 32px);
	min-height: 320px;
	background: var(--bg-elev);
	border-radius: 12px;
	box-shadow: var(--shadow);
}

.tcp-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 10px 12px 10px 16px;
	border-bottom: 1px solid var(--border);
}

.tcp-date {
	display: flex;
	align-items: center;
	gap: 8px;
	height: 30px;
	padding: 0 8px;
	margin-left: -8px;
	border-radius: 6px;
	color: var(--text-3);
	font-size: 14px;
}

.tcp-date.set {
	color: var(--accent);
}

.tcp-date:hover {
	background: var(--bg-hover);
}

.tcp-body {
	flex: 1;
	display: flex;
	flex-direction: column;
	gap: 6px;
	padding: 14px 18px;
	min-height: 0;
}

.tcp-title {
	border: none;
	outline: none;
	background: none;
	padding: 0;
	font-size: 18px;
	font-weight: 600;
	color: var(--text);
}

.tcp-desc {
	flex: 1;
	max-height: 40vh;
	overflow: auto;
}

.tcp-foot {
	display: flex;
	align-items: center;
	padding: 8px 12px 10px;
}

.tcp-list {
	display: flex;
	align-items: center;
	gap: 6px;
	height: 30px;
	padding: 0 8px;
	border-radius: 6px;
	color: var(--text-2);
	font-size: 14px;
}

.tcp-list:hover {
	background: var(--bg-hover);
}
</style>
