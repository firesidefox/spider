import { ChatEvent } from '../../../api/chat'
import { EventHandlerContext, EventHandler } from './EventHandler'
import { TextDeltaHandler } from './TextDeltaHandler'
import { ToolStartHandler } from './ToolStartHandler'
import { ToolResultHandler } from './ToolResultHandler'
import { ConfirmRequiredHandler } from './ConfirmRequiredHandler'
import { ErrorHandler } from './ErrorHandler'
import { DoneHandler } from './DoneHandler'
import { TodoUpdateHandler } from './TodoUpdateHandler'
import { TurnUsageHandler } from './TurnUsageHandler'
import { MessageHandler } from './MessageHandler'

export type { EventHandlerContext } from './EventHandler'

export class EventHandlerRegistry {
  private handlers = new Map<string, EventHandler>()

  constructor() {
    this.register('text_delta', new TextDeltaHandler())
    this.register('tool_start', new ToolStartHandler())
    this.register('tool_result', new ToolResultHandler())
    this.register('confirm_required', new ConfirmRequiredHandler())
    this.register('error', new ErrorHandler())
    this.register('done', new DoneHandler())
    this.register('todo_update', new TodoUpdateHandler())
    this.register('turn_usage', new TurnUsageHandler())
    this.register('message', new MessageHandler())
  }

  register(eventType: string, handler: EventHandler): void {
    this.handlers.set(eventType, handler)
  }

  handle(event: ChatEvent, context: EventHandlerContext): void {
    const handler = this.handlers.get(event.type)
    if (handler) {
      try {
        handler.handle(event, context)
      } catch (error) {
        console.error(`Handler error for ${event.type}:`, error)
      }
    } else {
      console.warn(`No handler for event type: ${event.type}`)
    }
  }
}
