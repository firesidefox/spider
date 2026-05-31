import type { ChatEvent, Todo } from '../../../api/chat'
import type { EventHandler, EventHandlerContext } from './EventHandler'

export class TodoUpdateHandler implements EventHandler {
  handle(event: ChatEvent, context: EventHandlerContext): void {
    const task = event.content as Todo
    if (!task) return

    if (!context.todoTasksMap.value[context.convId]) {
      context.todoTasksMap.value[context.convId] = new Map()
    }

    context.todoTasksMap.value[context.convId].set(task.id, task)
    context.todoTasksMap.value[context.convId] = context.todoTasksMap.value[context.convId]

    if (task.status === 'in_progress') {
      context.startTimer(task)
    } else {
      context.stopTimer(task.id)
    }
  }
}
