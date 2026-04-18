<template>
  <div class="sp-wrap">
    <!-- 左侧列表 -->
    <aside class="sp-sidebar">
      <div class="sp-toolbar">
        <span class="sp-title">Skills</span>
        <button class="btn btn-primary btn-sm" @click="triggerUpload(null)">+ 添加</button>
      </div>
      <div
        class="sp-list"
        :class="{ dragging }"
        @dragover.prevent="dragging = true"
        @dragleave.self="dragging = false"
        @drop.prevent="onDrop"
      >
        <div v-if="dragging" class="drop-hint">松手上传 .md 文件</div>
        <template v-else>
          <div
            v-for="skill in skills" :key="skill.name"
            class="sp-row"
            :class="{ selected: selected?.name === skill.name }"
            @click="selectSkill(skill)"
          >
            <span class="sp-row-name">{{ skill.name }}</span>
            <span v-if="skill.source === 'custom'" class="badge">自定义</span>
          </div>
          <div v-if="skills.length === 0" class="sp-empty">暂无 Skills</div>
        </template>
      </div>
      <div class="sp-status" :class="statusClass">{{ statusText }}</div>
    </aside>

    <!-- 右侧详情 -->
    <div class="sp-detail">
      <template v-if="selected">
        <div class="sp-topbar">
          <span class="sp-detail-title">{{ selected.name }}</span>
          <div class="sp-topbar-right">
            <button class="btn btn-sm" :class="{ active: viewMode === 'rendered' }" @click="viewMode = 'rendered'">渲染</button>
            <button class="btn btn-sm" :class="{ active: viewMode === 'raw' }" @click="viewMode = 'raw'">原文</button>
            <button class="btn btn-sm btn-primary" @click="triggerUpload(selected.name)">上传新版本</button>
            <button v-if="selected.source === 'custom'" class="btn btn-sm btn-danger" @click="deleteSkill(selected.name)">删除</button>
          </div>
        </div>
        <div class="sp-body">
          <div class="sp-card">
            <div v-if="loading" class="sp-loading">加载中…</div>
            <template v-else-if="viewMode === 'rendered'">
              <template v-for="(block, i) in mdBlocks" :key="i">
                <div v-if="block.type === 'html'" class="sp-markdown" v-html="block.content"></div>
                <div v-else class="sp-code-block">
                  <div v-if="block.lang" class="sp-code-lang">{{ block.lang }}</div>
                  <CodeBlock :code="block.content" />
                </div>
              </template>
            </template>
            <pre v-else class="sp-raw">{{ rawContent }}</pre>
          </div>
        </div>
      </template>
      <div v-else class="sp-empty-state">
        <div class="sp-empty-icon">←</div>
        <div>选择左侧 Skill 查看内容</div>
      </div>
    </div>

    <input ref="fileInput" type="file" accept=".md" style="display:none" @change="onFileChange" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { marked } from 'marked'
import CodeBlock from '../components/CodeBlock.vue'

// 斜杠不编码，只编码各段（支持 spider/cron 格式）
const encodeSkillName = (name: string) => name.split('/').map(encodeURIComponent).join('/')

type MdBlock = { type: 'html'; content: string } | { type: 'code'; content: string; lang: string }

function parseMdBlocks(src: string): MdBlock[] {
  const tokens = marked.lexer(src)
  const blocks: MdBlock[] = []
  let htmlBuf = ''
  for (const tok of tokens) {
    if (tok.type === 'code') {
      if (htmlBuf) { blocks.push({ type: 'html', content: marked.parser([]) }); htmlBuf = '' }
      // 收集非代码 token 渲染成 html
      blocks.push({ type: 'code', content: tok.text, lang: tok.lang ?? '' })
    } else {
      htmlBuf += marked.parser([tok as any])
    }
  }
  if (htmlBuf) blocks.push({ type: 'html', content: htmlBuf })
  return blocks
}

interface Skill { name: string; source: string }
type UploadStatus = { type: 'idle' } | { type: 'uploading'; name: string } | { type: 'success'; name: string } | { type: 'error'; msg: string }

