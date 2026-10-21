import {ref} from 'vue'

// Phone layout: bottom tab bar, sidebars as sheets, full-screen dialogs. CSS
// media queries can't read variables, so components repeat this breakpoint as
// `@media (max-width: 767px)`; keep them in sync. 768px (iPad portrait) stays
// on the desktop layout.
const query = window.matchMedia('(max-width: 767px)')

export const isMobile = ref(query.matches)

query.addEventListener('change', e => {
	isMobile.value = e.matches
})
