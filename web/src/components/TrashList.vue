<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {RotateCcw, Trash2, X} from 'lucide-vue-next'
import type {Task} from '@/api/types'
import {api} from '@/api/client'
import {useData} from '@/stores/data'
import {projectColor, splitEmoji} from '@/lib/tasks'
import {fmtDayMonth, parseDate} from '@/lib/dates'
import {t} from '@/lib/i18n'

const data = useData()

/** Days until the server purges trashed tasks; 0 keeps them, null while unknown. */
const retentionDays = ref<number | null>(null)

onMounted(async () => {
	data.loadTrash()
	try {
		const info = await api<{trash_retention_days?: number}>('GET', '/info', {auth: false})
		retentionDays.value = info.trash_retention_days ?? null
	} catch {
		// The hint is optional.
	}
})

function projectOf(task: Task) {
	return data.project(task.project_id)
}

function deletedOn(task: Task) {
	const d = parseDate(task.deleted_at)
	return d ? t('deletedOn', {date: fmtDayMonth(d)}) : ''
}

function deleteForever(task: Task) {
	if (!confirm(t('deleteForeverConfirm', {name: task.title}))) return
	data.deleteForever(task.id)
}

function emptyTrash() {
	if (!confirm(t('emptyTrashConfirm', {n: data.trash.length}))) return
	data.emptyTrash()
}
</script>

<template>
	<div class="trash">
		<div v-if="data.trash.length" class="bar">
			<span v-if="retentionDays !== null" class="hint">
				{{ retentionDays > 0 ? t('trashHint', {n: retentionDays}) : t('trashHintKeep') }}
			</span>
			<button class="btn danger" @click="emptyTrash"><Trash2 :size="15" /> {{ t('emptyTrash') }}</button>
		</div>
		<div v-for="task in data.trash" :key="task.id" class="row">
			<span class="title">{{ task.title }}</span>
			<span v-if="projectOf(task)" class="list">
				<span class="dot" :style="{background: projectColor(projectOf(task)!)}" />
				{{ splitEmoji(projectOf(task)!.title).name }}
			</span>
			<span class="when">{{ deletedOn(task) }}</span>
			<button class="icon-btn" :title="t('restore')" @click="data.restoreTask(task.id)"><RotateCcw :size="16" /></button>
			<button class="icon-btn" :title="t('deleteForever')" @click="deleteForever(task)"><X :size="17" /></button>
		</div>
		<div v-if="!data.trash.length" class="empty-state">
			<Trash2 :size="56" :stroke-width="1.2" class="empty-ic" />
			<div>{{ t('trashEmpty') }}</div>
			<div v-if="retentionDays !== null" class="sub">
				{{ retentionDays > 0 ? t('trashHint', {n: retentionDays}) : t('trashHintKeep') }}
			</div>
		</div>
	</div>
</template>

<style scoped>
.bar {
	display: flex;
	align-items: center;
	gap: 12px;
	margin-bottom: 8px;
}

.hint {
	flex: 1;
	font-size: 13px;
	color: var(--text-3);
}

.bar .btn {
	margin-left: auto;
	display: inline-flex;
	align-items: center;
	gap: 6px;
}

.btn.danger {
	color: var(--danger);
}

.row {
	display: flex;
	align-items: center;
	gap: 10px;
	min-height: 44px;
	padding: 0 8px 0 16px;
	border-radius: 6px;
	border-bottom: 1px solid var(--border);
}

.row:hover {
	background: var(--bg-hover);
}

.title {
	flex: 1;
	min-width: 0;
	font-size: 15px;
	color: var(--text-2);
	overflow: hidden;
	white-space: nowrap;
	text-overflow: ellipsis;
}

.list {
	display: inline-flex;
	align-items: center;
	gap: 5px;
	max-width: 30%;
	font-size: 12px;
	color: var(--text-3);
	overflow: hidden;
	white-space: nowrap;
	text-overflow: ellipsis;
}

.dot {
	width: 7px;
	height: 7px;
	flex-shrink: 0;
	border-radius: 50%;
}

.when {
	flex-shrink: 0;
	font-size: 12px;
	color: var(--text-3);
}

.empty-ic {
	color: var(--text-4);
}

@media (max-width: 640px) {
	.list {
		display: none;
	}
}
</style>
