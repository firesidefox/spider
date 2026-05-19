<template>
  <div class="app">
    <header v-if="route.path !== '/login'" class="nav">
      <div class="nav-brand">🕷 Spider</div>
      <nav class="nav-links">
        <RouterLink to="/hosts" class="nav-item">主机管理</RouterLink>
        <RouterLink to="/exec" class="nav-item">命令执行</RouterLink>
        <RouterLink to="/chat" class="nav-item">智能运维</RouterLink>
        <RouterLink to="/knowledge" class="nav-item">知识库</RouterLink>
        <RouterLink to="/topology" class="nav-item">网络拓扑</RouterLink>
        <RouterLink to="/tasks" class="nav-item">任务管理</RouterLink>
        <RouterLink to="/profile" class="nav-item">系统设置</RouterLink>
      </nav>
      <div v-if="currentUser" class="nav-user">
        <div class="user-dropdown" @click.stop="showUserMenu = !showUserMenu">
          <span class="nav-username nav-username--clickable">{{ currentUser.username }} ▾</span>
          <div v-if="showUserMenu" class="dropdown-menu">
            <button class="dropdown-item" @click="handleLogout">登出</button>
          </div>
        </div>
      </div>
      <div class="theme-dropdown-wrap" @click.stop="showThemeMenu = !showThemeMenu">
        <button class="theme-toggle">
          {{ themeMode === 'light' ? '☀️' : themeMode === 'dark' ? '🌙' : '🖥' }}
        </button>
        <div v-if="showThemeMenu" class="theme-dropdown">
          <button class="theme-option" :class="{ active: themeMode === 'light' }" @click.stop="setTheme('light')">
            <span class="theme-option-icon">☀️</span>
            <span class="theme-option-text"><strong>浅色模式</strong><small>始终使用浅色主题</small></span>
          </button>
          <button class="theme-option" :class="{ active: themeMode === 'dark' }" @click.stop="setTheme('dark')">
            <span class="theme-option-icon">🌙</span>
            <span class="theme-option-text"><strong>深色模式</strong><small>始终使用深色主题</small></span>
          </button>
          <button class="theme-option" :class="{ active: themeMode === 'system' }" @click.stop="setTheme('system')">
            <span class="theme-option-icon">🖥</span>
            <span class="theme-option-text"><strong>自动模式</strong><small>跟随系统主题设置</small></span>
          </button>
          <div v-if="themeMode === 'system'" class="theme-system-hint">当前跟随系统：{{ systemIsDark ? '深色' : '浅色' }}</div>
        </div>
      </div>
    </header>
    <main class="main">
      <KeepAlive include="ChatView">
        <RouterView />
      </KeepAlive>
    </main>
    <AgentStatusBar v-if="route.path !== '/login'" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watchEffect, provide, onMounted, onUnmounted, KeepAlive } from 'vue'
import { themes, getSavedTheme, saveTheme, type Theme } from './theme'

const themeMode = ref<Theme>(getSavedTheme())
const systemIsDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)
const showThemeMenu = ref(false)

let mql: MediaQueryList
onMounted(() => {
  mql = window.matchMedia('(prefers-color-scheme: dark)')
  systemIsDark.value = mql.matches
  mql.addEventListener('change', onSystemChange)
})
onUnmounted(() => { mql?.removeEventListener('change', onSystemChange) })
function onSystemChange(e: MediaQueryListEvent) { systemIsDark.value = e.matches }

const resolvedTheme = computed(() => {
  if (themeMode.value === 'system') return systemIsDark.value ? 'dark' : 'light'
  return themeMode.value
})
const isDark = computed(() => resolvedTheme.value === 'dark')

function setTheme(mode: Theme) {
  themeMode.value = mode
  saveTheme(mode)
  showThemeMenu.value = false
}

provide('isDark', () => isDark.value)

import { useRouter, useRoute } from 'vue-router'
import { useAuth } from './composables/useAuth'
import { logout } from './api/auth'
import AgentStatusBar from './components/AgentStatusBar.vue'

const router = useRouter()
const route = useRoute()
const { currentUser, isAdmin, clearUser } = useAuth()

const showUserMenu = ref(false)

function closeMenus() { showUserMenu.value = false; showThemeMenu.value = false }
onMounted(() => document.addEventListener('click', closeMenus))
onUnmounted(() => document.removeEventListener('click', closeMenus))

async function handleLogout() {
  await logout().catch(() => {})
  clearUser()
  router.push('/login')
}

