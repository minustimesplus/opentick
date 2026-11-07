<script setup lang="ts">
import DayIcon from '../DayIcon.vue'
import SettingSelect from './SettingSelect.vue'
import {t, type MessageKey} from '@/lib/i18n'
import {prefs} from '@/lib/prefs'
import {SMART_LISTS, type SmartListDef, type Visibility} from '@/lib/smartLists'

const groups = (['views', 'sections', 'closed'] as const).map(g => SMART_LISTS.filter(d => d.group === g))

const modeLabels: Record<Visibility, MessageKey> = {show: 'show', hide: 'hide', auto: 'showIfNotEmpty'}

function options(def: SmartListDef) {
	return def.modes.map(value => ({value, label: t(modeLabels[value])}))
}
</script>

<template>
	<h2 class="s-title">{{ t('smartList') }}</h2>

	<div v-for="(group, i) in groups" :key="i" class="s-card">
		<div v-for="item in group" :key="item.key" class="s-row">
			<span class="s-label">
				<component :is="item.icon" v-if="item.icon" :size="15" :stroke-width="1.7" />
				<DayIcon v-else :label="new Date().getDate()" :size="15" />
				{{ t(item.label) }}
			</span>
			<SettingSelect v-model="prefs.smartLists[item.key]" :options="options(item)" />
		</div>
	</div>
</template>
