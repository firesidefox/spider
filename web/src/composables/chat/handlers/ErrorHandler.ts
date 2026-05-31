import { nextTick } from 'vue'
import type { ChatEvent } from '../../../api/chat'
import type { EventHandler, EventHandlerContext } from './EventHandler'

export class ErrorHandler implements EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void {
    const convMsgs = context.messagesMap.value[context.convId]
    if (!convMsgs) return

    const last = convMsgs[convMsgs.length - 1]
    if (!last || last.role !== 'assistant') return

    context.clearRetryState()

    context.queuedMessages.value = new Map(context.queuedMessages.value)
    context.queuedMessages.value.delete(context.convId)

    const errText = `\n\n**Error:** ${event.content?.error || 'unknown error'}`
    const lastBlk = last.blocks[last.blocks.length - 1]

    if (lastBlk?.type === 'text') {
      lastBlk.content += errText
    } else {
      last.blocks.push({ type: 'text', content: errText })
    }

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
  }
}