const skills = ref<Skill[]>([])
const selected = ref<Skill | null>(null)
const rawContent = ref('')
const loading = ref(false)
const viewMode = ref<'rendered' | 'raw'>('rendered')
const dragging = ref(false)
const status = ref<UploadStatus>({ type: 'idle' })
const fileInput = ref<HTMLInputElement | null>(null)
const uploadTarget = ref<string | null>(null)

const renderedContent = computed(() => marked.parse(rawContent.value) as string)
const mdBlocks = computed(() => parseMdBlocks(rawContent.value))

const statusClass = computed(() => ({
  'sp-status--uploading': status.value.type === 'uploading',
  'sp-status--success': status.value.type === 'success',
  'sp-status--error': status.value.type === 'error',
}))

const statusText = computed(() => {
  const s = status.value
  if (s.type === 'idle') return '拖拽 .md 文件到列表区上传'
  if (s.type === 'uploading') return `⟳ 上传 ${s.name} 中…`
  if (s.type === 'success') return `✓ ${s.name} 已上传`
  return `✗ ${s.msg}`
})

async function loadSkills() {
  const res = await fetch('/api/v1/skills')
  if (res.ok) skills.value = await res.json()
}

async function selectSkill(skill: Skill) {
  selected.value = skill
  loading.value = true
  rawContent.value = ''
  try {
    const res = await fetch(`/api/v1/skills/${encodeSkillName(skill.name)}`)
    if (res.ok) rawContent.value = await res.text()
  } finally {
    loading.value = false
  }
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

async function uploadFile(file: File, name: string) {
  setStatus({ type: 'uploading', name })
  try {
    const content = await file.text()
    const res = await fetch(`/api/v1/skills/${encodeSkillName(name)}`, {
      method: 'PUT', headers: { 'Content-Type': 'text/plain' }, body: content,
    })
    if (res.ok) {
      setStatus({ type: 'success', name })
      await loadSkills()
      if (selected.value?.name === name) await selectSkill(selected.value)
    } else {
      setStatus({ type: 'error', msg: '上传失败，请重试' })
    }
  } catch { setStatus({ type: 'error', msg: '上传失败，请重试' }) }
}

async function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (!file.name.match(/\.md$/i)) { setStatus({ type: 'error', msg: '仅支持 .md 文件' }); return }
  const name = uploadTarget.value ?? file.name.replace(/\.md$/i, '')
  ;(e.target as HTMLInputElement).value = ''
  await uploadFile(file, name)
}

async function onDrop(e: DragEvent) {
  dragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (!file) return
  if (!file.name.endsWith('.md')) { setStatus({ type: 'error', msg: '仅支持 .md 文件' }); return }
  await uploadFile(file, file.name.replace(/\.md$/i, ''))
}

async function deleteSkill(name: string) {
  if (!confirm(`确认删除 Skill "${name}"？`)) return
  await fetch(`/api/v1/skills/${encodeSkillName(name)}`, { method: 'DELETE' })
  if (selected.value?.name === name) selected.value = null
  await loadSkills()
}

onMounted(() => { loadSkills() })
</script>

<style scoped>
.sp-wrap { display: flex; flex: 1; min-height: 0; overflow: hidden; }

