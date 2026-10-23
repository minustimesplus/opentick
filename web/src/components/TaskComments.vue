<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue'
import {Pencil, Trash2} from 'lucide-vue-next'
import PersonAvatar from './PersonAvatar.vue'
import {api, fetchAll} from '@/api/client'
import type {TaskComment} from '@/api/types'
import {useData} from '@/stores/data'
import {displayName} from '@/lib/tasks'
import {fmtDayMonth, fmtTime, parseDate} from '@/lib/dates'
import {renderMarkdown} from '@/lib/markdown'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

const props = defineProps<{taskId: number}>()
const data = useData()

const comments = ref<TaskComment[]>([])
const composing = ref(false)
const draft = ref('')
const editing = ref<number | null>(null)
const editDraft = ref('')
const inputEl = ref<HTMLTextAreaElement | null>(null)

const count = computed(() => data.tasks.get(props.taskId)?.comment_count ?? 0)

async function load() {
	try {
		comments.value = await fetchAll<TaskComment>(`/tasks/${props.taskId}/comments`, {format: 'markdown'}, 10)
		data.patchLocal(props.taskId, {comment_count: comments.value.length})
	} catch {
		comments.value = []
	}
}

watch(() => props.taskId, load, {immediate: true})

async function focusComposer() {
	composing.value = true
	await nextTick()
	inputEl.value?.focus()
	inputEl.value?.scrollIntoView({block: 'nearest'})
}

async function send() {
	const text = draft.value.trim()
	if (!text) return
	draft.value = ''
	try {
		const c = await api<TaskComment>('POST', `/tasks/${props.taskId}/comments`, {body: {comment: text}, query: {format: 'markdown'}})
		comments.value = [...comments.value, c]
		data.patchLocal(props.taskId, {comment_count: comments.value.length})
	} catch (e) {
		draft.value = text
		toast(String((e as Error).message))
	}
}

function startEdit(c: TaskComment) {
	editing.value = c.id
	editDraft.value = c.comment
}

async function saveEdit(c: TaskComment) {
	const text = editDraft.value.trim()
	editing.value = null
	if (!text || text === c.comment) return
	try {
		const updated = await api<TaskComment>('PUT', `/tasks/${props.taskId}/comments/${c.id}`, {body: {comment: text}, query: {format: 'markdown'}})
		comments.value = comments.value.map(x => x.id === c.id ? {...x, ...updated, author: x.author} : x)
	} catch (e) {
		toast(String((e as Error).message))
	}
}

async function remove(c: TaskComment) {
	if (!confirm(t('deleteCommentConfirm'))) return
	const before = comments.value
	comments.value = before.filter(x => x.id !== c.id)
	try {
		await api('DELETE', `/tasks/${props.taskId}/comments/${c.id}`)
		data.patchLocal(props.taskId, {comment_count: comments.value.length})
	} catch (e) {
		comments.value = before
		toast(String((e as Error).message))
	}
}

function when(c: TaskComment) {
	const d = parseDate(c.created)
	return d ? `${fmtDayMonth(d)} ${fmtTime(d)}` : ''
}

function onKey(e: KeyboardEvent) {
	if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
		e.preventDefault()
		send()
	} else if (e.key === 'Escape') {
		e.stopPropagation()
		if (!draft.value.trim()) composing.value = false
	}
}

defineExpose({focusComposer, count})
</script>

<template>
	<div v-if="comments.length || composing" class="comments">
		<div v-if="comments.length" class="c-head">{{ t('comments') }} · {{ comments.length }}</div>
		<div v-for="c in comments" :key="c.id" class="comment">
			<PersonAvatar :user="c.author" :size="26" />
			<div class="c-main">
				<div class="c-meta">
					<span class="c-name">{{ displayName(c.author) }}</span>
					<span class="muted">{{ when(c) }}</span>
					<span class="spacer" />
					<template v-if="c.author.id === data.user?.id && editing !== c.id">
						<button class="icon-btn c-act" :title="t('edit')" @click="startEdit(c)"><Pencil :size="13" /></button>
						<button class="icon-btn c-act" :title="t('delete')" @click="remove(c)"><Trash2 :size="13" /></button>
					</template>
				</div>
				<textarea
					v-if="editing === c.id"
					v-model="editDraft"
					v-focus
					class="input c-input"
					rows="2"
					@keydown.enter.exact.prevent="saveEdit(c)"
					@keydown.esc.stop="editing = null"
					@blur="saveEdit(c)"
				/>
				<div v-else class="c-body md" v-html="renderMarkdown(c.comment)" />
			</div>
		</div>
		<div v-if="composing" class="composer">
			<textarea
				ref="inputEl"
				v-model="draft"
				class="input c-input"
				rows="2"
				:placeholder="t('commentPlaceholder')"
				@keydown="onKey"
			/>
			<div class="c-actions">
				<button class="btn" @click="composing = false; draft = ''">{{ t('cancel') }}</button>
				<button class="btn primary" :disabled="!draft.trim()" @click="send">{{ t('comment') }}</button>
			</div>
		</div>
	</div>
</template>

<style scoped>
.comments {
	margin-top: 18px;
	padding-top: 12px;
	border-top: 1px solid var(--border);
}

.c-head {
	margin-bottom: 8px;
	font-size: 12px;
	font-weight: 600;
	color: var(--text-2);
}

.comment {
	display: flex;
	gap: 10px;
	padding: 6px 0;
}

.c-main {
	flex: 1;
	min-width: 0;
}

.c-meta {
	display: flex;
	align-items: center;
	gap: 8px;
	min-height: 24px;
	font-size: 12px;
}

.c-name {
	font-weight: 600;
	color: var(--text-2);
}

.spacer {
	flex: 1;
}

.c-act {
	width: 24px;
	height: 24px;
	opacity: 0;
}

.comment:hover .c-act {
	opacity: 1;
}

.c-body {
	font-size: 14px;
	line-height: 1.5;
	overflow-wrap: anywhere;
}

.c-body :deep(p) {
	margin: 0 0 4px;
}

.c-input {
	height: auto;
	padding: 8px 10px;
	resize: vertical;
	line-height: 1.4;
}

.composer {
	margin-top: 8px;
}

.c-actions {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	margin-top: 8px;
}
</style>
