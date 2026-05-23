import { ref, readonly } from 'vue'

export interface AgentStatus {
  conversationId: string
  title: string
  phase: 'thinking' | 'tool' | 'confirm' | 'done'
  toolName?: string
  toolInput?: string
  hosts?: string[]
  startedAt?: number
  updatedAt: number
}

const statuses = ref<Map<string, AgentStatus>>(new Map())
const doneTimers = new Map<string, ReturnType<typeof setTimeout>>()

export function useAgentStatus() {
  return { statuses: readonly(statuses) }
}

export function updateAgentStatus(update: Omit<AgentStatus, 'updatedAt' | 'startedAt'>) {
  const existing = doneTimers.get(update.conversationId)
  if (existing) {
    clearTimeout(existing)
    doneTimers.delete(update.conversationId)
  }

  const prev = statuses.value.get(update.conversationId)
  // Skip reactivity trigger if only phase is 'thinking' and it's already set —
  // text_delta fires per token; avoid a full Map copy on every token.
  if (update.phase === 'thinking' && prev?.phase === 'thinking') {
    return
  }

  const now = Date.now()
  const phaseChanged = prev?.phase !== update.phase
  statuses.value.set(update.conversationId, {
    ...update,
    startedAt: phaseChanged || !prev?.startedAt ? now : prev.startedAt,
    updatedAt: now,
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

export function formatToolDetail(name: string, inputJson?: string): string {
  if (!inputJson) return name
  let inp: Record<string, unknown>
  try { inp = JSON.parse(inputJson) } catch { return name }
  if (!inp || typeof inp !== 'object' || Array.isArray(inp)) return name
  const val = inp.command ?? inp.path ?? Object.values(inp).find(v => typeof v === 'string')
  if (!val) return name
  return `${name}: ${truncate(String(val), 40)}`
}
