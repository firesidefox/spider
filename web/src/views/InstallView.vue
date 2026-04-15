<template>
  <div class="install-page">
    <div class="install-container">

      <!-- 卡片1：一键安装 -->
      <div class="settings-card">
        <h3>一键安装</h3>
        <p class="dim">在目标主机上运行以下命令，自动下载并安装 Spider Agent。</p>
        <div class="copy-row">
          <code class="code">{{ curlCmd }}</code>
          <button class="btn btn-sm btn-primary" @click="copyCmd">{{ copied ? '已复制 ✓' : '复制' }}</button>
        </div>
        <ul class="checklist">
          <li>✓ 自动检测系统架构（amd64 / arm64）</li>
          <li>✓ 安装完成后自动注册为系统服务</li>
        </ul>
      </div>

      <!-- 卡片2：查看安装脚本（可折叠） -->
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

      <!-- 卡片3：Skills 管理 -->
      <div
        class="settings-card"
        :class="{ dragging }"
        @dragover.prevent="dragging = true"
        @dragleave.self="dragging = false"
        @drop.prevent="onDrop"
      >
        <div class="card-header-row">
          <h3>Skills 管理</h3>
          <button class="btn btn-sm btn-primary" @click="triggerUpload(null)">添加 Skill</button>
        </div>
        <input ref="fileInput" type="file" accept=".md" style="display:none" @change="onFileChange" />
        <p v-if="dragging" class="drop-hint">松手上传 .md 文件</p>
        <table v-else class="table">
          <thead>
            <tr><th>名称</th><th>来源</th><th class="actions">操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="skill in skills" :key="skill.name">
              <td>{{ skill.name }}</td>
              <td>
                <span v-if="skill.source === 'custom'" class="badge">自定义</span>
                <span v-else class="dim">内嵌</span>
              </td>
              <td class="actions">
                <button class="btn btn-sm" @click="triggerUpload(skill.name)">上传新版本</button>
                <button v-if="skill.source === 'custom'" class="btn btn-sm btn-danger" @click="deleteSkill(skill.name)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

interface Skill { name: string; source: string }

type UploadStatus =
  | { type: 'idle' }
  | { type: 'uploading'; name: string }
  | { type: 'success'; name: string }
  | { type: 'error'; msg: string }

const skills = ref<Skill[]>([])
const scriptContent = ref('')
const scriptOpen = ref(false)
const copied = ref(false)
const dragging = ref(false)
const status = ref<UploadStatus>({ type: 'idle' })
const fileInput = ref<HTMLInputElement | null>(null)
const uploadTarget = ref<string | null>(null)

const curlCmd = computed(() => `curl -fsSL ${window.location.origin}/install.sh | sh`)

async function copyCmd() {
  await navigator.clipboard.writeText(curlCmd.value)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

async function loadScript() {
  const res = await fetch('/install.sh')
  if (res.ok) scriptContent.value = await res.text()
}

async function loadSkills() {
  const res = await fetch('/api/v1/skills')
  if (res.ok) skills.value = await res.json()
}

function setStatus(s: UploadStatus) {
  status.value = s
  if (s.type === 'success') setTimeout(() => { status.value = { type: 'idle' } }, 2000)
  if (s.type === 'error') setTimeout(() => { status.value = { type: 'idle' } }, 3000)
}

function triggerUpload(name: string | null) {
  uploadTarget.value = name
  fileInput.value?.click()
}

async function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const name = uploadTarget.value ?? file.name.replace(/\.md$/i, '')
  setStatus({ type: 'uploading', name })
  const content = await file.text()
  const res = await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
    body: content,
  })
  ;(e.target as HTMLInputElement).value = ''
  if (res.ok) {
    setStatus({ type: 'success', name })
    await loadSkills()
  } else {
    setStatus({ type: 'error', msg: '上传失败，请重试' })
  }
}

async function onDrop(e: DragEvent) {
  dragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (!file) return
  if (!file.name.endsWith('.md')) {
    setStatus({ type: 'error', msg: '仅支持 .md 文件' })
    return
  }
  const name = file.name.replace(/\.md$/i, '')
  setStatus({ type: 'uploading', name })
  const content = await file.text()
  const res = await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
    body: content,
  })
  if (res.ok) {
    setStatus({ type: 'success', name })
    await loadSkills()
  } else {
    setStatus({ type: 'error', msg: '上传失败，请重试' })
  }
}

async function deleteSkill(name: string) {
  if (!confirm(`确认删除 Skill "${name}"？`)) return
  await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, { method: 'DELETE' })
  await loadSkills()
}

onMounted(() => {
  loadSkills()
  loadScript()
})
</script>

<style scoped>
.install-page {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 32px 40px;
}

.install-container {
  max-width: 760px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-card h3 {
  font-size: 14px;
  font-weight: 700;
  margin-bottom: 10px;
}

.copy-row {
  display: flex;
  align-items: center;
  gap: 10px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px 14px;
  margin: 12px 0;
}

.copy-row .code {
  flex: 1;
  font-size: 13px;
  word-break: break-all;
}

.checklist {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
  color: var(--text-sub);
}

.collapsible-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  user-select: none;
}

.chevron {
  font-size: 11px;
  color: var(--muted);
  transition: transform 0.2s;
}

.chevron.open {
  transform: rotate(90deg);
}

.script-body {
  margin-top: 12px;
}

.card-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.settings-card.dragging {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
}

.drop-hint {
  text-align: center;
  padding: 32px;
  font-size: 14px;
  color: var(--primary);
  border: 2px dashed var(--primary);
  border-radius: 8px;
  background: rgba(99, 102, 241, 0.04);
}
</style>
