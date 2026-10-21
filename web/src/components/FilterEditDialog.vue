<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {Filter, X} from 'lucide-vue-next'
import Modal from './Modal.vue'
import MultiSelect from './MultiSelect.vue'
import {api, fetchAll} from '@/api/client'
import type {Project, Task} from '@/api/types'
import {useData} from '@/stores/data'
import {EMPTY_RULES, filterIdOf, fromFilter, toFilter, type DateRange, type FilterRules} from '@/lib/filters'
import {priorityName, splitEmoji, type Level} from '@/lib/tasks'
import {t, type MessageKey} from '@/lib/i18n'

/** The saved filter's pseudo project, or null for a new filter. */
const props = defineProps<{filter?: Project | null}>()
const emit = defineEmits<{close: [], saved: [number]}>()
const data = useData()

const filterId = computed(() => props.filter ? filterIdOf(props.filter.id) : null)

const name = ref(props.filter?.title ?? '')
const mode = ref<'normal' | 'advanced'>('normal')
const rules = ref<FilterRules>({...EMPTY_RULES, lists: [], labels: [], priorities: []})
const raw = ref('done = false')
const loading = ref(!!props.filter)
const preview = ref<number | null>(null)

onMounted(async () => {
	if (filterId.value === null) return
	try {
		const f = await api<{title: string, filters?: {filter?: string}}>('GET', `/filters/${filterId.value}`)
		name.value = f.title
		raw.value = f.filters?.filter ?? ''
		const parsed = fromFilter(raw.value)
		if (parsed) rules.value = parsed
		else mode.value = 'advanced'
	} finally {
		loading.value = false
	}
})

// Switching to Advanced starts from the Normal settings; switching back only works for filters Normal can show.
const canNormal = computed(() => mode.value === 'normal' || fromFilter(raw.value) !== null)
watch(mode, (m, old) => {
	if (m === 'advanced' && old === 'normal') raw.value = toFilter(rules.value)
	if (m === 'normal') rules.value = fromFilter(raw.value) ?? rules.value
	preview.value = null
})
watch(rules, () => { preview.value = null }, {deep: true})

const filterString = computed(() => mode.value === 'normal' ? toFilter(rules.value) : raw.value.trim())

const listOptions = computed(() => data.realProjects.map(p => ({
	value: p.id,
	label: p.id === data.inboxId ? t('inbox') : splitEmoji(p.title).name,
})))
const labelOptions = computed(() => data.labels.map(l => ({value: l.id, label: l.title})))

const DATES: {key: DateRange, label: MessageKey}[] = [
	{key: 'all', label: 'all'},
	{key: 'overdue', label: 'overdue'},
	{key: 'today', label: 'today'},
	{key: 'tomorrow', label: 'tomorrow'},
	{key: 'next7', label: 'next7'},
	{key: 'next30', label: 'next30'},
]

const LEVELS: Level[] = [3, 2, 1, 0]

function togglePriority(l: Level) {
	const p = rules.value.priorities
	rules.value.priorities = p.includes(l) ? p.filter(x => x !== l) : [...p, l]
}

async function runPreview() {
	try {
		const res = await fetchAll<Task>('/tasks', {filter: filterString.value}, 2)
		preview.value = res.length
	} catch (e) {
		preview.value = null
		alert((e as Error).message)
	}
}

const saving = ref(false)

async function save() {
	if (!name.value.trim() || !filterString.value) return
	saving.value = true
	const id = await data.saveFilter(filterId.value, name.value.trim(), filterString.value)
	saving.value = false
	if (id !== null) {
		emit('saved', id)
		emit('close')
	}
}
</script>

