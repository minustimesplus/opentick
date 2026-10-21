<script setup lang="ts">
import {computed, onMounted} from 'vue'
import {Check} from 'lucide-vue-next'
import PersonAvatar from './PersonAvatar.vue'
import type {Task} from '@/api/types'
import {useData} from '@/stores/data'
import {displayName} from '@/lib/tasks'
import {t} from '@/lib/i18n'

const props = defineProps<{task: Task}>()
const data = useData()

onMounted(() => data.loadMembers(props.task.project_id))

const members = computed(() => data.members.get(props.task.project_id) ?? [])
const assigned = computed(() => new Set((props.task.assignees ?? []).map(u => u.id)))

function toggle(id: number) {
	const u = members.value.find(m => m.id === id)
	if (!u) return
	if (assigned.value.has(id)) data.unassign(props.task.id, id)
	else data.assign(props.task.id, u)
}
</script>

<template>
	<div class="menu assignees">
		<div class="menu-label">{{ t('assignTo') }}</div>
		<button v-for="m in members" :key="m.id" class="menu-item" @click="toggle(m.id)">
			<PersonAvatar :user="m" :size="22" />
			<span class="name">{{ displayName(m) }}<span v-if="m.id === data.user?.id" class="muted"> ({{ t('you') }})</span></span>
			<Check :size="15" :style="{visibility: assigned.has(m.id) ? 'visible' : 'hidden'}" class="check" />
		</button>
		<div v-if="!members.length" class="menu-label">{{ t('noMembers') }}</div>
	</div>
</template>

<style scoped>
.assignees {
	width: 240px;
}

.name {
	flex: 1;
	overflow: hidden;
	text-overflow: ellipsis;
}

.check {
	color: var(--accent);
}
</style>
