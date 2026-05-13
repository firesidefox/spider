<template>
  <div class="fullscreen-page tasks-page">
    <!-- 左侧面板 -->
    <aside class="tasks-sidebar">
      <div class="sidebar-toolbar">
        <span class="sidebar-title">任务管理</span>
      </div>
      <div class="sidebar-list">
        <div
          v-for="t in tasks" :key="t.id"
          class="task-row"
          :class="{ selected: activeTask?.id === t.id }"
          @click="selectTask(t)"
        >
          <div class="task-row-name">{{ t.name }}</div>
          <div class="task-row-meta">
            <span :class="taskStatusBadge(t.status)">{{ t.status }}</span>
            <span class="task-row-schedule">{{ t.schedule || '手动' }}</span>
          </div>
        </div>
        <div v-if="tasks.length === 0 && !loading && !listError" class="sidebar-empty">暂无任务</div>
        <div v-if="loading" class="sidebar-empty">加载中...</div>
        <div v-if="listError" class="err" style="padding: 12px 16px; font-size: 13px;">{{ listError }}</div>
      </div>
    </aside>

    <!-- 右侧详情 -->
    <div class="tasks-detail">
      <template v-if="!activeTask">
        <div class="detail-empty">选择一个任务</div>
      </template>
      <template v-else>
        <div class="detail-topbar">
          <span class="detail-title">{{ activeTask.name }}</span>
          <div class="detail-topbar-right">
            <span v-if="triggerMsg" :class="triggerMsgClass">{{ triggerMsg }}</span>
            <button class="btn btn-primary btn-sm" :disabled="triggering" @click="doTrigger">
              {{ triggering ? '执行中…' : '立即执行' }}
            </button>
          </div>
        </div>

        <div class="detail-body">
          <!-- 任务详情 -->
          <div class="settings-card">
            <h3>任务详情</h3>
            <div class="detail-meta">
              <span><strong>目标：</strong>{{ activeTask.goal }}</span>
              <span><strong>计划：</strong>{{ activeTask.schedule || '手动触发' }}</span>
              <span><strong>通知：</strong>{{ activeTask.notify_mode }}</span>
              <span><strong>超时：</strong>{{ activeTask.timeout_minutes }} 分钟</span>
              <span><strong>状态：</strong>
                <span :class="taskStatusBadge(activeTask.status)">{{ activeTask.status }}</span>
              </span>
            </div>
          </div>

          <!-- 执行历史 -->
          <div class="settings-card">
            <h3>执行历史</h3>
            <div v-if="runsLoading" class="dim">加载中...</div>
            <div v-else-if="runsError" class="err">{{ runsError }}</div>
            <template v-else>
              <table class="table">
                <thead>
                  <tr>
                    <th>开始时间</th>
                    <th>状态</th>
                    <th>摘要</th>
                    <th>耗时</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="r in runs" :key="r.id">
                    <td>{{ formatTime(r.started_at) }}</td>
                    <td><span :class="runStatusBadge(r.status)" class="badge">{{ r.status }}</span></td>
                    <td>{{ r.summary || '—' }}</td>
                    <td>{{ duration(r.started_at, r.finished_at) }}</td>
                  </tr>
                  <tr v-if="runs.length === 0">
                    <td colspan="4" style="text-align:center;color:var(--muted)">暂无执行记录</td>
                  </tr>
                </tbody>
              </table>
              <div v-if="hasMoreRuns" style="margin-top:12px">
                <button class="btn btn-sm" :disabled="runsLoading" @click="loadMoreRuns">加载更多</button>
              </div>
            </template>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listTasks, triggerTask, listTaskRuns, type Task, type TaskRun } from '../api/tasks'

const tasks = ref<Task[]>([])
const loading = ref(false)
const listError = ref('')
const activeTask = ref<Task | null>(null)

const runs = ref<TaskRun[]>([])
const runsLoading = ref(false)
const runsError = ref('')
const runsOffset = ref(0)
const hasMoreRuns = ref(false)
const RUNS_LIMIT = 20

const triggering = ref(false)
const triggerMsg = ref('')
const triggerMsgClass = ref<'ok' | 'err'>('ok')

onMounted(async () => {
  loading.value = true
  try {
    tasks.value = await listTasks()
  } catch (e: unknown) {
    listError.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
})

async function selectTask(t: Task) {
  activeTask.value = t
  runs.value = []
  runsOffset.value = 0
  hasMoreRuns.value = false
  triggerMsg.value = ''
  await fetchRuns(true)
}

async function fetchRuns(reset: boolean) {
  if (reset) runsOffset.value = 0
  runsLoading.value = true
  runsError.value = ''
  try {
    const batch = await listTaskRuns(activeTask.value!.id, RUNS_LIMIT, runsOffset.value)
    if (reset) runs.value = batch
    else runs.value = [...runs.value, ...batch]
    hasMoreRuns.value = batch.length === RUNS_LIMIT
    runsOffset.value += batch.length
  } catch (e: unknown) {
    runsError.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    runsLoading.value = false
  }
}

async function loadMoreRuns() {
  await fetchRuns(false)
}

async function doTrigger() {
  if (!activeTask.value) return
  triggering.value = true
  triggerMsg.value = ''
  try {
    await triggerTask(activeTask.value.id)
    triggerMsgClass.value = 'ok'
    triggerMsg.value = '已触发'
    await fetchRuns(true)
  } catch (e: unknown) {
    triggerMsgClass.value = 'err'
    triggerMsg.value = e instanceof Error ? e.message : '触发失败'
  } finally {
    triggering.value = false
  }
}

function taskStatusBadge(status: string): string {
  return status === 'active' ? 'badge badge-ok' : 'badge'
}

function runStatusBadge(status: string): string {
  if (status === 'completed') return 'badge-ok'
  if (status === 'failed') return 'badge-err'
  return 'badge'
}

function formatTime(iso: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('zh-CN', { hour12: false })
}

function duration(start: string, end?: string): string {
  if (!end) return '—'
  const ms = new Date(end).getTime() - new Date(start).getTime()
  if (ms < 0) return '—'
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s`
  return `${Math.floor(s / 60)}m ${s % 60}s`
}
</script>

<style scoped>
.tasks-page { display: flex; height: 100%; overflow: hidden; }

.tasks-sidebar {
  width: 280px;
  flex-shrink: 0;
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--surface);
}

.sidebar-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border-bottom: 1px solid var(--border);
}

.sidebar-title { font-size: 15px; font-weight: 700; color: var(--text); }

.sidebar-list { flex: 1; overflow-y: auto; }

.sidebar-empty { padding: 24px 16px; color: var(--muted); font-size: 13px; text-align: center; }

.task-row {
  padding: 12px 16px;
  cursor: pointer;
  border-bottom: 1px solid var(--border);
  transition: background 0.1s;
}
.task-row:hover { background: var(--row-hover); }
.task-row.selected { background: rgba(99,102,241,0.08); }

.task-row-name { font-size: 14px; font-weight: 500; color: var(--text); margin-bottom: 4px; }
.task-row-meta { display: flex; align-items: center; gap: 8px; }
.task-row-schedule { font-size: 12px; color: var(--muted); }

.tasks-detail { flex: 1; display: flex; flex-direction: column; overflow: hidden; }

.detail-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--muted);
  font-size: 15px;
}

.detail-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.detail-title { font-size: 16px; font-weight: 700; color: var(--text); }
.detail-topbar-right { display: flex; align-items: center; gap: 10px; }

.detail-body { flex: 1; overflow-y: auto; padding: 16px 20px; }
</style>
