<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {Check, ChevronDown, ChevronRight, Flame, Plus} from 'lucide-vue-next'
import HabitDialog from '@/components/HabitDialog.vue'
import HabitDetail from '@/components/HabitDetail.vue'
import type {Habit} from '@/api/types'
import {habitColor, isDue, todayKey, useHabits} from '@/stores/habits'
import {addDays, dateKey, fmtWeekdayNarrow, startOfDay} from '@/lib/dates'
import {t} from '@/lib/i18n'
import {isMobile} from '@/lib/viewport'

const habits = useHabits()
const route = useRoute()
const router = useRouter()

const dialog = ref<{habit: Habit | null} | null>(null)
const showArchived = ref(false)
const busy = ref(new Set<number>())

const selectedId = computed(() => Number(route.params.habitId) || null)
const selected = computed(() => selectedId.value ? habits.habit(selectedId.value) ?? null : null)

onMounted(() => {
	habits.load()
})

/** The last 7 days, oldest first. */
const week = computed(() => {
	const today = startOfDay(new Date())
	return Array.from({length: 7}, (_, i) => addDays(today, i - 6))
})

function open(h: Habit) {
	router.push({name: 'habits', params: {habitId: h.id}})
}

function progress(h: Habit): number {
	return Math.min(1, h.today_value / h.goal)
}

function dueToday(h: Habit) {
	return isDue(h, new Date())
}

async function tap(h: Habit, date?: string) {
	if (busy.value.has(h.id)) return
	busy.value.add(h.id)
	try {
		await habits.tap(h, date)
	} finally {
		busy.value.delete(h.id)
	}
}

function dayState(h: Habit, d: Date): 'done' | 'partial' | 'none' {
	const value = habits.valueOn(h, dateKey(d))
	if (value >= h.goal) return 'done'
	return value > 0 ? 'partial' : 'none'
}

/** Ring around the check button that fills as a counter habit nears its goal. */
function ring(h: Habit) {
	const c = habitColor(h)
	const deg = Math.round(progress(h) * 360)
	return {background: `conic-gradient(${c} ${deg}deg, var(--border-strong) ${deg}deg)`}
}

function onDeleted() {
	router.push({name: 'habits'})
}
</script>

<template>
	<div class="habits-view">
		<section v-if="!isMobile || !selected" class="list-pane">
			<header class="head">
				<h1>{{ t('habits') }}</h1>
				<button class="icon-btn" :title="t('newHabit')" @click="dialog = {habit: null}"><Plus :size="20" /></button>
			</header>

			<div class="week-head">
				<span class="spacer" />
				<span v-for="d in week" :key="d.getTime()" class="wd" :class="{today: dateKey(d) === todayKey()}">{{ fmtWeekdayNarrow(d) }}</span>
				<span class="check-col" />
			</div>

			<div class="list">
				<div v-if="habits.loaded && !habits.habits.length" class="empty-state">
					<div class="empty-icon">🌱</div>
					<div>{{ t('noHabits') }}</div>
					<div class="muted">{{ t('noHabitsHint') }}</div>
					<button class="btn primary" @click="dialog = {habit: null}"><Plus :size="16" /> {{ t('newHabit') }}</button>
				</div>

				<div
					v-for="h in habits.active"
					:key="h.id"
					class="row"
					:class="{active: h.id === selectedId, 'not-due': !dueToday(h)}"
					@click="open(h)"
				>
					<span class="icon" :style="{background: habitColor(h)}">{{ h.icon || '•' }}</span>
					<div class="info">
						<div class="title">{{ h.title }}</div>
						<div class="sub">
							<span v-if="h.current_streak" class="streak"><Flame :size="12" /> {{ t('streakDays', {n: h.current_streak}) }}</span>
							<span v-if="h.goal > 1">{{ h.today_value }}/{{ h.goal }} {{ h.unit }}</span>
							<span v-else-if="!dueToday(h)">{{ t('notDueToday') }}</span>
						</div>
					</div>
					<button
						v-for="d in week"
						:key="d.getTime()"
						class="dot"
						:class="[dayState(h, d), {due: isDue(h, d)}]"
						:style="{'--c': habitColor(h)}"
						:title="dateKey(d)"
						@click.stop="tap(h, dateKey(d))"
					/>
					<button
						class="check"
						:class="{done: h.today_value >= h.goal}"
						:style="h.today_value >= h.goal ? {background: habitColor(h)} : ring(h)"
						:title="h.today_value >= h.goal ? t('undoCheckIn') : t('checkIn')"
						:disabled="busy.has(h.id)"
						@click.stop="tap(h)"
					>
						<span class="check-inner"><Check v-if="h.today_value >= h.goal" :size="16" :stroke-width="3" /><Plus v-else-if="h.goal > 1" :size="14" :stroke-width="3" /></span>
					</button>
				</div>

				<template v-if="habits.archived.length">
					<button class="group-head" @click="showArchived = !showArchived">
						<component :is="showArchived ? ChevronDown : ChevronRight" :size="14" />
						<span>{{ t('archivedHabits') }}</span>
						<span class="count">{{ habits.archived.length }}</span>
					</button>
					<template v-if="showArchived">
						<div
							v-for="h in habits.archived"
							:key="h.id"
							class="row archived"
							:class="{active: h.id === selectedId}"
							@click="open(h)"
						>
							<span class="icon" :style="{background: habitColor(h)}">{{ h.icon || '•' }}</span>
							<div class="info"><div class="title">{{ h.title }}</div></div>
						</div>
					</template>
				</template>
			</div>
		</section>

		<section v-if="selected" class="detail-pane">
			<HabitDetail :key="selected.id" :habit="selected" @edit="dialog = {habit: selected}" @back="router.push({name: 'habits'})" />
		</section>
		<section v-else-if="!isMobile" class="detail-pane placeholder">
			<div class="empty-state muted">{{ t('noHabitSelected') }}</div>
		</section>

		<HabitDialog
			v-if="dialog"
			:habit="dialog.habit"
			@close="dialog = null"
			@saved="h => !dialog?.habit && open(h)"
			@deleted="onDeleted"
		/>
	</div>
