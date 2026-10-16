<script setup lang="ts">
import {nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'

export type Anchor = HTMLElement | {x: number, y: number, w?: number, h?: number}

const props = withDefaults(defineProps<{
	anchor: Anchor | null
	placement?: 'bottom-start' | 'bottom-end' | 'right-start' | 'left-start' | 'top-start'
	offset?: number
}>(), {placement: 'bottom-start', offset: 6})

const emit = defineEmits<{close: []}>()

const el = ref<HTMLElement | null>(null)
const style = ref<Record<string, string>>({left: '-9999px', top: '-9999px'})

function rectOf(a: Anchor) {
	if (a instanceof HTMLElement) return a.getBoundingClientRect()
	return {left: a.x, top: a.y, right: a.x + (a.w ?? 0), bottom: a.y + (a.h ?? 0), width: a.w ?? 0, height: a.h ?? 0}
}

async function position() {
	if (!props.anchor) return
	await nextTick()
	const r = rectOf(props.anchor)
	const box = el.value?.getBoundingClientRect()
	const w = box?.width ?? 0
	const h = box?.height ?? 0
	let x = r.left
	let y = r.bottom + props.offset
	switch (props.placement) {
		case 'bottom-end':
			x = r.right - w
			break
		case 'right-start':
			x = r.right + props.offset
			y = r.top
			if (x + w > window.innerWidth - 8) x = r.left - w - props.offset
			break
		case 'left-start':
			x = r.left - w - props.offset
			y = r.top
			if (x < 8) x = r.right + props.offset
			break
		case 'top-start':
			y = r.top - h - props.offset
			break
	}
	if (y + h > window.innerHeight - 8) {
		y = props.placement.startsWith('bottom') ? Math.max(8, r.top - h - props.offset) : window.innerHeight - h - 8
	}
	x = Math.min(Math.max(8, x), window.innerWidth - w - 8)
	y = Math.max(8, y)
	style.value = {left: `${Math.round(x)}px`, top: `${Math.round(y)}px`}
}

function onDown(e: MouseEvent) {
	const target = e.target as Node
	if (el.value?.contains(target)) return
	if (props.anchor instanceof HTMLElement && props.anchor.contains(target)) return
	// Clicks inside another popover stacked above this one don't close it.
	if ((target as HTMLElement).closest?.('.popover') && (target as HTMLElement).closest('.popover') !== el.value) {
		const all = [...document.querySelectorAll('.popover')]
		if (all.indexOf((target as HTMLElement).closest('.popover')!) > all.indexOf(el.value!)) return
	}
	emit('close')
}

function onKey(e: KeyboardEvent) {
	if (e.key === 'Escape') {
		e.stopPropagation()
		emit('close')
	}
}

let ro: ResizeObserver | null = null

onMounted(() => {
	position()
	setTimeout(() => document.addEventListener('mousedown', onDown, true), 0)
	document.addEventListener('keydown', onKey, true)
	window.addEventListener('resize', position)
	if (el.value) {
		ro = new ResizeObserver(() => position())
		ro.observe(el.value)
	}
})

onBeforeUnmount(() => {
	document.removeEventListener('mousedown', onDown, true)
	document.removeEventListener('keydown', onKey, true)
	window.removeEventListener('resize', position)
	ro?.disconnect()
})

watch(() => props.anchor, position)
</script>

<template>
	<Teleport to="body">
		<div ref="el" class="popover" :style="style" @mousedown.stop>
			<slot />
		</div>
	</Teleport>
</template>

<style scoped>
.popover {
	position: fixed;
	z-index: 1000;
}
</style>
