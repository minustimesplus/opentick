<script lang="ts">
import {api} from '@/api/client'

// One request per contact and photo version for the whole session; null when
// the image failed. Bounded, as the list scrolls through hundreds of photos.
const MAX_CACHED = 400
const cache = new Map<string, Promise<string | null>>()

function load(id: number, version: number): Promise<string | null> {
	const key = `${id}:${version}`
	let p = cache.get(key)
	if (!p) {
		p = api<Blob>('GET', `/contacts/${id}/photo`, {blob: true})
			.then(b => URL.createObjectURL(b))
			.catch(() => null)
		cache.set(key, p)
		if (cache.size > MAX_CACHED) {
			const [oldest, url] = cache.entries().next().value!
			cache.delete(oldest)
			url.then(u => u && URL.revokeObjectURL(u))
		}
	}
	return p
}
</script>

<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {initials, nameColor} from '@/lib/people'
import {usePeople} from '@/stores/people'

const props = withDefaults(defineProps<{
	contact: {id: number, name: string, has_photo: boolean}
	size?: number
}>(), {size: 36})

const people = usePeople()
const src = ref<string | null>(null)
const version = computed(() => people.photoVersions[props.contact.id] ?? 0)

watch(() => [props.contact.id, props.contact.has_photo, version.value] as const, async ([id, hasPhoto, v]) => {
	src.value = null
	if (!hasPhoto) return
	const url = await load(id, v)
	// The row may show someone else by the time the photo arrives.
	if (props.contact.id === id && version.value === v) src.value = url
}, {immediate: true})
</script>

<template>
	<span
		class="contact-avatar"
		:style="{
			width: `${size}px`,
			height: `${size}px`,
			fontSize: `${Math.round(size * 0.38)}px`,
			background: src ? 'var(--bg-active)' : nameColor(contact.name),
		}"
	>
		<img v-if="src" :src="src" alt="">
		<template v-else>{{ initials(contact.name) }}</template>
	</span>
</template>

<style scoped>
.contact-avatar {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	overflow: hidden;
	border-radius: 50%;
	color: #fff;
	font-weight: 600;
	letter-spacing: 0.02em;
	user-select: none;
}

.contact-avatar img {
	width: 100%;
	height: 100%;
	object-fit: cover;
}
</style>
