/// <reference types="vite/client" />

// Injected by vite.config.ts; empty when unknown.
declare const __APP_VERSION__: string
declare const __APP_COMMIT__: string

declare module '*.vue' {
	import type {DefineComponent} from 'vue'
	const component: DefineComponent<object, object, unknown>
	export default component
}
