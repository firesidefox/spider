<template>
  <div>
    <div class="page-header"><h2>设置</h2></div>

    <div class="settings-card">
      <h3>MCP Server</h3>
      <div class="form-row">
        <label>监听地址</label>
        <input v-model="form.sse_addr" class="input" placeholder=":8000" />
      </div>
      <div class="form-row">
        <label>Base URL</label>
        <input v-model="form.sse_base_url" class="input" placeholder="http://localhost:8000" />
      </div>
    </div>

    <div class="settings-card">
      <h3>SSH 默认配置</h3>
      <div class="form-row">
        <label>命令超时（秒）</label>
        <input v-model.number="form.ssh_default_timeout_seconds" class="input" type="number" style="width:120px" />
      </div>
      <div class="form-row">
        <label>连接池 TTL（秒）</label>
        <input v-model.number="form.ssh_pool_ttl_seconds" class="input" type="number" style="width:120px" />
      </div>
      <div class="form-row">
        <label>最大连接数</label>
        <input v-model.number="form.ssh_max_pool_size" class="input" type="number" style="width:120px" />
      </div>
    </div>

    <div style="display:flex;gap:12px;align-items:center">
      <button class="btn btn-primary" @click="save">保存</button>
      <span v-if="saved" class="ok">已保存</span>
      <span v-if="error" class="err">{{ error }}</span>
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
