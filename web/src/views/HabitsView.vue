<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {ArrowDown, ArrowDownToLine, ArrowUp, ArrowUpToLine, Check, ChevronDown, ChevronRight, Flame, Pencil, Plus} from 'lucide-vue-next'
import HabitDialog from '@/components/HabitDialog.vue'
import HabitDetail from '@/components/HabitDetail.vue'
import Popover from '@/components/Popover.vue'
import type {Habit} from '@/api/types'
import {habitColor, isDue, todayKey, useHabits} from '@/stores/habits'
import {addDays, dateKey, fmtWeekdayNarrow, startOfDay} from '@/lib/dates'
import {t} from '@/lib/i18n'
import {isMobile} from '@/lib/viewport'
import {prefs} from '@/lib/prefs'
import {clamp, startResize} from '@/lib/resize'

const DETAIL_RANGE = [320, 760] as const

const habits = useHabits()
const route = useRoute()
const router = useRouter()

const dialog = ref<{habit: Habit | null} | null>(null)
const showArchived = ref(false)
const busy = ref(new Set<number>())

const selectedId = computed(() => Number(route.params.habitId) || null)
const selected = computed(() => selectedId.value ? habits.habit(selectedId.value) ?? null : null)
const detailStyle = computed(() => isMobile.value ? undefined : {width: clamp(prefs.habitDetailWidth, DETAIL_RANGE) + 'px'})

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

// ---- reordering ----
// Rows drag onto other rows: the top half drops before a row, the bottom half after it.
const dragId = ref<number | null>(null)
const dropAt = ref<{id: number, zone: 'before' | 'after'} | null>(null)

function onDragStart(e: DragEvent, h: Habit) {
	dragId.value = h.id
	e.dataTransfer?.setData('application/x-habit-id', String(h.id))
	if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragOver(e: DragEvent, h: Habit) {
	if (dragId.value === null || dragId.value === h.id) return
	e.preventDefault()
	if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
	const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
	dropAt.value = {id: h.id, zone: e.clientY - rect.top < rect.height / 2 ? 'before' : 'after'}
}

function onDragLeave(h: Habit) {
	if (dropAt.value?.id === h.id) dropAt.value = null
}

function onDragEnd() {
	dragId.value = null
	dropAt.value = null
}

function onDrop(h: Habit) {
	const dragged = dragId.value !== null ? habits.habit(dragId.value) : undefined
	const drop = dropAt.value
	onDragEnd()
	if (!dragged || !drop || drop.id !== h.id) return
	const others = habits.active.filter(x => x.id !== dragged.id)
	const i = others.findIndex(x => x.id === h.id)
	habits.move(dragged, drop.zone === 'before' ? i : i + 1)
}

function dropClass(h: Habit) {
	return dropAt.value?.id === h.id ? `drop-${dropAt.value.zone}` : ''
}

// Right-click (or long-press) menu with explicit moves, for touch and keyboard users.
const menu = ref<{habit: Habit, x: number, y: number} | null>(null)
const menuIndex = computed(() => menu.value ? habits.active.findIndex(x => x.id === menu.value!.habit.id) : -1)

function openMenu(e: MouseEvent, h: Habit) {
	menu.value = {habit: h, x: e.clientX, y: e.clientY}
}

function moveTo(index: number) {
	const h = menu.value?.habit
	menu.value = null
	if (h) habits.move(h, index)
}

/** Alt+Up/Down moves the focused habit by one. */
function onRowKey(e: KeyboardEvent, h: Habit) {
	if (!e.altKey || (e.key !== 'ArrowUp' && e.key !== 'ArrowDown')) return
	e.preventDefault()
	const i = habits.active.findIndex(x => x.id === h.id)
	habits.move(h, e.key === 'ArrowUp' ? i - 1 : i + 1)
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
					:class="[{active: h.id === selectedId, 'not-due': !dueToday(h), dragging: dragId === h.id}, dropClass(h)]"
					tabindex="0"
					:draggable="!isMobile"
					:title="isMobile ? undefined : t('habitReorderHint')"
					@click="open(h)"
					@keydown.enter.self="open(h)"
					@keydown="onRowKey($event, h)"
					@contextmenu.prevent="openMenu($event, h)"
					@dragstart="onDragStart($event, h)"
					@dragover="onDragOver($event, h)"
					@dragleave="onDragLeave(h)"
					@drop.prevent="onDrop(h)"
					@dragend="onDragEnd"
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

		<div
			v-if="!isMobile"
			class="resizer"
			@pointerdown="startResize($event, 'habitDetailWidth', DETAIL_RANGE, -1)"
		/>
		<section v-if="selected" class="detail-pane" :style="detailStyle">
			<HabitDetail :key="selected.id" :habit="selected" @edit="dialog = {habit: selected}" @back="router.push({name: 'habits'})" />
		</section>
		<section v-else-if="!isMobile" class="detail-pane placeholder" :style="detailStyle">
			<div class="empty-state muted">{{ t('noHabitSelected') }}</div>
		</section>

		<Popover v-if="menu" :anchor="{x: menu.x, y: menu.y}" @close="menu = null">
			<div class="menu">
				<button class="menu-item" :disabled="menuIndex <= 0" @click="moveTo(0)"><ArrowUpToLine :size="15" /> {{ t('moveToTop') }}</button>
				<button class="menu-item" :disabled="menuIndex <= 0" @click="moveTo(menuIndex - 1)"><ArrowUp :size="15" /> {{ t('moveUp') }}</button>
				<button class="menu-item" :disabled="menuIndex >= habits.active.length - 1" @click="moveTo(menuIndex + 1)"><ArrowDown :size="15" /> {{ t('moveDown') }}</button>
				<button class="menu-item" :disabled="menuIndex >= habits.active.length - 1" @click="moveTo(habits.active.length)"><ArrowDownToLine :size="15" /> {{ t('moveToBottom') }}</button>
				<button class="menu-item" @click="dialog = {habit: menu.habit}; menu = null"><Pencil :size="15" /> {{ t('edit') }}</button>
			</div>
		</Popover>

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
	flex-shrink: 0;
	max-width: 60%;
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

.row:focus-visible {
	outline: 2px solid var(--accent);
	outline-offset: -2px;
}

.row.dragging {
	opacity: 0.4;
}

.row.drop-before {
	box-shadow: inset 0 2px 0 var(--accent);
}

.row.drop-after {
	box-shadow: inset 0 -2px 0 var(--accent);
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
