<script setup lang="ts">
import {computed, reactive, type Component} from 'vue'
import {
	CalendarRange, FileText, Inbox, Layers, ListFilter, SquareCheck, SquareX, Sunrise, Tag, Trash2, UserCheck,
} from 'lucide-vue-next'
import DayIcon from '../DayIcon.vue'
import SettingSelect from './SettingSelect.vue'
import {t, type MessageKey} from '@/lib/i18n'

type Visibility = 'show' | 'hide' | 'auto'

type ListKey = 'all' | 'today' | 'tomorrow' | 'next7' | 'assigned' | 'inbox' | 'summary'
	| 'tags' | 'filters' | 'completed' | 'wontDo' | 'trash'

interface SmartList {
	key: ListKey
	label: MessageKey
	icon?: Component
}

const groups: SmartList[][] = [
	[
		{key: 'all', label: 'all', icon: Layers},
		{key: 'today', label: 'today'},
		{key: 'tomorrow', label: 'tomorrow', icon: Sunrise},
		{key: 'next7', label: 'next7', icon: CalendarRange},
		{key: 'assigned', label: 'assignedToMe', icon: UserCheck},
		{key: 'inbox', label: 'inbox', icon: Inbox},
		{key: 'summary', label: 'summary', icon: FileText},
	],
	[
		{key: 'tags', label: 'tags', icon: Tag},
		{key: 'filters', label: 'filters', icon: ListFilter},
	],
	[
		{key: 'completed', label: 'completed', icon: SquareCheck},
		{key: 'wontDo', label: 'wontDo', icon: SquareX},
		{key: 'trash', label: 'trash', icon: Trash2},
	],
]

// Not wired up yet: the sidebar ignores these.
const visibility = reactive<Record<ListKey, Visibility>>({
	all: 'hide',
	today: 'show',
	tomorrow: 'hide',
	next7: 'show',
	assigned: 'auto',
	inbox: 'show',
	summary: 'hide',
	tags: 'show',
	filters: 'show',
	completed: 'show',
	wontDo: 'auto',
	trash: 'show',
})

const options = computed(() => [
	{value: 'show' as const, label: t('show')},
	{value: 'hide' as const, label: t('hide')},
	{value: 'auto' as const, label: t('showIfNotEmpty')},
])
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
			<SettingSelect v-model="visibility[item.key]" :options="options" />
		</div>
	</div>
</template>