</template>

<style scoped>
.habits-view {
	display: flex;
	height: 100%;
}

.list-pane {
	display: flex;
	flex-direction: column;
	flex: 1;
	min-width: 0;
	padding: 0 16px 16px;
}

.detail-pane {
	width: min(460px, 45%);
	flex-shrink: 0;
	border-left: 1px solid var(--border);
	overflow: auto;
}

.detail-pane.placeholder {
	display: flex;
	align-items: center;
	justify-content: center;
}

.head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	height: 72px;
	padding: 0 8px;
	flex-shrink: 0;
}

h1 {
	margin: 0;
	font-size: 22px;
	font-weight: 700;
}

.week-head,
.row {
	display: grid;
	grid-template-columns: 36px 1fr repeat(7, 22px) 44px;
	align-items: center;
	column-gap: 6px;
}

.week-head {
	padding: 0 10px 6px;
	font-size: 11px;
	color: var(--text-3);
	text-align: center;
}

.wd.today {
	color: var(--accent);
	font-weight: 600;
}

.list {
	flex: 1;
	overflow: auto;
}

.row {
	min-height: 60px;
	padding: 8px 10px;
	border-radius: 8px;
	cursor: pointer;
}

.week-head .spacer {
	grid-column: span 2;
}

.row:hover {
	background: var(--bg-hover);
}

.row.active {
	background: var(--bg-active);
}

.row.archived {
	grid-template-columns: 36px 1fr;
	opacity: 0.7;
}

.icon {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 34px;
	height: 34px;
	border-radius: 50%;
	font-size: 17px;
}

.info {
	min-width: 0;
}

.title {
	overflow: hidden;
	font-size: 14px;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.sub {
	display: flex;
	gap: 8px;
	margin-top: 2px;
	font-size: 12px;
	color: var(--text-3);
}

.streak {
	display: inline-flex;
	align-items: center;
	gap: 2px;
	color: #f76707;
}

.not-due .title {
	color: var(--text-2);
}

.dot {
	justify-self: center;
	width: 14px;
	height: 14px;
	border-radius: 50%;
	border: 1.5px dashed var(--border-strong);
}

.dot.due {
	border-style: solid;
}

.dot.partial {
	border-color: var(--c);
	background: color-mix(in srgb, var(--c) 35%, transparent);
}

.dot.done {
	border-color: var(--c);
	background: var(--c);
}

.check {
	justify-self: end;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 32px;
	height: 32px;
	padding: 3px;
	border-radius: 50%;
	color: #fff;
	transition: transform 0.12s;
}

.check:active {
	transform: scale(0.9);
}

.check-inner {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 100%;
	height: 100%;
	border-radius: 50%;
	background: var(--bg);
	color: var(--text-3);
}

.check.done .check-inner {
	background: transparent;
	color: #fff;
}

.group-head {
	display: flex;
	align-items: center;
	gap: 6px;
	width: 100%;
	margin-top: 12px;
	padding: 6px 10px;
	font-size: 13px;
	font-weight: 600;
	color: var(--text-2);
}

.count {
	color: var(--text-3);
	font-weight: 400;
}

.empty-state {
	padding: 48px 16px;
	text-align: center;
}

.empty-icon {
	font-size: 36px;
}

/* Phones: one pane at a time, and fewer day dots so titles keep their room. */
@media (max-width: 767px) {
	.list-pane {
		padding: 0 8px 8px;
	}

	.detail-pane {
		width: 100%;
		border-left: 0;
	}

	.week-head,
	.row {
		grid-template-columns: 36px 1fr repeat(7, 16px) 40px;
		column-gap: 4px;
	}

	.dot {
		width: 12px;
		height: 12px;
	}
}
</style>
