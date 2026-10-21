<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue'
import {Plus, CalendarDays, Flag, Hash, Inbox, ListIcon, Menu, Tag} from 'lucide-vue-next'
import Popover from './Popover.vue'
import DatePicker from './DatePicker.vue'
import {useData, type TaskPatch} from '@/stores/data'
import {parseQuickAdd} from '@/lib/parse'
import {parseDate, schedulePatch, scheduleLabel} from '@/lib/dates'
import {labelColor, PRIORITY_COLORS, priorityName, splitEmoji, type Level} from '@/lib/tasks'
import {t} from '@/lib/i18n'
import {ui} from '@/lib/ui'

const props = defineProps<{
	placeholder?: string
	/** Defaults applied to every task created here (list, date, priority). */
	defaults?: TaskPatch & {project_id?: number}
	defaultLabels?: string[]
}>()
const emit = defineEmits<{created: [number]}>()

const data = useData()
const text = ref('')
const focused = ref(false)
const input = ref<HTMLInputElement | null>(null)
const backdrop = ref<HTMLElement | null>(null)

watch(() => ui.focusQuickAdd, () => input.value?.focus())

const listNames = computed(() => data.realProjects.map(p => splitEmoji(p.title).name))

const parsed = computed(() => text.value.trim() ? parseQuickAdd(text.value, new Date(), listNames.value) : null)

const parsedList = computed(() => {
	const name = parsed.value?.list?.toLowerCase()
	if (!name) return null
	return data.realProjects.find(p => splitEmoji(p.title).name.toLowerCase() === name)
		?? data.realProjects.find(p => splitEmoji(p.title).name.toLowerCase().startsWith(name))
		?? null
})

const chips = computed(() => {
	const p = parsed.value
	if (!p) return []
	const out: {icon: unknown, label: string, color?: string}[] = []
	if (p.date) out.push({icon: CalendarDays, label: scheduleLabel({date: p.date, allDay: p.allDay, start: p.date, end: p.end})})
	if (p.priority !== null) out.push({icon: Flag, label: priorityName(p.priority as Level), color: PRIORITY_COLORS[p.priority as Level]})
	p.labels.forEach(l => out.push({icon: Hash, label: l}))
	if (parsedList.value) out.push({icon: ListIcon, label: splitEmoji(parsedList.value.title).name})
	return out
})

// ---- inline highlighting of recognised tokens ----

