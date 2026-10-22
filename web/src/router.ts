import {createRouter, createWebHistory} from 'vue-router'
import {openSettings} from './lib/ui'
import {getToken} from './api/client'

export const router = createRouter({
	history: createWebHistory(),
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
		{path: '/search', name: 'search', component: () => import('./views/SearchView.vue')},
		// OAuth providers return here; the login screen finishes the sign in.
		{path: '/sso-callback', component: {render: () => null}, beforeEnter: () => getToken() ? '/tasks/today' : true},
		{path: '/settings', redirect: () => {
			openSettings()
			return '/tasks/today'
		}},
		{path: '/:pathMatch(.*)*', redirect: '/tasks/today'},
	],
})
