<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, watchEffect} from 'vue'
import {useRouter} from 'vue-router'
import NavRail from './components/NavRail.vue'
import LoginView from './views/LoginView.vue'
import TaskDrawer from './components/TaskDrawer.vue'
import Toasts from './components/Toasts.vue'
import TaskContextMenu from './components/TaskContextMenu.vue'
import SettingsDialog from './components/settings/SettingsDialog.vue'
import {useData} from './stores/data'
import {prefs} from './lib/prefs'
import {locale} from './lib/i18n'
import {ui} from './lib/ui'
import {desktop} from './lib/desktop'

const data = useData()
const router = useRouter()

const media = window.matchMedia('(prefers-color-scheme: dark)')
watchEffect(() => {
	const theme = prefs.theme === 'system' ? (media.matches ? 'dark' : 'light') : prefs.theme
	document.documentElement.dataset.theme = theme
	desktop?.setTheme?.(theme)
	document.documentElement.lang = locale.value
})

const showApp = computed(() => data.authed)

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
			const map: Record<string, string> = {t: '/tasks/today', w: '/tasks/week', i: '/tasks/inbox', m: '/matrix', c: '/calendar'}
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
		<main class="main">
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
	<Toasts />
</template>

<style scoped>
.shell {
	display: flex;
	height: 100%;
	padding: 6px 6px 6px 0;
}

/* Room for the window controls the desktop app draws over the page. */
:global(html.desktop) .shell {
	padding-top: 28px;
}

.main {
	flex: 1;
	min-width: 0;
	background: var(--bg);
	border-radius: 10px;
	overflow: hidden;
	position: relative;
	border: 1px solid var(--border);
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
