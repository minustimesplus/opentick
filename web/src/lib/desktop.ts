import {ref} from 'vue'

// Bridge exposed by the Electron preload script (web-desktop/preload.js).
// Undefined when the app runs in a regular browser.
interface DesktopBridge {
	getServerUrl(): Promise<string>
	setServerUrl(url: string): Promise<string>
	onUpdateReady(callback: (version: string) => void): void
	installUpdate(): Promise<void>
	/** Recolors the window controls drawn over the page. Missing in builds before frameless windows. */
	setTheme?(theme: 'dark' | 'light'): void
}

declare global {
	interface Window {
		ttDesktop?: DesktopBridge
	}
}

export const desktop = window.ttDesktop

// Lets styles reserve room for the window controls drawn over the page.
if (desktop) document.documentElement.classList.add('desktop')

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
