<script setup lang="ts">
import {computed, onBeforeUnmount, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {Maximize2, Pause, PictureInPicture2, Play, SkipForward, Square, X} from 'lucide-vue-next'
import {useFocus} from '@/stores/focus'
import {useData} from '@/stores/data'
import {close as closePopout, openPopout} from '@/lib/focusPopout'
import {prefs} from '@/lib/prefs'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

/**
 * The compact focus timer: just the time. Hovering it shows the status, the
 * task and the controls. Used as the floating dock over other views, and as
 * the whole content of the always-on-top mini window (`popped`).
 */
const props = defineProps<{popped?: boolean}>()
const emit = defineEmits<{hide: []}>()

const focus = useFocus()
const data = useData()
const router = useRouter()
const state = focus.state
const desktop = window.ttDesktop

const root = ref<HTMLElement | null>(null)
const hover = ref(false)
/** The dock's controls open below it when it sits near the top of the window. */
const below = ref(false)

const task = computed(() => state.taskId ? data.tasks.get(state.taskId) : undefined)

const status = computed(() => {
	if (state.prompt === 'pomoDone') return t('gotPomo')
	if (state.phase === 'break') return t(state.running ? 'focusRelaxing' : (state.breakKind === 'long' ? 'longBreak' : 'shortBreak'))
	if (state.phase === 'focus') return t(state.running ? 'focusing' : 'focusPaused')
	return t(state.mode === 'pomo' ? 'pomo' : 'stopwatch')
})

const clock = computed(() => state.phase === 'idle' && focus.phaseLength !== null
	? `${String(Math.round(focus.phaseLength / 60_000)).padStart(2, '0')}:00`
	: focus.display)

const opacity = computed(() => hover.value ? 1 : Math.min(1, Math.max(0.1, prefs.focusMiniOpacity)))

/** The dock is styled here; the mini window gets its opacity from the desktop app's window. */
const rootStyle = computed(() => props.popped
	? {}
	: {'--s': String(Math.min(3, Math.max(0.5, prefs.focusMiniScale))), opacity: String(opacity.value)})

if (props.popped) {
	watch(opacity, value => desktop?.setFocusMiniOpacity?.(value), {immediate: true})
}

function onEnter() {
	hover.value = true
	if (!props.popped && root.value) below.value = root.value.getBoundingClientRect().top < 110
}

function openMain() {
	window.focus()
	desktop?.focusWindow?.()
	router.push('/focus')
}

function end() {
	focus.requestEnd()
	// The confirmation opens in the main window.
	if (props.popped && focus.ending) {
		window.focus()
		desktop?.focusWindow?.()
	}
}

async function togglePopout() {
	if (props.popped) {
		closePopout()
		return
	}
	if (!await openPopout()) toast(t('miniWindowBlocked'))
}

// --- moving the desktop mini window (it has no title bar) ---

let dragging = false

function onPointerDown(e: PointerEvent) {
	if (!props.popped || !desktop?.dragFocusMini || e.button !== 0) return
	if ((e.target as HTMLElement).closest('button')) return
	dragging = true
	;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
	desktop.dragFocusMini('start')
}

function onPointerMove() {
	if (dragging) desktop?.dragFocusMini?.('move')
}

function onPointerUp() {
	if (!dragging) return
	dragging = false
	desktop?.dragFocusMini?.('end')
}

onBeforeUnmount(onPointerUp)
</script>

<template>
	<div
		ref="root"
		class="focus-mini"
		:class="{popped, hover, below, break: state.phase === 'break', paused: state.phase !== 'idle' && !state.running}"
		:style="rootStyle"
		@pointerenter="onEnter"
		@pointerleave="hover = false"
		@pointerdown="onPointerDown"
		@pointermove="onPointerMove"
		@pointerup="onPointerUp"
		@pointercancel="onPointerUp"
	>
		<div class="clock" role="timer" aria-live="off" :aria-label="status">{{ clock }}</div>
		<div class="bar"><div class="fill" :style="{width: (focus.progress * 100) + '%'}" /></div>

		<div class="overlay">
			<div class="panel">
				<div class="info">
					<span class="status">{{ status }}</span>
					<span v-if="task" class="task" :title="task.title">{{ task.title }}</span>
				</div>
				<div class="buttons">
					<template v-if="state.phase === 'idle'">
						<button class="ctl primary" :title="t('focusStart')" @click="focus.startFocus()"><Play /></button>
					</template>
					<template v-else-if="state.phase === 'focus'">
						<button v-if="state.running" class="ctl primary" :title="t('focusPause')" @click="focus.pause()"><Pause /></button>
						<button v-else class="ctl primary" :title="t('focusContinue')" @click="focus.resume()"><Play /></button>
						<button class="ctl" :title="t('focusEnd')" @click="end"><Square /></button>
					</template>
					<template v-else>
						<button v-if="!state.running" class="ctl primary" :title="t('startBreak')" @click="focus.resume()"><Play /></button>
						<button v-else class="ctl primary" :title="t('focusPause')" @click="focus.pause()"><Pause /></button>
						<button class="ctl" :title="t('skipBreak')" @click="focus.stop()"><SkipForward /></button>
					</template>
					<button class="ctl plain" :title="t('openFocusView')" @click="openMain"><Maximize2 /></button>
					<button v-if="popped" class="ctl plain" :title="t('closeMiniWindow')" @click="togglePopout"><X /></button>
					<template v-else>
						<button class="ctl plain" :title="t('openMiniWindow')" @click="togglePopout"><PictureInPicture2 /></button>
						<button class="ctl plain" :title="t('hideMiniTimer')" @click="emit('hide')"><X /></button>
					</template>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>
.focus-mini {
	--s: 1;
	--ring-color: var(--accent);
	position: relative;
	display: inline-flex;
	flex-direction: column;
	align-items: stretch;
	padding: calc(3px * var(--s)) calc(10px * var(--s)) calc(4px * var(--s));
	border-radius: calc(10px * var(--s));
	border: 1px solid var(--border-strong);
	background: var(--bg-elev);
	box-shadow: var(--shadow);
	user-select: none;
	transition: opacity 0.2s;
}

.focus-mini.break {
	--ring-color: var(--success, #2fb36f);
}

.clock {
	font-size: calc(26px * var(--s));
	font-weight: 400;
	font-variant-numeric: tabular-nums;
	line-height: 1.15;
	text-align: center;
	white-space: nowrap;
}

.paused .clock {
	color: var(--text-3);
}

.bar {
	height: max(2px, calc(2px * var(--s)));
	border-radius: 1px;
	background: var(--bg-panel);
	overflow: hidden;
}

.fill {
	height: 100%;
	background: var(--ring-color);
	transition: width 0.9s linear;
}

/* --- hover: status, task and controls --- */

.overlay {
	position: absolute;
	right: 0;
	bottom: 100%;
	padding-bottom: 6px;
	display: none;
	cursor: default;
}

.below .overlay {
	top: 100%;
	bottom: auto;
	padding: 6px 0 0;
}

.hover .overlay,
.focus-mini:has(:focus-visible) .overlay {
	display: block;
}

.panel {
	display: flex;
	flex-direction: column;
	gap: 6px;
	width: max-content;
	max-width: 260px;
	padding: 8px 10px;
	border-radius: 10px;
	border: 1px solid var(--border-strong);
	background: var(--bg-elev);
	box-shadow: var(--shadow);
}

.info {
	display: flex;
	flex-direction: column;
	min-width: 0;
}

.status {
	font-size: 12px;
	color: var(--text-3);
}

.task {
	overflow: hidden;
	font-size: 13px;
	font-weight: 600;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.buttons {
	display: flex;
	align-items: center;
	gap: 6px;
}

.ctl {
	--size: 28px;
	display: inline-flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	width: var(--size);
	height: var(--size);
	border-radius: 50%;
	border: 1px solid var(--border-strong);
	color: var(--text-2);
}

.ctl svg {
	width: 55%;
	height: 55%;
}

.ctl:hover {
	background: var(--bg-hover);
	color: var(--text);
}

.ctl.primary {
	border-color: var(--ring-color);
	background: var(--ring-color);
	color: #fff;
}

.ctl.primary:hover {
	filter: brightness(1.08);
}

.ctl.plain {
	border-color: transparent;
	color: var(--text-3);
}

/* --- the mini window: the clock fills it, the controls cover it on hover --- */

.focus-mini.popped {
	display: flex;
	justify-content: center;
	width: 100vw;
	height: 100vh;
	padding: 0 4vw;
	border: none;
	border-radius: 0;
	box-shadow: none;
	overflow: hidden;
}

.popped .clock {
	font-size: min(66vh, 27vw);
	line-height: 1;
}

.popped .bar {
	position: absolute;
	right: 0;
	bottom: 0;
	left: 0;
	height: max(2px, 3vh);
	border-radius: 0;
}

.popped .overlay {
	inset: 0;
	padding: 0;
	background: color-mix(in srgb, var(--bg-elev) 90%, transparent);
}

.popped.hover .overlay,
.popped:has(:focus-visible) .overlay {
	display: flex;
	align-items: center;
	justify-content: center;
}

.popped .panel {
	width: auto;
	max-width: 100%;
	padding: 0 2vw;
	border: none;
	background: none;
	box-shadow: none;
	align-items: center;
}

.popped .buttons {
	gap: min(6px, 2vw);
}

.popped .ctl {
	--size: clamp(16px, min(42vh, 15vw), 34px);
}

/* Status and task only when the window has room for them. */
.popped .info {
	display: none;
	align-items: center;
	max-width: 92vw;
}

@media (min-height: 96px) and (min-width: 150px) {
	.popped .info {
		display: flex;
	}

	.popped .ctl {
		--size: clamp(16px, 28vh, 34px);
	}
}
</style>
