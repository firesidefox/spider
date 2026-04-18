<template>
  <div class="install-panel">
    <div class="settings-card">
      <h3>一键安装</h3>
      <p class="dim">在目标主机上运行以下命令，自动下载并安装 Spider Agent。</p>
      <div class="token-hint">
        <p class="dim">执行安装脚本前，请先准备一个<a class="link" @click="$emit('switch-tab', 'tokens')">访问令牌</a>，用于注册 MCP 服务器时的身份认证。</p>
        <div class="form-row">
          <label>访问令牌</label>
          <input v-model="token" class="input" placeholder="粘贴你的 Token" />
        </div>
      </div>
      <div class="copy-row">
        <code class="code">{{ curlCmd }}</code>
        <button class="btn btn-sm btn-primary" :disabled="!token" @click="copyCmd">{{ copied ? '已复制 ✓' : '复制' }}</button>
      </div>
      <ul class="checklist">
        <li>✓ 自动检测系统架构（amd64 / arm64）</li>
        <li>✓ 安装完成后自动注册为系统服务</li>
      </ul>
    </div>
    <div class="settings-card">
      <div class="collapsible-header" @click="scriptOpen = !scriptOpen">
        <h3>查看安装脚本</h3>
        <span class="chevron" :class="{ open: scriptOpen }">▶</span>
      </div>
      <div v-if="scriptOpen" class="script-body">
        <pre v-if="scriptContent" class="output">{{ scriptContent }}</pre>
        <p v-else class="dim">加载中…</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

const emit = defineEmits<{ 'switch-tab': [tab: string] }>()

const token = ref('')
const scriptContent = ref('')
const scriptOpen = ref(false)
const copied = ref(false)

const origin = window.location.origin
const curlCmd = computed(() =>
  token.value
    ? `curl -fsSL ${origin}/install.sh | sh -s -- --token ${token.value}`
    : `curl -fsSL ${origin}/install.sh | sh -s -- --token <YOUR_TOKEN>`
)

async function copyCmd() {
  await navigator.clipboard.writeText(curlCmd.value)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

async function loadScript() {
  const res = await fetch('/install.sh')
  if (res.ok) scriptContent.value = await res.text()
}

onMounted(() => { loadScript() })
</script>

<style scoped>
.install-panel { flex: 1; overflow-y: auto; padding: 20px 24px; display: flex; flex-direction: column; gap: 16px; }
.copy-row { display: flex; align-items: center; gap: 10px; background: var(--panel); border: 1px solid var(--border); border-radius: 8px; padding: 10px 14px; margin: 12px 0; }
.checklist { list-style: none; padding: 0; display: flex; flex-direction: column; gap: 4px; font-size: 13px; color: var(--text-sub); }
.collapsible-header { display: flex; align-items: center; justify-content: space-between; cursor: pointer; }
.chevron { font-size: 11px; color: var(--muted); transition: transform 0.2s; }
.chevron.open { transform: rotate(90deg); }
.script-body { margin-top: 12px; }
.token-hint { margin-bottom: 12px; }
.link { color: var(--primary); cursor: pointer; text-decoration: underline; }
</style>
