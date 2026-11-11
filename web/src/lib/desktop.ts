import {ref} from 'vue'

export type DesktopUpdateCheck =
	| {status: 'latest' | 'downloading' | 'ready', version: string}
	| {status: 'unsupported'}
	| {status: 'error', message: string}

// Bridge exposed by the Electron preload script (web-desktop/preload.js).
// Undefined when the app runs in a regular browser.
interface DesktopBridge {
	getServerUrl(): Promise<string>
	setServerUrl(url: string): Promise<string>
	/**
	 * Signs in through the system browser (OAuth with a loopback redirect) and
	 * resolves with an access token. error is 'cancelled' when the user backed
	 * out. Missing in builds before browser sign in.
	 */
	signInWithBrowser?(): Promise<{token?: string, error?: string}>
	cancelBrowserSignIn?(): void
	onUpdateReady(callback: (version: string) => void): void
	installUpdate(): Promise<void>
	/** Manual update check (Settings → About). Missing in older builds. */
	checkForUpdates?(): Promise<DesktopUpdateCheck>
	/**
	 * Recolors the window background to the frame color (#rrggbb). Builds before
	 * color series only understand 'dark' and 'light' and ignore anything else.
	 * Missing in builds before frameless windows.
	 */
	setTheme?(theme: string): void
	// Window controls drawn by the page (WindowControls.vue). Missing in older builds.
	getWindowInfo?(): Promise<{platform: string, maximized: boolean}>
	onMaximizedChange?(callback: (maximized: boolean) => void): void
	minimizeWindow?(): void
	toggleMaximizeWindow?(): void
	closeWindow?(): void
	/**
	 * Files for the offline copy of the account (src/api/offline), kept in the
	 * app's user data folder. Missing in builds before offline mode.
	 */
	offlineStore?: {
		read(name: string): Promise<string | null>
		write(name: string, text: string): Promise<void>
		/** Blocks until written; for saving while the page unloads. */
		writeSync(name: string, text: string): void
		clear(): Promise<void>
	}
}

declare global {
	interface Window {
		ttDesktop?: DesktopBridge
	}
}

export const desktop = window.ttDesktop

// Lets styles reserve room for the window controls drawn over the page.
if (desktop) document.documentElement.classList.add('desktop')

/** Whether the page draws its own minimize/maximize/close buttons (not on macOS). */
export const ownWindowControls = ref(false)
export const windowMaximized = ref(false)

desktop?.getWindowInfo?.().then(info => {
	ownWindowControls.value = info.platform !== 'darwin'
	windowMaximized.value = info.maximized
})
desktop?.onMaximizedChange?.(maximized => {
	windowMaximized.value = maximized
})

/** Version of a downloaded update waiting to be installed, if any. */
export const updateReady = ref<string | null>(null)
export const installingUpdate = ref(false)

desktop?.onUpdateReady(version => {
	updateReady.value = version
})

export async function installUpdate() {
	if (!desktop || installingUpdate.value) return
	installingUpdate.value = true
	try {
		await desktop.installUpdate()
	} finally {
		// Only reached when the install was cancelled (e.g. password prompt dismissed).
		installingUpdate.value = false
	}
}
