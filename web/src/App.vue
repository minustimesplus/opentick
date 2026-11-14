<script setup lang="ts">
import {computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref, watch, watchEffect} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import NavRail from './components/NavRail.vue'
import LoginView from './views/LoginView.vue'
import OAuthConsentView from './views/OAuthConsentView.vue'
import TaskDrawer from './components/TaskDrawer.vue'
import Toasts from './components/Toasts.vue'
import WindowControls from './components/WindowControls.vue'
import TaskContextMenu from './components/TaskContextMenu.vue'
import SettingsDialog from './components/settings/SettingsDialog.vue'
import AgentSendDialog from './components/AgentSendDialog.vue'
import {useAgents} from './stores/agents'
import {useCalendars} from './stores/calendars'
import InviteDialog from './components/collab/InviteDialog.vue'
import InviteBanner from './components/collab/InviteBanner.vue'
import {pendingInviteToken} from './lib/inviteLink'
import {useData} from './stores/data'
import {startViewSync} from './stores/viewSync'
import {prefs} from './lib/prefs'
import {locale} from './lib/i18n'
import {ui} from './lib/ui'
import {featureOn, routeAllowed} from './lib/features'
import {ownWindowControls} from './lib/desktop'
import {applyTheme} from './lib/theme'
import {background, clearBackground, loadBackground} from './lib/background'
import {onboardingPending} from './lib/onboarding'
import {onReconnect, remapText} from './api/offline'

const OnboardingFlow = defineAsyncComponent(() => import('./components/onboarding/OnboardingFlow.vue'))

const data = useData()
const router = useRouter()
const route = useRoute()

applyTheme()
watchEffect(() => {
	document.documentElement.lang = locale.value
})

const agents = useAgents()
startViewSync()
const showApp = computed(() => data.authed)

// An opened invitation link waits here until the user is signed in.
const inviteToken = ref(pendingInviteToken())
watch(() => ui.inviteLink, () => {
	inviteToken.value = pendingInviteToken()
})
const showConsent = computed(() => data.authed && route.name === 'oauth-authorize')

watch(() => data.authed, authed => {
	if (authed) loadBackground()
	else clearBackground()
}, {immediate: true})

watch(() => data.authed, authed => {
	if (!authed) agents.clear()
	// A fresh sign in; startup itself is covered by refresh() on mount.
	else preloadCalendar()
})

// The sign up and sign in pages lead into the app once signed in.
watch(() => data.authed && (route.name === 'signup' || route.name === 'login'), leave => {
	if (leave) router.replace('/tasks/today')
}, {immediate: true})

// The setup wizard after signing up, once the new account has loaded.
const showOnboarding = ref(false)
watch(() => data.authed && data.loaded, ready => {
	if (ready && onboardingPending()) showOnboarding.value = true
	if (!data.authed) showOnboarding.value = false
}, {immediate: true})

// Leave a view as soon as its feature is turned off.
watch(() => routeAllowed(route), allowed => {
	if (!allowed) router.replace('/tasks/today')
})

// Like TickTick, the background shows behind the task list and the matrix,
// but not the calendar, whose grid needs a calm surface.
const showBackground = computed(() =>
	!!background.url && (route.name === 'tasks' || route.name === 'matrix'))

let timer: number | undefined

const calendars = useCalendars()
let calendarViewWarmed = false

/**
 * Loads the calendar's code and the subscribed calendars' events for the
 * first two weeks in the background, so opening the calendar after startup
 * (or after the laptop wakes) shows everything at once.
 */
function preloadCalendar() {
	if (!featureOn('calendar')) return
	if (!calendarViewWarmed) {
		calendarViewWarmed = true
		import('./views/CalendarView.vue').catch(() => { calendarViewWarmed = false })
	}
	calendars.preload().catch(() => {})
}

function refresh() {
	if (data.authed && !data.loading) data.loadAll().catch(() => {})
	if (data.authed) {
		agents.load()
		preloadCalendar()
	}
}

watch(() => agents.enabled, on => {
	if (on && data.authed) agents.load()
})

function onKey(e: KeyboardEvent) {
	const target = e.target as HTMLElement
	if (target.closest('input, textarea, [contenteditable="true"]')) return
	if (e.metaKey || e.ctrlKey || e.altKey) return
	if (ui.settings || showConsent.value || showOnboarding.value) return
	if (e.key === '/' ) {
		e.preventDefault()
		router.push('/search')
	} else if (e.key === 'n' || e.key === 'N') {
		e.preventDefault()
		ui.focusQuickAdd++
	} else if (e.key === 'g') {
		const next = (ev: KeyboardEvent) => {
			window.removeEventListener('keydown', next, true)
			const map: Record<string, string> = {
				a: '/tasks/all', t: '/tasks/today', r: '/tasks/tomorrow', w: '/tasks/week', i: '/tasks/inbox', b: '/tasks/summary', g: '/tasks/trash',
				m: '/matrix', c: '/calendar', h: '/habits', f: '/focus', d: '/countdowns', p: '/people',
			}
			if (map[ev.key] && routeAllowed(router.resolve(map[ev.key]))) {
				ev.preventDefault()
				router.push(map[ev.key])
			}
		}
		window.addEventListener('keydown', next, true)
	}
}

