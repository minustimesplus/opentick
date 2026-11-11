// Thin fetch wrapper around the Vikunja /api/v2 REST API.
// Without an account (and in the demo build) requests go to the in-browser
// backend in ./local instead, which is only downloaded when used. The desktop
// app keeps working without a connection through ./offline.

import {isLocal} from '@/lib/localMode'
import {offlineApi, offlineEnabled} from './offline'

const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api/v2'
const TOKEN_KEY = 'tt.token'

let token: string | null = localStorage.getItem(TOKEN_KEY)
const unauthorizedListeners: (() => void)[] = []

export function getToken() {
	return token
}

export function setToken(value: string | null) {
	token = value
	if (value) {
		localStorage.setItem(TOKEN_KEY, value)
	} else {
		localStorage.removeItem(TOKEN_KEY)
	}
}

export function onUnauthorized(fn: () => void) {
	unauthorizedListeners.push(fn)
}

export class ApiError extends Error {
	status: number
	code?: number
	constructor(status: number, message: string, code?: number) {
		super(message)
		this.status = status
		this.code = code
	}
}

/**
 * The server could not be reached. `delivered` is false when the request
 * certainly never arrived, so it is safe to send again later.
 */
export class NetworkError extends Error {
	delivered: boolean
	constructor(message: string, delivered: boolean) {
		super(message)
		this.delivered = delivered
	}
}

type QueryValue = string | number | boolean | undefined | null | (string | number)[]

export interface RequestOptions {
	query?: Record<string, QueryValue>
	body?: unknown
	contentType?: string
	auth?: boolean
	/** Return the response body as a Blob instead of parsed JSON. */
	blob?: boolean
}

function buildUrl(path: string, query?: Record<string, QueryValue>) {
	const params = new URLSearchParams()
	for (const [key, value] of Object.entries(query ?? {})) {
		if (value === undefined || value === null) continue
		if (Array.isArray(value)) {
			value.forEach(v => params.append(key, String(v)))
		} else {
			params.append(key, String(value))
		}
	}
	const qs = params.toString()
	return BASE + path + (qs ? '?' + qs : '')
}

let refreshing: Promise<boolean> | null = null

async function tryRefresh(): Promise<boolean> {
	if (!refreshing) {
		refreshing = (async () => {
			try {
				const res = await fetch(BASE + '/user/token/refresh', {method: 'POST', credentials: 'include'})
				if (!res.ok) return false
				const data = await res.json() as {access_token?: string, token?: string}
				const next = data.access_token ?? data.token
				// Logged out while the refresh was in flight: stay logged out.
				if (!next || !token) return false
				setToken(next)
				return true
			} catch {
				return false
			} finally {
				setTimeout(() => { refreshing = null }, 0)
			}
		})()
	}
	return refreshing
}

/**
 * Turns failures to reach the server into NetworkErrors. The desktop app's
 * proxy answers them with an X-OpenTick-Offline header (web-desktop/main.js).
 */
async function send(method: string, path: string, opts: RequestOptions): Promise<Response> {
	let res: Response
	try {
		res = await sendRaw(method, path, opts)
	} catch (e) {
		// fetch() only rejects when no response arrived at all.
		throw new NetworkError(String((e as Error).message), method !== 'GET')
	}
	const offline = res.headers.get('X-OpenTick-Offline')
	if (offline) {
		let message = res.statusText
		try {
			message = ((await res.json()) as {message?: string}).message || message
		} catch {
			// non-JSON body
		}
		throw new NetworkError(message, offline !== 'unreachable')
	}
	// A reverse proxy (or CDN) in front of a server that is down. The API's
	// own 502s (a calendar feed or import source failing) carry a JSON error.
	if ((res.status === 502 || res.status === 504) && !await isApiError(res)) {
		throw new NetworkError(`${res.status} ${res.statusText}`.trim(), method !== 'GET')
	}
	return res
}

async function isApiError(res: Response): Promise<boolean> {
	try {
		const body = await res.clone().json() as Record<string, unknown> | null
		return !!body && typeof body === 'object' && ('code' in body || 'detail' in body || 'message' in body)
	} catch {
		return false
	}
}

async function sendRaw(method: string, path: string, opts: RequestOptions): Promise<Response> {
	const headers: Record<string, string> = {Accept: opts.blob ? '*/*' : 'application/json'}
	if (opts.auth !== false && token) headers.Authorization = `Bearer ${token}`
	let body: BodyInit | undefined
	if (opts.body instanceof FormData) {
		// The browser sets the multipart boundary itself.
		body = opts.body
	} else if (opts.body !== undefined) {
		headers['Content-Type'] = opts.contentType ?? 'application/json'
		body = JSON.stringify(opts.body)
	}
	return fetch(buildUrl(path, opts.query), {method, headers, body, credentials: 'include'})
}

function tokenExpiresSoon(): boolean {
	if (!token) return false
	try {
		const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/'))) as {exp?: number}
		return !!payload.exp && payload.exp * 1000 < Date.now() + 60_000
	} catch {
		return false
	}
}

export async function api<T = unknown>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
	if (isLocal()) {
		const {localApi} = await import('./local/backend')
		return localApi<T>(method, path, opts)
	}
	if (offlineEnabled()) return offlineApi<T>(method, path, opts)
	return serverApi<T>(method, path, opts)
}

/** Sends a request to the server, whatever the offline mode thinks of the connection. */
export async function serverApi<T = unknown>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
	// Access tokens are short-lived (10 min); renew them before they lapse.
	if (opts.auth !== false && tokenExpiresSoon()) await tryRefresh()
	let res = await send(method, path, opts)
	if (res.status === 401 && opts.auth !== false && token) {
		if (await tryRefresh()) {
			res = await send(method, path, opts)
		}
		if (res.status === 401) {
			setToken(null)
			unauthorizedListeners.forEach(fn => fn())
		}
	}
	if (!res.ok) {
		let message = res.statusText
		let code: number | undefined
		try {
			const problem = await res.json() as {detail?: string, title?: string, message?: string, code?: number}
			message = problem.detail || problem.message || problem.title || message
			code = problem.code
		} catch {
			// non-JSON error body
		}
		throw new ApiError(res.status, message, code)
	}
	if (res.status === 204 || res.status === 304) return undefined as T
	if (opts.blob) return await res.blob() as T
	const text = await res.text()
	return (text ? JSON.parse(text) : undefined) as T
}

export interface Paginated<T> {
	items?: T[] | null
	page?: number
	per_page?: number
	total?: number
	total_pages?: number
}

/** Sweeps every page of a paginated list endpoint. */
export async function fetchAll<T>(path: string, query: Record<string, QueryValue> = {}, maxPages = 50): Promise<T[]> {
	const out: T[] = []
	let page = 1
	for (;;) {
		const res = await api<Paginated<T>>('GET', path, {query: {...query, page}})
		out.push(...(res?.items ?? []))
		const totalPages = res?.total_pages ?? 1
		if (page >= totalPages || page >= maxPages || !(res?.items?.length)) break
		page++
	}
	return out
}
