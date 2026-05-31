<template>
  <div class="detail-topbar">
    <span class="detail-title">偏好设置</span>
    <div v-if="settingsEditing" style="display:flex;gap:8px">
      <button class="btn btn-primary btn-sm" @click="saveSettings">保存</button>
      <button class="btn btn-sm" @click="cancelSettings">取消</button>
    </div>
    <button v-else class="btn btn-sm" @click="settingsEditing = true">编辑</button>
  </div>
  <div class="detail-body">
    <!-- 只读视图 -->
    <template v-if="!settingsEditing">
      <div class="edit-card">
        <div class="edit-card-title">MCP Server</div>
        <div class="detail-grid">
          <div class="detail-field">
            <div class="detail-label">监听地址</div>
            <div class="detail-value">{{ settings.sse_addr || '—' }}</div>
          </div>
          <div class="detail-field">
            <div class="detail-label">Base URL</div>
            <div class="detail-value">{{ settings.sse_base_url || '—' }}</div>
          </div>
        </div>
      </div>
      <div class="edit-card">
        <div class="edit-card-title">SSH 默认配置</div>
        <div class="detail-grid">
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
          <div class="detail-field">
            <div class="detail-label">直连地址（No Proxy）</div>
            <div class="detail-value">{{ settings.ssh_no_proxy || '—' }}</div>
          </div>
        </div>
      </div>
      <div class="edit-card">
        <div class="edit-card-title">日志</div>
        <div class="log-cfg-row">
          <span class="log-cfg-lbl">全局级别</span>
          <span :class="['log-cfg-badge', `log-cfg-badge--${logLevel}`]">{{ levelLabel(logLevel) }}</span>
        </div>
        <hr class="log-cfg-divider">
        <div v-for="m in LOG_MODULES" :key="m" class="log-cfg-row">
          <span class="log-cfg-mod">{{ m }}</span>
          <span :class="['log-cfg-badge', `log-cfg-badge--${moduleLevels[m] ?? 'inherit'}`]">{{ levelLabel(moduleLevels[m] ?? 'inherit') }}</span>
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
          <div class="form-row"><label>直连地址（No Proxy）</label><input v-model="settings.ssh_no_proxy" class="input" placeholder="10.0.0.0/8,192.168.0.0/16" /></div>
        </div>
      </div>
      <div class="edit-card">
        <div class="edit-card-title">日志</div>
        <div class="log-cfg-row">
          <span class="log-cfg-lbl">全局级别</span>
          <select v-model="logLevel" class="input log-cfg-select">
            <option value="debug">调试 debug</option>
            <option value="info">信息 info</option>
            <option value="warn">警告 warn</option>
            <option value="error">错误 error</option>
          </select>
        </div>
        <div v-if="logLevelError" class="err" style="margin-top:4px;font-size:12px">{{ logLevelError }}</div>
        <hr class="log-cfg-divider">
        <div v-for="m in LOG_MODULES" :key="m" class="log-cfg-row">
          <span class="log-cfg-mod">{{ m }}</span>
          <select v-model="moduleLevels[m]" class="input log-cfg-select">
            <option value="inherit">继承 inherit</option>
            <option value="debug">调试 debug</option>
            <option value="info">信息 info</option>
            <option value="warn">警告 warn</option>
            <option value="error">错误 error</option>
          </select>
        </div>
      </div>
      <div v-if="settingsError" class="err" style="margin-top:4px">{{ settingsError }}</div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { authHeaders } from '../../api/auth'

interface Settings {
  sse_addr: string; sse_base_url: string
  ssh_default_timeout_seconds: number; ssh_pool_ttl_seconds: number; ssh_max_pool_size: number
  ssh_no_proxy: string
}

const settings = ref<Settings>({
  sse_addr: '', sse_base_url: '',
  ssh_default_timeout_seconds: 30, ssh_pool_ttl_seconds: 300, ssh_max_pool_size: 50,
  ssh_no_proxy: '',
})
const settingsEditing = ref(false)
const settingsError = ref('')
const LOG_MODULES = ['main', 'scheduler', 'agent', 'mcp', 'ssh'] as const
const logLevel = ref('info')
const logLevelError = ref('')
const moduleLevels = ref<Record<string, string>>({})

const LEVEL_LABELS: Record<string, string> = {
  inherit: '继承 inherit',
  debug: '调试 debug',
  info: '信息 info',
  warn: '警告 warn',
  error: '错误 error',
}
function levelLabel(v: string): string { return LEVEL_LABELS[v] ?? v }
let settingsLoaded = false

async function loadSettings() {
  if (settingsLoaded) return
  settingsLoaded = true
  const [res, lvlRes] = await Promise.all([
    fetch('/api/v1/settings', { headers: authHeaders() }),
    fetch('/api/v1/log-level', { headers: authHeaders() }),
  ])
  if (!res.ok) return
  const data = await res.json()
  settings.value = {
    sse_addr: data.sse_addr || '',
    sse_base_url: data.sse_base_url || '',
    ssh_default_timeout_seconds: data.ssh_default_timeout_seconds ?? 30,
    ssh_pool_ttl_seconds: data.ssh_pool_ttl_seconds ?? 300,
    ssh_max_pool_size: data.ssh_max_pool_size ?? 50,
    ssh_no_proxy: data.ssh_no_proxy || '',
  }
  if (lvlRes.ok) {
    const lvlData = await lvlRes.json()
    logLevel.value = lvlData.level || 'info'
    const mods = lvlData.modules ?? {}
    for (const m of LOG_MODULES) {
      moduleLevels.value[m] = mods[m] ?? 'inherit'
    }
  }
}

async function saveSettings() {
  settingsError.value = ''
  logLevelError.value = ''
  const res = await fetch('/api/v1/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(settings.value),
  })
  if (!res.ok) {
    settingsError.value = (await res.json().catch(() => ({}))).error || '保存失败'
    return
  }
  const lvlRes = await fetch('/api/v1/log-level', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ level: logLevel.value }),
  })
  if (!lvlRes.ok) {
    logLevelError.value = (await lvlRes.json().catch(() => ({}))).error || '保存失败'
    return
  }
  const modResults = await Promise.allSettled(LOG_MODULES.map(m =>
    fetch('/api/v1/log-level', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ module: m, level: moduleLevels.value[m] ?? 'inherit' }),
    }).then(r => { if (!r.ok) throw new Error(m) })
  ))
  const failed = modResults.filter(r => r.status === 'rejected').map(r => (r as PromiseRejectedResult).reason?.message ?? '?')
  if (failed.length) {
    logLevelError.value = `模块保存失败: ${failed.join(', ')}`
    return
  }
  settingsEditing.value = false
}

function cancelSettings() {
  settingsEditing.value = false
  settingsLoaded = false
  loadSettings()
}

onMounted(() => {
  loadSettings()
})
</script>

<style scoped>
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

.block-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

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
</style>
