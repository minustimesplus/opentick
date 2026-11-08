<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {Copy, Link2, Mail, UserPlus, X} from 'lucide-vue-next'
import Modal from './Modal.vue'
import PersonAvatar from './PersonAvatar.vue'
import {api, ApiError, fetchAll} from '@/api/client'
import type {Permission, Project, ProjectInvitation, ProjectMember, User} from '@/api/types'
import {useData} from '@/stores/data'
import {displayName, splitEmoji} from '@/lib/tasks'
import {DEMO} from '@/lib/localMode'
import {toast} from '@/lib/toast'
import {t} from '@/lib/i18n'
import {PERMISSIONS, permissionLabel} from './collab/permissions'

const props = defineProps<{project: Project}>()
const emit = defineEmits<{close: []}>()
const data = useData()

// Server error codes (pkg/models/error.go).
const ERR_ALREADY_HAS_ACCESS = 7002
const ERR_INVITATION_EXISTS = 24002

const shares = ref<ProjectMember[]>([])
const invitations = ref<ProjectInvitation[]>([])
const loading = ref(true)
const canManage = ref(true)
const query = ref('')
const permission = ref<Permission>(1)
const linkPermission = ref<Permission>(1)
const error = ref('')
const busy = ref(false)
/** Link of the email invitation just sent, to pass on by hand if the mail doesn't arrive. */
const sentLink = ref<{email: string, url: string} | null>(null)

const title = computed(() => splitEmoji(props.project.title).name)
const owner = computed<User | null>(() => props.project.owner ?? null)
const pending = computed(() => invitations.value.filter(i => i.kind !== 'link'))
const joinLink = computed(() => invitations.value.find(i => i.kind === 'link') ?? null)
const isEmail = computed(() => query.value.includes('@'))

async function load() {
	if (data.local) {
		loading.value = false
		return
	}
	try {
		const base = `/projects/${props.project.id}`
		const [users, invites] = await Promise.all([
			fetchAll<ProjectMember>(`${base}/users`, {}, 5),
			fetchAll<ProjectInvitation>(`${base}/invitations`, {}, 5).catch(e => {
				// Only list admins may see and send invitations.
				if (e instanceof ApiError && e.status === 403) canManage.value = false
				return [] as ProjectInvitation[]
			}),
		])
		shares.value = users
		invitations.value = invites
	} catch (e) {
		if (e instanceof ApiError && e.status === 403) canManage.value = false
		shares.value = []
	} finally {
		loading.value = false
	}
}

onMounted(load)

function inviteError(e: unknown, who: string): string {
	if (e instanceof ApiError) {
		if (e.code === ERR_INVITATION_EXISTS) return t('alreadyInvited')
		if (e.code === ERR_ALREADY_HAS_ACCESS) return t('alreadyMember')
		if (e.status === 404) return t('userNotFound', {name: who})
	}
	return (e as Error).message
}

async function invite() {
	const q = query.value.trim()
	if (!q) return
	error.value = ''
	sentLink.value = null
	busy.value = true
	try {
		const body = isEmail.value
			? {email: q, permission: permission.value}
			: {username: q.replace(/^@/, ''), permission: permission.value}
		const res = await api<ProjectInvitation>('POST', `/projects/${props.project.id}/invitations`, {body})
		query.value = ''
		if (res.status === 'accepted') {
			toast(t('memberAdded', {name: res.invitee ? displayName(res.invitee) : q}))
			data.loadMembers(props.project.id, true)
		} else {
			toast(t('invitationSent', {name: res.email || (res.invitee ? displayName(res.invitee) : q)}))
			if (res.kind === 'email' && res.invite_url) sentLink.value = {email: res.email ?? q, url: res.invite_url}
		}
		await load()
	} catch (e) {
		error.value = inviteError(e, q)
	} finally {
		busy.value = false
	}
}

