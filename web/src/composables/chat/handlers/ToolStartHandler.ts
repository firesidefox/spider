import { nextTick } from 'vue'
import type { ChatEvent } from '../../../api/chat'
import type { EventHandler, EventHandlerContext, DisplayMessage } from './EventHandler'
import { SSH_TOOLS } from './EventHandler'

export class ToolStartHandler implements EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void {
    const convMsgs = context.messagesMap.value[context.convId]
    if (!convMsgs) return

    this.ensureStreamingMessage(convMsgs, context)

    const last = convMsgs[convMsgs.length - 1]
    if (!last || last.role !== 'assistant') return

    const blocks = last.blocks
    if (!last.toolIndex) last.toolIndex = new Map<string, number>()
    const toolIndex = last.toolIndex

    const toolName = event.content?.name || 'unknown'
    const toolId = event.content?.id || `t-${Date.now()}`
    const idx = blocks.length

    blocks.push({
      type: 'tool',
      call: {
        id: toolId,
        name: toolName,
        input: event.content?.input,
        hostNames: event.content?.host_names,
      },
    })

    toolIndex.set(toolId, idx)

    if (SSH_TOOLS.has(toolName) && event.content?.host_names?.length) {
      context.markDevicesExecuting(event.content.host_names)
    }

    context.updateAgentStatus({
      conversationId: context.convId,
      title: context.getConvTitle(context.convId),
      phase: 'tool',
      toolName,
      toolInput: event.content?.input ? JSON.stringify(event.content.input) : undefined,
      hosts: event.content?.host_names,
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
