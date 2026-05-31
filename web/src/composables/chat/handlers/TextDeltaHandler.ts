import { nextTick } from 'vue'
import type { ChatEvent } from '../../../api/chat'
import type { EventHandler, EventHandlerContext, DisplayMessage } from './EventHandler'

export class TextDeltaHandler implements EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void {
    const convMsgs = context.messagesMap.value[context.convId]
    if (!convMsgs) return

    this.ensureStreamingMessage(convMsgs, context)

    const last = convMsgs[convMsgs.length - 1]
    if (!last || last.role !== 'assistant') return

    const blocks = last.blocks
    const lastIdx = blocks.length - 1
    const lastBlock = blocks[lastIdx]

    if (lastBlock?.type === 'text') {
      blocks[lastIdx] = { type: 'text', content: lastBlock.content + (event.content?.text || '') }
    } else {
      blocks.push({ type: 'text', content: event.content?.text || '' })
    }

    context.updateAgentStatus({
      conversationId: context.convId,
      title: context.getConvTitle(context.convId),
      phase: 'thinking',
    })

    if (context.activeConvId.value === context.convId) {
      nextTick(() => context.scrollToBottom())
    }
  }

  private ensureStreamingMessage(convMsgs: DisplayMessage[], context: EventHandlerContext): void {
    const last = convMsgs[convMsgs.length - 1]
    if (!last || last.role !== 'assistant' || !last.isStreaming) {
      const newMsg: DisplayMessage = {
        id: `a-${Date.now()}`,
        role: 'assistant',
        blocks: [],
        isStreaming: true,
        toolIndex: new Map(),
      }
      convMsgs.push(newMsg)
      context.setConversationStreaming(context.convId, true)
      if (context.activeConvId.value === context.convId) {
        nextTick(() => context.scrollToBottom())
      }
    }
  }
}
