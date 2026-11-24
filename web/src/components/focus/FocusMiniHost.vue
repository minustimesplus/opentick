<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useRoute} from 'vue-router'
import FocusMini from './FocusMini.vue'
import {useFocus} from '@/stores/focus'
import {close as closePopout, popout, popoutRoot} from '@/lib/focusPopout'
import {prefs} from '@/lib/prefs'
import {isMobile} from '@/lib/viewport'

/**
 * Places the mini timer: inside the pop-out window while one is open,
 * otherwise as a draggable card over every view but Focus itself while a
 * session runs (TickTick's minimized focus bar).
 */
const focus = useFocus()
const route = useRoute()
const state = focus.state

/** Session the user hid the dock for; it comes back with the next one. */
const hiddenFor = ref<number | null>(null)

const docked = computed(() => !popout.value
	&& prefs.features.pomo
	&& prefs.focusMini
	&& state.phase !== 'idle'
	&& !route.path.startsWith('/focus')
	&& hiddenFor.value !== state.startedAt)

// Turning the feature off closes the mini window too.
watch(() => prefs.features.pomo, on => {
	if (!on) closePopout()
})

// --- dragging the dock ---

const el = ref<HTMLElement | null>(null)
const pos = computed(() => {
	const {right, bottom} = prefs.focusMiniPos
	return {right: `${Math.max(0, right)}px`, bottom: `${Math.max(0, bottom)}px`}
})

function onPointerDown(e: PointerEvent) {
	if (e.button !== 0 || (e.target as HTMLElement).closest('button, input')) return
	const box = el.value?.getBoundingClientRect()
	if (!box) return
	const start = {x: e.clientX, y: e.clientY, ...prefs.focusMiniPos}
	const move = (ev: PointerEvent) => {
		prefs.focusMiniPos = {
			right: Math.min(window.innerWidth - box.width, Math.max(0, start.right - (ev.clientX - start.x))),
			bottom: Math.min(window.innerHeight - box.height, Math.max(0, start.bottom - (ev.clientY - start.y))),
		}
	}
	const up = () => {
		window.removeEventListener('pointermove', move)
		window.removeEventListener('pointerup', up)
	}
	window.addEventListener('pointermove', move)
	window.addEventListener('pointerup', up)
}
</script>

<template>
	<Teleport v-if="popout && popoutRoot && prefs.features.pomo" :to="popoutRoot">
		<FocusMini popped />
	</Teleport>
	<div
		v-else-if="docked"
		ref="el"
		class="focus-dock"
		:class="{mobile: isMobile}"
		:style="pos"
		@pointerdown="onPointerDown"
	>
		<FocusMini @hide="hiddenFor = state.startedAt" />
	</div>
</template>

<style scoped>
.focus-dock {
	position: fixed;
	z-index: 800;
	cursor: grab;
	touch-action: none;
}

.focus-dock:active {
	cursor: grabbing;
}

.focus-dock.mobile {
	right: 8px !important;
	bottom: 72px !important;
}
</style>
