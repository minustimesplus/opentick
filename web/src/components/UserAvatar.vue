<script setup lang="ts">
import {computed} from 'vue'
import {useData} from '@/stores/data'

withDefaults(defineProps<{size?: number}>(), {size: 30})

const data = useData()

const initial = computed(() => (data.user?.name || data.user?.username || '?').charAt(0).toUpperCase())
</script>

<template>
	<span class="user-avatar" :style="{width: `${size}px`, height: `${size}px`, fontSize: `${Math.round(size * 0.47)}px`}">
		<img v-if="data.avatarUrl" :src="data.avatarUrl" alt="">
		<template v-else>{{ initial }}</template>
	</span>
</template>

<style scoped>
.user-avatar {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	overflow: hidden;
	border-radius: 50%;
	background: linear-gradient(135deg, #4772fa, #8b5cf6);
	color: #fff;
	font-weight: 600;
}

.user-avatar img {
	width: 100%;
	height: 100%;
	object-fit: cover;
}
</style>
