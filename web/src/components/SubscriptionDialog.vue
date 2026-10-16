<script setup lang="ts">
import {ref} from 'vue'
import Modal from './Modal.vue'
import type {CalendarSubscription} from '@/api/types'
import {DEFAULT_CALENDAR_COLOR, useCalendars} from '@/stores/calendars'
import {prefs} from '@/lib/prefs'
import {SWATCHES} from '@/lib/tasks'
import {t} from '@/lib/i18n'

const props = defineProps<{subscription?: CalendarSubscription | null}>()
const emit = defineEmits<{close: []}>()
const calendars = useCalendars()

const url = ref(props.subscription?.url ?? '')
const name = ref(props.subscription?.title ?? '')
const color = ref((props.subscription?.hex_color || DEFAULT_CALENDAR_COLOR).replace('#', ''))
const saving = ref(false)
const COLORS = SWATCHES.filter(Boolean)

async function save() {
	if (!url.value.trim() || saving.value) return
	saving.value = true
	try {
		const body = {title: name.value.trim(), url: url.value.trim(), hex_color: color.value}
		if (props.subscription) {
			if (!body.title) body.title = props.subscription.title
			if (await calendars.update(props.subscription.id, body)) emit('close')
		} else {
			const sub = await calendars.create(body)
			if (sub) {
				// Make sure a new calendar shows even if it was hidden under a reused id.
				prefs.calendarHiddenSubs = prefs.calendarHiddenSubs.filter(id => id !== sub.id)
				emit('close')
			}
		}
	} finally {
		saving.value = false
	}
}

async function unsubscribe() {
	const sub = props.subscription
	if (!sub || !confirm(t('unsubscribeConfirm', {name: sub.title}))) return
	await calendars.remove(sub.id)
	prefs.calendarHiddenSubs = prefs.calendarHiddenSubs.filter(id => id !== sub.id)
	emit('close')
}
</script>

<template>
	<Modal @close="emit('close')">
		<form class="form" @submit.prevent="save">
			<h3>{{ subscription ? t('editCalendar') : t('subscribeCalendar') }}</h3>
			<label class="lbl" for="sub-url">{{ t('calendarUrl') }}</label>
			<input
				id="sub-url"
				v-model="url"
				class="input"
				type="url"
				required
				placeholder="https://… / webcal://…"
				autocomplete="off"
				spellcheck="false"
				v-focus
			>
			<p class="hint">{{ t('calendarUrlHint') }}</p>
			<label class="lbl" for="sub-name">{{ t('name') }}</label>
			<input id="sub-name" v-model="name" class="input" :placeholder="t('calendarNameHint')" maxlength="250">
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
				<button v-if="subscription" type="button" class="btn danger" @click="unsubscribe">{{ t('unsubscribe') }}</button>
				<div class="spacer" />
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button type="submit" class="btn primary" :disabled="saving || !url.trim()">{{ subscription ? t('save') : t('subscribe') }}</button>
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

.hint {
	margin: 0;
	font-size: 12px;
	line-height: 1.45;
	color: var(--text-3);
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
