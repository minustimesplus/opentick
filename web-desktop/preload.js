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
})
