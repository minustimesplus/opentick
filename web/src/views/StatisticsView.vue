<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {Info} from 'lucide-vue-next'
import Popover from '@/components/Popover.vue'
import {api} from '@/api/client'
import type {StatsBucket, UserStatistics} from '@/api/types'
import {intlLocale, t} from '@/lib/i18n'
import {fmtDayMonth, fmtMonthShort} from '@/lib/dates'
import {prefs} from '@/lib/prefs'
import {labelColor} from '@/lib/tasks'
import {useData} from '@/stores/data'

// Mirrors the score formula in pkg/models/user_statistics.go.
const RULES = {completed: 10, onTime: 5, late: 5, overdue: 2}

type Period = 'daily' | 'weekly' | 'monthly'

const data = useData()
const stats = ref<UserStatistics | null>(null)
const failed = ref(false)
const period = ref<Period>('daily')
const rulesEl = ref<HTMLElement | null>(null)
const rulesOpen = ref(false)

async function load() {
	failed.value = false
	try {
		stats.value = await api<UserStatistics>('GET', '/user/statistics', {query: {
			timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
			week_start: prefs.weekStart,
			days: 30,
			weeks: 12,
			months: 12,
		}})
	} catch {
		failed.value = true
	}
}

onMounted(load)
watch(() => prefs.weekStart, load)
// A sync, or completing a task elsewhere, changes the numbers.
watch(() => data.loading, loading => {
	if (!loading) load()
})

function parseDay(s: string): Date {
	const [y, m, d] = s.split('-').map(Number)
	return new Date(y, m - 1, d)
}

const score = computed(() => stats.value?.score)
const levelProgress = computed(() => {
	const s = score.value
	if (!s) return 0
	const span = s.next_level_score - s.level_score
	return span > 0 ? Math.min(1, (s.score - s.level_score) / span) : 0
})

const ratePercent = computed(() => Math.round((stats.value?.completion_rate.rate ?? 0) * 100))
const hasRated = computed(() => {
	const r = stats.value?.completion_rate
	return !!r && r.on_time + r.late > 0
})

const series = computed<StatsBucket[]>(() => stats.value?.[period.value] ?? [])
const seriesMax = computed(() => Math.max(1, ...series.value.map(b => b.count)))
const seriesTotal = computed(() => series.value.reduce((n, b) => n + b.count, 0))

function bucketLabel(b: StatsBucket): string {
	const d = parseDay(b.date)
	if (period.value === 'monthly') return fmtMonthShort(d)
	if (period.value === 'daily') return String(d.getDate())
	return fmtDayMonth(d)
}

function bucketTitle(b: StatsBucket): string {
	const d = parseDay(b.date)
	const when = period.value === 'monthly'
		? new Intl.DateTimeFormat(intlLocale.value, {month: 'long', year: 'numeric'}).format(d)
		: fmtDayMonth(d)
	return `${when}: ${b.count}`
}

// Daily and weekly bars are too narrow to label each one.
function showLabel(i: number): boolean {
	const n = series.value.length
	const every = period.value === 'daily' ? 5 : (period.value === 'weekly' ? 2 : 1)
	return (n - 1 - i) % every === 0
}

// Donut: circumference of r=15.9155 is 100, so dash lengths are percentages.
const donut = computed(() => {
	const r = stats.value?.completion_rate
	const total = r ? r.on_time + r.late : 0
	const onTime = total ? (r!.on_time / total) * 100 : 0
	return {onTime, late: total ? 100 - onTime : 0}
})

const projects = computed(() => stats.value?.projects ?? [])
const projectMax = computed(() => Math.max(1, ...projects.value.map(p => p.count)))

const periods: {key: Period, label: 'statsDaily' | 'statsWeekly' | 'statsMonthly'}[] = [
	{key: 'daily', label: 'statsDaily'},
	{key: 'weekly', label: 'statsWeekly'},
	{key: 'monthly', label: 'statsMonthly'},
]
</script>

