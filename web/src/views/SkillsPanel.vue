<template>
  <div class="skills-panel">
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
import { ref, onMounted } from 'vue'

interface Skill { name: string; source: string }
type UploadStatus = { type: 'idle' } | { type: 'uploading'; name: string } | { type: 'success'; name: string } | { type: 'error'; msg: string }

const skills = ref<Skill[]>([])
const dragging = ref(false)
const status = ref<UploadStatus>({ type: 'idle' })
const fileInput = ref<HTMLInputElement | null>(null)
const uploadTarget = ref<string | null>(null)

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

onMounted(() => { loadSkills() })
</script>

<style scoped>
.skills-panel { flex: 1; overflow-y: auto; padding: 20px 24px; }
.card-header-row { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.settings-card.dragging { border-color: var(--primary); box-shadow: 0 0 0 3px rgba(99,102,241,0.15); }
.upload-status { flex: 1; text-align: center; font-size: 12px; color: var(--muted); padding: 0 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.upload-status--uploading { color: var(--text-sub); }
.upload-status--success { color: var(--green); font-weight: 600; }
.upload-status--error { color: var(--red); font-weight: 600; }
.drop-hint { text-align: center; padding: 32px; font-size: 14px; color: var(--primary); border: 2px dashed var(--primary); border-radius: 8px; background: rgba(99,102,241,0.04); }
.actions { display: flex; gap: 6px; }
</style>
