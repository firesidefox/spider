<template>
  <div class="page-content">
    <div class="page-header"><h2>个人中心</h2></div>
    <div class="profile-tabs">
      <button class="tab-btn" :class="{ active: activeTab === 'info' }" @click="activeTab = 'info'">基本信息</button>
      <button class="tab-btn" :class="{ active: activeTab === 'tokens' }" @click="activeTab = 'tokens'">访问令牌</button>
      <button class="tab-btn" :class="{ active: activeTab === 'logs' }" @click="activeTab = 'logs'; loadLogs()">日志</button>
    </div>

    <!-- Tab 1: 基本信息 -->
    <div v-if="activeTab === 'info'">
      <div class="form-row"><label>用户名</label><input class="input" :value="currentUser?.username" readonly /></div>
      <div class="form-row"><label>角色</label><input class="input" :value="currentUser?.role" readonly /></div>
      <h3 style="margin:24px 0 16px">修改密码</h3>
      <div class="form-row"><label>旧密码</label><input v-model="pw.old" type="password" class="input" /></div>
      <div class="form-row"><label>新密码</label><input v-model="pw.new1" type="password" class="input" /></div>
      <div class="form-row"><label>确认新密码</label><input v-model="pw.new2" type="password" class="input" /></div>
      <div v-if="pwError" class="err" style="margin-bottom:12px">{{ pwError }}</div>
      <div v-if="pwSuccess" class="dim" style="margin-bottom:12px;color:var(--accent)">{{ pwSuccess }}</div>
      <button class="btn btn-primary" @click="handleChangePassword">保存密码</button>
    </div>

    <!-- Tab 2: 访问令牌 -->
    <div v-if="activeTab === 'tokens'">
      <div class="page-header" style="margin-bottom:16px">
        <span></span>
        <button class="btn btn-primary btn-sm" @click="showCreate = true">+ 新建 Token</button>
      </div>
      <table class="table">
        <thead><tr><th>名称</th><th>创建时间</th><th>过期时间</th><th>最后使用</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="t in tokens" :key="t.id">
            <td>{{ t.name }}</td>
            <td class="dim">{{ new Date(t.created_at).toLocaleString() }}</td>
            <td class="dim">{{ t.expires_at ? new Date(t.expires_at).toLocaleString() : '永不过期' }}</td>
            <td class="dim">{{ t.last_used ? new Date(t.last_used).toLocaleString() : '从未' }}</td>
            <td><button class="btn btn-sm btn-danger" @click="handleDelete(t.id)">撤销</button></td>
          </tr>
          <tr v-if="tokens.length === 0">
            <td colspan="5" class="dim" style="text-align:center;padding:24px">暂无 Token</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Tab 3: 日志 -->
    <div v-if="activeTab === 'logs'">
      <table class="table">
        <thead><tr><th>主机名</th><th>命令</th><th>退出码</th><th>耗时</th><th>时间</th></tr></thead>
        <tbody>
          <template v-for="log in logs" :key="log.id">
            <tr style="cursor:pointer" @click="toggleLog(log.id)">
              <td>{{ log.host_name }}</td>
              <td class="dim">{{ log.command.length > 40 ? log.command.slice(0, 40) + '…' : log.command }}</td>
              <td>{{ log.exit_code === 0 ? '✓' : '✗' }}</td>
              <td class="dim">{{ log.duration_ms != null ? log.duration_ms + 'ms' : '-' }}</td>
              <td class="dim">{{ new Date(log.created_at).toLocaleString() }}</td>
            </tr>
            <tr v-if="expandedLog === log.id">
              <td colspan="5" style="background:var(--bg-sub,#f5f5f5);padding:12px">
                <div v-if="log.stdout"><strong>stdout:</strong><pre class="code" style="margin:4px 0 8px;white-space:pre-wrap">{{ log.stdout }}</pre></div>
                <div v-if="log.stderr"><strong>stderr:</strong><pre class="code" style="margin:4px 0;white-space:pre-wrap;color:#c0392b">{{ log.stderr }}</pre></div>
                <div v-if="!log.stdout && !log.stderr" class="dim">无输出</div>
              </td>
            </tr>
          </template>
          <tr v-if="logs.length === 0">
            <td colspan="5" class="dim" style="text-align:center;padding:24px">暂无日志</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 新建 Token 弹窗 -->
    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h3>新建 API Token</h3>
        <div class="form-row"><label>名称</label><input v-model="form.name" class="input" placeholder="my-token" /></div>
        <div class="form-row"><label>过期时间（可选）</label><input v-model="form.expiresAt" type="datetime-local" class="input" /></div>
        <div v-if="formError" class="err" style="margin-bottom:12px">{{ formError }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showCreate = false">取消</button>
          <button class="btn btn-primary" @click="handleCreate">创建</button>
        </div>
      </div>
    </div>

    <!-- 明文展示弹窗 -->
    <div v-if="newToken" class="modal-overlay">
      <div class="modal">
        <h3>Token 已创建</h3>
        <p class="dim" style="margin-bottom:12px;font-size:13px">请立即复制，此后不再显示。</p>
        <div class="token-display"><code class="code">{{ newToken }}</code><button class="btn btn-sm" @click="copyToken">复制</button></div>
        <div class="modal-footer"><button class="btn btn-primary" @click="newToken = ''">我已复制</button></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuth } from '../composables/useAuth'
