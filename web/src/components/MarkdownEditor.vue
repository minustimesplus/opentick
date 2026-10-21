<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue'
import {renderMarkdown, toggleTaskLine} from '@/lib/markdown'
import {t, type MessageKey} from '@/lib/i18n'

const props = defineProps<{modelValue: string, placeholder?: string}>()
const emit = defineEmits<{'update:modelValue': [v: string], focus: [], blur: []}>()

const el = ref<HTMLTextAreaElement | null>(null)
const editing = ref(false)
const html = computed(() => renderMarkdown(props.modelValue))
const showPreview = computed(() => !editing.value && props.modelValue.trim() !== '')

function autosize() {
	const ta = el.value
	if (!ta) return
	ta.style.height = 'auto'
	ta.style.height = ta.scrollHeight + 'px'
}

watch(() => props.modelValue, () => nextTick(autosize))

function focus() {
	editing.value = true
	nextTick(() => {
		el.value?.focus()
		autosize()
	})
}
defineExpose({focus})

function onPreviewClick(e: MouseEvent) {
	const target = e.target as HTMLElement
	if (target.closest('a')) return
	if (target instanceof HTMLInputElement && target.type === 'checkbox') {
		const line = Number(target.dataset.line)
		emit('update:modelValue', toggleTaskLine(props.modelValue, line))
		e.preventDefault()
		return
	}
	focus()
}

function onBlur() {
	slash.value = null
	editing.value = false
	emit('blur')
}

// --- text editing helpers ---------------------------------------------------

function set(value: string, caret: number, caretEnd = caret) {
	emit('update:modelValue', value)
	nextTick(() => {
		const ta = el.value
		if (!ta) return
		ta.value = value
		ta.setSelectionRange(caret, caretEnd)
		autosize()
	})
}

function lineBounds(value: string, pos: number) {
	const start = value.lastIndexOf('\n', pos - 1) + 1
	let end = value.indexOf('\n', pos)
	if (end === -1) end = value.length
	return {start, end}
}

function wrap(mark: string) {
	const ta = el.value!
	const {selectionStart: s, selectionEnd: e, value} = ta
	const sel = value.slice(s, e)
	if (sel.startsWith(mark) && sel.endsWith(mark) && sel.length >= mark.length * 2) {
		set(value.slice(0, s) + sel.slice(mark.length, -mark.length) + value.slice(e), s, e - mark.length * 2)
		return
	}
	set(value.slice(0, s) + mark + sel + mark + value.slice(e), s + mark.length, e + mark.length)
}

// --- slash commands ---------------------------------------------------------

interface Command {
	id: string
	label: MessageKey
	hint: string
	keywords: string
	/** Text replacing the "/query"; `|` marks where the caret ends up. */
	insert: string
	/** Line-prefix commands only apply at the start of a line. */
	block?: boolean
}

const COMMANDS: Command[] = [
	{id: 'h1', label: 'mdH1', hint: '#', keywords: 'heading title h1', insert: '# |', block: true},
	{id: 'h2', label: 'mdH2', hint: '##', keywords: 'heading subtitle h2', insert: '## |', block: true},
	{id: 'h3', label: 'mdH3', hint: '###', keywords: 'heading h3', insert: '### |', block: true},
	{id: 'bullet', label: 'mdBullet', hint: '-', keywords: 'bullet list ul unordered', insert: '- |', block: true},
	{id: 'numbered', label: 'mdNumbered', hint: '1.', keywords: 'numbered ordered list ol', insert: '1. |', block: true},
	{id: 'todo', label: 'mdChecklist', hint: '[ ]', keywords: 'todo task checklist checkbox', insert: '- [ ] |', block: true},
	{id: 'quote', label: 'mdQuote', hint: '>', keywords: 'quote blockquote', insert: '> |', block: true},
	{id: 'code', label: 'mdCode', hint: '```', keywords: 'code block snippet', insert: '```\n|\n```', block: true},
	{id: 'divider', label: 'mdDivider', hint: '---', keywords: 'divider hr line separator', insert: '---\n|', block: true},
	{id: 'bold', label: 'mdBold', hint: '**', keywords: 'bold strong', insert: '**|**'},
	{id: 'italic', label: 'mdItalic', hint: '*', keywords: 'italic emphasis', insert: '*|*'},
	{id: 'strike', label: 'mdStrike', hint: '~~', keywords: 'strikethrough strike', insert: '~~|~~'},
	{id: 'link', label: 'mdLink', hint: '[]()', keywords: 'link url', insert: '[|](https://)'},
]

