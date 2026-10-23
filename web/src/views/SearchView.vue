<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {Search} from 'lucide-vue-next'
import TaskRow from '@/components/TaskRow.vue'
import {useData} from '@/stores/data'
import {openTaskDrawer} from '@/lib/ui'
import {t} from '@/lib/i18n'

const data = useData()
const q = ref('')
const input = ref<HTMLInputElement | null>(null)
const remoteIds = ref<number[]>([])

onMounted(() => input.value?.focus())

let timer: number | undefined
watch(q, v => {
	clearTimeout(timer)
	remoteIds.value = []
	if (v.trim().length < 2) return
	timer = window.setTimeout(async () => {
		try {
			remoteIds.value = (await data.searchRemote(v.trim())).map(t => t.id)
		} catch {
			// local results still show
		}
	}, 300)
})

function norm(s: string) {
	return s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()
}

const results = computed(() => {
	const needle = norm(q.value.trim())
	if (!needle) return []
	const ids = new Set(remoteIds.value)
	return data.allTasks
		.filter(task => ids.has(task.id) || norm(task.title).includes(needle) || norm(task.description ?? '').includes(needle) || task.labels?.some(l => norm(l.title).includes(needle)))
		.sort((a, b) => Number(a.done) - Number(b.done) || b.id - a.id)
})
</script>

<template>
	<div class="search-view">
		<div class="bar">
			<Search :size="18" class="ic" />
			<input ref="input" v-model="q" :placeholder="t('searchPlaceholder')">
		</div>
		<div class="results">
			<TaskRow v-for="task in results" :key="task.id" :task="task" show-list @select="openTaskDrawer" />
			<div v-if="q.trim() && !results.length" class="empty-state">{{ t('nothingFound') }}</div>
		</div>
	</div>
</template>

<style scoped>
.search-view {
	display: flex;
	flex-direction: column;
	height: 100%;
	max-width: 900px;
	margin: 0 auto;
	padding: 24px;
}

.bar {
	display: flex;
	align-items: center;
	gap: 10px;
	height: 46px;
	padding: 0 16px;
	border-radius: 10px;
	background: var(--bg-input);
	margin-bottom: 16px;
}

.ic {
	color: var(--text-3);
}

input {
	flex: 1;
	border: 0;
	outline: 0;
	background: transparent;
	font-size: 16px;
}

.results {
	flex: 1;
	overflow: auto;
}

.empty-state {
	height: 200px;
}

@media (max-width: 767px) {
	.search-view {
		padding: 12px 10px 0;
	}

	.bar {
		margin-bottom: 8px;
	}
}
</style>
