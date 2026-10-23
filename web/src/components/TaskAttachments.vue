<script setup lang="ts">
import {computed, onBeforeUnmount, ref, watch} from 'vue'
import {Download, File, FileText, Image as ImageIcon, Trash2} from 'lucide-vue-next'
import {api} from '@/api/client'
import type {Attachment} from '@/api/types'
import {useData} from '@/stores/data'
import {fmtFileSize} from '@/lib/tasks'
import {fmtDayMonth, parseDate} from '@/lib/dates'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

const props = defineProps<{taskId: number}>()
const data = useData()

const attachments = computed(() => data.tasks.get(props.taskId)?.attachments ?? [])

const isImage = (a: Attachment) => a.file.mime.startsWith('image/')

// Image thumbnails are fetched with auth, so they are object URLs.
const thumbs = ref<Record<number, string>>({})
watch(attachments, list => {
	for (const a of list) {
		if (!isImage(a) || a.id in thumbs.value) continue
		thumbs.value[a.id] = ''
		api<Blob>('GET', `/tasks/${props.taskId}/attachments/${a.id}`, {query: {preview_size: 'md'}, blob: true})
			.then(b => { thumbs.value[a.id] = URL.createObjectURL(b) })
			.catch(() => {})
	}
}, {immediate: true})
onBeforeUnmount(() => Object.values(thumbs.value).forEach(u => u && URL.revokeObjectURL(u)))

async function open(a: Attachment) {
	try {
		const blob = await api<Blob>('GET', `/tasks/${props.taskId}/attachments/${a.id}`, {blob: true})
		const url = URL.createObjectURL(blob)
		const link = document.createElement('a')
		link.href = url
		link.download = a.file.name
		link.click()
		setTimeout(() => URL.revokeObjectURL(url), 10_000)
	} catch (e) {
		toast(String((e as Error).message))
	}
}

function remove(a: Attachment) {
	if (confirm(t('deleteAttachmentConfirm', {name: a.file.name}))) data.deleteAttachment(props.taskId, a.id)
}

function created(a: Attachment) {
	const d = parseDate(a.created)
	return d ? fmtDayMonth(d) : ''
}
</script>

<template>
	<div v-if="attachments.length" class="attachments">
		<div v-for="a in attachments" :key="a.id" class="att" :title="a.file.name" @click="open(a)">
			<div class="thumb">
				<img v-if="thumbs[a.id]" :src="thumbs[a.id]" alt="">
				<ImageIcon v-else-if="isImage(a)" :size="20" />
				<FileText v-else-if="a.file.mime.startsWith('text/') || a.file.mime.includes('pdf')" :size="20" />
				<File v-else :size="20" />
			</div>
			<div class="info">
				<div class="name">{{ a.file.name }}</div>
				<div class="muted small">{{ fmtFileSize(a.file.size) }} · {{ created(a) }}</div>
			</div>
			<button class="icon-btn act" :title="t('download')" @click.stop="open(a)"><Download :size="15" /></button>
			<button class="icon-btn act" :title="t('delete')" @click.stop="remove(a)"><Trash2 :size="15" /></button>
		</div>
	</div>
</template>

<style scoped>
.attachments {
	display: flex;
	flex-direction: column;
	gap: 6px;
	margin-top: 14px;
}

.att {
	display: flex;
	align-items: center;
	gap: 10px;
	padding: 6px 8px;
	border: 1px solid var(--border);
	border-radius: 8px;
	cursor: pointer;
}

.att:hover {
	background: var(--bg-hover);
}

.thumb {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 40px;
	height: 40px;
	flex-shrink: 0;
	overflow: hidden;
	border-radius: 6px;
	background: var(--bg-active);
	color: var(--text-3);
}

.thumb img {
	width: 100%;
	height: 100%;
	object-fit: cover;
}

.info {
	flex: 1;
	min-width: 0;
}

.name {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 14px;
}

.small {
	font-size: 12px;
}

.act {
	width: 28px;
	height: 28px;
	opacity: 0;
}

.att:hover .act {
	opacity: 1;
}
</style>
