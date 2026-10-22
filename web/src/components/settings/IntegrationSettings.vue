<script setup lang="ts">
import {ChevronRight, CircleQuestionMark} from 'lucide-vue-next'
import {t, type MessageKey} from '@/lib/i18n'

interface Service {
	/** Brand name, shown as is. */
	name?: string
	label?: MessageKey
	desc?: MessageKey
	/** Letter mark and color standing in for the service's logo. */
	mark: string
	color: string
	help?: boolean
}

const sections: {title: MessageKey, items: Service[]}[] = [
	{title: 'featCalendar', items: [
		{name: 'Google Calendar', mark: '31', color: '#4285f4'},
		{name: 'Outlook Calendar', mark: 'O', color: '#0f6cbd'},
		{name: 'Exchange Calendar', mark: 'E', color: '#1a73e8'},
		{name: 'iCloud Calendar', mark: '☁', color: '#3d9bf0'},
		{name: 'CalDAV', mark: 'C', color: '#f0a03a'},
		{label: 'holidays', mark: '★', color: '#2fb592'},
		{name: 'URL', mark: '🔗', color: '#5a8df5', help: true},
	]},
	{title: 'connect', items: [
		{name: 'Google Calendar', mark: '31', color: '#4285f4'},
		{name: 'Notion', mark: 'N', color: '#191919'},
		{name: 'Zapier', mark: '✱', color: '#ff4f00'},
		{name: 'IFTTT', mark: 'if', color: '#222'},
	]},
	{title: 'import', items: [
		{name: 'Todoist', mark: 'T', color: '#e44332'},
		{name: 'Microsoft Todo', mark: '✓', color: '#2564cf'},
		{name: 'Things', mark: '✓', color: '#3e7bf6'},
		{name: 'OmniFocus', mark: '◐', color: '#7d3fc1', help: true},
		{name: 'Toodledo', mark: '✓', color: '#f08a24'},
		{name: 'iCal', mark: '31', color: '#f0a03a'},
	]},
	{title: 'email', items: [
		{name: 'Gmail', mark: 'M', color: '#ea4335'},
		{name: 'Spark', mark: '✈', color: '#2f80ed'},
		{name: 'Outlook Email', mark: 'O', color: '#0f6cbd'},
	]},
	{title: 'others', items: [
		{label: 'subscribeToApp', desc: 'subscribeToAppDesc', mark: '⇄', color: '#4772fa'},
		{label: 'addViaEmail', desc: 'addViaEmailDesc', mark: '@', color: '#4772fa'},
		{label: 'shortcutsShort', mark: '◆', color: '#e8457a'},
		{name: 'Telegram', mark: '➤', color: '#2aa3df'},
		{name: 'Slack', mark: '#', color: '#4a154b'},
		{name: 'Chatmind', mark: 'C', color: '#5b3cc4'},
	]},
]
</script>

<template>
	<template v-for="section in sections" :key="section.title">
		<h2 class="s-title">{{ t(section.title) }}</h2>
		<div class="tiles">
			<button v-for="(item, i) in section.items" :key="i" class="tile">
				<span class="logo" :style="{background: item.color}">{{ item.mark }}</span>
				<span class="text">
					<span class="name">
						{{ item.label ? t(item.label) : item.name }}
						<CircleQuestionMark v-if="item.help" :size="12" class="help" />
					</span>
					<span v-if="item.desc" class="desc">{{ t(item.desc) }}</span>
				</span>
			</button>
		</div>
	</template>

	<h2 class="s-title">{{ t('integrationsAuthorized') }}</h2>
	<div class="s-card authorized">
		<p>{{ t('integrationsAuthorizedDesc') }}</p>
		<button class="s-link learn">{{ t('learnMore') }} <ChevronRight :size="12" /></button>
	</div>
</template>

<style scoped>
.tiles {
	display: grid;
	grid-template-columns: repeat(2, minmax(0, 1fr));
	gap: 10px;
}

.tile {
	display: flex;
	align-items: center;
	gap: 10px;
	min-height: 58px;
	padding: 8px 12px;
	border-radius: 8px;
	background: var(--settings-card);
	text-align: left;
}

.tile:hover {
	background: var(--bg-active);
}

.logo {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 24px;
	height: 24px;
	flex-shrink: 0;
	border-radius: 6px;
	color: #fff;
	font-size: 11px;
	font-weight: 700;
}

.text {
	display: flex;
	flex-direction: column;
	min-width: 0;
}

.name {
	display: flex;
	align-items: center;
	gap: 5px;
	font-size: 13px;
}

.help {
	color: var(--text-3);
}

.desc {
	margin-top: 2px;
	font-size: 11px;
	line-height: 1.4;
	color: var(--text-3);
}

.authorized {
	padding: 12px 15px;
	font-size: 13px;
}

.authorized p {
	margin: 0 0 12px;
}

.learn {
	display: inline-flex;
	align-items: center;
	gap: 2px;
}
</style>
