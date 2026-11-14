<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import type {ContactListItem} from '@/api/types'
import {usePeople} from '@/stores/people'
import {project, wrapLon, type PlacePoint} from '@/lib/peopleStats'
import {t} from '@/lib/i18n'
import {prefs} from '@/lib/prefs'

const props = defineProps<{
	places: PlacePoint[]
	dots: Float32Array
	contacts: Map<number, ContactListItem>
}>()

const people = usePeople()
const g = people.globe

const wrap = ref<HTMLElement | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)
const size = ref({w: 0, h: 0})

/** Rotation speed in px per frame, carried on after a drag. */
let vel = {x: 0, y: 0}
let target: {lon: number, lat: number, zoom: number} | null = null
let frame = 0

const baseR = computed(() => Math.min(size.value.w, size.value.h) * 0.5 * 0.88)

defineExpose({flyTo})

function flyTo(lon: number, lat: number, zoom: number) {
	target = {lon, lat, zoom}
	vel = {x: 0, y: 0}
	redraw()
}

// ---------- colors ----------

type RGB = [number, number, number]

/** Resolves a theme variable to RGB; themes may define colors with color-mix(). */
function cssColor(name: string): RGB {
	const probe = document.createElement('span')
	probe.style.color = `var(${name})`
	probe.style.display = 'none'
	document.body.appendChild(probe)
	const value = getComputedStyle(probe).color
	probe.remove()
	const nums = value.match(/[\d.]+/g)?.map(Number) ?? [128, 128, 128]
	// color(srgb 0.1 0.2 0.3) is 0–1, rgb(25, 51, 76) is 0–255.
	return value.startsWith('color(') ? [nums[0] * 255, nums[1] * 255, nums[2] * 255] : [nums[0], nums[1], nums[2]]
}

const mix = (a: RGB, b: RGB, t: number): RGB => [0, 1, 2].map(i => a[i] + (b[i] - a[i]) * t) as RGB
const rgba = (c: RGB, a = 1) => `rgba(${c[0] | 0}, ${c[1] | 0}, ${c[2] | 0}, ${a})`

let colors = readColors()

function readColors() {
	const bg = cssColor('--bg')
	const accent = cssColor('--accent')
	const text = cssColor('--text')
	const dark = (0.299 * bg[0] + 0.587 * bg[1] + 0.114 * bg[2]) / 255 < 0.5
	const black: RGB = [0, 0, 0]
	const white: RGB = [255, 255, 255]
	return {
		dark,
		bg,
		accent,
		text,
		// Region and country fallbacks get a cooler tint than exact cities.
		coarse: mix(accent, [64, 150, 200], 0.55),
		oceanCenter: dark ? mix(mix(bg, white, 0.06), accent, 0.06) : mix(bg, white, 0.6),
		oceanRim: dark ? mix(bg, black, 0.45) : mix(mix(bg, accent, 0.1), black, 0.1),
		land: dark ? mix(text, accent, 0.2) : mix(mix(text, accent, 0.2), white, 0.25),
	}
}

let themeObserver: MutationObserver | null = null

// ---------- drawing ----------

interface OnScreen {
	x: number
	y: number
	r: number
	fade: number
}

let screen: (OnScreen | null)[] = []
const hovered = ref<number | null>(null)
const pointer = ref({x: 0, y: 0})

function redraw() {
	if (!frame) frame = requestAnimationFrame(tick)
}

function tick() {
	frame = 0
	const r = baseR.value * g.zoom
	if (!r) return
	const degPerPx = 180 / (Math.PI * r)
	let animating = false
	if (!dragging && Math.hypot(vel.x, vel.y) > 0.15) {
		g.lon0 = wrapLon(g.lon0 - vel.x * degPerPx)
		g.lat0 = Math.max(-85, Math.min(85, g.lat0 + vel.y * degPerPx))
		vel = {x: vel.x * 0.9, y: vel.y * 0.9}
		animating = true
	}
	if (target) {
		const dl = wrapLon(target.lon - g.lon0)
		g.lon0 = wrapLon(g.lon0 + dl * 0.14)
		g.lat0 += (target.lat - g.lat0) * 0.14
		g.zoom += (target.zoom - g.zoom) * 0.14
		const arrived = Math.abs(dl) < 0.05 && Math.abs(target.lat - g.lat0) < 0.05 && Math.abs(target.zoom - g.zoom) < 0.005
		if (arrived) target = null
		animating = true
	}
	draw()
	if (animating) redraw()
}

