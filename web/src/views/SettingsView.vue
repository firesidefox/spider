<template>
  <div class="settings-page">
    <div class="settings-container">
      <div class="settings-header">
        <h2>设置</h2>
        <p>配置 Spider 服务参数</p>
      </div>

      <div class="settings-block">
        <div class="block-title">MCP Server</div>
        <div class="block-grid">
          <div class="form-row">
            <label>监听地址</label>
            <input v-model="form.sse_addr" class="input" placeholder=":8000" />
          </div>
          <div class="form-row">
            <label>Base URL</label>
            <input v-model="form.sse_base_url" class="input" placeholder="http://localhost:8000" />
          </div>
        </div>
      </div>

      <div class="settings-block">
        <div class="block-title">SSH 默认配置</div>
        <div class="block-grid">
          <div class="form-row">
            <label>命令超时（秒）</label>
            <input v-model.number="form.ssh_default_timeout_seconds" class="input" type="number" />
          </div>
          <div class="form-row">
            <label>连接池 TTL（秒）</label>
            <input v-model.number="form.ssh_pool_ttl_seconds" class="input" type="number" />
          </div>
          <div class="form-row">
            <label>最大连接数</label>
            <input v-model.number="form.ssh_max_pool_size" class="input" type="number" />
          </div>
        </div>
      </div>

      <div class="settings-footer">
        <button class="btn btn-primary" @click="save">保存设置</button>
        <span v-if="saved" class="ok">已保存</span>
        <span v-if="error" class="err">{{ error }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'

interface Settings {
  sse_addr: string
  sse_base_url: string
  ssh_default_timeout_seconds: number
  ssh_pool_ttl_seconds: number
  ssh_max_pool_size: number
}

const form = ref<Settings>({ sse_addr: '', sse_base_url: '', ssh_default_timeout_seconds: 30, ssh_pool_ttl_seconds: 300, ssh_max_pool_size: 50 })
const saved = ref(false)
const error = ref('')

async function load() {
  const res = await fetch('/api/v1/settings')
  if (res.ok) form.value = await res.json()
}

async function save() {
  saved.value = false
  error.value = ''
  const res = await fetch('/api/v1/settings', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(form.value) })
  if (res.ok) { saved.value = true; setTimeout(() => saved.value = false, 2000) }
  else error.value = (await res.json()).error
}

onMounted(load)
</script>

<style scoped>
.settings-page {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 32px 40px;
}

.settings-container {
  max-width: 680px;
}

.settings-header {
  margin-bottom: 24px;
}

.settings-header h2 {
  font-size: 20px;
  font-weight: 700;
  color: var(--text);
  letter-spacing: -0.02em;
  margin-bottom: 4px;
}

.settings-header p {
  font-size: 13px;
  color: var(--muted);
}

.settings-block {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px 24px;
  box-shadow: var(--card-shadow);
  margin-bottom: 16px;
}

.block-title {
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

.settings-footer {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 48px;
}
</style>
