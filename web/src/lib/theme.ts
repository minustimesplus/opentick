import {ref, watchEffect} from 'vue'
import {prefs} from './prefs'
import {desktop} from './desktop'

export type ThemeColor =
	| 'default' | 'sky' | 'turquoise' | 'teal' | 'matcha' | 'sunshine' | 'peach' | 'lilac'
	| 'ebony' | 'navy' | 'gray' | 'dark'

export interface ColorSeries {
	key: ThemeColor
	/** Swatch shown in the settings grid. */
	swatch: string
}

// Order and swatches follow TickTick's "Color Series". The tint/frame values
// each series applies live in styles.css under :root[data-color="…"].
export const colorSeries: ColorSeries[] = [
	{key: 'default', swatch: '#ffffff'},
	{key: 'sky', swatch: 'linear-gradient(135deg, #6f8cff, #4a6cf7)'},
	{key: 'turquoise', swatch: 'linear-gradient(135deg, #6fe0bf, #3cc9a3)'},
	{key: 'teal', swatch: 'linear-gradient(135deg, #7fd0d6, #4fb3c2)'},
	{key: 'matcha', swatch: 'linear-gradient(135deg, #c4cfb0, #a3b493)'},
	{key: 'sunshine', swatch: 'linear-gradient(135deg, #ffd38a, #f8b55a)'},
	{key: 'peach', swatch: 'linear-gradient(135deg, #ffa4ae, #f6798f)'},
	{key: 'lilac', swatch: 'linear-gradient(135deg, #c6b8ec, #9f92dd)'},
	{key: 'ebony', swatch: 'linear-gradient(135deg, #c3a58f, #9d7f69)'},
	{key: 'navy', swatch: 'linear-gradient(135deg, #3d4a85, #273163)'},
	{key: 'gray', swatch: 'linear-gradient(135deg, #575b66, #363941)'},
	{key: 'dark', swatch: '#0d0d0d'},
]

const media = window.matchMedia('(prefers-color-scheme: dark)')
const systemDark = ref(media.matches)
media.addEventListener('change', e => {
	systemDark.value = e.matches
})

/** The series actually shown, after following the OS dark mode. */
export function activeThemeColor(): ThemeColor {
	return prefs.followSystemDark && systemDark.value ? 'dark' : prefs.themeColor
}

/** Applies the theme to <html> and the desktop window frame. */
export function applyTheme() {
	watchEffect(() => {
		const color = activeThemeColor()
		const root = document.documentElement
		root.dataset.theme = color === 'dark' ? 'dark' : 'light'
		if (color === 'dark' || color === 'default') {
			delete root.dataset.color
		} else {
			root.dataset.color = color
		}
		const frame = getComputedStyle(root).getPropertyValue('--frame').trim()
		desktop?.setTheme?.(frame || root.dataset.theme)
	})
}
