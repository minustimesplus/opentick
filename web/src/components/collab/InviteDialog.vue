<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useRouter} from 'vue-router'
import {Users, X} from 'lucide-vue-next'
import Modal from '../Modal.vue'
import {api, ApiError} from '@/api/client'
import type {InvitationPreview} from '@/api/types'
import {useData} from '@/stores/data'
import {DEMO} from '@/lib/localMode'
import {forgetInviteToken} from '@/lib/inviteLink'
import {splitEmoji} from '@/lib/tasks'
import {toast} from '@/lib/toast'
import {t} from '@/lib/i18n'
import {permissionLabel} from './permissions'

// Asks a signed in user whether to join the list an invitation link leads to.
const props = defineProps<{token: string}>()
const emit = defineEmits<{close: []}>()
const data = useData()
const router = useRouter()

const preview = ref<InvitationPreview | null>(null)
const invalid = ref(false)
const error = ref('')
const busy = ref(false)

const listName = computed(() => preview.value ? splitEmoji(preview.value.project_title).name : '')

onMounted(async () => {
	// Without an account requests go to the in-browser backend, which knows
	// nothing about the server the invitation is from.
	if (data.local) return
	try {
		preview.value = await api<InvitationPreview>('POST', '/project-invitations/preview', {auth: false, body: {token: props.token}})
	} catch (e) {
		if (e instanceof ApiError && e.status === 404) invalid.value = true
		else error.value = (e as Error).message
	}
})

function close() {
	forgetInviteToken()
	emit('close')
}

async function join() {
	busy.value = true
	error.value = ''
	try {
		const id = await data.claimInvitation(props.token)
		forgetInviteToken()
		toast(t('joinedList', {name: listName.value}))
		router.push(`/tasks/p${id}`)
		emit('close')
	} catch (e) {
		if (e instanceof ApiError && e.status === 404) invalid.value = true
		else error.value = (e as Error).message
	} finally {
		busy.value = false
	}
}

/** Leaves local mode; the token stays, so joining continues after sign in. */
function signIn() {
	emit('close')
	data.logout()
}
</script>

<template>
	<Modal :width="400" @close="close">
		<div class="invite">
			<button class="icon-btn close" @click="close"><X :size="18" /></button>
			<div class="badge"><Users :size="26" /></div>

			<template v-if="data.local">
				<p class="muted">{{ DEMO ? t('collabDemo') : t('collabNeedsAccount') }}</p>
				<div class="actions">
					<button class="btn" @click="close">{{ t('cancel') }}</button>
					<button v-if="!DEMO" class="btn primary" @click="signIn">{{ t('login') }}</button>
				</div>
			</template>

			<template v-else-if="invalid">
				<p class="muted">{{ t('inviteInvalid') }}</p>
				<div class="actions">
					<button class="btn" @click="close">{{ t('cancel') }}</button>
				</div>
			</template>

			<template v-else-if="preview">
				<h3>{{ t('inviteTitle', {name: listName}) }}</h3>
				<p class="muted">{{ t('inviteBy', {name: preview.inviter_name}) }} · {{ t(permissionLabel(preview.permission)) }}</p>
				<p v-if="error" class="error">{{ error }}</p>
				<div class="actions">
					<button class="btn" @click="close">{{ t('decline') }}</button>
					<button class="btn primary" :disabled="busy" @click="join">{{ t('inviteJoin') }}</button>
				</div>
			</template>

			<template v-else>
				<p v-if="error" class="error">{{ error }}</p>
				<div v-else class="spinner" />
			</template>
		</div>
	</Modal>
</template>

<style scoped>
.invite {
	position: relative;
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 10px;
	text-align: center;
}

.close {
	position: absolute;
	top: -8px;
	right: -8px;
}

.badge {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 56px;
	height: 56px;
	border-radius: 50%;
	color: var(--accent);
	background: color-mix(in srgb, var(--accent) 14%, transparent);
}

h3 {
	margin: 0;
	overflow-wrap: anywhere;
}

p {
	margin: 0;
	font-size: 14px;
}

.error {
	color: var(--danger);
}

.actions {
	display: flex;
	gap: 8px;
	margin-top: 8px;
}

.spinner {
	width: 22px;
	height: 22px;
	border: 2px solid var(--border);
	border-top-color: var(--accent);
	border-radius: 50%;
	animation: spin 0.8s linear infinite;
}

@keyframes spin {
	to {
		transform: rotate(360deg);
	}
}
</style>
