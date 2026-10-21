<script setup lang="ts">
import {onBeforeUnmount, onMounted} from 'vue'

withDefaults(defineProps<{width?: number}>(), {width: 420})
const emit = defineEmits<{close: []}>()
function onKey(e: KeyboardEvent) {
	if (e.key === 'Escape') emit('close')
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
	<Teleport to="body">
		<div class="overlay" @mousedown.self="emit('close')">
			<div class="dialog" :style="{width: `min(${width}px, calc(100vw - 32px))`}">
				<slot />
			</div>
		</div>
	</Teleport>
</template>

<style scoped>
.overlay {
	position: fixed;
	inset: 0;
	z-index: 900;
	display: flex;
	align-items: center;
	justify-content: center;
	background: rgba(0, 0, 0, 0.45);
}

.dialog {
	padding: 20px;
	border-radius: 12px;
	background: var(--bg-elev);
	box-shadow: var(--shadow);
}
</style>
