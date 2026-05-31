import { nextTick } from 'vue'
import type { ChatEvent } from '../../../api/chat'
import type { EventHandler, EventHandlerContext, DisplayMessage } from './EventHandler'

export class ConfirmRequiredHandler implements EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void {
    const convMsgs = context.messagesMap.value[context.convId]
    if (!convMsgs) return

    this.ensureStreamingMessage(convMsgs, context)

    const last = convMsgs[convMsgs.length - 1]
    if (!last || last.role !== 'assistant') return

    last.confirm = {
      requestId: event.content?.request_id || '',
      tool: event.content?.tool || '',
      input: event.content?.input || {},
      riskLevel: event.content?.risk_level || 'moderate',
    }

    context.updateAgentStatus({
      conversationId: context.convId,
      title: context.getConvTitle(context.convId),
      phase: 'confirm',
      toolName: event.content?.tool || '',
      toolInput: event.content?.input ? JSON.stringify(event.content.input) : undefined,
    })
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
