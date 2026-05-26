<template>
  <div class="kb-page" :style="kbVars">
    <!-- Sidebar: groups + documents -->
    <aside class="kb-sidebar" :style="{ width: sidebarWidth + 'px' }">
      <div class="sidebar-toolbar">
        <span class="sidebar-title">知识库</span>
        <div class="sidebar-actions">
          <button v-if="!selectMode" class="btn btn-sm" @click="enterSelectMode">编辑</button>
          <button v-else class="btn btn-sm" @click="exitSelectMode">完成</button>
          <button v-if="!selectMode" class="btn btn-primary btn-sm" @click="showNewGroup = true">+ 分组</button>
          <button v-if="!selectMode" class="btn btn-primary btn-sm" @click="showGlobalImport = true">+ 导入</button>
        </div>
      </div>
      <div class="sidebar-search">
        <input ref="searchInputRef" v-model="searchQuery" class="filter-input"
          placeholder="搜索分组或文档..." />
        <span class="search-hint">/</span>
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
        <div v-for="group in filteredGroups" :key="group.id" class="group-section">
          <div class="group-header" :class="{ active: activeGroupId === group.id && !activeDoc }"
            @click="onGroupClick(group)">
            <input v-if="selectMode" type="checkbox" class="row-check"
              :checked="selectedGroups.has(group.id)"
              @click.stop="toggleSelectGroup(group.id)" />
            <span class="chevron">{{ expandedGroups.has(group.id) ? '▼' : '▶' }}</span>
            <span class="group-name">{{ group.name }}</span>
            <span class="group-count">{{ docsByGroup[group.id]?.length ?? 0 }}</span>
            <template v-if="!selectMode">
              <button class="del-btn" @click.stop="doDeleteGroup(group)" title="删除分组">×</button>
              <button class="add-btn" @click.stop="openImport(group.id)" title="导入文档">+</button>
            </template>
          </div>
          <div v-if="expandedGroups.has(group.id)" class="doc-list">
            <div v-for="doc in filteredDocs(group.id)" :key="doc.id" class="doc-row"
              :class="{ active: activeDoc?.id === doc.id }"
              @click="onDocClick(doc)">
              <input v-if="selectMode" type="checkbox" class="row-check"
                :checked="selectedDocs.has(doc.id)"
                @click.stop="toggleSelectDoc(doc.id)" />
              <span class="doc-icon">📄</span>
              <span class="doc-name">{{ doc.name }}</span>
              <span class="doc-status" :class="doc.status">{{ doc.status }}</span>
            </div>
            <div v-if="!filteredDocs(group.id).length" class="doc-empty">无文档</div>
          </div>
        </div>
        <div v-if="filteredGroups.length === 0" class="sidebar-empty">
          {{ searchQuery ? '无匹配结果' : '暂无分组' }}
        </div>
        <div class="sidebar-width-hint">{{ sidebarWidth }}px</div>
      </div>
      <div class="resize-handle" @mousedown.prevent="startResize('sidebar', $event)"></div>
    </aside>

    <!-- Entries panel -->
    <section v-if="activeDoc" class="kb-entries">
      <div class="entries-toolbar">
        <input v-model="entryQuery" class="filter-input" placeholder="🔍 搜索 API 路径或描述..."
          :style="entriesView === 'raw' ? 'opacity:0.4;pointer-events:none' : ''" />
        <div class="method-filters"
          :style="entriesView === 'raw' ? 'opacity:0.4;pointer-events:none' : ''">
          <button v-for="m in METHODS" :key="m" class="method-chip"
            :class="[m.toLowerCase(), { active: methodFilters.has(m) }]"
            @click="toggleMethod(m)">{{ m }}</button>
        </div>
        <div class="view-tabs">
          <button class="view-tab" :class="{ active: entriesView === 'friendly' }"
            @click="entriesView = 'friendly'">友好</button>
          <button class="view-tab" :class="{ active: entriesView === 'raw' }"
            @click="entriesView = 'raw'">原文</button>
        </div>
      </div>
      <div class="entries-meta">
        来源: {{ activeDoc.name }} · 块: {{ totalEntries }} · {{ formatDate(activeDoc.updated_at || activeDoc.created_at) }}
      </div>
      <div class="entries-body">
        <!-- 原文视图 -->
        <pre v-if="entriesView === 'raw'" class="resp-body raw-source">{{ activeDoc?.raw_content }}</pre>

        <!-- 友好视图 -->
        <template v-else>
          <div v-if="loadingSections" class="entries-loading">加载中...</div>
          <div v-else-if="!sections.length && !flatEntries.length" class="entries-empty">无条目</div>
          <div v-else class="entries-list">
            <div v-for="(entry, idx) in filteredEntries" :key="entry.id"
              class="entry-card"
              :class="{
                expanded: expandedEntries.has(entry.id),
                focused: focusedIdx === idx
              }"
              @click="toggleEntry(entry)">

              <!-- Card header: always visible -->
              <div class="entry-row">
                <span class="method-badge" :class="entryMethod(entry).toLowerCase()">
                  {{ entryMethod(entry) || '·' }}
                </span>
                <span class="entry-path">{{ entryPath(entry) }}</span>
                <button class="copy-btn" :title="'复制 ' + entryPath(entry)"
                  @click.stop="copy(entryPath(entry))">📋</button>
              </div>
              <div class="entry-summary">{{ entry.summary }}</div>

              <!-- Inline detail: visible when expanded -->
              <div v-if="expandedEntries.has(entry.id)" class="inline-detail" @click.stop>

                <div v-if="loadingEntries.has(entry.id)" class="inline-loading">加载中...</div>

                <template v-else-if="entryDetails[entry.id]">
                  <div v-if="entryDetails[entry.id].description" class="inline-section">
                    <h5>描述</h5>
                    <p>{{ entryDetails[entry.id].description }}</p>
                  </div>

                  <div v-if="entryDetails[entry.id].parameters?.length" class="inline-section">
                    <h5>参数</h5>
                    <table class="inline-param-table">
                      <thead><tr><th>名称</th><th>位置</th><th>类型</th><th>说明</th></tr></thead>
                      <tbody>
                        <tr v-for="p in entryDetails[entry.id].parameters" :key="p.name + (p.in || '')">
                          <td>
                            <code>{{ p.name }}</code>
                            <span v-if="p.required" class="required-mark">*</span>
                          </td>
                          <td><span class="param-in">{{ p.in || '-' }}</span></td>
                          <td><span class="param-type">{{ p.type || '-' }}</span></td>
                          <td>{{ p.description || '-' }}</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>

                  <div v-if="entryRespTabs(entryDetails[entry.id]).length" class="inline-section">
                    <h5>响应示例</h5>
                    <div class="resp-tabs">
                      <button v-for="t in entryRespTabs(entryDetails[entry.id])" :key="t.code"
                        class="resp-tab"
                        :class="{ active: entryRespCodes[entry.id] === t.code, ok: t.ok, err: !t.ok }"
                        @click.stop="setEntryRespCode(entry.id, t.code)">
                        <span class="resp-icon">{{ t.ok ? '✓' : '✗' }}</span>
                        <span>{{ t.code }}</span>
                        <span class="resp-desc">{{ t.description }}</span>
                      </button>
                    </div>
                    <pre class="resp-body"><code>{{ entryRespBody(entryDetails[entry.id], entryRespCodes[entry.id]) }}</code></pre>
                  </div>

                  <div v-if="!entryDetails[entry.id].description
                             && !entryDetails[entry.id].parameters?.length
                             && !entryRespTabs(entryDetails[entry.id]).length"
                       class="inline-section">
                    <h5>原始内容</h5>
                    <pre class="resp-body"><code>{{ entryDetails[entry.id].content }}</code></pre>
                  </div>
                </template>

                <div class="collapse-btn" @click.stop="toggleEntry(entry)">▲ 收起</div>
              </div>
            </div>
            <div v-if="!filteredEntries.length" class="entries-empty">无匹配条目</div>
          </div>
        </template>
      </div>
    </section>

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
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
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
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
        </div>
        <div class="file-drop-zone" @dragover.prevent @drop.prevent="onDrop"
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
          <button class="btn btn-primary"
            :disabled="importing || importFiles.length === 0 || (showGlobalImport && !importGroupID)"
            @click="doImport">
            {{ importing ? '导入中…' : '导入' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<!-- KB_SCRIPT_PLACEHOLDER -->
<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import {
  listGroups, createGroup, deleteGroup, deleteGroups,
  listDocuments, getSections, getEntries, getEntry,
  deleteDocuments, importDocument, moveDocuments, reindexDocuments,
  type KnowledgeGroup, type KnowledgeDocument, type KnowledgeSection,
  type KnowledgeEntry, type KnowledgeEntryDetail,
} from '../api/knowledge'

const METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'] as const
type Method = typeof METHODS[number]

const groups = ref<KnowledgeGroup[]>([])
const docsByGroup = ref<Record<number, KnowledgeDocument[]>>({})
const expandedGroups = ref(new Set<number>())
const activeGroupId = ref<number | null>(null)
const activeDoc = ref<KnowledgeDocument | null>(null)

const sections = ref<KnowledgeSection[]>([])
const entriesBySection = ref<Record<number, KnowledgeEntry[]>>({})
const loadingSections = ref(false)
const focusedIdx = ref(-1)

const expandedEntries = ref(new Set<number>())
const entryDetails = ref<Record<number, KnowledgeEntryDetail>>({})
const loadingEntries = ref(new Set<number>())
const entryRespCodes = ref<Record<number, string>>({})
const entriesView = ref<'friendly' | 'raw'>('friendly')

const searchQuery = ref('')
const entryQuery = ref('')
const methodFilters = ref(new Set<Method>())

const sidebarWidth = ref(280)
const SIDEBAR_MIN = 200, SIDEBAR_MAX = 400

const searchInputRef = ref<HTMLInputElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)

const showNewGroup = ref(false)
const newGroupName = ref('')
const showImport = ref(false)
const showGlobalImport = ref(false)
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

const kbVars = computed(() => ({} as Record<string, string>))

const filteredGroups = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return groups.value
  return groups.value.filter(g => {
    if (g.name.toLowerCase().includes(q)) return true
    return (docsByGroup.value[g.id] ?? []).some(d => d.name.toLowerCase().includes(q))
  })
})
function filteredDocs(groupID: number): KnowledgeDocument[] {
  const q = searchQuery.value.trim().toLowerCase()
  const docs = docsByGroup.value[groupID] ?? []
  if (!q) return docs
  if ((groups.value.find(g => g.id === groupID)?.name ?? '').toLowerCase().includes(q)) return docs
  return docs.filter(d => d.name.toLowerCase().includes(q))
}