<template>
	<div class="stats-view">
		<header class="head">
			<h1>{{ t('statistics') }}</h1>
		</header>

		<div v-if="failed && !stats" class="empty-state">{{ t('statsLoadFailed') }}</div>

		<div v-else-if="stats && score" class="body">
			<section class="card score-card">
				<div class="score-top">
					<div>
						<div class="muted small">{{ t('statsScore') }}</div>
						<div class="score">{{ score.score.toLocaleString(intlLocale) }}</div>
					</div>
					<div class="level">{{ t('statsLevel', {n: score.level}) }}</div>
				</div>
				<div class="progress"><div class="progress-fill" :style="{width: levelProgress * 100 + '%'}" /></div>
				<div class="score-foot">
					<span class="muted small">{{ t('statsToNextLevel', {n: score.next_level_score - score.score, level: score.level + 1}) }}</span>
					<button ref="rulesEl" class="link small" @click="rulesOpen = true"><Info :size="14" /> {{ t('statsHowScored') }}</button>
				</div>
			</section>

			<div class="tiles">
				<div class="card tile">
					<div class="tile-value">{{ stats.total_completed }}</div>
					<div class="muted small">{{ t('statsTotalCompleted') }}</div>
				</div>
				<div class="card tile">
					<div class="tile-value">{{ stats.completed_today }}</div>
					<div class="muted small">{{ t('statsCompletedToday') }}</div>
				</div>
				<div class="card tile">
					<div class="tile-value">{{ hasRated ? ratePercent + '%' : '–' }}</div>
					<div class="muted small">{{ t('statsOnTimeRate') }}</div>
				</div>
				<div class="card tile">
					<div class="tile-value" :class="{danger: stats.open_overdue > 0}">{{ stats.open_overdue }}</div>
					<div class="muted small">{{ t('statsOverdue') }}</div>
				</div>
			</div>

			<section class="card">
				<div class="card-head">
					<h2>{{ t('statsCompletions') }}</h2>
					<div class="seg">
						<button v-for="p in periods" :key="p.key" :class="{active: period === p.key}" @click="period = p.key">{{ t(p.label) }}</button>
					</div>
				</div>
				<div class="muted small">{{ t('statsTasksCount', {n: seriesTotal}) }}</div>
				<div class="chart">
					<div class="chart-max muted">{{ seriesMax }}</div>
					<div class="bars">
						<div v-for="(b, i) in series" :key="b.date" class="bar-col" :title="bucketTitle(b)">
							<div class="bar-track">
								<div class="bar" :class="{zero: !b.count}" :style="{height: (b.count / seriesMax) * 100 + '%'}" />
							</div>
							<div class="bar-label">{{ showLabel(i) ? bucketLabel(b) : '' }}</div>
						</div>
					</div>
				</div>
			</section>

			<div class="split">
				<section class="card">
					<h2>{{ t('statsCompletionRate') }}</h2>
					<div class="rate">
						<svg viewBox="0 0 36 36" class="donut">
							<circle cx="18" cy="18" r="15.9155" class="donut-bg" />
							<circle
								v-if="donut.late"
								cx="18" cy="18" r="15.9155" class="donut-late"
								:stroke-dasharray="`${donut.late} ${100 - donut.late}`"
								:stroke-dashoffset="25 - donut.onTime"
							/>
							<circle
								v-if="donut.onTime"
								cx="18" cy="18" r="15.9155" class="donut-on-time"
								:stroke-dasharray="`${donut.onTime} ${100 - donut.onTime}`"
								stroke-dashoffset="25"
							/>
							<text x="18" y="19" class="donut-text">{{ hasRated ? ratePercent + '%' : '–' }}</text>
						</svg>
						<ul class="legend">
							<li><span class="swatch on-time" />{{ t('statsOnTime') }}<b>{{ stats.completion_rate.on_time }}</b></li>
							<li><span class="swatch late" />{{ t('statsLate') }}<b>{{ stats.completion_rate.late }}</b></li>
							<li><span class="swatch none" />{{ t('statsNoDueDate') }}<b>{{ stats.completion_rate.no_due_date }}</b></li>
						</ul>
					</div>
				</section>

				<section class="card">
					<h2>{{ t('statsByList') }}</h2>
					<div v-if="!projects.length" class="muted small pad">{{ t('statsNoData') }}</div>
					<ul class="projects">
						<li v-for="p in projects.slice(0, 8)" :key="p.project_id">
							<div class="project-row">
								<span class="project-title">{{ p.title || '#' + p.project_id }}</span>
								<span class="muted">{{ p.count }}</span>
							</div>
							<div class="project-track">
								<div class="project-fill" :style="{width: (p.count / projectMax) * 100 + '%', background: p.hex_color ? labelColor(p.hex_color) : 'var(--accent)'}" />
							</div>
						</li>
					</ul>
				</section>
			</div>
		</div>

		<Popover v-if="rulesOpen" :anchor="rulesEl" placement="bottom-end" @close="rulesOpen = false">
			<div class="menu rules">
				<div class="menu-label">{{ t('statsHowScored') }}</div>
				<div class="rule"><span>{{ t('statsRuleCompleted', {n: RULES.completed}) }}</span><b>+{{ score?.completion_points ?? 0 }}</b></div>
				<div class="rule"><span>{{ t('statsRuleOnTime', {n: RULES.onTime}) }}</span><b>+{{ score?.on_time_points ?? 0 }}</b></div>
				<div class="rule"><span>{{ t('statsRuleLate', {n: RULES.late}) }}</span><b>−{{ score?.late_penalty ?? 0 }}</b></div>
				<div class="rule"><span>{{ t('statsRuleOverdue', {n: RULES.overdue}) }}</span><b>−{{ score?.overdue_penalty ?? 0 }}</b></div>
			</div>
		</Popover>
	</div>
