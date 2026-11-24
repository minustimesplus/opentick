<script setup lang="ts">
import {computed, ref} from 'vue'
import Modal from '@/components/Modal.vue'
import type {FocusSessionType} from '@/api/types'
import {fmtMinutes, useFocus} from '@/stores/focus'
import {useData} from '@/stores/data'
import {dateKey} from '@/lib/dates'
import {t} from '@/lib/i18n'

/** Focus Record > +: records a session that was not timed in the app. */
const emit = defineEmits<{close: []}>()
const focus = useFocus()
const data = useData()

function hhmm(d: Date) {
	return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

const endDefault = new Date()
const startDefault = new Date(endDefault.getTime() - 25 * 60_000)

const type = ref<FocusSessionType>('pomo')
const taskId = ref(0)
const day = ref(dateKey(startDefault))
const start = ref(hhmm(startDefault))
const end = ref(hhmm(endDefault))
const note = ref('')
const saving = ref(false)

const tasks = computed(() => data.openTasks.slice().sort((a, b) => a.title.localeCompare(b.title)))

/** Start and end as dates; an end before the start runs past midnight. */
const span = computed(() => {
	const s = new Date(`${day.value}T${start.value}:00`)
	const e = new Date(`${day.value}T${end.value}:00`)
	if (Number.isNaN(s.getTime()) || Number.isNaN(e.getTime())) return null
	if (e <= s) e.setDate(e.getDate() + 1)
	return {s, e}
})

const valid = computed(() => !!span.value && span.value.e.getTime() <= Date.now() + 60_000)
const minutes = computed(() => span.value ? Math.round((span.value.e.getTime() - span.value.s.getTime()) / 60_000) : 0)

async function save() {
	if (!span.value || !valid.value || saving.value) return
	saving.value = true
	const ok = await focus.addRecord({
		task_id: taskId.value,
		type: type.value,
		start_time: span.value.s.toISOString(),
		end_time: span.value.e.toISOString(),
		duration: Math.round((span.value.e.getTime() - span.value.s.getTime()) / 1000),
		...(note.value.trim() ? {note: note.value.trim()} : {}),
	})
	saving.value = false
	if (ok) emit('close')
}
</script>

<template>
	<Modal :width="400" @close="emit('close')">
		<form class="record-form" @submit.prevent="save">
			<h2>{{ t('addFocusRecord') }}</h2>

			<div class="modes" role="radiogroup">
				<button
					v-for="m in (['pomo', 'stopwatch'] as const)"
					:key="m"
					type="button"
					role="radio"
					class="mode"
					:class="{active: type === m}"
					:aria-checked="type === m"
					@click="type = m"
				>{{ t(m) }}</button>
			</div>

			<label class="field">
				<span>{{ t('scTask') }}</span>
				<select v-model.number="taskId" class="input">
					<option :value="0">{{ t('noFocusTask') }}</option>
					<option v-for="tk in tasks" :key="tk.id" :value="tk.id">{{ tk.title }}</option>
				</select>
			</label>

			<label class="field">
				<span>{{ t('date') }}</span>
				<input v-model="day" class="input" type="date" required>
			</label>

			<div class="row">
				<label class="field">
					<span>{{ t('start') }}</span>
					<input v-model="start" class="input" type="time" required>
				</label>
				<label class="field">
					<span>{{ t('end') }}</span>
					<input v-model="end" class="input" type="time" required>
				</label>
			</div>

			<label class="field">
				<span>{{ t('focusNote') }}</span>
				<input v-model="note" class="input" :placeholder="t('focusNoteHint')" maxlength="2000">
			</label>

			<div class="foot">
				<span v-if="!valid" class="error">{{ t('focusRecordInvalid') }}</span>
				<span v-else class="muted">{{ fmtMinutes(minutes) }}</span>
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button type="submit" class="btn primary" :disabled="!valid || saving">{{ t('save') }}</button>
			</div>
		</form>
	</Modal>
</template>

<style scoped>
.record-form {
	display: flex;
	flex-direction: column;
	gap: 12px;
}

h2 {
	margin: 0;
	font-size: 17px;
}

.modes {
	display: flex;
	align-self: flex-start;
	padding: 3px;
	border-radius: 8px;
	background: var(--bg-panel);
}

.mode {
	height: 26px;
	padding: 0 14px;
	border-radius: 6px;
	font-size: 13px;
	color: var(--text-2);
}

.mode.active {
	background: var(--bg-elev);
	color: var(--text);
	font-weight: 600;
	box-shadow: var(--shadow);
}

.field {
	display: flex;
	flex: 1;
	flex-direction: column;
	gap: 4px;
	font-size: 12px;
	color: var(--text-3);
}

.field .input {
	color: var(--text);
	font-size: 14px;
}

.row {
	display: flex;
	gap: 10px;
}

.foot {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-top: 4px;
}

.foot > span {
	flex: 1;
	font-size: 12px;
}

.error {
	color: var(--danger);
}

.muted {
	color: var(--text-3);
}

.btn:disabled {
	opacity: 0.5;
	cursor: default;
}
</style>
