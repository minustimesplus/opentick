// Minimal, dependency-free markdown renderer for task descriptions.
// All input is HTML-escaped first, so the output is safe for v-html.

function esc(s: string): string {
	return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

function safeUrl(u: string): string {
	return /^(https?:|mailto:|\/|#)/i.test(u) ? u : ''
}

function inline(src: string): string {
	const codes: string[] = []
	let s = esc(src).replace(/`([^`]+)`/g, (_, c) => {
		codes.push(`<code>${c}</code>`)
		return `\u0000${codes.length - 1}\u0000`
	})
	s = s
		.replace(/!?\[([^\]]+)\]\(([^)\s]+)\)/g, (_, text, url) => {
			const href = safeUrl(url.replace(/&amp;/g, '&'))
			return href ? `<a href="${esc(href)}" target="_blank" rel="noopener noreferrer">${text}</a>` : text
		})
		.replace(/(^|[\s(])(https?:\/\/[^\s<)]+)/g, '$1<a href="$2" target="_blank" rel="noopener noreferrer">$2</a>')
		.replace(/\*\*([^*]+)\*\*|__([^_]+)__/g, (_, a, b) => `<strong>${a ?? b}</strong>`)
		.replace(/(^|[^*])\*([^*\s][^*]*)\*(?!\*)/g, '$1<em>$2</em>')
		.replace(/(^|[^\w_])_([^_\s][^_]*)_(?!\w)/g, '$1<em>$2</em>')
		.replace(/~~([^~]+)~~/g, '<del>$1</del>')
	return s.replace(/\u0000(\d+)\u0000/g, (_, i) => codes[+i]!)
}

const TASK_RE = /^(\s*)[-*+]\s+\[([ xX])\]\s+(.*)$/
const UL_RE = /^(\s*)[-*+]\s+(.*)$/
const OL_RE = /^(\s*)\d+[.)]\s+(.*)$/

export function renderMarkdown(src: string): string {
	const lines = src.replace(/\r\n?/g, '\n').split('\n')
	const out: string[] = []
	let i = 0
	while (i < lines.length) {
		const line = lines[i]!

		const fence = line.match(/^```(\w*)\s*$/)
		if (fence) {
			const buf: string[] = []
			i++
			while (i < lines.length && !/^```\s*$/.test(lines[i]!)) buf.push(lines[i++]!)
			i++
			out.push(`<pre><code>${esc(buf.join('\n'))}</code></pre>`)
			continue
		}

		const h = line.match(/^(#{1,6})\s+(.*)$/)
		if (h) {
			out.push(`<h${h[1]!.length}>${inline(h[2]!)}</h${h[1]!.length}>`)
			i++
			continue
		}

		if (/^\s*([-*_])(\s*\1){2,}\s*$/.test(line)) {
			out.push('<hr>')
			i++
			continue
		}

		if (/^>\s?/.test(line)) {
			const buf: string[] = []
			while (i < lines.length && /^>\s?/.test(lines[i]!)) buf.push(lines[i++]!.replace(/^>\s?/, ''))
			out.push(`<blockquote>${buf.map(inline).join('<br>')}</blockquote>`)
			continue
		}

		if (TASK_RE.test(line) || UL_RE.test(line) || OL_RE.test(line)) {
			const ordered = OL_RE.test(line) && !TASK_RE.test(line) && !UL_RE.test(line)
			const items: string[] = []
			while (i < lines.length) {
				const l = lines[i]!
				const task = l.match(TASK_RE)
				const ul = l.match(UL_RE)
				const ol = l.match(OL_RE)
				const indent = Math.min(3, Math.floor(((task ?? ul ?? ol)?.[1]?.length ?? 0) / 2))
				if (task) {
					const done = task[2] !== ' '
					items.push(`<li class="task${done ? ' done' : ''}" style="margin-left:${indent * 18}px"><input type="checkbox" data-line="${i}"${done ? ' checked' : ''}><span>${inline(task[3]!)}</span></li>`)
				} else if (!ordered && ul) {
					items.push(`<li style="margin-left:${indent * 18}px">${inline(ul[2]!)}</li>`)
				} else if (ordered && ol) {
					items.push(`<li style="margin-left:${indent * 18}px">${inline(ol[2]!)}</li>`)
				} else {
					break
				}
				i++
			}
			out.push(`<${ordered ? 'ol' : 'ul'}>${items.join('')}</${ordered ? 'ol' : 'ul'}>`)
			continue
		}

		if (line.trim() === '') {
			i++
			continue
		}

		const buf: string[] = []
		while (i < lines.length && lines[i]!.trim() !== '' && !/^(#{1,6}\s|```|>|\s*[-*+]\s|\s*\d+[.)]\s)/.test(lines[i]!)) buf.push(lines[i++]!)
		if (buf.length === 0) buf.push(lines[i++]!)
		out.push(`<p>${buf.map(inline).join('<br>')}</p>`)
	}
	return out.join('')
}

/** Toggle `- [ ]` / `- [x]` on the given source line. */
export function toggleTaskLine(src: string, lineIndex: number): string {
	const lines = src.split('\n')
	const l = lines[lineIndex]
	if (l === undefined) return src
	lines[lineIndex] = l.replace(/\[([ xX])\]/, (_, c) => c === ' ' ? '[x]' : '[ ]')
	return lines.join('\n')
}
