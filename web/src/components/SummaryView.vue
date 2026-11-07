<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {Copy} from 'lucide-vue-next'
import SettingSelect from './settings/SettingSelect.vue'
import TaskCheckbox from './TaskCheckbox.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {t, type MessageKey} from '@/lib/i18n'
import {addDays, addMonths, dateKey, fmtLongDate, getSchedule, parseDate, startOfDay, startOfMonth, startOfWeek} from '@/lib/dates'
import {splitEmoji} from '@/lib/tasks'
import {toast} from '@/lib/toast'

/**
 * Summary smart list: a report of what was done (and left undone) over a period,
 * grouped by day or by list, that can be copied as Markdown.
 */

const emit = defineEmits<{select: [number]}>()
const data = useData()

type Range = 'today' | 'yesterday' | 'thisWeek' | 'lastWeek' | 'thisMonth' | 'lastMonth'
type Status = 'completed' | 'wontDo' | 'undone'
type GroupBy = 'date' | 'list'

const range = ref<Range>('thisWeek')
const groupBy = ref<GroupBy>('date')
const statuses = ref<Record<Status, boolean>>({completed: true, wontDo: true, undone: false})

const rangeLabels: Record<Range, MessageKey> = {
	today: 'today',
	yesterday: 'yesterday',
	thisWeek: 'thisWeek',
	lastWeek: 'summaryLastWeek',
	thisMonth: 'summaryThisMonth',
	lastMonth: 'summaryLastMonth',
}
const statusLabels: Record<Status, MessageKey> = {completed: 'completed', wontDo: 'wontDo', undone: 'summaryUndone'}

const rangeOptions = computed(() => (Object.keys(rangeLabels) as Range[]).map(value => ({value, label: t(rangeLabels[value])})))
const groupOptions = computed(() => [
	{value: 'date' as const, label: t('date')},
	{value: 'list' as const, label: t('summaryByList')},
])

/** [from, to) of the picked period. */
const bounds = computed<[Date, Date]>(() => {
	const today = startOfDay(new Date())
	switch (range.value) {
		case 'today': return [today, addDays(today, 1)]
		case 'yesterday': return [addDays(today, -1), today]
		case 'thisWeek': {
			const from = startOfWeek(today)
			return [from, addDays(from, 7)]
		}
		case 'lastWeek': {
			const from = addDays(startOfWeek(today), -7)
			return [from, addDays(from, 7)]
		}
		case 'thisMonth': {
			const from = startOfMonth(today)
			return [from, addMonths(from, 1)]
		}
		case 'lastMonth': {
			const from = addMonths(startOfMonth(today), -1)
			return [from, addMonths(from, 1)]
		}
	}
	return [today, addDays(today, 1)]
})

const loading = ref(false)
watch(bounds, async ([from, to]) => {
	loading.value = true
	try {
		await data.loadClosedBetween(from, to)
	} catch (e) {
		toast(String((e as Error).message))
	} finally {
		loading.value = false
	}
}, {immediate: true})

function inRange(d: Date | null): d is Date {
	const [from, to] = bounds.value
	return !!d && d >= from && d < to
}

function statusOf(task: Task): Status {
	if (!task.done) return 'undone'
	return task.wont_do ? 'wontDo' : 'completed'
}

/** The day a task belongs to: when it was closed, or when it is scheduled if still open. */
function dayOf(task: Task): Date | null {
	return task.done ? parseDate(task.done_at) : getSchedule(task)?.date ?? null
}

const tasks = computed(() => data.allTasks
	.filter(task => statuses.value[statusOf(task)] && inRange(dayOf(task)) && data.project(task.project_id))
	.sort((a, b) => dayOf(a)!.getTime() - dayOf(b)!.getTime() || a.id - b.id))

interface Group {
	key: string
	title: string
	tasks: Task[]
}

function listName(task: Task) {
	const p = data.project(task.project_id)
	return p ? splitEmoji(p.title).name : ''
}

const groups = computed<Group[]>(() => {
	const out = new Map<string, Group>()
	for (const task of tasks.value) {
		const key = groupBy.value === 'date' ? dateKey(dayOf(task)!) : `p${task.project_id}`
		if (!out.has(key)) {
			const title = groupBy.value === 'date' ? fmtLongDate(dayOf(task)!) : data.project(task.project_id)?.title ?? ''
			out.set(key, {key, title, tasks: []})
		}
		out.get(key)!.tasks.push(task)
	}
	const list = [...out.values()]
	if (groupBy.value === 'list') list.sort((a, b) => a.title.localeCompare(b.title))
	return list
})