<template>
	<Modal :width="520" @close="emit('close')">
		<form class="form" @submit.prevent="save">
			<header class="head">
				<h3>{{ filter ? t('editFilter') : t('addFilter') }}</h3>
				<button type="button" class="icon-btn" @click="emit('close')"><X :size="18" /></button>
			</header>
			<div class="tabs">
				<button type="button" :class="{on: mode === 'normal'}" :disabled="!canNormal" @click="mode = 'normal'">{{ t('filterNormal') }}</button>
				<button type="button" :class="{on: mode === 'advanced'}" @click="mode = 'advanced'">{{ t('filterAdvanced') }}</button>
			</div>
			<div class="name">
				<Filter :size="16" class="name-ic" />
				<input v-model="name" class="input" :placeholder="t('name')" v-focus>
			</div>

			<div v-if="loading" class="muted">…</div>
			<template v-else-if="mode === 'normal'">
				<div class="row">
					<label>{{ t('lists') }}</label>
					<MultiSelect v-model="rules.lists" :options="listOptions" />
				</div>
				<div class="row">
					<label>{{ t('tags') }}</label>
					<MultiSelect v-model="rules.labels" :options="labelOptions" />
				</div>
				<div class="row">
					<label>{{ t('date') }}</label>
					<select v-model="rules.date" class="input">
						<option v-for="d in DATES" :key="d.key" :value="d.key">{{ t(d.label) }}</option>
					</select>
				</div>
				<div class="row">
					<label>{{ t('priority') }}</label>
					<div class="checks">
						<label class="check"><input type="checkbox" :checked="!rules.priorities.length" @change="rules.priorities = []"> {{ t('all') }}</label>
						<label v-for="l in LEVELS" :key="l" class="check">
							<input type="checkbox" :checked="rules.priorities.includes(l)" @change="togglePriority(l)"> {{ priorityName(l) }}
						</label>
					</div>
				</div>
				<div class="row">
					<label>{{ t('filterInclude') }}</label>
					<input v-model="rules.keyword" class="input" :placeholder="t('filterKeywords')">
				</div>
			</template>
			<template v-else>
				<textarea v-model="raw" class="input raw" rows="4" spellcheck="false" />
				<p class="muted hint">{{ t('filterAdvancedHint') }}</p>
			</template>

			<footer class="actions">
				<button type="button" class="link" @click="runPreview">{{ t('preview') }}</button>
				<span v-if="preview !== null" class="muted">{{ t('previewCount', {n: preview}) }}</span>
				<span class="spacer" />
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button type="submit" class="btn primary" :disabled="saving || !name.trim()">{{ t('save') }}</button>
			</footer>
		</form>
	</Modal>
</template>

<style scoped>
.form {
	display: flex;
	flex-direction: column;
	gap: 12px;
}

.head {
	display: flex;
	align-items: center;
	justify-content: space-between;
}

h3 {
	margin: 0;
}

.tabs {
	display: flex;
	justify-content: center;
	gap: 6px;
}

.tabs button {
	height: 26px;
	padding: 0 14px;
	border-radius: 13px;
	font-size: 13px;
	color: var(--text-3);
	background: var(--bg-hover);
}

.tabs button.on {
	color: var(--accent);
	background: color-mix(in srgb, var(--accent) 16%, transparent);
}

.tabs button:disabled {
	opacity: 0.5;
}

.name {
	position: relative;
}

.name .input {
	padding-left: 32px;
}

.name-ic {
	position: absolute;
	left: 10px;
	top: 9px;
	color: var(--text-3);
}

.row {
	display: grid;
	grid-template-columns: 70px 1fr;
	align-items: center;
	gap: 10px;
}

.row > label {
	font-size: 13px;
	color: var(--text-2);
}

.checks {
	display: flex;
	flex-wrap: wrap;
	gap: 14px;
}

.check {
	display: inline-flex;
	align-items: center;
	gap: 5px;
	font-size: 13px;
}

.raw {
	height: auto;
	padding: 8px 10px;
	font-family: ui-monospace, monospace;
	font-size: 13px;
	resize: vertical;
}

.hint {
	margin: 0;
	font-size: 12px;
}

.actions {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-top: 8px;
}

.link {
	color: var(--accent);
}

.spacer {
	flex: 1;
}
</style>