const slash = ref<{start: number, query: string, top: number} | null>(null)
const slashIndex = ref(0)

const slashItems = computed(() => {
	if (!slash.value) return []
	const q = slash.value.query.toLowerCase()
	const ta = el.value
	const atLineStart = ta ? lineBounds(ta.value, slash.value.start).start === slash.value.start : true
	return COMMANDS.filter(c => (atLineStart || !c.block) && (!q || c.keywords.includes(q) || t(c.label).toLowerCase().includes(q)))
})

watch(slashItems, items => {
	if (slash.value && items.length === 0 && slash.value.query.length > 2) slash.value = null
	slashIndex.value = 0
})

function updateSlash() {
	const ta = el.value
	if (!ta || ta.selectionStart !== ta.selectionEnd) {
		slash.value = null
		return
	}
	const pos = ta.selectionStart
	const {start} = lineBounds(ta.value, pos)
	const before = ta.value.slice(start, pos)
	const m = before.match(/(^|\s)\/(\w*)$/)
	if (!m) {
		slash.value = null
		return
	}
	const lines = ta.value.slice(0, pos).split('\n').length
	const lh = parseFloat(getComputedStyle(ta).lineHeight) || 22
	slash.value = {start: pos - m[2]!.length - 1, query: m[2]!, top: lines * lh + 4}
}

function applyCommand(c: Command) {
	const ta = el.value
	if (!ta || !slash.value) return
	const from = slash.value.start
	const to = ta.selectionStart
	const caretAt = c.insert.indexOf('|')
	const text = c.insert.replace('|', '')
	const value = ta.value.slice(0, from) + text + ta.value.slice(to)
	slash.value = null
	set(value, from + caretAt)
}

// --- key handling -----------------------------------------------------------

const LIST_RE = /^(\s*)(?:([-*+])\s+(\[[ xX]\]\s+)?|(\d+)([.)])\s+)/

function onKeydown(e: KeyboardEvent) {
	const ta = el.value!

	if (slash.value && slashItems.value.length) {
		const n = slashItems.value.length
		if (e.key === 'ArrowDown') { e.preventDefault(); slashIndex.value = (slashIndex.value + 1) % n; return }
		if (e.key === 'ArrowUp') { e.preventDefault(); slashIndex.value = (slashIndex.value - 1 + n) % n; return }
		if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); applyCommand(slashItems.value[slashIndex.value]!); return }
	}
	if (e.key === 'Escape') {
		e.preventDefault()
		if (slash.value) slash.value = null
		else ta.blur()
		return
	}

	const mod = e.ctrlKey || e.metaKey
	if (mod && !e.shiftKey && (e.key === 'b' || e.key === 'i')) {
		e.preventDefault()
		wrap(e.key === 'b' ? '**' : '*')
		return
	}
	if (mod && e.shiftKey && e.key.toLowerCase() === 'x') {
		e.preventDefault()
		wrap('~~')
		return
	}

	if (e.key === 'Enter' && !e.shiftKey && !mod && ta.selectionStart === ta.selectionEnd) {
		const pos = ta.selectionStart
		const {start, end} = lineBounds(ta.value, pos)
		const line = ta.value.slice(start, end)
		const m = line.match(LIST_RE)
		if (m && pos >= start + m[0].length) {
			e.preventDefault()
			if (line.length === m[0].length) {
				// empty item: leave the list
				set(ta.value.slice(0, start) + ta.value.slice(end), start)
				return
			}
			const marker = m[4]
				? `${m[1]}${Number(m[4]) + 1}${m[5]} `
				: `${m[1]}${m[2]} ${m[3] ? '[ ] ' : ''}`
			set(ta.value.slice(0, pos) + '\n' + marker + ta.value.slice(pos), pos + 1 + marker.length)
		}
		return
	}

	if (e.key === 'Tab') {
		const {start} = lineBounds(ta.value, ta.selectionStart)
		const line = ta.value.slice(start, lineBounds(ta.value, ta.selectionStart).end)
		if (!LIST_RE.test(line)) return
		e.preventDefault()
		if (e.shiftKey) {
			const stripped = line.replace(/^ {1,2}/, '')
			const removed = line.length - stripped.length
			set(ta.value.slice(0, start) + stripped + ta.value.slice(start + line.length), Math.max(start, ta.selectionStart - removed))
		} else {
			set(ta.value.slice(0, start) + '  ' + ta.value.slice(start), ta.selectionStart + 2)
		}
	}
}

