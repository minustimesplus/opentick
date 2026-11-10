// Electron shell for the TickTick-style frontend in web/.
//
// The bundled frontend talks to a relative /api/v2, so it is served from a
// small loopback HTTP server that also proxies /api/* to the Vikunja server
// the user picks on the login screen. Keeping everything same-origin means
// the refresh-token cookie and fetch() work exactly as in the browser.

const {app, BrowserWindow, ipcMain, shell, Menu, session} = require('electron')
const {autoUpdater} = require('electron-updater')
const crypto = require('crypto')
const fs = require('fs')
const http = require('http')
const https = require('https')
const path = require('path')

const APP_DIR = path.join(__dirname, 'app')
// Fixed so the page origin (and with it localStorage) stays stable between launches.
const PREFERRED_PORT = 45780
const UPDATE_CHECK_INTERVAL = 4 * 60 * 60 * 1000
// Idle time (no bytes either way) before a proxied API request is given up.
const UPSTREAM_TIMEOUT = 15 * 1000
const REPO = {owner: 'minustimesplus', repo: 'opentick'}
// First-party OAuth client id for signing in through the system browser.
const OAUTH_CLIENT_ID = 'opentick-desktop'
// How long the browser sign in may take before it is given up.
const BROWSER_SIGN_IN_TIMEOUT = 10 * 60 * 1000
// Matches the server's default service.jwtttl. The first refresh replaces the
// cookie with one carrying the server's real lifetime.
const REFRESH_COOKIE_LIFETIME = 72 * 60 * 60
const REFRESH_COOKIE_PATHS = ['/api/v1/user/token/refresh', '/api/v2/user/token/refresh']

const MIME_TYPES = {
	'.html': 'text/html; charset=utf-8',
	'.js': 'text/javascript; charset=utf-8',
	'.css': 'text/css; charset=utf-8',
	'.json': 'application/json',
	'.svg': 'image/svg+xml',
	'.png': 'image/png',
	'.ico': 'image/x-icon',
	'.woff': 'font/woff',
	'.woff2': 'font/woff2',
	'.webmanifest': 'application/manifest+json',
}

// The window has no title bar: the page draws a 24px strip in the frame
// color with its own minimize/maximize/close buttons (web/src/App.vue,
// WindowControls.vue). macOS keeps its native traffic lights instead.
const IS_MAC = process.platform === 'darwin'
// Matches --frame in web/src/styles.css, so resizing never flashes another color.
const FRAME_COLORS = {
	dark: '#242424',
	light: '#f3f3f3',
}

let mainWindow = null
let origin = null
let downloadedVersion = null

// The app used to be called "Vikunja TickTick". Carry its user data (server
// URL, login session, update token) over on the first launch after the rename.
// Has to run before the single-instance lock creates the new directory.
function migrateLegacyUserData() {
	const current = app.getPath('userData')
	const legacy = path.join(app.getPath('appData'), 'Vikunja TickTick')
	if (fs.existsSync(current) || !fs.existsSync(legacy)) return
	try {
		fs.cpSync(legacy, current, {
			recursive: true,
			filter: src => !path.basename(src).startsWith('Singleton'),
		})
	} catch (err) {
		console.warn('Failed to migrate user data:', err.message)
	}
}

migrateLegacyUserData()

if (!app.requestSingleInstanceLock()) {
	app.quit()
	process.exit(0)
}

// ─── Config ──────────────────────────────────────────────────────────
const configPath = () => path.join(app.getPath('userData'), 'config.json')

function loadConfig() {
	try {
		return JSON.parse(fs.readFileSync(configPath(), 'utf8'))
	} catch {
		return {}
	}
}

function saveConfig(config) {
	fs.mkdirSync(path.dirname(configPath()), {recursive: true})
	fs.writeFileSync(configPath(), JSON.stringify(config, null, 2))
}

let config = {}

