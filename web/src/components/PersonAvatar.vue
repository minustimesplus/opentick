<script lang="ts">
import {api} from '@/api/client'

// One request per username for the whole session; null when the image failed.
const cache = new Map<string, Promise<string | null>>()

function load(username: string): Promise<string | null> {
	let p = cache.get(username)
	if (!p) {
		p = api<Blob>('GET', `/avatar/${encodeURIComponent(username)}`, {query: {size: 64}, blob: true})
			.then(b => URL.createObjectURL(b))
			.catch(() => null)
		cache.set(username, p)
	}
	return p
}
</script>

<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import type {User} from '@/api/types'
import {displayName} from '@/lib/tasks'

const props = withDefaults(defineProps<{user?: User | null, size?: number}>(), {size: 24})

const src = ref<string | null>(null)
watch(() => props.user?.username, async name => {
	src.value = null
	if (name) src.value = await load(name)
}, {immediate: true})

const initial = computed(() => (props.user?.name || props.user?.username || '?').charAt(0).toUpperCase())
</script>

<template>
	<span class="person" :title="displayName(user)" :style="{width: `${size}px`, height: `${size}px`, fontSize: `${Math.round(size * 0.47)}px`}">
		<img v-if="src" :src="src" alt="">
		<template v-else>{{ initial }}</template>
	</span>
</template>

<style scoped>
.person {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	overflow: hidden;
	border-radius: 50%;
	background: var(--bg-active);
	color: var(--text-2);
	font-weight: 600;
}

.person img {
	width: 100%;
	height: 100%;
	object-fit: cover;
}
</style>