</template>

<style scoped>
.stats-view {
	display: flex;
	flex-direction: column;
	height: 100%;
	overflow: auto;
}

.head {
	display: flex;
	align-items: center;
	height: 72px;
	padding: 0 24px;
	flex-shrink: 0;
}

h1 {
	margin: 0;
	font-size: 22px;
	font-weight: 700;
}

h2 {
	margin: 0 0 4px;
	font-size: 15px;
	font-weight: 600;
}

.body {
	display: flex;
	flex-direction: column;
	gap: 14px;
	width: 100%;
	max-width: 960px;
	margin: 0 auto;
	padding: 0 24px 24px;
}

.card {
	padding: 16px 18px;
	border: 1px solid var(--border);
	border-radius: 12px;
	background: var(--bg-panel);
	min-width: 0;
}

.muted {
	color: var(--text-3);
}

.small {
	font-size: 12px;
}

.score-card {
	background: linear-gradient(135deg, var(--accent-soft), transparent 70%), var(--bg-panel);
}

.score-top {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
}

.score {
	font-size: 36px;
	font-weight: 700;
	line-height: 1.2;
	font-variant-numeric: tabular-nums;
}

.level {
	padding: 4px 12px;
	border-radius: 999px;
	background: var(--accent);
	color: #fff;
	font-weight: 700;
	font-size: 14px;
}

.progress {
	height: 8px;
	margin: 12px 0 8px;
	border-radius: 4px;
	background: var(--bg-active);
	overflow: hidden;
}

.progress-fill {
	height: 100%;
	border-radius: 4px;
	background: var(--accent);
	transition: width 0.4s ease;
}

.score-foot {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
}

.link {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	color: var(--accent);
}

.tiles {
	display: grid;
	grid-template-columns: repeat(4, minmax(0, 1fr));
	gap: 14px;
}

.tile-value {
	font-size: 24px;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}

