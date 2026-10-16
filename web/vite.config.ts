import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import {fileURLToPath, URL} from 'node:url'

const apiTarget = process.env.VIKUNJA_API ?? 'http://localhost:3456'

export default defineConfig({
	plugins: [vue()],
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
		// Built straight into the directory the Go binary embeds (frontend/embed.go).
		outDir: process.env.WEB_OUT_DIR ?? 'dist',
		emptyOutDir: true,
	},
})
