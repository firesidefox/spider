<template>
  <div class="fullscreen-page kb-page">
    <aside class="kb-sidebar">
      <div class="sidebar-toolbar">
        <span class="sidebar-title">知识库</span>
        <button class="btn btn-primary btn-sm" @click="showNewKB = true">+ 知识库</button>
      </div>
      <div class="sidebar-list">
        <div v-for="kb in kbs" :key="kb.id" class="kb-section">
          <div class="kb-header" @click="toggleKB(kb.id)">
            <span class="chevron">{{ collapsedKBs.has(kb.id) ? '▶' : '▼' }}</span>
            <span class="kb-name">{{ kb.name }}</span>
            <button class="del-btn" @click.stop="doDeleteKB(kb)" title="删除知识库">×</button>
          </div>
          <template v-if="!collapsedKBs.has(kb.id)">
            <div v-for="group in groupsByKB[kb.id] ?? []" :key="group.id" class="group-section">
              <div class="group-header" @click="toggleGroup(group.id)">
                <span class="chevron">{{ collapsedGroups.has(group.id) ? '▶' : '▼' }}</span>
                <span class="group-name">{{ group.name }}</span>
                <button class="del-btn" @click.stop="doDeleteGroup(group)" title="删除分组">×</button>
                <button class="add-btn" @click.stop="openImport(group.id)" title="导入文档">+</button>
              </div>
              <template v-if="!collapsedGroups.has(group.id)">
                <div v-for="doc in docsByGroup[group.id] ?? []" :key="doc.id"
                  class="doc-row" :class="{ selected: activeDoc?.id === doc.id }"
                  @click="activeDoc = doc">
                  <span class="doc-name">{{ doc.name }}</span>
                  <span class="doc-status" :class="doc.status">{{ doc.status }}</span>
                </div>
                <div v-if="!(docsByGroup[group.id]?.length)" class="group-empty">暂无文档</div>
              </template>
            </div>
            <button class="add-group-btn" @click.stop="openNewGroup(kb.id)">+ 分组</button>
          </template>
        </div>
        <div v-if="kbs.length === 0" class="sidebar-empty">暂无知识库</div>
      </div>
    </aside>

    <div class="kb-detail">
      <div v-if="activeDoc" style="flex:1;display:flex;flex-direction:column;overflow:hidden;min-height:0">
        <div class="detail-topbar">
          <span class="detail-title">{{ activeDoc.name }}</span>
          <span class="doc-status" :class="activeDoc.status">{{ activeDoc.status }}</span>
          <span v-if="activeDoc.entry_count" class="entry-count">{{ activeDoc.entry_count }} 条目</span>
          <button class="btn btn-sm btn-danger" style="margin-left:auto"
            @click="doDeleteDoc(activeDoc)">删除</button>
        </div>
        <div v-if="activeDoc.error_msg" class="detail-error">{{ activeDoc.error_msg }}</div>
        <div class="detail-empty" v-else>
          <div class="detail-empty-icon">📄</div>
          <div>{{ activeDoc.doc_type }} · {{ activeDoc.entry_count }} 条目已索引</div>
        </div>
      </div>
      <div v-else class="detail-empty">
        <div class="detail-empty-icon">📚</div>
        <div>选择左侧文档查看详情</div>
      </div>
    </div>

    <!-- 新建知识库 -->
    <div v-if="showNewKB" class="modal-overlay" @click.self="showNewKB = false">
      <div class="modal" style="max-width:360px">
        <h3>新建知识库</h3>
        <div class="form-row">
          <label>名称</label>
          <input v-model="newKBName" class="input" @keyup.enter="doCreateKB" />
        </div>
        <div class="modal-footer">
          <button class="btn" @click="showNewKB = false">取消</button>
          <button class="btn btn-primary" :disabled="!newKBName.trim()" @click="doCreateKB">创建</button>
        </div>
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

    <!-- 导入弹窗 -->
    <div v-if="showImport" class="modal-overlay" @click.self="showImport = false">
      <div class="modal">
        <h3>导入文档</h3>
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
          <button class="btn" @click="showImport = false; importFiles = []">取消</button>
          <button class="btn btn-primary" :disabled="importing || importFiles.length === 0" @click="doImport">
            {{ importing ? '导入中…' : '导入' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  listKBs, createKB, deleteKB,
  listGroups, createGroup, deleteGroup,
  listDocuments, deleteDocuments, importDocument,
  type KnowledgeBase, type KnowledgeGroup, type KnowledgeDocument,
} from '../api/knowledge'

const fileInputRef = ref<HTMLInputElement | null>(null)

const kbs = ref<KnowledgeBase[]>([])
const groupsByKB = ref<Record<number, KnowledgeGroup[]>>({})
const docsByGroup = ref<Record<number, KnowledgeDocument[]>>({})
const activeDoc = ref<KnowledgeDocument | null>(null)
const collapsedKBs = ref(new Set<number>())
const collapsedGroups = ref(new Set<number>())

const showNewKB = ref(false)
const newKBName = ref('')
const showNewGroup = ref(false)
const newGroupName = ref('')
const newGroupKBID = ref(0)

const showImport = ref(false)
const importGroupID = ref(0)
const importing = ref(false)
const importErr = ref('')

interface ImportFile { file: File; status: 'pending' | 'ok' | 'error'; statusText: string }
const importFiles = ref<ImportFile[]>([])

function toggleKB(id: number) {
  const s = new Set(collapsedKBs.value)
  s.has(id) ? s.delete(id) : s.add(id)
  collapsedKBs.value = s
  if (!s.has(id)) loadGroups(id)
}

function toggleGroup(id: number) {
  const s = new Set(collapsedGroups.value)
  s.has(id) ? s.delete(id) : s.add(id)
  collapsedGroups.value = s
  if (!s.has(id)) loadDocs(id)
}

async function loadKBs() {
  kbs.value = await listKBs()
}

async function loadGroups(kbID: number) {
  groupsByKB.value = { ...groupsByKB.value, [kbID]: await listGroups(kbID) }
}

async function loadDocs(groupID: number) {
  docsByGroup.value = { ...docsByGroup.value, [groupID]: await listDocuments(groupID) }
}

async function doCreateKB() {
  if (!newKBName.value.trim()) return
  const kb = await createKB(newKBName.value.trim())
  kbs.value.push(kb)
  newKBName.value = ''
  showNewKB.value = false
}

function openNewGroup(kbID: number) {
  newGroupKBID.value = kbID
  newGroupName.value = ''
  showNewGroup.value = true
}

async function doCreateGroup() {
  if (!newGroupName.value.trim()) return
  const g = await createGroup(newGroupKBID.value, newGroupName.value.trim())
  groupsByKB.value = {
    ...groupsByKB.value,
    [newGroupKBID.value]: [...(groupsByKB.value[newGroupKBID.value] ?? []), g],
  }
  newGroupName.value = ''
  showNewGroup.value = false
}

async function doDeleteKB(kb: KnowledgeBase) {
  if (!confirm(`删除知识库「${kb.name}」及其所有内容？`)) return
  await deleteKB(kb.id)
  kbs.value = kbs.value.filter(k => k.id !== kb.id)
  const g = { ...groupsByKB.value }; delete g[kb.id]; groupsByKB.value = g
}

async function doDeleteGroup(group: KnowledgeGroup) {
  if (!confirm(`删除分组「${group.name}」及其所有文档？`)) return
  await deleteGroup(group.id)
  groupsByKB.value = {
    ...groupsByKB.value,
    [group.kb_id]: (groupsByKB.value[group.kb_id] ?? []).filter(g => g.id !== group.id),
  }
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
  let anyError = false
  for (const item of importFiles.value) {
    item.status = 'pending'
    item.statusText = '导入中…'
    try {
      await importDocument(importGroupID.value, item.file)
      item.status = 'ok'
      item.statusText = '成功'
    } catch (e: any) {
      item.status = 'error'
      item.statusText = e.message ?? '失败'
      anyError = true
    }
  }
  importing.value = false
  await loadDocs(importGroupID.value)
  if (!anyError) {
    showImport.value = false
    importFiles.value = []
  }
}

onMounted(loadKBs)
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

