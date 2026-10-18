// Electron shell for the TickTick-style frontend in web/.
//
// The bundled frontend talks to a relative /api/v2, so it is served from a
// small loopback HTTP server that also proxies /api/* to the Vikunja server
// the user picks on the login screen. Keeping everything same-origin means
// the refresh-token cookie and fetch() work exactly as in the browser.

const {app, BrowserWindow, ipcMain, shell, Menu} = require('electron')
const {autoUpdater} = require('electron-updater')
const fs = require('fs')
const http = require('http')
const https = require('https')
const path = require('path')

const APP_DIR = path.join(__dirname, 'app')
// Fixed so the page origin (and with it localStorage) stays stable between launches.
const PREFERRED_PORT = 45780
const UPDATE_CHECK_INTERVAL = 4 * 60 * 60 * 1000
const REPO = {owner: 'minustimesplus', repo: 'vikunja-ticktick'}

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

// ─── Window ──────────────────────────────────────────────────────────
function createMainWindow() {
	mainWindow = new BrowserWindow({
		width: 1400,
		height: 900,
		icon: path.join(__dirname, 'icon.png'),
		autoHideMenuBar: true,
		backgroundColor: FRAME_COLORS.dark,
		...(IS_MAC ? {titleBarStyle: 'hidden'} : {frame: false}),
		webPreferences: {
			preload: path.join(__dirname, 'preload.js'),
			contextIsolation: true,
			nodeIntegration: false,
			sandbox: true,
		},
	})

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
		config = {...config, serverUrl}
		saveConfig(config)
	}
	return serverUrl
})

ipcMain.on('theme:set', (_event, theme) => {
	const color = FRAME_COLORS[theme]
	if (!color || !mainWindow) return
	mainWindow.setBackgroundColor(color)
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
