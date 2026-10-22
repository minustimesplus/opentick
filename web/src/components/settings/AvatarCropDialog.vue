<script setup lang="ts">
import {onBeforeUnmount, ref} from 'vue'
import Modal from '../Modal.vue'
import {t} from '@/lib/i18n'

const props = defineProps<{file: File}>()
const emit = defineEmits<{close: [], cropped: [File]}>()

/** Side of the square crop viewport, in CSS pixels. */
const VIEW = 280
/** Side of the exported square image. */
const OUT = 512
const MAX_ZOOM = 4

const src = URL.createObjectURL(props.file)
onBeforeUnmount(() => URL.revokeObjectURL(src))

const img = ref<HTMLImageElement | null>(null)
const ready = ref(false)
const zoom = ref(1)
// Top-left corner of the image relative to the viewport, in viewport pixels.
const x = ref(0)
const y = ref(0)
let baseScale = 1

const scale = () => baseScale * zoom.value

function clamp() {
	const el = img.value
	if (!el) return
	const w = el.naturalWidth * scale()
	const h = el.naturalHeight * scale()
	x.value = Math.min(0, Math.max(VIEW - w, x.value))
	y.value = Math.min(0, Math.max(VIEW - h, y.value))
}

function onLoad() {
	const el = img.value
	if (!el) return
	// Cover the viewport with the image's short side, centered.
	baseScale = VIEW / Math.min(el.naturalWidth, el.naturalHeight)
	x.value = (VIEW - el.naturalWidth * baseScale) / 2
	y.value = (VIEW - el.naturalHeight * baseScale) / 2
	ready.value = true
}

/** Zooms to `next`, keeping the point (px, py) of the viewport fixed. */
function setZoom(next: number, px = VIEW / 2, py = VIEW / 2) {
	next = Math.min(MAX_ZOOM, Math.max(1, next))
	const ratio = next / zoom.value
	x.value = px - (px - x.value) * ratio
	y.value = py - (py - y.value) * ratio
	zoom.value = next
	clamp()
}

function onWheel(e: WheelEvent) {
	const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
	setZoom(zoom.value * Math.exp(-e.deltaY * 0.002), e.clientX - rect.left, e.clientY - rect.top)
}

let drag: {id: number, px: number, py: number, x: number, y: number} | null = null

function onPointerDown(e: PointerEvent) {
	(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
	drag = {id: e.pointerId, px: e.clientX, py: e.clientY, x: x.value, y: y.value}
}

function onPointerMove(e: PointerEvent) {
	if (!drag || drag.id !== e.pointerId) return
	x.value = drag.x + e.clientX - drag.px
	y.value = drag.y + e.clientY - drag.py
	clamp()
}

function onPointerUp(e: PointerEvent) {
	if (drag?.id === e.pointerId) drag = null
}

function onKey(e: KeyboardEvent) {
	const step = e.shiftKey ? 20 : 5
	const moves: Record<string, [number, number]> = {ArrowLeft: [step, 0], ArrowRight: [-step, 0], ArrowUp: [0, step], ArrowDown: [0, -step]}
	const m = moves[e.key]
	if (m) {
		e.preventDefault()
		x.value += m[0]
		y.value += m[1]
		clamp()
	} else if (e.key === '+' || e.key === '=') {
		setZoom(zoom.value * 1.1)
	} else if (e.key === '-') {
		setZoom(zoom.value / 1.1)
	}
}

function save() {
	const el = img.value
	if (!el) return
	const canvas = document.createElement('canvas')
	canvas.width = OUT
	canvas.height = OUT
	const ctx = canvas.getContext('2d')
	if (!ctx) return
	ctx.imageSmoothingQuality = 'high'
	const s = scale()
	ctx.drawImage(el, -x.value / s, -y.value / s, VIEW / s, VIEW / s, 0, 0, OUT, OUT)
	canvas.toBlob(blob => {
		if (!blob) return
		emit('cropped', new File([blob], 'avatar.png', {type: 'image/png'}))
	}, 'image/png')
}
</script>

<template>
	<Modal @close="emit('close')">
		<div class="crop">
			<h3>{{ t('cropAvatar') }}</h3>
			<div
				class="viewport"
				:style="{width: VIEW + 'px', height: VIEW + 'px'}"
				tabindex="0"
				@wheel.prevent="onWheel"
				@pointerdown="onPointerDown"
				@pointermove="onPointerMove"
				@pointerup="onPointerUp"
				@pointercancel="onPointerUp"
				@keydown="onKey"
			>
				<img
					ref="img"
					:src="src"
					alt=""
					draggable="false"
					:style="{
						visibility: ready ? 'visible' : 'hidden',
						transform: `translate(${x}px, ${y}px) scale(${scale()})`,
					}"
					@load="onLoad"
				>
				<div class="mask" />
			</div>
			<label class="zoom">
				<span>{{ t('zoom') }}</span>
				<input
					type="range"
					min="1"
					:max="MAX_ZOOM"
					step="0.01"
					:value="zoom"
					@input="setZoom(+($event.target as HTMLInputElement).value)"
				>
			</label>
			<div class="actions">
				<button type="button" class="btn" @click="emit('close')">{{ t('cancel') }}</button>
				<button type="button" class="btn primary" :disabled="!ready" @click="save">{{ t('save') }}</button>
			</div>
		</div>
	</Modal>
</template>

<style scoped>
.crop {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 14px;
}

h3 {
	align-self: flex-start;
	margin: 0;
	font-size: 17px;
}

.viewport {
	position: relative;
	overflow: hidden;
	border-radius: 8px;
	background: var(--bg-input);
	cursor: grab;
	touch-action: none;
	user-select: none;
}

.viewport:active {
	cursor: grabbing;
}

.viewport img {
	position: absolute;
	top: 0;
	left: 0;
	max-width: none;
	transform-origin: 0 0;
	pointer-events: none;
}

/* Dims everything outside the circle the avatar is shown in. */
.mask {
	position: absolute;
	inset: 0;
	border-radius: 50%;
	box-shadow: 0 0 0 999px rgba(0, 0, 0, 0.5);
	pointer-events: none;
}

.zoom {
	display: flex;
	align-items: center;
	gap: 10px;
	width: 100%;
	font-size: 12px;
	color: var(--text-3);
}

.zoom input {
	flex: 1;
}

.actions {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	align-self: stretch;
}

.btn:disabled {
	opacity: 0.6;
	pointer-events: none;
}
</style>
