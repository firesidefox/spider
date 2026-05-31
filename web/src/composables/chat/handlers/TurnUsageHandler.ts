import type { ChatEvent } from '../../../api/chat'
import type { EventHandler, EventHandlerContext } from './EventHandler'

export class TurnUsageHandler implements EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void {
    const u = event.content as { output_tokens: number }
    if (!u) return

    if (context.activeConvId.value === context.convId) {
      context.turnUsage.value = u.output_tokens
    }
  }
}