const markdown = computed(() => {
	const mark: Record<Status, string> = {completed: '[x]', wontDo: '[-]', undone: '[ ]'}
	const lines = [`# ${t('summary')} · ${t(rangeLabels[range.value])}`, '']
	for (const g of groups.value) {
		lines.push(`## ${g.title}`)
		for (const task of g.tasks) {
			const suffix = groupBy.value === 'date' ? ` (${listName(task)})` : ''
			lines.push(`- ${mark[statusOf(task)]} ${task.title}${suffix}`)
		}
		lines.push('')
	}
	return lines.join('\n').trim() + '\n'
})

async function copy() {
	try {
		await navigator.clipboard.writeText(markdown.value)
		toast(t('copied'))
	} catch (e) {
		toast(String((e as Error).message))
	}
}

function toggleDone(task: Task) {
	if (task.wont_do) data.toggleWontDo(task.id)
	else data.toggleDone(task.id)
}
</script>

<template>
	<div class="summary">
		<div class="toolbar">
			<SettingSelect v-model="range" :options="rangeOptions" />
			<span class="sep" />
			<label v-for="(label, s) in statusLabels" :key="s" class="chip" :class="{on: statuses[s]}">
				<input v-model="statuses[s]" type="checkbox">
				{{ t(label) }}
			</label>
			<span class="sep" />
			<span class="muted">{{ t('summaryGroupBy') }}</span>
			<SettingSelect v-model="groupBy" :options="groupOptions" />
			<div class="spacer" />
			<button class="btn" :disabled="!tasks.length" @click="copy"><Copy :size="15" /> {{ t('summaryCopy') }}</button>
		</div>

		<div class="doc">
			<template v-for="g in groups" :key="g.key">
				<h3>{{ g.title }}</h3>
				<div v-for="task in g.tasks" :key="task.id" class="line" @click="emit('select', task.id)">
					<TaskCheckbox :done="task.done" :wont-do="task.wont_do" :priority="task.priority" :size="15" @toggle="toggleDone(task)" />
					<span class="title" :class="{closed: task.done}">{{ task.title }}</span>
					<span v-if="groupBy === 'date'" class="list">{{ listName(task) }}</span>
				</div>
			</template>
			<div v-if="!groups.length && !loading" class="empty-state">{{ t('summaryEmpty') }}</div>
		</div>
	</div>
</template>

<style scoped>
.summary {
	display: flex;
	flex-direction: column;
	gap: 12px;
}

.toolbar {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 8px;
	font-size: 13px;
}

.toolbar .sep {
	width: 1px;
	height: 16px;
	background: var(--border);
}

.chip {
	display: inline-flex;
	align-items: center;
	height: 26px;
	padding: 0 10px;
	border-radius: 13px;
	border: 1px solid var(--border);
	color: var(--text-2);
	cursor: pointer;
	user-select: none;
}

.chip input {
	display: none;
}

.chip.on {
	border-color: var(--accent);
	color: var(--accent);
	background: color-mix(in srgb, var(--accent) 10%, transparent);
}

.muted {
	color: var(--text-3);
}

.spacer {
	flex: 1;
}

.toolbar .btn {
	display: inline-flex;
	align-items: center;
	gap: 6px;
	height: 28px;
}

.doc {
	padding: 4px 4px 20px;
}

h3 {
	margin: 18px 0 6px;
	font-size: 14px;
	font-weight: 600;
	color: var(--text-2);
}

h3:first-child {
	margin-top: 6px;
}

.line {
	display: flex;
	align-items: center;
	gap: 10px;
	min-height: 34px;
	padding: 0 8px;
	border-radius: 6px;
	cursor: pointer;
}

.line:hover {
	background: var(--bg-hover);
}

.title {
	flex: 1;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 14px;
}

.title.closed {
	color: var(--text-3);
}

.list {
	font-size: 12px;
	color: var(--text-3);
	white-space: nowrap;
}

.empty-state {
	height: 40vh;
}
</style>
