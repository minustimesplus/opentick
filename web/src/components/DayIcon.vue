<script setup lang="ts">
defineProps<{label: string | number, size?: number}>()
</script>

<template>
	<svg :width="size ?? 18" :height="size ?? 18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
		<rect x="3" y="3" width="18" height="18" rx="4" />
		<text x="12" y="16.1" text-anchor="middle" font-size="9.5" font-weight="700" fill="currentColor" stroke="none" font-family="system-ui, sans-serif">{{ label }}</text>
	</svg>
</template>
