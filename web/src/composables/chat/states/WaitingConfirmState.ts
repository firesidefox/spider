import { BaseConversationState } from './BaseConversationState'
import type { ConversationStateContext } from './ConversationState'
import { confirmAction } from '../../../api/chat'
import { IdleState } from './IdleState'
import { StreamingState } from './StreamingState'

export class WaitingConfirmState extends BaseConversationState {
  readonly name = 'waiting_confirm' as const

  async send(_text: string, _context: ConversationStateContext): Promise<void> {
    throw new Error('Cannot send while waiting for confirmation. Approve or reject first.')
  }

  async cancel(_context: ConversationStateContext): Promise<void> {
    // Cancel = reject the confirmation
    // We need the requestId, which should be stored in the message's confirm field
    // For now, throw an error - the actual implementation should extract requestId from context
    throw new Error('Cancel in waiting_confirm state requires requestId from message')
  }

  async confirm(
    requestId: string,
    approved: boolean,
    context: ConversationStateContext
  ): Promise<void> {
    const { convId, messagesMap, setConversationStreaming } = context

    // Send confirmation to backend
    await confirmAction(convId, requestId, approved)

    // Clear the confirm field from the message
    const convMsgs = messagesMap.value[convId]
    if (convMsgs) {
      const msg = convMsgs.find(m => m.confirm?.requestId === requestId)
      if (msg) {
        msg.confirm = null
      }
    }

    // Transition based on approval
    if (approved) {
      // If approved, agent continues processing → streaming state
      setConversationStreaming(convId, true)
      this.transitionTo(new StreamingState(), context)
    } else {
      // If rejected, agent stops → idle state
      setConversationStreaming(convId, false)
      this.transitionTo(new IdleState(), context)
    }
  }
}
