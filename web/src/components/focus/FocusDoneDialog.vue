<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import {Check, Coffee, Pencil, Timer} from 'lucide-vue-next'
import Modal from '@/components/Modal.vue'
import {fmtMinutes, useFocus} from '@/stores/focus'
import {useData} from '@/stores/data'
import {t} from '@/lib/i18n'

/** The pop-up when a pomo or a break ends on its own (TickTick's focus completion page). */
const focus = useFocus()
const data = useData()
const state = focus.state

const now = ref(Date.now())
let timer: number | undefined
onMounted(() => { timer = window.setInterval(() => { now.value = Date.now() }, 5000) })
onBeforeUnmount(() => clearInterval(timer))

const task = computed(() => state.taskId ? data.tasks.get(state.taskId) : undefined)
const focusedMinutes = computed(() => Math.round((state.last?.duration ?? 0) / 60))

/** Whole minutes since the pomo ended, which "Add extra time" would add. */
const extraMinutes = computed(() => {
	if (!state.last) return 0
	return Math.floor((now.value - new Date(state.last.end_time).getTime()) / 60_000)
})
const extraAdded = ref(false)

async function addExtra() {
	if (await focus.addExtraTime()) {
		extraAdded.value = true
		now.value = Date.now()
	}
}

const editing = ref(false)
const draft = ref(0)
const maxMinutes = computed(() => Math.max(1, Math.floor((state.last?.duration ?? 60) / 60)))

function startEdit() {
	draft.value = focusedMinutes.value
	editing.value = true
}

async function saveEdit() {
	editing.value = false
	const minutes = Math.max(1, Math.min(maxMinutes.value, Math.round(Number(draft.value) || 1)))
	if (minutes !== focusedMinutes.value) await focus.editLastDuration(minutes)
}

function startBreak() {
	focus.resume()
	focus.dismissPrompt()
}

function exit() {
	if (state.prompt === 'pomoDone' && state.phase === 'break') focus.stop()
	focus.dismissPrompt()
}

function nextPomo() {
	focus.startFocus()
}
</script>

<template>
	<Modal :width="380" @close="focus.dismissPrompt()">
		<div class="done">
			<template v-if="state.prompt === 'pomoDone'">
				<div class="badge"><Timer :size="26" /></div>
				<div class="title">{{ t('gotPomo') }}</div>
				<div class="sub">{{ t(state.breakKind === 'long' ? 'pomoDoneLongBreak' : 'pomoDoneBreak') }}</div>
				<div v-if="task" class="task">{{ task.title }}</div>

				<div v-if="state.last" class="duration">
					<template v-if="editing">
						<span class="muted">{{ t('editFocusDurationDesc') }}</span>
						<input
							v-model.number="draft"
							class="input num"
							type="number"
							min="1"
							:max="maxMinutes"
							@keydown.enter="saveEdit"
						>
						<span class="muted">{{ t('minutesUnit') }}</span>
						<button class="icon-btn small" :title="t('save')" @click="saveEdit"><Check :size="15" /></button>
					</template>
					<template v-else>
						<span>{{ t('focusedFor', {time: fmtMinutes(focusedMinutes)}) }}</span>
						<button class="icon-btn small" :title="t('editFocusDuration')" @click="startEdit"><Pencil :size="14" /></button>
					</template>
				</div>

				<button
					v-if="state.last && (extraMinutes >= 1 || extraAdded)"
					class="extra"
					:disabled="extraAdded && extraMinutes < 1"
					:title="t('addExtraTimeHint')"
					@click="addExtra"
				>
					<template v-if="extraAdded && extraMinutes < 1"><Check :size="14" /> {{ t('focusSaved') }}</template>
					<template v-else>{{ t('addExtraTime', {time: fmtMinutes(extraMinutes)}) }}</template>
				</button>

				<div class="actions">
					<button class="btn" @click="exit">{{ t('focusExit') }}</button>
					<button v-if="state.phase === 'break' && !state.running" class="btn primary" @click="startBreak"><Coffee :size="15" /> {{ t('startBreak') }}</button>
					<button v-else class="btn primary" @click="focus.dismissPrompt()">{{ t('ok') }}</button>
				</div>
			</template>

			<template v-else>
				<div class="badge break"><Coffee :size="26" /></div>
				<div class="title">{{ state.phase === 'focus' ? t('breakOverAutoBody') : t('relaxDone') }}</div>
				<div v-if="task" class="task">{{ task.title }}</div>
				<div class="actions">
					<template v-if="state.phase === 'idle'">
						<button class="btn" @click="focus.dismissPrompt()">{{ t('focusExit') }}</button>
						<button class="btn primary" @click="nextPomo"><Timer :size="15" /> {{ t('startPomo') }}</button>
					</template>
					<button v-else class="btn primary" @click="focus.dismissPrompt()">{{ t('ok') }}</button>
				</div>
			</template>
		</div>
	</Modal>
</template>

<style scoped>
.done {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 6px;
	text-align: center;
}

.badge {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 56px;
	height: 56px;
	margin-bottom: 6px;
	border-radius: 50%;
	color: var(--accent);
	background: var(--accent-soft);
}

.badge.break {
	color: var(--success, #2fb36f);
	background: color-mix(in srgb, var(--success, #2fb36f) 16%, transparent);
}

.title {
	font-size: 18px;
	font-weight: 600;
}

.sub {
	font-size: 13px;
	color: var(--text-2);
}

.task {
	max-width: 100%;
	margin-top: 4px;
	overflow: hidden;
	font-size: 14px;
	font-weight: 600;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.duration {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	justify-content: center;
	gap: 6px;
	min-height: 30px;
	font-size: 13px;
}

.muted {
	color: var(--text-3);
}

.num {
	width: 64px;
	height: 28px;
}

.icon-btn.small {
	width: 26px;
	height: 26px;
}

.extra {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	height: 26px;
	padding: 0 10px;
	border-radius: 13px;
	font-size: 12px;
	color: var(--accent);
	background: var(--accent-soft);
}

.extra:disabled {
	cursor: default;
}

.actions {
	display: flex;
	justify-content: center;
	gap: 10px;
	margin-top: 14px;
}

.actions .btn {
	min-width: 110px;
	justify-content: center;
	border-radius: 16px;
}
</style>
