<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {Crown} from 'lucide-vue-next'
import SettingSelect from './SettingSelect.vue'
import SettingToggle from './SettingToggle.vue'
import {api} from '@/api/client'
import {useData} from '@/stores/data'
import {prefs} from '@/lib/prefs'
import {
	DEFAULT_DAILY_TIME,
	notificationPermission,
	notificationsSupported,
	requestNotificationPermission,
} from '@/lib/reminderNotifier'
import {t} from '@/lib/i18n'

// Daily notification and email are account settings on the server (the
// server mails the daily summary and reminders). Web and constant reminders
// are desktop notifications, so they are per device (prefs).
const data = useData()
const settings = computed(() => data.user?.settings ?? {})

const daily = computed({
	get: () => !!settings.value.overdue_tasks_reminders_enabled,
	set: (value: boolean) => {
		data.updateUserSettings({overdue_tasks_reminders_enabled: value})
		if (value) requestNotificationPermission()
	},
})

/** Stored as H:MM by older clients; shown and saved as HH:MM. */
function normalizeTime(value: string | undefined) {
	const m = /^(\d{1,2}):(\d{2})$/.exec(value ?? '')
	return m ? `${m[1].padStart(2, '0')}:${m[2]}` : DEFAULT_DAILY_TIME
}

const alertTime = computed({
	get: () => normalizeTime(settings.value.overdue_tasks_reminders_time),
	set: (value: string) => data.updateUserSettings({overdue_tasks_reminders_time: value}),
})

const times = computed(() => {
	const list = Array.from({length: 48}, (_, i) => {
		const value = `${String(Math.floor(i / 2)).padStart(2, '0')}:${i % 2 ? '30' : '00'}`
		return {value, label: value}
	})
	// Keep a time set elsewhere (e.g. 07:15) selectable.
	if (!list.some(o => o.value === alertTime.value)) {
		list.push({value: alertTime.value, label: alertTime.value})
		list.sort((a, b) => a.value.localeCompare(b.value))
	}
	return list
})

const webReminder = computed({
	get: () => prefs.webReminder,
	set: (value: boolean) => {
		prefs.webReminder = value
		if (value) requestNotificationPermission()
	},
})

const constantReminder = computed({
	get: () => prefs.constantReminder,
	set: (value: boolean) => {
		prefs.constantReminder = value
	},
})

/** Whether this server sends mail at all; null until /info answered. */
const serverMail = ref<boolean | null>(null)

onMounted(async () => {
	if (data.local) return
	try {
		const info = await api<{email_reminders_enabled?: boolean}>('GET', '/info', {auth: false})
		serverMail.value = !!info.email_reminders_enabled
	} catch {
		serverMail.value = null
	}
})

const email = computed({
	get: () => serverMail.value !== false && !!settings.value.email_reminders_enabled,
	set: (value: boolean) => data.updateUserSettings({email_reminders_enabled: value}),
})

const emailDisabled = computed(() => serverMail.value === false)

function sendTest() {
	if (notificationPermission.value !== 'granted') return
	try {
		new Notification(t('testNotificationTitle'), {body: t('testNotificationBody'), icon: '/favicon.svg'})
	} catch {
		// some platforms only allow notifications from a service worker
	}
}
</script>

<template>
	<h2 class="s-title">{{ t('notifications') }}</h2>

	<div class="s-card">
		<div class="s-row">
			<span>{{ t('dailyNotifications') }}</span>
			<SettingToggle v-model="daily" />
		</div>
		<div class="s-desc">{{ t('dailyNotificationsDesc') }}</div>
		<div class="s-row" :class="{soon: !daily}">
			<span>{{ t('dailyAlertTime') }}</span>
			<SettingSelect v-model="alertTime" :options="times" :disabled="!daily" />
		</div>
	</div>

	<div class="s-card">
		<div class="s-row">
			<span>{{ t('webReminder') }}</span>
			<SettingToggle v-model="webReminder" :disabled="!notificationsSupported" />
		</div>
		<div class="s-desc">{{ t('webReminderDesc') }}</div>
		<div v-if="!notificationsSupported" class="s-desc warn">{{ t('desktopNotificationsUnsupported') }}</div>
		<div v-else-if="notificationPermission === 'denied'" class="s-desc warn">{{ t('desktopNotificationsBlocked') }}</div>
		<div v-else-if="notificationPermission === 'default' && (webReminder || daily)" class="s-row">
			<span />
			<button class="s-link" @click="requestNotificationPermission">{{ t('desktopNotificationsAsk') }}</button>
		</div>
		<div v-else-if="notificationPermission === 'granted' && webReminder" class="s-row">
			<span />
			<button class="s-link" @click="sendTest">{{ t('sendTestNotification') }}</button>
		</div>
	</div>

	<div class="s-card" :class="{soon: !webReminder}">
		<div class="s-row">
			<span class="s-label">{{ t('constantReminder') }} <Crown :size="13" class="crown" /></span>
			<SettingToggle v-model="constantReminder" :disabled="!webReminder" />
		</div>
		<div class="s-desc">{{ t('constantReminderDesc') }}</div>
	</div>

	<div v-if="!data.local" class="s-card">
		<div class="s-row" :class="{soon: emailDisabled}">
			<span>{{ t('emailNotifications') }}</span>
			<SettingToggle v-model="email" :disabled="emailDisabled" />
		</div>
		<div class="s-desc">{{ t('emailNotificationsDesc') }}</div>
		<div v-if="serverMail === false" class="s-desc warn">{{ t('emailNotificationsUnavailable') }}</div>
		<div v-if="data.user?.email" class="s-row">
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

.s-desc.warn {
	color: var(--warning, #e5a50a);
}
</style>