watchEffect(() => {
  const c = themes[resolvedTheme.value]
  const root = document.documentElement
  root.style.setProperty('--bg', c.bg)
  root.style.setProperty('--nav', c.nav)
  root.style.setProperty('--nav-border', c.navBorder)
  root.style.setProperty('--surface', c.surface)
  root.style.setProperty('--card-bg', c.cardBg)
  root.style.setProperty('--panel', c.panel)
  root.style.setProperty('--border', c.border)
  root.style.setProperty('--border-focus', c.borderFocus)
  root.style.setProperty('--primary', c.primary)
  root.style.setProperty('--primary-hover', c.primaryHover)
  root.style.setProperty('--accent', c.accent)
  root.style.setProperty('--text', c.text)
  root.style.setProperty('--text-sub', c.textSub)
  root.style.setProperty('--muted', c.muted)
  root.style.setProperty('--label', c.label)
  root.style.setProperty('--green', c.green)
  root.style.setProperty('--red', c.red)
  root.style.setProperty('--yellow', c.yellow)
  root.style.setProperty('--purple', c.purple)
  root.style.setProperty('--input-bg', c.inputBg)
  root.style.setProperty('--row-alt', c.rowAlt)
  root.style.setProperty('--row-hover', c.rowHover)
  root.style.setProperty('--card-shadow', c.cardShadow)
})
</script>

<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', sans-serif;
  background: var(--bg);
  color: var(--text);
  transition: background 0.2s, color 0.2s;
}

/* ── 导航栏 ── */
.app { min-height: 100vh; height: 100vh; display: flex; flex-direction: column; overflow: hidden; }

.nav {
  background: var(--nav);
  border-bottom: 1px solid var(--nav-border);
  display: flex;
  align-items: stretch;
  padding: 0 24px;
  height: 52px;
  gap: 0;
  position: sticky;
  top: 0;
  z-index: 50;
  backdrop-filter: blur(8px);
}

.nav-brand {
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.3px;
  color: var(--text);
  flex-shrink: 0;
  display: flex;
  align-items: center;
  padding-right: 24px;
  border-right: 1px solid var(--nav-border);
  margin-right: 8px;
}

.nav-links { display: flex; gap: 0; flex: 1; align-items: stretch; }

.nav-item {
  color: var(--muted);
  text-decoration: none;
  padding: 0 16px;
  font-size: 14px;
  font-weight: 500;
  transition: color 0.15s;
  display: flex;
  align-items: center;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}

.nav-item:hover { color: var(--text); }

.nav-item.router-link-active {
  color: var(--primary);
  border-bottom-color: var(--primary);
}

.theme-dropdown-wrap {
  position: relative;
  align-self: center;
  flex-shrink: 0;
}
.theme-toggle {
  background: none;
  border: 1px solid var(--border);
  border-radius: 8px;
  width: 32px;
  height: 32px;
  cursor: pointer;
  font-size: 15px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s;
}
.theme-toggle:hover { background: var(--row-hover); }
.theme-dropdown {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 8px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 8px;
  min-width: 200px;
  box-shadow: var(--card-shadow);
  z-index: 100;
}
.theme-option {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  border: none;
  background: none;
  border-radius: 8px;
  cursor: pointer;
  color: var(--text);
  font-size: 14px;
  text-align: left;
}
.theme-option:hover { background: var(--row-hover); }
.theme-option.active { background: var(--row-hover); font-weight: 600; }
.theme-option-icon { font-size: 18px; flex-shrink: 0; }
.theme-option-text { display: flex; flex-direction: column; gap: 2px; }
.theme-option-text strong { font-size: 14px; font-weight: 500; }
.theme-option.active .theme-option-text strong { font-weight: 700; }
.theme-option-text small { font-size: 12px; color: var(--muted); }
.theme-system-hint {
  padding: 8px 12px 4px;
  font-size: 12px;
  color: var(--muted);
  border-top: 1px solid var(--border);
  margin-top: 4px;
}

.main {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 全屏子页面：突破 main 的 padding/max-width */
.main:has(.fullscreen-page) {
  padding: 0;
  max-width: 100%;
  margin: 0;
  overflow: hidden;
}

/* 普通页面内容区：居中限宽 */
.page-content {
  padding: 24px;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  flex: 1;
  min-height: 0;
}

/* CSS 变量：nav 高度，供子页面使用 */
:root { --nav-h: 52px; }

/* ── 通用组件 ── */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 20px;
  font-weight: 700;
  color: var(--text);
  letter-spacing: -0.02em;
}

.toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

/* ── 输入框 ── */
.input {
  background: var(--input-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 14px;
  color: var(--text);
  outline: none;
  width: 100%;
  transition: border-color 0.15s, box-shadow 0.15s;
  font-family: inherit;
}
.input::placeholder { color: var(--label); }
.input:focus {
  border-color: var(--border-focus);
  box-shadow: 0 0 0 3px rgba(99,102,241,0.15);
}
textarea.input { resize: vertical; }

/* ── 按钮 ── */
.btn {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 8px 16px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-sub);
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
  white-space: nowrap;
  font-family: inherit;
}
.btn:hover { background: var(--row-hover); border-color: var(--border-focus); color: var(--text); }
.btn:disabled { opacity: 0.45; cursor: not-allowed; }

