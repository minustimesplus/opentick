<script setup lang="ts">
import {computed, ref} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {Bell, CircleArrowDown, CircleHelp, RefreshCw, Settings, LogOut} from 'lucide-vue-next'
import Popover from './Popover.vue'
import {useData} from '@/stores/data'
import {t} from '@/lib/i18n'
import {CalendarIcon, MatrixIcon, SearchIcon, TasksIcon} from '@/lib/icons'
import {installUpdate, installingUpdate, updateReady} from '@/lib/desktop'
import {fmtDayMonth, fmtTime, parseDate} from '@/lib/dates'

const data = useData()
const route = useRoute()
const router = useRouter()

const avatarEl = ref<HTMLElement | null>(null)
const bellEl = ref<HTMLElement | null>(null)
const helpEl = ref<HTMLElement | null>(null)
const menu = ref<'avatar' | 'bell' | 'help' | null>(null)

const items = computed(() => [
	{key: 'tasks', icon: TasksIcon, to: '/tasks/today', active: route.path.startsWith('/tasks'), title: t('lists')},
	{key: 'matrix', icon: MatrixIcon, to: '/matrix', active: route.path.startsWith('/matrix'), title: t('matrix')},
	{key: 'calendar', icon: CalendarIcon, to: '/calendar', active: route.path.startsWith('/calendar'), title: t('week')},
	{key: 'search', icon: SearchIcon, to: '/search', active: route.path.startsWith('/search'), title: t('search')},
])

const initial = computed(() => (data.user?.name || data.user?.username || '?').charAt(0).toUpperCase())
const unread = computed(() => data.notifications.filter(n => !n.read_at).length)

function notificationText(n: {name: string, notification: Record<string, unknown>}) {
	const task = n.notification.task as {title?: string} | undefined
	const doer = n.notification.doer as {username?: string, name?: string} | undefined
	const who = doer?.name || doer?.username || ''
	const what = n.name.replace(/^task\.|^project\./, '').replace(/[._]/g, ' ')
	return [who, what, task?.title ? `“${task.title}”` : ''].filter(Boolean).join(' · ')
}

function openBell() {
	menu.value = 'bell'
	data.loadNotifications()
}

async function logout() {
	menu.value = null
	await data.logout()
}

const shortcuts = [
	['N', 'addTask'],
	['/', 'search'],
	['G T', 'today'],
	['G W', 'next7'],
	['G I', 'inbox'],
	['G M', 'matrix'],
	['G C', 'week'],
] as const
</script>

<template>
	<nav class="rail">
		<button ref="avatarEl" class="avatar" :title="data.user?.username" @click="menu = 'avatar'">{{ initial }}</button>
		<RouterLink v-for="i in items" :key="i.key" :to="i.to" class="rail-btn" :class="{active: i.active}" :title="i.title">
			<component :is="i.icon" :size="22" />
		</RouterLink>
		<div class="spacer" />
		<button
			v-if="updateReady"
			class="rail-btn update"
			:title="installingUpdate ? t('installingUpdate') : t('updateReady', {version: updateReady})"
			:disabled="installingUpdate"
			@click="installUpdate"
		>
			<CircleArrowDown :size="20" />
		</button>
		<button class="rail-btn" :title="t('sync')" @click="data.loadAll()">
			<RefreshCw :size="20" :class="{spinning: data.loading}" />
		</button>
		<button ref="bellEl" class="rail-btn" :title="t('notifications')" @click="openBell">
			<Bell :size="20" />
			<span v-if="unread" class="dot" />
		</button>
		<button ref="helpEl" class="rail-btn" :title="t('shortcuts')" @click="menu = 'help'">
			<CircleHelp :size="20" />
		</button>

		<Popover v-if="menu === 'avatar'" :anchor="avatarEl" placement="right-start" @close="menu = null">
			<div class="menu">
				<div class="menu-label">{{ data.user?.name || data.user?.username }}</div>
				<button class="menu-item" @click="menu = null; router.push('/settings')"><Settings :size="16" /> {{ t('settings') }}</button>
				<div class="menu-sep" />
				<button class="menu-item danger" @click="logout"><LogOut :size="16" /> {{ t('logout') }}</button>
			</div>
		</Popover>

		<Popover v-if="menu === 'bell'" :anchor="bellEl" placement="right-start" @close="menu = null">
			<div class="menu notif">
				<div class="notif-head">
					<span>{{ t('notifications') }}</span>
					<button v-if="unread" class="link" @click="data.markNotificationsRead()">✓</button>
				</div>
				<div v-if="!data.notifications.length" class="muted pad">{{ t('noNotifications') }}</div>
				<div v-for="n in data.notifications" :key="n.id" class="notif-row" :class="{unread: !n.read_at}">
					<div>{{ notificationText(n) }}</div>
					<div class="muted small">{{ parseDate(n.created) ? fmtDayMonth(parseDate(n.created)!) + ' ' + fmtTime(parseDate(n.created)!) : '' }}</div>
				</div>
			</div>
		</Popover>

		<Popover v-if="menu === 'help'" :anchor="helpEl" placement="right-start" @close="menu = null">
			<div class="menu help">
				<div class="menu-label">{{ t('shortcuts') }}</div>
				<div v-for="[k, label] in shortcuts" :key="k" class="kbd-row">
					<span>{{ t(label) }}</span>
					<kbd>{{ k }}</kbd>
				</div>
				<div class="menu-sep" />
				<div class="muted small pad">{{ t('quickAddHint') }}</div>
			</div>
		</Popover>
	</nav>
</template>

<style scoped>
.rail {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 6px;
	width: 60px;
	flex-shrink: 0;
	padding: 12px 0 10px;
}

.avatar {
	width: 34px;
	height: 34px;
	margin-bottom: 18px;
	border-radius: 50%;
	background: linear-gradient(135deg, #4772fa, #8b5cf6);
	color: #fff;
	font-weight: 600;
	font-size: 15px;
}

.rail-btn {
	position: relative;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 40px;
	height: 40px;
	border-radius: 8px;
	color: var(--text-3);
	margin-bottom: 12px;
}

.rail-btn:hover {
	color: var(--text);
}

.rail-btn.active {
	color: var(--text);
}

.rail-btn.update {
	color: var(--accent);
}

.rail-btn.update:disabled {
	opacity: 0.5;
}

.spacer {
	flex: 1;
}

.dot {
	position: absolute;
	top: 8px;
	right: 9px;
	width: 7px;
	height: 7px;
	border-radius: 50%;
	background: var(--danger);
}

.spinning {
	animation: spin 0.9s linear infinite;
}

@keyframes spin {
	to {
		transform: rotate(360deg);
	}
}

.notif {
	width: 320px;
	max-height: 420px;
	overflow: auto;
}

.notif-head {
	display: flex;
	justify-content: space-between;
	padding: 6px 10px;
	font-weight: 600;
}

.notif-row {
	padding: 8px 10px;
	border-radius: 6px;
	font-size: 13px;
}

.notif-row.unread {
	background: var(--accent-soft);
}

.small {
	font-size: 12px;
}

.pad {
	padding: 8px 10px;
}

.help {
	width: 260px;
}

.kbd-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 5px 10px;
}

kbd {
	font-family: inherit;
	font-size: 12px;
	padding: 1px 6px;
	border-radius: 4px;
	border: 1px solid var(--border-strong);
	color: var(--text-2);
}

.link {
	color: var(--accent);
}
</style>
