<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, watch, watchEffect} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import NavRail from './components/NavRail.vue'
import LoginView from './views/LoginView.vue'
import TaskDrawer from './components/TaskDrawer.vue'
import Toasts from './components/Toasts.vue'
import WindowControls from './components/WindowControls.vue'
import TaskContextMenu from './components/TaskContextMenu.vue'
import SettingsDialog from './components/settings/SettingsDialog.vue'
import {useData} from './stores/data'
import {prefs} from './lib/prefs'
import {locale} from './lib/i18n'
import {ui} from './lib/ui'
import {ownWindowControls} from './lib/desktop'
import {applyTheme} from './lib/theme'
import {background, clearBackground, loadBackground} from './lib/background'

const data = useData()
const router = useRouter()
const route = useRoute()

applyTheme()
watchEffect(() => {
	document.documentElement.lang = locale.value
})

const showApp = computed(() => data.authed)

watch(() => data.authed, authed => {
	if (authed) loadBackground()
	else clearBackground()
}, {immediate: true})

// Like TickTick, the background shows behind the task list and the matrix,
// but not the calendar, whose grid needs a calm surface.
const showBackground = computed(() =>
	!!background.url && (route.name === 'tasks' || route.name === 'matrix'))

let timer: number | undefined

function refresh() {
	if (data.authed && !data.loading) data.loadAll().catch(() => {})
}

function onKey(e: KeyboardEvent) {
	const target = e.target as HTMLElement
	if (target.closest('input, textarea, [contenteditable="true"]')) return
	if (e.metaKey || e.ctrlKey || e.altKey) return
	if (ui.settings) return
	if (e.key === '/' ) {
		e.preventDefault()
		router.push('/search')
	} else if (e.key === 'n' || e.key === 'N') {
		e.preventDefault()
		ui.focusQuickAdd++
	} else if (e.key === 'g') {
		const next = (ev: KeyboardEvent) => {
			window.removeEventListener('keydown', next, true)
			const map: Record<string, string> = {t: '/tasks/today', w: '/tasks/week', i: '/tasks/inbox', m: '/matrix', c: '/calendar', h: '/habits', f: '/focus', d: '/countdowns'}
			if (map[ev.key]) {
				ev.preventDefault()
				router.push(map[ev.key])
			}
		}
		window.addEventListener('keydown', next, true)
	}
}

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
	<div v-if="showApp" class="shell">
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
	</div>
	<LoginView v-else />
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