function onInput(e: Event) {
	emit('update:modelValue', (e.target as HTMLTextAreaElement).value)
	autosize()
	updateSlash()
}
</script>

<template>
	<div class="md-editor">
		<!-- eslint-disable-next-line vue/no-v-html -->
		<div v-if="showPreview" class="md-preview" tabindex="0" @click="onPreviewClick" @focus="focus" v-html="html" />
		<textarea
			v-show="!showPreview"
			ref="el"
			class="md-input"
			rows="3"
			:value="modelValue"
			:placeholder="placeholder"
			spellcheck="true"
			@input="onInput"
			@keydown="onKeydown"
			@keyup="e => ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key) && updateSlash()"
			@click="updateSlash"
			@focus="editing = true; emit('focus')"
			@blur="onBlur"
		/>
		<div v-if="slash && slashItems.length" class="slash-menu menu" :style="{top: slash.top + 'px'}" @mousedown.prevent>
			<button
				v-for="(c, i) in slashItems"
				:key="c.id"
				class="menu-item"
				:class="{active: i === slashIndex}"
				@mouseenter="slashIndex = i"
				@click="applyCommand(c)"
			>
				<span class="hint">{{ c.hint }}</span>
				{{ t(c.label) }}
			</button>
		</div>
	</div>
</template>

<style scoped>
.md-editor {
	position: relative;
}

.md-input {
	display: block;
	width: 100%;
	min-height: 120px;
	border: 0;
	outline: 0;
	resize: none;
	overflow: hidden;
	background: transparent;
	font: inherit;
	font-size: 14px;
	line-height: 1.6;
	color: var(--text-2);
}

.md-input::placeholder {
	color: var(--text-4);
}

.md-preview {
	min-height: 120px;
	font-size: 14px;
	line-height: 1.6;
	color: var(--text-2);
	cursor: text;
	outline: 0;
	word-break: break-word;
}

.md-preview :deep(p),
.md-preview :deep(ul),
.md-preview :deep(ol),
.md-preview :deep(blockquote),
.md-preview :deep(pre) {
	margin: 0 0 8px;
}

.md-preview :deep(h1),
.md-preview :deep(h2),
.md-preview :deep(h3),
.md-preview :deep(h4),
.md-preview :deep(h5),
.md-preview :deep(h6) {
	margin: 12px 0 6px;
	color: var(--text);
	line-height: 1.3;
}

.md-preview :deep(h1) { font-size: 20px; }
.md-preview :deep(h2) { font-size: 17px; }
.md-preview :deep(h3) { font-size: 15px; }

.md-preview :deep(strong) { color: var(--text); }

.md-preview :deep(a) {
	color: var(--accent);
	text-decoration: none;
}

.md-preview :deep(a:hover) {
	text-decoration: underline;
}

.md-preview :deep(code) {
	padding: 1px 4px;
	border-radius: 4px;
	background: var(--bg-active);
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 13px;
}

.md-preview :deep(pre) {
	padding: 10px 12px;
	border-radius: 6px;
	background: var(--bg-active);
	overflow-x: auto;
}

.md-preview :deep(pre code) {
	padding: 0;
	background: none;
}

.md-preview :deep(blockquote) {
	padding-left: 10px;
	border-left: 3px solid var(--border-strong);
	color: var(--text-3);
}

.md-preview :deep(hr) {
	border: 0;
	border-top: 1px solid var(--border-strong);
	margin: 12px 0;
}

.md-preview :deep(ul),
.md-preview :deep(ol) {
	padding-left: 20px;
}

.md-preview :deep(li.task) {
	display: flex;
	align-items: baseline;
	gap: 8px;
	list-style: none;
	margin-left: -20px;
}

.md-preview :deep(li.task input) {
	accent-color: var(--accent);
	cursor: pointer;
}

.md-preview :deep(li.task.done span) {
	color: var(--text-4);
	text-decoration: line-through;
}

.slash-menu {
	position: absolute;
	left: 0;
	z-index: 20;
	min-width: 220px;
	max-height: 260px;
	overflow-y: auto;
}

.hint {
	width: 28px;
	color: var(--text-4);
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 12px;
}
</style>
