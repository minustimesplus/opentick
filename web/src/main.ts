import {createApp, nextTick} from 'vue'
import {createPinia} from 'pinia'
import App from './App.vue'
import {router} from './router'
import './styles.css'

const app = createApp(App)
app.use(createPinia()).use(router)
// `autofocus` only fires once per document; popovers mount inputs repeatedly.
app.directive('focus', {
	mounted(el: HTMLElement) {
		nextTick(() => el.focus())
	},
})
app.mount('#app')
