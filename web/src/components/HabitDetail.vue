<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {ChevronLeft, ChevronRight, Pencil} from 'lucide-vue-next'
import type {Habit, HabitStats, HabitStatsDay} from '@/api/types'
import {habitColor, todayKey, useHabits} from '@/stores/habits'
import {addDays, addMonths, fmtMonthYear, fmtWeekdayNarrow, fmtWeekdayShort, startOfMonth, startOfWeek} from '@/lib/dates'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {isMobile} from '@/lib/viewport'

const props = defineProps<{habit: Habit}>()
const emit = defineEmits<{edit: [], back: []}>()
const habits = useHabits()

const month = ref(startOfMonth(new Date()))
const stats = ref<HabitStats | null>(null)
let request = 0

function monthKey(d: Date) {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
}

async function load() {
	const id = ++request
	const res = await habits.stats(props.habit.id, monthKey(month.value))
	// Paging quickly fires overlapping requests; only the latest may land.
	if (id === request) stats.value = res
}

// Reload when paging months and after every check-in (the habit object is replaced).
watch([month, () => props.habit], load, {immediate: true})

const color = computed(() => habitColor(props.habit))

const weekdayLabels = computed(() => {
	const first = startOfWeek(new Date())
	return Array.from({length: 7}, (_, i) => fmtWeekdayNarrow(addDays(first, i)))
})

/** The month's days padded to whole weeks; null cells are outside the month. */
const cells = computed<(HabitStatsDay | null)[]>(() => {
	const days = stats.value?.month === monthKey(month.value) ? stats.value.days : []
	if (!days.length) return []
	const lead = (month.value.getDay() - prefs.weekStart + 7) % 7
	const out: (HabitStatsDay | null)[] = [...Array(lead).fill(null), ...days]
	while (out.length % 7) out.push(null)
	return out
})

const today = todayKey()

function dayNumber(d: HabitStatsDay) {
	return Number(d.date.slice(8))
}

function cellStyle(d: HabitStatsDay) {
	if (d.completed) return {background: color.value, color: '#fff'}
	if (d.value > 0) return {background: `color-mix(in srgb, ${color.value} 30%, transparent)`}
	return {}
}

async function toggle(d: HabitStatsDay) {
	if (d.date > today) return
	await habits.checkin(props.habit.id, d.date, d.completed ? 0 : props.habit.goal)
}

const rate = computed(() => Math.round((stats.value?.completion_rate ?? 0) * 100))

const frequencyLabel = computed(() => {
	if (props.habit.frequency === 'daily') return t('everyDay')
	const first = startOfWeek(new Date())
	return Array.from({length: 7}, (_, i) => addDays(first, i))
		.filter(d => props.habit.weekdays & (1 << d.getDay()))
		.map(fmtWeekdayShort)
		.join(', ')
})
</script>

<template>
	<div class="detail">
		<header class="head">
			<button v-if="isMobile" class="icon-btn" @click="emit('back')"><ChevronLeft :size="20" /></button>
			<span class="icon" :style="{background: color}">{{ habit.icon || '•' }}</span>
			<div class="titles">
				<h2>{{ habit.title }}</h2>
				<div class="muted small">
					{{ frequencyLabel }}<template v-if="habit.goal > 1"> · {{ t('goalPerDay', {n: habit.goal, unit: habit.unit}) }}</template>
				</div>
			</div>
			<button class="icon-btn" :title="t('edit')" @click="emit('edit')"><Pencil :size="17" /></button>
		</header>

		<div class="cards">
			<div class="card">
				<div class="num" :style="{color}">{{ stats?.current_streak ?? habit.current_streak }}</div>
				<div class="label">{{ t('currentStreak') }}</div>
			</div>
			<div class="card">
				<div class="num">{{ stats?.longest_streak ?? '–' }}</div>
				<div class="label">{{ t('longestStreak') }}</div>
			</div>
			<div class="card">
				<div class="num">{{ stats ? rate + '%' : '–' }}</div>
				<div class="label">{{ t('completionRate') }}</div>
			</div>
			<div class="card">
				<div class="num">{{ stats?.total_checkins ?? '–' }}</div>
				<div class="label">{{ t('totalCheckIns') }}</div>
			</div>
		</div>

		<section class="calendar">
			<div class="cal-head">
				<span class="month">{{ fmtMonthYear(month) }}</span>
				<button class="icon-btn" @click="month = addMonths(month, -1)"><ChevronLeft :size="18" /></button>
				<button class="icon-btn" @click="month = startOfMonth(new Date())">{{ t('today') }}</button>
				<button class="icon-btn" @click="month = addMonths(month, 1)"><ChevronRight :size="18" /></button>
			</div>
			<div class="grid">
				<span v-for="(w, i) in weekdayLabels" :key="'w' + i" class="wd">{{ w }}</span>
				<template v-for="(d, i) in cells" :key="i">
					<button
						v-if="d"
						class="cell"
						:class="{today: d.date === today, future: d.date > today, 'not-due': !d.due}"
						:style="cellStyle(d)"
						:title="d.value ? `${d.value}/${habit.goal} ${habit.unit}` : d.date"
						:disabled="d.date > today"
						@click="toggle(d)"
					>{{ dayNumber(d) }}</button>
					<span v-else />
				</template>
			</div>
			<p class="hint muted small">{{ t('habitCalendarHint') }}</p>
		</section>
	</div>
</template>

<style scoped>
.detail {
	padding: 0 20px 24px;
}

.head {
	display: flex;
	align-items: center;
	gap: 12px;
	height: 72px;
}

.icon {
	display: flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	width: 40px;
	height: 40px;
	border-radius: 50%;
	font-size: 20px;
}

.titles {
	flex: 1;
	min-width: 0;
}

h2 {
	margin: 0;
	overflow: hidden;
	font-size: 18px;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.small {
	font-size: 12px;
}

.cards {
	display: grid;
	grid-template-columns: repeat(2, 1fr);
	gap: 10px;
}

.card {
	padding: 14px;
	border-radius: 10px;
	background: var(--bg-panel);
}

.num {
	font-size: 24px;
	font-weight: 700;
}

.label {
	margin-top: 2px;
	font-size: 12px;
	color: var(--text-3);
}

.calendar {
	margin-top: 20px;
	padding: 14px;
	border-radius: 10px;
	background: var(--bg-panel);
}

.cal-head {
	display: flex;
	align-items: center;
	gap: 2px;
	margin-bottom: 8px;
}

.cal-head .icon-btn {
	width: auto;
	min-width: 32px;
	padding: 0 6px;
	font-size: 13px;
}

.month {
	flex: 1;
	font-weight: 600;
}

.grid {
	display: grid;
	grid-template-columns: repeat(7, 1fr);
	gap: 6px;
	justify-items: center;
}

.wd {
	font-size: 11px;
	color: var(--text-3);
}

.cell {
	width: 34px;
	height: 34px;
	border-radius: 50%;
	font-size: 13px;
	color: var(--text);
}

.cell:hover:not(:disabled) {
	outline: 1px solid var(--border-strong);
}

.cell.not-due {
	color: var(--text-3);
}

.cell.today {
	outline: 2px solid var(--accent);
	outline-offset: 1px;
}

.cell.future {
	opacity: 0.4;
	cursor: default;
}

.hint {
	margin: 12px 0 0;
}
</style>