function draw() {
	const el = canvas.value
	const ctx = el?.getContext('2d')
	if (!el || !ctx) return
	const {w, h} = size.value
	const dpr = window.devicePixelRatio || 1
	if (el.width !== Math.round(w * dpr) || el.height !== Math.round(h * dpr)) {
		el.width = Math.round(w * dpr)
		el.height = Math.round(h * dpr)
	}
	ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
	ctx.clearRect(0, 0, w, h)

	const c = colors
	const {lon0, lat0, zoom} = g
	const r = baseR.value * zoom
	const cx = w / 2
	const cy = h / 2

	// Atmosphere glow.
	const glow = ctx.createRadialGradient(cx, cy, r, cx, cy, r + 22)
	glow.addColorStop(0, rgba(c.accent, c.dark ? 0.16 : 0.12))
	glow.addColorStop(1, rgba(c.accent, 0))
	ctx.fillStyle = glow
	ctx.beginPath()
	ctx.arc(cx, cy, r + 22, 0, Math.PI * 2)
	ctx.fill()

	// Ocean, lit from the upper left.
	const ocean = ctx.createRadialGradient(cx - 0.32 * r, cy - 0.38 * r, 0, cx, cy, r)
	ocean.addColorStop(0, rgba(c.oceanCenter))
	ocean.addColorStop(1, rgba(c.oceanRim))
	ctx.fillStyle = ocean
	ctx.beginPath()
	ctx.arc(cx, cy, r, 0, Math.PI * 2)
	ctx.fill()

	// Graticule.
	ctx.strokeStyle = rgba(c.text, c.dark ? 0.06 : 0.07)
	ctx.lineWidth = 0.8
	const curve = (points: [number, number][]) => {
		ctx.beginPath()
		let pen = false
		for (const [lon, lat] of points) {
			const [x, y, z] = project(lon, lat, lon0, lat0)
			if (z > 0) {
				if (pen) ctx.lineTo(cx + x * r, cy - y * r)
				else ctx.moveTo(cx + x * r, cy - y * r)
				pen = true
			} else {
				pen = false
			}
		}
		ctx.stroke()
	}
	for (let lon = -180; lon < 180; lon += 30) {
		const pts: [number, number][] = []
		for (let lat = -90; lat <= 90; lat += 3) pts.push([lon, lat])
		curve(pts)
	}
	for (let lat = -60; lat <= 60; lat += 30) {
		const pts: [number, number][] = []
		for (let lon = -180; lon <= 180; lon += 3) pts.push([lon, lat])
		curve(pts)
	}

	// Land as dots, shaded by depth: one path per shade keeps this to a few fills.
	const dotR = Math.max(1, Math.min(3.4, r / 170))
	const SHADES = 6
	const paths = Array.from({length: SHADES}, () => new Path2D())
	const dots = props.dots
	for (let i = 0; i < dots.length; i += 2) {
		const [x, y, z] = project(dots[i], dots[i + 1], lon0, lat0)
		if (z <= 0.02) continue
		const px = cx + x * r
		const py = cy - y * r
		if (px < -4 || py < -4 || px > w + 4 || py > h + 4) continue
		const shade = Math.min(SHADES - 1, Math.floor(z * SHADES))
		const dr = dotR * (0.6 + 0.4 * z)
		paths[shade].moveTo(px + dr, py)
		paths[shade].arc(px, py, dr, 0, Math.PI * 2)
	}
	for (let s = 0; s < SHADES; s++) {
		ctx.fillStyle = rgba(c.land, 0.35 + 0.6 * (s + 0.5) / SHADES)
		ctx.fill(paths[s])
	}

	// Rim.
	ctx.strokeStyle = rgba(c.text, c.dark ? 0.08 : 0.1)
	ctx.lineWidth = 1
	ctx.beginPath()
	ctx.arc(cx, cy, r, 0, Math.PI * 2)
	ctx.stroke()

	// Markers. Places come biggest first, so smaller ones land on top.
	const scale = Math.pow(zoom, 0.3)
	screen = props.places.map(p => {
		const [x, y, z] = project(p.lon, p.lat, lon0, lat0)
		if (z <= 0) return null
		return {
			x: cx + x * r,
			y: cy - y * r,
			r: Math.min(24, (2.6 + 2.3 * Math.sqrt(p.contactIds.length)) * scale),
			fade: Math.min(1, z / 0.18),
		}
	})
	ctx.textAlign = 'center'
	ctx.textBaseline = 'middle'
	props.places.forEach((p, i) => {
		const m = screen[i]
		if (!m) return
		const col = p.precise ? c.accent : c.coarse
		const on = hovered.value === i || g.place === p.label
		ctx.fillStyle = rgba(col, 0.16 * m.fade)
		ctx.beginPath()
		ctx.arc(m.x, m.y, m.r + 5, 0, Math.PI * 2)
		ctx.fill()
		ctx.fillStyle = rgba(col, (on ? 1 : 0.85) * m.fade)
		ctx.beginPath()
		ctx.arc(m.x, m.y, m.r, 0, Math.PI * 2)
		ctx.fill()
		ctx.strokeStyle = rgba(c.bg, 0.9 * m.fade)
		ctx.lineWidth = 1.2
		ctx.stroke()
		if (g.place === p.label) {
			ctx.strokeStyle = rgba(col, m.fade)
			ctx.lineWidth = 2
			ctx.beginPath()
			ctx.arc(m.x, m.y, m.r + 4, 0, Math.PI * 2)
			ctx.stroke()
		}
		if (p.contactIds.length >= 3 && m.r >= 9) {
			ctx.fillStyle = `rgba(255, 255, 255, ${m.fade})`
			ctx.font = `600 ${Math.max(9, Math.min(13, m.r * 0.85))}px system-ui, sans-serif`
			ctx.fillText(String(p.contactIds.length), m.x, m.y + 0.5)
		}
	})
}

