/** An http loopback redirect URI (RFC 8252), as used by native apps and CLI clients. */
export function isLoopbackRedirect(uri: string): boolean {
	try {
		const url = new URL(uri)
		return url.protocol === 'http:' && ['127.0.0.1', '[::1]', 'localhost'].includes(url.hostname)
	} catch {
		return false
	}
}

function withoutPort(uri: string): string {
	const url = new URL(uri)
	url.port = ''
	return url.toString()
}

/**
 * Whether a client may be sent back to uri. Mirrors OAuthClient.AllowsRedirectURI on the server:
 * loopback URIs match whatever their port (RFC 8252 §7.3), everything else must match exactly.
 */
export function redirectRegistered(registered: string[], uri: string): boolean {
	if (registered.includes(uri)) return true
	if (!isLoopbackRedirect(uri)) return false
	const wanted = withoutPort(uri)
	return registered.some(r => isLoopbackRedirect(r) && withoutPort(r) === wanted)
}
