import {api} from '@/api/client'

type Size = 'sm' | 'md' | 'lg' | 'xl'

// Attachments need the auth header, so images are fetched once and kept as
// object URLs for the session; v-html re-renders reuse them without a request.
const cache = new Map<string, Promise<string>>()

/** Object URL for an image attachment, optionally as a downscaled preview. */
export function attachmentImage(taskId: number, attachmentId: number, size?: Size): Promise<string> {
	const key = `${taskId}/${attachmentId}/${size ?? ''}`
	let url = cache.get(key)
	if (!url) {
		url = api<Blob>('GET', `/tasks/${taskId}/attachments/${attachmentId}`, {query: size ? {preview_size: size} : undefined, blob: true})
			.then(b => URL.createObjectURL(b))
		url.catch(() => cache.delete(key))
		cache.set(key, url)
	}
	return url
}

/** Ids of attachments embedded in a description as `![…](attachment:<id>)`. */
export function embeddedAttachmentIds(description: string | undefined | null): Set<number> {
	return new Set([...(description ?? '').matchAll(/!\[[^\]]*\]\(attachment:(\d+)\)/g)].map(m => Number(m[1])))
}
