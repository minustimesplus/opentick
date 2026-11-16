<script setup lang="ts">
// Latest weight, its 7- and 30-day change, a chart and the entry field.
import {computed, onMounted} from 'vue'
import {Minus, TrendingDown, TrendingUp} from 'lucide-vue-next'
import WeightChart from './WeightChart.vue'
import WeightInput from './WeightInput.vue'
import {fmtWeight, useWeights} from '@/stores/weights'
import {fmtDayMonth} from '@/lib/dates'
import {t} from '@/lib/i18n'

withDefaults(defineProps<{color?: string, days?: number}>(), {color: '#4772fa', days: 30})
const weights = useWeights()

onMounted(() => weights.load())

const changes = computed(() => [7, 30].map(n => ({n, delta: weights.change(n)})).filter(c => c.delta !== null))

const subtitle = computed(() => {
	const last = weights.latest
	if (!last) return ''
	if (weights.today) return t('weightToday')
	const [y, m, d] = last.date.split('-').map(Number)
	return t('lastWeighIn', {date: fmtDayMonth(new Date(y, m - 1, d))})
})
</script>

<template>
	<div class="weight-card">
		<div v-if="weights.latest" class="top">
			<div>
				<div class="big">{{ fmtWeight(weights.latest.weight) }}<span class="u">kg</span></div>
				<div class="muted small">{{ subtitle }}</div>
			</div>
			<div class="chips">
				<span v-for="c in changes" :key="c.n" class="chip" :class="{down: c.delta! < 0}">
					<component :is="c.delta! < 0 ? TrendingDown : c.delta! > 0 ? TrendingUp : Minus" :size="12" />
					{{ t('weightChange', {delta: fmtWeight(c.delta, true), n: c.n}) }}
				</span>
			</div>
		</div>
		<p v-else-if="weights.loaded" class="muted small empty">{{ t('noWeights') }}</p>
		<WeightChart :entries="weights.since(days)" :color="color" :height="110" />
		<WeightInput />
	</div>
</template>

<style scoped>
.weight-card {
	display: flex;
	flex-direction: column;
	gap: 12px;
	padding: 14px;
	border-radius: 10px;
	background: var(--bg-panel);
}

.top {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
}

.big {
	font-size: 28px;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}

.u {
	margin-left: 3px;
	font-size: 14px;
	font-weight: 500;
	color: var(--text-3);
}

.small {
	font-size: 12px;
}

.empty {
	margin: 0;
}

.chips {
	display: flex;
	flex-direction: column;
	align-items: flex-end;
	gap: 4px;
}

.chip {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	padding: 2px 8px;
	border-radius: 999px;
	background: var(--bg-hover);
	font-size: 12px;
	color: var(--text-2);
	font-variant-numeric: tabular-nums;
}

.chip.down {
	color: #2f9e44;
}
</style>
