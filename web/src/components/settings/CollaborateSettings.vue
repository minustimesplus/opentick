<script setup lang="ts">
import {computed} from 'vue'
import SettingCheckbox from './SettingCheckbox.vue'
import SettingToggle from './SettingToggle.vue'
import type {UserSettings} from '@/api/types'
import {useData} from '@/stores/data'
import {DEMO} from '@/lib/localMode'
import {t} from '@/lib/i18n'

// Stored on the server, which applies them when someone invites the user
// and when tasks change in shared lists.
const data = useData()
const settings = computed(() => data.user?.settings ?? {})

type Flag = 'notify_shared_task_done' | 'notify_shared_task_created' | 'notify_shared_task_deleted'

function flag(key: Flag) {
	return computed({
		get: () => !!settings.value[key],
		set: (value: boolean) => data.updateUserSettings({[key]: value} as Partial<UserSettings>),
	})
}

const autoAccept = computed({
	get: () => !settings.value.confirm_project_invites,
	set: (value: boolean) => data.updateUserSettings({confirm_project_invites: !value}),
})
const notifyComplete = flag('notify_shared_task_done')
const notifyAdd = flag('notify_shared_task_created')
const notifyDelete = flag('notify_shared_task_deleted')
</script>

<template>
	<h2 class="s-title">{{ t('collaborate') }}</h2>

	<div v-if="data.local" class="s-card">
		<div class="s-desc">{{ DEMO ? t('collabDemo') : t('collabNeedsAccount') }}</div>
	</div>

	<template v-else>
		<div class="s-card">
			<div class="s-row">
				<span>{{ t('autoAcceptInvites') }}</span>
				<SettingToggle v-model="autoAccept" />
			</div>
			<div class="s-desc">{{ t('autoAcceptInvitesDesc') }}</div>
		</div>

		<div class="s-card">
			<div class="s-row">
				<span>{{ t('sharedListNotifications') }}</span>
			</div>
			<div class="s-desc">{{ t('sharedListNotificationsDesc') }}</div>
			<label class="s-check">
				<SettingCheckbox v-model="notifyComplete" />
				{{ t('notifyComplete') }}
			</label>
			<label class="s-check">
				<SettingCheckbox v-model="notifyAdd" />
				{{ t('notifyAdd') }}
			</label>
			<label class="s-check">
				<SettingCheckbox v-model="notifyDelete" />
				{{ t('notifyDelete') }}
			</label>
		</div>
	</template>
</template>