const flatEntries = computed<KnowledgeEntry[]>(() => {
  const out: KnowledgeEntry[] = []
  for (const s of sections.value) {
    const arr = entriesBySection.value[s.id]
    if (arr) out.push(...arr)
  }
  return out
})
const totalEntries = computed(() => flatEntries.value.length)

function entryMethod(e: KnowledgeEntry): string {
  const sp = e.title.indexOf(' ')
  if (sp > 0) {
    const m = e.title.slice(0, sp).toUpperCase()
    if ((METHODS as readonly string[]).includes(m)) return m
  }
  return ''
}
function entryPath(e: KnowledgeEntry): string {
  const sp = e.title.indexOf(' ')
  return sp > 0 ? e.title.slice(sp + 1) : e.title
}

const filteredEntries = computed(() => {
  const q = entryQuery.value.trim().toLowerCase()
  const ms = methodFilters.value
  return flatEntries.value.filter(e => {
    if (ms.size > 0) {
      const m = entryMethod(e)
      if (!m || !ms.has(m as Method)) return false
    }
    if (q) {
      const path = entryPath(e).toLowerCase()
      const sum = (e.summary || '').toLowerCase()
      if (!path.includes(q) && !sum.includes(q)) return false
    }
    return true
  })
})

const docsSelectedCount = computed(() => selectedDocs.value.size)
const hasSelection = computed(() => selectedDocs.value.size + selectedGroups.value.size > 0)
const selectionLabel = computed(() => {
  const d = selectedDocs.value.size, g = selectedGroups.value.size
  if (!d && !g) return '未选择'
  const parts: string[] = []
  if (g) parts.push(`${g} 分组`)
  if (d) parts.push(`${d} 文档`)
  return '已选 ' + parts.join(' / ')
})