.btn-primary {
  background: var(--primary);
  color: #fff;
  border-color: var(--primary);
}
.btn-primary:hover { background: var(--primary-hover); border-color: var(--primary-hover); color: #fff; }

.btn-danger { color: var(--red); border-color: rgba(248,113,113,0.3); }
.btn-danger:hover { background: rgba(248,113,113,0.08); border-color: var(--red); }

.btn-sm { padding: 5px 11px; font-size: 13px; }

/* ── 表格 ── */
.table {
  width: 100%;
  border-collapse: collapse;
  background: var(--card-bg);
  border-radius: 10px;
  overflow: hidden;
  box-shadow: var(--card-shadow);
  border: 1px solid var(--border);
}

.table th {
  background: var(--surface);
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  padding: 11px 14px;
  text-align: left;
  border-bottom: 1px solid var(--border);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.table td {
  padding: 11px 14px;
  font-size: 14px;
  color: var(--text-sub);
  border-bottom: 1px solid var(--border);
  transition: background 0.1s;
}

.table tr:last-child td { border-bottom: none; }
.table tbody tr:nth-child(even) td { background: var(--row-alt); }
.table tbody tr:hover td { background: var(--row-hover); }

/* ── 操作区 ── */
.actions { display: flex; gap: 6px; align-items: center; }

/* ── 徽章 ── */
.badge {
  background: rgba(99,102,241,0.12);
  color: var(--primary);
  border: 1px solid rgba(99,102,241,0.3);
  border-radius: 5px;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
}
.badge-ok { background: rgba(34,197,94,0.12); color: var(--green); border-color: rgba(34,197,94,0.3); }
.badge-err { background: rgba(239,68,68,0.12); color: var(--red); border-color: rgba(239,68,68,0.3); }

/* ── 标签 ── */
.tag {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  padding: 4px 12px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  color: var(--muted);
  transition: all 0.15s;
}
.tag:hover { border-color: var(--primary); color: var(--primary); }
.tag.active {
  background: rgba(99,102,241,0.12);
  color: var(--primary);
  border-color: rgba(99,102,241,0.4);
}
.tag.small { padding: 2px 8px; font-size: 12px; margin-right: 4px; }
.tags { display: flex; gap: 6px; flex-wrap: wrap; }

/* ── 弹窗 ── */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  backdrop-filter: blur(4px);
}

.modal {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 24px;
  width: 480px;
  max-width: 95vw;
  max-height: 90vh;
  overflow-y: auto;
  box-shadow: var(--card-shadow);
}
.modal.wide { width: 680px; }

.modal h3 {
  font-size: 16px;
  font-weight: 700;
  margin-bottom: 20px;
  color: var(--text);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}

/* ── 表单 ── */
.form-row { display: flex; flex-direction: column; gap: 6px; margin-bottom: 14px; }

.form-row label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

/* ── 卡片 ── */
.card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: var(--card-shadow);
}

/* ── 杂项 ── */
.section-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  margin-bottom: 10px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.bulk-bar {
  margin-top: 12px;
  display: flex;
  gap: 12px;
  align-items: center;
  font-size: 14px;
  color: var(--text-sub);
  padding: 10px 14px;
  background: rgba(99,102,241,0.06);
  border: 1px solid rgba(99,102,241,0.2);
  border-radius: 8px;
}

.output {
  background: var(--panel);
  color: var(--text);
  padding: 12px 14px;
  border-radius: 8px;
  font-size: 13px;
  overflow-x: hidden;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin-top: 8px;
  font-family: 'SF Mono', Consolas, 'Courier New', monospace;
  border: 1px solid var(--border);
  line-height: 1.6;
  max-height: 400px;
}
.output.stderr { color: #f38ba8; }

.ok { color: var(--green); font-size: 13px; font-weight: 600; }
.err { color: var(--red); font-size: 13px; font-weight: 600; }
.dim { color: var(--label); font-size: 13px; }

.code { font-family: 'SF Mono', Consolas, monospace; font-size: 13px; }

.truncate { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* ── 命令执行布局（已移至 ExecView scoped） ── */

/* ── 设置 ── */
.settings-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 20px 24px;
  box-shadow: var(--card-shadow);
  margin-bottom: 16px;
}

.settings-card h3 {
  font-size: 14px;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

/* ── 详情元数据 ── */
.detail-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  font-size: 13px;
  margin-bottom: 16px;
  color: var(--text-sub);
  padding: 12px 14px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
}

/* ── 分页 ── */
.pagination {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 16px;
  justify-content: center;
}

.nav-user { display: flex; align-items: center; gap: 10px; margin-right: 12px; }
.nav-username { font-size: 13px; color: var(--text-sub); }
.nav-username--clickable { cursor: pointer; user-select: none; }
.nav-username--clickable:hover { color: var(--text); }
.user-dropdown { position: relative; display: flex; align-items: center; }
.dropdown-menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  min-width: 120px;
  box-shadow: var(--card-shadow);
  z-index: 100;
  overflow: hidden;
}
.dropdown-item {
  display: block;
  width: 100%;
  padding: 8px 16px;
  font-size: 13px;
  color: var(--text);
  background: none;
  border: none;
  text-align: left;
  cursor: pointer;
}
.dropdown-item:hover { background: var(--surface); }
</style>
