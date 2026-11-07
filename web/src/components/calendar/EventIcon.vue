<script setup lang="ts">
// Marks a read-only event from a calendar subscription, in the slot where a
// task shows its checkbox.
withDefaults(defineProps<{size?: number}>(), {size: 13})
</script>

<template>
	<svg class="event-icon" :width="size" :height="size" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" aria-hidden="true">
		<rect x="2" y="2.5" width="12" height="11.5" rx="2" />
		<path d="M5 1.5v2M11 1.5v2M2 6h12" />
		<path d="M6.5 8.2l-.6 4M9.6 8.2l-.6 4M5.2 9.4h5.4M4.9 11h5.4" stroke-width="1" />
	</svg>
</template>

<style scoped>
.event-icon {
	flex-shrink: 0;
	opacity: 0.6;
}
</style>
