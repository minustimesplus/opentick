<script setup lang="ts">
import {type Component} from 'vue'
import {Apple, CircleQuestionMark, Ellipsis, ExternalLink, Instagram, MessageCircle, Smartphone, Twitter, Youtube} from 'lucide-vue-next'
import {t, type MessageKey} from '@/lib/i18n'

const version = __APP_VERSION__
const commit = __APP_COMMIT__

// Release versions have an anchor on the changelog page; dev builds land on the top.
const changelogUrl = 'https://minustimesplus.github.io/opentick/'
	+ (/^\d+\.\d+\.\d+$/.test(version) ? `#v${version}` : '')

const community: {icon: Component, label?: MessageKey, name?: string}[] = [
	{icon: CircleQuestionMark, label: 'helpCenter'},
	{icon: Twitter, name: 'X(Twitter)'},
	{icon: MessageCircle, name: 'Reddit'},
	{icon: Youtube, name: 'YouTube'},
	{icon: Instagram, name: 'Instagram'},
]

const apps: {icon: Component, label?: MessageKey, name?: string}[] = [
	{icon: Smartphone, name: 'Android'},
	{icon: Apple, name: 'iOS'},
	{icon: Ellipsis, label: 'others'},
]
</script>

<template>
	<div class="about">
		<h2>{{ t('aboutWelcome') }}</h2>
		<p>
			{{ t('aboutHelpBefore') }}
			<button class="s-link soon" disabled :title="t('comingSoon')">{{ t('helpCenter') }}</button>
			{{ t('aboutHelpAfter') }}
			<br>
			{{ t('aboutContactBefore') }}
			<button class="s-link soon" disabled :title="t('comingSoon')">{{ t('feedbackAndSuggestions') }}</button>
			{{ t('aboutContactAfter') }}
		</p>
		<div class="tiles soon">
			<button v-for="(item, i) in community" :key="i" class="tile" disabled :title="t('comingSoon')">
				<component :is="item.icon" :size="20" :stroke-width="1.7" />
				<span>{{ item.label ? t(item.label) : item.name }}</span>
			</button>
		</div>

		<h2>{{ t('getApp') }} <span class="s-soon">{{ t('comingSoon') }}</span></h2>
		<p>{{ t('getAppDesc') }}</p>
		<div class="tiles soon">
			<button v-for="(item, i) in apps" :key="i" class="tile" disabled :title="t('comingSoon')">
				<component :is="item.icon" :size="20" :stroke-width="1.7" />
				<span>{{ item.label ? t(item.label) : item.name }}</span>
			</button>
		</div>

		<h2>{{ t('version') }}</h2>
		<p class="version">
			OpenTick {{ version || t('devBuild') }}
			<span v-if="commit && !version.endsWith(commit)" class="commit">{{ commit }}</span>
		</p>
		<a class="s-link changelog" :href="changelogUrl" target="_blank" rel="noopener noreferrer">
			{{ t('changelog') }}
			<ExternalLink :size="12" />
		</a>
	</div>
</template>

<style scoped>
.about {
	padding: 0 2px;
}

h2 {
	margin: 0 0 16px;
	font-size: 18px;
	font-weight: 400;
}

h2:not(:first-child) {
	margin-top: 30px;
}

p {
	margin: 0 0 16px;
	font-size: 13px;
	line-height: 1.6;
	color: var(--text-2);
}

p .s-link {
	display: inline;
}

.tiles {
	display: flex;
	flex-wrap: wrap;
	gap: 7px;
}

.tile {
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 7px;
	width: 68px;
	height: 68px;
	border-radius: 8px;
	border: 1px solid var(--border);
	color: var(--text-4);
	font-size: 10px;
}

.version {
	margin-bottom: 8px;
	color: var(--text);
}

.commit {
	margin-left: 6px;
	font-family: ui-monospace, monospace;
	font-size: 12px;
	color: var(--text-4);
}

.changelog {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	text-decoration: none;
}

.tile span {
	color: var(--text-3);
}

.tile:hover:not(:disabled) {
	background: var(--bg-hover);
}
</style>