interface RespTab { code: string; ok: boolean; description: string; example: any }

function entryRespTabs(detail: KnowledgeEntryDetail): RespTab[] {
  const r = detail.responses
  if (!r) return []
  return Object.keys(r).sort().map(code => {
    const v = r[code]
    return { code, ok: code.startsWith('2'), description: v.description || '', example: v.example }
  })
}

function entryRespBody(detail: KnowledgeEntryDetail, code: string): string {
  const tab = entryRespTabs(detail).find(t => t.code === code)
  if (!tab) return ''
  if (tab.example == null) return '(无示例)'
  if (typeof tab.example === 'string') return tab.example
  try { return JSON.stringify(tab.example, null, 2) } catch { return String(tab.example) }
}

function toggleMethod(m: Method) {
  const s = new Set(methodFilters.value)
  s.has(m) ? s.delete(m) : s.add(m)
  methodFilters.value = s
  saveFilters()
}

async function onGroupClick(g: KnowledgeGroup) {
  if (selectMode.value) { toggleSelectGroup(g.id); return }
  const s = new Set(expandedGroups.value)
  s.has(g.id) ? s.delete(g.id) : s.add(g.id)
  expandedGroups.value = s
  activeGroupId.value = g.id
  if (!docsByGroup.value[g.id]) await loadDocs(g.id)
}

