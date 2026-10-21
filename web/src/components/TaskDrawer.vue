<script setup lang="ts">
import {onBeforeUnmount, onMounted} from 'vue'
import TaskDetail from './TaskDetail.vue'
import {closeTaskDrawer} from '@/lib/ui'
import {useData} from '@/stores/data'

const props = defineProps<{taskId: number}>()
const data = useData()

function onKey(e: KeyboardEvent) {
	if (e.key === 'Escape' && !document.querySelector('.popover')) closeTaskDrawer()
}

onMounted(() => {
	if (!data.tasks.has(props.taskId)) closeTaskDrawer()
	window.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
	<Teleport to="body">
		<div class="backdrop" @mousedown.self="closeTaskDrawer()">
			<aside class="drawer">
				<TaskDetail :task-id="taskId" closable @close="closeTaskDrawer()" @deleted="closeTaskDrawer()" />
			</aside>
		</div>
	</Teleport>
</template>

<style scoped>
.backdrop {
	position: fixed;
	inset: 0;
	z-index: 500;
}

.drawer {
	position: absolute;
	top: 12px;
	right: 12px;
	bottom: 12px;
	width: min(460px, calc(100vw - 24px));
	background: var(--bg-elev);
	border-radius: 12px;
	box-shadow: var(--shadow);
	overflow: hidden;
	animation: slide 0.16s ease-out;
}

@keyframes slide {
	from {
		transform: translateX(24px);
		opacity: 0;
	}
}

/* Phones: a full-screen sheet that rises from the bottom. */
@media (max-width: 767px) {
	.drawer {
		inset: 0;
		width: auto;
		padding: env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left);
		border-radius: 0;
		animation-name: rise;
	}
}

@keyframes rise {
	from {
		transform: translateY(32px);
		opacity: 0;
	}
}
</style>
