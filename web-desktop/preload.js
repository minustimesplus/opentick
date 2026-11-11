const {contextBridge, ipcRenderer} = require('electron')

// Consumed by web/src/lib/desktop.ts.
contextBridge.exposeInMainWorld('ttDesktop', {
	getServerUrl: () => ipcRenderer.invoke('server:get'),
	setServerUrl: url => ipcRenderer.invoke('server:set', url),
	onUpdateReady: callback => {
		ipcRenderer.removeAllListeners('update:ready')
		ipcRenderer.on('update:ready', (_event, version) => callback(version))
	},
	signInWithBrowser: () => ipcRenderer.invoke('auth:browser-sign-in'),
	cancelBrowserSignIn: () => ipcRenderer.send('auth:cancel-browser-sign-in'),
	installUpdate: () => ipcRenderer.invoke('update:install'),
	checkForUpdates: () => ipcRenderer.invoke('update:check'),
	setTheme: theme => ipcRenderer.send('theme:set', theme),
	getWindowInfo: () => ipcRenderer.invoke('window:info'),
	onMaximizedChange: callback => {
		ipcRenderer.removeAllListeners('window:maximized')
		ipcRenderer.on('window:maximized', (_event, maximized) => callback(maximized))
	},
	minimizeWindow: () => ipcRenderer.send('window:minimize'),
	toggleMaximizeWindow: () => ipcRenderer.send('window:toggle-maximize'),
	closeWindow: () => ipcRenderer.send('window:close'),
	// Files for the offline copy of the account (web/src/api/offline).
	offlineStore: {
		read: name => ipcRenderer.invoke('offline:read', name),
		write: (name, text) => ipcRenderer.invoke('offline:write', name, text),
		writeSync: (name, text) => ipcRenderer.sendSync('offline:write-sync', name, text),
		clear: () => ipcRenderer.invoke('offline:clear'),
	},
})