.tile-value.danger {
	color: var(--danger);
}

.card-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
}

.seg {
	display: inline-flex;
	padding: 2px;
	border-radius: 8px;
	background: var(--bg-input);
}

.seg button {
	padding: 4px 10px;
	border-radius: 6px;
	font-size: 12px;
	color: var(--text-2);
}

.seg button.active {
	background: var(--bg-elev);
	color: var(--text);
	box-shadow: 0 1px 2px rgba(0, 0, 0, 0.15);
}

.chart {
	position: relative;
	margin-top: 12px;
}

.chart-max {
	position: absolute;
	top: -2px;
	left: 0;
	font-size: 11px;
}

.bars {
	display: flex;
	align-items: stretch;
	gap: 3px;
	height: 180px;
	padding-top: 16px;
	border-bottom: 1px solid var(--border-strong);
}

.bar-col {
	flex: 1;
	display: flex;
	flex-direction: column;
	min-width: 0;
}

.bar-track {
	flex: 1;
	display: flex;
	align-items: flex-end;
}

.bar {
	width: 100%;
	border-radius: 3px 3px 0 0;
	background: var(--accent);
	transition: height 0.3s ease;
}

.bar-col:hover .bar {
	opacity: 0.8;
}

.bar.zero {
	height: 2px !important;
	background: var(--border-strong);
}

.bar-label {
	height: 0;
	overflow: visible;
	white-space: nowrap;
	text-align: center;
	font-size: 11px;
	color: var(--text-3);
	transform: translateY(6px);
}

.chart {
	padding-bottom: 20px;
}

.split {
	display: grid;
	grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
	gap: 14px;
}

.rate {
	display: flex;
	align-items: center;
	gap: 24px;
	margin-top: 12px;
}

.donut {
	width: 120px;
	height: 120px;
	flex-shrink: 0;
}

.donut circle {
	fill: none;
	stroke-width: 4;
}

.donut-bg {
	stroke: var(--bg-active);
}

.donut-on-time {
	stroke: var(--q4);
}

.donut-late {
	stroke: var(--q2);
}

.donut-text {
	fill: var(--text);
	font-size: 7px;
	font-weight: 700;
	text-anchor: middle;
	dominant-baseline: middle;
}

.legend {
	flex: 1;
	margin: 0;
	padding: 0;
	list-style: none;
	font-size: 13px;
}

.legend li {
	display: flex;
	align-items: center;
	gap: 8px;
	padding: 4px 0;
}

.legend b {
	margin-left: auto;
	font-weight: 600;
}

.swatch {
	width: 10px;
	height: 10px;
	border-radius: 50%;
}

.swatch.on-time {
	background: var(--q4);
}

.swatch.late {
	background: var(--q2);
}

.swatch.none {
	background: var(--text-4);
}

.projects {
	margin: 8px 0 0;
	padding: 0;
	list-style: none;
}

.projects li {
	padding: 5px 0;
}

.project-row {
	display: flex;
	justify-content: space-between;
	gap: 8px;
	font-size: 13px;
}

.project-title {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.project-track {
	height: 6px;
	margin-top: 4px;
	border-radius: 3px;
	background: var(--bg-active);
	overflow: hidden;
}

.project-fill {
	height: 100%;
	border-radius: 3px;
}

.pad {
	padding: 12px 0;
}

.rules {
	min-width: 280px;
}

.rule {
	display: flex;
	justify-content: space-between;
	gap: 12px;
	padding: 6px 12px;
	font-size: 13px;
}

@media (max-width: 767px) {
	.head {
		height: 56px;
		padding: 0 16px;
	}

	.body {
		padding: 0 12px 16px;
		gap: 10px;
	}

	.tiles {
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 10px;
	}

	.split {
		grid-template-columns: minmax(0, 1fr);
		gap: 10px;
	}

	.bars {
		gap: 2px;
	}
}
</style>