async function onDocClick(d: KnowledgeDocument) {
  if (selectMode.value) { toggleSelectDoc(d.id); return }
  activeGroupId.value = d.group_id
  activeDoc.value = d
}

watch(activeDoc, async d => {
  expandedEntries.value = new Set()
  entryDetails.value = {}
  loadingEntries.value = new Set()
  entryRespCodes.value = {}
  entriesView.value = 'friendly'
  if (!d) { sections.value = []; entriesBySection.value = {}; return }
  loadingSections.value = true
  try {
    const ss = await getSections(d.id)
    sections.value = ss
    entriesBySection.value = {}
    await Promise.all(ss.map(async s => {
      try {
        entriesBySection.value = { ...entriesBySection.value, [s.id]: await getEntries(s.id) }
      } catch { entriesBySection.value = { ...entriesBySection.value, [s.id]: [] } }
    }))
    focusedIdx.value = filteredEntries.value.length ? 0 : -1
  } finally { loadingSections.value = false }
})

function setEntryRespCode(id: number, code: string) {
  entryRespCodes.value = { ...entryRespCodes.value, [id]: code }
}

async function toggleEntry(e: KnowledgeEntry) {
  const id = e.id
  const next = new Set(expandedEntries.value)
  if (next.has(id)) {
    next.delete(id)
    expandedEntries.value = next
    return
  }
  next.add(id)
  expandedEntries.value = next
  focusedIdx.value = filteredEntries.value.findIndex(x => x.id === id)
  if (!entryDetails.value[id]) {
    const loading = new Set(loadingEntries.value)
    loading.add(id)
    loadingEntries.value = loading
    try {
      const detail = await getEntry(id)
      entryDetails.value = { ...entryDetails.value, [id]: detail }
      const tabs = entryRespTabs(detail)
      entryRespCodes.value = {
        ...entryRespCodes.value,
        [id]: tabs.length ? (tabs.find(t => t.ok) ?? tabs[0]).code : ''
      }
    } finally {
      const loading2 = new Set(loadingEntries.value)
      loading2.delete(id)
      loadingEntries.value = loading2
    }
  }
}

async function copy(text: string) {
  try { await navigator.clipboard.writeText(text) } catch {}
}

// resize
let resizing: 'sidebar' | null = null
let startX = 0, startW = 0
function startResize(target: 'sidebar', e: MouseEvent) {
  resizing = target
  startX = e.clientX
  startW = target === 'sidebar' ? sidebarWidth.value : 0
  window.addEventListener('mousemove', onResizeMove)
  window.addEventListener('mouseup', stopResize)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}
function onResizeMove(e: MouseEvent) {
  if (!resizing) return
  const dx = e.clientX - startX
  if (resizing === 'sidebar') {
    sidebarWidth.value = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, startW + dx))
  }
}
function stopResize() {
  if (!resizing) return
  resizing = null
  window.removeEventListener('mousemove', onResizeMove)
  window.removeEventListener('mouseup', stopResize)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
  localStorage.setItem('kb_sidebar_width', String(sidebarWidth.value))
}

function saveFilters() {
  localStorage.setItem('kb_method_filters', JSON.stringify(Array.from(methodFilters.value)))
}
function loadPersistence() {
  const sw = +(localStorage.getItem('kb_sidebar_width') ?? '0')
  if (sw) sidebarWidth.value = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, sw))
  try {
    const mf = JSON.parse(localStorage.getItem('kb_method_filters') ?? '[]')
    if (Array.isArray(mf)) methodFilters.value = new Set(mf)
  } catch {}
}

// keyboard
function onKeydown(e: KeyboardEvent) {
  const tag = (e.target as HTMLElement)?.tagName
  const isInput = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
  if (e.key === '/' && !isInput) {
    e.preventDefault()
    searchInputRef.value?.focus()
    return
  }
  if (e.key === 'Escape') {
    if (isInput) (e.target as HTMLElement).blur()
    return
  }
  if (isInput) return
  if (!activeDoc.value || !filteredEntries.value.length) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    focusedIdx.value = Math.min(filteredEntries.value.length - 1, focusedIdx.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    focusedIdx.value = Math.max(0, focusedIdx.value - 1)
  } else if (e.key === 'Enter' && focusedIdx.value >= 0) {
    e.preventDefault()
    toggleEntry(filteredEntries.value[focusedIdx.value])
  }
}

