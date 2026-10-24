<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useRoute} from 'vue-router'
import {CircleArrowDown, RefreshCw, Settings, LogOut} from 'lucide-vue-next'
import Popover from './Popover.vue'
import UserAvatar from './UserAvatar.vue'
import {useData} from '@/stores/data'
import {useFocus} from '@/stores/focus'
import {t} from '@/lib/i18n'
import {BellIcon, CalendarIcon, CountdownIcon, FocusIcon, HabitIcon, HelpIcon, MatrixIcon, SearchIcon, StatsIcon, TasksIcon} from '@/lib/icons'
import {installUpdate, installingUpdate, updateReady} from '@/lib/desktop'
import {fmtDayMonth, fmtTime, parseDate} from '@/lib/dates'
import {openSettings} from '@/lib/ui'
import {featureOn} from '@/lib/features'
import type {Feature} from '@/lib/prefs'
import {isMobile} from '@/lib/viewport'

const data = useData()
const focus = useFocus()
const route = useRoute()

const avatarEl = ref<HTMLElement | null>(null)
const bellEl = ref<HTMLElement | null>(null)
const helpEl = ref<HTMLElement | null>(null)
const menu = ref<'avatar' | 'bell' | 'help' | null>(null)

const items = computed(() => [
	{key: 'tasks', icon: TasksIcon, to: '/tasks/today', active: route.path.startsWith('/tasks'), title: t('lists')},
	{key: 'matrix', feature: 'matrix', icon: MatrixIcon, to: '/matrix', active: route.path.startsWith('/matrix'), title: t('matrix')},
	{key: 'calendar', feature: 'calendar', icon: CalendarIcon, to: '/calendar', active: route.path.startsWith('/calendar'), title: t('week')},
	{key: 'statistics', icon: StatsIcon, to: '/statistics', active: route.path.startsWith('/statistics'), title: t('statistics')},
	{key: 'habits', feature: 'habit', icon: HabitIcon, to: '/habits', active: route.path.startsWith('/habits'), title: t('habits')},
	{key: 'focus', feature: 'pomo', icon: FocusIcon, to: '/focus', active: route.path.startsWith('/focus'), title: t('focus')},
	{key: 'countdowns', feature: 'countdown', icon: CountdownIcon, to: '/countdowns', active: route.path.startsWith('/countdowns'), title: t('countdowns')},
	{key: 'search', icon: SearchIcon, to: '/search', active: route.path.startsWith('/search'), title: t('search')},
].filter(i => !i.feature || featureOn(i.feature as Feature)))

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

// Sync icon: a click spins it at least MIN_SYNC_TURNS full turns, like
// TickTick; it keeps turning while a sync runs and always stops upright.
const MIN_SYNC_TURNS = 2
const syncSpinning = ref(false)
let syncTurns = 0
let syncMinTurns = 0

function startSpin(minTurns: number) {
	syncMinTurns = Math.max(syncMinTurns, minTurns)
	if (syncSpinning.value) return
	syncTurns = 0
	syncSpinning.value = true
}

function onSpinTurn() {
	syncTurns++
	if (syncTurns >= syncMinTurns && !data.loading) {
		syncSpinning.value = false
		syncMinTurns = 0
	}
}

watch(() => data.loading, loading => {
	if (loading) startSpin(1)
})

function sync() {
	startSpin(MIN_SYNC_TURNS)
	data.loadAll().catch(() => {})
}

// On phones the rail is a tab bar at the bottom, so menus open upwards.
const placement = computed(() => isMobile.value ? 'top-start' : 'right-start')

const allShortcuts: [string, 'addTask' | 'search' | 'today' | 'next7' | 'inbox' | 'matrix' | 'week' | 'habits' | 'focus' | 'countdowns', Feature?][] = [
	['N', 'addTask'],
	['/', 'search'],
	['G T', 'today'],
	['G W', 'next7'],
	['G I', 'inbox'],
	['G M', 'matrix', 'matrix'],
	['G C', 'week', 'calendar'],
	['G H', 'habits', 'habit'],
	['G F', 'focus', 'pomo'],
	['G D', 'countdowns', 'countdown'],
]
const shortcuts = computed(() => allShortcuts.filter(([, , feature]) => !feature || featureOn(feature)))
</script>

