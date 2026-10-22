import {h} from 'vue'
import {createLucideIcon} from 'lucide-vue-next'

// Nav-rail icons drawn after TickTick's: solid shapes with the details cut
// out (even-odd fill), rather than Lucide's outlines.
const solid = {fill: 'currentColor', stroke: 'none', 'fill-rule': 'evenodd'}

export const TasksIcon = createLucideIcon('rail-tasks', [
	['path', {
		...solid,
		key: 'box',
		d: 'M7 3h10a4 4 0 0 1 4 4v10a4 4 0 0 1-4 4H7a4 4 0 0 1-4-4V7a4 4 0 0 1 4-4z'
			+ 'M8.1 11.4 10.5 13.8l5.4-5.4a.85.85 0 0 1 1.2 1.2L10.5 16.2l-3.6-3.6a.85.85 0 0 1 1.2-1.2z',
	}],
])

// Four petals, each a circle with its inner corner squared off.
export const MatrixIcon = createLucideIcon('rail-matrix', [
	['path', {...solid, key: 'tl', d: 'M11.2 11.2H7.1A4.1 4.1 0 0 1 3 7.1a4.1 4.1 0 0 1 4.1-4.1 4.1 4.1 0 0 1 4.1 4.1z'}],
	['path', {...solid, key: 'tr', d: 'M12.8 11.2V7.1A4.1 4.1 0 0 1 16.9 3 4.1 4.1 0 0 1 21 7.1a4.1 4.1 0 0 1-4.1 4.1z'}],
	['path', {...solid, key: 'bl', d: 'M11.2 12.8v4.1A4.1 4.1 0 0 1 7.1 21 4.1 4.1 0 0 1 3 16.9a4.1 4.1 0 0 1 4.1-4.1z'}],
	['path', {...solid, key: 'br', d: 'M12.8 12.8h4.1a4.1 4.1 0 0 1 4.1 4.1 4.1 4.1 0 0 1-4.1 4.1 4.1 4.1 0 0 1-4.1-4.1z'}],
])

const dot = (x: number, y: number) => `M${x - 1} ${y - 1}h2v2h-2z`

export const CalendarIcon = createLucideIcon('rail-calendar', [
	['path', {...solid, key: 'head', d: 'M3 7.2V7a3.5 3.5 0 0 1 3.5-3.5h11A3.5 3.5 0 0 1 21 7v.2z'}],
	['path', {
		...solid,
		key: 'body',
		d: 'M3 8.6h18V17a3.5 3.5 0 0 1-3.5 3.5h-11A3.5 3.5 0 0 1 3 17z'
			+ dot(8, 12.4) + dot(12, 12.4) + dot(16, 12.4) + dot(8, 16.4) + dot(12, 16.4),
	}],
])

// An hourglass: two solid bulbs meeting at the neck, between a top and a bottom bar.
export const CountdownIcon = createLucideIcon('rail-countdown', [
	['path', {...solid, key: 'top', d: 'M5.5 2.5h13a1.2 1.2 0 0 1 0 2.4h-13a1.2 1.2 0 0 1 0-2.4z'}],
	['path', {...solid, key: 'bottom', d: 'M5.5 19.1h13a1.2 1.2 0 0 1 0 2.4h-13a1.2 1.2 0 0 1 0-2.4z'}],
	['path', {...solid, key: 'upper', d: 'M7 6.3h10v1c0 2-1.5 3.5-3.6 4.7l-1.4.6-1.4-.6C8.5 10.8 7 9.3 7 7.3z'}],
	['path', {...solid, key: 'lower', d: 'M12 12.6l1.4.6c2.1 1.2 3.6 2.7 3.6 4.7v.4H7v-.4c0-2 1.5-3.5 3.6-4.7z'}],
])

export const SearchIcon = createLucideIcon('rail-search', [
	['circle', {cx: '11', cy: '11', r: '7', 'stroke-width': '2.4', key: 'lens'}],
	['path', {d: 'm16.2 16.2 4.3 4.3', 'stroke-width': '2.6', key: 'handle'}],
])

export const BellIcon = createLucideIcon('rail-bell', [
	['path', {
		...solid,
		key: 'body',
		d: 'M12 2.2a1.3 1.3 0 0 1 1.3 1.3v.6a6.2 6.2 0 0 1 4.9 6.1v4.2l1.7 2.3a.9.9 0 0 1-.7 1.4H4.8a.9.9 0 0 1-.7-1.4l1.7-2.3v-4.2a6.2 6.2 0 0 1 4.9-6.1v-.6A1.3 1.3 0 0 1 12 2.2z',
	}],
	['path', {...solid, key: 'clapper', d: 'M9.4 19.4h5.2a2.6 2.6 0 0 1-5.2 0z'}],
])

// A solid disc with the question mark cut out. Lucide icons are flat lists of
// shapes, so the cut-out mask needs a hand-built SVG.
export function HelpIcon(props: {size?: number}) {
	const size = props.size ?? 24
	return h('svg', {width: size, height: size, viewBox: '0 0 24 24', fill: 'none'}, [
		h('mask', {id: 'rail-help-cut'}, [
			h('rect', {width: 24, height: 24, fill: '#fff'}),
			h('path', {
				d: 'M9.5 9.3a2.6 2.6 0 1 1 3.6 2.4c-.7.3-1.1.9-1.1 1.6v.5',
				stroke: '#000',
				'stroke-width': 2.2,
				'stroke-linecap': 'round',
			}),
			h('circle', {cx: 12, cy: 17.1, r: 1.3, fill: '#000'}),
		]),
		h('circle', {cx: 12, cy: 12, r: 10, fill: 'currentColor', mask: 'url(#rail-help-cut)'}),
	])
}
HelpIcon.props = ['size']
