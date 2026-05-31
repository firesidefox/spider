import { ref, computed, nextTick, type Ref } from 'vue'
import {
  sendMessage as apiSendMessage,
  subscribeConversation,
  getConversation,
  type ChatMessage as ChatMsg,
  type ChatEvent,
  type Todo,
} from '../../api/chat'
import { updateAgentStatus } from '../useAgentStatus'
import { IdleState, StreamingState, WaitingConfirmState, type ConversationState, type ConversationStateContext } from './states'
import { EventHandlerRegistry, type EventHandlerContext } from './handlers/EventHandlerRegistry'
import type { DisplayMessage } from './handlers/EventHandler'
import type { MessageBlock } from '../../components/ChatMessage.vue'

export interface RetryState {
  attempt: number
  maxRetries: number
  error: string
  retryInMs: number
  countdownMs: number
  timer: ReturnType<typeof setInterval> | null
}

export interface UseChatStreamOptions {
  activeConvId: Ref<string | null>
  getConvTitle: (id: string) => string
  onScrollToBottom?: () => void
  onDeviceStatusUpdate?: (hostName: string, status: string) => void
  onLoadConversations?: () => Promise<void>
  todoTasksMap: Ref<Record<string, Map<number, Todo>>>
  startTimer: (task: Todo) => void
  stopTimer: (taskId: number) => void
  clearAllTimers: () => void
  turnUsage: Ref<number | null>
}

