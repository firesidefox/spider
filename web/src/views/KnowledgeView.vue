<template>
  <div class="fullscreen-page kb-page">
    <aside class="kb-sidebar">
      <div class="sidebar-toolbar">
        <span class="sidebar-title">知识库 <span v-if="docs.length" class="doc-count">{{ docs.length }}</span></span>
        <div style="display:flex;gap:6px">
          <button v-if="!editMode" class="btn btn-sm btn-ghost" @click="enterEditMode">编辑</button>
          <button v-else class="btn btn-sm btn-ghost" @click="exitEditMode">完成</button>
          <template v-if="!editMode">
            <button class="btn btn-sm" @click="showNewGroup = true">+ 分组</button>
            <button class="btn btn-primary btn-sm" @click="showIngest = true">+ 导入</button>
          </template>
        </div>
      </div>
      <div class="sidebar-search">
        <input v-model="filterVendor" class="input" placeholder="按 vendor 过滤..." @input="debouncedLoad" style="margin-bottom:6px" />
        <input v-model="filterTag" class="input" placeholder="按 tag 过滤..." @input="debouncedLoad" />
      </div>
      <div class="sidebar-list">
        <!-- 分组 -->
        <div v-for="group in groups" :key="group.id" class="group-section"
          @dragover.prevent @drop="onDropToGroup($event, group.id)">
          <div class="group-header" @click="toggleGroup(group.id)">
            <div v-if="editMode" class="edit-cb" :class="{ checked: isGroupAllSelected(group.id) }" @click.stop="toggleGroupDocsSelect(group.id)"></div>
            <span class="group-chevron">{{ collapsedGroups.has(group.id) ? '▶' : '▼' }}</span>
            <span v-if="editingGroupId !== group.id" class="group-name" @dblclick.stop="startRename(group)">{{ group.name }}</span>
            <input v-else class="group-name-input" v-model="editingGroupName"
              @blur="commitRename(group)" @keyup.enter="commitRename(group)" @keyup.escape="editingGroupId = null"
              @click.stop ref="renameInputRef" />
            <button class="group-del" @click.stop="removeGroup(group)" title="删除分组">×</button>
          </div>
          <template v-if="!collapsedGroups.has(group.id)">
            <div v-for="doc in groupedDocs(group.id)" :key="doc.id"
              class="doc-row" :class="{ selected: activeDoc?.id === doc.id }"
              :draggable="!editMode" @dragstart="!editMode && onDragStart($event, doc)"
              @click="activeDoc = doc; searched = false">
              <div v-if="editMode" class="edit-cb" :class="{ checked: selectedDocIds.has(doc.id) }" @click.stop="toggleDocSelect(doc.id)"></div>
              <div class="doc-row-title">{{ docTitle(doc) }}</div>
              <div class="doc-row-meta">
                <span class="badge">{{ doc.vendor }}</span>
                <span v-for="t in doc.tags" :key="t" class="tag">{{ t }}</span>
              </div>
            </div>
            <div v-if="groupedDocs(group.id).length === 0" class="group-empty">拖入文档</div>
          </template>
        </div>

        <!-- 未分组 -->
        <div class="group-section" @dragover.prevent @drop="onDropToGroup($event, null)">
          <div class="group-header" @click="toggleGroup(0)">
            <div v-if="editMode" class="edit-cb" :class="{ checked: isGroupAllSelected(null) }" @click.stop="toggleGroupDocsSelect(null)"></div>
            <span class="group-chevron">{{ collapsedGroups.has(0) ? '▶' : '▼' }}</span>
            <span class="group-name">未分组</span>
          </div>
          <template v-if="!collapsedGroups.has(0)">
            <div v-for="doc in ungroupedDocs" :key="doc.id"
              class="doc-row" :class="{ selected: activeDoc?.id === doc.id }"
              :draggable="!editMode" @dragstart="!editMode && onDragStart($event, doc)"
              @click="activeDoc = doc; searched = false">
              <div v-if="editMode" class="edit-cb" :class="{ checked: selectedDocIds.has(doc.id) }" @click.stop="toggleDocSelect(doc.id)"></div>
              <div class="doc-row-title">{{ docTitle(doc) }}</div>
              <div class="doc-row-meta">
                <span class="badge">{{ doc.vendor }}</span>
                <span v-for="t in doc.tags" :key="t" class="tag">{{ t }}</span>
              </div>
            </div>
            <div v-if="ungroupedDocs.length === 0 && docs.length > 0" class="group-empty">拖入文档</div>
          </template>
        </div>

        <div v-if="docs.length === 0" class="sidebar-empty">暂无文档</div>
      </div>
    <div v-if="editMode" class="edit-bottom-bar">
      <span class="edit-sel-label">{{ selectionLabel }}</span>
      <div style="display:flex;align-items:center;gap:8px">
        <span v-if="deleteErr" class="err" style="font-size:11px">{{ deleteErr }}</span>
        <button class="btn btn-sm btn-danger" :disabled="!hasSelection || deleting" @click="onBatchDelete">删除</button>
      </div>
    </div>
    </aside>

    <div class="kb-detail">
      <div class="kb-search-bar">
        <input v-model="searchQuery" class="input" placeholder="语义搜索文档内容..." @keyup.enter="doSearch" />
        <button class="btn btn-primary" :disabled="searching" @click="doSearch">
          {{ searching ? '搜索中…' : '搜索' }}
        </button>
      </div>

      <div v-if="searched" class="kb-results">
        <div class="results-header">
          <span class="section-title" style="margin-bottom:0">搜索结果</span>
          <span class="dim">{{ searchResults.length }} 条</span>
          <button class="btn btn-sm" style="margin-left:auto" @click="searched = false">清除</button>
        </div>
        <div v-for="r in searchResults" :key="r.id" class="result-card" @click="activeDoc = r; searched = false">
          <div class="result-title">{{ docTitle(r) }}</div>
          <div class="result-meta">
            <span class="badge">{{ r.vendor }}</span>
            <span v-for="t in r.tags" :key="t" class="tag">{{ t }}</span>
          </div>
          <div class="result-content">{{ r.content.slice(0, 200) }}{{ r.content.length > 200 ? '…' : '' }}</div>
        </div>
        <div v-if="searchResults.length === 0" class="detail-empty">
          <div class="detail-empty-icon">🔍</div>
          <div>无匹配结果</div>
        </div>
      </div>

      <div v-else-if="activeDoc" style="flex:1;display:flex;flex-direction:column;overflow:hidden;min-height:0">
        <div class="detail-topbar">
          <div class="detail-topbar-left">
            <span class="detail-title">{{ docTitle(activeDoc) }}</span>
            <span class="badge">{{ activeDoc.vendor }}</span>
            <span v-for="t in activeDoc.tags" :key="t" class="tag">{{ t }}</span>
          </div>
          <div class="detail-topbar-right">
            <button class="btn btn-sm btn-danger" @click="remove(activeDoc)">删除</button>
          </div>
        </div>
        <div class="detail-body">
          <div class="detail-meta">
            <span>来源: <strong>{{ activeDoc.source_file }}</strong></span>
            <span>块: <strong>{{ activeDoc.chunk_index }}</strong></span>
            <span class="dim">{{ fmtTime(activeDoc.created_at) }}</span>
          </div>
          <div class="output">{{ activeDoc.content }}</div>
        </div>
      </div>

      <div v-else class="detail-empty">
        <div class="detail-empty-icon">📚</div>
        <div>选择左侧文档，或输入关键词语义搜索</div>
      </div>
    </div>

    <!-- 新建分组弹窗 -->
    <div v-if="showNewGroup" class="modal-overlay" @click.self="showNewGroup = false">
      <div class="modal" style="max-width:360px">
        <h3>新建分组</h3>
        <div class="form-row">
          <label>名称</label>
          <input v-model="newGroupName" class="input" @keyup.enter="doCreateGroup" />
        </div>
        <div class="modal-footer">
          <button class="btn" @click="showNewGroup = false">取消</button>
          <button class="btn btn-primary" :disabled="!newGroupName.trim()" @click="doCreateGroup">创建</button>
        </div>
      </div>
    </div>

    <!-- 导入弹窗 -->
    <div v-if="showIngest" class="modal-overlay" @click.self="showIngest = false">
      <div class="modal">
        <h3>导入文档</h3>
        <div class="form-row">
          <label>文件</label>
          <div class="file-picker-row">
            <span class="input file-picker-value">{{ pendingFiles.length ? `已选 ${pendingFiles.length} 个文件` : '未选择' }}</span>
            <button class="btn" type="button" @click="fileInputRef?.click()">选择文件</button>
            <input ref="fileInputRef" type="file" multiple accept=".txt,.md,.log,.conf,.cfg,.xml,.json,.yaml,.yml,.pdf" style="display:none" @change="onFileSelected" />
          </div>
        </div>
        <div class="form-row">
          <label>Vendor</label>
          <input v-model="form.vendor" class="input" placeholder="如 h3c" />
        </div>
        <div class="form-row">
          <label>分组</label>
          <select v-model="ingestGroupId" class="input">
            <option :value="null">未分组</option>
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
        </div>
        <div class="form-row">
          <label>
            <input type="checkbox" v-model="form.useEmbedding" />
            使用 Embedding（语义搜索，需配置 Embedding 模型）
          </label>
        </div>
        <div v-if="ingestErr" class="err" style="margin-bottom:8px">{{ ingestErr }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showIngest = false">取消</button>
          <button class="btn btn-primary" :disabled="ingesting" @click="doIngest">
            {{ ingesting ? '导入中…' : '导入' }}
          </button>
        </div>
      </div>
    </div>
    <!-- 批量删除分组确认弹窗 -->
    <div v-if="showDeleteGroupConfirm" class="modal-overlay" @click.self="showDeleteGroupConfirm = false">
      <div class="modal" style="max-width:360px">
        <h3>删除分组</h3>
        <p style="color:var(--text-sub);margin-bottom:12px">
          将删除 {{ selectedGroupIds.size }} 个分组，请选择组内文档处理方式：
        </p>
        <div class="form-row" style="flex-direction:column;gap:8px">
          <label class="radio-opt" :class="{ active: deleteGroupWithDocs }">
            <input type="radio" :value="true" v-model="deleteGroupWithDocs" />
            同时删除组内所有文档
          </label>
          <label class="radio-opt" :class="{ active: !deleteGroupWithDocs }">
            <input type="radio" :value="false" v-model="deleteGroupWithDocs" />
            将文档移至未分组
          </label>
        </div>
        <div v-if="deleteErr" class="err" style="margin-top:8px">{{ deleteErr }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showDeleteGroupConfirm = false; deleteErr = ''">取消</button>
          <button class="btn btn-danger" :disabled="deleting" @click="doDeleteGroups">
            {{ deleting ? '删除中…' : '确认删除' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import {
  listDocuments, ingestDocument, deleteDocument, searchDocuments, moveDocument,
  listGroups, createGroup, renameGroup, deleteGroup,
  deleteBatchDocuments, deleteBatchGroups,
  type Document, type DocumentGroup
} from '../api/documents'

const fileInputRef = ref<HTMLInputElement | null>(null)
const renameInputRef = ref<HTMLInputElement | null>(null)
const pendingFiles = ref<File[]>([])


async function readFileContent(file: File): Promise<{ content: string; pages: string[] }> {
  if (file.name.toLowerCase().endsWith('.pdf')) {
    const pdfjsLib = await import('pdfjs-dist')
    pdfjsLib.GlobalWorkerOptions.workerSrc = new URL('pdfjs-dist/build/pdf.worker.mjs', import.meta.url).href
    const buf = await file.arrayBuffer()
    const pdf = await pdfjsLib.getDocument({ data: buf }).promise
    const pages: string[] = []
    for (let i = 1; i <= pdf.numPages; i++) {
      const page = await pdf.getPage(i)
      const tc = await page.getTextContent()
      pages.push(tc.items.map((it: any) => it.str).join(' '))
    }
    return { content: pages[0] ?? '', pages }
  }
  return new Promise(resolve => {
    const reader = new FileReader()
    reader.onload = (ev) => resolve({ content: ev.target?.result as string ?? '', pages: [] })
    reader.readAsText(file)
  })
}

function onFileSelected(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files || files.length === 0) return
  pendingFiles.value = Array.from(files)
}

const docTitle = (doc: Document) => doc.title || doc.source_file

const docs = ref<Document[]>([])
const groups = ref<DocumentGroup[]>([])
const activeDoc = ref<Document | null>(null)
const filterVendor = ref('')
const filterTag = ref('')
const ingestGroupId = ref<number | null>(null)
const searchQuery = ref('')
const searchResults = ref<Document[]>([])
const searched = ref(false)
const searching = ref(false)
const showIngest = ref(false)
const ingesting = ref(false)
const ingestErr = ref('')
const collapsedGroups = ref(new Set<number>())
const showNewGroup = ref(false)
const newGroupName = ref('')
const editingGroupId = ref<number | null>(null)
const editingGroupName = ref('')
let dragDoc: Document | null = null

const editMode = ref(false)
const selectedDocIds = ref<Set<number>>(new Set())
const showDeleteGroupConfirm = ref(false)
const deleteGroupWithDocs = ref(true)
const deleting = ref(false)
const deleteErr = ref('')

function enterEditMode() { editMode.value = true }

function exitEditMode() {
  editMode.value = false
  selectedDocIds.value = new Set()
}

function toggleDocSelect(id: number) {
  const s = new Set(selectedDocIds.value)
  s.has(id) ? s.delete(id) : s.add(id)
  selectedDocIds.value = s
}

function toggleGroupDocsSelect(groupId: number | null) {
  const groupDocs = groupId === null ? ungroupedDocs.value : groupedDocs(groupId)
  if (groupDocs.length === 0) return
  const allSelected = groupDocs.every(d => selectedDocIds.value.has(d.id))
  const s = new Set(selectedDocIds.value)
  if (allSelected) {
    groupDocs.forEach(d => s.delete(d.id))
  } else {
    groupDocs.forEach(d => s.add(d.id))
  }
  selectedDocIds.value = s
}

function isGroupAllSelected(groupId: number | null): boolean {
  const groupDocs = groupId === null ? ungroupedDocs.value : groupedDocs(groupId)
  return groupDocs.length > 0 && groupDocs.every(d => selectedDocIds.value.has(d.id))
}

const hasSelection = computed(() => selectedDocIds.value.size > 0)

const selectionLabel = computed(() => {
  const d = selectedDocIds.value.size
  if (d === 0) return '未选择'
  return `已选 ${d} 个文档`
})

async function onBatchDelete() {
  deleteErr.value = ''
  await doDeleteDocs()
}

async function doDeleteDocs() {
  deleting.value = true
  deleteErr.value = ''
  try {
    await deleteBatchDocuments([...selectedDocIds.value])
    await load()
    exitEditMode()
  } catch (e: any) {
    deleteErr.value = e.message
  } finally {
    deleting.value = false
  }
}

async function doDeleteGroups() {
  deleting.value = true
  deleteErr.value = ''
  try {
    await deleteBatchGroups([...selectedGroupIds.value], deleteGroupWithDocs.value)
    await load()
    await loadGroups()
    exitEditMode()
    showDeleteGroupConfirm.value = false
  } catch (e: any) {
    deleteErr.value = e.message
  } finally {
    deleting.value = false
  }
}

const emptyForm = () => ({ vendor: '', useEmbedding: false })
const form = ref(emptyForm())

let loadTimer: ReturnType<typeof setTimeout> | null = null
function debouncedLoad() {
  if (loadTimer) clearTimeout(loadTimer)
  loadTimer = setTimeout(load, 300)
}

const docsByGroup = computed(() => {
  const map = new Map<number | null, Document[]>()
  for (const d of docs.value) {
    const key = d.group_id ?? null
    if (!map.has(key)) map.set(key, [])
    map.get(key)!.push(d)
  }
  return map
})
const groupedDocs = (groupId: number) => docsByGroup.value.get(groupId) ?? []
const ungroupedDocs = computed(() => docsByGroup.value.get(null) ?? [])

function toggleGroup(id: number) {
  const s = new Set(collapsedGroups.value)
  if (s.has(id)) s.delete(id); else s.add(id)
  collapsedGroups.value = s
}

function onDragStart(e: DragEvent, doc: Document) {
  dragDoc = doc
  e.dataTransfer?.setData('text/plain', String(doc.id))
}

async function onDropToGroup(_e: DragEvent, groupId: number | null) {
  if (!dragDoc || dragDoc.group_id === groupId) { dragDoc = null; return }
  const doc = dragDoc; dragDoc = null
  doc.group_id = groupId
  await moveDocument(doc.id, groupId)
}

async function load() {
  docs.value = await listDocuments(filterVendor.value || undefined, filterTag.value || undefined)
}

async function loadGroups() {
  groups.value = await listGroups() ?? []
}

async function doCreateGroup() {
  if (!newGroupName.value.trim()) return
  const g = await createGroup(newGroupName.value.trim())
  groups.value.push(g)
  newGroupName.value = ''
  showNewGroup.value = false
}

function startRename(group: DocumentGroup) {
  editingGroupId.value = group.id
  editingGroupName.value = group.name
  nextTick(() => (renameInputRef.value as any)?.[0]?.focus())
}

async function commitRename(group: DocumentGroup) {
  const name = editingGroupName.value.trim()
  editingGroupId.value = null
  if (!name || name === group.name) return
  await renameGroup(group.id, name)
  group.name = name
}

async function removeGroup(group: DocumentGroup) {
  if (!confirm(`删除分组「${group.name}」？分组内文档将移至未分组。`)) return
  await deleteGroup(group.id)
  docs.value.forEach(d => { if (d.group_id === group.id) d.group_id = null })
  groups.value = groups.value.filter(g => g.id !== group.id)
}

async function remove(doc: Document) {
  if (!confirm(`确认删除「${docTitle(doc)}」？`)) return
  await deleteDocument(doc.id)
  if (activeDoc.value?.id === doc.id) activeDoc.value = null
  load()
}

async function doSearch() {
  if (!searchQuery.value.trim()) return
  searching.value = true
  try {
    searchResults.value = await searchDocuments(searchQuery.value, filterVendor.value || undefined)
    searched.value = true
  } finally {
    searching.value = false
  }
}

async function doIngest() {
  ingestErr.value = ''
  if (pendingFiles.value.length === 0) { ingestErr.value = '请先选择文件'; return }
  ingesting.value = true
  try {
    for (const file of pendingFiles.value) {
      const { content, pages } = await readFileContent(file)
      const fullContent = pages.length > 0 ? pages.join('\n\n') : content
      if (fullContent.trim()) {
        await ingestDocument({ vendor: form.value.vendor, content: fullContent, source_file: file.name, chunk_index: 0, group_id: ingestGroupId.value, use_embedding: form.value.useEmbedding })
      }
    }
    showIngest.value = false
    form.value = emptyForm()
    pendingFiles.value = []
    ingestGroupId.value = null
    load()
  } catch (e: any) {
    ingestErr.value = e.message
  } finally {
    ingesting.value = false
  }
}

function fmtTime(s: string) {
  return new Date(s).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

onMounted(() => { load(); loadGroups() })
</script>

<style scoped>
.kb-page { display: flex; height: 100%; overflow: hidden; }

.kb-sidebar {
  width: 26%; min-width: 260px; max-width: 340px;
  background: var(--panel); border-right: 1px solid var(--border);
  display: flex; flex-direction: column; flex-shrink: 0; overflow: hidden;
}

.sidebar-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0;
}

.sidebar-title { font-size: 13px; font-weight: 700; color: var(--text); display: flex; align-items: center; gap: 6px; }
.doc-count { background: rgba(99,102,241,0.15); color: var(--primary); border-radius: 10px; padding: 1px 7px; font-size: 11px; font-weight: 600; }
.sidebar-search { padding: 10px 12px 8px; flex-shrink: 0; }
.sidebar-list { flex: 1; overflow-y: auto; }
.sidebar-empty { color: var(--label); font-size: 13px; padding: 32px 16px; text-align: center; }

.group-section { border-bottom: 1px solid var(--border); }
.group-section[dragover] { background: rgba(99,102,241,0.06); }

.group-header {
  display: flex; align-items: center; gap: 6px;
  padding: 7px 12px; cursor: pointer; user-select: none;
  background: var(--surface); font-size: 12px; font-weight: 600; color: var(--text-sub);
}
.group-header:hover { background: var(--row-hover); }
.group-chevron { font-size: 10px; color: var(--muted); width: 12px; flex-shrink: 0; }
.group-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.group-name-input { flex: 1; background: var(--input-bg); border: 1px solid var(--primary); border-radius: 4px; padding: 1px 6px; font-size: 12px; color: var(--text); outline: none; }
.group-del { margin-left: auto; background: none; border: none; color: var(--muted); cursor: pointer; font-size: 14px; line-height: 1; padding: 0 2px; }
.group-del:hover { color: #dc2626; }
.group-empty { font-size: 12px; color: var(--muted); padding: 8px 16px; font-style: italic; }

.doc-row {
  padding: 8px 16px; cursor: pointer; border-left: 3px solid transparent;
  transition: background 0.1s;
}
.doc-row:hover { background: var(--row-hover); }
.doc-row.selected { border-left-color: var(--primary); background: rgba(99,102,241,0.08); }
.doc-row[draggable="true"] { cursor: grab; }
.doc-row-title { font-size: 13px; font-weight: 600; color: var(--text); margin-bottom: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.doc-row-meta { display: flex; gap: 4px; align-items: center; flex-wrap: wrap; }

.kb-detail { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-width: 0; }

.kb-search-bar {
  display: flex; gap: 8px; padding: 12px 16px;
  border-bottom: 1px solid var(--border); flex-shrink: 0; background: var(--surface);
}
.kb-search-bar .input { flex: 1; }

.kb-results { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 10px; }
.results-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; flex-shrink: 0; }

.result-card {
  background: var(--card-bg); border: 1px solid var(--border); border-radius: 8px;
  padding: 12px 14px; cursor: pointer; transition: border-color 0.15s, box-shadow 0.15s;
}
.result-card:hover { border-color: var(--primary); box-shadow: 0 0 0 3px rgba(99,102,241,0.08); }
.result-title { font-size: 13px; font-weight: 600; color: var(--text); margin-bottom: 4px; }
.result-meta { display: flex; gap: 4px; margin-bottom: 6px; flex-wrap: wrap; }
.result-content { font-size: 12px; color: var(--text-sub); line-height: 1.5; }

.detail-topbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0; background: var(--surface);
}
.detail-topbar-left { display: flex; align-items: center; gap: 8px; min-width: 0; }
.detail-topbar-right { display: flex; gap: 8px; flex-shrink: 0; }
.detail-title { font-size: 15px; font-weight: 700; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.detail-body { flex: 1; display: flex; flex-direction: column; overflow: hidden; padding: 20px; }
.output { flex: 1; overflow-y: auto; white-space: pre-wrap; word-break: break-all; font-family: monospace; font-size: 13px; background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 14px; min-height: 0; }
.detail-meta {
  display: flex; flex-wrap: wrap; gap: 16px; font-size: 13px; color: var(--text-sub);
  padding: 12px 14px; background: var(--surface); border: 1px solid var(--border);
  border-radius: 8px; margin-bottom: 16px;
}
.detail-empty {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px; color: var(--label); font-size: 14px;
}
.detail-empty-icon { font-size: 36px; opacity: 0.5; }

.file-picker-row { display: flex; gap: 8px; align-items: center; }
.file-picker-value { flex: 1; color: var(--text-sub); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tag { background: rgba(16,185,129,0.12); color: #10b981; border-radius: 4px; padding: 1px 6px; font-size: 11px; font-weight: 500; }

@media (max-width: 520px) {
  .file-picker-row { align-items: stretch; flex-direction: column; }
}

.btn-ghost { background: transparent; border-color: var(--border); color: var(--text-sub); }
.btn-ghost:hover { background: var(--row-hover); }

.edit-cb {
  width: 14px; height: 14px; border-radius: 3px; flex-shrink: 0;
  border: 1.5px solid var(--border); background: var(--panel); cursor: pointer;
  display: flex; align-items: center; justify-content: center;
}
.edit-cb.checked { background: var(--primary); border-color: var(--primary); }
.edit-cb.checked::after { content: '✓'; font-size: 9px; color: #fff; }

.edit-bottom-bar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 12px; border-top: 1px solid var(--border);
  background: var(--surface); flex-shrink: 0;
}
.edit-sel-label { font-size: 11px; color: var(--text-sub); }

.radio-opt { display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 13px; padding: 6px 8px; border-radius: 6px; border: 1.5px solid var(--border); }
.radio-opt.active { border-color: var(--primary); background: rgba(99,102,241,0.06); }
</style>
