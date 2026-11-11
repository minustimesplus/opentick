import {describe, expect, it} from 'vitest'
import {applyMergePatch, mergePatchDiff, reconcile} from './viewSync'

describe('merge patches', () => {
	it('round-trips nested changes, including removed keys', () => {
		const from = {1: {group: 'list', sort: 'date'}, 2: {group: 'none', sort: 'title'}}
		const to = {1: {group: 'list', sort: 'custom'}}
		const patch = mergePatchDiff(from, to)
		expect(patch).toEqual({1: {sort: 'custom'}, 2: null})
		expect(applyMergePatch(from, patch)).toEqual(to)
	})

	it('replaces arrays as a whole', () => {
		expect(mergePatchDiff({a: [1, 2]}, {a: [2]})).toEqual({a: [2]})
		expect(applyMergePatch({a: [1, 2]}, {a: [2]})).toEqual({a: [2]})
	})
})

describe('reconcile', () => {
	it('seeds an empty server with every local value (first device wins)', () => {
		const r = reconcile({matrixLists: [4], matrixShowDone: true}, null, {}, 1000)
		expect(r.patch).toEqual({matrixLists: [4], matrixShowDone: true, rev: 1000})
		expect(r.values).toEqual({matrixLists: [4], matrixShowDone: true})
	})

	it('takes the server values on a device that has not synced before', () => {
		const r = reconcile({matrixLists: [], matrixShowDone: false}, null, {matrixLists: [7], matrixShowDone: true, rev: 5})
		expect(r.patch).toBeNull()
		expect(r.values).toEqual({matrixLists: [7], matrixShowDone: true})
	})

	it('pushes only local edits and keeps edits made elsewhere', () => {
		const snapshot = {matrixView: {1: {group: 'list', sort: 'date'}}, matrixShowDone: false, rev: 5}
		const local = {matrixView: {1: {group: 'list', sort: 'title'}}, matrixShowDone: false}
		const remote = {matrixView: {1: {group: 'list', sort: 'date'}, 2: {group: 'none', sort: 'date'}}, matrixShowDone: true, rev: 6}
		const r = reconcile(local, snapshot, remote, 3)
		expect(r.patch).toEqual({matrixView: {1: {sort: 'title'}}, rev: 7})
		expect(r.values).toEqual({
			matrixView: {1: {group: 'list', sort: 'title'}, 2: {group: 'none', sort: 'date'}},
			matrixShowDone: true,
		})
	})

	it('ignores a server response older than the last push', () => {
		const snapshot = {matrixShowDone: true, rev: 10}
		const r = reconcile({matrixShowDone: true}, snapshot, {matrixShowDone: false, rev: 9})
		expect(r.patch).toBeNull()
		expect(r.values).toEqual({matrixShowDone: true})
	})

	it('sends a removed quadrant setup as null', () => {
		const snapshot = {matrixView: {1: {group: 'list', sort: 'date'}}, rev: 1}
		const r = reconcile({matrixView: {}}, snapshot, snapshot, 50)
		expect(r.patch).toEqual({matrixView: {1: null}, rev: 50})
		expect(r.server.matrixView).toEqual({})
	})
})
