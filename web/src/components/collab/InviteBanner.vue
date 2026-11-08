<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {Users} from 'lucide-vue-next'
import {api} from '@/api/client'
import type {InvitationPreview} from '@/api/types'
import {splitEmoji} from '@/lib/tasks'
import {t} from '@/lib/i18n'

// Shown above the login screen after opening an invitation link, so the
// invitee knows why they should sign in.
const props = defineProps<{token: string}>()
const preview = ref<InvitationPreview | null>(null)

onMounted(async () => {
	try {
		preview.value = await api<InvitationPreview>('POST', '/project-invitations/preview', {auth: false, body: {token: props.token}})
	} catch {
		// Invalid links are explained after sign in.
	}
})
</script>

<template>
	<div v-if="preview" class="banner" role="status">
		<Users :size="16" />
		<span>
			{{ t('invitationFrom', {name: preview.inviter_name, list: splitEmoji(preview.project_title).name}) }}.
			{{ t('inviteSignIn') }}
		</span>
	</div>
</template>

<style scoped>
.banner {
	position: fixed;
	top: calc(12px + env(safe-area-inset-top));
	left: 50%;
	z-index: 50;
	display: flex;
	align-items: center;
	gap: 8px;
	max-width: calc(100vw - 32px);
	padding: 8px 14px;
	transform: translateX(-50%);
	border-radius: 10px;
	font-size: 13px;
	color: var(--text);
	background: var(--bg-elev);
	box-shadow: var(--shadow);
}

.banner svg {
	flex-shrink: 0;
	color: var(--accent);
}
</style>
