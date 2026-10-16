<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {ChevronLeft, ChevronRight} from 'lucide-vue-next'
import type {Task} from '@/api/types'
import {addDays, addMonths, dateKey, fmtMonthYear, fmtWeekdayNarrow, getSchedule, isSameDay, startOfDay, startOfMonth, startOfWeek} from '@/lib/dates'

const props = defineProps<{selected: Date, tasks: Task[], rangeStart: Date, rangeEnd: Date}>()
const emit = defineEmits<{pick: [Date]}>()

const month = ref(startOfMonth(props.selected))
watch(() => props.selected, d => { month.value = startOfMonth(d) })

const weeks = computed(() => {
	const first = startOfWeek(month.value)
	return Array.from({length: 6}, (_, w) => Array.from({length: 7}, (_, d) => addDays(first, w * 7 + d)))
})

const busy = computed(() => {
	const set = new Set<string>()
	for (const task of props.tasks) {
		const s = getSchedule(task)
		if (s && !task.done) set.add(dateKey(s.date))
	}
	return set
})

const today = startOfDay(new Date())

function inRange(d: Date) {
	return d >= props.rangeStart && d < props.rangeEnd
}
</script>

<template>
	<div class="mm">
		<div class="mm-head">
			<span class="title">{{ fmtMonthYear(month) }}</span>
			<button class="nav" @click="month = addMonths(month, -1)"><ChevronLeft :size="16" /></button>
			<button class="nav" @click="emit('pick', today)">○</button>
			<button class="nav" @click="month = addMonths(month, 1)"><ChevronRight :size="16" /></button>
		</div>
		<div class="grid">
			<span v-for="d in weeks[0]" :key="'h' + d.getDay()" class="wd">{{ fmtWeekdayNarrow(d) }}</span>
		</div>
		<div v-for="week in weeks" :key="dateKey(week[0])" class="grid week" :class="{sel: inRange(week[0]) && inRange(week[6])}">
			<button
				v-for="d in week"
				:key="dateKey(d)"
				class="day"
				:class="{other: d.getMonth() !== month.getMonth(), today: isSameDay(d, today), inrange: inRange(d)}"
				@click="emit('pick', d)"
			>
				{{ d.getDate() }}
				<span v-if="busy.has(dateKey(d))" class="dot" />
			</button>
		</div>
	</div>
</template>

<style scoped>
.mm {
	padding: 4px 4px 8px;
	user-select: none;
}

.mm-head {
	display: flex;
	align-items: center;
	gap: 2px;
	margin-bottom: 10px;
	padding-left: 8px;
}

.title {
	flex: 1;
	font-weight: 500;
}

.nav {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 24px;
	height: 24px;
	border-radius: 5px;
	color: var(--text-3);
	font-size: 11px;
}

.nav:hover {
	background: var(--bg-hover);
	color: var(--text);
}

.grid {
	display: grid;
	grid-template-columns: repeat(7, 1fr);
	text-align: center;
}

.week {
	border-radius: 18px;
	margin: 2px 0;
}

.week.sel {
	background: var(--bg-active);
}

.wd {
	font-size: 12px;
	color: var(--text-3);
	padding: 4px 0 6px;
}

.day {
	position: relative;
	height: 34px;
	font-size: 13px;
	border-radius: 17px;
}

.day:hover {
	background: var(--bg-hover);
}

.day.other {
	color: var(--text-4);
}

.day.today {
	color: var(--accent);
	font-weight: 700;
}

.day.inrange:not(.today) {
	color: var(--text);
}

.dot {
	position: absolute;
	left: 50%;
	bottom: 3px;
	width: 4px;
	height: 4px;
	margin-left: -2px;
	border-radius: 50%;
	background: var(--accent);
}
</style>