import { authHeaders } from '../api/auth'
import { listTokens, createToken, deleteToken } from '../api/tokens'
import type { TokenInfo } from '../api/tokens'

const { currentUser } = useAuth()
const activeTab = ref<'info' | 'tokens' | 'logs'>('info')

// Tab 1: 修改密码
const pw = ref({ old: '', new1: '', new2: '' })
const pwError = ref('')
const pwSuccess = ref('')

async function handleChangePassword() {
  pwError.value = ''
  pwSuccess.value = ''
  if (pw.value.new1 !== pw.value.new2) { pwError.value = '新密码与确认密码不一致'; return }
  try {
    const res = await fetch('/api/v1/me/password', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ old_password: pw.value.old, new_password: pw.value.new1 }),
    })
    if (!res.ok) throw new Error((await res.json()).error)
    pw.value = { old: '', new1: '', new2: '' }
    pwSuccess.value = '密码已修改'
  } catch (e: any) { pwError.value = e.message }
}

// Tab 2: 访问令牌
const tokens = ref<TokenInfo[]>([])
const showCreate = ref(false)
const newToken = ref('')
const formError = ref('')
const form = ref({ name: '', expiresAt: '' })

onMounted(async () => { tokens.value = await listTokens() })

async function handleCreate() {
  formError.value = ''
  try {
    const res = await createToken(form.value.name, form.value.expiresAt || undefined)
    newToken.value = res.token
    showCreate.value = false
    form.value = { name: '', expiresAt: '' }
    tokens.value = await listTokens()
  } catch (e: any) { formError.value = e.message }
}

async function handleDelete(id: string) {
  if (!confirm('确认撤销此 Token？')) return
  await deleteToken(id)
  tokens.value = await listTokens()
}

function copyToken() { navigator.clipboard.writeText(newToken.value) }

// Tab 3: 日志
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
    if (!res.ok) throw new Error((await res.json()).error)
    logs.value = await res.json()
  } catch {}
}

function toggleLog(id: string) {
  expandedLog.value = expandedLog.value === id ? null : id
}
</script>

<style scoped>
.profile-tabs { display: flex; border-bottom: 1px solid var(--border); margin-bottom: 24px; }
.tab-btn { padding: 8px 20px; border: none; background: none; cursor: pointer; color: var(--text-sub); font-size: 14px; border-bottom: 2px solid transparent; }
.tab-btn.active { color: var(--text); border-bottom-color: var(--accent); }
</style>

