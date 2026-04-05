import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import HostsView from './views/HostsView.vue'
import ExecView from './views/ExecView.vue'
import AuditView from './views/AuditView.vue'
import SettingsView from './views/SettingsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/hosts' },
    { path: '/hosts', component: HostsView },
    { path: '/exec', component: ExecView },
    { path: '/audit', component: AuditView },
    { path: '/settings', component: SettingsView },
  ],
})

createApp(App).use(router).mount('#app')
