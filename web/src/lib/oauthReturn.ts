// An OAuth client (e.g. Claude) sends the user to /oauth/authorize. Signing in
// with Google or Apple leaves the app and comes back on /sso-callback, so the
// consent page is remembered here and restored after sign in.

export const OAUTH_AUTHORIZE_PATH = '/oauth/authorize'
const KEY = 'tt.oauthReturn'
/** First-party client id the desktop app signs in with through the browser. */
export const DESKTOP_CLIENT_ID = 'opentick-desktop'

if (window.location.pathname === OAUTH_AUTHORIZE_PATH) {
	try {
		sessionStorage.setItem(KEY, window.location.pathname + window.location.search)
	} catch {
		// Storage blocked: email sign in still works since it stays on the page.
	}
}

/** Returns the remembered consent page, if any, and forgets it. */
export function takeOAuthReturn(): string | null {
	try {
		const value = sessionStorage.getItem(KEY)
		sessionStorage.removeItem(KEY)
		return value?.startsWith(OAUTH_AUTHORIZE_PATH + '?') ? value : null
	} catch {
		return null
	}
}
