import type {Project, Task} from '@/api/types'
import {t} from './i18n'

// TickTick has four priority levels; Vikunja has 0-5. High covers Vikunja's
// high, urgent and "do now".
export type Level = 0 | 1 | 2 | 3

export function level(p?: number): Level {
	if (!p || p <= 0) return 0
	if (p >= 3) return 3
	return p as Level
}

export const PRIORITY_COLORS: Record<Level, string> = {
	0: 'var(--prio-none)',
	1: 'var(--prio-low)',
	2: 'var(--prio-medium)',
	3: 'var(--prio-high)',
}

export function priorityName(l: Level): string {
	return [t('none'), t('low'), t('medium'), t('high')][l]
}

const EMOJI_RE = /^((?:\p{Extended_Pictographic}|\p{Regional_Indicator})(?:‍\p{Extended_Pictographic}|️|\p{Emoji_Modifier}|\p{Regional_Indicator})*)\s*/u

/** Splits a leading emoji off a list title ("✍️ Midterms" -> icon + name). */
export function splitEmoji(title: string): {emoji: string | null, name: string} {
	const m = title.match(EMOJI_RE)
	if (!m) return {emoji: null, name: title}
	return {emoji: m[1], name: title.slice(m[0].length) || title}
}

export const DEFAULT_COLOR = '#4F6BED'

export function projectColor(p?: Project | null): string {
	if (p?.hex_color) return '#' + p.hex_color.replace(/^#/, '')
	return DEFAULT_COLOR
}

export function hasColor(p?: Project | null): boolean {
	return !!p?.hex_color && p.hex_color.replace('#', '').length >= 6
}

export function labelColor(hex?: string): string {
	return hex ? '#' + hex.replace(/^#/, '') : '#8A8A8A'
}

export const SWATCHES = ['', 'E03131', 'F76707', 'FAB005', '40C057', '12B886', '15AABF', '4772FA', '7048E8', 'BE4BDB', 'E64980', '868E96']

export function compareTitles(a: Task, b: Task) {
	return a.title.localeCompare(b.title, undefined, {sensitivity: 'base'})
}
