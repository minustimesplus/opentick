<script setup lang="ts">
import PeopleImport from './PeopleImport.vue'
import {prefs} from '@/lib/prefs'
import {t, type MessageKey} from '@/lib/i18n'

const CUTOFFS: {key: 'peopleAboutCutoff' | 'peopleEntryCutoff', label: MessageKey}[] = [
	{key: 'peopleAboutCutoff', label: 'peopleAboutCutoff'},
	{key: 'peopleEntryCutoff', label: 'peopleEntryCutoff'},
]

function setCutoff(key: 'peopleAboutCutoff' | 'peopleEntryCutoff', e: Event) {
	const n = Math.round(Number((e.target as HTMLInputElement).value))
	prefs[key] = Number.isFinite(n) && n > 0 ? n : 0
}
</script>

<template>
	<div class="people-settings">
		<section class="card">
			<h2>{{ t('peopleSettingsDisplay') }}</h2>
			<p>{{ t('peopleSettingsDisplayHint') }}</p>
			<label v-for="c in CUTOFFS" :key="c.key" class="cutoff">
				<span class="name">{{ t(c.label) }}</span>
				<input
					class="input"
					type="number"
					min="0"
					step="10"
					:value="prefs[c.key]"
					@change="setCutoff(c.key, $event)"
				>
				<span class="unit">{{ t('peopleCharacters') }}</span>
			</label>
		</section>

		<h3>{{ t('importContacts') }}</h3>
		<PeopleImport />
	</div>
</template>

<style scoped>
.people-settings {
	flex: 1;
	min-height: 0;
	overflow: auto;
}

.card {
	max-width: 624px;
	margin: 4px 8px 0;
	padding: 18px 20px;
	border-radius: 12px;
	background: var(--bg-panel);
}

h2 {
	margin: 0 0 6px;
	font-size: 15px;
	font-weight: 600;
}

h3 {
	margin: 22px 8px 10px;
	font-size: 13px;
	font-weight: 600;
	color: var(--text-2);
}

p {
	margin: 0 0 14px;
	font-size: 13px;
	line-height: 1.5;
	color: var(--text-2);
}

.cutoff {
	display: flex;
	align-items: center;
	gap: 10px;
	font-size: 14px;
}

.cutoff + .cutoff {
	margin-top: 8px;
}

.name {
	flex: 1;
	min-width: 0;
}

.cutoff .input {
	width: 90px;
	text-align: right;
}

.unit {
	width: 70px;
	font-size: 13px;
	color: var(--text-3);
}
</style>
