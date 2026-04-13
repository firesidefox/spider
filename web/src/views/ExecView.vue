<template>
  <div class="fullscreen-page exec-page">
    <!-- 左侧主机列表 -->
    <aside class="exec-sidebar">
      <div class="sidebar-header">
        <span class="sidebar-title">目标主机</span>
        <span v-if="selectedHosts.length" class="sidebar-count">已选 {{ selectedHosts.length }}</span>
      </div>
      <div class="sidebar-search">
        <input v-model="hostSearch" class="input" placeholder="搜索主机名 / IP / 标签..." />
      </div>
      <div class="sidebar-list">
        <label v-for="h in filteredHosts" :key="h.id" class="sidebar-item" :class="{ selected: selectedHosts.includes(h.id) }">
          <input type="checkbox" v-model="selectedHosts" :value="h.id" />
          <div class="sidebar-item-body">
            <div class="sidebar-item-name">{{ h.name }}</div>
            <div class="sidebar-item-meta">
              <span class="sidebar-item-ip">{{ h.ip }}</span>
              <span v-for="t in h.tags" :key="t" class="tag small">{{ t }}</span>
            </div>
          </div>
        </label>
        <div v-if="filteredHosts.length === 0" class="sidebar-empty">无匹配主机</div>
      </div>
    </aside>

    <!-- 右侧主区域 -->
    <div class="exec-main">
      <!-- 顶部命令栏 -->
      <div class="exec-topbar">
        <div class="exec-topbar-left">
          <div class="exec-target-info">
            <span v-if="selectedHosts.length === 0" class="dim">未选择主机</span>
            <span v-else-if="selectedHosts.length === 1" class="exec-target-name">
              {{ hosts.find(h => h.id === selectedHosts[0])?.name }}
              <small class="dim">{{ hosts.find(h => h.id === selectedHosts[0])?.ip }}</small>
            </span>
            <span v-else class="exec-target-name">{{ selectedHosts.length }} 台主机</span>
          </div>
        </div>
        <div class="exec-topbar-right">
          <span class="exec-label">超时</span>
          <input v-model.number="timeout" class="input timeout-input" type="number" />
          <span class="exec-label">秒</span>
        </div>
      </div>

      <!-- 命令输入区 -->
      <div class="exec-input-area">
        <textarea
          v-model="command"
          class="input code exec-textarea"
          placeholder="输入要执行的命令..."
          @keydown.ctrl.enter="run"
          @keydown.meta.enter="run"
        />
        <button class="btn btn-primary exec-run-btn" :disabled="running || !selectedHosts.length" @click="run">
          <span v-if="running" class="running-dot" />
          {{ running ? '执行中...' : '▶ 执行' }}
        </button>
      </div>

      <!-- 结果区 -->
      <div class="exec-results-area">
        <div v-if="!results.length && !running" class="exec-empty">
          <div class="exec-empty-icon">⌨</div>
          <div>选择主机，输入命令，按 Ctrl+Enter 执行</div>
        </div>

        <div v-for="r in results" :key="r.host" class="result-block">
          <div class="result-header">
            <span class="result-host">{{ r.host }}</span>
            <span :class="r.exit_code === 0 ? 'ok' : 'err'">
              {{ r.exit_code === 0 ? '✓' : '✗' }} 退出码 {{ r.exit_code }}
            </span>
            <span class="dim">{{ r.duration_ms }}ms</span>
          </div>
          <template v-if="r.stdout">
            <div v-if="hlReady && hl(r.stdout)" class="hl-wrap" v-html="hl(r.stdout)" />
            <pre v-else class="output">{{ r.stdout }}</pre>
          </template>
          <pre v-if="r.stderr" class="output stderr">{{ r.stderr }}</pre>
          <pre v-if="r.error" class="output stderr">{{ r.error }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, inject } from 'vue'
import { useRoute } from 'vue-router'
import { listHosts, type SafeHost } from '../api/hosts'
import { execCommand, execBatch, type ExecResult } from '../api/exec'
import { useHighlight } from '../composables/useHighlight'

const { ready: hlReady, highlight } = useHighlight()
const isDark = inject<() => boolean>('isDark', () => true)
function hl(code: string) { return highlight(code, isDark()) }

