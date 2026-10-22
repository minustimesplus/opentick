<script setup lang="ts">
import {reactive} from 'vue'
import SettingToggle from './SettingToggle.vue'
import {t, type MessageKey} from '@/lib/i18n'

type Preview = 'calendar' | 'matrix' | 'habit' | 'pomo' | 'countdown' | 'ai'

const features: {key: Preview, title: MessageKey, desc: MessageKey, tint: string}[] = [
	{key: 'calendar', title: 'featCalendar', desc: 'featCalendarDesc', tint: '#7d95f7'},
	{key: 'matrix', title: 'matrix', desc: 'featMatrixDesc', tint: '#7fb2f2'},
	{key: 'habit', title: 'featHabit', desc: 'featHabitDesc', tint: '#9fdd9c'},
	{key: 'pomo', title: 'featPomo', desc: 'featPomoDesc', tint: '#f09a8a'},
	{key: 'countdown', title: 'featCountdown', desc: 'featCountdownDesc', tint: '#8fa6f6'},
	{key: 'ai', title: 'featAi', desc: 'featAiDesc', tint: '#8ab4f4'},
]

// Not wired up yet: the switches only flip locally.
const enabled = reactive<Record<Preview, boolean>>({
	calendar: true,
	matrix: true,
	habit: false,
	pomo: false,
	countdown: false,
	ai: true,
})
</script>

<template>
	<h2 class="s-title flush">{{ t('features') }}</h2>

	<div class="grid">
		<div v-for="f in features" :key="f.key" class="feature">
			<div class="head">
				<span class="name">{{ t(f.title) }}</span>
				<SettingToggle v-model="enabled[f.key]" />
			</div>
			<div class="desc">{{ t(f.desc) }}</div>
			<div class="preview" :style="{'--tint': f.tint}">
				<div class="window" :class="f.key">
					<template v-if="f.key === 'calendar'">
						<i v-for="n in 21" :key="n" :class="{ev: n % 4 === 1 || n % 7 === 3}" />
					</template>
					<template v-else-if="f.key === 'matrix'">
						<div v-for="(c, q) in ['#e5484d', '#f5a623', '#4772fa', '#2bb673']" :key="q" class="quad">
							<b :style="{background: c}" />
							<span v-for="n in 3" :key="n" />
						</div>
					</template>
					<template v-else-if="f.key === 'habit'">
						<div v-for="(c, r) in ['#4772fa', '#f5a623', '#e5484d', '#2bb673']" :key="r" class="habit-row">
							<b :style="{background: c}" />
							<span v-for="n in 5" :key="n" :class="{done: (n + r) % 3 !== 0}" />
						</div>
					</template>
					<template v-else-if="f.key === 'pomo'">
						<div class="ring">16:36</div>
						<div class="play" />
					</template>
					<template v-else-if="f.key === 'countdown'">
						<div v-for="(n, i) in [83, 204, 210]" :key="i" class="count">
							<b />
							<span>{{ n }}</span>
						</div>
					</template>
					<template v-else>
						<div class="spark">✦</div>
						<span v-for="n in 4" :key="n" class="line" />
					</template>
				</div>
			</div>
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
	border: 3px solid #e3e8fd;
	border-top-color: #4772fa;
	border-right-color: #4772fa;
	font-size: 10px;
	color: #333;
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
</style>
