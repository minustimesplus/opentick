<script setup lang="ts">
import {onBeforeUnmount, onMounted} from 'vue'
import TaskDetail from './TaskDetail.vue'
import {closeTaskDrawer, openTaskDrawer} from '@/lib/ui'
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
				<TaskDetail :key="taskId" :task-id="taskId" closable @close="closeTaskDrawer()" @deleted="closeTaskDrawer()" @open="openTaskDrawer" />
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
</style>
