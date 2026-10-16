import {createRouter, createWebHistory} from 'vue-router'

export const router = createRouter({
	history: createWebHistory(),
	routes: [
		{path: '/', redirect: '/tasks/today'},
		{path: '/tasks', redirect: '/tasks/today'},
		{path: '/tasks/:list/:taskId?', name: 'tasks', component: () => import('./views/TasksView.vue')},
		{path: '/matrix', name: 'matrix', component: () => import('./views/MatrixView.vue')},
		{path: '/calendar', name: 'calendar', component: () => import('./views/CalendarView.vue')},
		{path: '/search', name: 'search', component: () => import('./views/SearchView.vue')},
		{path: '/settings', name: 'settings', component: () => import('./views/SettingsView.vue')},
		{path: '/:pathMatch(.*)*', redirect: '/tasks/today'},
	],
})
