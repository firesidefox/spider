import { nextTick } from 'vue'
import type { ChatEvent, ChatMessage as ChatMsg } from '../../../api/chat'
import type { EventHandler, EventHandlerContext, DisplayMessage } from './EventHandler'
import type { MessageBlock } from '../../../components/ChatMessage.vue'

export class MessageHandler implements EventHandler {
  private toolCallsCache = new Map<string, any[]>()

  handle(event: ChatEvent, context: EventHandlerContext): void {
    const convMsgs = context.messagesMap.value[context.convId]
    if (!convMsgs) return

    const msg = event.content as ChatMsg
    if (!msg || msg.role === 'user' || msg.role === 'tool_result') return

    if (!convMsgs.find(m => m.id === msg.id)) {
      convMsgs.push(...this.buildDisplayMessages([msg]))
      if (context.activeConvId.value === context.convId) {
        nextTick(() => context.scrollToBottom())
      }
    }
  }

  private buildDisplayMessages(msgs: ChatMsg[]): DisplayMessage[] {
    return msgs.filter(m => m.role !== 'tool_result').map(m => {
      const blocks: MessageBlock[] = []
      if (m.content) blocks.push({ type: 'text', content: m.content })
      if (m.tool_calls) {
        let parsed: any[] = this.toolCallsCache.get(m.id) || []
        if (parsed.length === 0) {
          try {
            const result = JSON.parse(m.tool_calls)
            parsed = Array.isArray(result) ? result : []
            this.toolCallsCache.set(m.id, parsed)
          } catch { parsed = [] }
        }
        for (const tc of parsed) {
          blocks.push({ type: 'tool', call: {
            id: tc.id, name: tc.name, input: tc.input,
            result: tc.result, isError: tc.is_error, durationMs: tc.duration_ms,
            summary: tc.summary, hostNames: tc.host_names,
          }})
        }
      }
      return { id: m.id, role: m.role, blocks, toolIndex: new Map() } as DisplayMessage
    }).filter(m => m.blocks.length > 0)
  }
}
