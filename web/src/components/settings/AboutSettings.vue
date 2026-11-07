<script setup lang="ts">
import {computed, ref, type Component} from 'vue'
import {Apple, CircleQuestionMark, Ellipsis, ExternalLink, Instagram, MessageCircle, RefreshCw, Smartphone, Twitter, Youtube} from 'lucide-vue-next'
import {t, type MessageKey} from '@/lib/i18n'
import {installUpdate, installingUpdate, updateReady} from '@/lib/desktop'
import {checkForUpdates, type UpdateCheck} from '@/lib/updates'

const version = __APP_VERSION__
const commit = __APP_COMMIT__

// Release versions have an anchor on the changelog page; dev builds land on the top.
const changelogUrl = 'https://minustimesplus.github.io/opentick/'
	+ (/^\d+\.\d+\.\d+$/.test(version) ? `#v${version}` : '')

const checking = ref(false)
const check = ref<UpdateCheck | null>(null)

async function runUpdateCheck() {
	if (checking.value) return
	checking.value = true
	try {
		check.value = await checkForUpdates()
	} finally {
		checking.value = false
	}
}

// A download that finishes after the check (or before the page was opened) offers the restart.
const readyVersion = computed(() => updateReady.value
	?? (check.value?.status === 'ready' ? check.value.version : null))

const checkMessage = computed(() => {
	const result = check.value
	if (!result) return ''
	switch (result.status) {
		case 'latest': return t('updateUpToDate')
		case 'downloading': return t('updateDownloading', {version: result.version})
		case 'available': return t('updateAvailableServer', {version: result.version})
		case 'unknown': return t('updateLatestRelease', {version: result.version})
		case 'error': return t('updateCheckFailed', {message: result.message})
		default: return ''
	}
})

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
		<div class="update-row">
			<button v-if="readyVersion" class="btn primary" :disabled="installingUpdate" @click="installUpdate">
				{{ installingUpdate ? t('installingUpdate') : t('updateInstallRestart', {version: readyVersion}) }}
			</button>
			<button v-else class="btn" :disabled="checking" @click="runUpdateCheck">
				<RefreshCw :size="14" :class="{spin: checking}" />
				{{ checking ? t('checkingForUpdates') : t('checkForUpdates') }}
			</button>
			<a class="s-link changelog" :href="changelogUrl" target="_blank" rel="noopener noreferrer">
				{{ t('changelog') }}
				<ExternalLink :size="12" />
			</a>
		</div>
		<p v-if="checkMessage && !readyVersion" class="update-status" :class="{error: check?.status === 'error'}">
			{{ checkMessage }}
			<a
				v-if="check && 'url' in check"
				class="s-link release"
				:href="check.url"
				target="_blank"
				rel="noopener noreferrer"
			>{{ t('updateViewRelease') }}</a>
		</p>
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

.update-row {
	display: flex;
	align-items: center;
	gap: 14px;
	margin-top: 12px;
}

.update-status {
	margin: 10px 0 0;
}

.update-status.error {
	color: var(--danger);
}

.update-status .release {
	margin-left: 4px;
}

.spin {
	animation: spin 1s linear infinite;
}

@keyframes spin {
	to {
		transform: rotate(360deg);
	}
}

.tile span {
	color: var(--text-3);
}

.tile:hover:not(:disabled) {
	background: var(--bg-hover);
}
</style>