export function useChatStream(options: UseChatStreamOptions) {
  const {
    activeConvId,
    getConvTitle,
    onScrollToBottom,
    onDeviceStatusUpdate,
    onLoadConversations,
    todoTasksMap,
    startTimer,
    stopTimer,
    clearAllTimers,
    turnUsage,
  } = options

  // State
  const messagesMap = ref<Record<string, DisplayMessage[]>>({})
  const queuedMessages = ref<Map<string, string[]>>(new Map())
  const streamingConvIds = ref<Set<string>>(new Set())
  const streamingTimeouts = new Map<string, ReturnType<typeof setTimeout>>()
  const retryState = ref<RetryState | null>(null)
  const convSubscriptions = new Map<string, () => void>()
  let pendingSendCtrl: AbortController | null = null

  // Tool calls cache
  const toolCallsCache = new Map<string, any[]>()

  // State Pattern: per-conversation states
  const conversationStates = new Map<string, ConversationState>()

  // Strategy Pattern: event handler registry
  const eventRegistry = new EventHandlerRegistry()

  // Device status tracking
  const deviceResetTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const executingHosts = new Set<string>()

  // Computed
  const messages = computed(() => messagesMap.value[activeConvId.value ?? ''] ?? [])
  const isStreaming = computed({
    get: () => activeConvId.value ? streamingConvIds.value.has(activeConvId.value) : false,
    set: (streaming: boolean) => {
      if (!activeConvId.value) return
      setConversationStreaming(activeConvId.value, streaming)
    },
  })

  // State Pattern helpers
  function getConversationState(convId: string): ConversationState {
    if (!conversationStates.has(convId)) {
      conversationStates.set(convId, new IdleState())
    }
    return conversationStates.get(convId)!
  }

  function setConversationState(convId: string, state: ConversationState): void {
    conversationStates.set(convId, state)
  }

  function buildStateContext(convId: string): ConversationStateContext {
    return {
      convId,
      messagesMap,
      queuedMessages,
      streamingConvIds,
      setConversationStreaming,
      scrollToBottom: () => onScrollToBottom?.(),
      updateAgentStatus,
      getConvTitle,
    }
  }

  // Core functions
  function getOrInitMessages(convId: string): DisplayMessage[] {
    if (!messagesMap.value[convId]) {
      messagesMap.value[convId] = []
    }
    return messagesMap.value[convId]
  }

  function buildDisplayMessages(msgs: ChatMsg[]): DisplayMessage[] {
    return msgs.filter(m => m.role !== 'tool_result').map(m => {
      const blocks: MessageBlock[] = []
      if (m.content) blocks.push({ type: 'text', content: m.content })
      if (m.tool_calls) {
        let parsed = toolCallsCache.get(m.id)
        if (!parsed) {
          try {
            parsed = JSON.parse(m.tool_calls)
            toolCallsCache.set(m.id, parsed ?? [])
          } catch { parsed = [] }
        }
        const toolCalls = (Array.isArray(parsed) ? parsed : []) as any[]
        for (const tc of toolCalls) {
          blocks.push({ type: 'tool', call: {
            id: tc.id, name: tc.name, input: tc.input,
            result: tc.result, isError: tc.is_error, durationMs: tc.duration_ms,
            summary: tc.summary, hostNames: tc.host_names,
          }})
        }
      }
      return { id: m.id, role: m.role, blocks } as DisplayMessage
    }).filter(m => m.blocks.length > 0)
  }

  function setConversationStreaming(convId: string, streaming: boolean) {
    const next = new Set(streamingConvIds.value)
    if (streaming) {
      next.add(convId)
      // Fallback: if SSE done/error never arrives (server crash, network drop),
      // poll DB once after 90s to recover from stuck streaming state.
      if (!streamingTimeouts.has(convId)) {
        streamingTimeouts.set(convId, setTimeout(async () => {
          streamingTimeouts.delete(convId)
          if (!streamingConvIds.value.has(convId)) return
          try {
            const data = await getConversation(convId)
            if (data.conversation.status !== 'processing') {
              messagesMap.value[convId] = buildDisplayMessages(data.messages)
              setConversationStreaming(convId, false)
            }
          } catch { /* ignore — will retry on next user action */ }
        }, 90_000))
      }
    } else {
      next.delete(convId)
      const t = streamingTimeouts.get(convId)
      if (t !== undefined) { clearTimeout(t); streamingTimeouts.delete(convId) }
    }
    streamingConvIds.value = next
  }

  function setDeviceStatus(hostName: string, status: string) {
    onDeviceStatusUpdate?.(hostName, status)
  }

  function markDevicesExecuting(hostNames: string[]) {
    for (const name of hostNames) {
      executingHosts.add(name)
      const t = deviceResetTimers.get(name)
      if (t) { clearTimeout(t); deviceResetTimers.delete(name) }
      setDeviceStatus(name, 'executing')
    }
  }

  function markDevicesDone(hostNames: string[], failed: boolean) {
    const finalStatus = failed ? 'failed' : 'success'
    for (const name of hostNames) {
      setDeviceStatus(name, finalStatus)
      const t = setTimeout(() => {
        executingHosts.delete(name)
        setDeviceStatus(name, 'online')
        deviceResetTimers.delete(name)
      }, 2000)
      deviceResetTimers.set(name, t)
    }
  }

  function scrollToBottom() {
    onScrollToBottom?.()
  }

  // Event handling with Strategy Pattern
  function handleConvEvent(convId: string, event: ChatEvent) {
    const context: EventHandlerContext = {
      convId,
      messagesMap,
      activeConvId,
      getConvTitle,
      setConversationStreaming,
      scrollToBottom,
      markDevicesExecuting,
      markDevicesDone,
      updateAgentStatus,
      clearRetryState: () => { retryState.value = null },
      queuedMessages,
      todoTasksMap,
      startTimer,
      stopTimer,
      clearAllTimers,
      turnUsage,
      loadConversations: onLoadConversations || (async () => {}),
    }

    // Handle mid_turn_user_message separately (not in registry yet)
    if (event.type === 'mid_turn_user_message') {
      handleMidTurnUserMessage(convId, event)
      return
    }

    // Use Strategy Pattern for other events
    eventRegistry.handle(event, context)

    // State transitions based on events
    if (event.type === 'confirm_required') {
      setConversationState(convId, new WaitingConfirmState())
    } else if (event.type === 'done' || event.type === 'error') {
      setConversationState(convId, new IdleState())
    }
  }

  function handleMidTurnUserMessage(convId: string, event: ChatEvent) {
    const text = event.content?.text as string | undefined
    if (!text) return

    // Backend joins multiple queued messages with \n\n.
    // Try exact match first (single message or message containing \n\n).
    // Fall back to removing any queued entries that are exact components of the joined text.
    const convQueue = queuedMessages.value.get(convId) ?? []
    const idx = convQueue.indexOf(text)
    let newQueue: string[]
    if (idx !== -1) {
      newQueue = [...convQueue]
      newQueue.splice(idx, 1)
    } else {
      // Build the expected joined string from queued messages and remove matched ones.
      // Walk from the front: greedily consume queued messages that form a prefix of text.
      const remaining = [...convQueue]
      const consumed: string[] = []
      let joined = ''
      for (let i = 0; i < remaining.length; i++) {
        const candidate = joined ? joined + '\n\n' + remaining[i] : remaining[i]
        if (text === candidate || text.startsWith(candidate + '\n\n')) {
          joined = candidate
          consumed.push(remaining[i])
        }
      }
      if (consumed.length > 0) {
        const consumedSet = new Set(consumed)
        newQueue = convQueue.filter(q => !consumedSet.has(q))
      } else {
        newQueue = convQueue
      }
    }
    const nextMap = new Map(queuedMessages.value)
    if (newQueue.length === 0) nextMap.delete(convId)
    else nextMap.set(convId, newQueue)
    queuedMessages.value = nextMap

    // Insert as a real user message in the conversation
    const convMsgsForInject = messagesMap.value[convId]
    if (convMsgsForInject) {
      // Close any in-progress streaming assistant message before injecting user message
      const prevLast = convMsgsForInject[convMsgsForInject.length - 1]
      if (prevLast?.role === 'assistant' && prevLast.isStreaming) {
        prevLast.isStreaming = false
        for (const b of prevLast.blocks) {
          if (b.type === 'tool' && b.call.durationMs == null) b.call.durationMs = 0
        }
      }
      convMsgsForInject.push({
        id: `u-injected-${Date.now()}`,
        role: 'user',
        blocks: [{ type: 'text', content: text }],
      })
      if (activeConvId.value === convId) nextTick(() => scrollToBottom())
    }
  }

  // Public API
  async function loadConversationMessages(convId: string): Promise<void> {
    const data = await getConversation(convId)
    if (activeConvId.value !== convId) return  // user switched to another conv while loading

    queuedMessages.value.delete(convId)

    // Sync queued messages from backend (in-memory store, survives page refresh within session)
    const next = new Map(queuedMessages.value)
    next.set(convId, data.queued_messages ?? [])
    queuedMessages.value = next

    if (data.conversation.status === 'processing' && messagesMap.value[convId]) {
      // SSE is still writing into messagesMap[id] — don't overwrite it.
      setConversationStreaming(convId, true)
    } else {
      messagesMap.value[convId] = buildDisplayMessages(data.messages)
      setConversationStreaming(convId, data.conversation.status === 'processing')
    }

    if (data.conversation.status === 'processing') {
      updateAgentStatus({ conversationId: convId, title: data.conversation.title || convId.slice(0, 8), phase: 'thinking' })
    }

    if (!convSubscriptions.has(convId)) {
      const lastMsg = data.messages[data.messages.length - 1]
      const unsub = subscribeConversation(convId, (event) => handleConvEvent(convId, event), lastMsg?.id)
      convSubscriptions.set(convId, unsub)
    }

    await nextTick()
    scrollToBottom()
  }

  async function sendMessage(text: string, hostIds?: string[] | null): Promise<void> {
    const convId = activeConvId.value
    if (!convId) return

    // Ensure subscription exists
    if (!convSubscriptions.has(convId)) {
      const unsub = subscribeConversation(convId, (event) => handleConvEvent(convId, event))
      convSubscriptions.set(convId, unsub)
    }

    // Optimistically show as queued immediately; remove if backend starts a new agent turn
    const pushQueued = () => {
      const next = new Map(queuedMessages.value)
      next.set(convId, [...(next.get(convId) ?? []), text])
      queuedMessages.value = next
    }
    const removeQueued = () => {
      const queue = queuedMessages.value.get(convId) ?? []
      const idx = queue.lastIndexOf(text)
      if (idx !== -1) {
        const next = new Map(queuedMessages.value)
        const newQueue = [...queue]
        newQueue.splice(idx, 1)
        if (newQueue.length === 0) next.delete(convId)
        else next.set(convId, newQueue)
        queuedMessages.value = next
      }
    }
    pushQueued()

    pendingSendCtrl = new AbortController()
    try {
      const res = await apiSendMessage(convId, text, hostIds, pendingSendCtrl.signal)
      if (res.status === 'queued') {
        // Backend injected into running agent — keep queued display as-is
      } else {
        // Backend accepted as new agent turn — remove from queued, show optimistic messages
        removeQueued()
        const convMsgs = getOrInitMessages(convId)
        const userMsg: DisplayMessage = {
          id: `u-${Date.now()}`, role: 'user', blocks: [{ type: 'text', content: text }],
        }
        const assistantMsg: DisplayMessage = {
          id: `a-${Date.now()}`, role: 'assistant',
          blocks: [], isStreaming: true, toolIndex: new Map(),
        }
        convMsgs.push(userMsg)
        convMsgs.push(assistantMsg)
        setConversationStreaming(convId, true)
        setConversationState(convId, new StreamingState())
        updateAgentStatus({ conversationId: convId, title: getConvTitle(convId), phase: 'thinking' })
        turnUsage.value = null
        nextTick(() => scrollToBottom())
      }
    } catch (e: any) {
      // Remove optimistic queued entry on error
      removeQueued()
      if (e?.name === 'AbortError') return
      // 503 = LLM not configured; surface it since no SSE event will arrive
      const msg = e?.status === 503
        ? 'LLM 未配置，请先在设置中添加 Provider'
        : `发送失败：${e?.message || 'unknown error'}`
      addSystemMessage(msg, convId)
    } finally {
      pendingSendCtrl = null
    }
  }

  async function cancelSend(): Promise<void> {
    const convId = activeConvId.value
    if (!convId) return

    pendingSendCtrl?.abort()
    pendingSendCtrl = null

    const state = getConversationState(convId)
    const context = buildStateContext(convId)

    try {
      // State Pattern handles the cancellation logic (including DB reload)
      await state.cancel(context)
    } catch (e) {
      console.error('Cancel failed:', e)
    }
  }

  async function handleConfirm(requestId: string, approved: boolean): Promise<void> {
    const convId = activeConvId.value
    if (!convId) return

    const state = getConversationState(convId)
    const context = buildStateContext(convId)

    try {
      // State Pattern handles the confirmation logic (including state transition)
      await state.confirm(requestId, approved, context)
    } catch (e) {
      console.error('Confirm failed:', e)
    }
  }

  function addSystemMessage(content: string, targetConvId?: string) {
    const cid = targetConvId ?? activeConvId.value
    if (cid) {
      getOrInitMessages(cid).push({
        id: Date.now().toString(),
        role: 'assistant',
        blocks: [{ type: 'text', content }],
      })
    }
  }

  function cleanup() {
    convSubscriptions.forEach(unsub => unsub())
    convSubscriptions.clear()
    toolCallsCache.clear()
    streamingTimeouts.forEach(t => clearTimeout(t))
    streamingTimeouts.clear()
    deviceResetTimers.forEach(t => clearTimeout(t))
    deviceResetTimers.clear()
    pendingSendCtrl?.abort()
    pendingSendCtrl = null
  }

  return {
    // State
    messages,
    isStreaming,
    queuedMessages: computed(() => queuedMessages.value),
    retryState: computed(() => retryState.value),
    messagesMap,

    // Operations
    loadConversationMessages,
    sendMessage,
    cancelSend,
    handleConfirm,
    addSystemMessage,
    getOrInitMessages,
    setConversationStreaming,

    // Lifecycle
    cleanup,
  }
}
