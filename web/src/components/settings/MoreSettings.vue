<script setup lang="ts">
import {computed, reactive} from 'vue'
import {CircleQuestionMark, ChevronDown} from 'lucide-vue-next'
import SettingCheckbox from './SettingCheckbox.vue'
import SettingSelect from './SettingSelect.vue'
import SettingToggle from './SettingToggle.vue'
import {useData} from '@/stores/data'
import {t, type MessageKey} from '@/lib/i18n'

const data = useData()

// Not wired up yet: none of these change how tasks are added.
const state = reactive({
	language: 'en',
	dateRecognition: true,
	removeDateText: true,
	tagRecognition: true,
	removeTagText: true,
	urlParsing: true,
	defaultDate: 'tomorrow',
	dateMode: 'date',
	duration: 120,
	reminderTimed: 'none',
	reminderAllDay: 'none',
	priority: 'medium',
	tag: 'none',
	list: 0,
	addTo: 'top',
	overdue: 'top',
	miniCalendar: false,
})

const opts = (keys: [string, MessageKey][]) => keys.map(([value, key]) => ({value, label: t(key)}))

const languages = [
	{value: 'de', label: 'Deutsch'},
	{value: 'en', label: 'English'},
	{value: 'es', label: 'Español'},
	{value: 'fr', label: 'Français'},
	{value: 'it', label: 'Italiano'},
	{value: 'nl', label: 'Nederlands'},
	{value: 'pl', label: 'Polski'},
	{value: 'pt', label: 'Português (Brasil)'},
	{value: 'ru', label: 'Русский'},
	{value: 'zh', label: '简体中文'},
	{value: 'ja', label: '日本語'},
	{value: 'ko', label: '한국어'},
]

const dates = computed(() => opts([
	['none', 'none'], ['today', 'today'], ['tomorrow', 'tomorrow'], ['dayAfter', 'dayAfterTomorrow'], ['nextWeek', 'nextWeek'],
]))
const dateModes = computed(() => opts([['date', 'date'], ['duration', 'duration']]))
const durations = computed(() => [10, 15, 30, 45, 60, 90, 120, 150, 180, 1440].map(min => ({value: min, label: durationLabel(min)})))
const timedReminders = computed(() => opts([
	['none', 'none'], ['onTime', 'remOnTime'], ['5m', 'remBefore5m'], ['30m', 'remBefore30m'], ['1h', 'remBefore1h'], ['1d', 'remBefore1d'], ['end', 'reminderAtEnd'],
]))
const allDayReminders = computed(() => opts([
	['none', 'none'], ['0d', 'remDay9'], ['1d', 'remDayBefore9'], ['2d', 'reminder2dAt9'], ['3d', 'reminder3dAt9'], ['1w', 'reminder1wAt9'],
]))
const priorities = computed(() => opts([['none', 'none'], ['low', 'low'], ['medium', 'medium'], ['high', 'high']]))
const tags = computed(() => opts([['none', 'none']]))
const lists = computed(() => [
	{value: 0, label: t('inbox')},
	...data.realProjects.map(p => ({value: p.id, label: p.title})),
])
const positions = computed(() => opts([['top', 'topOfList'], ['bottom', 'bottomOfList']]))

function durationLabel(min: number) {
	if (min === 1440) return t('durationDay')
	const h = Math.floor(min / 60)
	const m = min % 60
	if (!h) return t('durationMins', {n: m})
	const hours = h === 1 ? t('durationHour') : t('durationHours', {n: h})
	return m ? `${hours} ${t('durationMins', {n: m})}` : hours
}
</script>