function normalizeServerUrl(input) {
	let value = String(input || '').trim()
	if (!/^https?:\/\//i.test(value)) value = 'https://' + value
	const url = new URL(value)
	// Accept both "https://host" and "https://host/api/v2" style input.
	url.pathname = url.pathname.replace(/\/+$/, '').replace(/\/api(\/v\d+)?$/, '')
	url.search = ''
	url.hash = ''
	return url.toString().replace(/\/$/, '')
}

// ─── Local server + API proxy ────────────────────────────────────────
function serveStatic(req, res) {
	const urlPath = decodeURIComponent(new URL(req.url, 'http://x').pathname)
	let filePath = path.normalize(path.join(APP_DIR, urlPath))
	if (!filePath.startsWith(APP_DIR + path.sep) || !fs.existsSync(filePath) || fs.statSync(filePath).isDirectory()) {
		// SPA fallback: the router handles every non-asset path.
		filePath = path.join(APP_DIR, 'index.html')
	}
	res.writeHead(200, {'Content-Type': MIME_TYPES[path.extname(filePath)] || 'application/octet-stream'})
	fs.createReadStream(filePath).pipe(res)
}

function proxyApi(req, res) {
	if (!config.serverUrl) {
		res.writeHead(502, {'Content-Type': 'application/json'})
		res.end(JSON.stringify({message: 'No server configured'}))
		return
	}

	const target = new URL(config.serverUrl + req.url)
	const client = target.protocol === 'https:' ? https : http
	const headers = {...req.headers, host: target.host}
	delete headers.origin
	delete headers.referer

	const upstream = client.request(target, {method: req.method, headers}, upstreamRes => {
		const resHeaders = {...upstreamRes.headers}
		// The page lives on plain-http loopback: drop Secure/Domain so the
		// refresh-token cookie is stored for it.
		if (resHeaders['set-cookie']) {
			resHeaders['set-cookie'] = resHeaders['set-cookie'].map(cookie => cookie
				.replace(/;\s*Secure/gi, '')
				.replace(/;\s*Domain=[^;]*/gi, ''))
		}
		res.writeHead(upstreamRes.statusCode || 502, resHeaders)
		upstreamRes.pipe(res)
	})
	// Fail requests to an unreachable server instead of leaving the page
	// waiting forever: the login screen needs /info to let the user switch servers.
	upstream.setTimeout(UPSTREAM_TIMEOUT, () => upstream.destroy(new Error('timed out')))
	upstream.on('error', err => {
		if (!res.headersSent) {
			res.writeHead(502, {'Content-Type': 'application/json'})
		}
		res.end(JSON.stringify({message: `Could not reach ${config.serverUrl}: ${err.message}`}))
	})
	req.pipe(upstream)
}

function startServer() {
	const server = http.createServer((req, res) => {
		if (req.url.startsWith('/api/')) {
			proxyApi(req, res)
		} else {
			serveStatic(req, res)
		}
	})

	return new Promise(resolve => {
		server.once('error', () => {
			// Preferred port taken (e.g. a second user session): fall back to a random one.
			server.listen(0, '127.0.0.1', () => resolve(server.address().port))
		})
		server.listen(PREFERRED_PORT, '127.0.0.1', () => resolve(server.address().port))
	})
}

// ─── Browser sign in ─────────────────────────────────────────────────
// Hosted servers sign in through Clerk, which only runs on the server's own
// domain. So the system browser signs in there and approves this app through
// the OAuth authorization code flow (PKCE, loopback redirect per RFC 8252).

let browserSignIn = null

const base64url = buf => buf.toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')

function callbackPage(title, message) {
	const escape = s => s.replace(/[&<>"]/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'})[c])
	return `<!doctype html><html><head><meta charset="utf-8"><title>OpenTick</title>
<style>body{margin:0;display:flex;align-items:center;justify-content:center;height:100vh;background:#f8f8f8;color:#191919;font:15px system-ui,sans-serif}
main{max-width:380px;padding:40px;border-radius:14px;background:#fff;box-shadow:0 4px 24px rgba(0,0,0,.04);text-align:center}
h1{margin:0 0 10px;font-size:22px}p{margin:0;color:#757575;line-height:1.5}</style></head>
<body><main><h1>${escape(title)}</h1><p>${escape(message)}</p></main></body></html>`
}

/** Stores the refresh token where the proxy would have put the server's cookie. */
async function storeRefreshToken(refreshToken) {
	const expirationDate = Math.floor(Date.now() / 1000) + REFRESH_COOKIE_LIFETIME
	for (const cookiePath of REFRESH_COOKIE_PATHS) {
		await session.defaultSession.cookies.set({
			url: origin + cookiePath,
			name: 'vikunja_refresh_token',
			value: refreshToken,
			path: cookiePath,
			httpOnly: true,
			sameSite: 'lax',
			expirationDate,
		})
	}
}

async function exchangeCode(serverUrl, code, redirectUri, verifier) {
	const res = await fetch(serverUrl + '/api/v2/oauth/token', {
		method: 'POST',
		headers: {'Content-Type': 'application/json', Accept: 'application/json'},
		body: JSON.stringify({
			grant_type: 'authorization_code',
			code,
			client_id: OAUTH_CLIENT_ID,
			redirect_uri: redirectUri,
			code_verifier: verifier,
		}),
		signal: AbortSignal.timeout(UPSTREAM_TIMEOUT),
	})
	const body = await res.json().catch(() => ({}))
	if (!res.ok || !body.access_token) throw new Error(body.message || body.error_description || `Sign in failed (${res.status})`)
	if (body.refresh_token) await storeRefreshToken(body.refresh_token)
	return body.access_token
}

function cancelBrowserSignIn() {
	browserSignIn?.finish(new Error('cancelled'))
}

/** Resolves with an access token once the user approved this app in the browser. */
function startBrowserSignIn() {
	cancelBrowserSignIn()
	if (!config.serverUrl) return Promise.reject(new Error('No server configured'))

	const verifier = base64url(crypto.randomBytes(32))
	const challenge = base64url(crypto.createHash('sha256').update(verifier).digest())
	const state = base64url(crypto.randomBytes(16))
	const serverUrl = config.serverUrl

	return new Promise((resolve, reject) => {
		let redirectUri = ''
		let timer = null
		const server = http.createServer(async (req, res) => {
			const url = new URL(req.url, 'http://127.0.0.1')
			if (url.pathname !== '/callback' || url.searchParams.get('state') !== state) {
				res.writeHead(404)
				res.end()
				return
			}
			const send = (status, title, message) => {
				res.writeHead(status, {'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store'})
				res.end(callbackPage(title, message))
			}
			const code = url.searchParams.get('code')
			if (!code) {
				send(200, 'Sign in cancelled', 'You can close this tab.')
				flow.finish(new Error('cancelled'))
				return
			}
			try {
				const token = await exchangeCode(serverUrl, code, redirectUri, verifier)
				send(200, 'Signed in', 'You can close this tab and return to OpenTick.')
				flow.finish(null, token)
			} catch (err) {
				send(400, 'Sign in failed', err.message)
				flow.finish(err)
			}
		})

		const flow = {
			finish(err, token) {
				if (browserSignIn !== flow) return
				browserSignIn = null
				clearTimeout(timer)
				// Let the response to the browser go out before closing.
				setTimeout(() => server.close(), 1000)
				if (err) {
					reject(err)
					return
				}
				if (mainWindow) {
					if (mainWindow.isMinimized()) mainWindow.restore()
					mainWindow.show()
					mainWindow.focus()
				}
				resolve(token)
			},
		}
		browserSignIn = flow
		timer = setTimeout(() => flow.finish(new Error('timed out')), BROWSER_SIGN_IN_TIMEOUT)

		server.on('error', err => flow.finish(err))
		server.listen(0, '127.0.0.1', () => {
			redirectUri = `http://127.0.0.1:${server.address().port}/callback`
			const authorize = new URL(serverUrl + '/oauth/authorize')
			authorize.search = new URLSearchParams({
				response_type: 'code',
				client_id: OAUTH_CLIENT_ID,
				redirect_uri: redirectUri,
				state,
				code_challenge: challenge,
				code_challenge_method: 'S256',
			}).toString()
			shell.openExternal(authorize.toString()).catch(err => flow.finish(err))
		})
	})
}

// ─── Window ──────────────────────────────────────────────────────────
function createMainWindow() {
	mainWindow = new BrowserWindow({
		width: 1400,
		height: 900,
		icon: path.join(__dirname, 'icon.png'),
		autoHideMenuBar: true,
		// Start with the frame color the page used last time, so the first
		// frame already matches the theme.
		backgroundColor: config.frameColor || FRAME_COLORS.dark,
		// Mapping a frameless window before it has painted shows the desktop
		// through it on Linux; stay hidden until the first frame is ready.
		show: false,
		...(IS_MAC ? {titleBarStyle: 'hidden'} : {frame: false}),
		webPreferences: {
			preload: path.join(__dirname, 'preload.js'),
			contextIsolation: true,
			nodeIntegration: false,
			sandbox: true,
		},
	})

	const showWindow = () => {
		if (mainWindow && !mainWindow.isVisible()) mainWindow.show()
	}
	mainWindow.once('ready-to-show', showWindow)
	// Don't leave the app invisible if the page never paints (e.g. load error).
	setTimeout(showWindow, 3000)

	mainWindow.webContents.setWindowOpenHandler(({url}) => {
		if (/^(https?|mailto):/i.test(url)) shell.openExternal(url)
		return {action: 'deny'}
	})
	mainWindow.webContents.on('will-navigate', (event, url) => {
		if (!url.startsWith(origin + '/')) {
			event.preventDefault()
			if (/^https?:/i.test(url)) shell.openExternal(url)
		}
	})
	mainWindow.webContents.on('did-finish-load', () => {
		if (downloadedVersion) mainWindow.webContents.send('update:ready', downloadedVersion)
	})
	const sendMaximized = () => mainWindow.webContents.send('window:maximized', mainWindow.isMaximized())
	mainWindow.on('maximize', sendMaximized)
	mainWindow.on('unmaximize', sendMaximized)
	mainWindow.on('closed', () => {
		mainWindow = null
	})

	mainWindow.loadURL(origin + '/')
}

app.on('second-instance', () => {
	if (!mainWindow) return
	if (mainWindow.isMinimized()) mainWindow.restore()
	mainWindow.show()
	mainWindow.focus()
})

// ─── Updates ─────────────────────────────────────────────────────────
// Releases are published by .github/workflows/desktop-release.yml. AppImages
// replace themselves in place; .deb installs go through pkexec, which shows
// the system password prompt.
function setupUpdater() {
	if (!app.isPackaged) return

	// Reading releases of a private repository needs a token with read access
	// to its contents, stored in <userData>/update-token.
	let token = process.env.GH_TOKEN
	try {
		token ||= fs.readFileSync(path.join(app.getPath('userData'), 'update-token'), 'utf8').trim()
	} catch {
		// Public repository, no token needed
	}
	autoUpdater.setFeedURL({provider: 'github', ...REPO, ...(token ? {private: true, token} : {})})
	autoUpdater.autoDownload = true
	// Installing a .deb needs the password prompt, so only install when asked.
	autoUpdater.autoInstallOnAppQuit = false

	autoUpdater.on('update-downloaded', info => {
		downloadedVersion = info.version
		mainWindow?.webContents.send('update:ready', info.version)
	})
	autoUpdater.on('error', err => {
		console.warn('Update check failed:', err.message)
	})

	const check = () => autoUpdater.checkForUpdates().catch(() => {})
	check()
	setInterval(check, UPDATE_CHECK_INTERVAL)
}

// ─── IPC ─────────────────────────────────────────────────────────────
ipcMain.handle('server:get', () => config.serverUrl || '')

ipcMain.handle('server:set', (_event, input) => {
	const serverUrl = normalizeServerUrl(input)
	if (serverUrl !== config.serverUrl) {
		cancelBrowserSignIn()
		config = {...config, serverUrl}
		saveConfig(config)
	}
	return serverUrl
})

// Errors cross IPC as plain messages; 'cancelled' means the user backed out.
ipcMain.handle('auth:browser-sign-in', async () => {
	try {
		return {token: await startBrowserSignIn()}
	} catch (err) {
		return {error: err.message}
	}
})

ipcMain.on('auth:cancel-browser-sign-in', cancelBrowserSignIn)

ipcMain.on('theme:set', (_event, theme) => {
	// Color series send their frame color; older pages send 'dark' / 'light'.
	const color = /^#[0-9a-f]{6}$/i.test(theme) ? theme : FRAME_COLORS[theme]
	if (!color || !mainWindow) return
	mainWindow.setBackgroundColor(color)
	if (color !== config.frameColor) {
		config = {...config, frameColor: color}
		saveConfig(config)
	}
})

ipcMain.handle('window:info', () => ({
	platform: process.platform,
	maximized: mainWindow?.isMaximized() ?? false,
}))

ipcMain.on('window:minimize', () => mainWindow?.minimize())
ipcMain.on('window:toggle-maximize', () => {
	if (!mainWindow) return
	if (mainWindow.isMaximized()) mainWindow.unmaximize()
	else mainWindow.maximize()
})
ipcMain.on('window:close', () => mainWindow?.close())

// Manual check from Settings → About. An available update downloads in the
// background (autoDownload) and announces itself via 'update:ready'.
ipcMain.handle('update:check', async () => {
	if (!app.isPackaged) return {status: 'unsupported'}
	if (downloadedVersion) return {status: 'ready', version: downloadedVersion}
	try {
		const result = await autoUpdater.checkForUpdates()
		if (!result) return {status: 'unsupported'}
		return result.isUpdateAvailable
			? {status: 'downloading', version: result.updateInfo.version}
			: {status: 'latest', version: app.getVersion()}
	} catch (err) {
		return {status: 'error', message: err.message}
	}
})

ipcMain.handle('update:install', () => {
	if (!downloadedVersion) return
	// isSilent=false, isForceRunAfter=true: restart into the new version.
	autoUpdater.quitAndInstall(false, true)
})

// ─── Lifecycle ───────────────────────────────────────────────────────
app.whenReady().then(async () => {
	config = loadConfig()
	Menu.setApplicationMenu(null)
	const port = await startServer()
	origin = `http://127.0.0.1:${port}`
	createMainWindow()
	setupUpdater()
})

app.on('window-all-closed', () => app.quit())