async function setPermission(m: ProjectMember, value: Permission) {
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

async function revoke(inv: ProjectInvitation) {
	try {
		await api('DELETE', `/projects/${props.project.id}/invitations/${inv.id}`)
		invitations.value = invitations.value.filter(x => x.id !== inv.id)
		if (sentLink.value && inv.email === sentLink.value.email) sentLink.value = null
	} catch (e) {
		error.value = (e as Error).message
	}
}

async function createLink() {
	error.value = ''
	try {
		const res = await api<ProjectInvitation>('POST', `/projects/${props.project.id}/invitations`, {body: {link: true, permission: linkPermission.value}})
		invitations.value = [...invitations.value, res]
		if (res.invite_url) copy(res.invite_url)
	} catch (e) {
		error.value = (e as Error).message
	}
}

async function copy(text: string) {
	try {
		await navigator.clipboard.writeText(text)
		toast(t('copied'))
	} catch {
		// Clipboard blocked: the link is shown, so it can still be copied by hand.
	}
}

function inviteeName(inv: ProjectInvitation) {
	return inv.invitee ? displayName(inv.invitee) : (inv.email ?? '')
}
</script>

<template>
	<Modal :width="480" @close="emit('close')">
		<div class="share">
			<header class="head">
				<h3>{{ t('shareListTitle', {name: title}) }}</h3>
				<button class="icon-btn" @click="emit('close')"><X :size="18" /></button>
			</header>

			<p v-if="data.local" class="muted">{{ DEMO ? t('collabDemo') : t('collabNeedsAccount') }}</p>

			<template v-else>
				<form v-if="canManage" class="invite" @submit.prevent="invite">
					<input v-model="query" v-focus class="input" :placeholder="t('inviteePlaceholder')" autocomplete="off">
					<select v-model="permission" class="input perm">
						<option v-for="p in PERMISSIONS" :key="p.value" :value="p.value">{{ t(p.label) }}</option>
					</select>
					<button type="submit" class="btn primary" :disabled="busy || !query.trim()" :title="t('invite')">
						<Mail v-if="isEmail" :size="16" />
						<UserPlus v-else :size="16" />
					</button>
				</form>
				<p v-if="error" class="error">{{ error }}</p>
				<p v-if="!canManage" class="muted">{{ t('shareNoPermission') }}</p>

				<div v-if="sentLink" class="sent">
					<div class="muted small">{{ t('emailInviteFallback') }}</div>
					<div class="link-row">
						<input class="input" readonly :value="sentLink.url" @focus="($event.target as HTMLInputElement).select()">
						<button class="btn" :title="t('copyLink')" @click="copy(sentLink.url)"><Copy :size="16" /></button>
					</div>
				</div>

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
						<select v-if="canManage" class="input perm" :value="m.permission" @change="setPermission(m, Number(($event.target as HTMLSelectElement).value) as Permission)">
							<option v-for="p in PERMISSIONS" :key="p.value" :value="p.value">{{ t(p.label) }}</option>
						</select>
						<span v-else class="muted small">{{ t(permissionLabel(m.permission)) }}</span>
						<button v-if="canManage" class="icon-btn" :title="t('removeMember')" @click="remove(m)"><X :size="16" /></button>
					</div>
					<div v-for="inv in pending" :key="'inv' + inv.id" class="member pending">
						<PersonAvatar v-if="inv.invitee" :user="inv.invitee" :size="30" />
						<span v-else class="mail-avatar"><Mail :size="15" /></span>
						<div class="who">
							<div class="name">{{ inviteeName(inv) }}</div>
							<div class="muted small">{{ t('pendingInvite') }} · {{ t(permissionLabel(inv.permission)) }}</div>
						</div>
						<button class="icon-btn" :title="t('revokeInvite')" @click="revoke(inv)"><X :size="16" /></button>
					</div>
					<div v-if="!loading && !shares.length && !pending.length" class="empty">
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

				<section v-if="canManage" class="link-box">
					<div class="link-head">
						<Link2 :size="16" />
						<span>{{ t('inviteLinkTitle') }}</span>
					</div>
					<div class="muted small">{{ t('inviteLinkDesc') }}</div>
					<template v-if="joinLink">
						<div class="link-row">
							<input class="input" readonly :value="joinLink.invite_url" @focus="($event.target as HTMLInputElement).select()">
							<button class="btn" :title="t('copyLink')" @click="copy(joinLink.invite_url ?? '')"><Copy :size="16" /></button>
						</div>
						<div class="link-foot">
							<span class="muted small">{{ t(permissionLabel(joinLink.permission)) }} · {{ t('inviteLinkUses', {n: joinLink.uses}) }}</span>
							<button class="link" @click="revoke(joinLink)">{{ t('disableLink') }}</button>
						</div>
					</template>
					<div v-else class="link-row">
						<select v-model="linkPermission" class="input perm">
							<option v-for="p in PERMISSIONS" :key="p.value" :value="p.value">{{ t(p.label) }}</option>
						</select>
						<button class="btn" @click="createLink">{{ t('createInviteLink') }}</button>
					</div>
				</section>
			</template>
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

.invite,
.link-row {
	display: flex;
	gap: 6px;
}

.invite .btn,
.link-row .btn {
	padding: 0 10px;
	height: 34px;
	flex-shrink: 0;
}

.link-row .input {
	min-width: 0;
	flex: 1;
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

.sent {
	display: flex;
	flex-direction: column;
	gap: 6px;
}

.members {
	display: flex;
	flex-direction: column;
	min-height: 120px;
	max-height: 320px;
	overflow-y: auto;
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

.name {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.mail-avatar {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 30px;
	height: 30px;
	flex-shrink: 0;
	border-radius: 8px;
	color: var(--text-2);
	background: var(--bg-hover);
}

.small {
	font-size: 12px;
}

.empty {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 10px;
	padding: 16px 0 8px;
	font-size: 13px;
}

.link-box {
	display: flex;
	flex-direction: column;
	gap: 6px;
	padding: 12px;
	border-radius: 8px;
	background: var(--bg-hover);
}

.link-head {
	display: flex;
	align-items: center;
	gap: 6px;
	font-size: 14px;
	font-weight: 500;
}

.link-foot {
	display: flex;
	align-items: center;
	justify-content: space-between;
}

.link {
	padding: 0;
	border: 0;
	background: none;
	font-size: 12px;
	color: var(--danger);
	cursor: pointer;
}
</style>
