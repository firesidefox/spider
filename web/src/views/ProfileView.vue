<template>
  <div class="page-content">
    <div class="page-header"><h2>个人设置</h2></div>

    <div class="profile-tabs">
      <button class="tab-btn" :class="{ active: activeTab === 'info' }" @click="activeTab = 'info'">基本信息</button>
      <button class="tab-btn" :class="{ active: activeTab === 'tokens' }" @click="activeTab = 'tokens'; loadTokens()">访问令牌</button>
      <button class="tab-btn" :class="{ active: activeTab === 'logs' }" @click="activeTab = 'logs'; loadLogs()">操作日志</button>
    </div>

    <!-- Tab 1: 基本信息 -->
    <div v-if="activeTab === 'info'">
      <div class="settings-card">
        <h3>账号信息</h3>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">用户名</span>
            <span class="info-value">{{ currentUser?.username }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">角色</span>
            <span class="role-badge" :class="currentUser?.role">{{ roleLabel }}</span>
          </div>
          <div class="info-item" v-if="currentUser?.created_at">
            <span class="info-label">注册时间</span>
            <span class="info-value dim">{{ new Date(currentUser.created_at).toLocaleString() }}</span>
          </div>
          <div class="info-item" v-if="currentUser?.last_login">
            <span class="info-label">上次登录</span>
            <span class="info-value dim">{{ new Date(currentUser.last_login).toLocaleString() }}</span>
          </div>
        </div>
      </div>

      <div class="settings-card">
        <h3>修改密码</h3>
        <div class="pwd-form">
          <div class="form-row"><label>旧密码</label><input v-model="pw.old" type="password" class="input" placeholder="当前密码" /></div>
          <div class="form-row"><label>新密码</label><input v-model="pw.new1" type="password" class="input" placeholder="至少 6 位" /></div>
          <div class="form-row"><label>确认新密码</label><input v-model="pw.new2" type="password" class="input" placeholder="再次输入新密码" /></div>
        </div>
        <div v-if="pwError" class="err" style="margin-bottom:12px">{{ pwError }}</div>
        <div v-if="pwSuccess" class="ok" style="margin-bottom:12px">{{ pwSuccess }}</div>
        <button class="btn btn-primary" @click="handleChangePassword" :disabled="pwLoading">
          {{ pwLoading ? '保存中…' : '保存密码' }}
        </button>
      </div>
    </div>

    <!-- Tab 2: 访问令牌 -->
    <div v-if="activeTab === 'tokens'">
      <div class="settings-card">
        <div class="card-toolbar">
          <h3 style="margin:0;border:none;padding:0">访问令牌</h3>
          <button class="btn btn-primary btn-sm" @click="showCreate = true">+ 新建 Token</button>
        </div>
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
              <td><button class="btn btn-sm btn-danger" @click="handleDelete(t.id)">撤销</button></td>
            </tr>
            <tr v-if="tokens.length === 0">
              <td colspan="5" class="dim" style="text-align:center;padding:32px">暂无 Token</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Tab 3: 操作日志 -->
    <div v-if="activeTab === 'logs'">
      <div class="settings-card">
        <h3>我的操作日志</h3>
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

    <!-- 明文展示弹窗（仅一次） -->
    <div v-if="newToken" class="modal-overlay">
      <div class="modal">
        <h3>Token 已创建</h3>
        <p class="dim" style="margin-bottom:12px;font-size:13px">请立即复制，此后不再显示。</p>
        <div class="token-display">
          <code class="code token-code">{{ newToken }}</code>
          <button class="btn btn-sm" @click="copyToken">复制</button>
        </div>
        <div class="modal-footer">
          <button class="btn btn-primary" @click="newToken = ''">我已复制</button>
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

const { currentUser } = useAuth()

const roleLabel = computed(() => {
  const map: Record<string, string> = { admin: '管理员', operator: '操作员', viewer: '只读' }
  return map[currentUser.value?.role ?? ''] ?? currentUser.value?.role ?? '—'
})

const activeTab = ref<'info' | 'tokens' | 'logs'>('info')

// ── 基本信息 / 改密码 ──────────────────────────────────
const pw = ref({ old: '', new1: '', new2: '' })
const pwError = ref('')
const pwSuccess = ref('')
const pwLoading = ref(false)

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
    setTimeout(() => { pwSuccess.value = '' }, 3000)
  } catch { pwError.value = '修改失败' }
  finally { pwLoading.value = false }
}

// ── 访问令牌 ───────────────────────────────────────────
const tokens = ref<TokenInfo[]>([])
const showCreate = ref(false)
const newToken = ref('')
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

async function handleDelete(id: string) {
  if (!confirm('确认撤销此 Token？撤销后立即失效。')) return
  await deleteToken(id)
  tokens.value = await listTokens()
}

function copyToken() { navigator.clipboard.writeText(newToken.value) }

function isExpired(expiresAt: string) { return new Date(expiresAt) < new Date() }

// ── 操作日志 ───────────────────────────────────────────
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
</script>

<style scoped>
.profile-tabs {
  display: flex;
  border-bottom: 1px solid var(--border);
  margin-bottom: 24px;
}
.tab-btn {
  padding: 8px 22px;
  border: none;
  background: none;
  cursor: pointer;
  color: var(--text-sub);
  font-size: 14px;
  font-weight: 500;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  transition: color 0.15s;
}
.tab-btn:hover { color: var(--text); }
.tab-btn.active { color: var(--primary); border-bottom-color: var(--primary); }

/* 账号信息网格 */
.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.info-item { display: flex; flex-direction: column; gap: 4px; }
.info-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: var(--muted);
}
.info-value { font-size: 14px; color: var(--text); }

/* 角色徽章 */
.role-badge {
  display: inline-block;
  font-size: 12px;
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 4px;
  border: 1px solid;
}
.role-badge.admin {
  background: rgba(99,102,241,0.12);
  color: var(--primary);
  border-color: rgba(99,102,241,0.3);
}
.role-badge.operator {
  background: rgba(74,222,128,0.12);
  color: var(--green);
  border-color: rgba(74,222,128,0.3);
}
.role-badge.viewer {
  background: rgba(167,139,250,0.1);
  color: var(--purple);
  border-color: rgba(167,139,250,0.25);
}

/* 密码表单 */
.pwd-form { max-width: 400px; }

/* Token 卡片工具栏 */
.card-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

/* 日志行 */
.log-row { cursor: pointer; }
.cmd-cell { font-family: 'SF Mono', Consolas, monospace; font-size: 12px; color: var(--text-sub); }

.status-badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid;
}
.status-badge.ok {
  background: rgba(74,222,128,0.12);
  color: var(--green);
  border-color: rgba(74,222,128,0.3);
}
.status-badge.fail {
  background: rgba(248,113,113,0.12);
  color: var(--red);
  border-color: rgba(248,113,113,0.3);
}

/* 日志展开区 */
.log-expand td { padding: 0 !important; }
.log-output {
  padding: 12px 16px;
  background: var(--surface);
  border-top: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.output-block { display: flex; flex-direction: column; gap: 4px; }
.output-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: var(--muted);
}
.err-label { color: var(--red); }
.output-pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 10px 12px;
  font-size: 12px;
  color: var(--text-sub);
  max-height: 240px;
  overflow-y: auto;
}
.err-pre { color: var(--red); }

/* Token 明文展示 */
.token-display {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 16px;
}
.token-code {
  flex: 1;
  word-break: break-all;
  font-size: 12px;
  color: var(--green);
}
</style>
