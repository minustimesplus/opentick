// "Use without an account": the app runs against an in-browser backend
// (src/api/local) that keeps everything in this browser's localStorage.
// Demo builds (VITE_DEMO=1, the GitHub Pages demo) always run this way.

const LOCAL_KEY = 'tt.local'

/** True in the static demo build, which has no server at all. */
export const DEMO = import.meta.env.VITE_DEMO === '1'

/** Sign up page of the hosted instance; empty hides the option. */
export const HOSTED_URL = (import.meta.env.VITE_HOSTED_URL as string | undefined) ?? ''

export const REPO_URL = 'https://github.com/minustimesplus/opentick'
export const SELF_HOST_DOCS_URL = `${REPO_URL}#installing`

/** Session token the local backend hands out; never sent to a server. */
export const LOCAL_TOKEN = 'local'

function read(): boolean {
	try {
		return localStorage.getItem(LOCAL_KEY) === '1'
	} catch {
		return false
	}
}

let local = DEMO || read()

export function isLocal(): boolean {
	return local
}

export function setLocal(value: boolean) {
	local = DEMO || value
	try {
		if (value) localStorage.setItem(LOCAL_KEY, '1')
		else localStorage.removeItem(LOCAL_KEY)
	} catch {
		// Storage blocked: local mode lasts for this page load.
	}
}
