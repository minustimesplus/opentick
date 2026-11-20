import {describe, expect, it} from 'vitest'
import {clearHistory, record, redo, undo} from './history'

describe('history', () => {
	it('undoes and redoes, without recording the replay', async () => {
		clearHistory()
		let value = 0
		const set = (v: number) => {
			const old = value
			value = v
			record({undo: () => set(old), redo: () => set(v)})
		}
		set(1)
		set(2)
		expect(await undo()).toBe(true)
		expect(value).toBe(1)
		expect(await undo()).toBe(true)
		expect(value).toBe(0)
		expect(await undo()).toBe(false)
		expect(await redo()).toBe(true)
		expect(value).toBe(1)
		expect(await redo()).toBe(true)
		expect(value).toBe(2)
		expect(await redo()).toBe(false)
	})

	it('merges quick changes with the same key into one step', async () => {
		clearHistory()
		let title = 'a'
		const set = (v: string) => {
			const old = title
			title = v
			record({key: 'title', undo: () => set(old), redo: () => set(v)})
		}
		set('ab')
		set('abc')
		await undo()
		expect(title).toBe('a')
		await redo()
		expect(title).toBe('abc')
	})
})
