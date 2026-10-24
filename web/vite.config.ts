/// <reference types="vitest/config" />
import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import {fileURLToPath, URL} from 'node:url'
import {execSync} from 'node:child_process'

const apiTarget = process.env.VIKUNJA_API ?? 'http://localhost:3456'

function git(args: string): string {
	try {
		return execSync(`git ${args}`, {stdio: ['ignore', 'pipe', 'ignore']}).toString().trim()
	} catch {
		return ''
	}
}

// The desktop release sets APP_VERSION; otherwise derive it from the nearest
// release tag (e.g. "0.1.34-2-gabc1234" for unreleased commits on top of it).
// The Docker build has no git checkout and passes only the commit.
const appVersion = process.env.APP_VERSION
	|| git('describe --tags --match "desktop-v*"').replace(/^desktop-v/, '')
const appCommit = process.env.APP_COMMIT || git('rev-parse --short HEAD')

export default defineConfig({
	plugins: [vue()],
	define: {
		__APP_VERSION__: JSON.stringify(appVersion),
		__APP_COMMIT__: JSON.stringify(appCommit),
	},
	resolve: {
		alias: {'@': fileURLToPath(new URL('./src', import.meta.url))},
	},
	server: {
		port: 5180,
		proxy: {
			'/api': {target: apiTarget, changeOrigin: true},
		},
	},
	build: {
		// web/embed.go embeds dist/ into the Go binary; the desktop build overrides it via WEB_OUT_DIR.
		outDir: process.env.WEB_OUT_DIR ?? 'dist',
		emptyOutDir: true,
	},
	test: {
		// lib/viewport.ts reads matchMedia on import.
		environment: 'happy-dom',
	},
})
