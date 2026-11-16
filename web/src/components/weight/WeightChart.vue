<script setup lang="ts">
// A small SVG line chart of body weight: the raw weigh-ins faintly, a
// 7-entry moving average as the trend line on top.
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import type {WeightEntry} from '@/api/types'
import {fmtDayMonth} from '@/lib/dates'

function parseDay(key: string) {
	const [y, m, d] = key.split('-').map(Number)
	return new Date(y, m - 1, d)
}

const props = withDefaults(defineProps<{entries: WeightEntry[], color?: string, height?: number, axis?: boolean}>(), {
	color: '#4772fa',
	height: 120,
	axis: true,
})

const el = ref<HTMLElement | null>(null)
const width = ref(400)
let ro: ResizeObserver | null = null
onMounted(() => {
	ro = new ResizeObserver(([e]) => width.value = Math.max(100, e.contentRect.width))
	if (el.value) ro.observe(el.value)
})
onBeforeUnmount(() => ro?.disconnect())
const pad = computed(() => props.axis ? {l: 34, r: 8, t: 8, b: 18} : {l: 2, r: 2, t: 4, b: 4})

const geo = computed(() => {
	const W = width.value
	const e = props.entries
	if (e.length < 2) return null
	const kg = e.map(x => x.weight)
	const min = Math.floor(Math.min(...kg) - 0.3)
	const max = Math.ceil(Math.max(...kg) + 0.3)
	const t0 = parseDay(e[0].date).getTime()
	const t1 = parseDay(e.at(-1)!.date).getTime()
	const p = pad.value
	const x = (d: string) => p.l + (parseDay(d).getTime() - t0) / (t1 - t0 || 1) * (W - p.l - p.r)
	const y = (v: number) => p.t + (max - v) / (max - min) * (props.height - p.t - p.b)
	const pts = e.map(v => [x(v.date), y(v.weight)] as const)
	const avg = e.map((_, i) => {
		const win = e.slice(Math.max(0, i - 6), i + 1)
		return [pts[i][0], y(win.reduce((s, w) => s + w.weight, 0) / win.length)] as const
	})
	const ticks = [min, (min + max) / 2, max].map(v => ({v, y: y(v)}))
	return {
		W,
		line: pts.map(p => p.join(',')).join(' '),
		trend: avg.map(p => p.join(',')).join(' '),
		area: `${pts[0][0]},${props.height - p.b} ${pts.map(p => p.join(',')).join(' ')} ${pts.at(-1)![0]},${props.height - p.b}`,
		last: pts.at(-1)!,
		ticks,
		first: fmtDayMonth(parseDay(e[0].date)),
		end: fmtDayMonth(parseDay(e.at(-1)!.date)),
	}
})
</script>

<template>
	<div ref="el" class="wrap">
	<svg v-if="geo" :viewBox="`0 0 ${geo.W} ${height}`" :width="geo.W" :height="height" class="chart">
		<defs>
			<linearGradient :id="'wg' + color.slice(1)" x1="0" x2="0" y1="0" y2="1">
				<stop offset="0" :stop-color="color" stop-opacity="0.25" />
				<stop offset="1" :stop-color="color" stop-opacity="0" />
			</linearGradient>
		</defs>
		<template v-if="axis">
			<g v-for="tk in geo.ticks" :key="tk.v">
				<line :x1="pad.l" :x2="geo.W - pad.r" :y1="tk.y" :y2="tk.y" class="grid" />
				<text :x="pad.l - 6" :y="tk.y + 3" text-anchor="end" class="lbl">{{ tk.v.toFixed(1) }}</text>
			</g>
			<text :x="pad.l" :y="height - 4" class="lbl">{{ geo.first }}</text>
			<text :x="geo.W - pad.r" :y="height - 4" text-anchor="end" class="lbl">{{ geo.end }}</text>
		</template>
		<polygon :points="geo.area" :fill="`url(#wg${color.slice(1)})`" />
		<polyline :points="geo.line" fill="none" :stroke="color" stroke-opacity="0.45" stroke-width="1.5" vector-effect="non-scaling-stroke" />
		<polyline :points="geo.trend" fill="none" :stroke="color" stroke-width="2.5" stroke-linejoin="round" vector-effect="non-scaling-stroke" />
		<circle :cx="geo.last[0]" :cy="geo.last[1]" r="3.5" :fill="color" />
	</svg>
	</div>
</template>

<style scoped>
.wrap {
	width: 100%;
}

.chart {
	display: block;
}

.grid {
	stroke: var(--border);
	stroke-width: 1;
	vector-effect: non-scaling-stroke;
}

.lbl {
	font-size: 10px;
	fill: var(--text-3);
}
</style>
