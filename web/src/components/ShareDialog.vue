<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {UserPlus, X} from 'lucide-vue-next'
import Modal from './Modal.vue'
import PersonAvatar from './PersonAvatar.vue'
import {api, ApiError, fetchAll} from '@/api/client'
import type {Project, ProjectMember, User} from '@/api/types'
import {useData} from '@/stores/data'
import {displayName, splitEmoji} from '@/lib/tasks'
import {t, type MessageKey} from '@/lib/i18n'

const props = defineProps<{project: Project}>()
const emit = defineEmits<{close: []}>()
const data = useData()

const shares = ref<ProjectMember[]>([])
const loading = ref(true)
const canManage = ref(true)
const query = ref('')
const permission = ref<0 | 1 | 2>(1)
const error = ref('')
const busy = ref(false)

const PERMISSIONS: {value: 0 | 1 | 2, label: MessageKey}[] = [
	{value: 0, label: 'permRead'},
	{value: 1, label: 'permWrite'},
	{value: 2, label: 'permAdmin'},
]

const title = computed(() => splitEmoji(props.project.title).name)
const owner = computed<User | null>(() => props.project.owner ?? null)

async function load() {
	try {
		shares.value = await fetchAll<ProjectMember>(`/projects/${props.project.id}/users`, {}, 5)
	} catch (e) {
		// Only list admins may see and change shares.
		if (e instanceof ApiError && e.status === 403) canManage.value = false
		shares.value = []
	} finally {
		loading.value = false
	}
}

onMounted(load)

async function invite() {
	const q = query.value.trim()
	if (!q) return
	error.value = ''
	busy.value = true
	try {
		// User search only matches exact usernames (and emails if the server allows it).
		const found = await fetchAll<User>('/users', {q}, 1)
		const username = found[0]?.username ?? q
		await api('POST', `/projects/${props.project.id}/users`, {body: {username, permission: permission.value}})
		query.value = ''
		await load()
		data.loadMembers(props.project.id, true)
	} catch (e) {
		error.value = e instanceof ApiError && e.status === 404 ? t('userNotFound', {name: q}) : (e as Error).message
	} finally {
		busy.value = false
	}
}

async function setPermission(m: ProjectMember, value: 0 | 1 | 2) {
	const before = m.permission
	m.permission = value
	try {
		await api('PUT', `/projects/${props.project.id}/users/${encodeURIComponent(m.username)}`, {body: {permission: value}})
	} catch (e) {
		m.permission = before
		error.value = (e as Error).message
	}
}

async function remove(m: ProjectMember) {
	try {
		await api('DELETE', `/projects/${props.project.id}/users/${encodeURIComponent(m.username)}`)
		shares.value = shares.value.filter(x => x.id !== m.id)
		data.loadMembers(props.project.id, true)
	} catch (e) {
		error.value = (e as Error).message
	}
}
</script>

<template>
	<Modal :width="460" @close="emit('close')">
		<div class="share">
			<header class="head">
				<h3>{{ t('shareListTitle', {name: title}) }}</h3>
				<button class="icon-btn" @click="emit('close')"><X :size="18" /></button>
			</header>

			<form v-if="canManage" class="invite" @submit.prevent="invite">
				<input v-model="query" v-focus class="input" :placeholder="t('inviteePlaceholder')">
				<select v-model="permission" class="input perm">
					<option v-for="p in PERMISSIONS" :key="p.value" :value="p.value">{{ t(p.label) }}</option>
				</select>
				<button type="submit" class="btn primary" :disabled="busy || !query.trim()" :title="t('invite')"><UserPlus :size="16" /></button>
			</form>
			<p v-if="error" class="error">{{ error }}</p>
			<p v-if="!canManage" class="muted">{{ t('shareNoPermission') }}</p>

			<div class="members">
				<div v-if="owner" class="member">
					<PersonAvatar :user="owner" :size="30" />
					<div class="who">
						<div>{{ displayName(owner) }}<span v-if="owner.id === data.user?.id" class="muted"> ({{ t('you') }})</span></div>
						<div class="muted small">@{{ owner.username }}</div>
					</div>
					<span class="muted small">{{ t('owner') }}</span>
				</div>
				<div v-for="m in shares" :key="m.id" class="member">
					<PersonAvatar :user="m" :size="30" />
					<div class="who">
						<div>{{ displayName(m) }}<span v-if="m.id === data.user?.id" class="muted"> ({{ t('you') }})</span></div>
						<div class="muted small">@{{ m.username }}</div>
					</div>
					<select v-if="canManage" class="input perm" :value="m.permission" @change="setPermission(m, Number(($event.target as HTMLSelectElement).value) as 0 | 1 | 2)">
						<option v-for="p in PERMISSIONS" :key="p.value" :value="p.value">{{ t(p.label) }}</option>
					</select>
					<button v-if="canManage" class="icon-btn" :title="t('removeMember')" @click="remove(m)"><X :size="16" /></button>
				</div>
				<div v-if="!loading && !shares.length" class="empty">
					<svg width="120" height="84" viewBox="0 0 120 84" fill="none">
						<circle cx="60" cy="44" r="38" fill="var(--bg-hover)" />
						<rect x="22" y="22" width="38" height="26" rx="4" stroke="var(--text-3)" stroke-width="2" fill="var(--bg-elev)" />
						<circle cx="34" cy="32" r="4" fill="var(--accent)" />
						<path d="M28 42c2-4 10-4 12 0" stroke="var(--accent)" stroke-width="2" />
						<rect x="56" y="34" width="38" height="30" rx="4" fill="var(--accent)" />
						<path d="M75 42v14M68 49h14" stroke="#fff" stroke-width="2.5" stroke-linecap="round" />
					</svg>
					<div class="muted">{{ t('inviteFriends') }}</div>
				</div>
			</div>
		</div>
	</Modal>
</template>

<style scoped>
.share {
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
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.invite {
	display: flex;
	gap: 6px;
}

.invite .btn {
	padding: 0 10px;
	height: 34px;
}

.perm {
	width: auto;
	flex-shrink: 0;
}

.error {
	margin: 0;
	font-size: 13px;
	color: var(--danger);
}

.members {
	display: flex;
	flex-direction: column;
	min-height: 160px;
}

.member {
	display: flex;
	align-items: center;
	gap: 10px;
	padding: 8px 0;
	border-bottom: 1px solid var(--border);
}

.who {
	flex: 1;
	min-width: 0;
	font-size: 14px;
}

.small {
	font-size: 12px;
}

.empty {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 10px;
	padding: 24px 0 8px;
	font-size: 13px;
}
</style>
