<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type Component} from 'vue'
import {
	Bell, CircleUserRound, Clock, Crown, Funnel, Import, Info, Keyboard, LayoutGrid, PaintRoller, Server, Sparkles, Users, X,
} from 'lucide-vue-next'
import AboutSettings from './AboutSettings.vue'
import AiFeaturesSettings from './AiFeaturesSettings.vue'
import AccountSettings from './AccountSettings.vue'
import AppearanceSettings from './AppearanceSettings.vue'
import CollaborateSettings from './CollaborateSettings.vue'
import DateTimeSettings from './DateTimeSettings.vue'
import FeaturesSettings from './FeaturesSettings.vue'
import IntegrationSettings from './IntegrationSettings.vue'
import MoreSettings from './MoreSettings.vue'
import NotificationSettings from './NotificationSettings.vue'
import PremiumSettings from './PremiumSettings.vue'
import ShortcutSettings from './ShortcutSettings.vue'
import SmartListSettings from './SmartListSettings.vue'
import './settings.css'
import {t, type MessageKey} from '@/lib/i18n'
import {ui, type SettingsSection} from '@/lib/ui'

interface NavItem {
	key: SettingsSection
	icon: Component
	label: MessageKey
	/** Nothing in the section works yet. */
	soon?: boolean
}

const groups: NavItem[][] = [
	[
		{key: 'account', icon: CircleUserRound, label: 'account'},
		{key: 'premium', icon: Crown, label: 'premium'},
	],
	[
		{key: 'features', icon: LayoutGrid, label: 'features'},
		{key: 'smartList', icon: Funnel, label: 'smartList', soon: true},
		{key: 'notifications', icon: Bell, label: 'notifications', soon: true},
		{key: 'dateTime', icon: Clock, label: 'dateTime'},
		{key: 'appearance', icon: PaintRoller, label: 'appearance'},
		{key: 'aiFeatures', icon: Sparkles, label: 'aiFeatures'},
		{key: 'more', icon: Server, label: 'moreSettings'},
	],
	[
		{key: 'integrations', icon: Import, label: 'integrations', soon: true},
		{key: 'collaborate', icon: Users, label: 'collaborate', soon: true},
		{key: 'shortcuts', icon: Keyboard, label: 'shortcutsShort'},
	],
	[
		{key: 'about', icon: Info, label: 'about'},
	],
]

const panels: Partial<Record<SettingsSection, Component>> = {
	account: AccountSettings,
	premium: PremiumSettings,
	features: FeaturesSettings,
	smartList: SmartListSettings,
	notifications: NotificationSettings,
	dateTime: DateTimeSettings,
	appearance: AppearanceSettings,
	aiFeatures: AiFeaturesSettings,
	more: MoreSettings,
	integrations: IntegrationSettings,
	collaborate: CollaborateSettings,
	shortcuts: ShortcutSettings,
	about: AboutSettings,
}

const panel = computed(() => (ui.settings ? panels[ui.settings] : undefined))

function close() {
	ui.settings = null
}

// On phones the nav is a horizontal strip; keep the open section in view.
const navEl = ref<HTMLElement | null>(null)
watch(() => ui.settings, () => nextTick(() => {
	navEl.value?.querySelector('.nav-item.active')?.scrollIntoView({block: 'nearest', inline: 'nearest'})
}), {immediate: true})

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
				<aside ref="navEl" class="nav">
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
							<span v-if="item.soon" class="s-soon" :title="t('comingSoon')">{{ t('soon') }}</span>
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

.nav-item > span:not(.s-soon) {
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
}

.nav-item .s-soon {
	margin-left: auto;
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

/* Phones: full screen, with the sections as a scrolling strip on top. */
@media (max-width: 767px) {
	.settings {
		flex-direction: column;
		width: 100%;
		height: 100%;
		padding: env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left);
		border-radius: 0;
	}

	.nav {
		flex-direction: row;
		align-items: center;
		gap: 4px;
		width: auto;
		padding: 10px 52px 10px 10px;
		overflow-x: auto;
		overflow-y: hidden;
		border-right: 0;
		border-bottom: 1px solid var(--border);
		scrollbar-width: none;
	}

	.nav-title {
		display: none;
	}

	.nav-group {
		flex-direction: row;
		gap: 4px;
	}

	.nav-group + .nav-group {
		margin-top: 0;
		padding-left: 4px;
		border-left: 1px solid var(--border);
	}

	.nav-item {
		height: 36px;
	}

	.content {
		position: static;
		min-height: 0;
	}

	.close {
		top: calc(env(safe-area-inset-top) + 13px);
		right: calc(env(safe-area-inset-right) + 10px);
		width: 32px;
		height: 32px;
		background: var(--bg-panel);
	}

	.panel {
		padding: 20px 16px calc(24px + env(safe-area-inset-bottom));
	}
}
</style>
