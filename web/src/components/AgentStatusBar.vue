<template>
  <div v-if="sorted.length > 0" class="agent-status-bar">
    <div
      v-for="s in sorted"
      :key="s.conversationId"
      class="agent-status-row"
      :class="{ 'is-current': s.conversationId === currentConvId }"
      @click="handleClick(s.conversationId)"
    >
      <span class="status-dot" :class="dotClass(s.phase)" />
      <span class="status-title">{{ s.title }}</span>
      <span class="status-sep">·</span>
      <span class="status-detail" :class="{ monospace: s.phase === 'tool' || s.phase === 'confirm' }">
        {{ rowDetail(s) }}
      </span>
      <span class="status-arrow">→</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAgentStatus, removeAgentStatus, formatToolDetail, type AgentStatus } from '../composables/useAgentStatus'

const { statuses } = useAgentStatus()
const router = useRouter()
const route = useRoute()

const currentConvId = computed(() => route.params.id as string | undefined)

const sorted = computed<AgentStatus[]>(() => {
  const all = Array.from(statuses.value.values())
  return all.sort((a, b) => {
    if (a.conversationId === currentConvId.value) return -1
    if (b.conversationId === currentConvId.value) return 1
    return b.updatedAt - a.updatedAt
  })
})

function dotClass(phase: AgentStatus['phase']) {
  if (phase === 'thinking' || phase === 'tool') return 'dot-running'
  if (phase === 'confirm') return 'dot-confirm'
  return 'dot-done'
}

function rowDetail(s: AgentStatus): string {
  switch (s.phase) {
    case 'thinking': return '思考中'
    case 'tool': return s.toolName ? formatToolDetail(s.toolName, s.toolInput) : '执行中'
    case 'confirm': {
      const tool = s.toolName ? formatToolDetail(s.toolName, s.toolInput) : ''
      return tool ? `等待确认 · ${tool}` : '等待确认'
    }
    case 'done': return '完成'
  }
}

function handleClick(convId: string) {
  removeAgentStatus(convId)
  router.push(`/chat/${convId}`)
}
</script>

<style scoped>
.agent-status-bar {
  border-top: 1px solid var(--border);
  background: var(--nav);
  max-height: 84px;
  overflow-y: auto;
  scrollbar-width: thin;
}

.agent-status-row {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 16px;
  cursor: pointer;
  font-size: 12px;
  color: var(--text-sub);
  transition: background 0.15s;
}

.agent-status-row:hover {
  background: var(--row-hover);
}

.agent-status-row.is-current {
  background: var(--row-alt);
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-running {
  background: var(--primary);
  animation: status-pulse 1.2s ease-in-out infinite;
}

.dot-confirm {
  background: var(--yellow);
}

.dot-done {
  background: var(--green);
}

@keyframes status-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.status-title {
  color: var(--text);
  font-weight: 500;
  flex-shrink: 0;
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-sep {
  flex-shrink: 0;
}

.status-detail {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.status-detail.monospace {
  font-family: ui-monospace, monospace;
  font-size: 11px;
}

.status-arrow {
  flex-shrink: 0;
  margin-left: auto;
  color: var(--text-sub);
  font-size: 11px;
}
</style>
