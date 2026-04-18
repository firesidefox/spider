<template>
  <div class="install-panel">
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
    <div
      class="settings-card"
      :class="{ dragging }"
      @dragover.prevent="dragging = true"
      @dragleave.self="dragging = false"
      @drop.prevent="onDrop"
    >
      <div class="card-header-row">
        <h3>Skills 管理</h3>
        <span class="upload-status"
          :class="{
            'upload-status--uploading': status.type === 'uploading',
            'upload-status--success': status.type === 'success',
            'upload-status--error': status.type === 'error',
          }"
        >
          <template v-if="status.type === 'idle'">拖拽 .md 文件到此处</template>
          <template v-else-if="status.type === 'uploading'">⟳ 上传 {{ status.name }} 中…</template>
          <template v-else-if="status.type === 'success'">✓ {{ status.name }} 已上传</template>
          <template v-else-if="status.type === 'error'">✗ {{ status.msg }}</template>
        </span>
        <button class="btn btn-sm btn-primary" @click="triggerUpload(null)">添加 Skill</button>
      </div>
      <input ref="fileInput" type="file" accept=".md" style="display:none" @change="onFileChange" />
      <p v-if="dragging" class="drop-hint">松手上传 .md 文件</p>
      <table v-else class="table">
        <thead><tr><th>名称</th><th>来源</th><th class="actions">操作</th></tr></thead>
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
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

interface Skill { name: string; source: string }
type UploadStatus = { type: 'idle' } | { type: 'uploading'; name: string } | { type: 'success'; name: string } | { type: 'error'; msg: string }

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
  if (!file.name.match(/\.md$/i)) { setStatus({ type: 'error', msg: '仅支持 .md 文件' }); return }
  const name = uploadTarget.value ?? file.name.replace(/\.md$/i, '')
  setStatus({ type: 'uploading', name })
  try {
    const content = await file.text()
    const res = await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, { method: 'PUT', headers: { 'Content-Type': 'text/plain' }, body: content })
    ;(e.target as HTMLInputElement).value = ''
    if (res.ok) { setStatus({ type: 'success', name }); await loadSkills() }
    else setStatus({ type: 'error', msg: '上传失败，请重试' })
  } catch { setStatus({ type: 'error', msg: '上传失败，请重试' }) }
}

async function onDrop(e: DragEvent) {
  dragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (!file) return
  if (!file.name.endsWith('.md')) { setStatus({ type: 'error', msg: '仅支持 .md 文件' }); return }
  const name = file.name.replace(/\.md$/i, '')
  setStatus({ type: 'uploading', name })
  try {
    const content = await file.text()
    const res = await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, { method: 'PUT', headers: { 'Content-Type': 'text/plain' }, body: content })
    if (res.ok) { setStatus({ type: 'success', name }); await loadSkills() }
    else setStatus({ type: 'error', msg: '上传失败，请重试' })
  } catch { setStatus({ type: 'error', msg: '上传失败，请重试' }) }
}

async function deleteSkill(name: string) {
  if (!confirm(`确认删除 Skill "${name}"？`)) return
  await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, { method: 'DELETE' })
  await loadSkills()
}

onMounted(() => { loadSkills(); loadScript() })
</script>

<style scoped>
.install-panel { flex: 1; overflow-y: auto; padding: 20px 24px; display: flex; flex-direction: column; gap: 16px; }
.copy-row { display: flex; align-items: center; gap: 10px; background: var(--panel); border: 1px solid var(--border); border-radius: 8px; padding: 10px 14px; margin: 12px 0; }
.checklist { list-style: none; padding: 0; display: flex; flex-direction: column; gap: 4px; font-size: 13px; color: var(--text-sub); }
.collapsible-header { display: flex; align-items: center; justify-content: space-between; cursor: pointer; }
.chevron { font-size: 11px; color: var(--muted); transition: transform 0.2s; }
.chevron.open { transform: rotate(90deg); }
.script-body { margin-top: 12px; }
.card-header-row { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.settings-card.dragging { border-color: var(--primary); box-shadow: 0 0 0 3px rgba(99,102,241,0.15); }
.upload-status { flex: 1; text-align: center; font-size: 12px; color: var(--muted); padding: 0 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.upload-status--uploading { color: var(--text-sub); }
.upload-status--success { color: var(--green); font-weight: 600; }
.upload-status--error { color: var(--red); font-weight: 600; }
.drop-hint { text-align: center; padding: 32px; font-size: 14px; color: var(--primary); border: 2px dashed var(--primary); border-radius: 8px; background: rgba(99,102,241,0.04); }
.actions { display: flex; gap: 6px; }
</style>