function hitTest(x: number, y: number): number | null {
	let best: number | null = null
	let bestScore = Infinity
	screen.forEach((m, i) => {
		if (!m) return
		const d = Math.hypot(x - m.x, y - m.y)
		if (d <= m.r + 3 && d / m.r < bestScore) {
			best = i
			bestScore = d / m.r
		}
	})
	return best
}

// ---------- input ----------

const pointers = new Map<number, {x: number, y: number}>()
let dragging = false
let moved = 0
let pinchDist = 0

function local(e: PointerEvent | WheelEvent | MouseEvent) {
	const rect = canvas.value!.getBoundingClientRect()
	return {x: e.clientX - rect.left, y: e.clientY - rect.top}
}

function onPointerDown(e: PointerEvent) {
	canvas.value?.setPointerCapture(e.pointerId)
	pointers.set(e.pointerId, local(e))
	dragging = true
	moved = 0
	target = null
	vel = {x: 0, y: 0}
	if (pointers.size === 2) {
		const [a, b] = [...pointers.values()]
		pinchDist = Math.hypot(a.x - b.x, a.y - b.y)
	}
}

function onPointerMove(e: PointerEvent) {
	const p = local(e)
	pointer.value = p
	const prev = pointers.get(e.pointerId)
	if (!dragging || !prev) {
		const hit = hitTest(p.x, p.y)
		if (hit !== hovered.value) {
			hovered.value = hit
			redraw()
		}
		return
	}
	pointers.set(e.pointerId, p)
	if (pointers.size === 2) {
		const [a, b] = [...pointers.values()]
		const dist = Math.hypot(a.x - b.x, a.y - b.y)
		if (pinchDist) zoomBy(dist / pinchDist)
		pinchDist = dist
		moved += 10
		return
	}
	const dx = p.x - prev.x
	const dy = p.y - prev.y
	moved += Math.abs(dx) + Math.abs(dy)
	const degPerPx = 180 / (Math.PI * baseR.value * g.zoom)
	g.lon0 = wrapLon(g.lon0 - dx * degPerPx)
	g.lat0 = Math.max(-85, Math.min(85, g.lat0 + dy * degPerPx))
	vel = {x: dx, y: dy}
	hovered.value = null
	redraw()
}

