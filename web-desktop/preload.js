const {contextBridge, ipcRenderer} = require('electron')

// Consumed by web/src/lib/desktop.ts.
contextBridge.exposeInMainWorld('ttDesktop', {
	getServerUrl: () => ipcRenderer.invoke('server:get'),
	setServerUrl: url => ipcRenderer.invoke('server:set', url),
	onUpdateReady: callback => {
		ipcRenderer.removeAllListeners('update:ready')
		ipcRenderer.on('update:ready', (_event, version) => callback(version))
	},
	installUpdate: () => ipcRenderer.invoke('update:install'),
	setTheme: theme => ipcRenderer.send('theme:set', theme),
	getWindowInfo: () => ipcRenderer.invoke('window:info'),
	onMaximizedChange: callback => {
		ipcRenderer.removeAllListeners('window:maximized')
		ipcRenderer.on('window:maximized', (_event, maximized) => callback(maximized))
	},
	minimizeWindow: () => ipcRenderer.send('window:minimize'),
	toggleMaximizeWindow: () => ipcRenderer.send('window:toggle-maximize'),
	closeWindow: () => ipcRenderer.send('window:close'),
})
