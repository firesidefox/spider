import type { Ref } from 'vue'
import type { AgentStatus } from '../../useAgentStatus'

export interface DisplayMessage {
  id: string
  role: string
  blocks: any[]
  confirm?: { requestId: string; tool: string; input: any; riskLevel: string } | null
  isStreaming?: boolean
  toolIndex?: Map<string, number>
}

export type AgentStatusUpdate = Omit<AgentStatus, 'updatedAt' | 'startedAt'>

export interface ConversationStateContext {
  convId: string
  messagesMap: Ref<Record<string, DisplayMessage[]>>
  queuedMessages: Ref<Map<string, string[]>>
  streamingConvIds: Ref<Set<string>>
  setConversationStreaming: (id: string, streaming: boolean) => void
  scrollToBottom: () => void
  updateAgentStatus?: (status: AgentStatusUpdate) => void
  getConvTitle: (id: string) => string
}

export interface ConversationState {
  readonly name: 'idle' | 'streaming' | 'waiting_confirm'

  send(text: string, context: ConversationStateContext): Promise<void>
  cancel(context: ConversationStateContext): Promise<void>
  confirm(requestId: string, approved: boolean, context: ConversationStateContext): Promise<void>

  transitionTo(newState: ConversationState, context: ConversationStateContext): void
}
