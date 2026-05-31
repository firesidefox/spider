type EventCallback = (data?: any) => void

export class ChatEventBus {
  private listeners = new Map<string, Set<EventCallback>>()

  on(event: string, callback: EventCallback): () => void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set())
    }
    this.listeners.get(event)!.add(callback)
    return () => this.off(event, callback)
  }

  off(event: string, callback: EventCallback): void {
    this.listeners.get(event)?.delete(callback)
  }

  emit(event: string, data?: any): void {
    this.listeners.get(event)?.forEach(cb => cb(data))
  }

  clear(): void {
    this.listeners.clear()
  }
}

export const chatEventBus = new ChatEventBus()

export const ChatEvents = {
  SCROLL_TO_BOTTOM: 'scroll:bottom',
  DEVICE_STATUS_UPDATE: 'device:status',
  AGENT_STATUS_UPDATE: 'agent:status',
  TODO_UPDATE: 'todo:update',
  CONVERSATION_SELECTED: 'conversation:selected',
  MESSAGE_SENT: 'message:sent',
} as const