const route = useRoute()
const hosts = ref<SafeHost[]>([])
const hostSearch = ref('')
const selectedHosts = ref<string[]>([])

const filteredHosts = computed(() => {
  const q = hostSearch.value.toLowerCase().trim()
  return hosts.value.filter(h => {
    if (!q) return true
    return h.name.toLowerCase().includes(q) || h.ip.includes(q) || h.tags.some(t => t.toLowerCase().includes(q))
  })
})

const command = ref('')
const timeout = ref(30)
const running = ref(false)
const results = ref<ExecResult[]>([])

async function load() {
  hosts.value = await listHosts()
  const q = route.query
  if (q.host) selectedHosts.value = [q.host as string]
  if (q.hosts) selectedHosts.value = (q.hosts as string).split(',')
}

async function run() {
  if (!command.value.trim() || !selectedHosts.value.length) return
  running.value = true
  results.value = []
  try {
    if (selectedHosts.value.length > 1) {
      results.value = await execBatch(command.value, {
        hostIds: selectedHosts.value.join(','),
        timeoutSeconds: timeout.value,
      })
    } else {
      const r = await execCommand(selectedHosts.value[0], command.value, timeout.value)
      results.value = [r]
    }
  } finally {
    running.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.exec-page {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* ── 左侧边栏 ── */
.exec-sidebar {
  width: 30%;
  min-width: 220px;
  max-width: 360px;
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 16px 10px;
  flex-shrink: 0;
}

.sidebar-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.sidebar-count {
  font-size: 11px;
  font-weight: 600;
  color: var(--primary);
  background: rgba(99,102,241,0.12);
  border: 1px solid rgba(99,102,241,0.3);
  border-radius: 10px;
  padding: 1px 8px;
}

.sidebar-search {
  padding: 0 12px 12px;
  flex-shrink: 0;
}

.sidebar-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px 8px;
}

.sidebar-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.1s;
  border: 1px solid transparent;
  margin-bottom: 2px;
}

.sidebar-item:hover { background: var(--row-hover); }

.sidebar-item.selected {
  background: rgba(99,102,241,0.1);
  border-color: rgba(99,102,241,0.25);
}

.sidebar-item input[type="checkbox"] { margin-top: 2px; flex-shrink: 0; accent-color: var(--primary); }

.sidebar-item-body { display: flex; flex-direction: column; gap: 4px; min-width: 0; }

.sidebar-item-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sidebar-item-meta {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.sidebar-item-ip { font-size: 12px; color: var(--label); font-family: 'SF Mono', Consolas, monospace; }

.sidebar-empty { color: var(--label); font-size: 13px; padding: 16px 10px; text-align: center; }

/* ── 右侧主区域 ── */
.exec-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--bg);
}

/* 顶部信息栏 */
.exec-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  flex-shrink: 0;
}

.exec-topbar-left { display: flex; align-items: center; gap: 12px; }
.exec-topbar-right { display: flex; align-items: center; gap: 8px; }

.exec-target-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  display: flex;
  align-items: center;
  gap: 8px;
}

.exec-label { font-size: 12px; color: var(--muted); }

.timeout-input {
  width: 64px;
  padding: 5px 8px;
  font-size: 13px;
  text-align: center;
}

/* 命令输入区 */
.exec-input-area {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  flex-shrink: 0;
}

.exec-textarea {
  flex: 1;
  resize: none;
  min-height: 72px;
  max-height: 200px;
  overflow-y: auto;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}

.exec-run-btn {
  height: 72px;
  padding: 0 24px;
  font-size: 14px;
  font-weight: 600;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.running-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #fff;
  animation: pulse 1s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

/* 结果区 */
.exec-results-area {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.exec-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--label);
  font-size: 14px;
}

.exec-empty-icon { font-size: 36px; opacity: 0.4; }

.result-host { font-weight: 600; color: var(--text); }

.hl-wrap :deep(pre.shiki) {
  margin: 0;
  padding: 12px 14px;
  border-radius: 0;
  font-size: 13px;
  line-height: 1.6;
  overflow-x: hidden;
  overflow-y: auto;
  max-height: 400px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'SF Mono', Consolas, 'Courier New', monospace;
}
</style>
