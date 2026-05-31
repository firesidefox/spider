<template>
  <div class="detail-topbar">
    <span class="detail-title">操作日志</span>
  </div>
  <div class="detail-body">
    <div class="edit-card">
      <div class="edit-card-title">我的操作日志</div>
      <table class="table">
        <thead><tr><th>主机</th><th>命令</th><th>状态</th><th>耗时</th><th>时间</th></tr></thead>
        <tbody>
          <template v-for="log in logs" :key="log.id">
            <tr class="log-row" @click="toggleLog(log.id)">
              <td style="font-weight:500;color:var(--text)">{{ log.host_name || '—' }}</td>
              <td class="cmd-cell">{{ log.command.length > 48 ? log.command.slice(0, 48) + '…' : log.command }}</td>
              <td>
                <span class="status-badge" :class="log.exit_code === 0 ? 'ok' : 'fail'">
                  {{ log.exit_code === 0 ? '✓ 成功' : `✗ ${log.exit_code}` }}
                </span>
              </td>
              <td class="dim">{{ log.duration_ms != null ? log.duration_ms + 'ms' : '—' }}</td>
              <td class="dim">{{ new Date(log.created_at).toLocaleString() }}</td>
            </tr>
            <tr v-if="expandedLog === log.id" class="log-expand">
              <td colspan="5">
                <div class="log-output">
                  <div v-if="log.stdout" class="output-block">
                    <div class="output-label">stdout</div>
                    <pre class="code output-pre">{{ log.stdout }}</pre>
                  </div>
                  <div v-if="log.stderr" class="output-block">
                    <div class="output-label err-label">stderr</div>
                    <pre class="code output-pre err-pre">{{ log.stderr }}</pre>
                  </div>
                  <div v-if="!log.stdout && !log.stderr" class="dim" style="padding:8px 0">无输出</div>
                </div>
              </td>
            </tr>
          </template>
          <tr v-if="logs.length === 0">
            <td colspan="5" class="dim" style="text-align:center;padding:32px">暂无操作日志</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { authHeaders } from '../../api/auth'

interface LogEntry {
  id: string; host_name: string; command: string; exit_code: number
  duration_ms: number | null; created_at: string; stdout: string; stderr: string
}

const logs = ref<LogEntry[]>([])
const expandedLog = ref<string | null>(null)
let logsLoaded = false

async function loadLogs() {
  if (logsLoaded) return
  logsLoaded = true
  try {
    const res = await fetch('/api/v1/logs?triggered_by=me&limit=50', { headers: authHeaders() })
    if (!res.ok) throw new Error()
    logs.value = await res.json()
  } catch {}
}

function toggleLog(id: string) {
  expandedLog.value = expandedLog.value === id ? null : id
}

onMounted(() => {
  loadLogs()
})
</script>

<style scoped>
.detail-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  flex-shrink: 0;
}

.detail-title { font-size: 15px; font-weight: 700; color: var(--text); }

.detail-body { flex: 1; overflow-y: auto; padding: 20px 24px; }

.edit-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 20px 24px;
  box-shadow: var(--card-shadow);
  margin-bottom: 16px;
}

.edit-card-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

.log-row { cursor: pointer; }
.cmd-cell { font-family: 'SF Mono', Consolas, monospace; font-size: 12px; color: var(--text-sub); }

.status-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 4px; border: 1px solid;
}
.status-badge.ok   { background: rgba(74,222,128,0.12); color: var(--green); border-color: rgba(74,222,128,0.3); }
.status-badge.fail { background: rgba(248,113,113,0.12); color: var(--red);  border-color: rgba(248,113,113,0.3); }

.log-expand td { padding: 0 !important; }
.log-output { padding: 12px 16px; display: flex; flex-direction: column; gap: 8px; }
.output-block { display: flex; flex-direction: column; gap: 4px; }
.output-label { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.07em; color: var(--muted); }
.err-label { color: var(--red); }
.output-pre {
  margin: 0; white-space: pre-wrap; word-break: break-all;
  background: var(--panel); border: 1px solid var(--border); border-radius: 6px;
  padding: 10px 12px; font-size: 12px; color: var(--text-sub);
  max-height: 240px; overflow-y: auto;
}
.err-pre { color: var(--red); }
</style>