// data
async function loadGroups() { groups.value = await listGroups() }
async function loadDocs(groupID: number) {
  docsByGroup.value = { ...docsByGroup.value, [groupID]: await listDocuments(groupID) }
}

async function init() {
  loadPersistence()
  await loadGroups()
  await Promise.all(groups.value.map(g => loadDocs(g.id)))
  if (groups.value.length) {
    expandedGroups.value = new Set(groups.value.map(g => g.id))
    activeGroupId.value = groups.value[0].id
  }
}

// --- existing batch / import / modal ops ---
function enterSelectMode() { selectMode.value = true; editMenuOpen.value = false; clearSelection() }
function exitSelectMode() { selectMode.value = false; editMenuOpen.value = false; clearSelection() }
function clearSelection() { selectedGroups.value = new Set(); selectedDocs.value = new Set() }
function toggleInSet(s: Set<number>, id: number) {
  const n = new Set(s); n.has(id) ? n.delete(id) : n.add(id); return n
}
function toggleSelectGroup(id: number) {
  selectedGroups.value = toggleInSet(selectedGroups.value, id)
  const docs = docsByGroup.value[id] ?? []
  const newDocs = new Set(selectedDocs.value)
  if (selectedGroups.value.has(id)) {
    docs.forEach(d => newDocs.add(d.id))
  } else {
    docs.forEach(d => newDocs.delete(d.id))
  }
  selectedDocs.value = newDocs
}
function toggleSelectDoc(id: number) { selectedDocs.value = toggleInSet(selectedDocs.value, id) }

async function doCreateGroup() {
  if (!newGroupName.value.trim()) return
  const g = await createGroup(newGroupName.value.trim())
  groups.value.push(g); newGroupName.value = ''; showNewGroup.value = false
}
async function doDeleteGroup(g: KnowledgeGroup) {
  if (!confirm(`删除分组「${g.name}」及其所有文档？`)) return
  await deleteGroup(g.id)
  groups.value = groups.value.filter(x => x.id !== g.id)
  const d = { ...docsByGroup.value }; delete d[g.id]; docsByGroup.value = d
  if (activeGroupId.value === g.id) activeGroupId.value = groups.value[0]?.id ?? null
  if (activeDoc.value?.group_id === g.id) activeDoc.value = null
}
function openImport(groupID: number) {
  importGroupID.value = groupID; importFiles.value = []; importErr.value = ''; showImport.value = true
}
function closeImport() {
  showImport.value = false; showGlobalImport.value = false; importFiles.value = []; importErr.value = ''
}
function onFileSelected(e: Event) {
  const fs = (e.target as HTMLInputElement).files
  if (!fs) return
  importFiles.value = Array.from(fs).map(f => ({ file: f, status: 'pending', statusText: '待导入' }))
}
function onDrop(e: DragEvent) {
  const fs = e.dataTransfer?.files
  if (!fs) return
  importFiles.value = Array.from(fs).map(f => ({ file: f, status: 'pending', statusText: '待导入' }))
}
async function doImport() {
  importing.value = true; importErr.value = ''
  const results = await Promise.allSettled(
    importFiles.value.map(async item => {
      item.status = 'pending'; item.statusText = '导入中…'
      try {
        await importDocument(importGroupID.value, item.file)
        item.status = 'ok'; item.statusText = '成功'
      } catch (e: any) {
        item.status = 'error'; item.statusText = e.message ?? '失败'; throw e
      }
    })
  )
  const anyError = results.some(r => r.status === 'rejected')
  importing.value = false
  await loadDocs(importGroupID.value)
  if (!anyError) closeImport()
}

function onMoveDocs() {
  editMenuOpen.value = false; batchErr.value = ''; moveTargetGroupID.value = 0; showMove.value = true
}
async function doMove() {
  if (!moveTargetGroupID.value) return
  batching.value = true; batchErr.value = ''
  try {
    const ids = Array.from(selectedDocs.value)
    await moveDocuments(ids, moveTargetGroupID.value)
    const groupIDs = new Set<number>([moveTargetGroupID.value])
    for (const gid of Object.keys(docsByGroup.value)) {
      const arr = docsByGroup.value[Number(gid)] ?? []
      if (arr.some(d => selectedDocs.value.has(d.id))) groupIDs.add(Number(gid))
    }
    await Promise.all(Array.from(groupIDs).map(loadDocs))
    showMove.value = false; exitSelectMode()
  } catch (e: any) { batchErr.value = e.message ?? '移动失败' }
  finally { batching.value = false }
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
  } catch (e: any) { alert(e.message ?? '重建失败') }
  finally { batching.value = false }
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
    activeGroupId.value = groups.value[0]?.id ?? null
    await Promise.all(groups.value.map(x => loadDocs(x.id)))
    exitSelectMode()
  } catch (e: any) { alert(e.message ?? '删除失败') }
  finally { batching.value = false }
}

