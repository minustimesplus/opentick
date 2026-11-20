import {afterEach, describe, expect, it, vi} from 'vitest'
import {addShortcuts, dispatchKey, formatBinding, formatSequence, keyToken, rebind, resetShortcuts, binding} from './shortcuts'

function key(k: string, opts: Partial<KeyboardEventInit> & {code?: string} = {}, target: EventTarget = document.body): KeyboardEvent {
	const e = new KeyboardEvent('keydown', {key: k, bubbles: true, cancelable: true, ...opts})
	Object.defineProperty(e, 'target', {value: target})
	return e
}

afterEach(() => resetShortcuts())

describe('keyToken', () => {
	it('names plain keys, symbols and modifiers', () => {
		expect(keyToken(key('n'))).toBe('n')
		expect(keyToken(key('N', {shiftKey: true}))).toBe('shift+n')
		expect(keyToken(key('?', {shiftKey: true}))).toBe('?')
		expect(keyToken(key('Z', {ctrlKey: true, shiftKey: true, code: 'KeyZ'}))).toBe('ctrl+shift+z')
		expect(keyToken(key('s', {metaKey: true, code: 'KeyS'}))).toBe('ctrl+s')
		expect(keyToken(key('Enter', {shiftKey: true}))).toBe('shift+enter')
		expect(keyToken(key('Shift', {shiftKey: true}))).toBeNull()
	})

	it('reads letters and digits from the key position when Alt changes the character', () => {
		expect(keyToken(key('¡', {altKey: true, code: 'Digit1'}))).toBe('alt+1')
	})
})

describe('formatting', () => {
	it('shows sequences the way the settings list them', () => {
		expect(formatSequence('g c')).toBe('G then C')
		expect(formatSequence('tab m')).toBe('Tab+M')
		expect(formatSequence('ctrl+shift+z')).toBe('Ctrl+Shift+Z')
		expect(formatSequence('ctrl+delete')).toBe('Ctrl+Del')
		expect(formatBinding('dayView')).toBe('D / 1')
	})
})

describe('dispatchKey', () => {
	it('runs single keys and two-key sequences', () => {
		const calendar = vi.fn()
		const add = vi.fn()
		const off = addShortcuts({goCalendar: calendar, addTask: add})
		expect(dispatchKey(key('n'))).toBe(true)
		expect(add).toHaveBeenCalledOnce()
		expect(dispatchKey(key('g'))).toBe(true)
		expect(calendar).not.toHaveBeenCalled()
		dispatchKey(key('c'))
		expect(calendar).toHaveBeenCalledOnce()
		off()
	})

	it('ignores plain keys in text fields but not Ctrl shortcuts', () => {
		const add = vi.fn()
		const save = vi.fn()
		const off = addShortcuts({addTask: add, save})
		const input = document.createElement('input')
		document.body.append(input)
		expect(dispatchKey(key('n', {}, input))).toBe(false)
		expect(dispatchKey(key('s', {ctrlKey: true, code: 'KeyS'}, input))).toBe(true)
		expect(add).not.toHaveBeenCalled()
		expect(save).toHaveBeenCalledOnce()
		input.remove()
		off()
	})

	it('lets Tab through unless a Tab sequence applies', () => {
		const complete = vi.fn()
		let hasTask = false
		const off = addShortcuts({complete: {when: () => hasTask, run: complete}})
		expect(dispatchKey(key('Tab'))).toBe(false)
		hasTask = true
		expect(dispatchKey(key('Tab'))).toBe(true)
		dispatchKey(key('m'))
		expect(complete).toHaveBeenCalledOnce()
		off()
	})

	it('prefers the latest layer and falls back when a handler declines', () => {
		const outer = vi.fn()
		const inner = vi.fn(() => false)
		const offOuter = addShortcuts({sync: outer})
		const offInner = addShortcuts({sync: inner})
		dispatchKey(key('.'))
		expect(inner).toHaveBeenCalledOnce()
		expect(outer).toHaveBeenCalledOnce()
		offInner()
		offOuter()
	})

	it('does nothing while blocked', () => {
		const add = vi.fn()
		const off = addShortcuts({addTask: add})
		expect(dispatchKey(key('n'), true)).toBe(false)
		expect(add).not.toHaveBeenCalled()
		off()
	})
})

describe('rebind', () => {
	it('moves keys from the shortcut that had them', () => {
		expect(rebind('goAi', 'g c')).toBe('goCalendar')
		expect(binding('goAi')).toEqual(['g c'])
		expect(binding('goCalendar')).toEqual([])
		resetShortcuts()
		expect(binding('goCalendar')).toEqual(['g c'])
	})

	it('runs the rebound action', () => {
		const ai = vi.fn()
		const off = addShortcuts({goAi: ai})
		rebind('goAi', 'alt+a')
		dispatchKey(key('å', {altKey: true, code: 'KeyA'}))
		expect(ai).toHaveBeenCalledOnce()
		off()
	})
})
