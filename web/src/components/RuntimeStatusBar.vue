<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import type { AgentStatus } from '../composables/useAgentStatus'
import { EXPLORE_TOOLS } from '../composables/toolSets'

const props = defineProps<{
  status: AgentStatus | null
}>()

const SPINNER = ['✻', '✦', '✶', '✷', '✸', '✹']

const spinnerIdx = ref(0)
const elapsedSec = ref(0)
let spinnerTimer: ReturnType<typeof setInterval> | null = null
let elapsedTimer: ReturnType<typeof setInterval> | null = null

function recomputeElapsed() {
  const t = props.status?.startedAt
  elapsedSec.value = t ? Math.floor((Date.now() - t) / 1000) : 0
}

onMounted(() => {
  spinnerTimer = setInterval(() => {
    spinnerIdx.value = (spinnerIdx.value + 1) % SPINNER.length
  }, 120)
  elapsedTimer = setInterval(recomputeElapsed, 1000)
  recomputeElapsed()
})

onUnmounted(() => {
  if (spinnerTimer) clearInterval(spinnerTimer)
  if (elapsedTimer) clearInterval(elapsedTimer)
})

watch(() => props.status?.startedAt, () => recomputeElapsed())

const visible = computed(() => {
  const s = props.status
  return !!s && s.phase !== 'done'
})

const spinnerChar = computed(() => SPINNER[spinnerIdx.value])

function truncate(s: string, n = 50): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}

function formatHosts(hosts: string[] | undefined): string {
  if (!hosts || hosts.length === 0) return ''
  if (hosts.length <= 3) return hosts.join(', ')
  return hosts.slice(0, 3).join(', ') + `, +${hosts.length - 3}`
}

function parseInput(inputJson?: string): Record<string, unknown> | null {
  if (!inputJson) return null
  try {
    const parsed = JSON.parse(inputJson)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : null
  } catch { return null }
}

function firstStringValue(inp: Record<string, unknown>): string | null {
  const v = inp.command ?? inp.path ?? inp.query ?? Object.values(inp).find(x => typeof x === 'string')
  return typeof v === 'string' && v ? v : null
}

const verbAndContext = computed<{ verb: string; context: string }>(() => {
  const s = props.status
  if (!s) return { verb: '', context: '' }
  const inp = parseInput(s.toolInput)

  switch (s.phase) {
    case 'thinking':
      return { verb: 'Processing…', context: '' }
    case 'confirm':
      return { verb: 'Awaiting confirm', context: s.toolName ? `· ${s.toolName}` : '' }
    case 'tool': {
      const name = s.toolName || 'unknown'
      if (name === 'RunCommand') {
        const hosts = formatHosts(s.hosts)
        const verb = hosts ? `Running on ${hosts}` : 'Running'
        const cmd = inp ? firstStringValue(inp) : null
        return { verb, context: cmd ? `· ${truncate(cmd)}` : '' }
      }
      if (EXPLORE_TOOLS.has(name)) {
        const arg = inp ? firstStringValue(inp) : null
        return { verb: `Exploring · ${name}`, context: arg ? `· ${truncate(arg)}` : '' }
      }
      return { verb: `Working · ${name}`, context: '' }
    }
    default:
      return { verb: '', context: '' }
  }
})
</script>

<template>
  <div v-if="visible" class="runtime-status-bar">
    <span class="spinner">{{ spinnerChar }}</span>
    <span class="verb">{{ verbAndContext.verb }}</span>
    <span v-if="verbAndContext.context" class="context">{{ verbAndContext.context }}</span>
    <span class="elapsed">{{ elapsedSec }}s</span>
    <span class="sep">·</span>
    <span class="esc">esc</span>
  </div>
</template>

<style scoped>
.runtime-status-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 16px;
  border-top: 1px solid var(--border);
  background: var(--nav);
  font-family: ui-monospace, 'SF Mono', monospace;
  font-size: 12px;
  color: var(--text-sub);
  overflow: hidden;
  white-space: nowrap;
}

.spinner {
  color: var(--primary);
  font-size: 13px;
  flex-shrink: 0;
}

.verb {
  color: var(--text);
  flex-shrink: 0;
}

.context {
  color: var(--text-sub);
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.elapsed {
  margin-left: auto;
}

.elapsed,
.sep,
.esc {
  color: var(--muted, var(--text-sub));
  flex-shrink: 0;
  font-size: 11px;
}
</style>
