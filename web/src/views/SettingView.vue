<template>
  <div class="fullscreen-page profile-page">
    <aside class="profile-sidebar">
      <div class="sidebar-toolbar">
        <div class="sidebar-user">
          <span class="sidebar-username">{{ currentUser?.username }}</span>
          <span class="role-badge" :class="currentUser?.role">{{ roleLabel }}</span>
        </div>
      </div>
      <nav class="sidebar-list">
        <div class="nav-section-label">个人</div>
        <div class="nav-row" :class="{ selected: activeTab === 'info' }" @click="activeTab = 'info'">
          <span class="nav-icon">👤</span><span class="nav-label">基本信息</span>
        </div>
        <div class="nav-row" :class="{ selected: activeTab === 'tokens' }" @click="activeTab = 'tokens'">
          <span class="nav-icon">🔑</span><span class="nav-label">访问令牌</span>
        </div>
        <div class="nav-row" :class="{ selected: activeTab === 'ssh-keys' }" @click="activeTab = 'ssh-keys'">
          <span class="nav-icon">🔐</span><span class="nav-label">SSH Keys</span>
        </div>
        <div class="nav-row" :class="{ selected: activeTab === 'logs' }" @click="activeTab = 'logs'">
          <span class="nav-icon">📋</span><span class="nav-label">操作日志</span>
        </div>
        <div class="nav-row" :class="{ selected: activeTab === 'chat-theme' }" @click="activeTab = 'chat-theme'">
          <span class="nav-icon">🎨</span><span class="nav-label">对话框主题</span>
        </div>
        <template v-if="isAdmin">
          <div class="nav-section-label">管理</div>
          <div class="nav-row" :class="{ selected: activeTab === 'users' }" @click="activeTab = 'users'">
            <span class="nav-icon">👥</span><span class="nav-label">用户管理</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'audit' }" @click="activeTab = 'audit'">
            <span class="nav-icon">📋</span><span class="nav-label">审计日志</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'notify' }" @click="activeTab = 'notify'">
            <span class="nav-icon">🔔</span><span class="nav-label">通知渠道</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'settings' }" @click="activeTab = 'settings'">
            <span class="nav-icon">⚙️</span><span class="nav-label">偏好设置</span>
          </div>
          <div class="nav-section-label">Agent</div>
          <div class="nav-row" :class="{ selected: activeTab === 'agent' }" @click="activeTab = 'agent'">
            <span class="nav-icon">🧠</span><span class="nav-label">智能体</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'kb' }" @click="activeTab = 'kb'">
            <span class="nav-icon">📚</span><span class="nav-label">知识库</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'skills' }" @click="activeTab = 'skills'">
            <span class="nav-icon">🧩</span><span class="nav-label">Skills</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'datasources' }" @click="activeTab = 'datasources'">
            <span class="nav-icon">📡</span><span class="nav-label">数据源</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'install' }" @click="activeTab = 'install'">
            <span class="nav-icon">📦</span><span class="nav-label">安装</span>
          </div>
        </template>
      </nav>
    </aside>
    <div class="profile-detail">
      <template v-if="activeTab === 'users'">
        <UsersPanel />
      </template>
      <template v-else-if="activeTab === 'tokens'">
        <TokenSettings />
      </template>
      <template v-else-if="activeTab === 'audit'">
        <AuditLogs />
      </template>
      <template v-else-if="activeTab === 'install'">
        <InstallPanel @switch-tab="activeTab = $event as any" />
      </template>
      <template v-else-if="activeTab === 'skills'">
        <SkillsPanel />
      </template>
      <template v-else-if="activeTab === 'datasources'">
        <div class="detail-topbar">
          <span class="detail-title">数据源</span>
        </div>
        <div class="datasources-subtabs">
          <span class="datasources-subtab active">Prometheus</span>
        </div>
        <div class="detail-body">
          <PrometheusDataSourcesPanel />
        </div>
      </template>
      <template v-else-if="activeTab === 'chat-theme'">
        <ChatThemeSettings />
      </template>
      <template v-else-if="activeTab === 'ssh-keys'">
        <SSHKeySettings />
      </template>
      <template v-else-if="activeTab === 'logs'">
        <LogsViewer />
      </template>
      <template v-else-if="activeTab === 'notify'">
        <NotifyChannelSettings />
      </template>
      <template v-else-if="activeTab === 'agent'">
        <AgentSettings />
      </template>
      <template v-else-if="activeTab === 'kb'">
        <RagSettings />
      </template>
      <template v-else-if="activeTab === 'settings'">
        <ProviderSettings />
      </template>
      <template v-else>
        <div class="detail-topbar">
          <span class="detail-title">{{ tabTitle }}</span>
        </div>
        <div class="detail-body">
        <template v-if="activeTab === 'info'">
          <div class="detail-grid">
            <div class="detail-field">
              <div class="detail-label">用户名</div>
              <div class="detail-value">{{ currentUser?.username }}</div>
            </div>
            <div class="detail-field">
              <div class="detail-label">角色</div>
              <div class="detail-value"><span class="role-badge" :class="currentUser?.role">{{ roleLabel }}</span></div>
            </div>
            <div class="detail-field" v-if="currentUser?.created_at">
              <div class="detail-label">注册时间</div>
              <div class="detail-value dim">{{ new Date(currentUser.created_at).toLocaleString() }}</div>
            </div>
            <div class="detail-field" v-if="currentUser?.last_login">
              <div class="detail-label">上次登录</div>
              <div class="detail-value dim">{{ new Date(currentUser.last_login).toLocaleString() }}</div>
            </div>
          </div>
          <PasswordSettings />
        </template>



      </div>
      </template><!-- end v-else -->
    </div>


  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import UsersPanel from '@/components/settings/UsersPanel.vue'
