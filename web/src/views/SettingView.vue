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
          <div class="nav-row" :class="{ selected: activeTab === 'agent' }" @click="activeTab = 'agent'; loadAgentSettings(); loadProviders()">
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

        <!-- Tab: 智能体 -->
        <template v-if="activeTab === 'agent'">
          <!-- 模型供应商 card -->
          <div class="edit-card">
            <div class="edit-card-title" style="display:flex;justify-content:space-between;align-items:center">
              <span>模型供应商</span>
              <button class="btn btn-primary btn-sm" @click="addProvider">+ 添加供应商</button>
            </div>
            <p class="dim" style="margin-bottom:16px;font-size:13px">配置 AI 模型供应商，用于智能运维对话和工具调用。</p>
            <table class="table">
              <thead><tr><th>名称</th><th>接口类型</th><th>请求地址</th><th>APIKey</th><th>模型</th><th>状态</th><th>操作</th></tr></thead>
              <tbody>
                <tr v-for="p in providers" :key="p.id">
                  <template v-if="editingProviderId === p.id">
                    <td><input v-model="editForm.name" class="input input-inline" placeholder="供应商名称" /></td>
                    <td>
                      <select v-model="editForm.type" class="input input-inline">
                        <option value="anthropic">Anthropic 兼容</option>
                        <option value="openai">OpenAI 兼容</option>
                      </select>
                    </td>
                    <td><input v-model="editForm.base_url" class="input input-inline" placeholder="留空使用默认" /></td>
                    <td><input v-model="editForm.api_key" class="input input-inline" placeholder="API Key" type="password" /></td>
                    <td></td>
                    <td></td>
                    <td style="white-space:nowrap">
                      <button class="btn btn-primary btn-sm" @click="saveProvider" style="margin-right:4px">保存</button>
                      <button class="btn btn-sm" @click="cancelEdit" style="margin-right:4px">取消</button>
                      <button class="btn btn-sm btn-danger" @click="removeProvider(p.id)">删除</button>
                    </td>
                  </template>
                  <template v-else>
                    <td>{{ p.name || '未命名' }}</td>
                    <td>{{ p.type === 'anthropic' ? 'Anthropic 兼容' : 'OpenAI 兼容' }}</td>
                    <td>{{ p.base_url || '默认' }}</td>
                    <td class="dim">—</td>
                    <td>
                      <select @change="changeModel(p.id, ($event.target as HTMLSelectElement).value)" class="input input-inline">
                        <option v-for="m in p.models" :key="m.model_id" :value="m.model_id" :selected="m.model_id === p.selected_model">
                          {{ m.display_name || m.model_id }}
                        </option>
                        <option v-if="!p.models?.length" value="" disabled>无模型</option>
                      </select>
                    </td>
                    <td><span v-if="p.is_active" class="status-badge ok">已启用</span><span v-else class="dim">未启用</span></td>
                    <td style="white-space:nowrap">
                      <button v-if="!p.is_active" class="btn btn-sm" @click="enableProvider(p.id)" style="margin-right:4px">启用</button>
                      <button class="btn btn-sm" @click="startEditProvider(p.id)" style="margin-right:4px">编辑</button>
                      <button class="btn btn-sm" @click="refreshModels(p.id)">获取模型</button>
                    </td>
                  </template>
                </tr>
                <tr v-if="providers.length === 0">
                  <td colspan="7" class="dim" style="text-align:center;padding:24px">暂无供应商配置</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="fetchError" class="edit-card">
            <p class="err" style="padding:12px;text-align:center">{{ fetchError }}</p>
          </div>

          <div v-if="agentError" class="err" style="margin-bottom:12px">{{ agentError }}</div>

          <!-- 权限模式 card -->
          <div class="edit-card">
            <div class="edit-card-title" style="display:flex;justify-content:space-between;align-items:center">
              <span>权限模式</span>
              <button v-if="!agentEditing" class="btn btn-primary btn-sm" @click="agentEditing = true">编辑</button>
              <div v-else style="display:flex;gap:8px">
                <button class="btn btn-primary btn-sm" :disabled="agentSaving" @click="saveAgentSettings">
                  {{ agentSaving ? '保存中…' : '保存' }}
                </button>
                <button class="btn btn-sm" @click="agentEditing = false">取消</button>
              </div>
            </div>
            <div class="block-grid">
              <div class="form-row">
                <label>模式</label>
                <template v-if="agentEditing">
                  <select v-model="agentSettings.permission_mode" class="input">
                    <option value="ask">询问模式 ask（默认）</option>
                    <option value="auto">自动模式 auto</option>
                    <option value="plan">计划模式 plan</option>
                    <option value="readonly">只读模式 readonly</option>
                  </select>
                </template>
                <span v-else class="detail-value">
                  <template v-if="agentSettings.permission_mode === 'ask'">询问模式 ask（默认）</template>
                  <template v-else-if="agentSettings.permission_mode === 'auto'">自动模式 auto</template>
                  <template v-else-if="agentSettings.permission_mode === 'plan'">计划模式 plan</template>
                  <template v-else-if="agentSettings.permission_mode === 'readonly'">只读模式 readonly</template>
                  <template v-else>{{ agentSettings.permission_mode }}</template>
                </span>
              </div>
              <div class="form-row">
                <label>审批超时（秒）</label>
                <input v-if="agentEditing" v-model.number="agentSettings.approval_timeout" class="input" type="number" min="0" />
                <span v-else class="detail-value">{{ agentSettings.approval_timeout }}</span>
              </div>
            </div>
            <div class="mode-desc">
              <template v-if="agentSettings.permission_mode === 'ask'">
                <strong>询问模式 ask</strong> — L3 及以上命令暂停执行，等待人工审批后继续。适合日常运维场景。
              </template>
              <template v-else-if="agentSettings.permission_mode === 'auto'">
                <strong>自动模式 auto</strong> — L4 命令等待审批，其余自动执行并记录审计。适合 CI/CD 流水线。
              </template>
              <template v-else-if="agentSettings.permission_mode === 'plan'">
                <strong>计划模式 plan</strong> — 所有命令只生成执行计划，不实际执行。适合变更评审和演练。
              </template>
              <template v-else-if="agentSettings.permission_mode === 'readonly'">
                <strong>只读模式 readonly</strong> — 只允许 L1 只读操作，其余全部拒绝。适合审计巡检。
              </template>
            </div>
          </div>

          <!-- 风险级别定义 card -->
          <div class="edit-card">
            <div class="edit-card-toolbar" style="cursor:pointer" @click="showRiskLevels = !showRiskLevels">
              <div class="edit-card-title" style="margin-bottom:0;padding-bottom:0;border-bottom:none">风险级别定义</div>
              <span class="dim">{{ showRiskLevels ? '收起 ▲' : '展开 ▼' }}</span>
            </div>
            <table v-if="showRiskLevels" class="table" style="margin-top:12px">
              <thead><tr><th>级别</th><th>名称</th><th>描述</th><th>示例</th></tr></thead>
              <tbody>
                <tr><td><span class="risk-badge l1">L1</span></td><td>读</td><td>只读，无副作用</td><td class="dim">ls, cat, ps, df, ping</td></tr>
                <tr><td><span class="risk-badge l2">L2</span></td><td>写</td><td>可逆写操作，系统可自动恢复</td><td class="dim">cp, chmod, systemctl restart</td></tr>
                <tr><td><span class="risk-badge l3">L3</span></td><td>危险</td><td>删除或停止资源，恢复需额外操作</td><td class="dim">rm, kill, systemctl stop</td></tr>
                <tr><td><span class="risk-badge l4">L4</span></td><td>毁灭</td><td>批量/不可逆，影响超出单个资源</td><td class="dim">rm -rf, dd, mkfs</td></tr>
              </tbody>
            </table>
          </div>

          <!-- 模式×级别矩阵 card -->
          <div class="edit-card">
            <div class="edit-card-toolbar" style="cursor:pointer" @click="showMatrix = !showMatrix">
              <div class="edit-card-title" style="margin-bottom:0;padding-bottom:0;border-bottom:none">模式 × 级别决策矩阵</div>
              <span class="dim">{{ showMatrix ? '收起 ▲' : '展开 ▼' }}</span>
            </div>
            <table v-if="showMatrix" class="table" style="text-align:center;margin-top:12px">
              <thead><tr><th style="text-align:left">级别</th><th>只读</th><th>询问（默认）</th><th>自动</th><th>计划</th></tr></thead>
              <tbody>
                <tr><td style="text-align:left"><span class="risk-badge l1">L1</span> 读</td><td class="ok">✓ 执行</td><td class="ok">✓ 执行</td><td class="ok">✓ 执行</td><td class="plan-cell">📋 计划</td></tr>
                <tr><td style="text-align:left"><span class="risk-badge l2">L2</span> 写</td><td class="no">✗ 拒绝</td><td class="ok">✓ 执行</td><td class="ok">✓ 执行</td><td class="plan-cell">📋 计划</td></tr>
                <tr><td style="text-align:left"><span class="risk-badge l3">L3</span> 危险</td><td class="no">✗ 拒绝</td><td class="wait">⏸ 等审批</td><td class="ok">✓ 执行</td><td class="plan-cell">📋 计划</td></tr>
                <tr><td style="text-align:left"><span class="risk-badge l4">L4</span> 毁灭</td><td class="no">✗ 拒绝</td><td class="wait">⏸ 等审批</td><td class="wait">⏸ 等审批</td><td class="plan-cell">📋 计划</td></tr>
              </tbody>
            </table>
          </div>

          <!-- 自定义规则 card -->
          <div class="edit-card">
            <div class="edit-card-toolbar">
              <div class="edit-card-title" style="margin-bottom:0;padding-bottom:0;border-bottom:none">自定义规则</div>
              <button class="btn btn-primary btn-sm" @click="showAddRule = true" v-if="!showAddRule">+ 添加规则</button>
            </div>
            <div v-if="showAddRule" style="display:flex;gap:8px;align-items:flex-end;margin-bottom:12px;flex-wrap:wrap">
              <div class="form-row" style="flex:2;min-width:140px;margin-bottom:0">
                <label>Pattern</label>
                <input v-model="newRule.pattern" class="input" placeholder="e.g. rm -rf *" />
              </div>
              <div class="form-row" style="flex:1;min-width:80px;margin-bottom:0">
                <label>Level</label>
                <select v-model="newRule.level" class="input">
                  <option value="L1">L1</option>
                  <option value="L2">L2</option>
                  <option value="L3">L3</option>
                  <option value="L4">L4</option>
                </select>
              </div>
              <div class="form-row" style="flex:2;min-width:140px;margin-bottom:0">
                <label>描述</label>
                <input v-model="newRule.description" class="input" placeholder="规则说明" />
              </div>
              <div style="display:flex;gap:4px">
                <button class="btn btn-primary btn-sm" @click="addRule">确认</button>
                <button class="btn btn-sm" @click="showAddRule = false">取消</button>
              </div>
            </div>
            <table class="table">
              <thead><tr><th>#</th><th>Pattern</th><th>Level</th><th>描述</th><th>操作</th></tr></thead>
              <tbody>
                <tr v-for="(r, idx) in customRules" :key="idx">
                  <td class="dim">{{ idx + 1 }}</td>
                  <td style="font-family:'SF Mono',Consolas,monospace;font-size:12px">{{ r.pattern }}</td>
                  <td>{{ r.level }}</td>
                  <td class="dim">{{ r.description || '—' }}</td>
                  <td><button class="btn btn-sm btn-danger" @click="deleteRule(idx)">删除</button></td>
                </tr>
                <tr v-if="customRules.length === 0">
                  <td colspan="5" class="dim" style="text-align:center;padding:24px">暂无自定义规则</td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- 内置规则 card -->
          <div class="edit-card">
            <div class="edit-card-toolbar" style="cursor:pointer" @click="showBuiltinRules = !showBuiltinRules">
              <div class="edit-card-title" style="margin-bottom:0;padding-bottom:0;border-bottom:none">
                内置规则 ({{ builtinRules.length }})
              </div>
              <span class="dim">{{ showBuiltinRules ? '收起 ▲' : '展开 ▼' }}</span>
            </div>
            <table v-if="showBuiltinRules" class="table" style="margin-top:12px">
              <thead><tr><th>Pattern</th><th>Level</th></tr></thead>
              <tbody>
                <tr v-for="(r, idx) in builtinRules" :key="idx">
                  <td style="font-family:'SF Mono',Consolas,monospace;font-size:12px">{{ r.pattern }}</td>
                  <td>{{ r.level }}</td>
                </tr>
                <tr v-if="builtinRules.length === 0">
                  <td colspan="2" class="dim" style="text-align:center;padding:24px">暂无内置规则</td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>



      </div>
      </template><!-- end v-else -->
    </div>


  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { authHeaders } from '../api/auth'
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