<template>
	<nav class="rail">
		<button ref="avatarEl" class="avatar" :title="data.user?.username" @click="menu = 'avatar'"><UserAvatar :size="30" /></button>
		<RouterLink v-for="i in items" :key="i.key" :to="i.to" class="rail-btn" :class="{active: i.active}" :title="i.title">
			<component :is="i.icon" :size="24" />
			<span v-if="i.key === 'focus' && focus.state.running" class="dot running" :class="{break: focus.state.phase === 'break'}" />
		</RouterLink>
		<div class="spacer" />
		<button
			v-if="updateReady"
			class="rail-btn update"
			:title="installingUpdate ? t('installingUpdate') : t('updateReady', {version: updateReady})"
			:disabled="installingUpdate"
			@click="installUpdate"
		>
			<CircleArrowDown :size="22" :stroke-width="2.2" />
		</button>
		<button class="rail-btn" :title="t('sync')" @click="sync">
			<RefreshCw :size="22" :stroke-width="2.2" :class="{spinning: syncSpinning}" @animationiteration="onSpinTurn" />
		</button>
		<button ref="bellEl" class="rail-btn" :title="t('notifications')" @click="openBell">
			<BellIcon :size="22" />
			<span v-if="unread" class="dot" />
		</button>
		<button ref="helpEl" class="rail-btn help-btn" :title="t('shortcuts')" @click="menu = 'help'">
			<HelpIcon :size="22" />
		</button>

		<Popover v-if="menu === 'avatar'" :anchor="avatarEl" :placement="placement" @close="menu = null">
			<div class="menu">
				<div class="menu-label">{{ data.user?.name || data.user?.username }}</div>
				<button class="menu-item" @click="menu = null; openSettings()"><Settings :size="16" /> {{ t('settings') }}</button>
				<div class="menu-sep" />
				<button class="menu-item danger" @click="logout"><LogOut :size="16" /> {{ t('logout') }}</button>
			</div>
		</Popover>

		<Popover v-if="menu === 'bell'" :anchor="bellEl" :placement="placement" @close="menu = null">
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

		<Popover v-if="menu === 'help'" :anchor="helpEl" :placement="placement" @close="menu = null">
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
	gap: 10px;
	width: 48px;
	flex-shrink: 0;
	padding: 12px 0 10px;
}

/* Desktop: the rail runs up into the title strip, so the avatar sits level
   with the window controls like TickTick's. */
:global(html.desktop .rail) {
	width: 46px;
	padding-top: 21px;
}

.avatar {
	display: flex;
	margin-bottom: 5px;
	border-radius: 50%;
}

.rail-btn {
	position: relative;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 36px;
	height: 36px;
	border-radius: 8px;
	color: var(--rail-icon);
}

.rail-btn:hover {
	color: var(--rail-active);
}

.rail-btn.active {
	color: var(--rail-active);
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
	top: 7px;
	right: 7px;
	width: 7px;
	height: 7px;
	border-radius: 50%;
	background: var(--danger);
}

.dot.running {
	background: var(--accent);
}

.dot.running.break {
	background: #2fb36f;
}

.spinning {
	animation: spin 0.6s linear infinite;
}

@keyframes spin {
	to {
		transform: rotate(360deg);
	}
}

.notif {
	width: min(320px, calc(100vw - 16px));
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

/* Phones: a tab bar under the main panel. The avatar moves to the end, and
   keyboard shortcuts have no use on a touch screen. */
@media (max-width: 767px) {
	.rail,
	:global(html.desktop .rail) {
		flex-direction: row;
		justify-content: space-around;
		gap: 0;
		width: auto;
		height: calc(54px + env(safe-area-inset-bottom));
		padding: 0 env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left);
	}

	.rail-btn {
		width: 44px;
		height: 44px;
	}

	.dot {
		top: 9px;
		right: 9px;
	}

	.avatar {
		order: 1;
		margin: 0;
		padding: 7px;
	}

	.spacer,
	.help-btn {
		display: none;
	}
}
</style>
