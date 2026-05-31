import { nextTick } from 'vue'
import type { ChatEvent } from '../../../api/chat'
import type { EventHandler, EventHandlerContext } from './EventHandler'

export class DoneHandler implements EventHandler {
  handle(_event: ChatEvent, context: EventHandlerContext): void {
    const convMsgs = context.messagesMap.value[context.convId]
    if (!convMsgs) return

    const last = convMsgs[convMsgs.length - 1]
    if (!last || last.role !== 'assistant') return

    context.clearRetryState()

    context.queuedMessages.value = new Map(context.queuedMessages.value)
    context.queuedMessages.value.delete(context.convId)

    last.isStreaming = false
    context.setConversationStreaming(context.convId, false)

    for (const b of last.blocks) {
      if (b.type === 'tool' && b.call.durationMs == null) {
        b.call.durationMs = 0
      }
    }

    if (context.activeConvId.value === context.convId) {
      nextTick(() => context.scrollToBottom())
    }

    context.updateAgentStatus({
      conversationId: context.convId,
      title: context.getConvTitle(context.convId),
      phase: 'done',
    })

    context.loadConversations()

    delete context.todoTasksMap.value[context.convId]
    context.clearAllTimers()
  }
}
