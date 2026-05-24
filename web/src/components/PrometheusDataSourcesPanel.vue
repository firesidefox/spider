<template>
  <div class="prom-page">
    <div class="page-title">Data Sources — Prometheus</div>
    <div class="page-subtitle">管理 Prometheus 监控数据源，在拓扑页面或主机页面绑定到具体作用域</div>

    <div class="ds-list">
      <div class="ds-list-header">
        <span class="ds-list-title">已配置数据源</span>
        <button class="btn-add" @click="openNew">+ 新增数据源</button>
      </div>
      <div v-if="loading" class="ds-empty">加载中...</div>
      <div v-else-if="sources.length === 0" class="ds-empty">暂无数据源</div>
      <div
        v-for="src in sources"
        :key="src.id"
        class="ds-row"
        :class="{ selected: drawerSourceId === src.id }"
        @click="openEdit(src)"
      >
        <div class="ds-icon">P</div>
        <div class="ds-info">
          <div class="ds-name">{{ src.name }}</div>
          <div class="ds-url">{{ src.base_url }} · {{ authLabel(src.auth_type) }}</div>
        </div>
        <div class="ds-chevron">›</div>
      </div>
    </div>

    <template v-if="drawerOpen">
      <div class="overlay" @click="closeDrawer" />
      <div class="drawer">
        <div class="drawer-header">
          <div class="drawer-title">
            <div class="ds-icon">P</div>
            {{ isNew ? '新增数据源' : '编辑数据源' }}
          </div>
          <button class="drawer-close" @click="closeDrawer">×</button>
        </div>
        <div class="drawer-body">
          <div class="form-section">
            <div class="form-section-title">HTTP</div>
            <div class="form-row">
              <label class="form-label">名称 <span class="req">*</span></label>
              <input v-model="form.name" class="form-input" placeholder="如：生产环境 Prometheus" />
            </div>
            <div class="form-row">
              <label class="form-label">URL <span class="req">*</span></label>
              <input v-model="form.base_url" class="form-input" placeholder="http://prometheus:9090" />
              <div class="form-hint">Prometheus 实例地址</div>
            </div>
            <div class="form-row">
              <label class="form-label">超时（秒）</label>
              <input v-model.number="form.timeout_seconds" class="form-input sm" type="number" min="1" />
            </div>
          </div>
          <div class="form-section">
            <div class="form-section-title">认证</div>
            <div class="form-row">
              <label class="form-label">认证方式</label>
              <select v-model="form.auth_type" class="form-select">
                <option value="none">无认证</option>
                <option value="basic">Basic Auth</option>
                <option value="bearer">Bearer Token</option>
              </select>
            </div>
            <div class="toggle-row">
              <div class="toggle" :class="{ on: form.skip_tls_verify }" @click="form.skip_tls_verify = !form.skip_tls_verify" />
              <span class="toggle-label">跳过 TLS 验证</span>
              <span class="toggle-hint">内网自签证书时启用</span>
            </div>
            <div v-if="form.auth_type === 'basic'" class="auth-detail">
              <div class="auth-detail-title">Basic Auth 详情</div>
              <div class="form-row">
                <label class="form-label">用户名</label>
                <input v-model="form.username" class="form-input" style="max-width:240px" />
              </div>
              <div class="form-row" style="margin-bottom:0">
                <label class="form-label">密码</label>
                <input v-model="form.password" class="form-input" type="password" placeholder="••••••••" style="max-width:240px" />
              </div>
            </div>
            <div v-if="form.auth_type === 'bearer'" class="auth-detail">
              <div class="auth-detail-title">Bearer Token</div>
              <div class="form-row" style="margin-bottom:0">
                <label class="form-label">Token</label>
                <input v-model="form.token" class="form-input" type="password" />
              </div>
            </div>
          </div>
        </div>
        <div class="drawer-footer">
          <button class="btn-save" :disabled="saving" @click="save">{{ saving ? '保存中...' : '保存' }}</button>
          <button class="btn-test" :disabled="testing || isNew" @click="testConn">测试连接</button>
          <span v-if="testResult" :class="testResult.ok ? 'test-ok' : 'test-err'">
            {{ testResult.ok ? `连接正常 · ${testResult.latency_ms}ms` : testResult.error }}
          </span>
          <button v-if="!isNew" class="btn-del" @click="confirmDelete">删除</button>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import {
  listPrometheusSources,
  addPrometheusSource,
  updatePrometheusSource,
  deletePrometheusSource,
  testPrometheusConnection,
  type PrometheusSource,
  type PrometheusAuthType,
} from '../api/prometheus'

const sources = ref<PrometheusSource[]>([])
const loading = ref(false)
const drawerOpen = ref(false)
const drawerSourceId = ref<string | null>(null)
const isNew = ref(false)
const saving = ref(false)
const testing = ref(false)
const testResult = ref<{ ok: boolean; latency_ms?: number; error?: string } | null>(null)

