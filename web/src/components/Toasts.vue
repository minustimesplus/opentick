<script setup lang="ts">
import {dismiss, toasts} from '@/lib/toast'
</script>

<template>
	<div class="toasts">
		<TransitionGroup name="toast">
			<div v-for="tt in toasts" :key="tt.id" class="toast">
				<span>{{ tt.message }}</span>
				<button v-if="tt.action" class="action" @click="tt.action.run(); dismiss(tt.id)">{{ tt.action.label }}</button>
			</div>
		</TransitionGroup>
	</div>
</template>

<style scoped>
.toasts {
	position: fixed;
	bottom: 24px;
	left: 50%;
	transform: translateX(-50%);
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 8px;
	z-index: 2000;
	pointer-events: none;
}

.toast {
	display: flex;
	align-items: center;
	gap: 16px;
	padding: 10px 16px;
	border-radius: 8px;
	background: #333;
	color: #fff;
	box-shadow: var(--shadow);
	pointer-events: auto;
	max-width: 520px;
}

/* Phones: clear the tab bar. */
@media (max-width: 767px) {
	.toasts {
		bottom: calc(66px + env(safe-area-inset-bottom));
		width: calc(100vw - 24px);
	}

	.toast {
		max-width: 100%;
	}
}

.action {
	color: #8fb0ff;
	font-weight: 600;
}

.toast-enter-active,
.toast-leave-active {
	transition: all 0.2s;
}

.toast-enter-from,
.toast-leave-to {
	opacity: 0;
	transform: translateY(8px);
}
</style>
