<script setup lang="ts">
import {ref} from 'vue'
import {Upload} from 'lucide-vue-next'
import {usePeople} from '@/stores/people'
import {t} from '@/lib/i18n'

const people = usePeople()
const fileInput = ref<HTMLInputElement | null>(null)
const importing = ref(false)
const result = ref('')

const COMMAND = 'opentick import-personalcrm --url ' + window.location.origin + ' --token <token>'

async function pick(e: Event) {
	const input = e.target as HTMLInputElement
	const file = input.files?.[0]
	input.value = ''
	if (!file || importing.value) return
	importing.value = true
	result.value = ''
	const res = await people.importLinkedinCsv(await file.text())
	importing.value = false
	if (res) result.value = t('importResult', {created: res.created, updated: res.updated})
}
</script>

<template>
	<div class="import">
		<section class="card">
			<h2>{{ t('importLinkedinCsv') }}</h2>
			<p>{{ t('importLinkedinCsvHint') }}</p>
			<input ref="fileInput" type="file" accept=".csv,text/csv" hidden @change="pick">
			<div class="row">
				<button class="btn primary" :aria-disabled="importing" @click="!importing && fileInput?.click()">
					<Upload :size="15" /> {{ importing ? t('importing') : t('importChooseCsv') }}
				</button>
				<span v-if="result" class="result">{{ result }}</span>
			</div>
		</section>

		<section class="card">
			<h2>{{ t('importPersonalCrm') }}</h2>
			<p>{{ t('importPersonalCrmHint') }}</p>
			<pre>{{ COMMAND }}</pre>
		</section>
	</div>
</template>

<style scoped>
.import {
	display: flex;
	flex-direction: column;
	gap: 14px;
	max-width: 640px;
	padding: 4px 8px 24px;
}

.card {
	padding: 18px 20px;
	border-radius: 12px;
	background: var(--bg-panel);
}

h2 {
	margin: 0 0 6px;
	font-size: 15px;
	font-weight: 600;
}

p {
	margin: 0 0 14px;
	font-size: 13px;
	line-height: 1.5;
	color: var(--text-2);
}

.row {
	display: flex;
	align-items: center;
	gap: 12px;
}

.result {
	font-size: 13px;
	color: var(--text-2);
}

pre {
	margin: 0;
	padding: 10px 12px;
	overflow-x: auto;
	border-radius: 6px;
	background: var(--bg-input);
	font-size: 12px;
	user-select: all;
}

.btn[aria-disabled="true"] {
	opacity: 0.5;
	cursor: default;
}
</style>