<template>
	<div class="s-card">
		<div class="s-row">
			<span>{{ t('language') }}</span>
			<SettingSelect v-model="state.language" :options="languages" />
		</div>
	</div>

	<h2 class="s-title">{{ t('smartRecognition') }}</h2>
	<div class="s-card">
		<div class="s-row">
			<span>{{ t('dateRecognition') }}</span>
			<SettingToggle v-model="state.dateRecognition" />
		</div>
		<div class="s-desc">{{ t('dateRecognitionDesc') }}</div>
		<label class="s-check">
			<SettingCheckbox v-model="state.removeDateText" />
			{{ t('removeDateText') }}
		</label>
		<div class="s-row">
			<span>{{ t('tagRecognition') }}</span>
			<SettingToggle v-model="state.tagRecognition" />
		</div>
		<div class="s-desc">{{ t('tagRecognitionDesc') }}</div>
		<label class="s-check">
			<SettingCheckbox v-model="state.removeTagText" />
			{{ t('removeTagText') }}
		</label>
		<div class="s-row">
			<span>{{ t('urlParsing') }}</span>
			<SettingToggle v-model="state.urlParsing" />
		</div>
		<div class="s-desc">{{ t('urlParsingDesc') }}</div>
	</div>

	<h2 class="s-title">{{ t('taskDefault') }} <CircleQuestionMark :size="13" /></h2>
	<div class="s-card">
		<div class="s-row">
			<span>{{ t('defaultDate') }}</span>
			<SettingSelect v-model="state.defaultDate" :options="dates" />
		</div>
		<div class="s-row">
			<span>{{ t('defaultDateMode') }}</span>
			<SettingSelect v-model="state.dateMode" :options="dateModes" />
		</div>
		<div class="s-row">
			<span>{{ t('defaultDuration') }}</span>
			<SettingSelect v-model="state.duration" :options="durations" />
		</div>
		<div class="s-row">
			<span>{{ t('defaultReminderTimed') }}</span>
			<SettingSelect v-model="state.reminderTimed" :options="timedReminders" />
		</div>
		<div class="s-row">
			<span>{{ t('defaultReminderAllDay') }}</span>
			<SettingSelect v-model="state.reminderAllDay" :options="allDayReminders" />
		</div>
	</div>
	<div class="s-card">
		<div class="s-row">
			<span>{{ t('defaultPriority') }}</span>
			<SettingSelect v-model="state.priority" :options="priorities" />
		</div>
		<div class="s-row">
			<span>{{ t('defaultTag') }}</span>
			<SettingSelect v-model="state.tag" :options="tags" />
		</div>
		<div class="s-row">
			<span>{{ t('defaultListSetting') }}</span>
			<SettingSelect v-model="state.list" :options="lists" />
		</div>
	</div>
	<div class="s-card">
		<div class="s-row">
			<span>{{ t('defaultAddTo') }}</span>
			<SettingSelect v-model="state.addTo" :options="positions" />
		</div>
		<div class="s-row">
			<span>{{ t('overdueShowsAt') }}</span>
			<SettingSelect v-model="state.overdue" :options="positions" />
		</div>
	</div>
	<button class="s-link reset">{{ t('resetDefault') }}</button>

	<h2 class="s-title">{{ t('miniCalendar') }} <CircleQuestionMark :size="13" /></h2>
	<div class="s-card">
		<div class="s-row">
			<span>{{ t('showMiniCalendar') }}</span>
			<SettingToggle v-model="state.miniCalendar" />
		</div>
	</div>

	<h2 class="s-title template-title">{{ t('taskTemplate') }} <ChevronDown :size="13" /></h2>
	<div class="templates">
		<div class="template">
			<div class="template-name">{{ t('tplBeforeWork') }}</div>
			<div v-for="key in (['tplReflect', 'tplEmails', 'tplToday', 'tplTop3'] as const)" :key="key" class="template-item">
				<span class="box" /><span class="item-text">{{ t(key) }}</span>
			</div>
		</div>
		<div class="template">
			<div class="template-name">{{ t('tplDailyRecord') }}</div>
			<p class="template-note">{{ t('tplDailyRecordNote') }}</p>
		</div>
		<div class="template">
			<div class="template-name">{{ t('tplPacking') }}</div>
			<div v-for="key in (['tplPassport', 'tplCharger', 'tplUmbrella'] as const)" :key="key" class="template-item">
				<span class="box" /><span class="item-text">{{ t(key) }}</span>
			</div>
		</div>
	</div>
</template>

<style scoped>
.reset {
	display: block;
	margin: 4px 0 0 15px;
}

.template-title {
	font-size: 15px;
}

.templates {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 10px;
}

.template {
	height: 92px;
	overflow: hidden;
	padding: 9px 10px;
	border-radius: 8px;
	background: var(--settings-card);
	font-size: 11px;
}

.template-name {
	margin-bottom: 5px;
	font-weight: 700;
}

.template-item {
	display: flex;
	align-items: center;
	gap: 5px;
	margin-bottom: 3px;
	overflow: hidden;
	white-space: nowrap;
	text-overflow: ellipsis;
}

.item-text {
	overflow: hidden;
	text-overflow: ellipsis;
}

.box {
	width: 9px;
	height: 9px;
	flex-shrink: 0;
	border: 1px solid var(--text-4);
	border-radius: 2px;
}

.template-note {
	margin: 0;
	white-space: pre-line;
	color: var(--text-3);
}
</style>
