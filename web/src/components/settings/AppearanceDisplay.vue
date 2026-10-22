<script setup lang="ts">
import {reactive} from 'vue'
import {Check, CircleQuestionMark, Inbox} from 'lucide-vue-next'
import DayIcon from '../DayIcon.vue'
import {t} from '@/lib/i18n'

// Not wired up yet: the sidebar and task rows ignore these.
const state = reactive({
	count: 'all' as 'all' | 'hideNote' | 'none',
	listColor: true,
	strike: false,
})

const counts = [
	{value: 'all' as const, label: 'countShowAll' as const, today: 3, inbox: 16},
	{value: 'hideNote' as const, label: 'countHideNote' as const, today: 3, inbox: 10},
	{value: 'none' as const, label: 'countHideAll' as const, today: null, inbox: null},
]
</script>

<template>
	<h2 class="s-title group">{{ t('sidebarCount') }}</h2>
	<div class="options">
		<button v-for="o in counts" :key="o.value" class="option" @click="state.count = o.value">
			<span class="card" :class="{active: state.count === o.value}">
				<span class="line"><DayIcon :label="new Date().getDate()" :size="11" />{{ t('today') }}<em>{{ o.today }}</em></span>
				<span class="line"><Inbox :size="11" />{{ t('inbox') }}<em>{{ o.inbox }}</em></span>
				<span v-if="state.count === o.value" class="badge"><Check :size="10" :stroke-width="3" /></span>
			</span>
			<span class="name">{{ t(o.label) }}</span>
		</button>
	</div>

	<h2 class="s-title group">{{ t('listColor') }} <CircleQuestionMark :size="13" /></h2>
	<div class="options">
		<button v-for="on in [true, false]" :key="String(on)" class="option" @click="state.listColor = on">
			<span class="card" :class="{active: state.listColor === on, colored: on}">
				<span class="line"><i class="box" />{{ t('taskTitle') }}</span>
				<span class="line"><i class="box" />{{ t('taskTitle') }}</span>
				<span v-if="state.listColor === on" class="badge"><Check :size="10" :stroke-width="3" /></span>
			</span>
			<span class="name">{{ on ? t('show') : t('hide') }}</span>
		</button>
	</div>

	<h2 class="s-title group">{{ t('completedStyle') }}</h2>
	<div class="options">
		<button v-for="strike in [false, true]" :key="String(strike)" class="option" @click="state.strike = strike">
			<span class="card done" :class="{active: state.strike === strike, strike}">
				<span class="line"><i class="box checked" /><span class="title">{{ t('taskTitle') }}</span></span>
				<span class="line"><i class="box checked" /><span class="title">{{ t('taskTitle') }}</span></span>
				<span v-if="state.strike === strike" class="badge"><Check :size="10" :stroke-width="3" /></span>
			</span>
			<span class="name">{{ strike ? t('completedStrikethrough') : t('themeDefault') }}</span>
		</button>
	</div>
</template>

<style scoped>
.s-title.group {
	margin: 18px 0 10px;
}

.s-title.group:first-child {
	margin-top: 0;
}

.options {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 10px;
}

.option {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 7px;
	min-width: 0;
}

.card {
	position: relative;
	display: flex;
	flex-direction: column;
	gap: 9px;
	width: 100%;
	padding: 10px 12px;
	border-radius: 8px;
	border: 1px solid var(--border-strong);
	font-size: 11px;
	text-align: left;
}

.card.colored .line:first-child::before,
.card.colored .line:nth-child(2)::before {
	content: '';
	position: absolute;
	left: 0;
	width: 3px;
	height: 18px;
	margin-top: -2px;
}

.card.colored .line:first-child::before {
	background: #4772fa;
}

.card.colored .line:nth-child(2)::before {
	background: #f5a623;
}

.line {
	display: flex;
	align-items: center;
	gap: 7px;
	color: var(--text-2);
}

.line svg {
	color: var(--text-3);
}

.line em {
	margin-left: auto;
	font-style: normal;
	color: var(--text-3);
}

.box {
	width: 10px;
	height: 10px;
	border-radius: 2px;
	border: 1px solid var(--text-4);
}

.box.checked {
	border-color: var(--border-strong);
	background: var(--border-strong);
}

.done .title {
	color: var(--text-4);
}

.strike .title {
	text-decoration: line-through;
}

.badge {
	position: absolute;
	right: -6px;
	bottom: -6px;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 17px;
	height: 17px;
	border-radius: 50%;
	border: 2px solid var(--bg);
	background: var(--accent);
	color: #fff;
}

.name {
	font-size: 11px;
	color: var(--text-3);
}
</style>