.sp-sidebar {
  width: 260px; flex-shrink: 0;
  background: var(--panel); border-right: 1px solid var(--border);
  display: flex; flex-direction: column; overflow: hidden;
}
.sp-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0;
}
.sp-title { font-size: 13px; font-weight: 700; color: var(--text); }
.sp-list { flex: 1; overflow-y: auto; }
.sp-list.dragging { border: 2px dashed var(--primary); background: rgba(99,102,241,0.04); }
.drop-hint { text-align: center; padding: 32px 16px; font-size: 13px; color: var(--primary); }
.sp-row {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 16px; border-bottom: 1px solid var(--border);
  border-left: 3px solid transparent; cursor: pointer; gap: 8px; transition: background 0.1s;
}
.sp-row:hover { background: var(--row-hover); }
.sp-row.selected { border-left-color: var(--primary); background: rgba(99,102,241,0.1); }
.sp-row-name { font-size: 13px; font-weight: 500; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.sp-empty { color: var(--label); font-size: 13px; padding: 32px 16px; text-align: center; }
.sp-status {
  font-size: 11px; color: var(--label); padding: 8px 16px;
  border-top: 1px solid var(--border); flex-shrink: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.sp-status--uploading { color: var(--text-sub); }
.sp-status--success { color: var(--green); font-weight: 600; }
.sp-status--error { color: var(--red); font-weight: 600; }

.sp-detail { flex: 1; overflow: hidden; min-width: 0; display: flex; flex-direction: column; }
.sp-topbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 20px; border-bottom: 1px solid var(--border);
  background: var(--surface); flex-shrink: 0;
}
.sp-detail-title { font-size: 14px; font-weight: 700; color: var(--text); }
.sp-topbar-right { display: flex; gap: 8px; }
.btn.active { background: rgba(99,102,241,0.15); color: var(--primary); border-color: rgba(99,102,241,0.4); }
.sp-body { flex: 1; overflow-y: auto; padding: 20px 24px; }
.sp-card {
  background: var(--card-bg); border: 1px solid var(--border);
  border-radius: 10px; padding: 20px 24px; box-shadow: var(--card-shadow);
}
.sp-loading { color: var(--muted); font-size: 13px; }
.sp-empty-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: var(--muted); font-size: 14px; }
.sp-empty-icon { color: var(--border); font-size: 40px; }

.sp-raw {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px; line-height: 1.6; color: var(--text);
  white-space: pre-wrap; word-break: break-word; margin: 0;
}

.sp-markdown { color: var(--text); font-size: 14px; line-height: 1.7; margin-bottom: 4px; }
.sp-markdown :deep(h1) { font-size: 20px; font-weight: 700; margin: 0 0 16px; color: var(--text); border-bottom: 1px solid var(--border); padding-bottom: 8px; }
.sp-markdown :deep(h2) { font-size: 16px; font-weight: 700; margin: 24px 0 10px; color: var(--text); }
.sp-markdown :deep(h3) { font-size: 14px; font-weight: 600; margin: 18px 0 8px; color: var(--text); }
.sp-markdown :deep(p) { margin: 0 0 12px; }
.sp-markdown :deep(code) { font-family: 'JetBrains Mono', monospace; font-size: 12px; background: rgba(99,102,241,0.1); color: var(--primary); padding: 1px 5px; border-radius: 4px; }

.sp-code-block {
  background: var(--panel); border: 1px solid var(--border); border-radius: 10px;
  overflow: hidden; margin: 0 0 16px;
}
.sp-code-lang {
  padding: 7px 16px 6px; font-size: 11px; font-weight: 600;
  color: var(--muted); border-bottom: 1px solid var(--border); letter-spacing: 0.04em;
}

.sp-markdown :deep(table) { width: 100%; border-collapse: collapse; margin: 0 0 14px; font-size: 13px; }
.sp-markdown :deep(th) { background: var(--panel); color: var(--muted); font-weight: 600; font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; padding: 8px 12px; border: 1px solid var(--border); text-align: left; }
.sp-markdown :deep(td) { padding: 8px 12px; border: 1px solid var(--border); color: var(--text); }
.sp-markdown :deep(tr:nth-child(even) td) { background: var(--row-alt); }
.sp-markdown :deep(ul), .sp-markdown :deep(ol) { padding-left: 20px; margin: 0 0 12px; }
.sp-markdown :deep(li) { margin-bottom: 4px; }
.sp-markdown :deep(blockquote) { border-left: 3px solid var(--primary); margin: 0 0 12px; padding: 8px 16px; background: rgba(99,102,241,0.05); color: var(--muted); }
.sp-markdown :deep(hr) { border: none; border-top: 1px solid var(--border); margin: 20px 0; }
.sp-markdown :deep(a) { color: var(--primary); text-decoration: none; }
.sp-markdown :deep(a:hover) { text-decoration: underline; }
</style>
