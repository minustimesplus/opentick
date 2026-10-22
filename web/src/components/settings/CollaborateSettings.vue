<script setup lang="ts">
import {reactive} from 'vue'
import SettingCheckbox from './SettingCheckbox.vue'
import SettingToggle from './SettingToggle.vue'
import {t} from '@/lib/i18n'

// Not wired up yet: sharing ignores these.
const state = reactive({
	autoAccept: true,
	notifyComplete: false,
	notifyAdd: false,
	notifyDelete: false,
})
</script>

<template>
	<h2 class="s-title">{{ t('collaborate') }}</h2>

	<div class="s-card">
		<div class="s-row">
			<span>{{ t('autoAcceptInvites') }}</span>
			<SettingToggle v-model="state.autoAccept" />
		</div>
		<div class="s-desc">{{ t('autoAcceptInvitesDesc') }}</div>
	</div>

	<div class="s-card">
		<div class="s-row">
			<span>{{ t('sharedListNotifications') }}</span>
		</div>
		<div class="s-desc">{{ t('sharedListNotificationsDesc') }}</div>
		<label class="s-check">
			<SettingCheckbox v-model="state.notifyComplete" />
			{{ t('notifyComplete') }}
		</label>
		<label class="s-check">
			<SettingCheckbox v-model="state.notifyAdd" />
			{{ t('notifyAdd') }}
		</label>
		<label class="s-check">
			<SettingCheckbox v-model="state.notifyDelete" />
			{{ t('notifyDelete') }}
		</label>
	</div>
</template>
