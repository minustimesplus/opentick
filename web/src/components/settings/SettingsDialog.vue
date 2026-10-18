<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, type Component} from 'vue'
import {
	Bell, CircleUserRound, Clock, Crown, Funnel, Import, Info, Keyboard, LayoutGrid, PaintRoller, Server, Sparkles, Users, X,
} from 'lucide-vue-next'
import AiFeaturesSettings from './AiFeaturesSettings.vue'
import DateTimeSettings from './DateTimeSettings.vue'
import {t, type MessageKey} from '@/lib/i18n'
import {ui, type SettingsSection} from '@/lib/ui'

interface NavItem {
	key: SettingsSection
	icon: Component
	label: MessageKey
}

const groups: NavItem[][] = [
	[
		{key: 'account', icon: CircleUserRound, label: 'account'},
		{key: 'premium', icon: Crown, label: 'premium'},
	],
	[
		{key: 'features', icon: LayoutGrid, label: 'features'},
		{key: 'smartList', icon: Funnel, label: 'smartList'},
		{key: 'notifications', icon: Bell, label: 'notifications'},
		{key: 'dateTime', icon: Clock, label: 'dateTime'},
		{key: 'appearance', icon: PaintRoller, label: 'appearance'},
		{key: 'aiFeatures', icon: Sparkles, label: 'aiFeatures'},
		{key: 'more', icon: Server, label: 'moreSettings'},
	],
	[
		{key: 'integrations', icon: Import, label: 'integrations'},
		{key: 'collaborate', icon: Users, label: 'collaborate'},
		{key: 'shortcuts', icon: Keyboard, label: 'shortcutsShort'},
	],
	[
		{key: 'about', icon: Info, label: 'about'},
	],
]

// Sections without a panel yet render an empty page.
const panels: Partial<Record<SettingsSection, Component>> = {
	dateTime: DateTimeSettings,
	aiFeatures: AiFeaturesSettings,
}

const panel = computed(() => (ui.settings ? panels[ui.settings] : undefined))

function close() {
	ui.settings = null
}

function onKey(e: KeyboardEvent) {
	if (e.key === 'Escape') close()
}

onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
	<Teleport to="body">
		<div class="overlay" @mousedown.self="close">
			<div class="settings" role="dialog" aria-modal="true" :aria-label="t('settings')">
				<aside class="nav">
					<div class="nav-title">{{ t('settings') }}</div>
					<div v-for="(group, i) in groups" :key="i" class="nav-group">
						<button
							v-for="item in group"
							:key="item.key"
							class="nav-item"
							:class="{active: ui.settings === item.key}"
							@click="ui.settings = item.key"
						>
							<component :is="item.icon" :size="16" :stroke-width="1.7" />
							<span>{{ t(item.label) }}</span>
						</button>
					</div>
				</aside>
				<section class="content">
					<button class="close" :title="t('close')" :aria-label="t('close')" @click="close">
						<X :size="16" :stroke-width="1.8" />
					</button>
					<div class="panel">
						<component :is="panel" v-if="panel" />
					</div>
				</section>
			</div>
		</div>
	</Teleport>
</template>

<style scoped>
.overlay {
	position: fixed;
	inset: 0;
	z-index: 900;
	display: flex;
	align-items: center;
	justify-content: center;
	background: rgba(0, 0, 0, 0.5);
}

.settings {
	--settings-card: var(--bg-hover);
	display: flex;
	width: min(728px, calc(100vw - 32px));
	height: min(620px, calc(100vh - 32px));
	border-radius: 12px;
	overflow: hidden;
	background: var(--bg);
	box-shadow: var(--shadow);
}

.nav {
	display: flex;
	flex-direction: column;
	width: 194px;
	flex-shrink: 0;
	padding: 14px 8px;
	overflow-y: auto;
	background: var(--bg-panel);
	border-right: 1px solid var(--border);
}

.nav-title {
	padding: 0 8px;
	margin-bottom: 12px;
	font-size: 15px;
	font-weight: 600;
}

.nav-group {
	display: flex;
	flex-direction: column;
	gap: 1px;
}

.nav-group + .nav-group {
	margin-top: 14px;
}

.nav-item {
	display: flex;
	align-items: center;
	gap: 9px;
	height: 34px;
	padding: 0 10px;
	border-radius: 6px;
	font-size: 13px;
	text-align: left;
	white-space: nowrap;
}

.nav-item svg {
	flex-shrink: 0;
	color: var(--text-2);
}

.nav-item:hover {
	background: var(--bg-hover);
}

.nav-item.active {
	background: var(--bg-active);
}

.content {
	position: relative;
	flex: 1;
	min-width: 0;
}

.close {
	position: absolute;
	top: 14px;
	right: 14px;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 24px;
	height: 24px;
	border-radius: 5px;
	color: var(--text-3);
}

.close:hover {
	background: var(--bg-hover);
	color: var(--text);
}

.panel {
	height: 100%;
	padding: 48px 20px 20px;
	overflow-y: auto;
}

@media (max-width: 600px) {
	.nav {
		width: 150px;
	}
}
</style>
