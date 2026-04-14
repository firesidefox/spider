<template>
  <div class="fullscreen-page audit-page">
    <!-- 左侧日志列表 -->
    <aside class="audit-sidebar">
      <div class="sidebar-toolbar">
        <span class="sidebar-title">审计日志</span>
      </div>
      <div class="sidebar-search">
        <input v-model="filterHost" class="input" placeholder="按主机名过滤..." @keyup.enter="load" />
      </div>
      <div class="sidebar-list">
        <div
          v-for="log in logs" :key="log.id"
          class="log-row"
          :class="{ selected: activeLog?.id === log.id }"
          @click="activeLog = log"
        >
          <div class="log-row-top">
            <span class="log-host">{{ log.host_name || log.host_id }}</span>
            <span :class="log.exit_code === 0 ? 'status-ok' : 'status-err'">
              {{ log.exit_code === 0 ? '✓' : '✗' }}
            </span>
          </div>
          <div class="log-cmd">{{ log.command }}</div>
          <div class="log-meta">
            <span class="dim">{{ fmtTime(log.created_at) }}</span>
            <span class="dim">{{ log.duration_ms }}ms</span>
            <span class="badge">{{ log.triggered_by }}</span>
          </div>
        </div>
        <div v-if="logs.length === 0" class="sidebar-empty">暂无记录</div>
      </div>
      <div class="sidebar-pagination">
        <button class="btn btn-sm" :disabled="offset === 0" @click="prev">上一页</button>
        <span class="dim">第 {{ offset / limit + 1 }} 页</span>
        <button class="btn btn-sm" :disabled="logs.length < limit" @click="next">下一页</button>
      </div>
    </aside>

    <!-- 右侧详情 -->
    <div class="audit-detail">
      <template v-if="activeLog">
        <div class="detail-topbar">
          <div class="detail-topbar-left">
            <span class="detail-title">{{ activeLog.host_name || activeLog.host_id }}</span>
            <span :class="activeLog.exit_code === 0 ? 'badge-ok' : 'badge-err'">
              退出码 {{ activeLog.exit_code }}
            </span>
          </div>
          <span class="dim">{{ fmtTime(activeLog.created_at) }} · {{ activeLog.duration_ms }}ms</span>
        </div>
        <div class="detail-body">
          <div class="output-block">
            <div class="output-header">
              <span class="section-title" style="margin-bottom:0">stdin</span>
              <span class="badge">{{ activeLog.triggered_by }}</span>
            </div>
            <CodeBlock :code="activeLog.command" :html="hlReady ? hl(activeLog.command) : ''" />
          </div>
          <div v-if="activeLog.stdout" class="output-block">
            <div class="output-header">
              <span class="section-title" style="margin-bottom:0">stdout</span>
            </div>
            <CodeBlock :code="activeLog.stdout" :html="hlReady ? hl(activeLog.stdout) : ''" />
          </div>
          <div v-if="activeLog.stderr" class="output-block">
            <div class="output-header">
              <span class="section-title" style="margin-bottom:0">stderr</span>
            </div>
            <CodeBlock :code="activeLog.stderr" />
          </div>
        </div>
      </template>
      <div v-else class="detail-empty">
        <div class="detail-empty-icon">←</div>
        <div>选择左侧记录查看详情</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import { listLogs, type ExecutionLog } from '../api/logs'
import { listHosts } from '../api/hosts'
import { useHighlight } from '../composables/useHighlight'
import CodeBlock from '../components/CodeBlock.vue'

const { ready: hlReady, highlight } = useHighlight()
const isDark = inject<() => boolean>('isDark', () => true)
function hl(code: string) { return highlight(code, isDark()) }

const logs = ref<ExecutionLog[]>([])
const filterHost = ref('')
const activeLog = ref<ExecutionLog | null>(null)
const limit = 20
const offset = ref(0)

async function load() {
  let hostId = ''
  if (filterHost.value) {
    const hosts = await listHosts()
    const h = hosts.find(h => h.name === filterHost.value || h.id === filterHost.value)
    hostId = h?.id ?? filterHost.value
  }
  logs.value = await listLogs({ hostId, limit, offset: offset.value })
}

function prev() { offset.value = Math.max(0, offset.value - limit); load() }
function next() { offset.value += limit; load() }

function fmtTime(s: string) {
  return new Date(s).toLocaleString('zh-CN', { hour12: false })
}

onMounted(load)
</script>

<style scoped>
.audit-page {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* ── 左侧面板 ── */
.audit-sidebar {
  width: 26%;
  min-width: 280px;
  max-width: 400px;
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow: hidden;
}

.sidebar-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.sidebar-title { font-size: 13px; font-weight: 700; color: var(--text); }

.sidebar-search {
  padding: 10px 12px 8px;
  flex-shrink: 0;
}

.sidebar-list { flex: 1; overflow-y: auto; }

.log-row {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  border-left: 3px solid transparent;
  cursor: pointer;
  transition: background 0.1s;
}

.log-row:hover { background: var(--row-hover); }

.log-row.selected {
  border-left-color: var(--primary);
  background: rgba(99,102,241,0.1);
}

.log-row-top { display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px; }

.log-host { font-size: 14px; font-weight: 500; color: var(--text); }

.status-ok { color: var(--green); font-size: 13px; font-weight: 700; }
.status-err { color: var(--red); font-size: 13px; font-weight: 700; }

.log-cmd {
  font-size: 12px;
  color: var(--muted);
  font-family: 'SF Mono', Consolas, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-bottom: 6px;
}

.log-meta { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }

.sidebar-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 10px 16px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.sidebar-empty { color: var(--label); font-size: 13px; padding: 32px 16px; text-align: center; }

/* ── 右侧详情 ── */
.audit-detail {
  flex: 1;
  overflow: hidden;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.detail-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  flex-shrink: 0;
}

.detail-topbar-left { display: flex; align-items: center; gap: 10px; }

.detail-title { font-size: 15px; font-weight: 700; color: var(--text); }

.badge-ok {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 4px;
  background: rgba(74,222,128,0.12); color: var(--green); border: 1px solid rgba(74,222,128,0.3);
}
.badge-err {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 4px;
  background: rgba(248,113,113,0.12); color: var(--red); border: 1px solid rgba(248,113,113,0.3);
}

.detail-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.output-block {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
  box-shadow: var(--card-shadow);
}

.output-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
}

.detail-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--muted);
  font-size: 14px;
}

.detail-empty-icon { color: var(--border); font-size: 40px; }
</style>
