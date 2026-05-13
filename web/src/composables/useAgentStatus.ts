import { ref, readonly } from 'vue'

export interface AgentStatus {
  conversationId: string
  title: string
  phase: 'thinking' | 'tool' | 'confirm' | 'done'
  toolName?: string
  toolInput?: string
  updatedAt: number
}

const statuses = ref<Map<string, AgentStatus>>(new Map())
const doneTimers = new Map<string, ReturnType<typeof setTimeout>>()

export function useAgentStatus() {
  return { statuses: readonly(statuses) }
}

export function updateAgentStatus(update: Omit<AgentStatus, 'updatedAt'>) {
  const existing = doneTimers.get(update.conversationId)
  if (existing) {
    clearTimeout(existing)
    doneTimers.delete(update.conversationId)
  }

  statuses.value.set(update.conversationId, {
    ...update,
    updatedAt: Date.now(),
  })
  statuses.value = new Map(statuses.value)

  if (update.phase === 'done') {
    const timer = setTimeout(() => {
      removeAgentStatus(update.conversationId)
      doneTimers.delete(update.conversationId)
    }, 3000)
    doneTimers.set(update.conversationId, timer)
  }
}

export function removeAgentStatus(conversationId: string) {
  const timer = doneTimers.get(conversationId)
  if (timer) {
    clearTimeout(timer)
    doneTimers.delete(conversationId)
  }
  statuses.value.delete(conversationId)
  statuses.value = new Map(statuses.value)
}

export function clearAllAgentTimers() {
  doneTimers.forEach(timer => clearTimeout(timer))
  doneTimers.clear()
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}

export function formatToolDetail(name: string, input: unknown): string {
  if (!input || typeof input !== 'object') return name
  const inp = input as Record<string, unknown>
  const val = inp.command ?? inp.path ?? Object.values(inp).find(v => typeof v === 'string')
  if (!val) return name
  return `${name}: ${truncate(String(val), 40)}`
}
