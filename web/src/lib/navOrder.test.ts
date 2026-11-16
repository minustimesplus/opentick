import {describe, expect, it} from 'vitest'
import {moveKey, orderKeys} from './navOrder'

const defaults = ['tasks', 'matrix', 'calendar', 'habits', 'search']

describe('orderKeys', () => {
	it('keeps the defaults without a saved order', () => {
		expect(orderKeys(defaults, [])).toEqual(defaults)
	})

	it('follows the saved order', () => {
		const saved = ['calendar', 'tasks', 'search', 'matrix', 'habits']
		expect(orderKeys(defaults, saved)).toEqual(saved)
	})

	it('drops unknown keys and slots new ones after their default neighbour', () => {
		expect(orderKeys(defaults, ['calendar', 'gone', 'tasks', 'matrix'])).toEqual(['calendar', 'habits', 'search', 'tasks', 'matrix'])
		expect(orderKeys(defaults, ['matrix', 'calendar'])).toEqual(['tasks', 'matrix', 'calendar', 'habits', 'search'])
	})
})

describe('moveKey', () => {
	it('moves down past the target', () => {
		expect(moveKey(defaults, 'matrix', 'habits')).toEqual(['tasks', 'calendar', 'habits', 'matrix', 'search'])
	})

	it('moves up before the target', () => {
		expect(moveKey(defaults, 'search', 'matrix')).toEqual(['tasks', 'search', 'matrix', 'calendar', 'habits'])
	})

	it('jumps over hidden keys between visible ones', () => {
		// habits hidden: moving search onto calendar lands before calendar
		expect(moveKey(defaults, 'search', 'calendar')).toEqual(['tasks', 'matrix', 'search', 'calendar', 'habits'])
	})
})
