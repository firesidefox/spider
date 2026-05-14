<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Host } from '../api/hosts'

export interface DeviceStatus {
  id: string
  name: string
  ip: string
  vendor: string
  status: 'online' | 'offline' | 'executing' | 'success' | 'failed'
  detail?: string
}

const props = defineProps<{
  devices: DeviceStatus[]
  allHosts: Host[]
  modelValue: string[] | null
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', v: string[] | null): void
}>()

// ── resize ──────────────────────────────────────────────────────────────────
const panelRef = ref<HTMLElement | null>(null)
const statusHeight = ref<number | null>(null) // null = auto

function onDividerMousedown(e: MouseEvent) {
  e.preventDefault()
  const startY = e.clientY
  const startH = statusHeight.value ?? (panelRef.value?.querySelector('.status-zone') as HTMLElement)?.offsetHeight ?? 0
  const panelH = panelRef.value?.offsetHeight ?? 0

  function onMove(ev: MouseEvent) {
    const delta = ev.clientY - startY
    const next = Math.max(0, Math.min(startH + delta, panelH))
    statusHeight.value = next
  }
  function onUp() {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

// ── edit mode ────────────────────────────────────────────────────────────────
const editMode = ref(false)
const activeTags = ref<string[]>([])
const search = ref('')

const allTags = computed(() => {
  const set = new Set<string>()
  for (const h of props.allHosts) for (const t of h.tags ?? []) set.add(t)
  return [...set].sort()
})

const filteredHosts = computed(() => {
  const q = search.value.toLowerCase()
  return props.allHosts.filter(h => {
    if (activeTags.value.length > 0 && !activeTags.value.some(t => h.tags?.includes(t))) return false
    if (q && !h.name.toLowerCase().includes(q) && !h.ip.includes(q)) return false
    return true
  })
})

const isAllSelected = computed(() => props.modelValue === null)

function toggleTag(tag: string) {
  if (activeTags.value.includes(tag)) {
    activeTags.value = activeTags.value.filter(t => t !== tag)
  } else {
    activeTags.value = [...activeTags.value, tag]
    // auto-check hosts with this tag
    const current = new Set(props.modelValue ?? props.allHosts.map(h => h.id))
    for (const h of props.allHosts) {
      if (h.tags?.includes(tag)) current.add(h.id)
    }
    emit('update:modelValue', [...current])
  }
}

function toggleHost(id: string) {
  if (isAllSelected.value) {
    // deselect one → switch to partial
    emit('update:modelValue', props.allHosts.map(h => h.id).filter(i => i !== id))
  } else {
    const set = new Set(props.modelValue!)
    if (set.has(id)) set.delete(id)
    else set.add(id)
    emit('update:modelValue', set.size === props.allHosts.length ? null : [...set])
  }
}

function selectAll() {
  emit('update:modelValue', null)
}

function clearAll() {
  emit('update:modelValue', [])
}

function selectFiltered() {
  const ids = new Set(props.modelValue ?? props.allHosts.map(h => h.id))
  for (const h of filteredHosts.value) ids.add(h.id)
  emit('update:modelValue', ids.size === props.allHosts.length ? null : [...ids])
}

function clearFiltered() {
  const ids = new Set(props.modelValue ?? props.allHosts.map(h => h.id))
  for (const h of filteredHosts.value) ids.delete(h.id)
  emit('update:modelValue', [...ids])
}

function isChecked(id: string): boolean {
  return isAllSelected.value || (props.modelValue?.includes(id) ?? false)
}

// ── heat matrix ──────────────────────────────────────────────────────────────
function cellClass(d: DeviceStatus): string {
  return `hc hc-${d.status}`
}

function isCellVisible(id: string): boolean {
  return isAllSelected.value || (props.modelValue?.includes(id) ?? false)
}

// ── stats ────────────────────────────────────────────────────────────────────
const stats = computed(() => {
  const s = { online: 0, offline: 0, executing: 0, failed: 0 }
  for (const d of props.devices) {
    if (d.status === 'online' || d.status === 'success') s.online++
    else if (d.status === 'offline') s.offline++
    else if (d.status === 'executing') s.executing++
    else if (d.status === 'failed') s.failed++
  }
  return s
})

// ── view mode chip list ───────────────────────────────────────────────────────
const MAX_CHIPS = 5
const selectedChips = computed(() => {
  if (isAllSelected.value) return []
  return (props.modelValue ?? [])
    .map(id => props.allHosts.find(h => h.id === id))
    .filter(Boolean) as Host[]
})

// ── device row status dot ─────────────────────────────────────────────────────
function statusColor(hostId: string): string {
  const d = props.devices.find(x => x.id === hostId)
  if (!d) return 'var(--muted)'
  switch (d.status) {
    case 'online': case 'success': return 'var(--green)'
    case 'executing': return 'var(--yellow)'
    case 'failed': return 'var(--red)'
    default: return 'var(--muted)'
  }
}
</script>

<template>
  <div class="target-panel" ref="panelRef">
    <!-- ── Status zone ── -->
    <div
      class="status-zone"
      :style="statusHeight !== null ? { height: statusHeight + 'px', overflow: 'hidden auto' } : { overflow: 'hidden auto' }"
    >
      <div class="zone-hdr">
        <span class="zone-title">状态</span>
        <div class="stats-bar">
          <span class="stat"><span class="sdot" style="background:var(--green)"></span>{{ stats.online }}</span>
          <span class="stat"><span class="sdot" style="background:var(--muted)"></span>{{ stats.offline }}</span>
          <span class="stat"><span class="sdot" style="background:var(--yellow)"></span>{{ stats.executing }}</span>
          <span class="stat"><span class="sdot" style="background:var(--red)"></span>{{ stats.failed }}</span>
        </div>
      </div>
      <div class="heat-matrix">
        <template v-for="d in devices" :key="d.id">
          <div
            v-if="isCellVisible(d.id)"
            :class="cellClass(d)"
            :style="!isAllSelected && isChecked(d.id) ? { outline: '2px solid var(--primary)', outlineOffset: '1px' } : {}"
            :title="`${d.name} — ${d.status}`"
          ></div>
          <div v-else class="hc hc-placeholder"></div>
        </template>
      </div>
    </div>

    <!-- ── Drag divider ── -->
    <div class="divider" @mousedown="onDividerMousedown"></div>

    <!-- ── Selection zone ── -->
    <div class="selection-zone">
      <!-- View mode header -->
      <div v-if="!editMode" class="zone-hdr">
        <span class="zone-title">目标主机</span>
        <div class="hdr-right">
          <span v-if="isAllSelected" class="badge badge-all">全部 {{ allHosts.length }} 台</span>
          <span v-else class="badge badge-partial">已选 {{ modelValue?.length ?? 0 }} / {{ allHosts.length }}</span>
          <button class="edit-btn" @click="editMode = true">编辑</button>
        </div>
      </div>
      <!-- Edit mode header -->
      <div v-else class="zone-hdr">
        <span class="zone-title">目标主机</span>
        <div class="hdr-right">
          <span v-if="isAllSelected" class="badge badge-all">全部 {{ allHosts.length }} 台</span>
          <span v-else class="badge badge-partial">已选 {{ modelValue?.length ?? 0 }} / {{ allHosts.length }}</span>
          <button class="edit-btn done-btn" @click="editMode = false">完成</button>
        </div>
      </div>

      <!-- View mode body -->
      <div v-if="!editMode" class="view-body">
        <div v-if="isAllSelected" class="all-label">全部主机（AI 自行选择目标）</div>
        <template v-else>
          <div class="chip-list">
            <div v-for="h in selectedChips.slice(0, MAX_CHIPS)" :key="h.id" class="host-chip">
              <span class="chip-name">{{ h.name }}</span>
              <span class="chip-ip">{{ h.ip }}</span>
            </div>
            <div v-if="selectedChips.length > MAX_CHIPS" class="chip-more">
              +{{ selectedChips.length - MAX_CHIPS }} 台
            </div>
          </div>
        </template>
      </div>

      <!-- Edit mode body -->
      <div v-else class="edit-body">
        <!-- Tag filter -->
        <div v-if="allTags.length > 0" class="tag-bar">
          <button
            v-for="tag in allTags" :key="tag"
            class="tag-chip"
            :class="{ active: activeTags.includes(tag) }"
            @click="toggleTag(tag)"
          >{{ tag }}</button>
        </div>
        <!-- Search -->
        <input v-model="search" class="search-input" placeholder="搜索名称或 IP..." />
        <!-- Bulk row -->
        <div class="bulk-row">
          <span class="bulk-count">过滤结果 {{ filteredHosts.length }} 台</span>
          <button class="bulk-btn" @click="selectFiltered">全选</button>
          <span class="bulk-sep">·</span>
          <button class="bulk-btn" @click="clearFiltered">清空</button>
          <span class="bulk-sep">·</span>
          <button class="bulk-btn" @click="selectAll">全部</button>
        </div>
        <!-- Device list -->
        <div class="device-list">
          <div
            v-for="h in filteredHosts" :key="h.id"
            class="device-row"
            :class="{ checked: isChecked(h.id) }"
            @click="toggleHost(h.id)"
          >
            <input type="checkbox" :checked="isChecked(h.id)" @click.stop="toggleHost(h.id)" class="row-check" />
            <span class="row-dot" :style="{ background: statusColor(h.id) }"></span>
            <span class="row-name">{{ h.name }}</span>
            <span class="row-ip">{{ h.ip }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.target-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 12px;
  background: var(--panel);
  border-left: 1px solid var(--border);
  overflow: hidden;
}

/* ── zones ── */
.status-zone {
  flex-shrink: 0;
  min-height: 0;
}
.selection-zone {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

/* ── divider ── */
.divider {
  height: 5px;
  background: var(--border);
  cursor: row-resize;
  flex-shrink: 0;
  position: relative;
}
.divider::after {
  content: '';
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 24px;
  height: 2px;
  background: #444;
  border-radius: 1px;
}
.divider:hover { background: var(--primary); opacity: 0.4; }

/* ── zone header ── */
.zone-hdr {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.zone-title {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-sub);
}
.hdr-right { display: flex; align-items: center; gap: 6px; }

/* ── badges ── */
.badge {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 8px;
}
.badge-all { background: rgba(63,185,80,0.12); color: var(--green); border: 1px solid rgba(63,185,80,0.25); }
.badge-partial { background: rgba(74,158,255,0.12); color: var(--primary); border: 1px solid rgba(74,158,255,0.25); }

.edit-btn {
  font-size: 10px;
  color: var(--primary);
  background: none;
  border: none;
  cursor: pointer;
  font-family: inherit;
  padding: 0;
}
.done-btn { color: var(--green); }

/* ── stats bar ── */
.stats-bar { display: flex; gap: 8px; }
.stat { display: flex; align-items: center; gap: 3px; color: var(--text-sub); font-size: 10px; }
.sdot { width: 5px; height: 5px; border-radius: 50%; flex-shrink: 0; }

/* ── heat matrix ── */
.heat-matrix {
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
  padding: 8px 10px;
}
.hc {
  width: 12px;
  height: 12px;
  border-radius: 2px;
  transition: opacity 0.15s;
}
.hc:hover { opacity: 1; transform: scale(1.3); }
.hc-placeholder { background: transparent; }
.hc-online  { background: #3fb950; }
.hc-offline { background: #3a3a3a; }
.hc-executing {
  background: #d29922;
  animation: hc-pulse 1.2s ease-in-out infinite;
}
.hc-success {
  background: #3fb950;
  animation: hc-flash 0.6s ease-out forwards;
}
.hc-failed {
  background: #f85149;
  animation: hc-shake 0.4s ease-out;
}

@keyframes hc-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(210,153,34,0.6); }
  50%       { opacity: 0.7; box-shadow: 0 0 0 4px rgba(210,153,34,0); }
}
@keyframes hc-flash {
  0%   { background: #7ee787; box-shadow: 0 0 6px #7ee787; }
  100% { background: #3fb950; box-shadow: none; }
}
@keyframes hc-shake {
  0%, 100% { transform: translateX(0); }
  25%      { transform: translateX(-2px); }
  75%      { transform: translateX(2px); }
}

/* ── view body ── */
.view-body { padding: 8px 10px; flex: 1; overflow-y: auto; }
.all-label { color: var(--green); font-size: 11px; }
.chip-list { display: flex; flex-direction: column; gap: 3px; }
.host-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 7px;
  background: rgba(74,158,255,0.06);
  border: 1px solid rgba(74,158,255,0.15);
  border-radius: 4px;
}
.chip-name { color: var(--text); flex: 1; }
.chip-ip { color: var(--text-sub); font-size: 10px; }
.chip-more { color: var(--text-sub); font-size: 11px; padding: 2px 7px; }

/* ── edit body ── */
.edit-body { display: flex; flex-direction: column; flex: 1; min-height: 0; overflow: hidden; }

.tag-bar { display: flex; flex-wrap: wrap; gap: 4px; padding: 6px 10px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.tag-chip {
  font-size: 10px;
  padding: 2px 7px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: none;
  color: var(--text-sub);
  cursor: pointer;
  font-family: inherit;
}
.tag-chip.active { background: rgba(74,158,255,0.15); color: var(--primary); border-color: var(--primary); }

.search-input {
  margin: 6px 10px;
  background: var(--input-bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 5px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-family: inherit;
  outline: none;
  flex-shrink: 0;
}
.search-input:focus { border-color: var(--primary); }

.bulk-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.bulk-count { color: var(--text-sub); font-size: 10px; flex: 1; }
.bulk-btn { font-size: 10px; color: var(--primary); background: none; border: none; cursor: pointer; font-family: inherit; padding: 0; }
.bulk-sep { color: var(--border); font-size: 10px; }

.device-list { flex: 1; overflow-y: auto; }
.device-row {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 5px 10px;
  cursor: pointer;
  border-left: 2px solid transparent;
}
.device-row:hover { background: var(--row-hover); }
.device-row.checked {
  background: rgba(74,158,255,0.06);
  border-left-color: var(--primary);
}
.device-row:not(.checked) { opacity: 0.5; }
.row-check { accent-color: var(--primary); flex-shrink: 0; cursor: pointer; }
.row-dot { width: 5px; height: 5px; border-radius: 50%; flex-shrink: 0; }
.row-name { flex: 1; color: var(--text); }
.row-ip { color: var(--text-sub); font-size: 10px; }
</style>
