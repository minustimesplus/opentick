// Someone opened an invitation link (/invite/<token>). The token is kept
// until the user is signed in: the login screen, and signing in with Google
// or Apple (which leaves the app), would lose it otherwise.

const KEY = 'tt.invite'
const PATTERN = /\/invite\/([A-Za-z0-9]+)\/?$/

export const INVITE_PATH = '/invite/:token'

export function rememberInviteToken(token: string) {
	try {
		localStorage.setItem(KEY, token)
	} catch {
		// Storage blocked: the invitation still works while the page stays open.
		memory = token
	}
}

let memory: string | null = null

const opened = window.location.pathname.match(PATTERN)
if (opened) rememberInviteToken(opened[1])

export function pendingInviteToken(): string | null {
	try {
		return localStorage.getItem(KEY) ?? memory
	} catch {
		return memory
	}
}

export function forgetInviteToken() {
	memory = null
	try {
		localStorage.removeItem(KEY)
	} catch {
		// nothing stored
	}
}
