import {ref} from 'vue'
import {createRouter, createWebHistory} from 'vue-router'
import {openSettings, ui} from './lib/ui'
import {getToken} from './api/client'
import {routeAllowed} from './lib/features'
import {OAUTH_AUTHORIZE_PATH, takeOAuthReturn} from './lib/oauthReturn'
import {INVITE_PATH, rememberInviteToken} from './lib/inviteLink'
import {DEMO, HOSTED_SIGNUP_URL} from './lib/localMode'
import {useData} from './stores/data'

export const router = createRouter({
	history: createWebHistory(import.meta.env.BASE_URL),
	routes: [
		{path: '/', redirect: '/tasks/today'},
		{path: '/tasks', redirect: '/tasks/today'},
		{path: '/tasks/:list/:taskId?', name: 'tasks', component: () => import('./views/TasksView.vue')},
		{path: '/matrix', name: 'matrix', component: () => import('./views/MatrixView.vue')},
		{path: '/calendar', name: 'calendar', component: () => import('./views/CalendarView.vue')},
		{path: '/statistics', name: 'statistics', component: () => import('./views/StatisticsView.vue')},
		{path: '/habits/:habitId?', name: 'habits', component: () => import('./views/HabitsView.vue')},
		{path: '/focus', name: 'focus', component: () => import('./views/FocusView.vue')},
		{path: '/countdowns', name: 'countdowns', component: () => import('./views/CountdownsView.vue')},
		{path: '/people/:list?/:contactId?', name: 'people', component: () => import('./views/PeopleView.vue')},
		{path: '/search', name: 'search', component: () => import('./views/SearchView.vue')},
		// Linkable sign up and sign in pages (e.g. from the README or the demo).
		// Signed out, App shows LoginView in the matching mode; signed in, they
		// lead into the app.
		{path: '/signup', name: 'signup', component: {render: () => null}, beforeEnter: async () => {
			// The demo has no server: send people to the hosted instance's page.
			if (DEMO) {
				if (HOSTED_SIGNUP_URL) window.location.assign(HOSTED_SIGNUP_URL)
				return '/tasks/today'
			}
			if (getToken()) return '/tasks/today'
			// Using the app without an account: signing up means leaving that
			// mode. The local data stays in the browser.
			const data = useData()
			if (data.local) await data.logout()
			return true
		}},
		{path: '/login', name: 'login', component: {render: () => null}, beforeEnter: () => getToken() || DEMO ? '/tasks/today' : true},
		// OAuth providers return here; the login screen finishes the sign in.
		{path: '/sso-callback', component: {render: () => null}, beforeEnter: () => getToken() ? (takeOAuthReturn() ?? '/tasks/today') : true},
		// OAuth consent for MCP clients; App renders OAuthConsentView instead of the shell.
		{path: OAUTH_AUTHORIZE_PATH, name: 'oauth-authorize', component: {render: () => null}},
		// Invitation links: App asks whether to join once the user is signed in.
		{path: INVITE_PATH, redirect: to => {
			rememberInviteToken(String(to.params.token))
			ui.inviteLink++
			return '/tasks/today'
		}},
		// Linked from invitation emails to existing users.
		{path: '/invitations', redirect: () => {
			ui.invitations++
			return '/tasks/today'
		}},
		{path: '/settings', redirect: () => {
			openSettings()
			return '/tasks/today'
		}},
		{path: '/:pathMatch(.*)*', redirect: '/tasks/today'},
	],
})

// Views of features turned off in Settings > Features are not reachable.
router.beforeEach(to => routeAllowed(to) || '/tasks/today')

// The list last open in the tasks view, so coming back from another view
// (the rail's tasks button) lands on it instead of Today.
export const lastTasksList = ref('/tasks/today')
router.afterEach(to => {
	if (to.name === 'tasks' && typeof to.params.list === 'string') lastTasksList.value = `/tasks/${to.params.list}`
})
