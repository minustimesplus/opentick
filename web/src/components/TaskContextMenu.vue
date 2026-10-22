<script setup lang="ts">
import {computed, ref} from 'vue'
import {Ban, CalendarPlus, ChevronRight, Copy, Flag, FolderInput, RotateCcw, Sun, Sunrise, Trash2, CalendarX} from 'lucide-vue-next'
import Popover from './Popover.vue'
import ListPicker from './ListPicker.vue'
import DatePicker from './DatePicker.vue'
import {useData} from '@/stores/data'
import {ui} from '@/lib/ui'
import {t} from '@/lib/i18n'
import {addDays, getSchedule, moveSchedule, schedulePatch, startOfDay, startOfWeek, withMinutes, minutesOfDay} from '@/lib/dates'
import {level, PRIORITY_COLORS, priorityName, type Level} from '@/lib/tasks'
import {toast} from '@/lib/toast'

const data = useData()
const task = computed(() => ui.ctx ? data.tasks.get(ui.ctx.taskId) : undefined)
const anchor = computed(() => ui.ctx ? {x: ui.ctx.x, y: ui.ctx.y} : null)
const moveEl = ref<HTMLElement | null>(null)
const sub = ref<'move' | 'date' | null>(null)

function close() {
	ui.ctx = null
	sub.value = null
}

function setDay(d: Date) {
	const tk = task.value
	if (!tk) return
	const s = getSchedule(tk)
	const patch = s && !s.allDay
		? moveSchedule(s, withMinutes(d, minutesOfDay(s.start)), false)
		: schedulePatch(d, true)
	data.updateTask(tk.id, patch)
	close()
}

function setPriority(l: Level) {
	if (task.value) data.updateTask(task.value.id, {priority: l})
	close()
}

function move(projectId: number) {
	if (task.value) data.updateTask(task.value.id, {project_id: projectId})
	close()
}

async function duplicate() {
	if (task.value) await data.duplicateTask(task.value.id)
	close()
}

function toggleWontDo() {
	if (task.value) data.toggleWontDo(task.value.id)
	close()
}

function remove() {
	const tk = task.value
	if (!tk) return
	close()
	data.deleteTask(tk.id)
	toast(`🗑 ${tk.title}`)
}

const today = startOfDay(new Date())
</script>

<template>
	<Popover v-if="task && anchor && sub !== 'date'" :anchor="anchor" placement="bottom-start" @close="close">
		<div class="menu ctx">
			<div class="menu-label">{{ t('date') }}</div>
			<div class="icons">
				<button :title="t('today')" @click="setDay(today)"><Sun :size="18" /></button>
				<button :title="t('tomorrow')" @click="setDay(addDays(today, 1))"><Sunrise :size="18" /></button>
				<button :title="t('nextWeek')" @click="setDay(addDays(startOfWeek(today, 1), 7))"><span class="plus7">+7</span></button>
				<button :title="t('date')" @click="sub = 'date'"><CalendarPlus :size="18" /></button>
				<button :title="t('clearDate')" @click="data.updateTask(task.id, schedulePatch(null, false)); close()"><CalendarX :size="18" /></button>
			</div>
			<div class="menu-label">{{ t('priority') }}</div>
			<div class="icons">
				<button v-for="l in ([3, 2, 1, 0] as Level[])" :key="l" :title="priorityName(l)" :class="{on: level(task.priority) === l}" @click="setPriority(l)">
					<Flag :size="18" :color="PRIORITY_COLORS[l]" :fill="l ? PRIORITY_COLORS[l] : 'none'" />
				</button>
			</div>
			<div class="menu-sep" />
			<button ref="moveEl" class="menu-item" @click="sub = 'move'"><FolderInput :size="16" /> {{ t('moveTo') }} <ChevronRight :size="14" class="end" /></button>
			<button class="menu-item" @click="toggleWontDo">
				<template v-if="task.done && task.wont_do"><RotateCcw :size="16" /> {{ t('reopen') }}</template>
				<template v-else><Ban :size="16" /> {{ t('wontDo') }}</template>
			</button>
			<button class="menu-item" @click="duplicate"><Copy :size="16" /> {{ t('duplicate') }}</button>
			<button class="menu-item danger" @click="remove"><Trash2 :size="16" /> {{ t('delete') }}</button>
		</div>
	</Popover>
	<Popover v-if="task && sub === 'move'" :anchor="moveEl" placement="right-start" @close="sub = null">
		<ListPicker :value="task.project_id" @select="move" />
	</Popover>
	<Popover v-if="task && sub === 'date' && anchor" :anchor="anchor" placement="bottom-start" @close="close">
		<DatePicker :task="task" @apply="p => data.updateTask(task!.id, p)" @close="close" />
	</Popover>
</template>

<style scoped>
.ctx {
	width: 220px;
}

.icons {
	display: flex;
	justify-content: space-between;
	padding: 2px 4px 6px;
}

.icons button {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 34px;
	height: 30px;
	border-radius: 6px;
	color: var(--text-2);
}

.icons button:hover,
.icons button.on {
	background: var(--bg-hover);
}

.plus7 {
	font-size: 11px;
	font-weight: 700;
	border: 1.5px solid currentColor;
	border-radius: 4px;
	padding: 0 2px;
}

.end {
	margin-left: auto;
	color: var(--text-3);
}
</style>
