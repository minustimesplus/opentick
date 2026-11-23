import {api} from '@/api/client'

// One request per contact and photo version for the whole session; null when
// the image failed. Bounded, as the list scrolls through hundreds of photos.
const MAX_CACHED = 400
const cache = new Map<string, Promise<string | null>>()

/** Object URL of a contact's photo, shared by avatars and the photo viewer. */
export function loadContactPhoto(id: number, version: number): Promise<string | null> {
	const key = `${id}:${version}`
	let p = cache.get(key)
	if (!p) {
		p = api<Blob>('GET', `/contacts/${id}/photo`, {blob: true})
			.then(b => URL.createObjectURL(b))
			.catch(() => null)
		cache.set(key, p)
		if (cache.size > MAX_CACHED) {
			const [oldest, url] = cache.entries().next().value!
			cache.delete(oldest)
			url.then(u => u && URL.revokeObjectURL(u))
		}
	}
	return p
}
