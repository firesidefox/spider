<template>
  <div class="fullscreen-page kb-page">
    <aside class="kb-sidebar">
      <div class="sidebar-toolbar">
        <span class="sidebar-title">知识库</span>
        <div class="sidebar-actions">
          <button v-if="!selectMode" class="btn btn-sm" @click="enterSelectMode">选择</button>
          <button v-else class="btn btn-sm" @click="exitSelectMode">完成</button>
          <button v-if="!selectMode" class="btn btn-primary btn-sm" @click="showNewGroup = true">+ 分组</button>
          <button v-if="!selectMode" class="btn btn-primary btn-sm" @click="showGlobalImport = true">+ 导入</button>
        </div>
      </div>
      <div class="sidebar-filters">
        <input v-model="searchQuery" class="filter-input" placeholder="语义搜索文档内容..." />
        <input v-model="vendorFilter" class="filter-input" placeholder="按 vendor 过滤..." />
        <input v-model="tagFilter" class="filter-input" placeholder="按 tag 过滤..." />
      </div>
      <div v-if="selectMode" class="select-toolbar">
        <span class="select-info">{{ selectionLabel }}</span>
        <div class="select-menu-wrap">
          <button class="btn btn-sm btn-primary" :disabled="!hasSelection" @click="editMenuOpen = !editMenuOpen">
            编辑 ▾
          </button>
          <div v-if="editMenuOpen" class="edit-menu" @click.stop>
            <button class="edit-menu-item" :disabled="!docsSelectedCount" @click="onMoveDocs">
              移动到分组（{{ docsSelectedCount }}）
            </button>
            <button class="edit-menu-item" :disabled="!docsSelectedCount" @click="onReindexDocs">
              重建索引（{{ docsSelectedCount }}）
            </button>
            <button class="edit-menu-item danger" :disabled="!hasSelection" @click="onBatchDelete">
              删除选中
            </button>
          </div>
        </div>
      </div>
      <div class="sidebar-list">
        <div v-for="group in groups" :key="group.id" class="group-section">
          <div class="group-header" @click="toggleGroup(group.id)">
            <input v-if="selectMode" type="checkbox" class="row-check"
              :checked="selectedGroups.has(group.id)"
              @click.stop="toggleSelectGroup(group.id)" />
            <span class="chevron">{{ collapsedGroups.has(group.id) ? '▶' : '▼' }}</span>
            <span class="group-name">{{ group.name }}</span>
            <template v-if="!selectMode">
              <button class="del-btn" @click.stop="doDeleteGroup(group)" title="删除分组">×</button>
              <button class="add-btn" @click.stop="openImport(group.id)" title="导入文档">+</button>
            </template>
          </div>
          <template v-if="!collapsedGroups.has(group.id)">
            <div v-for="doc in docsByGroup[group.id] ?? []" :key="doc.id"
              class="doc-row" :class="{ selected: !selectMode && activeDoc?.id === doc.id }"
              @click="onDocRowClick(doc)">
              <input v-if="selectMode" type="checkbox" class="row-check"
                :checked="selectedDocs.has(doc.id)"
                @click.stop="toggleSelectDoc(doc.id)" />
              <span class="doc-name">{{ doc.name }}</span>
              <span class="doc-status" :class="doc.status">{{ doc.status }}</span>
            </div>
            <div v-if="!(docsByGroup[group.id]?.length)" class="group-empty">暂无文档</div>
          </template>
        </div>
        <div v-if="groups.length === 0" class="sidebar-empty">暂无分组</div>
      </div>
    </aside>

    <div class="kb-detail">
      <div v-if="activeDoc" style="flex:1;display:flex;flex-direction:column;overflow:hidden;min-height:0">
        <div class="detail-topbar">
          <span class="detail-title">{{ activeDoc.name }}</span>
          <span class="doc-status" :class="activeDoc.status">{{ activeDoc.status }}</span>
          <span v-if="activeDoc.entry_count" class="entry-count">{{ activeDoc.entry_count }} 条目</span>
          <button class="btn btn-sm" style="margin-left:auto"
            :disabled="reindexing" @click="doReindex(activeDoc)">
            {{ reindexing ? '重建中…' : '重建索引' }}
          </button>
          <button class="btn btn-sm btn-danger"
            @click="doDeleteDoc(activeDoc)">删除</button>
        </div>
        <div v-if="activeDoc.error_msg" class="detail-error">{{ activeDoc.error_msg }}</div>
        <div v-else class="detail-body">
          <div class="detail-meta-line">
            来源: {{ activeDoc.name }} 块: {{ activeDoc.entry_count }} {{ formatDate(activeDoc.created_at) }}
          </div>
          <div v-if="loadingSections" class="detail-content-placeholder">
            <div class="detail-meta-text">加载中...</div>
          </div>
          <div v-else-if="sections.length > 0" class="detail-sections">
            <div v-for="section in sections" :key="section.id" class="section-item">
              <div class="section-header" @click="toggleSection(section.id)">
                <span class="chevron">{{ expandedSections.has(section.id) ? '▼' : '▶' }}</span>
                <span class="section-name">{{ section.name }}</span>
                <span class="section-count">{{ section.entry_count }} 条目</span>
              </div>
              <div v-if="expandedSections.has(section.id)" class="section-entries">
                <div v-if="loadingEntries.has(section.id)" class="entry-loading">加载中...</div>
                <div v-else-if="entriesBySection[section.id]?.length" class="entry-list">
                  <div v-for="entry in entriesBySection[section.id]" :key="entry.id"
                    class="entry-item" :class="{ active: activeEntry?.id === entry.id }"
                    @click="selectEntry(entry)">
                    <div class="entry-title">{{ entry.title }}</div>
                    <div class="entry-summary">{{ entry.summary }}</div>
                  </div>
                </div>
                <div v-else class="entry-empty">无条目</div>
              </div>
            </div>
          </div>
          <div v-else class="detail-content-placeholder">
            <div class="detail-meta-icon">📄</div>
            <div class="detail-meta-text">{{ activeDoc.doc_type }} · {{ activeDoc.entry_count }} 条目已索引</div>
            <div class="detail-hint">无章节</div>
          </div>
        </div>
      </div>
      <div v-else class="detail-empty">
        <div class="detail-empty-icon">📚</div>
        <div>选择左侧文档查看详情</div>
      </div>
    </div>

    <!-- 新建分组 -->
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

    <!-- 移动到分组 -->
    <div v-if="showMove" class="modal-overlay" @click.self="showMove = false">
      <div class="modal" style="max-width:400px">
        <h3>移动 {{ docsSelectedCount }} 个文档到</h3>
        <div class="form-row">
          <label>目标分组</label>
          <select v-model.number="moveTargetGroupID" class="input">
            <option :value="0" disabled>选择分组…</option>
            <option v-for="g in groups" :key="g.id" :value="g.id">
              {{ g.name }}
            </option>
          </select>
        </div>
        <div v-if="batchErr" class="err" style="margin-top:8px">{{ batchErr }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showMove = false">取消</button>
          <button class="btn btn-primary" :disabled="!moveTargetGroupID || batching" @click="doMove">
            {{ batching ? '移动中…' : '移动' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 导入弹窗 -->
    <div v-if="showImport || showGlobalImport" class="modal-overlay" @click.self="closeImport">
      <div class="modal">
        <h3>导入文档</h3>
        <div v-if="showGlobalImport" class="form-row">
          <label>目标分组</label>
          <select v-model.number="importGroupID" class="input">
            <option :value="0" disabled>选择分组…</option>
            <option v-for="g in groups" :key="g.id" :value="g.id">
              {{ g.name }}
            </option>
          </select>
        </div>
        <div class="file-drop-zone"
          @dragover.prevent @drop.prevent="onDrop"
          @click="fileInputRef?.click()">
          <div v-if="importFiles.length === 0" class="drop-hint">拖拽文件到此处，或点击选择（.yaml/.yml/.json/.md）</div>
          <div v-else class="file-list">
            <div v-for="(f, i) in importFiles" :key="i" class="file-item">
              <span class="file-item-name">{{ f.file.name }}</span>
              <span class="file-item-status" :class="f.status">{{ f.statusText }}</span>
            </div>
          </div>
          <input ref="fileInputRef" type="file" multiple accept=".yaml,.yml,.json,.md"
            style="display:none" @change="onFileSelected" />
        </div>
        <div v-if="importErr" class="err" style="margin-top:8px">{{ importErr }}</div>
        <div class="modal-footer">
          <button class="btn" @click="closeImport">取消</button>
          <button class="btn btn-primary" :disabled="importing || importFiles.length === 0 || (showGlobalImport && !importGroupID)" @click="doImport">
            {{ importing ? '导入中…' : '导入' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import {
  listGroups, createGroup, deleteGroup, deleteGroups,
  listDocuments, getDocument, getSections, getEntries, deleteDocuments, importDocument, moveDocuments, reindexDocuments,
  type KnowledgeGroup, type KnowledgeDocument, type KnowledgeSection, type KnowledgeEntry,
} from '../api/knowledge'

const fileInputRef = ref<HTMLInputElement | null>(null)

const groups = ref<KnowledgeGroup[]>([])
const docsByGroup = ref<Record<number, KnowledgeDocument[]>>({})
const activeDoc = ref<KnowledgeDocument | null>(null)
const docContent = ref<string>('')
const loadingContent = ref(false)
const collapsedGroups = ref(new Set<number>())

const sections = ref<KnowledgeSection[]>([])
const loadingSections = ref(false)
const expandedSections = ref(new Set<number>())
const entriesBySection = ref<Record<number, KnowledgeEntry[]>>({})
const loadingEntries = ref(new Set<number>())
const activeEntry = ref<KnowledgeEntry | null>(null)

const showNewGroup = ref(false)
const newGroupName = ref('')

const showImport = ref(false)
const importGroupID = ref(0)
const importing = ref(false)
const importErr = ref('')

interface ImportFile { file: File; status: 'pending' | 'ok' | 'error'; statusText: string }
const importFiles = ref<ImportFile[]>([])

const selectMode = ref(false)
const editMenuOpen = ref(false)
const selectedGroups = ref(new Set<number>())
const selectedDocs = ref(new Set<number>())
const showMove = ref(false)
const moveTargetGroupID = ref(0)
const batching = ref(false)
const batchErr = ref('')
const reindexing = ref(false)

const searchQuery = ref('')
const vendorFilter = ref('')
const tagFilter = ref('')
const showGlobalImport = ref(false)

const docsSelectedCount = computed(() => selectedDocs.value.size)
const hasSelection = computed(() =>
  selectedDocs.value.size + selectedGroups.value.size > 0
)
const selectionLabel = computed(() => {
  const d = selectedDocs.value.size, g = selectedGroups.value.size
  if (d === 0 && g === 0) return '未选择'
  const parts: string[] = []
  if (g) parts.push(`${g} 分组`)
  if (d) parts.push(`${d} 文档`)
  return '已选 ' + parts.join(' / ')
})

function enterSelectMode() {
  selectMode.value = true
  editMenuOpen.value = false
  clearSelection()
}
function exitSelectMode() {
  selectMode.value = false
  editMenuOpen.value = false
  clearSelection()
}
function clearSelection() {
  selectedGroups.value = new Set()
  selectedDocs.value = new Set()
}
function toggleInSet(set: import('vue').Ref<Set<number>>, id: number) {
  const s = new Set(set.value)
  s.has(id) ? s.delete(id) : s.add(id)
  set.value = s
}
function toggleSelectGroup(id: number) { toggleInSet(selectedGroups, id) }
function toggleSelectDoc(id: number) { toggleInSet(selectedDocs, id) }

function onDocRowClick(doc: KnowledgeDocument) {
  if (selectMode.value) {
    toggleSelectDoc(doc.id)
  } else {
    activeDoc.value = doc
  }
}

watch(activeDoc, async (newDoc) => {
  if (!newDoc) {
    sections.value = []
    expandedSections.value = new Set()
    entriesBySection.value = {}
    activeEntry.value = null
    return
  }
  loadingSections.value = true
  try {
    sections.value = await getSections(newDoc.id)
  } catch (e: any) {
    sections.value = []
  } finally {
    loadingSections.value = false
  }
})

function toggleGroup(id: number) {
  const s = new Set(collapsedGroups.value)
  s.has(id) ? s.delete(id) : s.add(id)
  collapsedGroups.value = s
  if (!s.has(id) && !docsByGroup.value[id]) loadDocs(id)
}

async function toggleSection(sectionID: number) {
  const s = new Set(expandedSections.value)
  if (s.has(sectionID)) {
    s.delete(sectionID)
  } else {
    s.add(sectionID)
    if (!entriesBySection.value[sectionID]) {
      const loading = new Set(loadingEntries.value)
      loading.add(sectionID)
      loadingEntries.value = loading
      try {
        const entries = await getEntries(sectionID)
        entriesBySection.value = { ...entriesBySection.value, [sectionID]: entries }
      } catch (e: any) {
        entriesBySection.value = { ...entriesBySection.value, [sectionID]: [] }
      } finally {
        const loading = new Set(loadingEntries.value)
        loading.delete(sectionID)
        loadingEntries.value = loading
      }
    }
  }
  expandedSections.value = s
}

function selectEntry(entry: KnowledgeEntry) {
  activeEntry.value = entry
}

async function loadGroups() {
  groups.value = await listGroups()
}

async function loadDocs(groupID: number) {
  docsByGroup.value = { ...docsByGroup.value, [groupID]: await listDocuments(groupID) }
}

async function doCreateGroup() {
  if (!newGroupName.value.trim()) return
  const g = await createGroup(newGroupName.value.trim())
  groups.value.push(g)
  newGroupName.value = ''
  showNewGroup.value = false
}

async function doDeleteGroup(group: KnowledgeGroup) {
  if (!confirm(`删除分组「${group.name}」及其所有文档？`)) return
  await deleteGroup(group.id)
  groups.value = groups.value.filter(g => g.id !== group.id)
  const d = { ...docsByGroup.value }; delete d[group.id]; docsByGroup.value = d
}

async function doDeleteDoc(doc: KnowledgeDocument) {
  if (!confirm(`删除文档「${doc.name}」？`)) return
  await deleteDocuments([doc.id])
  docsByGroup.value = {
    ...docsByGroup.value,
    [doc.group_id]: (docsByGroup.value[doc.group_id] ?? []).filter(d => d.id !== doc.id),
  }
  if (activeDoc.value?.id === doc.id) activeDoc.value = null
}

function openImport(groupID: number) {
  importGroupID.value = groupID
  importFiles.value = []
  importErr.value = ''
  showImport.value = true
}

function closeImport() {
  showImport.value = false
  showGlobalImport.value = false
  importFiles.value = []
  importErr.value = ''
}

function onFileSelected(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files) return
  importFiles.value = Array.from(files).map(f => ({ file: f, status: 'pending', statusText: '待导入' }))
}

function onDrop(e: DragEvent) {
  const files = e.dataTransfer?.files
  if (!files) return
  importFiles.value = Array.from(files).map(f => ({ file: f, status: 'pending', statusText: '待导入' }))
}

async function doImport() {
  importing.value = true
  importErr.value = ''

  const results = await Promise.allSettled(
    importFiles.value.map(async item => {
      item.status = 'pending'
      item.statusText = '导入中…'
      try {
        await importDocument(importGroupID.value, item.file)
        item.status = 'ok'
        item.statusText = '成功'
      } catch (e: any) {
        item.status = 'error'
        item.statusText = e.message ?? '失败'
        throw e
      }
    })
  )

  const anyError = results.some(r => r.status === 'rejected')
  importing.value = false
  await loadDocs(importGroupID.value)
  if (!anyError) {
    closeImport()
  }
}

function onMoveDocs() {
  editMenuOpen.value = false
  batchErr.value = ''
  moveTargetGroupID.value = 0
  showMove.value = true
}

async function doMove() {
  if (!moveTargetGroupID.value) return
  batching.value = true
  batchErr.value = ''
  try {
    const ids = Array.from(selectedDocs.value)
    await moveDocuments(ids, moveTargetGroupID.value)
    const groupIDs = new Set<number>([moveTargetGroupID.value])
    for (const gid of Object.keys(docsByGroup.value)) {
      const arr = docsByGroup.value[Number(gid)] ?? []
      if (arr.some(d => selectedDocs.value.has(d.id))) groupIDs.add(Number(gid))
    }
    await Promise.all(Array.from(groupIDs).map(loadDocs))
    showMove.value = false
    exitSelectMode()
  } catch (e: any) {
    batchErr.value = e.message ?? '移动失败'
  } finally {
    batching.value = false
  }
}

async function onReindexDocs() {
  editMenuOpen.value = false
  if (!selectedDocs.value.size) return
  if (!confirm(`重建 ${selectedDocs.value.size} 个文档的索引？`)) return
  batching.value = true
  try {
    const ids = Array.from(selectedDocs.value)
    const resp = await reindexDocuments(ids)
    const errCount = Object.keys(resp.errors ?? {}).length
    if (errCount) alert(`完成。${errCount} 个文档失败，详情见状态。`)
    const groupIDs = new Set<number>()
    for (const gid of Object.keys(docsByGroup.value)) {
      const arr = docsByGroup.value[Number(gid)] ?? []
      if (arr.some(d => selectedDocs.value.has(d.id))) groupIDs.add(Number(gid))
    }
    await Promise.all(Array.from(groupIDs).map(loadDocs))
    exitSelectMode()
  } catch (e: any) {
    alert(e.message ?? '重建失败')
  } finally {
    batching.value = false
  }
}

async function onBatchDelete() {
  editMenuOpen.value = false
  const d = selectedDocs.value.size, g = selectedGroups.value.size
  if (!d && !g) return
  const parts: string[] = []
  if (g) parts.push(`${g} 个分组`)
  if (d) parts.push(`${d} 个文档`)
  if (!confirm(`删除 ${parts.join('、')}？此操作不可恢复。`)) return
  batching.value = true
  try {
    if (d) await deleteDocuments(Array.from(selectedDocs.value))
    if (g) await deleteGroups(Array.from(selectedGroups.value))
    if (activeDoc.value && selectedDocs.value.has(activeDoc.value.id)) activeDoc.value = null
    await loadGroups()
    docsByGroup.value = {}
    collapsedGroups.value = new Set()
    exitSelectMode()
  } catch (e: any) {
    alert(e.message ?? '删除失败')
  } finally {
    batching.value = false
  }
}

async function doReindex(doc: KnowledgeDocument) {
  if (!confirm(`重建文档「${doc.name}」的索引？`)) return
  reindexing.value = true
  try {
    const resp = await reindexDocuments([doc.id])
    const err = resp.errors?.[String(doc.id)]
    if (err) alert(`重建失败：${err}`)
    await loadDocs(doc.group_id)
    const fresh = (docsByGroup.value[doc.group_id] ?? []).find(d => d.id === doc.id)
    if (fresh) activeDoc.value = fresh
  } catch (e: any) {
    alert(e.message ?? '重建失败')
  } finally {
    reindexing.value = false
  }
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr)
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hour = String(d.getHours()).padStart(2, '0')
  const minute = String(d.getMinutes()).padStart(2, '0')
  return `${month}/${day} ${hour}:${minute}`
}

onMounted(loadGroups)
</script>

<style scoped>
.kb-page { display: flex; height: 100%; overflow: hidden; }

.kb-sidebar {
  width: 280px; min-width: 220px; max-width: 340px;
  background: var(--panel); border-right: 1px solid var(--border);
  display: flex; flex-direction: column; flex-shrink: 0; overflow: hidden;
}
.sidebar-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0;
}
.sidebar-title { font-size: 13px; font-weight: 700; color: var(--text); }
.sidebar-actions { display: flex; gap: 6px; }
.sidebar-filters {
  padding: 8px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0;
  display: flex; flex-direction: column; gap: 6px;
}
.filter-input {
  width: 100%; padding: 6px 10px; font-size: 12px;
  border: 1px solid var(--border); border-radius: 4px;
  background: var(--surface); color: var(--text);
}
.filter-input::placeholder { color: var(--muted); }
.select-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 12px; border-bottom: 1px solid var(--border);
  background: rgba(99,102,241,0.06); flex-shrink: 0;
}
.select-info { font-size: 12px; color: var(--text-sub); }
.select-menu-wrap { position: relative; }
.edit-menu {
  position: absolute; right: 0; top: 100%; margin-top: 4px;
  background: var(--panel); border: 1px solid var(--border); border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.15); min-width: 180px; z-index: 10;
  display: flex; flex-direction: column;
}
.edit-menu-item {
  text-align: left; background: none; border: none; padding: 8px 12px;
  font-size: 13px; color: var(--text); cursor: pointer;
}
.edit-menu-item:hover:not(:disabled) { background: var(--row-hover); }
.edit-menu-item:disabled { color: var(--muted); cursor: not-allowed; }
.edit-menu-item.danger { color: #dc2626; border-top: 1px solid var(--border); }
.row-check { margin: 0 4px 0 0; cursor: pointer; flex-shrink: 0; }
.sidebar-list { flex: 1; overflow-y: auto; }
.sidebar-empty { color: var(--label); font-size: 13px; padding: 32px 16px; text-align: center; }

.kb-section { border-bottom: 1px solid var(--border); }
.kb-header {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 12px; cursor: pointer; background: var(--surface);
  font-size: 12px; font-weight: 700; color: var(--text);
}
.kb-header:hover { background: var(--row-hover); }
.kb-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.group-section { padding-left: 12px; }
.group-header {
  display: flex; align-items: center; gap: 6px;
  padding: 6px 8px; cursor: pointer;
  font-size: 12px; font-weight: 600; color: var(--text-sub);
}
.group-header:hover { background: var(--row-hover); }
.group-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.group-empty { font-size: 12px; color: var(--muted); padding: 4px 16px; font-style: italic; }

.chevron { font-size: 10px; color: var(--muted); width: 12px; flex-shrink: 0; }
.del-btn { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 14px; padding: 0 2px; }
.del-btn:hover { color: #dc2626; }
.add-btn { background: none; border: none; color: var(--primary); cursor: pointer; font-size: 14px; padding: 0 2px; }
.add-group-btn {
  display: block; width: calc(100% - 24px); margin: 4px 12px;
  background: none; border: 1px dashed var(--border); border-radius: 4px;
  color: var(--text-sub); font-size: 11px; padding: 4px; cursor: pointer;
}
.add-group-btn:hover { border-color: var(--primary); color: var(--primary); }

.doc-row {
  padding: 5px 8px 5px 20px; cursor: pointer; display: flex; align-items: center; gap: 8px;
  border-left: 3px solid transparent;
}
.doc-row:hover { background: var(--row-hover); }
.doc-row.selected { border-left-color: var(--primary); background: rgba(99,102,241,0.08); }
.doc-name { flex: 1; font-size: 12px; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.doc-status { font-size: 10px; border-radius: 4px; padding: 1px 5px; flex-shrink: 0; }
.doc-status.ready { background: rgba(16,185,129,0.12); color: #10b981; }
.doc-status.indexing { background: rgba(245,158,11,0.12); color: #f59e0b; }
.doc-status.error { background: rgba(220,38,38,0.12); color: #dc2626; }
.doc-status.pending { background: rgba(107,114,128,0.12); color: #6b7280; }

.kb-detail { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-width: 0; }
.detail-topbar {
  display: flex; align-items: center; gap: 8px;
  padding: 14px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0; background: var(--surface);
}
.detail-title { font-size: 15px; font-weight: 700; color: var(--text); }
.entry-count { font-size: 12px; color: var(--text-sub); }
.detail-error { padding: 16px 20px; color: #dc2626; font-size: 13px; }
.detail-body {
  flex: 1; display: flex; flex-direction: column; overflow: hidden; min-height: 0;
}
.detail-meta-line {
  padding: 12px 20px; border-bottom: 1px solid var(--border);
  font-size: 12px; color: var(--text-sub); background: var(--surface);
}
.detail-content-placeholder {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px;
}
.detail-meta {
  display: flex; flex-direction: column; align-items: center; gap: 12px;
  color: var(--label); font-size: 14px;
}
.detail-meta-icon { font-size: 36px; opacity: 0.5; }
.detail-meta-text { text-align: center; color: var(--label); font-size: 14px; }
.detail-hint { font-size: 12px; color: var(--muted); font-style: italic; }
.detail-empty {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px; color: var(--label); font-size: 14px;
}
.detail-empty-icon { font-size: 36px; opacity: 0.5; }

.file-drop-zone {
  border: 2px dashed var(--border); border-radius: 8px; padding: 20px;
  cursor: pointer; min-height: 100px; display: flex; flex-direction: column;
  align-items: center; justify-content: center;
}
.file-drop-zone:hover { border-color: var(--primary); }
.drop-hint { color: var(--text-sub); font-size: 13px; text-align: center; }
.file-list { width: 100%; display: flex; flex-direction: column; gap: 6px; }
.file-item { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.file-item-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); }
.file-item-status { flex-shrink: 0; }
.file-item-status.ok { color: #10b981; }
.file-item-status.error { color: #dc2626; }
</style>

