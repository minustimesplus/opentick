import {prefs, type Prefs} from './prefs'

/** The plain number prefs, which excludes unions like weekStart's 0 | 1 | 6. */
type WidthPref = {[K in keyof Prefs]: [Prefs[K]] extends [number] ? number extends Prefs[K] ? K : never : never}[keyof Prefs]

export const clamp = (v: number, [lo, hi]: readonly [number, number]) => Math.min(hi, Math.max(lo, v))

/** Drags a pane border; `dir` is +1 when moving right grows the pane, -1 when it shrinks it. */
export function startResize(e: PointerEvent, key: WidthPref, range: readonly [number, number], dir: 1 | -1) {
	const startX = e.clientX
	const startW = clamp(prefs[key], range)
	const move = (ev: PointerEvent) => {
		prefs[key] = clamp(startW + dir * (ev.clientX - startX), range)
	}
	const up = () => {
		window.removeEventListener('pointermove', move)
		window.removeEventListener('pointerup', up)
		document.body.classList.remove('col-resizing')
	}
	document.body.classList.add('col-resizing')
	window.addEventListener('pointermove', move)
	window.addEventListener('pointerup', up)
	e.preventDefault()
}
