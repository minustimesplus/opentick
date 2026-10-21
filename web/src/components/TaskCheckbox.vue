<script setup lang="ts">
import {computed} from 'vue'
import {level, PRIORITY_COLORS} from '@/lib/tasks'

const props = withDefaults(defineProps<{done: boolean, priority?: number, size?: number, onColor?: boolean}>(), {size: 16})
const emit = defineEmits<{toggle: []}>()

const color = computed(() => props.onColor ? 'currentColor' : PRIORITY_COLORS[level(props.priority)])
const fill = computed(() => !props.onColor && level(props.priority) > 0 ? `color-mix(in srgb, ${color.value} 14%, transparent)` : 'transparent')
</script>

<template>
	<button
		class="cb"
		:class="{done}"
		:style="{width: size + 'px', height: size + 'px', '--c': color, '--f': fill}"
		role="checkbox"
		:aria-checked="done"
		@click.stop="emit('toggle')"
		@mousedown.stop
		@pointerdown.stop
	>
		<svg v-if="done" viewBox="0 0 16 16"><path d="M4 8.2l2.6 2.6L12 5.4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" /></svg>
	</button>
</template>

<style scoped>
.cb {
	flex-shrink: 0;
	display: inline-flex;
	align-items: center;
	justify-content: center;
	border: 1.5px solid var(--c);
	background: var(--f);
	border-radius: 3px;
	color: #fff;
	transition: background 0.12s;
}

@media (hover: hover) {
	.cb:hover {
		background: color-mix(in srgb, var(--c) 28%, transparent);
	}
}

/* Fingertip-sized hit area around the small box. */
@media (pointer: coarse) {
	.cb {
		position: relative;
	}

	.cb::before {
		content: '';
		position: absolute;
		inset: -12px;
	}
}

.cb.done {
	border-color: var(--text-4);
	background: var(--text-4);
}

.cb svg {
	width: 100%;
	height: 100%;
}
</style>
