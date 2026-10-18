// Thin fetch wrapper around the Vikunja /api/v2 REST API.

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
				if (!next) return false
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

async function send(method: string, path: string, opts: RequestOptions): Promise<Response> {
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
