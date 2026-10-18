import {reactive} from 'vue'
import {api, ApiError} from '@/api/client'

// The user's app-wide background image, synced through
// /user/settings/background and shown behind the task list and matrix.
export const background = reactive({
	/** Object URL of the loaded image, null when none is set. */
	url: null as string | null,
	/** Whether the server accepts uploads (enabled_background_providers). */
	uploadEnabled: false,
	busy: false,
})

function show(blob: Blob | null) {
	if (background.url) URL.revokeObjectURL(background.url)
	background.url = blob ? URL.createObjectURL(blob) : null
}

export async function loadBackground() {
	try {
		const info = await api<{enabled_background_providers?: string[] | null}>('GET', '/info', {auth: false})
		background.uploadEnabled = info?.enabled_background_providers?.includes('upload') ?? false
		if (!info?.enabled_background_providers) return
		show(await api<Blob>('GET', '/user/settings/background', {blob: true}))
	} catch (e) {
		// 404: no background set.
		if (e instanceof ApiError && e.status === 404) show(null)
	}
}

export function clearBackground() {
	show(null)
}

export async function uploadBackground(file: File) {
	const form = new FormData()
	form.append('background', file)
	background.busy = true
	try {
		await api('PUT', '/user/settings/background', {body: form})
		// Show what the server stored (resized JPEG), not the raw upload.
		show(await api<Blob>('GET', '/user/settings/background', {blob: true}))
	} finally {
		background.busy = false
	}
}

export async function removeBackground() {
	background.busy = true
	try {
		await api('DELETE', '/user/settings/background')
		show(null)
	} finally {
		background.busy = false
	}
}