function formatDate(s?: string): string {
  if (!s) return ''
  const d = new Date(s)
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const mi = String(d.getMinutes()).padStart(2, '0')
  return `${m}/${day} ${h}:${mi}`
}

onMounted(() => {
  init()
  window.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('mousemove', onResizeMove)
  window.removeEventListener('mouseup', stopResize)
})
</script>

<!-- KB_STYLE_PLACEHOLDER -->
<style scoped>
.kb-page {
  display: flex; height: 100%; overflow: hidden;
  --m-get: #10b981; --m-post: #3b82f6; --m-put: #f59e0b;
  --m-delete: #dc2626; --m-patch: #8b5cf6;
}

/* Sidebar */
.kb-sidebar {
  position: relative; flex-shrink: 0;
  background: var(--panel); border-right: 1px solid var(--border);
  display: flex; flex-direction: column; overflow: hidden;
}
.sidebar-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0;
}
.sidebar-title { font-size: 13px; font-weight: 700; color: var(--text); }
.sidebar-actions { display: flex; gap: 6px; flex-wrap: wrap; }
.sidebar-search {
  position: relative; padding: 8px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0;
}
.search-hint {
  position: absolute; right: 18px; top: 50%; transform: translateY(-50%);
  font-size: 10px; color: var(--muted); border: 1px solid var(--border);
  border-radius: 3px; padding: 1px 5px; background: var(--surface);
}
.filter-input {
  width: 100%; padding: 6px 26px 6px 10px; font-size: 12px;
  border: 1px solid var(--border); border-radius: 4px;
  background: var(--surface); color: var(--text); box-sizing: border-box;
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
.sidebar-list { flex: 1; overflow-y: auto; padding-bottom: 24px; position: relative; }
.sidebar-empty { color: var(--label); font-size: 13px; padding: 32px 16px; text-align: center; }
.sidebar-width-hint {
  position: sticky; bottom: 0; text-align: right;
  padding: 4px 8px; font-size: 10px; color: var(--muted);
  background: linear-gradient(transparent, var(--panel) 60%);
  pointer-events: none;
}

.group-section { border-bottom: 1px solid var(--border); }
.group-header {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 12px; cursor: pointer;
  font-size: 12px; font-weight: 700; color: var(--text);
}
.group-header:hover { background: var(--row-hover); }
.group-header.active { background: rgba(99,102,241,0.08); }
.group-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.group-count {
  font-size: 10px; color: var(--muted); background: var(--surface);
  border-radius: 10px; padding: 1px 6px;
}
.chevron { font-size: 10px; color: var(--muted); width: 12px; flex-shrink: 0; }
.del-btn { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 14px; padding: 0 2px; }
.del-btn:hover { color: #dc2626; }
.add-btn { background: none; border: none; color: var(--primary); cursor: pointer; font-size: 14px; padding: 0 2px; }

.doc-list { padding: 2px 0 6px; }
.doc-row {
  display: flex; align-items: center; gap: 8px;
  padding: 5px 12px 5px 28px; cursor: pointer;
  border-left: 3px solid transparent;
}
.doc-row:hover { background: var(--row-hover); }
.doc-row.active { border-left-color: var(--primary); background: rgba(99,102,241,0.08); }
.doc-icon { font-size: 12px; flex-shrink: 0; }
.doc-name {
  flex: 1; font-size: 12px; color: var(--text);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.doc-status { font-size: 10px; border-radius: 4px; padding: 1px 5px; flex-shrink: 0; }
.doc-status.ready { background: rgba(16,185,129,0.12); color: #10b981; }
.doc-status.indexing { background: rgba(245,158,11,0.12); color: #f59e0b; }
.doc-status.error { background: rgba(220,38,38,0.12); color: #dc2626; }
.doc-status.pending { background: rgba(107,114,128,0.12); color: #6b7280; }
.doc-empty { font-size: 11px; color: var(--muted); padding: 4px 28px; font-style: italic; }
</style>
<style scoped>
/* Entries panel */
.kb-entries {
  position: relative; flex: 1; min-width: 0;
  background: var(--panel);
  display: flex; flex-direction: column; overflow: hidden;
}
.entries-toolbar {
  padding: 12px; border-bottom: 1px solid var(--border); flex-shrink: 0;
  display: flex; flex-direction: column; gap: 8px;
}
.method-filters { display: flex; gap: 6px; flex-wrap: wrap; }
.method-chip {
  font-size: 10px; font-weight: 700; padding: 3px 8px;
  border-radius: 4px; cursor: pointer; border: 1px solid transparent;
  background: var(--surface); color: var(--muted); letter-spacing: 0.5px;
}
.method-chip.get.active    { background: var(--m-get);    color: #fff; }
.method-chip.post.active   { background: var(--m-post);   color: #fff; }
.method-chip.put.active    { background: var(--m-put);    color: #fff; }
.method-chip.delete.active { background: var(--m-delete); color: #fff; }
.method-chip.patch.active  { background: var(--m-patch);  color: #fff; }
.method-chip:hover { border-color: var(--border); }

.entries-meta {
  padding: 10px 14px; font-size: 11px; color: var(--text-sub);
  background: var(--surface); border-bottom: 1px solid var(--border);
}
.entries-body { flex: 1; overflow-y: auto; padding: 10px; }
.entries-loading, .entries-empty {
  padding: 40px 20px; text-align: center; color: var(--label); font-size: 13px;
}
.entries-list { display: flex; flex-direction: column; gap: 8px; }

.entry-card {
  background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
  padding: 10px 12px; cursor: pointer; transition: border-color 0.15s, box-shadow 0.15s;
}
.entry-card:hover { border-color: var(--primary); }
.entry-card:hover .copy-btn { opacity: 1; }
.entry-card.focused {
  box-shadow: 0 0 0 2px rgba(99,102,241,0.4);
  border-color: var(--primary);
}
.entry-card.active {
  border-color: var(--primary); background: rgba(99,102,241,0.08);
}
.entry-row { display: flex; align-items: center; gap: 8px; }
.method-badge {
  font-size: 10px; font-weight: 700; padding: 2px 6px;
  border-radius: 3px; color: #fff; letter-spacing: 0.5px; flex-shrink: 0;
  min-width: 44px; text-align: center;
}
.method-badge.get    { background: var(--m-get); }
.method-badge.post   { background: var(--m-post); }
.method-badge.put    { background: var(--m-put); }
.method-badge.delete { background: var(--m-delete); }
.method-badge.patch  { background: var(--m-patch); }
.method-badge.lg { font-size: 12px; padding: 4px 10px; min-width: 60px; }

.entry-path {
  flex: 1; font-family: ui-monospace, monospace; font-size: 12px;
  color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.copy-btn {
  background: none; border: none; cursor: pointer; font-size: 13px;
  opacity: 0; transition: opacity 0.15s; padding: 0 4px;
}
.entry-summary {
  margin-top: 4px; font-size: 11px; color: var(--text-sub);
  overflow: hidden; text-overflow: ellipsis; display: -webkit-box;
  -webkit-line-clamp: 2; -webkit-box-orient: vertical;
}
</style>
<style scoped>
/* Detail panel */
.kb-detail { flex: 1; min-width: 0; display: flex; flex-direction: column; overflow: hidden; }
.detail-content { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.detail-topbar {
  display: flex; align-items: center; gap: 12px;
  padding: 14px 20px; border-bottom: 1px solid var(--border);
  background: var(--surface); flex-shrink: 0;
}
.detail-topbar kbd {
  font-size: 9px; padding: 1px 4px; border: 1px solid var(--border);
  border-radius: 3px; background: var(--panel); margin-left: 4px;
}
.detail-path {
  font-family: ui-monospace, monospace; font-size: 14px; font-weight: 600; color: var(--text);
  flex-shrink: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.detail-body { flex: 1; overflow-y: auto; padding: 20px; }
.detail-section {
  margin-bottom: 20px; padding: 16px; background: var(--surface);
  border: 1px solid var(--border); border-radius: 6px;
}
.detail-section h4 {
  margin: 0 0 12px; font-size: 13px; font-weight: 700; color: var(--text);
  letter-spacing: 0.5px; text-transform: uppercase;
}
.detail-section p { margin: 0; color: var(--text); font-size: 13px; line-height: 1.6; }
.detail-empty {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px; color: var(--label); font-size: 14px;
}
.detail-empty-icon { font-size: 36px; opacity: 0.5; }

.param-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.param-table th, .param-table td {
  padding: 8px 10px; text-align: left; border-bottom: 1px solid var(--border);
  vertical-align: top;
}
.param-table th {
  font-weight: 600; color: var(--text-sub); background: var(--panel);
  font-size: 11px; letter-spacing: 0.5px; text-transform: uppercase;
}
.param-table code {
  background: var(--panel); padding: 1px 6px; border-radius: 3px;
  font-size: 12px; color: var(--text);
}
.required-mark { color: #dc2626; margin-left: 2px; font-weight: 700; }
.param-in, .param-type {
  font-size: 11px; padding: 1px 6px; border-radius: 3px;
  background: var(--panel); color: var(--text-sub);
}

.resp-tabs { display: flex; gap: 4px; flex-wrap: wrap; margin-bottom: 12px; }
.resp-tab {
  display: flex; align-items: center; gap: 6px;
  padding: 6px 12px; border-radius: 4px; cursor: pointer;
  border: 1px solid var(--border); background: var(--panel);
  font-size: 11px; color: var(--text-sub);
}
.resp-tab.ok .resp-icon { color: #10b981; }
.resp-tab.err .resp-icon { color: #dc2626; }
.resp-tab.active.ok  { border-color: #10b981; background: rgba(16,185,129,0.08); color: var(--text); }
.resp-tab.active.err { border-color: #dc2626; background: rgba(220,38,38,0.08); color: var(--text); }
.resp-icon { font-weight: 700; }
.resp-desc { color: var(--muted); }
.resp-body {
  background: #1e293b; color: #e2e8f0; padding: 14px; border-radius: 4px;
  font-family: ui-monospace, monospace; font-size: 12px;
  overflow-x: auto; margin: 0; white-space: pre-wrap; line-height: 1.5;
}

/* Resize handle */
.resize-handle {
  position: absolute; top: 0; right: -3px; width: 6px; height: 100%;
  cursor: col-resize; z-index: 5;
}
.resize-handle:hover { background: rgba(99,102,241,0.3); }

/* shared modal/btn — reuse global styles via fullscreen-page if exists */
.btn { padding: 4px 10px; border-radius: 4px; border: 1px solid var(--border); background: var(--surface); color: var(--text); cursor: pointer; font-size: 12px; }
.btn:hover:not(:disabled) { background: var(--row-hover); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-sm { font-size: 11px; padding: 3px 8px; }
.btn-primary { background: var(--primary); color: #fff; border-color: var(--primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.1); background: var(--primary); }
.btn-danger { background: #dc2626; color: #fff; border-color: #dc2626; }
.modal-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,0.4); z-index: 100;
  display: flex; align-items: center; justify-content: center;
}
.modal {
  background: var(--panel); border: 1px solid var(--border); border-radius: 8px;
  padding: 20px; min-width: 320px; max-width: 90vw;
}
.modal h3 { margin: 0 0 16px; font-size: 16px; color: var(--text); }
.form-row { margin-bottom: 12px; display: flex; flex-direction: column; gap: 6px; }
.form-row label { font-size: 12px; color: var(--text-sub); }
.input {
  width: 100%; padding: 6px 10px; font-size: 13px;
  border: 1px solid var(--border); border-radius: 4px;
  background: var(--surface); color: var(--text); box-sizing: border-box;
}
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
.err { color: #dc2626; font-size: 12px; }

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

.view-tabs {
  display: flex;
  border-top: 1px solid var(--border);
  margin: 0 -12px;
}
.view-tab {
  flex: 1; text-align: center;
  font-size: 12px; font-weight: 600;
  padding: 7px 0; cursor: pointer;
  color: var(--muted); background: none; border: none;
  border-bottom: 2px solid transparent;
  transition: color 0.15s, border-color 0.15s;
}
.view-tab:hover { color: var(--text-sub); }
.view-tab.active { color: var(--primary); border-bottom-color: var(--primary); }
.raw-source {
  flex: 1; margin: 0; white-space: pre-wrap; word-break: break-all;
}

/* Inline detail */
.entry-card.expanded {
  border-color: var(--primary);
  background: rgba(99,102,241,0.04);
}
.inline-detail {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
}
.inline-loading {
  padding: 12px 0;
  font-size: 12px;
  color: var(--muted);
  text-align: center;
}
.inline-section {
  margin-bottom: 14px;
}
.inline-section h5 {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-sub);
  letter-spacing: 0.5px;
  text-transform: uppercase;
  margin-bottom: 8px;
}
.inline-section p {
  font-size: 12px;
  color: var(--text);
  line-height: 1.6;
  margin: 0;
}
.inline-param-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.inline-param-table th, .inline-param-table td {
  padding: 6px 8px;
  text-align: left;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}
.inline-param-table th {
  font-weight: 600;
  color: var(--text-sub);
  background: var(--surface);
  font-size: 11px;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}
.collapse-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
  font-size: 11px;
  color: var(--muted);
  cursor: pointer;
  gap: 4px;
}
.collapse-btn:hover { color: var(--text-sub); }
</style>
