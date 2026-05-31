import { nextTick } from 'vue'
import { BaseConversationState } from './BaseConversationState'
import type { ConversationStateContext, DisplayMessage } from './ConversationState'
import { sendMessage } from '../../../api/chat'
import { StreamingState } from './StreamingState'

export class IdleState extends BaseConversationState {
  readonly name = 'idle' as const

  async send(text: string, context: ConversationStateContext): Promise<void> {
    const { convId, messagesMap, queuedMessages, setConversationStreaming, scrollToBottom, updateAgentStatus, getConvTitle } = context

    // Get or initialize messages for this conversation
    const convMsgs = messagesMap.value[convId] || []
    if (!messagesMap.value[convId]) {
      messagesMap.value[convId] = convMsgs
    }

    // Create optimistic user and assistant messages
    const userMsg: DisplayMessage = {
      id: `u-${Date.now()}`,
      role: 'user',
      blocks: [{ type: 'text', content: text }],
    }
    const assistantMsg: DisplayMessage = {
      id: `a-${Date.now()}`,
      role: 'assistant',
      blocks: [],
      isStreaming: true,
      toolIndex: new Map(),
    }

    // Optimistically show as queued immediately
    const pushQueued = () => {
      const next = new Map(queuedMessages.value)
      next.set(convId, [...(next.get(convId) ?? []), text])
      queuedMessages.value = next
    }
    const removeQueued = () => {
      const queue = queuedMessages.value.get(convId) ?? []
      const idx = queue.lastIndexOf(text)
      if (idx !== -1) {
        const next = new Map(queuedMessages.value)
        const newQueue = [...queue]
        newQueue.splice(idx, 1)
        if (newQueue.length === 0) next.delete(convId)
        else next.set(convId, newQueue)
        queuedMessages.value = next
      }
    }
    pushQueued()

    try {
      // Send message to backend
      const res = await sendMessage(convId, text, null)

      if (res.status === 'queued') {
        // Backend injected into running agent — keep queued display as-is
      } else {
        // Backend accepted as new agent turn — remove from queued, show optimistic messages
        removeQueued()
        convMsgs.push(userMsg)
        convMsgs.push(assistantMsg)
        setConversationStreaming(convId, true)

        // Update agent status
        if (updateAgentStatus) {
          updateAgentStatus({
            conversationId: convId,
            title: getConvTitle(convId),
            phase: 'thinking',
          })
        }

        await nextTick()
        scrollToBottom()

        // Transition to streaming state
        this.transitionTo(new StreamingState(), context)
      }
    } catch (e: any) {
      // Remove optimistic queued entry on error
      removeQueued()
      if (e?.name === 'AbortError') return
      throw e
    }
  }

  async cancel(_context: ConversationStateContext): Promise<void> {
    throw new Error('Cannot cancel in idle state')
  }
}
