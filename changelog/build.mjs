#!/usr/bin/env node
// Builds the static changelog site published to GitHub Pages.
//
// Every desktop-v* tag is one release (the desktop release workflow tags each
// push to main). The entries are the Conventional Commit subjects between
// consecutive tags, so the page needs no hand-maintained changelog file.
//
// Usage: node changelog/build.mjs [outDir]   (default: changelog/dist)
// Needs the full history and tags (fetch-depth: 0 in CI).

import {execFileSync} from 'node:child_process'
import {copyFileSync, mkdirSync, writeFileSync} from 'node:fs'
import {dirname, join} from 'node:path'
import {fileURLToPath} from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const outDir = process.argv[2] ?? join(root, 'changelog', 'dist')
const repo = process.env.GITHUB_REPOSITORY ?? 'minustimesplus/opentick'
const repoUrl = `https://github.com/${repo}`

// Last upstream Vikunja commit before OpenTick started; the first release's
// changes are counted from here instead of from the beginning of history.
const forkBase = '024654319'

const groups = [
	{title: 'Features', types: ['feat']},
	{title: 'Fixes', types: ['fix']},
	{title: 'Performance', types: ['perf']},
	{title: 'Other changes', types: null},
]

function git(...args) {
	return execFileSync('git', args, {cwd: root, encoding: 'utf8'}).trim()
}

