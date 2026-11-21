import {describe, expect, it} from 'vitest'
import {redirectRegistered} from './oauthRedirect'

describe('redirectRegistered', () => {
	const registered = [
		'https://claude.ai/api/mcp/auth_callback',
		'http://localhost:33418/callback',
	]

	it('matches registered URIs exactly', () => {
		expect(redirectRegistered(registered, 'https://claude.ai/api/mcp/auth_callback')).toBe(true)
		expect(redirectRegistered(registered, 'https://claude.ai:8443/api/mcp/auth_callback')).toBe(false)
	})

	it('ignores the port of loopback URIs', () => {
		expect(redirectRegistered(registered, 'http://localhost:51234/callback')).toBe(true)
		expect(redirectRegistered(registered, 'http://localhost:51234/other')).toBe(false)
		expect(redirectRegistered(registered, 'http://127.0.0.1:51234/callback')).toBe(false)
		expect(redirectRegistered(registered, 'not a url')).toBe(false)
	})
})
