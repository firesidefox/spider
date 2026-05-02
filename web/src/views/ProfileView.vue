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
        <div class="nav-row" :class="{ selected: activeTab === 'tokens' }" @click="activeTab = 'tokens'; loadTokens()">
          <span class="nav-icon">🔑</span><span class="nav-label">访问令牌</span>
        </div>
        <div class="nav-row" :class="{ selected: activeTab === 'ssh-keys' }" @click="activeTab = 'ssh-keys'; loadSSHKeys()">
          <span class="nav-icon">🔐</span><span class="nav-label">SSH Keys</span>
        </div>
        <div class="nav-row" :class="{ selected: activeTab === 'logs' }" @click="activeTab = 'logs'; loadLogs()">
          <span class="nav-icon">📋</span><span class="nav-label">操作日志</span>
        </div>
        <template v-if="isAdmin">
          <div class="nav-section-label">管理</div>
          <div class="nav-row" :class="{ selected: activeTab === 'users' }" @click="activeTab = 'users'">
            <span class="nav-icon">👥</span><span class="nav-label">用户管理</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'install' }" @click="activeTab = 'install'">
            <span class="nav-icon">📦</span><span class="nav-label">安装</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'skills' }" @click="activeTab = 'skills'">
            <span class="nav-icon">🧩</span><span class="nav-label">Skills</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'llm' }" @click="activeTab = 'llm'; loadSettings()">
            <span class="nav-icon">🤖</span><span class="nav-label">模型供应商</span>
          </div>
          <div class="nav-row" :class="{ selected: activeTab === 'settings' }" @click="activeTab = 'settings'; loadSettings()">
            <span class="nav-icon">⚙️</span><span class="nav-label">系统设置</span>
          </div>
        </template>
      </nav>
    </aside>
    <div class="profile-detail">
      <template v-if="activeTab === 'users'">
        <UsersPanel />
      </template>
      <template v-else-if="activeTab === 'install'">
        <InstallPanel @switch-tab="activeTab = $event as any" />
      </template>
      <template v-else-if="activeTab === 'skills'">
        <SkillsPanel />
      </template>
      <template v-else>
        <div class="detail-topbar">
          <span class="detail-title">{{ tabTitle }}</span>
          <button v-if="activeTab === 'info'" class="btn btn-sm" @click="showPwModal = true">修改密码</button>
          <button v-if="activeTab === 'tokens'" class="btn btn-primary btn-sm" @click="showCreate = true">+ 新建 Token</button>
          <button v-if="activeTab === 'ssh-keys'" class="btn btn-primary btn-sm" @click="showAddKey = true">+ 添加 Key</button>
          <template v-if="activeTab === 'llm'">
            <div v-if="settingsEditing" style="display:flex;gap:8px">
              <button class="btn btn-primary btn-sm" @click="saveSettings">保存</button>
              <button class="btn btn-sm" @click="cancelSettings">取消</button>
            </div>
            <button v-else class="btn btn-primary btn-sm" @click="addProvider">+ 添加供应商</button>
          </template>
          <template v-if="activeTab === 'settings'">
            <div v-if="settingsEditing" style="display:flex;gap:8px">
              <button class="btn btn-primary btn-sm" @click="saveSettings">保存</button>
              <button class="btn btn-sm" @click="cancelSettings">取消</button>
            </div>
            <button v-else class="btn btn-sm" @click="settingsEditing = true">编辑</button>
          </template>
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
        </template>

        <template v-if="activeTab === 'tokens'">
          <div class="edit-card">
            <p class="dim" style="margin-bottom:16px;font-size:13px">Token 可用于 MCP 工具或 API 调用，权限与账号角色一致。</p>
            <table class="table">
              <thead><tr><th>名称</th><th>创建时间</th><th>过期时间</th><th>最后使用</th><th>操作</th></tr></thead>
              <tbody>
                <tr v-for="t in tokens" :key="t.id">
                  <td style="font-weight:500;color:var(--text)">{{ t.name }}</td>
                  <td class="dim">{{ new Date(t.created_at).toLocaleString() }}</td>
                  <td>
                    <span v-if="t.expires_at" :class="isExpired(t.expires_at) ? 'err' : 'dim'">
                      {{ new Date(t.expires_at).toLocaleString() }}
                    </span>
                    <span v-else class="dim">永不过期</span>
                  </td>
                  <td class="dim">{{ t.last_used ? new Date(t.last_used).toLocaleString() : '从未' }}</td>
                  <td>
                    <button class="btn btn-sm" @click="handleCopyToken(t.id)">{{ copiedTokenId === t.id ? '已复制 ✓' : '复制' }}</button>
                    <button class="btn btn-sm btn-danger" style="margin-left:6px" @click="handleDelete(t.id)">撤销</button>
                  </td>
                </tr>
                <tr v-if="tokens.length === 0">
                  <td colspan="5" class="dim" style="text-align:center;padding:32px">暂无 Token</td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>

        <template v-if="activeTab === 'ssh-keys'">
          <div class="edit-card">
            <p class="dim" style="margin-bottom:16px;font-size:13px">管理 SSH 私钥，可在添加主机时引用。</p>
            <table class="table">
              <thead><tr><th>名称</th><th>指纹</th><th>创建时间</th><th>操作</th></tr></thead>
              <tbody>
                <tr v-for="k in sshKeys" :key="k.id">
                  <td style="font-weight:500;color:var(--text)">{{ k.name }}</td>
                  <td class="dim" style="font-family:'SF Mono',Consolas,monospace;font-size:12px">{{ k.fingerprint.slice(0, 24) }}…</td>
                  <td class="dim">{{ new Date(k.created_at).toLocaleString() }}</td>
                  <td><button class="btn btn-sm btn-danger" @click="handleDeleteKey(k.id)">删除</button></td>
                </tr>
                <tr v-if="sshKeys.length === 0">
                  <td colspan="4" class="dim" style="text-align:center;padding:32px">暂无 SSH Key</td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>

        <template v-if="activeTab === 'logs'">
          <div class="edit-card">
            <div class="edit-card-title">我的操作日志</div>
            <table class="table">
              <thead><tr><th>主机</th><th>命令</th><th>状态</th><th>耗时</th><th>时间</th></tr></thead>
              <tbody>
                <template v-for="log in logs" :key="log.id">
                  <tr class="log-row" @click="toggleLog(log.id)">
                    <td style="font-weight:500;color:var(--text)">{{ log.host_name || '—' }}</td>
                    <td class="cmd-cell">{{ log.command.length > 48 ? log.command.slice(0, 48) + '…' : log.command }}</td>
                    <td>
                      <span class="status-badge" :class="log.exit_code === 0 ? 'ok' : 'fail'">
                        {{ log.exit_code === 0 ? '✓ 成功' : `✗ ${log.exit_code}` }}
                      </span>
                    </td>
                    <td class="dim">{{ log.duration_ms != null ? log.duration_ms + 'ms' : '—' }}</td>
                    <td class="dim">{{ new Date(log.created_at).toLocaleString() }}</td>
                  </tr>
                  <tr v-if="expandedLog === log.id" class="log-expand">
                    <td colspan="5">
                      <div class="log-output">
                        <div v-if="log.stdout" class="output-block">
                          <div class="output-label">stdout</div>
                          <pre class="code output-pre">{{ log.stdout }}</pre>
                        </div>
                        <div v-if="log.stderr" class="output-block">
                          <div class="output-label err-label">stderr</div>
                          <pre class="code output-pre err-pre">{{ log.stderr }}</pre>
                        </div>
                        <div v-if="!log.stdout && !log.stderr" class="dim" style="padding:8px 0">无输出</div>
                      </div>
                    </td>
                  </tr>
                </template>
                <tr v-if="logs.length === 0">
                  <td colspan="5" class="dim" style="text-align:center;padding:32px">暂无操作日志</td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>

        <!-- Tab: 模型供应商 -->
        <template v-if="activeTab === 'llm'">
          <div class="edit-card">
            <p class="dim" style="margin-bottom:16px;font-size:13px">配置 AI 模型供应商，用于智能运维对话和工具调用。</p>
            <table class="table">
              <thead><tr><th style="width:30px"></th><th>名称</th><th>类型</th><th>API Key</th><th>请求地址</th><th>操作</th></tr></thead>
              <tbody>
                <tr v-for="(p, i) in settings.model.providers" :key="p.id">
                  <td><input type="radio" :value="p.id" v-model="settings.model.active_provider" @change="settingsEditing = true" style="accent-color:var(--primary)" /></td>
                  <td><input v-model="p.name" class="input input-inline" placeholder="供应商名称" @input="settingsEditing = true" /></td>
                  <td>
                    <select v-model="p.type" class="input input-inline" @change="settingsEditing = true">
                      <option value="claude">Claude</option>
                      <option value="openai">OpenAI</option>
                    </select>
                  </td>
                  <td><input v-model="p.api_key" class="input input-inline" placeholder="API Key" @input="settingsEditing = true" /></td>
                  <td><input v-model="p.base_url" class="input input-inline" placeholder="留空使用默认地址" @input="settingsEditing = true" /></td>
                  <td style="white-space:nowrap">
                    <button class="btn btn-sm" @click="fetchModels(p.id)" style="margin-right:4px">获取模型</button>
                    <button class="btn btn-sm btn-danger" @click="removeProvider(i); settingsEditing = true">删除</button>
                  </td>
                </tr>
                <tr v-if="settings.model.providers.length === 0">
                  <td colspan="6" class="dim" style="text-align:center;padding:24px">暂无供应商配置</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="settings.model.active_provider && providerModels[settings.model.active_provider]?.length" class="edit-card">
            <div class="edit-card-title">选择模型</div>
            <div v-for="m in providerModels[settings.model.active_provider]" :key="m.id" class="model-option"
                 :class="{ active: settings.model.active_model === m.id }"
                 @click="settings.model.active_model = m.id; settingsEditing = true">
              <span>{{ m.display_name || m.id }}</span>
              <span v-if="settings.model.active_model === m.id" class="check">✓</span>
            </div>
          </div>
        </template>

        <!-- Tab: 系统设置 -->
        <template v-if="activeTab === 'settings'">
          <!-- 只读视图 -->
          <template v-if="!settingsEditing">
            <div class="detail-grid">
              <div class="detail-field">
                <div class="detail-label">监听地址</div>
                <div class="detail-value">{{ settings.sse_addr || '—' }}</div>
              </div>
              <div class="detail-field">
                <div class="detail-label">Base URL</div>
                <div class="detail-value">{{ settings.sse_base_url || '—' }}</div>
              </div>
              <div class="detail-field">
                <div class="detail-label">命令超时（秒）</div>
                <div class="detail-value">{{ settings.ssh_default_timeout_seconds }}</div>
              </div>
              <div class="detail-field">
                <div class="detail-label">连接池 TTL（秒）</div>
                <div class="detail-value">{{ settings.ssh_pool_ttl_seconds }}</div>
              </div>
              <div class="detail-field">
                <div class="detail-label">最大连接数</div>
                <div class="detail-value">{{ settings.ssh_max_pool_size }}</div>
              </div>
            </div>
          </template>
          <!-- 编辑视图 -->
          <template v-else>
            <div class="edit-card">
              <div class="edit-card-title">MCP Server</div>
              <div class="block-grid">
                <div class="form-row"><label>监听地址</label><input v-model="settings.sse_addr" class="input" placeholder=":8000" /></div>
                <div class="form-row"><label>Base URL</label><input v-model="settings.sse_base_url" class="input" placeholder="http://localhost:8000" /></div>
              </div>
            </div>
            <div class="edit-card">
              <div class="edit-card-title">SSH 默认配置</div>
              <div class="block-grid">
                <div class="form-row"><label>命令超时（秒）</label><input v-model.number="settings.ssh_default_timeout_seconds" class="input" type="number" /></div>
                <div class="form-row"><label>连接池 TTL（秒）</label><input v-model.number="settings.ssh_pool_ttl_seconds" class="input" type="number" /></div>
                <div class="form-row"><label>最大连接数</label><input v-model.number="settings.ssh_max_pool_size" class="input" type="number" /></div>
              </div>
            </div>
            <div v-if="settingsError" class="err" style="margin-top:4px">{{ settingsError }}</div>
          </template>
        </template>

      </div>
      </template><!-- end v-else -->
    </div>

    <!-- 新建 Token 弹窗 -->
    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h3>新建 API Token</h3>
        <div class="form-row"><label>名称</label><input v-model="form.name" class="input" placeholder="my-token" /></div>
        <div class="form-row">
          <label>过期时间（可选）</label>
          <input v-model="form.expiresAt" type="datetime-local" class="input" />
        </div>
        <div v-if="formError" class="err" style="margin-bottom:12px">{{ formError }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showCreate = false">取消</button>
          <button class="btn btn-primary" @click="handleCreate">创建</button>
        </div>
      </div>
    </div>

    <!-- 修改密码弹窗 -->
    <div v-if="showPwModal" class="modal-overlay" @click.self="showPwModal = false">
      <div class="modal">
        <h3>修改密码</h3>
        <div class="form-row"><label>旧密码</label><input v-model="pw.old" type="password" class="input" placeholder="当前密码" /></div>
        <div class="form-row"><label>新密码</label><input v-model="pw.new1" type="password" class="input" placeholder="至少 6 位" /></div>
        <div class="form-row"><label>确认新密码</label><input v-model="pw.new2" type="password" class="input" placeholder="再次输入新密码" /></div>
        <div v-if="pwError" class="err" style="margin-bottom:10px">{{ pwError }}</div>
        <div v-if="pwSuccess" class="ok" style="margin-bottom:10px">{{ pwSuccess }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showPwModal = false; pw = { old: '', new1: '', new2: '' }; pwError = ''; pwSuccess = ''">取消</button>
          <button class="btn btn-primary" @click="handleChangePassword" :disabled="pwLoading">{{ pwLoading ? '保存中…' : '保存密码' }}</button>
        </div>
      </div>
    </div>

    <!-- Token 明文展示弹窗 -->
    <div v-if="newToken" class="modal-overlay">
      <div class="modal">
        <h3>Token 已创建</h3>
        <p class="dim" style="margin-bottom:12px;font-size:13px">请立即复制，此后不再显示。</p>
        <div class="token-display">
          <code class="code token-code">{{ newToken }}</code>
          <button class="btn btn-sm" :class="{ 'btn-copied': copied }" @click="copyToken">{{ copied ? '✓ 已复制' : '复制' }}</button>
        </div>
        <div class="modal-footer">
          <button class="btn btn-primary" @click="newToken = ''; copied = false">我已复制</button>
        </div>
      </div>
    </div>

    <!-- 添加 SSH Key 弹窗 -->
    <div v-if="showAddKey" class="modal-overlay" @click.self="showAddKey = false">
      <div class="modal">
        <h3>添加 SSH Key</h3>
        <div class="form-row"><label>名称</label><input v-model="keyForm.name" class="input" placeholder="prod-key" /></div>
        <div class="form-row">
          <label>私钥内容</label>
          <textarea v-model="keyForm.privateKey" class="input" rows="5" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" />
        </div>
        <div class="form-row"><label>Passphrase（可选）</label><input v-model="keyForm.passphrase" type="password" class="input" /></div>
        <div v-if="keyFormError" class="err" style="margin-bottom:12px">{{ keyFormError }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showAddKey = false">取消</button>
          <button class="btn btn-primary" @click="handleAddKey">添加</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuth } from '../composables/useAuth'
import { authHeaders } from '../api/auth'
import { listTokens, createToken, deleteToken } from '../api/tokens'
import type { TokenInfo } from '../api/tokens'
import { listSSHKeys, addSSHKey, deleteSSHKey } from '../api/ssh-keys'
import type { SafeSSHKey } from '../api/ssh-keys'
import UsersPanel from './UsersPanel.vue'
import InstallPanel from './InstallPanel.vue'
import SkillsPanel from './SkillsPanel.vue'

const { currentUser, isAdmin } = useAuth()

const roleLabel = computed(() => {
  const map: Record<string, string> = { admin: '管理员', operator: '操作员', viewer: '只读' }
  return map[currentUser.value?.role ?? ''] ?? currentUser.value?.role ?? '—'
})

const activeTab = ref<'info' | 'tokens' | 'ssh-keys' | 'logs' | 'users' | 'install' | 'skills' | 'llm' | 'settings'>('info')
const tabTitle = computed(() => ({
  info: '基本信息', tokens: '访问令牌', 'ssh-keys': 'SSH Keys', logs: '操作日志',
  users: '用户管理', install: '安装', llm: '模型供应商', settings: '系统设置',
}[activeTab.value]))

const pw = ref({ old: '', new1: '', new2: '' })
const pwError = ref('')
const pwSuccess = ref('')
const pwLoading = ref(false)
const showPwModal = ref(false)

async function handleChangePassword() {
  pwError.value = ''
  pwSuccess.value = ''
  if (!pw.value.old) { pwError.value = '请输入旧密码'; return }
  if (pw.value.new1.length < 6) { pwError.value = '新密码至少 6 位'; return }
  if (pw.value.new1 !== pw.value.new2) { pwError.value = '两次新密码不一致'; return }
  pwLoading.value = true
  try {
    const res = await fetch('/api/v1/me/password', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ old_password: pw.value.old, new_password: pw.value.new1 }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      pwError.value = res.status === 403 ? '旧密码错误' : (data.error || '修改失败')
      return
    }
    pw.value = { old: '', new1: '', new2: '' }
    pwSuccess.value = '密码已修改'
    setTimeout(() => { pwSuccess.value = ''; showPwModal.value = false }, 1500)
  } catch { pwError.value = '修改失败' }
  finally { pwLoading.value = false }
}

const tokens = ref<TokenInfo[]>([])
const showCreate = ref(false)
const newToken = ref('')
const copied = ref(false)
const copiedTokenId = ref('')
const formError = ref('')
const form = ref({ name: '', expiresAt: '' })
let tokensLoaded = false

async function loadTokens() {
  if (tokensLoaded) return
  tokensLoaded = true
  tokens.value = await listTokens()
}

onMounted(() => { loadTokens() })

async function handleCreate() {
  formError.value = ''
  if (!form.value.name.trim()) { formError.value = '请输入名称'; return }
  try {
    const res = await createToken(form.value.name, form.value.expiresAt || undefined)
    newToken.value = res.token
    showCreate.value = false
    form.value = { name: '', expiresAt: '' }
    tokensLoaded = false
    tokens.value = await listTokens()
    tokensLoaded = true
  } catch (e: any) { formError.value = e.message }
}

async function handleCopyToken(id: string) {
  await navigator.clipboard.writeText(id)
  copiedTokenId.value = id
  setTimeout(() => { copiedTokenId.value = '' }, 2000)
}

async function handleDelete(id: string) {
  if (!confirm('确认撤销此 Token？撤销后立即失效。')) return
  await deleteToken(id)
  tokens.value = await listTokens()
}

async function copyToken() {
  try {
    await navigator.clipboard.writeText(newToken.value)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch {
    // clipboard 不可用时（HTTP 环境/权限拒绝），静默失败，用户可手动复制
  }
}
function isExpired(expiresAt: string) { return new Date(expiresAt) < new Date() }

// ── SSH Keys ──
const sshKeys = ref<SafeSSHKey[]>([])
const showAddKey = ref(false)
const keyForm = ref({ name: '', privateKey: '', passphrase: '' })
const keyFormError = ref('')
let sshKeysLoaded = false

async function loadSSHKeys() {
  if (sshKeysLoaded) return
  sshKeysLoaded = true
  sshKeys.value = await listSSHKeys()
}

async function handleAddKey() {
  keyFormError.value = ''
  if (!keyForm.value.name.trim()) { keyFormError.value = '请输入名称'; return }
  if (!keyForm.value.privateKey.trim()) { keyFormError.value = '请输入私钥内容'; return }
  try {
    await addSSHKey(keyForm.value.name, keyForm.value.privateKey, keyForm.value.passphrase || undefined)
    showAddKey.value = false
    keyForm.value = { name: '', privateKey: '', passphrase: '' }
    sshKeysLoaded = false
    sshKeys.value = await listSSHKeys()
    sshKeysLoaded = true
  } catch (e: any) { keyFormError.value = e.message }
}

async function handleDeleteKey(id: string) {
  if (!confirm('确认删除此 SSH Key？')) return
  try {
    await deleteSSHKey(id)
    sshKeys.value = await listSSHKeys()
  } catch (e: any) { alert(e.message) }
}

interface LogEntry {
  id: string; host_name: string; command: string; exit_code: number
  duration_ms: number | null; created_at: string; stdout: string; stderr: string
}
const logs = ref<LogEntry[]>([])
const expandedLog = ref<string | null>(null)
let logsLoaded = false

async function loadLogs() {
  if (logsLoaded) return
  logsLoaded = true
  try {
    const res = await fetch('/api/v1/logs?triggered_by=me&limit=50', { headers: authHeaders() })
    if (!res.ok) throw new Error()
    logs.value = await res.json()
  } catch {}
}

function toggleLog(id: string) {
  expandedLog.value = expandedLog.value === id ? null : id
}

interface Provider {
  id: string; name: string; type: string; api_key: string; base_url: string
}
interface Settings {
  sse_addr: string; sse_base_url: string
  ssh_default_timeout_seconds: number; ssh_pool_ttl_seconds: number; ssh_max_pool_size: number
  model: {
    providers: Provider[]
    active_provider: string
    active_model: string
  }
}
const settings = ref<Settings>({
  sse_addr: '', sse_base_url: '',
  ssh_default_timeout_seconds: 30, ssh_pool_ttl_seconds: 300, ssh_max_pool_size: 50,
  model: { providers: [], active_provider: '', active_model: '' },
})
const settingsEditing = ref(false)
const settingsError = ref('')
let settingsLoaded = false

async function loadSettings() {
  if (settingsLoaded) return
  settingsLoaded = true
  const res = await fetch('/api/v1/settings', { headers: authHeaders() })
  if (!res.ok) return
  const data = await res.json()
  if (!data.model) data.model = { providers: [], active_provider: '', active_model: '' }
  if (!data.model.providers) data.model.providers = []
  settings.value = data
}

async function saveSettings() {
  settingsError.value = ''
  const res = await fetch('/api/v1/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(settings.value),
  })
  if (res.ok) { settingsEditing.value = false }
  else settingsError.value = (await res.json()).error
}

function addProvider() {
  settings.value.model.providers.push({ id: '', name: '', type: 'claude', api_key: '', base_url: '' })
  settingsEditing.value = true
}
function removeProvider(idx: number) {
  const p = settings.value.model.providers[idx]
  if (p.id === settings.value.model.active_provider) {
    settings.value.model.active_provider = ''
    settings.value.model.active_model = ''
  }
  settings.value.model.providers.splice(idx, 1)
}

const providerModels = ref<Record<string, {id: string, display_name: string}[]>>({})

async function fetchModels(providerId: string) {
  const res = await fetch(`/api/v1/providers/${providerId}/models`, { headers: authHeaders() })
  if (!res.ok) return
  providerModels.value[providerId] = await res.json()
}

function cancelSettings() {
  settingsEditing.value = false
  settingsLoaded = false
  loadSettings()
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

.token-display {
  display: flex; align-items: center; gap: 8px;
  background: var(--panel); border: 1px solid var(--border);
  border-radius: 8px; padding: 10px 12px; margin-bottom: 16px;
}
.token-code { flex: 1; word-break: break-all; font-size: 12px; color: var(--green); }
.btn-copied { background: rgba(74,222,128,0.15) !important; color: var(--green) !important; border-color: rgba(74,222,128,0.4) !important; }
.input-inline { padding: 4px 8px !important; font-size: 12px !important; width: 100%; }

.model-option { padding: 8px 12px; cursor: pointer; border-radius: 6px; display: flex; justify-content: space-between; align-items: center; color: var(--text-sub); font-size: 13px; }
.model-option:hover { background: var(--row-hover); }
.model-option.active { background: var(--row-hover); color: var(--primary); font-weight: 500; }
.check { color: var(--green); }
</style>
