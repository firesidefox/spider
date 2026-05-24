import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import { useAuth } from './composables/useAuth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/hosts' },
    { path: '/login', component: () => import('./views/LoginView.vue') },
    { path: '/hosts',    component: () => import('./views/HostsView.vue') },
    { path: '/exec',     component: () => import('./views/ExecView.vue') },
    { path: '/chat',     component: () => import('./views/ChatView.vue') },
    { path: '/chat/:id', component: () => import('./views/ChatView.vue') },
    { path: '/install',  component: () => import('./views/InstallView.vue') },
    { path: '/users',    component: () => import('./views/UsersView.vue') },
    { path: '/setting',  component: () => import('./views/ProfileView.vue') },
    { path: '/knowledge', component: () => import('./views/KnowledgeView.vue') },
    { path: '/topology',  component: () => import('./views/TopologyView.vue') },
    { path: '/tasks',    component: () => import('./views/TasksView.vue') },
  ],
})

router.beforeEach(async (to) => {
  if (to.path === '/login') return true
  const { checkAuth, isAdmin } = useAuth()
  const ok = await checkAuth()
  if (!ok) return '/login'
  if (to.path === '/users' && !isAdmin.value) return '/hosts'
  return true
})

createApp(App).use(router).mount('#app')