function onPointerUp(e: PointerEvent) {
	pointers.delete(e.pointerId)
	if (pointers.size) return
	dragging = false
	pinchDist = 0
	if (moved < 4) {
		vel = {x: 0, y: 0}
		const p = local(e)
		const hit = hitTest(p.x, p.y)
		g.place = hit === null ? '' : props.places[hit].label
	}
	redraw()
}

function onDoubleClick(e: MouseEvent) {
	const p = local(e)
	const hit = hitTest(p.x, p.y)
	if (hit === null) return
	const place = props.places[hit]
	flyTo(place.lon, place.lat, Math.min(g.zoom * 2, 10))
}

function zoomBy(factor: number) {
	g.zoom = Math.max(0.8, Math.min(12, g.zoom * factor))
	target = null
	redraw()
}

function onWheel(e: WheelEvent) {
	e.preventDefault()
	// Trackpad pinches arrive as wheel events with ctrlKey, in finer steps.
	zoomBy(Math.exp(-e.deltaY * (e.ctrlKey ? 0.01 : 0.0025)))
}

function onLeave() {
	if (hovered.value !== null) {
		hovered.value = null
		redraw()
	}
}

const tip = computed(() => {
	if (hovered.value === null) return null
	const place = props.places[hovered.value]
	if (!place) return null
	const names = place.contactIds.slice(0, 6).map(id => props.contacts.get(id)?.name ?? '')
	const n = place.contactIds.length
	return {
		label: place.label,
		count: n === 1 ? t('statsOneContact') : t('contactsCount', {n}),
		names,
		more: n - names.length,
		left: Math.min(pointer.value.x + 14, size.value.w - 200),
		top: Math.min(pointer.value.y + 14, size.value.h - 40),
	}
})

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
	resizeObserver = new ResizeObserver(([entry]) => {
		size.value = {w: entry.contentRect.width, h: entry.contentRect.height}
		redraw()
	})
	resizeObserver.observe(wrap.value!)
	themeObserver = new MutationObserver(() => {
		colors = readColors()
		redraw()
	})
	themeObserver.observe(document.documentElement, {attributes: true})
	canvas.value?.addEventListener('wheel', onWheel, {passive: false})
})

onBeforeUnmount(() => {
	resizeObserver?.disconnect()
	themeObserver?.disconnect()
	canvas.value?.removeEventListener('wheel', onWheel)
	cancelAnimationFrame(frame)
	frame = 0
})

watch(() => [props.places, props.dots, g.place, prefs.themeColor], redraw)
</script>

<template>
	<div ref="wrap" class="globe">
		<canvas
			ref="canvas"
			:class="{pointing: hovered !== null}"
			@pointerdown="onPointerDown"
			@pointermove="onPointerMove"
			@pointerup="onPointerUp"
			@pointercancel="onPointerUp"
			@pointerleave="onLeave"
			@dblclick="onDoubleClick"
		/>
		<div v-if="tip" class="tip" :style="{left: tip.left + 'px', top: tip.top + 'px'}">
			<div class="tip-label">{{ tip.label }}</div>
			<div class="tip-count">{{ tip.count }}</div>
			<div v-for="(name, i) in tip.names" :key="i" class="tip-name">{{ name }}</div>
			<div v-if="tip.more > 0" class="tip-count">{{ t('more', {n: tip.more}) }}</div>
		</div>
	</div>
</template>

<style scoped>
.globe {
	position: relative;
	flex: 1;
	min-height: 240px;
	overflow: hidden;
}

canvas {
	position: absolute;
	inset: 0;
	width: 100%;
	height: 100%;
	cursor: grab;
	touch-action: none;
}

canvas:active {
	cursor: grabbing;
}

canvas.pointing {
	cursor: pointer;
}

.tip {
	position: absolute;
	z-index: 2;
	max-width: 220px;
	padding: 8px 10px;
	border-radius: 8px;
	background: var(--bg-elev);
	box-shadow: var(--shadow);
	pointer-events: none;
	font-size: 13px;
}

.tip-label {
	font-weight: 600;
}

.tip-count {
	margin-bottom: 2px;
	font-size: 12px;
	color: var(--text-3);
}

.tip-name {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}
</style>
