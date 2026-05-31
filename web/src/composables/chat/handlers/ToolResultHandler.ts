import type { ChatEvent } from '../../../api/chat'
import type { EventHandler, EventHandlerContext } from './EventHandler'
import type { ToolCallBlock } from '../../../components/ChatMessage.vue'
import { SSH_TOOLS } from './EventHandler'

export class ToolResultHandler implements EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void {
    const convMsgs = context.messagesMap.value[context.convId]
    if (!convMsgs) return

    const last = convMsgs[convMsgs.length - 1]
    if (!last) return

    const toolId = event.content?.id || ''

    // Search backwards through all assistant messages for the one that owns this tool call.
    // After mid_turn_user_message, the last message is a user message, so last.toolIndex won't have it.
    let ownerBlocks = last.role === 'assistant' ? last.blocks : []
    let ownerToolIndex = last.role === 'assistant' && last.toolIndex ? last.toolIndex : new Map<string, number>()

    if (ownerToolIndex.get(toolId) === undefined) {
      for (let i = convMsgs.length - 1; i >= 0; i--) {
        const m = convMsgs[i]
        if (m.role === 'assistant' && m.toolIndex?.has(toolId)) {
          ownerBlocks = m.blocks
          ownerToolIndex = m.toolIndex!
          break
        }
      }
    }

    const idx = ownerToolIndex.get(toolId)
    if (idx !== undefined && idx < ownerBlocks.length) {
      const old = (ownerBlocks[idx] as { type: 'tool'; call: ToolCallBlock }).call
      ownerBlocks[idx] = {
        type: 'tool',
        call: {
          ...old,
          input: event.content?.input ?? old.input,
          result: event.content?.result,
          summary: event.content?.summary,
          isError: event.content?.is_error,
          durationMs: event.content?.duration_ms,
        },
      }

      if (SSH_TOOLS.has(old.name) && old.hostNames?.length) {
        context.markDevicesDone(old.hostNames, !!event.content?.is_error)
      }
    }
  }
}
