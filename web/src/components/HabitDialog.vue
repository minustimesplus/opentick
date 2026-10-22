<script setup lang="ts">
import {computed, ref} from 'vue'
import Modal from './Modal.vue'
import type {Habit, HabitFrequency} from '@/api/types'
import {ALL_WEEKDAYS, DEFAULT_HABIT_COLOR, useHabits} from '@/stores/habits'
import {SWATCHES} from '@/lib/tasks'
import {addDays, fmtWeekdayShort, startOfWeek} from '@/lib/dates'
import {t} from '@/lib/i18n'

const props = defineProps<{habit?: Habit | null}>()
const emit = defineEmits<{close: [], saved: [habit: Habit], deleted: []}>()
const habits = useHabits()

const ICONS = ['💧', '🏃', '📚', '🧘', '🥗', '😴', '💪', '🚭', '🎸', '✍️', '🌱', '☀️']
const COLORS = SWATCHES.filter(Boolean)

const title = ref(props.habit?.title ?? '')
const icon = ref(props.habit?.icon || ICONS[0])
const color = ref((props.habit?.hex_color || DEFAULT_HABIT_COLOR).replace('#', ''))
const frequency = ref<HabitFrequency>(props.habit?.frequency ?? 'daily')
const weekdays = ref(props.habit?.frequency === 'weekdays' ? props.habit.weekdays : ALL_WEEKDAYS)
const goal = ref(props.habit?.goal ?? 1)
const unit = ref(props.habit?.unit ?? '')
const saving = ref(false)

/** The days of the week in the user's order, as [weekday number, label]. */
const days = computed(() => {
	const first = startOfWeek(new Date())
	return Array.from({length: 7}, (_, i) => {
		const d = addDays(first, i)
		return [d.getDay(), fmtWeekdayShort(d)] as const
	})
})

function toggleDay(day: number) {
	weekdays.value ^= 1 << day
}

const valid = computed(() => title.value.trim() && (frequency.value === 'daily' || weekdays.value !== 0))

async function save() {
	if (!valid.value || saving.value) return
	saving.value = true
	try {
		const body = {
			title: title.value.trim(),
			icon: icon.value.trim(),
			hex_color: color.value,
			frequency: frequency.value,
			weekdays: frequency.value === 'daily' ? ALL_WEEKDAYS : weekdays.value,
			goal: Math.max(1, Math.round(goal.value) || 1),
			unit: unit.value.trim(),
		}
		const saved = props.habit ? await habits.update(props.habit.id, body) : await habits.create(body)
		if (saved) {
			emit('saved', saved)
			emit('close')
		}
	} finally {
		saving.value = false
	}
}

async function toggleArchived() {
	const h = props.habit
	if (!h) return
	if (await habits.update(h.id, {archived: !h.archived})) emit('close')
}

async function remove() {
	const h = props.habit
	if (!h || !confirm(t('deleteHabitConfirm', {name: h.title}))) return
	await habits.remove(h.id)
	emit('deleted')
	emit('close')
}
</script>

<template>
	<Modal @close="emit('close')">
		<form class="form" @submit.prevent="save">
			<h3>{{ habit ? t('editHabit') : t('newHabit') }}</h3>

			<div class="name-row">
				<span class="icon-preview" :style="{background: '#' + color}">{{ icon }}</span>
				<input v-model="title" class="input" :placeholder="t('habitNamePlaceholder')" maxlength="250" required v-focus>
			</div>

			<label class="lbl">{{ t('icon') }}</label>
			<div class="icons">
				<button
					v-for="i in ICONS"
					:key="i"
					type="button"
					class="ic"
					:class="{on: icon === i}"
					@click="icon = i"
				>{{ i }}</button>
				<input v-model="icon" class="input ic-input" maxlength="8" :title="t('icon')">
			</div>

			<label class="lbl">{{ t('color') }}</label>
			<div class="swatches">
				<button
					v-for="s in COLORS"
					:key="s"
					type="button"
					class="sw"
					:class="{on: color.toLowerCase() === s.toLowerCase()}"
					:style="{background: '#' + s}"
					@click="color = s"
				/>
			</div>

			<label class="lbl" for="habit-frequency">{{ t('frequency') }}</label>
			<select id="habit-frequency" v-model="frequency" class="input">
				<option value="daily">{{ t('everyDay') }}</option>
				<option value="weekdays">{{ t('specificWeekdays') }}</option>
			</select>
			<div v-if="frequency === 'weekdays'" class="days">
				<button
					v-for="[d, label] in days"
					:key="d"
					type="button"
					class="day"
					:class="{on: weekdays & (1 << d)}"
					@click="toggleDay(d)"
				>{{ label }}</button>
			</div>

			<label class="lbl">{{ t('dailyGoal') }}</label>
			<div class="goal-row">
				<input v-model.number="goal" class="input goal" type="number" min="1" max="100000" step="1">
				<input v-model="unit" class="input" :placeholder="t('habitUnitPlaceholder')" maxlength="50">
			</div>

			<div class="actions">
				<template v-if="habit">
					<button type="button" class="btn danger" @click="remove">{{ t('delete') }}</button>
					<button type="button" class="btn" @click="toggleArchived">{{ habit.archived ? t('unarchive') : t('archive') }}</button>
				</template>
				<div class="spacer" />
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button type="submit" class="btn primary" :disabled="saving || !valid">{{ t('save') }}</button>
			</div>
		</form>
	</Modal>
</template>

<style scoped>
.form {
	display: flex;
	flex-direction: column;
	gap: 8px;
}

h3 {
	margin: 0 0 6px;
	font-size: 17px;
}

.lbl {
	font-size: 12px;
	color: var(--text-3);
	margin-top: 4px;
}

.name-row,
.goal-row {
	display: flex;
	align-items: center;
	gap: 8px;
}

.icon-preview {
	display: flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	width: 34px;
	height: 34px;
	border-radius: 50%;
	font-size: 18px;
}

.icons {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 4px;
}

.ic {
	width: 30px;
	height: 30px;
	border-radius: 6px;
	font-size: 17px;
}

.ic:hover {
	background: var(--bg-hover);
}

.ic.on {
	background: var(--accent-soft);
	outline: 1px solid var(--accent);
}

.ic-input {
	width: 56px;
	text-align: center;
}

.swatches {
	display: flex;
	flex-wrap: wrap;
	gap: 8px;
}

.sw {
	width: 22px;
	height: 22px;
	border-radius: 50%;
	outline-offset: 2px;
}

.sw.on {
	outline: 2px solid var(--text);
}

.days {
	display: flex;
	gap: 4px;
}

.day {
	flex: 1;
	height: 30px;
	border-radius: 6px;
	border: 1px solid var(--border-strong);
	font-size: 12px;
	color: var(--text-2);
}

.day.on {
	background: var(--accent);
	border-color: var(--accent);
	color: #fff;
}

.goal {
	width: 96px;
	flex-shrink: 0;
}

.actions {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 8px;
	margin-top: 10px;
}

.spacer {
	flex: 1;
}

.btn.danger {
	color: var(--danger);
}
</style>