import AuditLogs from '@/components/settings/AuditLogs.vue'
import InstallPanel from '@/components/settings/InstallPanel.vue'
import SkillsPanel from '@/components/settings/SkillsPanel.vue'
import PrometheusDataSourcesPanel from '@/components/settings/PrometheusDataSourcesPanel.vue'
import PasswordSettings from '../components/settings/PasswordSettings.vue'
import ChatThemeSettings from '../components/settings/ChatThemeSettings.vue'
import TokenSettings from '../components/settings/TokenSettings.vue'
import SSHKeySettings from '@/components/settings/SSHKeySettings.vue'
import LogsViewer from '@/components/settings/LogsViewer.vue'
import NotifyChannelSettings from '@/components/settings/NotifyChannelSettings.vue'
import ProviderSettings from '@/components/settings/ProviderSettings.vue'
import RagSettings from '@/components/settings/RagSettings.vue'
import AgentSettings from '@/components/settings/AgentSettings.vue'

const { currentUser, isAdmin } = useAuth()
const route = useRoute()
const router = useRouter()

const roleLabel = computed(() => {
  const map: Record<string, string> = { admin: '管理员', operator: '操作员', viewer: '只读' }
  return map[currentUser.value?.role ?? ''] ?? currentUser.value?.role ?? '—'
})

const allowedTabs = computed(() => {
  const base = ['info', 'tokens', 'ssh-keys', 'logs', 'chat-theme']
  return isAdmin.value ? [...base, 'users', 'audit', 'install', 'skills', 'agent', 'kb', 'settings', 'notify', 'datasources'] : base
})

const queryTab = route.query.tab as string
const initialTab = allowedTabs.value.includes(queryTab) ? queryTab : 'info'
const activeTab = ref<'info' | 'tokens' | 'ssh-keys' | 'logs' | 'chat-theme' | 'users' | 'audit' | 'install' | 'skills' | 'agent' | 'kb' | 'settings' | 'notify' | 'datasources'>(initialTab)
watch(activeTab, (tab) => router.replace({ query: { tab } }))
const tabTitle = computed(() => ({
  info: '基本信息', tokens: '访问令牌', 'ssh-keys': 'SSH Keys', logs: '操作日志',
  'chat-theme': '对话框主题',
  users: '用户管理', install: '安装', agent: '智能体', kb: '知识库', settings: '偏好设置', notify: '通知渠道',
}[activeTab.value]))


</script>

<style scoped>
.profile-page {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.profile-sidebar {
  width: 220px;
  flex-shrink: 0;
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sidebar-toolbar {
  padding: 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.sidebar-user { display: flex; flex-direction: column; gap: 8px; }
.sidebar-username { font-size: 15px; font-weight: 600; color: var(--text); }

.sidebar-list { flex: 1; overflow-y: auto; padding: 8px 0; }

.nav-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  cursor: pointer;
  font-size: 14px;
  color: var(--text-sub);
  border-left: 3px solid transparent;
  transition: background 0.1s, color 0.1s;
}

.nav-row:hover { background: var(--row-hover); }

.nav-row.selected {
  color: var(--primary);
  background: rgba(99,102,241,0.1);
  border-left-color: var(--primary);
}

.nav-icon { font-size: 15px; }
.nav-label { font-size: 14px; font-weight: 500; }

.nav-section-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  padding: 12px 16px 4px;
}

.profile-detail {
  flex: 1;
  overflow: hidden;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.detail-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  flex-shrink: 0;
}

.detail-title { font-size: 15px; font-weight: 700; color: var(--text); }

.detail-body { flex: 1; overflow-y: auto; padding: 20px 24px; }

.detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 16px;
}

.detail-field {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px 20px;
  box-shadow: var(--card-shadow);
}

.detail-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.07em;
  margin-bottom: 6px;
}

.detail-value { font-size: 15px; font-weight: 600; color: var(--text); }

.role-badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid transparent;
}
.role-badge.admin    { background: rgba(99,102,241,0.12); color: var(--primary); border-color: rgba(99,102,241,0.3); }
.role-badge.operator { background: rgba(74,222,128,0.12); color: var(--green);   border-color: rgba(74,222,128,0.3); }
.role-badge.viewer   { background: rgba(167,139,250,0.1); color: var(--purple);  border-color: rgba(167,139,250,0.25); }

.datasources-subtabs { display: flex; gap: 0; border-bottom: 1px solid var(--border); padding: 0 20px; flex-shrink: 0; background: var(--surface); }
.datasources-subtab { padding: 10px 16px; font-size: 13px; color: var(--text-sub); cursor: pointer; border-bottom: 2px solid transparent; margin-bottom: -1px; }
.datasources-subtab.active { color: var(--primary); border-bottom-color: var(--primary); font-weight: 500; }
</style>
