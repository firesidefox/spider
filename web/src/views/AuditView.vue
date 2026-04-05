<template>
  <div>
    <div class="page-header"><h2>审计</h2></div>

    <div class="toolbar">
      <input v-model="filterHost" class="input" placeholder="按主机名过滤..." style="width:200px" @keyup.enter="load" />
      <button class="btn" @click="load">查询</button>
    </div>

    <table class="table">
      <thead>
        <tr>
          <th>时间</th><th>主机</th><th>命令</th><th>来源</th><th>退出码</th><th>耗时</th><th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="log in logs" :key="log.id">
          <td class="dim">{{ fmtTime(log.created_at) }}</td>
          <td>{{ log.host_name || log.host_id }}</td>
          <td class="code truncate">{{ log.command }}</td>
          <td><span class="badge">{{ log.triggered_by }}</span></td>
          <td :class="log.exit_code === 0 ? 'ok' : 'err'">{{ log.exit_code }}</td>
          <td class="dim">{{ log.duration_ms }}ms</td>
          <td><button class="btn btn-sm" @click="detail = log">查看</button></td>
        </tr>
        <tr v-if="logs.length === 0">
          <td colspan="7" style="text-align:center;color:#999;padding:32px">暂无记录</td>
        </tr>
      </tbody>
    </table>

    <div class="pagination">
      <button class="btn btn-sm" :disabled="offset === 0" @click="prev">上一页</button>
      <span class="dim">第 {{ offset / limit + 1 }} 页</span>
      <button class="btn btn-sm" :disabled="logs.length < limit" @click="next">下一页</button>
    </div>

    <!-- 详情弹窗 -->
    <div v-if="detail" class="modal-overlay" @click.self="detail = null">
      <div class="modal wide">
        <h3>执行详情</h3>
        <div class="detail-meta">
          <span>主机: {{ detail.host_name }}</span>
          <span>命令: <code>{{ detail.command }}</code></span>
          <span>退出码: {{ detail.exit_code }}</span>
          <span>耗时: {{ detail.duration_ms }}ms</span>
        </div>
        <div v-if="detail.stdout">
          <div class="section-title">stdout</div>
          <pre class="output">{{ detail.stdout }}</pre>
        </div>
        <div v-if="detail.stderr">
          <div class="section-title">stderr</div>
          <pre class="output stderr">{{ detail.stderr }}</pre>
        </div>
        <div class="modal-footer">
          <button class="btn" @click="detail = null">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listLogs, type ExecutionLog } from '../api/logs'
import { listHosts } from '../api/hosts'

const logs = ref<ExecutionLog[]>([])
const filterHost = ref('')
const detail = ref<ExecutionLog | null>(null)
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
