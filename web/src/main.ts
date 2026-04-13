import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/hosts' },
    { path: '/hosts',    component: () => import('./views/HostsView.vue') },
    { path: '/exec',     component: () => import('./views/ExecView.vue') },
    { path: '/audit',    component: () => import('./views/AuditView.vue') },
    { path: '/settings', component: () => import('./views/SettingsView.vue') },
  ],
})

createApp(App).use(router).mount('#app')
