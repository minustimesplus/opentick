<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {Plus, CalendarDays, Flag, Hash, ListIcon} from 'lucide-vue-next'
import {useData, type TaskPatch} from '@/stores/data'
import {parseQuickAdd} from '@/lib/parse'
import {schedulePatch, scheduleLabel} from '@/lib/dates'
import {PRIORITY_COLORS, priorityName, splitEmoji, type Level} from '@/lib/tasks'
import {t} from '@/lib/i18n'
import {ui} from '@/lib/ui'

const props = defineProps<{
	placeholder?: string
	/** Defaults applied to every task created here (list, date, priority). */
	defaults?: TaskPatch & {project_id?: number}
	defaultLabels?: string[]
}>()
const emit = defineEmits<{created: [number]}>()

const data = useData()
const text = ref('')
const focused = ref(false)
const input = ref<HTMLInputElement | null>(null)

watch(() => ui.focusQuickAdd, () => input.value?.focus())

const parsed = computed(() => text.value.trim() ? parseQuickAdd(text.value) : null)

const parsedList = computed(() => {
	const name = parsed.value?.list?.toLowerCase()
	if (!name) return null
	return data.realProjects.find(p => splitEmoji(p.title).name.toLowerCase().startsWith(name)) ?? null
})

const chips = computed(() => {
	const p = parsed.value
	if (!p) return []
	const out: {icon: unknown, label: string, color?: string}[] = []
	if (p.date) out.push({icon: CalendarDays, label: scheduleLabel({date: p.date, allDay: p.allDay, start: p.date, end: p.end})})
	if (p.priority !== null) out.push({icon: Flag, label: priorityName(p.priority as Level), color: PRIORITY_COLORS[p.priority as Level]})
	p.labels.forEach(l => out.push({icon: Hash, label: l}))
	if (parsedList.value) out.push({icon: ListIcon, label: splitEmoji(parsedList.value.title).name})
	return out
})

async function submit() {
	const p = parsed.value
	if (!p || !p.title) return
	const body: TaskPatch & {title: string} = {...(props.defaults ?? {}), title: p.title}
	if (p.date) Object.assign(body, schedulePatch(p.date, p.allDay, p.end))
	if (p.priority !== null) body.priority = p.priority
	if (parsedList.value) body.project_id = parsedList.value.id
	text.value = ''
	const created = await data.createTask(body, [...(props.defaultLabels ?? []), ...p.labels])
	if (created) emit('created', created.id)
}
</script>

<template>
	<div class="qa" :class="{focused}">
		<div class="bar">
			<Plus :size="18" class="plus" />
			<input
				ref="input"
				v-model="text"
				:placeholder="placeholder ?? t('addTask')"
				@focus="focused = true"
				@blur="focused = false"
				@keydown.enter.prevent="submit"
				@keydown.esc="text = ''; input?.blur()"
			>
		</div>
		<div v-if="chips.length" class="chips">
			<span v-for="(c, i) in chips" :key="i" class="chip" :style="c.color ? {color: c.color} : {}">
				<component :is="c.icon" :size="12" /> {{ c.label }}
			</span>
		</div>
	</div>
</template>

<style scoped>
.qa {
	border-radius: 8px;
	background: var(--bg-input);
	border: 1px solid transparent;
}

.qa.focused {
	border-color: var(--accent);
	background: var(--bg);
}

.bar {
	display: flex;
	align-items: center;
	gap: 8px;
	height: 42px;
	padding: 0 14px;
}

.plus {
	color: var(--text-3);
}

input {
	flex: 1;
	border: 0;
	outline: 0;
	background: transparent;
	font-size: 14px;
}

input::placeholder {
	color: var(--text-3);
}

.chips {
	display: flex;
	flex-wrap: wrap;
	gap: 6px;
	padding: 0 14px 10px 40px;
}

.chip {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	padding: 2px 8px;
	border-radius: 10px;
	font-size: 12px;
	color: var(--accent);
	background: var(--accent-soft);
}
</style>
