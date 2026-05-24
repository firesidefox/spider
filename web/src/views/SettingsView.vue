<template>
  <div class="settings-layout">
    <nav class="settings-sidebar">
      <div class="sidebar-section">系统设置</div>
      <router-link class="sidebar-item" to="/settings/prometheus">Data Sources · Prometheus</router-link>
    </nav>
    <div class="settings-content">
      <router-view />
    </div>
  </div>
</template>

<style scoped>
.settings-layout { display: flex; height: 100%; min-height: 0; }
.settings-sidebar { width: 220px; background: var(--bg-secondary, #181b1f); border-right: 1px solid var(--border, #2c2f36); padding: 16px 0; flex-shrink: 0; }
.sidebar-section { padding: 6px 16px; font-size: 11px; font-weight: 600; color: #6c7280; text-transform: uppercase; letter-spacing: .06em; margin-top: 12px; }
.sidebar-item { display: block; padding: 7px 16px; color: #9ca3af; font-size: 13px; border-left: 2px solid transparent; text-decoration: none; }
.sidebar-item:hover { background: #1f2228; color: #d9d9d9; }
.sidebar-item.router-link-active { background: #1a2035; color: #5794f2; border-left-color: #5794f2; font-weight: 500; }
.settings-content { flex: 1; overflow-y: auto; padding: 32px 40px; }
</style>
