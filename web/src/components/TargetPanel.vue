<script setup lang="ts">
import { ref, computed } from 'vue'

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
}>()

const search = ref('')

const filtered = computed(() => {
  const q = search.value.toLowerCase()
  const list = q
    ? props.devices.filter(d => d.name.toLowerCase().includes(q) || d.ip.includes(q))
    : [...props.devices]
  list.sort((a, b) => {
    if (a.status === 'failed' && b.status !== 'failed') return -1
    if (b.status === 'failed' && a.status !== 'failed') return 1
    return 0
  })
  return list
})

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

function statusColor(status: string): string {
  switch (status) {
    case 'online': case 'success': return 'var(--green)'
    case 'offline': return 'var(--muted)'
    case 'executing': return 'var(--yellow)'
    case 'failed': return 'var(--red)'
    default: return 'var(--muted)'
  }
}

const selectedDevice = ref<string | null>(null)
</script>

<template>
  <div class="target-panel">
    <div class="stats-bar">
      <span class="stat"><span class="dot" style="background:var(--green)"></span>{{ stats.online }} 在线</span>
      <span class="stat"><span class="dot" style="background:var(--muted)"></span>{{ stats.offline }} 离线</span>
      <span class="stat"><span class="dot" style="background:var(--yellow)"></span>{{ stats.executing }} 执行中</span>
      <span class="stat"><span class="dot" style="background:var(--red)"></span>{{ stats.failed }} 失败</span>
    </div>

    <div class="heat-matrix">
      <div
        v-for="d in devices" :key="d.id"
        class="heat-cell"
        :style="{ background: statusColor(d.status) }"
        :title="`${d.name} — ${d.status}`"
        @click="selectedDevice = d.id"
      ></div>
    </div>

    <input v-model="search" class="search-input" placeholder="搜索设备..." />

    <div class="device-list">
      <div
        v-for="d in filtered" :key="d.id"
        class="device-row"
        :class="{ selected: selectedDevice === d.id, failed: d.status === 'failed' }"
        @click="selectedDevice = d.id"
      >
        <span class="device-dot" :style="{ background: statusColor(d.status) }"></span>
        <span class="device-name">{{ d.name }}</span>
        <span class="device-ip">{{ d.ip }}</span>
        <span class="device-vendor">{{ d.vendor }}</span>
      </div>
    </div>

    <div v-if="selectedDevice" class="device-detail">
      <div v-for="d in devices.filter(x => x.id === selectedDevice)" :key="d.id">
        <div class="detail-header">{{ d.name }} <span class="detail-status">{{ d.status }}</span></div>
        <div class="detail-info">{{ d.ip }} · {{ d.vendor }}</div>
        <pre v-if="d.detail" class="detail-output">{{ d.detail }}</pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.target-panel { display: flex; flex-direction: column; height: 100%; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 12px; background: var(--panel); border-left: 1px solid var(--border); padding: 12px; gap: 12px; overflow-y: auto; }

.stats-bar { display: flex; gap: 12px; flex-wrap: wrap; }
.stat { display: flex; align-items: center; gap: 4px; color: var(--text-sub); font-size: 11px; }
.dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }

.heat-matrix { display: flex; flex-wrap: wrap; gap: 3px; }
.heat-cell { width: 12px; height: 12px; border-radius: 2px; cursor: pointer; opacity: 0.8; transition: opacity 0.15s; }
.heat-cell:hover { opacity: 1; transform: scale(1.3); }

.search-input { background: var(--input-bg); border: 1px solid var(--border); color: var(--text); padding: 6px 10px; border-radius: 4px; font-size: 12px; font-family: inherit; outline: none; }
.search-input:focus { border-color: var(--primary); }

.device-list { flex: 1; overflow-y: auto; }
.device-row { display: flex; align-items: center; gap: 8px; padding: 5px 8px; cursor: pointer; border-radius: 4px; }
.device-row:hover { background: var(--row-hover); }
.device-row.selected { background: var(--row-hover); border-left: 2px solid var(--primary); }
.device-row.failed { color: var(--red); }
.device-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.device-name { color: var(--text); flex: 1; }
.device-ip { color: var(--muted); font-size: 11px; }
.device-vendor { color: var(--label); font-size: 11px; }

.device-detail { border-top: 1px solid var(--border); padding-top: 10px; }
.detail-header { color: var(--text); font-weight: 500; }
.detail-status { color: var(--muted); font-weight: normal; margin-left: 8px; }
.detail-info { color: var(--text-sub); font-size: 11px; margin: 4px 0; }
.detail-output { background: var(--input-bg); padding: 8px; border-radius: 4px; font-size: 11px; color: var(--text-sub); white-space: pre-wrap; max-height: 200px; overflow-y: auto; }
</style>
