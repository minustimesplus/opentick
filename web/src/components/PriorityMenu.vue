<script setup lang="ts">
import {Flag} from 'lucide-vue-next'
import {level, PRIORITY_COLORS, priorityName, type Level} from '@/lib/tasks'

const props = defineProps<{value?: number}>()
const emit = defineEmits<{select: [Level]}>()
const levels: Level[] = [3, 2, 1, 0]
</script>

<template>
	<div class="menu">
		<button v-for="l in levels" :key="l" class="menu-item" :class="{active: level(props.value) === l}" @click="emit('select', l)">
			<Flag :size="16" :color="PRIORITY_COLORS[l]" :fill="l ? PRIORITY_COLORS[l] : 'none'" />
			{{ priorityName(l) }}
		</button>
	</div>
</template>
