<script setup lang="ts">
import {Check} from 'lucide-vue-next'
import {t, type MessageKey} from '@/lib/i18n'

const unlocked: MessageKey[] = [
	'premiumCalendarViews',
	'premiumTimeline',
	'premiumFilters',
	'premiumDurations',
	'premiumReminders',
	'premiumStats',
	'premiumThemes',
	'premiumCollaboration',
	'premiumAttachments',
]
</script>

<template>
	<div class="premium">
		<h2>{{ t('premiumUnlockedTitle') }}</h2>
		<p>{{ t('premiumUnlockedDesc') }}</p>
		<ul>
			<li v-for="key in unlocked" :key="key">
				<Check :size="15" :stroke-width="2" />
				<span>{{ t(key) }}</span>
			</li>
		</ul>
	</div>
</template>

<style scoped>
.premium {
	padding: 0 2px;
}

h2 {
	margin: 0 0 16px;
	font-size: 18px;
	font-weight: 400;
}

p {
	margin: 0 0 16px;
	font-size: 13px;
	line-height: 1.6;
	color: var(--text-2);
}

ul {
	display: flex;
	flex-direction: column;
	gap: 10px;
	margin: 0;
	padding: 0;
	list-style: none;
	font-size: 13px;
}

li {
	display: flex;
	align-items: center;
	gap: 9px;
}

li svg {
	flex-shrink: 0;
	color: var(--accent);
}
</style>
