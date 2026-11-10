<script setup lang="ts">
import SettingToggle from './SettingToggle.vue'
import FeaturePreview from '../FeaturePreview.vue'
import {t} from '@/lib/i18n'
import {prefs} from '@/lib/prefs'
import {OPTIONAL_FEATURES as features} from '@/lib/features'
</script>

<template>
	<h2 class="s-title flush">{{ t('features') }}</h2>

	<div class="grid">
		<div v-for="f in features" :key="f.key" class="feature">
			<div class="head">
				<span class="name">{{ t(f.title) }}</span>
				<SettingToggle v-model="prefs.features[f.key]" />
			</div>
			<div class="desc">{{ t(f.desc) }}</div>
			<FeaturePreview :feature="f.key" />
		</div>
	</div>
</template>

<style scoped>
.grid {
	display: grid;
	grid-template-columns: repeat(2, minmax(0, 1fr));
	gap: 20px 16px;
}

.head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
}

.name {
	font-size: 13px;
	font-weight: 500;
}

.desc {
	min-height: 30px;
	margin: 3px 0 6px;
	font-size: 11px;
	line-height: 1.4;
	color: var(--text-3);
}

</style>