// A task or list created offline that is open right now has a server id now.
onReconnect(ids => {
	const path = remapText(route.fullPath, ids)
	if (path !== route.fullPath) router.replace(path)
})

onMounted(() => {
	if (data.authed) refresh()
	timer = window.setInterval(refresh, 60_000)
	window.addEventListener('focus', refresh)
	window.addEventListener('keydown', onKey)
})

onBeforeUnmount(() => {
	clearInterval(timer)
	window.removeEventListener('focus', refresh)
	window.removeEventListener('keydown', onKey)
})
</script>

<template>
	<OAuthConsentView v-if="showConsent" />
	<div v-else-if="showApp" class="shell">
		<NavRail />
		<main class="main" :class="{'has-bg': showBackground}">
			<div
				v-if="showBackground"
				class="main-bg"
				:class="{blur: prefs.backgroundBlur}"
				:style="{backgroundImage: `url(${background.url})`}"
			/>
			<RouterView v-if="data.loaded" />
			<div v-else class="boot">
				<div class="spinner" />
			</div>
		</main>
		<TaskDrawer v-if="ui.drawerTaskId !== null" :key="ui.drawerTaskId" :task-id="ui.drawerTaskId" />
		<TaskContextMenu v-if="ui.ctx" :key="ui.ctx.taskId + ':' + ui.ctx.x + ':' + ui.ctx.y" />
		<SettingsDialog v-if="ui.settings" />
		<AgentSendDialog
			v-if="ui.agentSend"
			:key="ui.agentSend.taskId"
			:task-id="ui.agentSend.taskId"
			:agent="ui.agentSend.agent"
			@close="ui.agentSend = null"
		/>
		<OnboardingFlow v-if="showOnboarding" @close="showOnboarding = false" />
		<InviteDialog v-if="inviteToken && data.loaded && !showOnboarding" :token="inviteToken" @close="inviteToken = pendingInviteToken()" />
	</div>
	<template v-else>
		<InviteBanner v-if="inviteToken" :token="inviteToken" />
		<LoginView />
	</template>
	<WindowControls v-if="ownWindowControls" />
	<Toasts />
</template>

<style scoped>
.shell {
	display: flex;
	height: 100%;
	padding: 6px 6px 6px 0;
}

/* Desktop app: no title bar, so a 24px strip of frame above the panel holds
   the window controls (TickTick's proportions). The nav rail runs up into it. */
:global(html.desktop .shell) {
	padding: 0 4px 4px 0;
}

:global(html.desktop .main) {
	margin-top: 24px;
}

.main {
	flex: 1;
	min-width: 0;
	background: var(--bg);
	border-radius: 10px;
	overflow: hidden;
	position: relative;
	border: 1px solid var(--border-strong);
}

/* Panes turn translucent so the image layer below shows through. The
   variables cascade into every view; teleported popovers keep solid colors. */
.main.has-bg {
	isolation: isolate;
	--bg: var(--glass);
	--bg-panel: var(--glass);
	--bg-input: var(--glass);
	--bg-hover: rgba(128, 128, 128, 0.16);
	--bg-active: color-mix(in srgb, var(--accent) 22%, transparent);
	--border: rgba(128, 128, 128, 0.2);
}

.main-bg {
	position: absolute;
	inset: 0;
	z-index: -1;
	background-position: center;
	background-size: cover;
}

/* A veil keeps text readable on busy photos, even where no pane covers it. */
.main-bg::after {
	content: '';
	position: absolute;
	inset: 0;
	background: var(--glass);
	opacity: 0.5;
}

.main-bg.blur {
	/* Scale up so the blur doesn't fade the edges to transparent. */
	inset: -40px;
	filter: blur(18px);
}

/* Phones: the nav rail becomes a tab bar below the panel (column-reverse, as
   it is the first child), and the panel fills the screen edge to edge. */
@media (max-width: 767px) {
	.shell {
		flex-direction: column-reverse;
		padding: env(safe-area-inset-top) 0 0;
	}

	.main {
		border: 0;
		border-bottom: 1px solid var(--border);
		border-radius: 0;
	}
}

.boot {
	display: flex;
	align-items: center;
	justify-content: center;
	height: 100%;
}

.spinner {
	width: 28px;
	height: 28px;
	border: 3px solid var(--border-strong);
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
