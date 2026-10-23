// Converts between TickTick-style filter settings and Vikunja filter strings.
// Only strings this module wrote can be read back; anything else is edited
// as a raw ("advanced") filter.

import type {Level} from './tasks'

export type DateRange = 'all' | 'overdue' | 'today' | 'tomorrow' | 'next7' | 'next30'

export interface FilterRules {
	lists: number[]
	labels: number[]
	date: DateRange
	/** Empty means any priority. */
	priorities: Level[]
	keyword: string
}

export const EMPTY_RULES: FilterRules = {lists: [], labels: [], date: 'all', priorities: [], keyword: ''}

const DATES: Record<Exclude<DateRange, 'all'>, string> = {
	overdue: 'due_date < now/d',
	today: 'due_date >= now/d && due_date < now/d+1d',
	tomorrow: 'due_date >= now/d+1d && due_date < now/d+2d',
	next7: 'due_date >= now/d && due_date < now/d+7d',
	next30: 'due_date >= now/d && due_date < now/d+30d',
}

// TickTick's High covers Vikunja's 3-5 (high, urgent, do now).
const PRIORITY_VALUES: Record<Level, number[]> = {0: [0], 1: [1], 2: [2], 3: [3, 4, 5]}

function ids(list: string): number[] {
	return list.split(',').map(s => Number(s.trim()))
}

export function toFilter(r: FilterRules): string {
	const parts = ['done = false']
	if (r.lists.length) parts.push(`project in ${r.lists.join(', ')}`)
	if (r.labels.length) parts.push(`labels in ${r.labels.join(', ')}`)
	if (r.date !== 'all') parts.push(DATES[r.date])
	if (r.priorities.length) {
		const values = [...r.priorities].sort().flatMap(l => PRIORITY_VALUES[l])
		parts.push(`priority in ${values.join(', ')}`)
	}
	const kw = r.keyword.replace(/['%\\]/g, '').trim()
	if (kw) parts.push(`(title like '%${kw}%' || description like '%${kw}%')`)
	return parts.join(' && ')
}

/** Reads back a filter written by toFilter; null for any other filter. */
export function fromFilter(filter: string): FilterRules | null {
	let rest = filter.trim()
	if (!rest.startsWith('done = false')) return null
	rest = rest.slice('done = false'.length)
	const r: FilterRules = {...EMPTY_RULES, lists: [], labels: [], priorities: []}
	const take = (re: RegExp): RegExpMatchArray | null => {
		const m = rest.match(re)
		if (m) rest = rest.slice(m[0].length)
		return m
	}
	let m = take(/^ && project in ([\d, ]+?)(?= &&|$)/)
	if (m) r.lists = ids(m[1])
	m = take(/^ && labels in ([\d, ]+?)(?= &&|$)/)
	if (m) r.labels = ids(m[1])
	for (const [key, expr] of Object.entries(DATES)) {
		if (rest.startsWith(' && ' + expr) && (rest.length === expr.length + 4 || rest.startsWith(' && ' + expr + ' &&'))) {
			r.date = key as DateRange
			rest = rest.slice(expr.length + 4)
			break
		}
	}
	m = take(/^ && priority in ([\d, ]+?)(?= &&|$)/)
	if (m) {
		const values = new Set(ids(m[1]))
		r.priorities = ([0, 1, 2, 3] as Level[]).filter(l => PRIORITY_VALUES[l].every(v => values.has(v)))
	}
	m = take(/^ && \(title like '%([^']*)%' \|\| description like '%\1%'\)$/)
	if (m) r.keyword = m[1]
	return rest === '' ? r : null
}

/** The saved filter id behind a saved filter's (negative) project id. */
export function filterIdOf(projectId: number): number {
	return -projectId - 1
}
