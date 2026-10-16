<script setup lang="ts">
import {useData} from '@/stores/data'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'

const data = useData()
</script>

<template>
	<div class="settings">
		<h1>{{ t('settings') }}</h1>

		<h2>{{ t('general') }}</h2>
		<label class="row">
			<span>{{ t('language') }}</span>
			<select v-model="prefs.language" class="input">
				<option value="auto">Auto</option>
				<option value="en">English</option>
				<option value="fr">Français</option>
			</select>
		</label>
		<label class="row">
			<span>{{ t('theme') }}</span>
			<select v-model="prefs.theme" class="input">
				<option value="dark">{{ t('themeDark') }}</option>
				<option value="light">{{ t('themeLight') }}</option>
				<option value="system">{{ t('themeSystem') }}</option>
			</select>
		</label>
		<label class="row">
			<span>{{ t('weekStart') }}</span>
			<select v-model.number="prefs.weekStart" class="input">
				<option :value="1">{{ t('monday') }}</option>
				<option :value="0">{{ t('sunday') }}</option>
				<option :value="6">{{ t('saturday') }}</option>
			</select>
		</label>
		<label class="row">
			<span>{{ t('timeFormat') }}</span>
			<select v-model="prefs.hour12" class="input">
				<option :value="false">24h</option>
				<option :value="true">12h</option>
			</select>
		</label>
		<label class="row">
			<span>{{ t('defaultList') }}</span>
			<select v-model="prefs.defaultListId" class="input">
				<option :value="null">{{ t('inbox') }}</option>
				<option v-for="p in data.listProjects" :key="p.id" :value="p.id">{{ p.title }}</option>
			</select>
		</label>

		<h2>{{ t('account') }}</h2>
		<div class="row">
			<span>{{ data.user?.username }}</span>
			<button class="btn" @click="data.logout()">{{ t('logout') }}</button>
		</div>
	</div>
</template>

<style scoped>
.settings {
	max-width: 620px;
	margin: 0 auto;
	padding: 32px 24px;
	height: 100%;
	overflow: auto;
}

h1 {
	font-size: 22px;
	margin: 0 0 20px;
}

h2 {
	font-size: 13px;
	font-weight: 600;
	color: var(--text-3);
	margin: 28px 0 8px;
	text-transform: uppercase;
	letter-spacing: 0.04em;
}

.row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 16px;
	min-height: 48px;
	border-bottom: 1px solid var(--border);
}

.row .input {
	width: 220px;
}
</style>
