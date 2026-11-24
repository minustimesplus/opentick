import {shallowRef} from 'vue'

/**
 * The focus timer's pop-out mini window (TickTick's desktop "mini window"):
 * a small window that stays on top of other apps.
 *
 * - Desktop app: a frameless, always-on-top child window (web-desktop/main.js
 *   allows `window.open` for POPOUT_NAME).
 * - Chromium browsers: Document Picture-in-Picture, which also stays on top.
 * - Other browsers: a plain popup window.
 *
 * Every variant is a same-origin window this page scripts directly, so the
 * mini timer is rendered into it with a Vue <Teleport> and shares the
 * running app's state; nothing is synced between windows.
 */

export const POPOUT_NAME = 'opentick-focus-mini'
const WIDTH = 300
const HEIGHT = 168

interface DocumentPictureInPicture {
	requestWindow(options?: {width?: number, height?: number}): Promise<Window>
}

declare global {
	interface Window {
		documentPictureInPicture?: DocumentPictureInPicture
	}
}

/** The open mini window, or null. */
export const popout = shallowRef<Window | null>(null)
/** The element in the mini window that the timer is teleported into. */
export const popoutRoot = shallowRef<HTMLElement | null>(null)

let observer: MutationObserver | null = null

/** Whether this browser can keep the mini window above other apps. */
export function popoutStaysOnTop(): boolean {
	return !!window.ttDesktop || !!window.documentPictureInPicture
}

function copyStyles(target: Document) {
	for (const node of document.head.querySelectorAll('style, link[rel="stylesheet"]')) {
		const copy = node.cloneNode(true) as HTMLElement
		// The new document has no base URL of its own.
		if (node instanceof HTMLLinkElement) copy.setAttribute('href', node.href)
		target.head.appendChild(copy)
	}
}

/** Mirrors the theme attributes and inline variables of <html> onto the mini window. */
function mirrorRoot(target: Document) {
	const sync = () => {
		const from = document.documentElement
		const to = target.documentElement
		to.className = from.className
		to.lang = from.lang
		to.setAttribute('style', from.getAttribute('style') ?? '')
		for (const key of Object.keys(to.dataset)) delete to.dataset[key]
		Object.assign(to.dataset, from.dataset)
	}
	sync()
	observer?.disconnect()
	observer = new MutationObserver(sync)
	observer.observe(document.documentElement, {attributes: true})
}

function prepare(win: Window) {
	const doc = win.document
	doc.title = 'OpenTick'
	copyStyles(doc)
	mirrorRoot(doc)
	doc.body.classList.add('focus-popout')
	const root = doc.createElement('div')
	root.id = 'focus-popout-root'
	doc.body.appendChild(root)
	popoutRoot.value = root
	win.addEventListener('pagehide', () => {
		if (popout.value === win) close()
	})
}

async function openWindow(): Promise<Window | null> {
	if (!window.ttDesktop && window.documentPictureInPicture) {
		return window.documentPictureInPicture.requestWindow({width: WIDTH, height: HEIGHT})
	}
	const left = Math.max(0, window.screenX + window.outerWidth - WIDTH - 40)
	const top = Math.max(0, window.screenY + 80)
	return window.open('', POPOUT_NAME, `popup,width=${WIDTH},height=${HEIGHT},left=${left},top=${top}`)
}

/** Opens the mini window (or brings it forward). Call from a click: browsers require a user gesture. */
export async function openPopout(): Promise<boolean> {
	if (popout.value && !popout.value.closed) {
		popout.value.focus()
		return true
	}
	let win: Window | null = null
	try {
		win = await openWindow()
	} catch {
		win = null
	}
	if (!win) return false
	// A popup that survived a reload of the main window still shows the old content.
	win.document.body.replaceChildren()
	win.document.head.replaceChildren()
	prepare(win)
	popout.value = win
	return true
}

export function close() {
	const win = popout.value
	popout.value = null
	popoutRoot.value = null
	observer?.disconnect()
	observer = null
	if (win && !win.closed) win.close()
}

// A popup outlives the page that opened it; it would be left empty.
window.addEventListener('pagehide', () => close())
