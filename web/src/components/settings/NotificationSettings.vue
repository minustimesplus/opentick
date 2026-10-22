<script setup lang="ts">
import {computed, reactive} from 'vue'
import {Crown} from 'lucide-vue-next'
import SettingSelect from './SettingSelect.vue'
import SettingToggle from './SettingToggle.vue'
import {useData} from '@/stores/data'
import {t} from '@/lib/i18n'

const data = useData()

// Not wired up yet: nothing sends these notifications.
const state = reactive({
	daily: true,
	alertTime: '08:00',
	web: true,
	constant: false,
	email: false,
})

const times = computed(() => [
	{value: '', label: t('never')},
	...Array.from({length: 24}, (_, h) => {
		const value = `${String(h).padStart(2, '0')}:00`
		return {value, label: value}
	}),
])
</script>

<template>
	<h2 class="s-title">{{ t('notifications') }}</h2>

	<div class="s-card">
		<div class="s-row">
			<span>{{ t('dailyNotifications') }}</span>
			<SettingToggle v-model="state.daily" />
		</div>
		<div class="s-desc">{{ t('dailyNotificationsDesc') }}</div>
		<div class="s-row">
			<span>{{ t('dailyAlertTime') }}</span>
			<SettingSelect v-model="state.alertTime" :options="times" />
		</div>
	</div>

	<div class="s-card">
		<div class="s-row">
			<span>{{ t('webReminder') }}</span>
			<SettingToggle v-model="state.web" />
		</div>
	</div>

	<div class="s-card">
		<div class="s-row">
			<span class="s-label">{{ t('constantReminder') }} <Crown :size="13" class="crown" /></span>
			<SettingToggle v-model="state.constant" />
		</div>
		<div class="s-desc">{{ t('constantReminderDesc') }}</div>
	</div>

	<div class="s-card">
		<div class="s-row">
			<span>{{ t('emailNotifications') }}</span>
			<SettingToggle v-model="state.email" />
		</div>
		<div class="s-desc">{{ t('emailNotificationsDesc') }}</div>
		<div class="s-row">
			<span>{{ t('email') }}</span>
			<span class="s-value">{{ data.user?.email }}</span>
		</div>
	</div>
</template>

<style scoped>
.s-label {
	gap: 5px;
}

.s-label > .crown {
	color: #f5a623;
}
</style>