const emptyForm = () => ({
  name: '',
  base_url: '',
  timeout_seconds: 30,
  auth_type: 'none' as PrometheusAuthType,
  username: '',
  password: '',
  token: '',
  skip_tls_verify: false,
})
const form = reactive(emptyForm())

async function load() {
  loading.value = true
  try {
    sources.value = (await listPrometheusSources()) ?? []
  } finally {
    loading.value = false
  }
}
onMounted(load)

function authLabel(t: PrometheusAuthType) {
  return t === 'none' ? 'No Auth' : t === 'basic' ? 'Basic Auth' : 'Bearer Token'
}

function openNew() {
  Object.assign(form, emptyForm())
  drawerSourceId.value = null
  isNew.value = true
  testResult.value = null
  drawerOpen.value = true
}

function openEdit(src: PrometheusSource) {
  Object.assign(form, {
    name: src.name,
    base_url: src.base_url,
    timeout_seconds: src.timeout_seconds,
    auth_type: src.auth_type,
    username: src.username ?? '',
    password: '',
    token: '',
    skip_tls_verify: src.skip_tls_verify,
  })
  drawerSourceId.value = src.id
  isNew.value = false
  testResult.value = null
  drawerOpen.value = true
}

function closeDrawer() {
  drawerOpen.value = false
  drawerSourceId.value = null
}

async function save() {
  if (!form.name.trim() || !form.base_url.trim()) return
  saving.value = true
  try {
    if (isNew.value) {
      await addPrometheusSource({
        name: form.name,
        base_url: form.base_url,
        timeout_seconds: form.timeout_seconds,
        auth_type: form.auth_type,
        username: form.username || undefined,
        password: form.password || undefined,
        token: form.token || undefined,
        skip_tls_verify: form.skip_tls_verify,
      })
    } else if (drawerSourceId.value) {
      await updatePrometheusSource(drawerSourceId.value, {
        name: form.name,
        base_url: form.base_url,
        timeout_seconds: form.timeout_seconds,
        auth_type: form.auth_type,
        username: form.username || undefined,
        password: form.password || undefined,
        token: form.token || undefined,
        skip_tls_verify: form.skip_tls_verify,
      })
    }
    await load()
    closeDrawer()
  } catch (e: any) {
    alert(e.message)
  } finally {
    saving.value = false
  }
}

async function testConn() {
  if (!drawerSourceId.value) return
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await testPrometheusConnection(drawerSourceId.value)
  } catch (e: any) {
    testResult.value = { ok: false, error: e.message }
  } finally {
    testing.value = false
  }
}

async function confirmDelete() {
  if (!drawerSourceId.value) return
  if (!confirm('确认删除该数据源？关联绑定将一并删除。')) return
  await deletePrometheusSource(drawerSourceId.value)
  await load()
  closeDrawer()
}
</script>

<style scoped>
.prom-page {
  max-width: 800px;
}

.page-title {
  font-size: 20px;
  font-weight: 700;
  color: var(--text);
  letter-spacing: -0.02em;
  margin-bottom: 6px;
}

