import {createLucideIcon} from 'lucide-vue-next'

// Lucide's clover with the stem cut short: the stock stem runs out to (2,22),
// well past the leaves, which looks lopsided at nav-rail size.
export const MatrixIcon = createLucideIcon('matrix-clover', [
	['path', {d: 'M16.17 7.83 5 19', key: 'stem'}],
	['path', {d: 'M4.02 12a2.827 2.827 0 1 1 3.81-4.17A2.827 2.827 0 1 1 12 4.02a2.827 2.827 0 1 1 4.17 3.81A2.827 2.827 0 1 1 19.98 12a2.827 2.827 0 1 1-3.81 4.17A2.827 2.827 0 1 1 12 19.98a2.827 2.827 0 1 1-4.17-3.81A1 1 0 1 1 4 12', key: 'leaves'}],
	['path', {d: 'm7.83 7.83 8.34 8.34', key: 'vein'}],
])
