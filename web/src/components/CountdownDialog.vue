<script setup lang="ts">
import {computed, ref} from 'vue'
import Modal from './Modal.vue'
import type {Countdown, CountdownType} from '@/api/types'
import {useCountdowns} from '@/stores/countdowns'
import {countdownDate, countdownDateValue} from '@/lib/countdowns'
import {dateKey} from '@/lib/dates'
import {SWATCHES} from '@/lib/tasks'
import {t, type MessageKey} from '@/lib/i18n'

const props = defineProps<{countdown?: Countdown | null}>()
const emit = defineEmits<{close: []}>()
const store = useCountdowns()

const TYPES: {value: CountdownType, label: MessageKey, icon: string}[] = [
	{value: 'countdown', label: 'countdownTypeCountdown', icon: '⏳'},
	{value: 'anniversary', label: 'countdownTypeAnniversary', icon: '💝'},
	{value: 'birthday', label: 'countdownTypeBirthday', icon: '🎂'},
]
const ICONS = ['⏳', '🎂', '💝', '💍', '✈️', '🏖️', '🎓', '🎉', '🎄', '🏠', '👶', '💼', '📚', '🏃', '⭐']
const COLORS = SWATCHES.filter(Boolean)

const c = props.countdown
const title = ref(c?.title ?? '')
const type = ref<CountdownType>(c?.type ?? 'countdown')
const date = ref(c ? dateKey(countdownDate(c.date)) : '')
const repeatYearly = ref(c?.repeat_yearly ?? false)
const icon = ref(c?.icon ?? '')
const color = ref((c?.hex_color || '4772FA').replace('#', ''))
const saving = ref(false)

const shownIcon = computed(() => icon.value || TYPES.find(x => x.value === type.value)!.icon)

function pickType(value: CountdownType) {
	type.value = value
	// Anniversaries and birthdays come back every year; one-off countdowns usually don't.
	if (!c) repeatYearly.value = value !== 'countdown'
}

async function save() {
	if (!title.value.trim() || !date.value || saving.value) return
	const [y, m, d] = date.value.split('-').map(Number)
	const body = {
		title: title.value.trim(),
		date: countdownDateValue(new Date(y, m - 1, d)),
		type: type.value,
		repeat_yearly: repeatYearly.value,
		icon: icon.value,
		hex_color: color.value,
	}
	saving.value = true
	try {
		const ok = c ? await store.update(c.id, body) : !!await store.create(body)
		if (ok) emit('close')
	} finally {
		saving.value = false
	}
}

async function remove() {
	if (!c || !confirm(t('deleteCountdownConfirm', {name: c.title}))) return
	if (await store.remove(c.id)) emit('close')
}
</script>

<template>
	<Modal @close="emit('close')">
		<form class="form" @submit.prevent="save">
			<h3>{{ countdown ? t('editCountdown') : t('addCountdown') }}</h3>
			<div class="types" role="radiogroup">
				<button
					v-for="x in TYPES"
					:key="x.value"
					type="button"
					role="radio"
					class="type"
					:class="{on: type === x.value}"
					:aria-checked="type === x.value"
					@click="pickType(x.value)"
				>
					{{ t(x.label) }}
				</button>
			</div>
			<label class="lbl" for="cd-title">{{ t('name') }}</label>
			<div class="title-row">
				<span class="preview" :style="{background: '#' + color + '22'}">{{ shownIcon }}</span>
				<input
					id="cd-title"
					v-model="title"
					class="input"
					required
					maxlength="250"
					:placeholder="t(type === 'birthday' ? 'countdownBirthdayPlaceholder' : 'countdownTitlePlaceholder')"
					v-focus
				>
			</div>
			<label class="lbl" for="cd-date">{{ t(type === 'birthday' ? 'birthDate' : 'date') }}</label>
			<input id="cd-date" v-model="date" class="input" type="date" required>
			<label class="check">
				<input v-model="repeatYearly" type="checkbox">
				{{ t('repeatYearly') }}
			</label>
			<label class="lbl">{{ t('icon') }}</label>
			<div class="icons">
				<button
					v-for="i in ICONS"
					:key="i"
					type="button"
					class="ic"
					:class="{on: icon === i}"
					@click="icon = icon === i ? '' : i"
				>
					{{ i }}
				</button>
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
			<div class="actions">
				<button v-if="countdown" type="button" class="btn danger" @click="remove">{{ t('delete') }}</button>
				<div class="spacer" />
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button type="submit" class="btn primary" :disabled="saving || !title.trim() || !date">{{ t('save') }}</button>
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

.types {
	display: flex;
	padding: 3px;
	border-radius: 8px;
	background: var(--bg-input);
}

.type {
	flex: 1;
	padding: 6px 0;
	border-radius: 6px;
	font-size: 13px;
	color: var(--text-2);
}

.type.on {
	background: var(--bg-elev);
	color: var(--text);
	box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.title-row {
	display: flex;
	align-items: center;
	gap: 8px;
}

.title-row .input {
	flex: 1;
}

.preview {
	display: flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	width: 34px;
	height: 34px;
	border-radius: 8px;
	font-size: 18px;
}

.check {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-top: 4px;
	font-size: 13px;
}

.icons {
	display: flex;
	flex-wrap: wrap;
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
	outline: 1.5px solid var(--accent);
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

.actions {
	display: flex;
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
