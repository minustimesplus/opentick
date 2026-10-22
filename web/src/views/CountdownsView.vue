<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {Plus} from 'lucide-vue-next'
import CountdownDialog from '@/components/CountdownDialog.vue'
import type {Countdown} from '@/api/types'
import {useCountdowns} from '@/stores/countdowns'
import {compareCountdowns, countdownState, type CountdownState} from '@/lib/countdowns'
import {fmtDayMonth, fmtNumericDate} from '@/lib/dates'
import {t} from '@/lib/i18n'

const store = useCountdowns()
const editing = ref<Countdown | null>(null)
const adding = ref(false)

// Re-render after midnight so the day counts stay right on a page left open.
const now = ref(new Date())
let timer: number | undefined
onMounted(() => {
	if (!store.loaded) store.load()
	timer = window.setInterval(() => { now.value = new Date() }, 60_000)
})
onUnmounted(() => clearInterval(timer))

const DEFAULT_ICONS = {countdown: '⏳', anniversary: '💝', birthday: '🎂'}

interface Row {
	c: Countdown
	s: CountdownState
}

const rows = computed<Row[]>(() => store.countdowns
	.map(c => ({c, s: countdownState(c, now.value)}))
	.sort((a, b) => compareCountdowns(a.s, b.s) || a.c.title.localeCompare(b.c.title)))
const upcoming = computed(() => rows.value.filter(r => r.s.days >= 0))
const passed = computed(() => rows.value.filter(r => r.s.days < 0))

function color(c: Countdown) {
	return '#' + (c.hex_color?.replace(/^#/, '') || '4772FA')
}

function detail({c, s}: Row): string {
	if (s.years !== null) {
		const when = fmtDayMonth(s.target)
		if (c.type === 'birthday') return t('turnsAge', {n: s.years, date: when})
		return t('yearsOn', {n: s.years, date: when})
	}
	return fmtNumericDate(s.date)
}

function since({c, s}: Row): string {
	// Repeating anniversaries also show how long it has been, TickTick-style.
	if (c.type !== 'anniversary' || s.years === null || s.daysSince === null) return ''
	return t('daysSinceCount', {n: s.daysSince})
}

function unit(s: CountdownState): string {
	const n = Math.abs(s.days)
	if (s.days > 0) return t(n === 1 ? 'dayLeft' : 'daysLeft')
	return t(n === 1 ? 'dayAgo' : 'daysAgo')
}
</script>

<template>
	<div class="countdowns-view">
		<header class="head">
			<h1>{{ t('countdowns') }}</h1>
			<button class="icon-btn" :title="t('addCountdown')" @click="adding = true"><Plus :size="20" /></button>
		</header>

		<div v-if="store.loaded && !rows.length" class="empty-state">
			<div class="big-emoji">⏳</div>
			<div>{{ t('noCountdowns') }}</div>
			<div class="sub">{{ t('noCountdownsHint') }}</div>
			<button class="btn primary" @click="adding = true"><Plus :size="16" /> {{ t('addCountdown') }}</button>
		</div>

		<div v-else class="list">
			<template v-for="group in [upcoming, passed]" :key="group === upcoming ? 'up' : 'past'">
				<h2 v-if="group === passed && passed.length" class="group">{{ t('countdownsPassed') }}</h2>
				<button
					v-for="row in group"
					:key="row.c.id"
					class="card"
					:class="{past: row.s.days < 0, today: row.s.days === 0}"
					:style="{'--cd-color': color(row.c)}"
					@click="editing = row.c"
				>
					<span class="emoji">{{ row.c.icon || DEFAULT_ICONS[row.c.type] }}</span>
					<span class="text">
						<span class="title">{{ row.c.title }}</span>
						<span class="detail">
							{{ detail(row) }}<template v-if="since(row)"> · {{ since(row) }}</template>
						</span>
					</span>
					<span v-if="row.s.days === 0" class="count">
						<span class="num today-label">{{ t('today') }}</span>
					</span>
					<span v-else class="count">
						<span class="num">{{ Math.abs(row.s.days) }}</span>
						<span class="unit">{{ unit(row.s) }}</span>
					</span>
				</button>
			</template>
		</div>

		<CountdownDialog v-if="adding" @close="adding = false" />
		<CountdownDialog v-if="editing" :countdown="editing" @close="editing = null" />
	</div>
</template>

<style scoped>
.countdowns-view {
	display: flex;
	flex-direction: column;
	height: 100%;
	max-width: 900px;
	margin: 0 auto;
	padding: 0 24px 24px;
}

.head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	height: 72px;
	padding: 0 4px;
	flex-shrink: 0;
}

h1 {
	margin: 0;
	font-size: 22px;
	font-weight: 700;
}

.list {
	flex: 1;
	overflow: auto;
	display: flex;
	flex-direction: column;
	gap: 10px;
}

.group {
	margin: 14px 4px 2px;
	font-size: 13px;
	font-weight: 600;
	color: var(--text-3);
}

.card {
	display: flex;
	align-items: center;
	gap: 14px;
	width: 100%;
	padding: 14px 18px 14px 14px;
	border-radius: 12px;
	background: var(--bg-panel);
	border-left: 4px solid var(--cd-color);
	text-align: left;
}

@media (hover: hover) {
	.card:hover {
		background: var(--bg-hover);
	}
}

.card.today {
	background: color-mix(in srgb, var(--cd-color) 12%, var(--bg-panel));
}

.emoji {
	display: flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	width: 44px;
	height: 44px;
	border-radius: 10px;
	font-size: 24px;
	background: color-mix(in srgb, var(--cd-color) 15%, transparent);
}

.text {
	display: flex;
	flex-direction: column;
	gap: 3px;
	flex: 1;
	min-width: 0;
}

.title {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 15px;
	font-weight: 600;
}

.detail {
	font-size: 12px;
	color: var(--text-3);
}

.count {
	display: flex;
	flex-direction: column;
	align-items: flex-end;
	flex-shrink: 0;
	color: var(--cd-color);
}

.num {
	font-size: 28px;
	font-weight: 700;
	line-height: 1;
	font-variant-numeric: tabular-nums;
}

.today-label {
	font-size: 18px;
}

.unit {
	margin-top: 3px;
	font-size: 12px;
	color: var(--text-3);
}

.past .count {
	color: var(--text-3);
}

.big-emoji {
	font-size: 40px;
}

.empty-state .btn {
	margin-top: 10px;
}

@media (max-width: 767px) {
	.countdowns-view {
		padding: 0 10px 10px;
	}

	.head {
		height: 56px;
	}

	.card {
		padding: 12px 14px 12px 10px;
	}
}
</style>
