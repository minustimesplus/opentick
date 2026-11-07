import {desktop, updateReady} from '@/lib/desktop'

export type UpdateCheck =
	/** Running the newest release. */
	| {status: 'latest', version: string}
	/** Desktop: the update is downloading and will offer a restart once done. */
	| {status: 'downloading', version: string}
	/** Desktop: the update is downloaded and waiting to be installed. */
	| {status: 'ready', version: string}
	/** Browser: a newer release exists; the server has to be updated. */
	| {status: 'available', version: string, url: string}
	/** The running version is unknown (dev or self-built), so only the latest release can be shown. */
	| {status: 'unknown', version: string, url: string}
	| {status: 'error', message: string}

const LATEST_RELEASE_URL = 'https://api.github.com/repos/minustimesplus/opentick/releases/latest'

/** Leading "x.y.z" of a version or release tag, e.g. "0.1.34-2-gabc1234" → [0, 1, 34]. */
function parseVersion(version: string): number[] | null {
	const match = /^(?:desktop-v)?(\d+)\.(\d+)\.(\d+)/.exec(version)
	return match ? match.slice(1).map(Number) : null
}

function isNewer(a: number[], b: number[]): boolean {
	for (let i = 0; i < 3; i++) {
		if (a[i] !== b[i]) return a[i] > b[i]
	}
	return false
}

async function checkLatestRelease(current: string): Promise<UpdateCheck> {
	const res = await fetch(LATEST_RELEASE_URL, {headers: {Accept: 'application/vnd.github+json'}})
	if (!res.ok) throw new Error(`GitHub responded with ${res.status}`)
	const release = await res.json() as {tag_name: string, html_url: string}
	const latest = parseVersion(release.tag_name)
	if (!latest) throw new Error(`Unexpected release tag ${release.tag_name}`)
	const version = latest.join('.')
	const running = parseVersion(current)
	if (!running) return {status: 'unknown', version, url: release.html_url}
	return isNewer(latest, running)
		? {status: 'available', version, url: release.html_url}
		: {status: 'latest', version: current}
}

/**
 * Checks for a newer release. The desktop app asks electron-updater, which
 * also downloads the update; browsers compare against the latest GitHub release.
 */
export async function checkForUpdates(): Promise<UpdateCheck> {
	try {
		const result = await desktop?.checkForUpdates?.()
		if (result && result.status !== 'unsupported') {
			if (result.status === 'ready') updateReady.value = result.version
			return result
		}
		return await checkLatestRelease(__APP_VERSION__)
	} catch (err) {
		return {status: 'error', message: err instanceof Error ? err.message : String(err)}
	}
}