function commits(range) {
	const out = git('log', '--no-merges', '--format=%h%x1f%s%x1e', range)
	return out.split('\x1e').map(s => s.trim()).filter(Boolean).map(line => {
		const [hash, subject] = line.split('\x1f')
		const m = subject.match(/^(\w+)(?:\(([^)]+)\))?(!)?:\s*(.+)$/)
		let text = m ? m[4] : subject
		const pr = text.match(/\s*\(#(\d+)\)$/)
		if (pr) text = text.slice(0, pr.index)
		return {
			hash,
			type: m ? m[1].toLowerCase() : '',
			scope: m?.[2] ?? '',
			breaking: Boolean(m?.[3]),
			text: text.charAt(0).toUpperCase() + text.slice(1),
			pr: pr?.[1],
		}
	})
}

const tags = git('tag', '--list', 'desktop-v*', '--sort=v:refname').split('\n').filter(Boolean)
const releases = tags.map((tag, i) => ({
	version: tag.replace(/^desktop-v/, ''),
	tag,
	date: git('log', '-1', '--format=%cs', tag),
	commits: commits(`${i ? tags[i - 1] : forkBase}..${tag}`),
}))
const unreleased = commits(`${tags.at(-1) ?? forkBase}..HEAD`)
if (unreleased.length) releases.push({version: null, tag: null, date: null, commits: unreleased})
releases.reverse()

const esc = s => s.replace(/[&<>"']/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', '\'': '&#39;'}[c]))

function entry(c) {
	const scope = c.scope ? `<span class="scope">${esc(c.scope)}</span>` : ''
	const breaking = c.breaking ? '<span class="breaking">Breaking</span>' : ''
	const ref = c.pr
		? `<a class="ref" href="${repoUrl}/pull/${c.pr}">#${c.pr}</a>`
		: `<a class="ref" href="${repoUrl}/commit/${c.hash}">${c.hash}</a>`
	return `<li>${breaking}${scope}<span class="text">${esc(c.text)}</span>${ref}</li>`
}

function release(r) {
	const id = r.version ? `v${r.version}` : 'unreleased'
	const heading = r.version
		? `<a href="#${id}">${esc(r.version)}</a>`
		: 'Unreleased'
	const meta = r.version
		? `<time datetime="${r.date}">${r.date}</time> · <a href="${repoUrl}/releases/tag/${r.tag}">Downloads</a>`
		: 'On main, not yet released'
	const sections = groups.map(g => {
		const items = r.commits.filter(c => g.types
			? g.types.includes(c.type)
			: !groups.some(o => o.types?.includes(c.type)))
		if (!items.length) return ''
		const list = `<ul>${items.map(entry).join('')}</ul>`
		return g.types
			? `<h3>${g.title}</h3>${list}`
			: `<details><summary>${g.title} (${items.length})</summary>${list}</details>`
	}).join('')
	return `<section class="release" id="${id}">
<header><h2>${heading}</h2><p class="meta">${meta}</p></header>
${sections || '<p class="empty">No changes recorded.</p>'}
</section>`
}

const html = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>OpenTick Changelog</title>
<meta name="description" content="What changed in each OpenTick release.">
<link rel="icon" href="favicon.svg" type="image/svg+xml">
<style>
:root {
	--bg: #ffffff;
	--bg-soft: #f6f7f9;
	--border: #ececec;
	--text: #191919;
	--text-2: #4a4a4a;
	--text-3: #8c8c8c;
	--accent: #4772fa;
	--danger: #e03131;
	color-scheme: light;
}
@media (prefers-color-scheme: dark) {
	:root {
		--bg: #1c1c1c;
		--bg-soft: #242424;
		--border: #2d2d2d;
		--text: #e8e8e8;
		--text-2: #b8b8b8;
		--text-3: #868686;
		--danger: #ff6b6b;
		color-scheme: dark;
	}
}
* { box-sizing: border-box; }
body {
	margin: 0;
	background: var(--bg);
	color: var(--text);
	font: 15px/1.6 "Ubuntu", "Segoe UI", system-ui, -apple-system, "Helvetica Neue", Arial, sans-serif;
}
main { max-width: 760px; margin: 0 auto; padding: 48px 16px 80px; }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.top { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
.top img { width: 36px; height: 36px; }
h1 { margin: 0; font-size: 28px; font-weight: 500; }
.intro { margin: 0 0 40px; color: var(--text-2); }
.release { padding: 28px 0; border-top: 1px solid var(--border); scroll-margin-top: 16px; }
.release:target header h2 a { color: var(--accent); }
.release header { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 16px; }
.release h2 { margin: 0; font-size: 22px; font-weight: 500; }
.release h2 a { color: var(--text); }
.meta { margin: 0; font-size: 13px; color: var(--text-3); }
h3 { margin: 18px 0 6px; font-size: 13px; font-weight: 500; text-transform: uppercase; letter-spacing: .04em; color: var(--text-3); }
ul { margin: 0; padding: 0; list-style: none; }
li { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 8px; padding: 4px 0; color: var(--text-2); }
.text { flex: 1 1 240px; min-width: 0; overflow-wrap: anywhere; }
.scope, .breaking {
	flex: none;
	padding: 0 7px;
	border-radius: 4px;
	background: var(--bg-soft);
	border: 1px solid var(--border);
	font-size: 12px;
	color: var(--text-3);
}
.breaking { color: var(--danger); border-color: currentColor; }
.ref { flex: none; font: 12px ui-monospace, SFMono-Regular, Menlo, monospace; }
details { margin-top: 14px; }
summary { cursor: pointer; font-size: 13px; color: var(--text-3); }
details ul { margin-top: 6px; }
.empty { margin: 12px 0 0; font-size: 13px; color: var(--text-3); }
footer { margin-top: 40px; font-size: 13px; color: var(--text-3); }
</style>
</head>
<body>
<main>
<div class="top"><img src="favicon.svg" alt=""><h1>OpenTick Changelog</h1></div>
<p class="intro">Every release of the OpenTick web app and desktop app. Your installed version is under Settings → About.</p>
${releases.map(release).join('\n')}
<footer><a href="${repoUrl}">${esc(repo)}</a> on GitHub</footer>
</main>
</body>
</html>
`

mkdirSync(outDir, {recursive: true})
writeFileSync(join(outDir, 'index.html'), html)
copyFileSync(join(root, 'web', 'public', 'favicon.svg'), join(outDir, 'favicon.svg'))
// GitHub Pages would otherwise run the output through Jekyll.
writeFileSync(join(outDir, '.nojekyll'), '')
console.log(`Wrote ${releases.length} releases to ${outDir}`)