onMounted(() => {
  const tab = activeTab.value
  if (tab === 'agent') { loadAgentSettings(); loadProviders() }
})

interface ProviderModel { model_id: string; display_name: string }
interface Provider {
  id: string; name: string; type: string; base_url: string
  api_key: string
  selected_model: string; is_active: boolean
  models: ProviderModel[]
  created_at: string; updated_at: string
}
const providers = ref<Provider[]>([])
const editingProviderId = ref('')
const editForm = ref({ name: '', type: 'anthropic', api_key: '', base_url: '' })
const fetchError = ref('')
let providersLoaded = false

async function loadProviders() {
  if (providersLoaded) return
  providersLoaded = true
  const res = await fetch('/api/v1/providers', { headers: authHeaders() })
  if (!res.ok) return
  providers.value = await res.json()
}

async function saveProvider() {
  const id = editingProviderId.value
  const body: any = { name: editForm.value.name, type: editForm.value.type, base_url: editForm.value.base_url }
  if (editForm.value.api_key) body.api_key = editForm.value.api_key
  const res = await fetch(`/api/v1/providers/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(body),
  })
  if (!res.ok) { alert('保存失败'); return }
  editingProviderId.value = ''
  providersLoaded = false
  loadProviders()
}

async function addProvider() {
  const res = await fetch('/api/v1/providers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ name: '', type: 'anthropic', api_key: '', base_url: '' }),
  })
  if (!res.ok) return
  const p = await res.json()
  providers.value.push(p)
  editingProviderId.value = p.id
  editForm.value = { name: p.name, type: p.type, api_key: '', base_url: p.base_url }
}

async function removeProvider(id: string) {
  await fetch(`/api/v1/providers/${id}`, { method: 'DELETE', headers: authHeaders() })
  providers.value = providers.value.filter(p => p.id !== id)
}

async function enableProvider(id: string) {
  await fetch(`/api/v1/providers/${id}/activate`, { method: 'PUT', headers: authHeaders() })
  providersLoaded = false
  loadProviders()
}

function startEditProvider(id: string) {
  const p = providers.value.find(x => x.id === id)
  if (!p) return
  editingProviderId.value = id
  editForm.value = { name: p.name, type: p.type, api_key: '', base_url: p.base_url }
}

function cancelEdit() { editingProviderId.value = '' }

async function changeModel(providerId: string, model: string) {
  await fetch(`/api/v1/providers/${providerId}/model`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ model }),
  })
  const p = providers.value.find(x => x.id === providerId)
  if (p) p.selected_model = model
}

async function refreshModels(id: string) {
  fetchError.value = ''
  const res = await fetch(`/api/v1/providers/${id}/refresh`, { method: 'POST', headers: authHeaders() })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: '请求失败' }))
    fetchError.value = `获取模型失败: ${err.error || res.statusText}`
    return
  }
  const models = await res.json()
  const p = providers.value.find(x => x.id === id)
  if (p) p.models = models
  fetchError.value = ''
}

// ── Agent / 智能体 ──
const agentSettings = ref({ permission_mode: 'ask', approval_timeout: 300 })
const customRules = ref<{ pattern: string; level: string; description: string }[]>([])
const builtinRules = ref<{ pattern: string; level: string }[]>([])
const showBuiltinRules = ref(false)
const showRiskLevels = ref(false)
const showMatrix = ref(false)
const showAddRule = ref(false)
const newRule = ref({ pattern: '', level: 'L3', description: '' })
const agentSaving = ref(false)
const agentEditing = ref(false)
const agentError = ref('')

async function loadAgentSettings() {
  agentError.value = ''
  try {
    const res = await fetch('/api/v1/settings', { headers: authHeaders() })
    const data = await res.json()
    agentSettings.value = {
      permission_mode: data.permission_mode || 'ask',
      approval_timeout: data.approval_timeout || 300,
    }
    const rulesRes = await fetch('/api/v1/permission/rules', { headers: authHeaders() })
    customRules.value = await rulesRes.json()
    const builtinRes = await fetch('/api/v1/permission/builtin-rules', { headers: authHeaders() })
    builtinRules.value = await builtinRes.json()
  } catch (e: any) {
    agentError.value = e.message
  }
}

async function saveAgentSettings() {
  agentSaving.value = true
  agentError.value = ''
  try {
    await fetch('/api/v1/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify(agentSettings.value),
    })
  } catch (e: any) {
    agentError.value = e.message
  }
  agentSaving.value = false
  if (!agentError.value) agentEditing.value = false
}

async function addRule() {
  agentError.value = ''
  try {
    const res = await fetch('/api/v1/permission/rules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify(newRule.value),
    })
    if (!res.ok) {
      const err = await res.json()
      agentError.value = err.error || 'Failed to add rule'
      return
    }
    newRule.value = { pattern: '', level: 'L3', description: '' }
    showAddRule.value = false
    await loadAgentSettings()
  } catch (e: any) {
    agentError.value = e.message
  }
}

async function deleteRule(idx: number) {
  agentError.value = ''
  try {
    await fetch(`/api/v1/permission/rules/${idx}`, {
      method: 'DELETE',
      headers: authHeaders(),
    })
    await loadAgentSettings()
  } catch (e: any) {
    agentError.value = e.message
  }
}


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

.edit-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 20px 24px;
  box-shadow: var(--card-shadow);
  margin-bottom: 16px;
}

.edit-card-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

.edit-card-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

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

.log-row { cursor: pointer; }
.cmd-cell { font-family: 'SF Mono', Consolas, monospace; font-size: 12px; color: var(--text-sub); }

.status-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 4px; border: 1px solid;
}
.status-badge.ok   { background: rgba(74,222,128,0.12); color: var(--green); border-color: rgba(74,222,128,0.3); }
.status-badge.fail { background: rgba(248,113,113,0.12); color: var(--red);  border-color: rgba(248,113,113,0.3); }

.log-expand td { padding: 0 !important; }
.log-cfg-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 0;
  border-bottom: 1px solid var(--border);
}
.log-cfg-row:last-child { border-bottom: none; }
.log-cfg-lbl { font-size: 12px; color: var(--muted); }
.log-cfg-mod { font-size: 12px; font-family: 'SF Mono', Consolas, monospace; color: var(--text); }
.log-cfg-divider { border: none; border-top: 1px solid var(--border); margin: 8px 0 4px; }
.log-cfg-select { width: 140px; }
.log-cfg-badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid transparent;
}
.log-cfg-badge--inherit { background: rgba(124,133,162,0.1); color: var(--muted); border-color: rgba(124,133,162,0.2); }
.log-cfg-badge--debug   { background: rgba(74,222,128,0.1);  color: var(--green); border-color: rgba(74,222,128,0.25); }
.log-cfg-badge--info    { background: rgba(99,102,241,0.1);  color: var(--primary); border-color: rgba(99,102,241,0.25); }
.log-cfg-badge--warn    { background: rgba(234,179,8,0.1);   color: var(--yellow); border-color: rgba(234,179,8,0.25); }
.log-cfg-badge--error   { background: rgba(248,113,113,0.1); color: var(--red);  border-color: rgba(248,113,113,0.25); }
.log-output { padding: 12px 16px; display: flex; flex-direction: column; gap: 8px; }
.output-block { display: flex; flex-direction: column; gap: 4px; }
.output-label { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.07em; color: var(--muted); }
.err-label { color: var(--red); }
.output-pre {
  margin: 0; white-space: pre-wrap; word-break: break-all;
  background: var(--panel); border: 1px solid var(--border); border-radius: 6px;
  padding: 10px 12px; font-size: 12px; color: var(--text-sub);
  max-height: 240px; overflow-y: auto;
}
.err-pre { color: var(--red); }

.nav-section-label {
  font-size: 10px;
  font-weight: 700;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.1em;
  padding: 12px 16px 4px;
}

.block-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.input-inline { padding: 4px 8px !important; font-size: 12px !important; width: 100%; }

.model-option { padding: 8px 12px; cursor: pointer; border-radius: 6px; display: flex; justify-content: space-between; align-items: center; color: var(--text-sub); font-size: 13px; }
.model-option:hover { background: var(--row-hover); }
.model-option.active { background: var(--row-hover); color: var(--primary); font-weight: 500; }
.check { color: var(--green); }
.mode-desc { margin-top: 12px; font-size: 12px; color: #60a5fa; background: rgba(96, 165, 250, 0.08); border: 1px solid rgba(96, 165, 250, 0.25); border-radius: 4px; padding: 7px 10px; line-height: 1.6; }
.risk-badge { display: inline-block; padding: 1px 6px; border-radius: 3px; font-size: 11px; font-weight: 700; }
.risk-badge.l1 { background: rgba(74, 222, 128, 0.15); color: #4ade80; }
.risk-badge.l2 { background: rgba(96, 165, 250, 0.15); color: #60a5fa; }
.risk-badge.l3 { background: rgba(251, 146, 60, 0.15); color: #fb923c; }
.risk-badge.l4 { background: rgba(248, 113, 113, 0.15); color: #f87171; }
.ok { color: #4ade80; }
.wait { color: #fb923c; }
.no { color: #f87171; }
.plan-cell { color: #a78bfa; }


.theme-cards { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 8px; }
.theme-card { cursor: pointer; border: 2px solid var(--border); border-radius: 8px; overflow: hidden; width: 100px; }
.theme-card.selected { border-color: var(--primary); }
.theme-preview { display: flex; align-items: center; gap: 6px; padding: 8px 10px; font-family: 'SF Mono', monospace; font-size: 11px; }
.theme-name { font-size: 11px; color: var(--text-sub); padding: 5px 8px; text-align: center; background: var(--card-bg); }
.density-btns { display: flex; gap: 8px; margin-top: 8px; }
.density-btn { padding: 5px 16px; border: 1px solid var(--border); border-radius: 4px; background: transparent; color: var(--text-sub); cursor: pointer; font-size: 12px; }
.density-btn.selected { border-color: var(--primary); color: var(--primary); background: var(--row-hover); }
.field-group { margin-bottom: 16px; }
.field-group:last-child { margin-bottom: 0; }
.field-label { font-size: 11px; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.07em; margin-bottom: 4px; }
.ct-display-row { display: flex; align-items: center; gap: 12px; padding: 8px 0; border-bottom: 1px solid var(--border); }
.ct-display-row:last-child { border-bottom: none; }
.ct-display-label { font-size: 12px; color: var(--muted); width: 72px; flex-shrink: 0; }
.ct-display-chip { display: inline-block; font-size: 12px; font-weight: 500; padding: 3px 10px; border-radius: 12px; border: 1px solid var(--border); color: var(--text-sub); background: var(--panel); }
.datasources-subtabs { display: flex; gap: 0; border-bottom: 1px solid var(--border); padding: 0 20px; flex-shrink: 0; background: var(--surface); }
.datasources-subtab { padding: 10px 16px; font-size: 13px; color: var(--text-sub); cursor: pointer; border-bottom: 2px solid transparent; margin-bottom: -1px; }
.datasources-subtab.active { color: var(--primary); border-bottom-color: var(--primary); font-weight: 500; }
</style>
