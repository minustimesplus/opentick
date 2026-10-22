<script setup lang="ts">
import {computed, ref} from 'vue'
import SettingSelect from './SettingSelect.vue'
import SettingToggle from './SettingToggle.vue'
import {prefs, type AltCalendar, type DateFormat} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {fmtNumericDate, fmtTime} from '@/lib/dates'

const thirteen = new Date(2000, 0, 1, 13, 0)

// Not wired up yet: tasks have no per-task time zone.
const timeZone = ref(false)

const timeFormats = computed(() => [
	{value: false, label: `${t('hour24')} (${fmtTime(thirteen, false)})`},
	{value: true, label: `${t('hour12')} (${fmtTime(thirteen, true)})`},
])

const dateFormats = computed(() => {
	const lastDay = new Date(new Date().getFullYear(), 11, 31)
	return (['mdy', 'dmy', 'ymd'] as DateFormat[]).map(value => ({value, label: fmtNumericDate(lastDay, value)}))
})

const weekStarts = computed(() => [
	{value: 1 as const, label: t('monday')},
	{value: 0 as const, label: t('sunday')},
	{value: 6 as const, label: t('saturday')},
])

const calendars = computed<{value: AltCalendar, label: string}[]>(() => [
	{value: 'none', label: t('none')},
	{value: 'chinese', label: t('calChinese')},
	{value: 'hebrew', label: t('calHebrew')},
	{value: 'islamic', label: t('calIslamic')},
	{value: 'persian', label: t('calPersian')},
	{value: 'indian', label: t('calIndian')},
])
</script>

<template>
	<h2 class="section-title">{{ t('dateTime') }}</h2>

	<div class="card">
		<div class="row">
			<span>{{ t('timeFormat') }}</span>
			<SettingSelect v-model="prefs.hour12" :options="timeFormats" />
		</div>
		<div class="row">
			<span>{{ t('dateFormat') }}</span>
			<SettingSelect v-model="prefs.dateFormat" :options="dateFormats" />
		</div>
		<div class="row">
			<span>{{ t('weekStart') }}</span>
			<SettingSelect v-model="prefs.weekStart" :options="weekStarts" />
		</div>
	</div>

	<div class="card">
		<div class="row">
			<span>{{ t('additionalCalendar') }}</span>
			<SettingSelect v-model="prefs.altCalendar" :options="calendars" />
		</div>
		<div class="row">
			<span>{{ t('showWeekNumbers') }}</span>
			<SettingToggle v-model="prefs.showWeekNumbers" />
		</div>
		<div class="row">
			<span>{{ t('timeZone') }}</span>
			<SettingToggle v-model="timeZone" />
		</div>
		<div class="s-desc">{{ t('timeZoneDesc') }}</div>
	</div>
</template>

<style scoped>
.section-title {
	margin: 0 0 13px 15px;
	font-size: 14px;
	font-weight: 600;
}

.card {
	margin-bottom: 14px;
	padding: 2px 15px;
	border-radius: 8px;
	background: var(--settings-card);
}

.row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 16px;
	min-height: 36px;
	font-size: 13px;
}
</style>
