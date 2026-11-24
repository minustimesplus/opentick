<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {ChevronLeft, ChevronRight, Minimize2, Pause, Play, SkipForward, Square} from 'lucide-vue-next'
import FocusEndPrompt from './FocusEndPrompt.vue'
import {type EarlyEnd, useFocus} from '@/stores/focus'
import {useData} from '@/stores/data'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'

/** Full-screen focus clock; a click or mouse move shows the controls for a few seconds. */
const emit = defineEmits<{close: []}>()
const focus = useFocus()
const data = useData()
const state = focus.state

const root = ref<HTMLElement | null>(null)
const task = computed(() => state.taskId ? data.tasks.get(state.taskId) : undefined)

const clock = computed(() => state.phase === 'idle' && focus.phaseLength !== null
	? `${String(Math.round(focus.phaseLength / 60_000)).padStart(2, '0')}:00`
	: focus.display)

/** The clock split into flip cards: digit groups between the colons. */
const groups = computed(() => clock.value.split(':'))

const status = computed(() => {
	if (state.phase === 'break') return t('focusRelaxing')
	if (state.phase === 'focus') return t(state.running ? 'focusing' : 'focusPaused')
	return t(state.mode === 'pomo' ? 'pomo' : 'stopwatch')
})

const showControls = ref(true)
let hideTimer: number | undefined
function poke() {
	showControls.value = true
	clearTimeout(hideTimer)
	hideTimer = window.setTimeout(() => { showControls.value = false }, 3000)
}

const ending = ref<EarlyEnd | null>(null)
watch(() => state.phase, () => { ending.value = null })

function end() {
	const kind = focus.earlyEnd()
	if (kind) ending.value = kind
	else focus.stop()
}

function switchStyle() {
	prefs.focusFullscreenStyle = prefs.focusFullscreenStyle === 'digital' ? 'flip' : 'digital'
}

function exit() {
	if (document.fullscreenElement) document.exitFullscreen().catch(() => {})
	emit('close')
}

function onFullscreenChange() {
	if (!document.fullscreenElement) emit('close')
}

function onKey(e: KeyboardEvent) {
	if (e.key === 'Escape') exit()
	else if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') switchStyle()
	else if (e.key === ' ') {
		e.preventDefault()
		focus.toggle()
	}
}

onMounted(() => {
	poke()
	window.addEventListener('keydown', onKey)
	// Without the Fullscreen API it still covers the window.
	root.value?.requestFullscreen?.().then(() => {
		document.addEventListener('fullscreenchange', onFullscreenChange)
	}).catch(() => {})
})

onBeforeUnmount(() => {
	clearTimeout(hideTimer)
	window.removeEventListener('keydown', onKey)
	document.removeEventListener('fullscreenchange', onFullscreenChange)
	if (document.fullscreenElement) document.exitFullscreen().catch(() => {})
})
</script>

<template>
	<Teleport to="body">
		<div ref="root" class="fullscreen" :class="{break: state.phase === 'break', idle: !showControls}" @mousemove="poke" @click="poke">
			<div class="center">
				<div v-if="task" class="task">{{ task.title }}</div>
				<div v-if="prefs.focusFullscreenStyle === 'flip'" class="flip" role="timer">
					<template v-for="(g, i) in groups" :key="i">
						<span v-if="i" class="colon">:</span>
						<span class="card">{{ g }}</span>
					</template>
				</div>
				<div v-else class="digital" role="timer">{{ clock }}</div>
				<div class="status">{{ status }}</div>

				<div v-if="ending" class="prompt" @click.stop>
					<FocusEndPrompt :kind="ending" @close="ending = null" />
				</div>
			</div>

			<div class="controls" :class="{hidden: !showControls && !ending}">
				<button class="ctl" :title="t('switchClockStyle')" @click.stop="switchStyle">
					<ChevronLeft :size="18" /><ChevronRight :size="18" />
				</button>
				<template v-if="state.phase === 'idle'">
					<button class="ctl primary" :title="t('focusStart')" @click.stop="focus.startFocus()"><Play :size="22" /></button>
				</template>
				<template v-else>
					<button v-if="state.running" class="ctl primary" :title="t('focusPause')" @click.stop="focus.pause()"><Pause :size="22" /></button>
					<button v-else class="ctl primary" :title="state.phase === 'break' ? t('startBreak') : t('focusContinue')" @click.stop="focus.resume()"><Play :size="22" /></button>
					<button v-if="state.phase === 'focus'" class="ctl" :title="t('focusEnd')" @click.stop="end"><Square :size="18" /></button>
					<button v-else class="ctl" :title="t('skipBreak')" @click.stop="focus.stop()"><SkipForward :size="18" /></button>
				</template>
				<button class="ctl" :title="t('exitFullScreen')" @click.stop="exit"><Minimize2 :size="18" /></button>
			</div>
		</div>
	</Teleport>
</template>

<style scoped>
.fullscreen {
	--ring-color: var(--accent);
	position: fixed;
	inset: 0;
	z-index: 950;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	background: #101010;
	color: #f2f2f2;
	-webkit-app-region: no-drag;
}

.fullscreen.break {
	--ring-color: var(--success, #2fb36f);
}

.fullscreen.idle {
	cursor: none;
}

.center {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 18px;
}

.task {
	max-width: 80vw;
	overflow: hidden;
	font-size: clamp(16px, 2.4vw, 26px);
	color: #bdbdbd;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.digital {
	font-size: clamp(72px, 18vw, 260px);
	font-weight: 200;
	font-variant-numeric: tabular-nums;
	line-height: 1;
	letter-spacing: 2px;
}

.flip {
	display: flex;
	align-items: center;
	gap: clamp(8px, 1.6vw, 20px);
}

.card {
	position: relative;
	padding: 0.08em 0.14em;
	border-radius: 0.12em;
	background: linear-gradient(#262626 0 49.6%, #1a1a1a 50.4% 100%);
	box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
	font-size: clamp(64px, 15vw, 220px);
	font-weight: 600;
	font-variant-numeric: tabular-nums;
	line-height: 1;
}

.card::after {
	position: absolute;
	inset: 50% 0 auto;
	height: 2px;
	background: #101010;
	content: '';
}

.colon {
	font-size: clamp(40px, 8vw, 120px);
	color: #555;
}

.status {
	font-size: clamp(14px, 1.6vw, 18px);
	color: var(--ring-color);
	letter-spacing: 1px;
}

.prompt {
	padding: 18px 20px;
	border-radius: 12px;
	background: var(--bg-elev);
	color: var(--text);
}

.controls {
	position: absolute;
	bottom: 8vh;
	display: flex;
	align-items: center;
	gap: 18px;
	transition: opacity 0.3s;
}

.controls.hidden {
	opacity: 0;
	pointer-events: none;
}

.ctl {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 52px;
	height: 52px;
	border-radius: 50%;
	border: 1px solid #444;
	color: #ddd;
}

.ctl:hover {
	background: rgba(255, 255, 255, 0.08);
}

.ctl.primary {
	width: 64px;
	height: 64px;
	border-color: var(--ring-color);
	background: var(--ring-color);
	color: #fff;
}
</style>