function escapeHtml(s: string) {
	return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

/** The input's text as HTML, with recognised tokens wrapped in <mark>. */
const highlighted = computed(() => {
	const src = text.value
	const ranges: [number, number][] = []
	for (const m of parsed.value?.matches ?? []) {
		for (let from = 0; ;) {
			const i = src.indexOf(m, from)
			if (i < 0) break
			from = i + m.length
			if (!ranges.some(([a, b]) => i < b && from > a)) {
				ranges.push([i, from])
				break
			}
		}
	}
	ranges.sort((a, b) => a[0] - b[0])
	let out = ''
	let at = 0
	for (const [a, b] of ranges) {
		out += escapeHtml(src.slice(at, a)) + `<mark>${escapeHtml(src.slice(a, b))}</mark>`
		at = b
	}
	return out + escapeHtml(src.slice(at))
})

function syncScroll() {
	if (backdrop.value && input.value) backdrop.value.scrollLeft = input.value.scrollLeft
}

// ---- suggestions for ~list, #tag and !priority ----

type Trigger = '~' | '#' | '!'

interface Suggestion {
	key: string
	label: string
	insert: string
	emoji?: string | null
	icon?: unknown
	color?: string
}

const token = ref<{trigger: Trigger, start: number, end: number, query: string} | null>(null)
const active = ref(0)

/** Finds the token being typed at the caret, if it starts with a trigger character. */
function readToken() {
	const el = input.value
	if (!el || el.selectionStart !== el.selectionEnd) {
		token.value = null
		return
	}
	const caret = el.selectionStart ?? text.value.length
	const before = text.value.slice(0, caret)
	const start = before.lastIndexOf(' ') + 1
	const word = before.slice(start)
	const trigger = word[0] === '^' ? '~' : word[0]
	if (trigger === '~' || trigger === '#' || trigger === '!') {
		token.value = {trigger, start, end: caret, query: word.slice(1)}
		return
	}
	// A ~ token stays open across spaces while it is the start of a multi-word list name.
	const tilde = before.lastIndexOf('~')
	const q = before.slice(tilde + 1).toLowerCase()
	const open = tilde >= 0 && (tilde === 0 || before[tilde - 1] === ' ') && q.includes(' ')
		&& listNames.value.some(n => n.toLowerCase().startsWith(q) && n.toLowerCase() !== q.trimEnd())
	token.value = open ? {trigger: '~', start: tilde, end: caret, query: before.slice(tilde + 1)} : null
}

const suggestions = computed<Suggestion[]>(() => {
	const tk = token.value
	if (!tk) return []
	const q = tk.query.toLowerCase()
	if (tk.trigger === '~') {
		return data.realProjects
			.map(p => {
				const s = splitEmoji(p.title)
				const inbox = p.id === data.inboxId
				return {p, inbox, emoji: inbox ? null : s.emoji, label: inbox ? t('inbox') : s.name, name: s.name}
			})
			.filter(x => x.label.toLowerCase().includes(q) || x.name.toLowerCase().includes(q))
			.sort((a, b) => Number(b.inbox) - Number(a.inbox))
			.slice(0, 8)
			.map(x => ({key: `p${x.p.id}`, label: x.label, emoji: x.emoji, icon: x.inbox ? Inbox : Menu, insert: `~${x.name}`}))
	}
	if (tk.trigger === '#') {
		const tags: Suggestion[] = data.labels
			.filter(l => l.title.toLowerCase().includes(q) && !/\s/.test(l.title))
			.slice(0, 8)
			.map(l => ({key: `l${l.id}`, label: l.title, icon: Tag, color: labelColor(l.hex_color), insert: `#${l.title}`}))
		if (q && /^[\p{L}\p{N}_\-/]+$/u.test(tk.query) && !data.labels.some(l => l.title.toLowerCase() === q)) {
			tags.push({key: 'new', label: t('createTag', {name: tk.query}), icon: Plus, insert: `#${tk.query}`})
		}
		return tags
	}
	const levels: {l: Level, word: string}[] = [{l: 3, word: 'high'}, {l: 2, word: 'medium'}, {l: 1, word: 'low'}, {l: 0, word: 'none'}]
	return levels
		.filter(x => !q || x.word.startsWith(q) || priorityName(x.l).toLowerCase().startsWith(q))
		.map(x => ({key: `!${x.l}`, label: priorityName(x.l), icon: Flag, color: PRIORITY_COLORS[x.l], insert: `!${x.word}`}))
})

watch(suggestions, () => {
	active.value = 0
})

// Anchor the menu below the token's first character.
const anchor = computed(() => {
	const el = input.value
	const tk = token.value
	if (!el || !tk) return null
	const r = el.getBoundingClientRect()
	const ctx = document.createElement('canvas').getContext('2d')!
	ctx.font = getComputedStyle(el).font
	const x = r.left + ctx.measureText(text.value.slice(0, tk.start)).width - el.scrollLeft
	return {x: Math.min(x, r.right - 40), y: r.top, h: r.height}
})

async function pick(s: Suggestion) {
	const tk = token.value
	if (!tk) return
	const head = text.value.slice(0, tk.start) + s.insert + ' '
	// Drop the rest of the word under the caret too.
	const tail = text.value.slice(tk.end).replace(/^\S*\s*/, '')
	text.value = head + tail
	token.value = null
	await nextTick()
	input.value?.focus()
	input.value?.setSelectionRange(head.length, head.length)
	syncScroll()
}

function onKeydown(e: KeyboardEvent) {
	if (e.isComposing) return
	const open = suggestions.value.length > 0
	if (open && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
		e.preventDefault()
		const n = suggestions.value.length
		active.value = (active.value + (e.key === 'ArrowDown' ? 1 : n - 1)) % n
	} else if (open && (e.key === 'Enter' || e.key === 'Tab')) {
		e.preventDefault()
		pick(suggestions.value[active.value])
	} else if (e.key === 'Escape') {
		if (open) {
			token.value = null
		} else {
			text.value = ''
			pickedDate.value = null
			input.value?.blur()
		}
	} else if (e.key === 'Enter') {
		e.preventDefault()
		submit()
	}
}

/** Toolbar buttons type a trigger character, which opens its menu. */
async function insertTrigger(trigger: Trigger) {
	const el = input.value
	if (!el) return
	const caret = el.selectionStart ?? text.value.length
	const before = text.value.slice(0, caret)
	const pad = before && !before.endsWith(' ') ? ' ' : ''
	text.value = before + pad + trigger + text.value.slice(caret)
	await nextTick()
	el.focus()
	const at = caret + pad.length + 1
	el.setSelectionRange(at, at)
	readToken()
}

// ---- date from the toolbar ----
const dateBtn = ref<HTMLElement | null>(null)
const pickingDate = ref(false)
const pickedDate = ref<TaskPatch | null>(null)
const pickedLabel = computed(() => {
	const d = pickedDate.value
	const start = parseDate(d?.start_date) ?? parseDate(d?.due_date)
	if (!d || !start) return ''
	return scheduleLabel({date: start, allDay: !d.start_date, start, end: parseDate(d.end_date)})
})

async function submit() {
	const p = parsed.value
	if (!p || !p.title) return
	const body: TaskPatch & {title: string} = {...(props.defaults ?? {}), ...(pickedDate.value ?? {}), title: p.title}
	if (p.date) Object.assign(body, schedulePatch(p.date, p.allDay, p.end))
	if (p.priority !== null) body.priority = p.priority
	if (parsedList.value) body.project_id = parsedList.value.id
	text.value = ''
	pickedDate.value = null
	token.value = null
	const created = await data.createTask(body, [...(props.defaultLabels ?? []), ...p.labels])
	if (created) emit('created', created.id)
}
</script>

<template>
	<div class="qa" :class="{focused}">
		<div class="bar">
			<Plus :size="18" class="plus" />
			<div class="field">
				<!-- The input's own text is transparent; this draws it, with recognised tokens marked. -->
				<div ref="backdrop" class="backdrop" aria-hidden="true" v-html="highlighted" />
				<input
					ref="input"
					v-model="text"
					:placeholder="placeholder ?? t('addTask')"
					@focus="focused = true"
					@blur="focused = false; token = null"
					@input="readToken(); syncScroll()"
					@click="readToken"
					@keyup.left="readToken"
					@keyup.right="readToken"
					@scroll="syncScroll"
					@keydown="onKeydown"
				>
			</div>
			<div v-if="focused || text" class="tools" @mousedown.prevent>
				<button ref="dateBtn" class="tool" :class="{on: !!pickedDate}" :title="t('date')" @click="pickingDate = true">
					<CalendarDays :size="16" /><span v-if="pickedLabel" class="tool-label">{{ pickedLabel }}</span>
				</button>
				<button class="tool" :title="t('priority')" @click="insertTrigger('!')"><Flag :size="16" /></button>
				<button class="tool" :title="t('lists')" @click="insertTrigger('~')"><ListIcon :size="16" /></button>
				<button class="tool" :title="t('tags')" @click="insertTrigger('#')"><Tag :size="16" /></button>
			</div>
		</div>
		<div v-if="chips.length" class="chips">
			<span v-for="(c, i) in chips" :key="i" class="chip" :style="c.color ? {color: c.color} : {}">
				<component :is="c.icon" :size="12" /> {{ c.label }}
			</span>
		</div>

		<Popover v-if="focused && suggestions.length && anchor" :anchor="anchor" @close="token = null">
			<div class="menu suggest" @mousedown.prevent>
				<button
					v-for="(s, i) in suggestions"
					:key="s.key"
					class="menu-item"
					:class="{active: i === active}"
					@mouseenter="active = i"
					@click="pick(s)"
				>
					<span v-if="s.emoji" class="emoji">{{ s.emoji }}</span>
					<component :is="s.icon" v-else-if="s.icon" :size="15" :style="s.color ? {color: s.color} : {}" />
					{{ s.label }}
				</button>
			</div>
		</Popover>
		<Popover v-if="pickingDate" :anchor="dateBtn" placement="bottom-end" @close="pickingDate = false">
			<DatePicker @apply="p => { pickedDate = p; input?.focus() }" @close="pickingDate = false" />
		</Popover>
	</div>
</template>

<style scoped>
.qa {
	border-radius: 8px;
	background: var(--bg-input);
	border: 1px solid transparent;
}

.qa.focused {
	border-color: var(--accent);
	background: var(--bg);
}

.bar {
	display: flex;
	align-items: center;
	gap: 8px;
	height: 42px;
	padding: 0 8px 0 14px;
}

.plus {
	color: var(--text-3);
	flex-shrink: 0;
}

.field {
	position: relative;
	flex: 1;
	min-width: 0;
	height: 100%;
}

.backdrop,
input {
	position: absolute;
	inset: 0;
	padding: 0;
	font: inherit;
	font-size: 14px;
	line-height: 42px;
	white-space: pre;
}

.backdrop {
	overflow: hidden;
	color: var(--text);
	pointer-events: none;
}

.backdrop :deep(mark) {
	color: var(--accent);
	background: var(--accent-soft);
	border-radius: 3px;
}

input {
	width: 100%;
	border: 0;
	outline: 0;
	background: transparent;
	color: transparent;
	caret-color: var(--text);
}

input::placeholder {
	color: var(--text-3);
}

.tools {
	display: flex;
	gap: 2px;
	flex-shrink: 0;
}

.tool {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	height: 28px;
	min-width: 28px;
	padding: 0 6px;
	border-radius: 6px;
	color: var(--text-3);
}

.tool:hover {
	background: var(--bg-hover);
	color: var(--text);
}

.tool.on {
	color: var(--accent);
}

.tool-label {
	font-size: 12px;
}

.chips {
	display: flex;
	flex-wrap: wrap;
	gap: 6px;
	padding: 0 14px 10px 40px;
}

.chip {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	padding: 2px 8px;
	border-radius: 10px;
	font-size: 12px;
	color: var(--accent);
	background: var(--accent-soft);
}

.suggest {
	min-width: 200px;
	max-height: 280px;
	overflow-y: auto;
}

.emoji {
	width: 15px;
	text-align: center;
}
</style>