.page-subtitle {
  font-size: 13px;
  color: var(--muted, #6c7280);
  margin-bottom: 28px;
  line-height: 1.5;
}

/* ── Data Source List ── */
.ds-list {
  background: var(--card-bg, #1e2128);
  border: 1px solid var(--border, #2c2f36);
  border-radius: 10px;
  overflow: hidden;
}

.ds-list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border, #2c2f36);
  background: var(--surface, #1a1d23);
}

.ds-list-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-sub, #9ca3af);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.btn-add {
  background: #5794f2;
  border: none;
  border-radius: 6px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  cursor: pointer;
  transition: background 0.15s;
}
.btn-add:hover { background: #4080e8; }

.ds-empty {
  padding: 32px;
  text-align: center;
  font-size: 14px;
  color: var(--muted, #6c7280);
}

.ds-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border, #2c2f36);
  cursor: pointer;
  transition: background 0.12s;
}
.ds-row:last-child { border-bottom: none; }
.ds-row:hover { background: var(--row-hover, #1f2228); }
.ds-row.selected { background: rgba(87, 148, 242, 0.06); }

.ds-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: rgba(87, 148, 242, 0.15);
  color: #5794f2;
  font-size: 14px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.ds-info { flex: 1; min-width: 0; }

.ds-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text, #e2e8f0);
  margin-bottom: 2px;
}

.ds-url {
  font-size: 12px;
  color: var(--muted, #6c7280);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ds-chevron {
  font-size: 18px;
  color: var(--muted, #6c7280);
  flex-shrink: 0;
}

/* ── Overlay + Drawer ── */
.overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  z-index: 200;
  backdrop-filter: blur(2px);
}

.drawer {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: 480px;
  max-width: 95vw;
  background: var(--surface, #1a1d23);
  border-left: 1px solid var(--border, #2c2f36);
  z-index: 201;
  display: flex;
  flex-direction: column;
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.4);
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border, #2c2f36);
  flex-shrink: 0;
}

.drawer-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text, #e2e8f0);
}

.drawer-close {
  background: none;
  border: none;
  font-size: 22px;
  color: var(--muted, #6c7280);
  cursor: pointer;
  line-height: 1;
  padding: 2px 6px;
  border-radius: 4px;
  transition: color 0.12s, background 0.12s;
}
.drawer-close:hover { color: var(--text, #e2e8f0); background: var(--row-hover, #1f2228); }

.drawer-body {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

/* ── Form Sections ── */
.form-section {
  margin-bottom: 24px;
}

.form-section-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted, #6c7280);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  margin-bottom: 14px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border, #2c2f36);
}

.form-row {
  display: flex;
  flex-direction: column;
  gap: 5px;
  margin-bottom: 14px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-sub, #9ca3af);
  letter-spacing: 0.02em;
}

.req {
  color: #f87171;
  margin-left: 2px;
}

.form-input {
  background: var(--input-bg, #13151a);
  border: 1px solid var(--border, #2c2f36);
  border-radius: 6px;
  padding: 8px 11px;
  font-size: 13px;
  color: var(--text, #e2e8f0);
  outline: none;
  font-family: inherit;
  transition: border-color 0.15s, box-shadow 0.15s;
  width: 100%;
}
.form-input:focus {
  border-color: #5794f2;
  box-shadow: 0 0 0 3px rgba(87, 148, 242, 0.15);
}
.form-input::placeholder { color: var(--muted, #6c7280); }
.form-input.sm { width: 100px; }

.form-hint {
  font-size: 11px;
  color: var(--muted, #6c7280);
}

.form-select {
  background: var(--input-bg, #13151a);
  border: 1px solid var(--border, #2c2f36);
  border-radius: 6px;
  padding: 8px 11px;
  font-size: 13px;
  color: var(--text, #e2e8f0);
  outline: none;
  font-family: inherit;
  cursor: pointer;
  transition: border-color 0.15s;
  width: 200px;
}
.form-select:focus { border-color: #5794f2; }

/* ── Toggle ── */
.toggle-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}

.toggle {
  width: 36px;
  height: 20px;
  border-radius: 10px;
  background: var(--border, #2c2f36);
  cursor: pointer;
  position: relative;
  transition: background 0.2s;
  flex-shrink: 0;
}
.toggle::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #fff;
  transition: transform 0.2s;
}
.toggle.on {
  background: #5794f2;
}
.toggle.on::after {
  transform: translateX(16px);
}

.toggle-label {
  font-size: 13px;
  color: var(--text-sub, #9ca3af);
  font-weight: 500;
}

.toggle-hint {
  font-size: 11px;
  color: var(--muted, #6c7280);
}

/* ── Auth Detail Block ── */
.auth-detail {
  background: var(--card-bg, #1e2128);
  border: 1px solid var(--border, #2c2f36);
  border-radius: 8px;
  padding: 14px 16px;
  margin-top: 4px;
}

.auth-detail-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted, #6c7280);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 12px;
}

/* ── Drawer Footer ── */
.drawer-footer {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid var(--border, #2c2f36);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.btn-save {
  background: #5794f2;
  border: none;
  border-radius: 6px;
  padding: 8px 18px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  cursor: pointer;
  transition: background 0.15s;
}
.btn-save:hover:not(:disabled) { background: #4080e8; }
.btn-save:disabled { opacity: 0.5; cursor: not-allowed; }

.btn-test {
  background: var(--card-bg, #1e2128);
  border: 1px solid var(--border, #2c2f36);
  border-radius: 6px;
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-sub, #9ca3af);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.btn-test:hover:not(:disabled) { background: var(--row-hover, #1f2228); color: var(--text, #e2e8f0); }
.btn-test:disabled { opacity: 0.45; cursor: not-allowed; }

.test-ok {
  font-size: 12px;
  color: #22c55e;
  font-weight: 500;
}

.test-err {
  font-size: 12px;
  color: #f87171;
  font-weight: 500;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.btn-del {
  background: none;
  border: 1px solid rgba(248, 113, 113, 0.3);
  border-radius: 6px;
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 500;
  color: #f87171;
  cursor: pointer;
  margin-left: auto;
  transition: background 0.15s, border-color 0.15s;
}
.btn-del:hover { background: rgba(248, 113, 113, 0.08); border-color: #f87171; }
</style>
