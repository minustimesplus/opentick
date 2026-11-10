<script setup lang="ts">
// Miniature of an optional view, shown in Settings > Features and onboarding.
// `animated` brings it to life (events landing, habits ticking, the timer
// running) for the onboarding cards.
import type {Feature} from '@/lib/prefs'
import {FEATURE_TINTS} from '@/lib/features'

defineProps<{feature: Feature, animated?: boolean}>()
</script>

<template>
	<div class="preview" :class="{animated}" :style="{'--tint': FEATURE_TINTS[feature]}">
		<div class="window" :class="feature">
			<template v-if="feature === 'calendar'">
				<i v-for="n in 21" :key="n" :class="{ev: n % 4 === 1 || n % 7 === 3}" :style="{'--i': n}" />
			</template>
			<template v-else-if="feature === 'matrix'">
				<div v-for="(c, q) in ['#e5484d', '#f5a623', '#4772fa', '#2bb673']" :key="q" class="quad" :style="{'--i': q}">
					<b :style="{background: c}" />
					<span v-for="n in 3" :key="n" />
				</div>
			</template>
			<template v-else-if="feature === 'habit'">
				<div v-for="(c, r) in ['#4772fa', '#f5a623', '#e5484d', '#2bb673']" :key="r" class="habit-row">
					<b :style="{background: c}" />
					<span v-for="n in 5" :key="n" :class="{done: (n + r) % 3 !== 0}" :style="{'--i': r * 5 + n}" />
				</div>
			</template>
			<template v-else-if="feature === 'pomo'">
				<div class="ring"><span>16:36</span></div>
				<div class="play" />
			</template>
			<template v-else-if="feature === 'countdown'">
				<div v-for="(n, i) in [83, 204, 210]" :key="i" class="count" :style="{'--i': i}">
					<b />
					<span>{{ n }}</span>
				</div>
			</template>
			<template v-else>
				<div class="spark">✦</div>
				<span v-for="n in 4" :key="n" class="line" :style="{'--i': n}" />
			</template>
		</div>
	</div>
</template>

<style scoped>
.preview {
	position: relative;
	aspect-ratio: 250 / 147;
	overflow: hidden;
	border-radius: 6px;
	background: linear-gradient(160deg, color-mix(in srgb, var(--tint) 45%, #fff), var(--tint));
}

.window {
	position: absolute;
	inset: 12% 0 0 9%;
	border-radius: 6px 0 0 0;
	background: #fff;
	box-shadow: 0 2px 10px rgba(0, 0, 0, 0.08);
	padding: 10% 8% 0;
}

.window.calendar {
	display: grid;
	grid-template-columns: repeat(7, 1fr);
	gap: 4px;
	align-content: start;
}

.calendar i {
	height: 16px;
	border-top: 1px solid #eee;
}

.calendar i.ev {
	border-top: 4px solid #c9d3fb;
}

.window.matrix {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 12px 14px;
}

.quad {
	display: flex;
	flex-direction: column;
	gap: 5px;
}

.quad b {
	width: 40%;
	height: 4px;
	border-radius: 2px;
}

.quad span,
.ai .line {
	height: 4px;
	border-radius: 2px;
	background: #ececec;
}

.quad span:nth-child(odd) {
	width: 75%;
}

.window.habit {
	display: flex;
	flex-direction: column;
	gap: 10px;
}

.habit-row {
	display: flex;
	align-items: center;
	gap: 5px;
}

.habit-row b {
	width: 10px;
	height: 10px;
	margin-right: 8px;
	border-radius: 50%;
}

.habit-row span {
	width: 8px;
	height: 8px;
	border-radius: 50%;
	background: #e3e8fd;
}

.habit-row span.done {
	background: #7d95f7;
}

.window.pomo {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 8px;
}

.ring {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 56px;
	height: 56px;
	border-radius: 50%;
	position: relative;
	border: 3px solid #e3e8fd;
	font-size: 10px;
	color: #333;
}

.ring::before {
	content: '';
	position: absolute;
	inset: -3px;
	border: 3px solid transparent;
	border-top-color: #4772fa;
	border-right-color: #4772fa;
	border-radius: 50%;
}

.play {
	width: 26px;
	height: 8px;
	border-radius: 4px;
	background: #4772fa;
}

.window.countdown {
	display: flex;
	gap: 8px;
}

.count {
	display: flex;
	flex: 1;
	flex-direction: column;
	align-items: center;
	gap: 5px;
	padding: 6px 0;
	border-radius: 4px;
	background: #f4f6fe;
	font-size: 11px;
	color: #4772fa;
}

.count b {
	width: 9px;
	height: 9px;
	border-radius: 50%;
	background: #f5a623;
}

.window.ai {
	display: flex;
	flex-direction: column;
	gap: 6px;
}

.spark {
	color: #4772fa;
	font-size: 14px;
}

.ai .line:nth-child(even) {
	width: 60%;
}

/* ---------- animated ---------- */

.animated .window {
	transition: transform 0.35s ease;
}

.animated .calendar i.ev {
	animation: drop 3.2s ease-out infinite;
	animation-delay: calc(var(--i) * 70ms);
}

.animated .quad span {
	transform-origin: left;
	animation: grow 3.2s ease-out infinite;
	animation-delay: calc(var(--i) * 180ms);
}

.animated .habit-row span.done {
	animation: tick 3.2s ease-out infinite;
	animation-delay: calc(var(--i) * 60ms);
}

.animated .ring::before {
	animation: spin 6s linear infinite;
}

.animated .count {
	animation: bob 2.4s ease-in-out infinite;
	animation-delay: calc(var(--i) * 240ms);
}

.animated .ai .line {
	transform-origin: left;
	animation: grow 2.6s ease-out infinite;
	animation-delay: calc(var(--i) * 220ms);
}

.animated .spark {
	animation: twinkle 1.6s ease-in-out infinite;
}

@keyframes drop {
	0%, 8% { opacity: 0; transform: translateY(-6px); }
	22%, 100% { opacity: 1; transform: none; }
}

@keyframes grow {
	0%, 6% { transform: scaleX(0); }
	30%, 100% { transform: scaleX(1); }
}

@keyframes tick {
	0%, 10% { background: #e3e8fd; transform: scale(0.6); }
	24% { transform: scale(1.25); }
	34%, 100% { background: #7d95f7; transform: none; }
}

@keyframes spin {
	to { transform: rotate(360deg); }
}

@keyframes bob {
	0%, 100% { transform: none; }
	50% { transform: translateY(-3px); }
}

@keyframes twinkle {
	0%, 100% { opacity: 1; transform: none; }
	50% { opacity: 0.5; transform: scale(0.8) rotate(20deg); }
}

@media (prefers-reduced-motion: reduce) {
	.animated * {
		animation: none !important;
	}
}
</style>
