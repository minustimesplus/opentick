// Evaluates Vikunja filter strings ("done = false && due_date < now/d+1d")
// against tasks for the in-browser backend. Covers the fields and operators
// the app writes; anything unknown matches nothing.

import type {Task} from '../types'

type Token = {kind: 'lp' | 'rp' | 'and' | 'or' | 'comma'} | {kind: 'word', value: string}

type Node =
	| {kind: 'and' | 'or', left: Node, right: Node}
	| {kind: 'cmp', field: string, op: string, values: string[]}

const OPS = ['>=', '<=', '!=', '=', '>', '<', 'like', 'in', 'not in']

function tokenize(src: string): Token[] {
	const out: Token[] = []
	let i = 0
	while (i < src.length) {
		const c = src[i]
		if (/\s/.test(c)) {
			i++
		} else if (c === '(') {
			out.push({kind: 'lp'}); i++
		} else if (c === ')') {
			out.push({kind: 'rp'}); i++
		} else if (c === ',') {
			out.push({kind: 'comma'}); i++
		} else if (src.startsWith('&&', i)) {
			out.push({kind: 'and'}); i += 2
		} else if (src.startsWith('||', i)) {
			out.push({kind: 'or'}); i += 2
		} else if (c === '"' || c === '\'') {
			const end = src.indexOf(c, i + 1)
			const stop = end < 0 ? src.length : end
			out.push({kind: 'word', value: src.slice(i + 1, stop)})
			i = stop + 1
		} else if (/[<>!=]/.test(c)) {
			const op = ['>=', '<=', '!='].find(o => src.startsWith(o, i)) ?? c
			out.push({kind: 'word', value: op}); i += op.length
		} else {
			let j = i
			while (j < src.length && !/[\s(),<>!=&|"']/.test(src[j])) j++
			out.push({kind: 'word', value: src.slice(i, j)})
			i = j
		}
	}
	return out
}

export function parseFilter(src: string): Node | null {
	const tokens = tokenize(src)
	let pos = 0
	const peek = () => tokens[pos]
	const word = (): string => {
		const t = tokens[pos++]
		if (!t || t.kind !== 'word') throw new Error('filter: expected a value')
		return t.value
	}

	function expr(): Node {
		let left = conj()
		while (peek()?.kind === 'or') {
			pos++
			left = {kind: 'or', left, right: conj()}
		}
		return left
	}

	function conj(): Node {
		let left = atom()
		while (peek()?.kind === 'and') {
			pos++
			left = {kind: 'and', left, right: atom()}
		}
		return left
	}

	function atom(): Node {
		if (peek()?.kind === 'lp') {
			pos++
			const inner = expr()
			if (peek()?.kind === 'rp') pos++
			return inner
		}
		const field = word()
		let op = word().toLowerCase()
		if (op === 'not') op = 'not ' + word().toLowerCase()
		if (!OPS.includes(op)) throw new Error(`filter: unknown operator ${op}`)
		const values = [word()]
		if (op === 'in' || op === 'not in') {
			while (peek()?.kind === 'comma') {
				pos++
				values.push(word())
			}
		}
		return {kind: 'cmp', field, op, values}
	}

	try {
		return tokens.length ? expr() : null
	} catch {
		return null
	}
}

/** "now", "now/d", "now/d+1d", "now-7d" or a date string, as epoch ms. */
export function resolveDate(value: string, now = new Date()): number | null {
	const m = value.match(/^now(\/[dwMy])?(([+-]\d+[smhdwMy])*)$/)
	if (!m) {
		const d = new Date(value)
		return isNaN(d.getTime()) ? null : d.getTime()
	}
	const d = new Date(now)
	if (m[1] === '/d') d.setHours(0, 0, 0, 0)
	if (m[1] === '/w') {
		d.setHours(0, 0, 0, 0)
		d.setDate(d.getDate() - ((d.getDay() + 6) % 7))
	}
	if (m[1] === '/M') {
		d.setHours(0, 0, 0, 0)
		d.setDate(1)
	}
	if (m[1] === '/y') {
		d.setHours(0, 0, 0, 0)
		d.setMonth(0, 1)
	}
	for (const [, sign, n, unit] of m[2].matchAll(/([+-])(\d+)([smhdwMy])/g)) {
		const k = (sign === '-' ? -1 : 1) * Number(n)
		switch (unit) {
		case 's': d.setSeconds(d.getSeconds() + k); break
		case 'm': d.setMinutes(d.getMinutes() + k); break
		case 'h': d.setHours(d.getHours() + k); break
		case 'd': d.setDate(d.getDate() + k); break
		case 'w': d.setDate(d.getDate() + 7 * k); break
		case 'M': d.setMonth(d.getMonth() + k); break
		case 'y': d.setFullYear(d.getFullYear() + k); break
		}
	}
	return d.getTime()
}

const DATE_FIELDS = new Set(['due_date', 'start_date', 'end_date', 'done_at', 'created', 'updated'])

function dateOf(value?: string | null): number | null {
	if (!value) return null
	const t = new Date(value).getTime()
	return isNaN(t) || new Date(t).getFullYear() < 1900 ? null : t
}

function compare(a: number | string, b: number | string, op: string): boolean {
	switch (op) {
	case '=': return a === b
	case '!=': return a !== b
	case '>': return a > b
	case '>=': return a >= b
	case '<': return a < b
	case '<=': return a <= b
	}
	return false
}

function matchCmp(task: Task, n: Extract<Node, {kind: 'cmp'}>): boolean {
	const {op, values} = n
	const field = n.field === 'project_id' ? 'project' : n.field
	const v = values[0]

	if (field === 'labels' || field === 'label_id' || field === 'assignees') {
		const have = field === 'assignees'
			? (task.assignees ?? []).flatMap(a => [String(a.id), a.username])
			: (task.labels ?? []).flatMap(l => [String(l.id), l.title])
		const any = values.some(x => have.includes(x))
		if (op === 'in' || op === '=') return any
		if (op === 'not in' || op === '!=') return !any
		return false
	}

	if (field === 'done' || field === 'wont_do' || field === 'is_favorite') {
		const actual = !!task[field as 'done' | 'wont_do' | 'is_favorite']
		const want = v === 'true'
		return op === '!=' ? actual !== want : actual === want
	}

	if (DATE_FIELDS.has(field)) {
		const actual = dateOf(task[field as 'due_date'])
		if (actual === null) return false
		const want = resolveDate(v)
		return want !== null && compare(actual, want, op)
	}

	if (field === 'title' || field === 'description') {
		const actual = (task[field] ?? '').toLowerCase()
		if (op === 'like') {
			const needle = v.replace(/^%|%$/g, '').toLowerCase()
			return actual.includes(needle)
		}
		return compare(actual, v.toLowerCase(), op)
	}

	const numeric: Record<string, number> = {
		project: task.project_id,
		priority: task.priority ?? 0,
		id: task.id,
		index: task.index ?? 0,
		repeat_after: task.repeat_after ?? 0,
	}
	if (!(field in numeric)) return false
	const actual = numeric[field]
	if (op === 'in') return values.some(x => Number(x) === actual)
	if (op === 'not in') return !values.some(x => Number(x) === actual)
	return compare(actual, Number(v), op)
}

export function matches(task: Task, node: Node | null): boolean {
	if (!node) return true
	switch (node.kind) {
	case 'and': return matches(task, node.left) && matches(task, node.right)
	case 'or': return matches(task, node.left) || matches(task, node.right)
	case 'cmp': return matchCmp(task, node)
	}
}

/** Compiles a filter once; an unparsable filter matches nothing. */
export function compileFilter(src: string | undefined): (task: Task) => boolean {
	if (!src?.trim()) return () => true
	const node = parseFilter(src)
	if (!node) return () => false
	return task => matches(task, node)
}
