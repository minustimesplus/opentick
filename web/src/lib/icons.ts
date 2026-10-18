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

export const SearchIcon = createLucideIcon('rail-search', [
	['circle', {cx: '11', cy: '11', r: '7', 'stroke-width': '2.4', key: 'lens'}],
	['path', {d: 'm16.2 16.2 4.3 4.3', 'stroke-width': '2.6', key: 'handle'}],
])
