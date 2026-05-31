import type { ConversationState, ConversationStateContext } from './ConversationState'

export abstract class BaseConversationState implements ConversationState {
  abstract readonly name: 'idle' | 'streaming' | 'waiting_confirm'

  async send(_text: string, _context: ConversationStateContext): Promise<void> {
    throw new Error(`Cannot send in ${this.name} state`)
  }

  async cancel(_context: ConversationStateContext): Promise<void> {
    throw new Error(`Cannot cancel in ${this.name} state`)
  }

  async confirm(
    _requestId: string,
    _approved: boolean,
    _context: ConversationStateContext
  ): Promise<void> {
    throw new Error(`Cannot confirm in ${this.name} state`)
  }

  transitionTo(_newState: ConversationState, _context: ConversationStateContext): void {
    // Default implementation - subclasses can override for custom transition logic
  }

  /**
   * Safe transition helper with error handling
   */
  protected safeTransition(
    newState: ConversationState,
    context: ConversationStateContext,
    onTransition?: () => void | Promise<void>
  ): void {
    try {
      if (onTransition) {
        const result = onTransition()
        if (result instanceof Promise) {
          result.catch((error) => {
            console.error(`Error during transition from ${this.name} to ${newState.name}:`, error)
          })
        }
      }
      newState.transitionTo(newState, context)
    } catch (error) {
      console.error(`Failed to transition from ${this.name} to ${newState.name}:`, error)
      throw error
    }
  }
}
