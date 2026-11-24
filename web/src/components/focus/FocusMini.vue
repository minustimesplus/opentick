<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {Maximize2, Pause, PictureInPicture2, Play, SkipForward, Square, X} from 'lucide-vue-next'
import FocusEndPrompt from './FocusEndPrompt.vue'
import {type EarlyEnd, useFocus} from '@/stores/focus'
import {useData} from '@/stores/data'
import {close as closePopout, openPopout, popout} from '@/lib/focusPopout'
import {t} from '@/lib/i18n'
import {toast} from '@/lib/toast'

/**
 * The compact focus timer: the floating dock over other views, and the
 * content of the always-on-top mini window (`popped`).
 */
const props = defineProps<{popped?: boolean}>()
const emit = defineEmits<{hide: []}>()

const focus = useFocus()
const data = useData()
const router = useRouter()
const state = focus.state

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

const ending = ref<EarlyEnd | null>(null)
watch(() => state.phase, () => { ending.value = null })

function end() {
	const kind = focus.earlyEnd()
	if (kind) ending.value = kind
	else focus.stop()
}

function openMain() {
	window.focus()
	window.ttDesktop?.focusWindow?.()
	router.push('/focus')
}

async function togglePopout() {
	if (props.popped) {
		closePopout()
		return
	}
	if (!await openPopout()) toast(t('miniWindowBlocked'))
}
</script>

<template>
	<div class="focus-mini" :class="{popped, break: state.phase === 'break', paused: state.phase !== 'idle' && !state.running}">
		<div class="top">
			<div class="labels">
				<div class="status">{{ status }}</div>
				<div v-if="task" class="task" :title="task.title">{{ task.title }}</div>
			</div>
			<div class="tools">
				<button class="icon-btn tiny" :title="t('openFocusView')" @click="openMain"><Maximize2 :size="14" /></button>
				<button class="icon-btn tiny" :title="popped ? t('closeMiniWindow') : t('openMiniWindow')" @click="togglePopout">
					<PictureInPicture2 :size="14" />
				</button>
				<button v-if="!popped" class="icon-btn tiny" :title="t('hideMiniTimer')" @click="emit('hide')"><X :size="14" /></button>
			</div>
		</div>

		<template v-if="ending">
			<FocusEndPrompt :kind="ending" compact @close="ending = null" />
		</template>
		<template v-else>
			<div class="clock" role="timer" aria-live="off">{{ clock }}</div>
			<div class="bar"><div class="fill" :style="{width: (focus.progress * 100) + '%'}" /></div>

			<div class="controls">
				<template v-if="state.phase === 'idle'">
					<button class="ctl primary" :title="t('focusStart')" @click="focus.startFocus()"><Play :size="16" /></button>
				</template>
				<template v-else-if="state.phase === 'focus'">
					<button v-if="state.running" class="ctl primary" :title="t('focusPause')" @click="focus.pause()"><Pause :size="16" /></button>
					<button v-else class="ctl primary" :title="t('focusContinue')" @click="focus.resume()"><Play :size="16" /></button>
					<button class="ctl" :title="t('focusEnd')" @click="end"><Square :size="14" /></button>
				</template>
				<template v-else>
					<button v-if="!state.running" class="ctl primary" :title="t('startBreak')" @click="focus.resume()"><Play :size="16" /></button>
					<button v-else class="ctl primary" :title="t('focusPause')" @click="focus.pause()"><Pause :size="16" /></button>
					<button class="ctl" :title="t('skipBreak')" @click="focus.stop()"><SkipForward :size="15" /></button>
				</template>
			</div>
		</template>
	</div>
</template>

<style scoped>
.focus-mini {
	--ring-color: var(--accent);
	display: flex;
	flex-direction: column;
	gap: 6px;
	width: 260px;
	padding: 10px 12px 12px;
	border-radius: 12px;
	border: 1px solid var(--border-strong);
	background: var(--bg-elev);
	box-shadow: var(--shadow);
	user-select: none;
}

.focus-mini.break {
	--ring-color: var(--success, #2fb36f);
}

/* The mini window is all widget: the frameless desktop window is dragged by it. */
.focus-mini.popped {
	width: 100%;
	height: 100vh;
	border: none;
	border-radius: 0;
	box-shadow: none;
	justify-content: center;
	-webkit-app-region: drag;
}

.focus-mini.popped button {
	-webkit-app-region: no-drag;
}

.top {
	display: flex;
	align-items: flex-start;
	gap: 6px;
}

.labels {
	flex: 1;
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

.tools {
	display: flex;
	gap: 2px;
	flex-shrink: 0;
}

.icon-btn.tiny {
	width: 24px;
	height: 24px;
}

.clock {
	font-size: 38px;
	font-weight: 300;
	font-variant-numeric: tabular-nums;
	line-height: 1.1;
	text-align: center;
}

.popped .clock {
	font-size: clamp(30px, 22vw, 64px);
}

.bar {
	height: 4px;
	border-radius: 2px;
	background: var(--bg-panel);
	overflow: hidden;
}

.fill {
	height: 100%;
	background: var(--ring-color);
	transition: width 0.9s linear;
}

.paused .fill {
	opacity: 0.5;
}

.controls {
	display: flex;
	justify-content: center;
	gap: 10px;
	margin-top: 2px;
}

.ctl {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 34px;
	height: 34px;
	border-radius: 50%;
	border: 1px solid var(--border-strong);
	color: var(--text-2);
}

.ctl:hover {
	background: var(--bg-hover);
}

.ctl.primary {
	border-color: var(--ring-color);
	background: var(--ring-color);
	color: #fff;
}

.ctl.primary:hover {
	filter: brightness(1.08);
}
</style>
