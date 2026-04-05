<template>
  <div>
    <div class="page-header"><h2>命令执行</h2></div>

    <div class="exec-layout">
      <div class="exec-left">
        <div class="section-title">目标主机</div>
        <div class="host-list">
          <label v-for="h in hosts" :key="h.id" class="host-item">
            <input type="checkbox" v-model="selectedHosts" :value="h.id" />
            <span>{{ h.name }}</span>
            <small>{{ h.ip }}</small>
          </label>
        </div>
        <div class="form-row" style="margin-top:12px">
          <label>按标签</label>
          <input v-model="batchTag" class="input" placeholder="如 prod" />
        </div>
      </div>

      <div class="exec-right">
        <div class="form-row">
          <label>命令</label>
          <textarea v-model="command" class="input code" rows="3" placeholder="输入要执行的命令..." />
        </div>
        <div class="form-row">
          <label>超时（秒）</label>
          <input v-model.number="timeout" class="input" type="number" style="width:100px" />
        </div>
        <button class="btn btn-primary" :disabled="running" @click="run">
          {{ running ? '执行中...' : '▶ 执行' }}
        </button>

        <div v-if="results.length" class="results">
          <div v-for="r in results" :key="r.host" class="result-block">
            <div class="result-header">
              <span>{{ r.host }}</span>
              <span :class="r.exit_code === 0 ? 'ok' : 'err'">退出码 {{ r.exit_code }}</span>
              <span class="dim">{{ r.duration_ms }}ms</span>
            </div>
            <pre v-if="r.stdout" class="output">{{ r.stdout }}</pre>
            <pre v-if="r.stderr" class="output stderr">{{ r.stderr }}</pre>
            <pre v-if="r.error" class="output stderr">{{ r.error }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { listHosts, type SafeHost } from '../api/hosts'
import { execCommand, execBatch, type ExecResult } from '../api/exec'

const route = useRoute()
const hosts = ref<SafeHost[]>([])
const selectedHosts = ref<string[]>([])
const batchTag = ref('')
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
  if (!command.value.trim()) return
  running.value = true
  results.value = []
  try {
    if (batchTag.value || selectedHosts.value.length > 1) {
      results.value = await execBatch(command.value, {
        hostIds: selectedHosts.value.join(',') || undefined,
        tag: batchTag.value || undefined,
        timeoutSeconds: timeout.value,
      })
    } else if (selectedHosts.value.length === 1) {
      const r = await execCommand(selectedHosts.value[0], command.value, timeout.value)
      results.value = [r]
    }
  } finally {
    running.value = false
  }
}

onMounted(load)
</script>
