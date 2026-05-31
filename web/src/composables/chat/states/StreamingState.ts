import { nextTick } from 'vue'
import { BaseConversationState } from './BaseConversationState'
import type { ConversationStateContext } from './ConversationState'
import { cancelConversation, getConversation, type ChatMessage } from '../../../api/chat'
import { IdleState } from './IdleState'
import type { DisplayMessage } from './ConversationState'

export class StreamingState extends BaseConversationState {
  readonly name = 'streaming' as const

  async send(_text: string, _context: ConversationStateContext): Promise<void> {
    throw new Error('Cannot send while streaming. Cancel first.')
  }

  async cancel(context: ConversationStateContext): Promise<void> {
    const { convId, messagesMap, queuedMessages, setConversationStreaming, scrollToBottom } = context

    // Cancel the conversation on the backend
    await cancelConversation(convId)

    // Clear queued messages
    queuedMessages.value.delete(convId)
    queuedMessages.value = new Map(queuedMessages.value)

    // Reload messages from DB to replace the truncated in-memory assistant message
    const data = await getConversation(convId)
    messagesMap.value[convId] = buildDisplayMessages(data.messages)

    // Transition to idle state
    setConversationStreaming(convId, false)
    this.transitionTo(new IdleState(), context)

    await nextTick()
    scrollToBottom()
  }
}

// Helper function to build display messages (simplified version from ChatView)
const toolCallsCache = new Map<string, any[]>()

function buildDisplayMessages(msgs: ChatMessage[]): DisplayMessage[] {
  return msgs.filter(m => m.role !== 'tool_result').map(m => {
    const blocks: any[] = []
    if (m.content) blocks.push({ type: 'text', content: m.content })
    if (m.tool_calls) {
      let parsed = toolCallsCache.get(m.id)
      if (!parsed) {
        try {
          parsed = JSON.parse(m.tool_calls)
          if (parsed) {
            toolCallsCache.set(m.id, parsed)
          }
        } catch { parsed = [] }
      }
      const toolCalls = Array.isArray(parsed) ? parsed : []
      for (const tc of toolCalls) {
        blocks.push({ type: 'tool', call: {
          id: tc.id, name: tc.name, input: tc.input,
          result: tc.result, isError: tc.is_error, durationMs: tc.duration_ms,
          summary: tc.summary, hostNames: tc.host_names,
        }})
      }
    }
    return { id: m.id, role: m.role, blocks } as DisplayMessage
  }).filter(m => m.blocks.length > 0)
}
