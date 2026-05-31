import { Ref } from 'vue'
import { ChatEvent, Todo } from '../../../api/chat'
import type { MessageBlock } from '../../../components/ChatMessage.vue'
import type { AgentStatus } from '../../useAgentStatus'

export interface DisplayMessage {
  id: string
  role: string
  blocks: MessageBlock[]
  confirm?: { requestId: string; tool: string; input: any; riskLevel: string } | null
  isStreaming?: boolean
  toolIndex?: Map<string, number>
}

export interface EventHandlerContext {
  convId: string
  messagesMap: Ref<Record<string, DisplayMessage[]>>
  activeConvId: Ref<string | null>
  getConvTitle: (id: string) => string
  setConversationStreaming: (id: string, streaming: boolean) => void
  scrollToBottom: () => void
  markDevicesExecuting: (hosts: string[]) => void
  markDevicesDone: (hosts: string[], failed: boolean) => void
  updateAgentStatus: (status: Omit<AgentStatus, 'updatedAt' | 'startedAt'>) => void
  clearRetryState: () => void
  queuedMessages: Ref<Map<string, string[]>>
  todoTasksMap: Ref<Record<string, Map<number, Todo>>>
  startTimer: (task: Todo) => void
  stopTimer: (taskId: number) => void
  clearAllTimers: () => void
  turnUsage: Ref<number | null>
  loadConversations: () => Promise<void>
}

export interface EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void
}

export const SSH_TOOLS = new Set(['RunCommand', 'RunCommandBatch'])
