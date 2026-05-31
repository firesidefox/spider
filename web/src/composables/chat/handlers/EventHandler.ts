import { Ref } from 'vue'
import { ChatEvent, Todo } from '../../../api/chat'

export interface DisplayMessage {
  id: string
  role: string
  content: string
  streaming?: boolean
}

export interface AgentStatusUpdate {
  status: string
  [key: string]: any
}

export interface EventHandlerContext {
  convId: string
  messagesMap: Ref<Record<string, DisplayMessage[]>>
  activeConvId: Ref<string | null>
  getConvTitle: (id: string) => string
  setConversationStreaming: (id: string, streaming: boolean) => void
  scrollToBottom: () => void
  markDevicesExecuting?: (hosts: string[]) => void
  markDevicesDone?: (hosts: string[], failed: boolean) => void
  updateAgentStatus?: (status: AgentStatusUpdate) => void
  clearRetryState?: () => void
  queuedMessages: Ref<Map<string, string[]>>
  todoTasksMap: Ref<Record<string, Map<number, Todo>>>
  startTimer?: (task: Todo) => void
  stopTimer?: (taskId: number) => void
  clearAllTimers?: () => void
  turnUsage: Ref<number | null>
  loadConversations?: () => Promise<void>
}

export interface EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void
}
