// Clerk sign in for hosted instances. The SDK only loads when the server
// enables Clerk, so self-hosted builds never download it.
import type {Clerk} from '@clerk/clerk-js'
import {api} from '@/api/client'

export interface AuthInfo {
	local: {enabled: boolean, registration_enabled: boolean}
	clerk?: {enabled: boolean, publishable_key?: string, social_providers: string[]}
}

let info: Promise<AuthInfo | null> | null = null

/** The server's enabled auth methods, or null when it can't be reached. */
export function getAuthInfo(force = false): Promise<AuthInfo | null> {
	if (force || !info) {
		info = api<{auth: AuthInfo}>('GET', '/info', {auth: false})
			.then(res => res.auth)
			.catch(() => {
				info = null
				return null
			})
	}
	return info
}

let instance: Promise<Clerk> | null = null
// The desktop app can switch servers, and with them Clerk instances.
let instanceKey = ''

export function loadClerk(publishableKey: string): Promise<Clerk> {
	if (instance && instanceKey !== publishableKey) instance = null
	instanceKey = publishableKey
	instance ??= (async () => {
		const {Clerk} = await import('@clerk/clerk-js')
		const clerk = new Clerk(publishableKey)
		await clerk.load()
		return clerk
	})().catch(e => {
		instance = null
		throw e
	})
	return instance
}

/** Ends the Clerk session too, so the next visitor can't sign straight back in. */
export async function signOutOfClerk() {
	if (!instance) {
		const auth = await getAuthInfo()
		if (!auth?.clerk?.enabled || !auth.clerk.publishable_key) return
		loadClerk(auth.clerk.publishable_key)
	}
	try {
		await (await instance!).signOut()
	} catch {
		// Clerk unreachable: the app session is gone either way.
	}
}

/** Where OAuth providers send the browser back to. */
export const SSO_CALLBACK_PATH = '/sso-callback'
