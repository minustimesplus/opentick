<script setup lang="ts">
import {computed} from 'vue'
import type {Task} from '@/api/types'
import type {CalEvent} from '@/stores/calendars'
import {addDays, dateKey, fmtWeekdayNarrow, getSchedule, isSameDay, startOfDay, startOfWeek} from '@/lib/dates'
import {intlLocale} from '@/lib/i18n'

/** The year view: twelve small months, with a dot on days that have something. Clicking a day opens it. */
const props = defineProps<{year: number, tasks: Task[], events: CalEvent[]}>()
const emit = defineEmits<{day: [Date]}>()

const months = computed(() => Array.from({length: 12}, (_, m) => {
	const first = new Date(props.year, m, 1)
	const start = startOfWeek(first)
	return {
		first,
		name: first.toLocaleDateString(intlLocale.value, {month: 'long'}),
		weeks: Array.from({length: 6}, (_, w) => Array.from({length: 7}, (_, d) => addDays(start, w * 7 + d))),
	}
}))

const busy = computed(() => {
	const set = new Set<string>()
	for (const task of props.tasks) {
		const s = getSchedule(task)
		if (s) set.add(dateKey(s.date))
	}
	for (const e of props.events) {
		// Multi-day events mark each of their days, up to two months.
		let d = startOfDay(e.start)
		for (let i = 0; i < 62 && (d < e.end || i === 0); i++, d = addDays(d, 1)) set.add(dateKey(d))
	}
	return set
})

const today = startOfDay(new Date())
</script>

<template>
	<div class="year">
		<div v-for="m in months" :key="m.first.getMonth()" class="month">
			<div class="m-name" :class="{current: m.first.getMonth() === today.getMonth() && year === today.getFullYear()}">{{ m.name }}</div>
			<div class="grid">
				<span v-for="d in m.weeks[0]" :key="'h' + d.getDay()" class="wd">{{ fmtWeekdayNarrow(d) }}</span>
			</div>
			<div v-for="week in m.weeks" :key="dateKey(week[0])" class="grid">
				<button
					v-for="d in week"
					:key="dateKey(d)"
					class="day"
					:class="{other: d.getMonth() !== m.first.getMonth(), today: isSameDay(d, today)}"
					:tabindex="d.getMonth() !== m.first.getMonth() ? -1 : undefined"
					@click="emit('day', d)"
				>
					{{ d.getDate() }}
					<span v-if="d.getMonth() === m.first.getMonth() && busy.has(dateKey(d))" class="dot" />
				</button>
			</div>
		</div>
	</div>
</template>

<style scoped>
.year {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
	gap: 20px 28px;
	height: 100%;
	padding: 12px 20px 24px;
	overflow-y: auto;
	user-select: none;
}

.m-name {
	margin: 0 0 6px 6px;
	font-weight: 600;
	text-transform: capitalize;
}

.m-name.current {
	color: var(--accent);
}

.grid {
	display: grid;
	grid-template-columns: repeat(7, 1fr);
	text-align: center;
}

.wd {
	padding: 2px 0 4px;
	font-size: 11px;
	color: var(--text-3);
}

.day {
	position: relative;
	height: 28px;
	border-radius: 14px;
	font-size: 12px;
}

.day:hover {
	background: var(--bg-hover);
}

.day.other {
	visibility: hidden;
}

.day.today {
	background: var(--accent);
	color: #fff;
	font-weight: 700;
}

.dot {
	position: absolute;
	left: 50%;
	bottom: 2px;
	width: 4px;
	height: 4px;
	margin-left: -2px;
	border-radius: 50%;
	background: var(--accent);
}

.day.today .dot {
	background: #fff;
}
</style>
