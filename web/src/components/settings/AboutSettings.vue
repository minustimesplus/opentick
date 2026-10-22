<script setup lang="ts">
import {type Component} from 'vue'
import {Apple, CircleQuestionMark, Ellipsis, Instagram, MessageCircle, Smartphone, Twitter, Youtube} from 'lucide-vue-next'
import {t, type MessageKey} from '@/lib/i18n'

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
			<button class="s-link">{{ t('helpCenter') }}</button>
			{{ t('aboutHelpAfter') }}
			<br>
			{{ t('aboutContactBefore') }}
			<button class="s-link">{{ t('feedbackAndSuggestions') }}</button>
			{{ t('aboutContactAfter') }}
		</p>
		<div class="tiles">
			<button v-for="(item, i) in community" :key="i" class="tile">
				<component :is="item.icon" :size="20" :stroke-width="1.7" />
				<span>{{ item.label ? t(item.label) : item.name }}</span>
			</button>
		</div>

		<h2>{{ t('getApp') }}</h2>
		<p>{{ t('getAppDesc') }}</p>
		<div class="tiles">
			<button v-for="(item, i) in apps" :key="i" class="tile">
				<component :is="item.icon" :size="20" :stroke-width="1.7" />
				<span>{{ item.label ? t(item.label) : item.name }}</span>
			</button>
		</div>
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

.tile span {
	color: var(--text-3);
}

.tile:hover {
	background: var(--bg-hover);
}
</style>
