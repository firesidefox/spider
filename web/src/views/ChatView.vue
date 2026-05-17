<script setup lang="ts">
defineOptions({ name: 'ChatView' })
import { ref, onMounted, onActivated, onDeactivated, onUnmounted, nextTick, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ChatMessage from '../components/ChatMessage.vue'
import type { MessageBlock, ToolCallBlock } from '../components/ChatMessage.vue'
import TargetPanel from '../components/TargetPanel.vue'
import type { DeviceStatus } from '../components/TargetPanel.vue'
import { useTargetHosts } from '../composables/useTargetHosts'
import {
  sendMessage, subscribeConversation, createConversation, listConversations,
  getConversation, deleteConversation, confirmAction, cancelConversation,
  getActiveModel, setActiveModel, updateTitle, exportConversation, getHostStatuses,
  suggestTitle,
  type Conversation, type ChatMessage as ChatMsg, type ChatEvent, type Todo,
} from '../api/chat'
import { listHosts, type Host } from '../api/hosts'
import { authHeaders, getUIPrefs, setUIPrefs } from '../api/auth'
import { listGroups, listDocumentsByGroup, type DocumentGroup, type Document as KbDocument } from '../api/documents'
import { updateAgentStatus, type AgentStatus } from '../composables/useAgentStatus'

const route = useRoute()
const router = useRouter()

interface DisplayMessage {
  id: string
  role: string
  blocks: MessageBlock[]
  confirm?: { requestId: string; tool: string; input: any; riskLevel: string } | null
  isStreaming?: boolean
  toolIndex?: Map<string, number>
}

const conversations = ref<Conversation[]>([])

function getConvTitle(convId: string): string {
  return conversations.value.find(c => c.id === convId)?.title || convId.slice(0, 8)
}

const showExportMenu = ref(false)

async function doExport(format: 'md' | 'json') {
  showExportMenu.value = false
  if (!activeConvId.value) return
  await exportConversation(activeConvId.value, format)
}

const activeConvId = ref<string | null>(null)
const messagesMap = ref<Record<string, DisplayMessage[]>>({})
const messages = computed(() => messagesMap.value[activeConvId.value ?? ''] ?? [])

function getOrInitMessages(convId: string): DisplayMessage[] {
  if (!messagesMap.value[convId]) {
    messagesMap.value[convId] = []
  }
  return messagesMap.value[convId]
}

const toolCallsCache = new Map<string, any[]>()

function buildDisplayMessages(msgs: ChatMsg[]): DisplayMessage[] {
  return msgs.map(m => {
    const blocks: MessageBlock[] = []
    if (m.content) blocks.push({ type: 'text', content: m.content })
    if (m.tool_calls) {
      let parsed = toolCallsCache.get(m.id)
      if (!parsed) {
        try {
          parsed = JSON.parse(m.tool_calls)
          toolCallsCache.set(m.id, parsed)
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
    return { id: m.id, role: m.role, blocks } as DisplayMessage
  }).filter(m => m.blocks.length > 0)
}

let pollTimers: Map<string, ReturnType<typeof setTimeout>> = new Map()

function clearPollTimer(convId: string) {
  const t = pollTimers.get(convId)
  if (t !== undefined) { clearTimeout(t); pollTimers.delete(convId) }
}

function addSystemMessage(content: string) {
  if (activeConvId.value) {
    getOrInitMessages(activeConvId.value).push({
      id: Date.now().toString(),
      role: 'assistant',
      blocks: [{ type: 'text', content }],
    })
  }
}

async function pollUntilIdle(convId: string) {
  const check = async () => {
    try {
      const data = await getConversation(convId)
      if (data.conversation.status === 'idle') {
        pollTimers.delete(convId)
        messagesMap.value[convId] = buildDisplayMessages(data.messages)
        if (activeConvId.value === convId) {
          isStreaming.value = false
          await nextTick()
          scrollToBottom()
        }
      } else {
        pollTimers.set(convId, setTimeout(check, 2000))
      }
    } catch {
      pollTimers.set(convId, setTimeout(check, 2000))
    }
  }
  pollTimers.set(convId, setTimeout(check, 2000))
}

const inputText = ref('')
const isStreaming = ref(false)
const queuedMessages = ref<string[]>([])

const slashCommands = [
  { cmd: '/rename', hint: '[name] — 重命名会话（空=AI生成）' },
  { cmd: '/model', hint: '— 切换模型' },
  { cmd: '/export', hint: '[md|json] — 导出会话' },
]
const slashHint = computed(() => {
  const text = inputText.value
  if (!text.startsWith('/')) return ''
  for (const { cmd, hint } of slashCommands) {
    if (cmd.startsWith(text) && text.length < cmd.length) {
      return cmd.slice(text.length) + ' ' + hint
    }
    if (text === cmd || text.startsWith(cmd + ' ')) {
      return ' ' + hint
    }
  }
  return ''
})

const todoTasksMap = ref<Record<string, Map<number, Todo>>>({})
const completedFolded = ref(true)
const pendingFolded = ref(true)
const turnUsage = ref<number | null>(null)

interface RetryState {
  attempt: number
  maxRetries: number
  error: string
  retryInMs: number
  countdownMs: number
  timer: ReturnType<typeof setInterval> | null
}
const retryState = ref<RetryState | null>(null)

function startRetryCountdown(attempt: number, maxRetries: number, error: string, retryInMs: number) {
  if (retryState.value?.timer) clearInterval(retryState.value.timer)
  const state: RetryState = { attempt, maxRetries, error, retryInMs, countdownMs: 0, timer: null }
  state.timer = setInterval(() => {
    state.countdownMs += 1000
    retryState.value = { ...state }
    if (state.countdownMs >= retryInMs) {
      clearInterval(state.timer!)
      state.timer = null
    }
  }, 1000)
  retryState.value = state
}

function clearRetryState() {
  if (retryState.value?.timer) clearInterval(retryState.value.timer)
  retryState.value = null
}

const taskTimers = ref<Map<number, ReturnType<typeof setInterval>>>(new Map())
const taskElapsed = ref<Map<number, number>>(new Map())

const allTasks = computed(() =>
  Array.from((todoTasksMap.value[activeConvId.value ?? ''] ?? new Map<number, Todo>()).values())
)
const inProgressTasks = computed(() =>
  allTasks.value.filter(t => t.status === 'in_progress').sort((a, b) => a.id - b.id)
)
const pendingTasks = computed(() =>
  allTasks.value.filter(t => t.status === 'pending').sort((a, b) => a.id - b.id)
)
const completedTasks = computed(() =>
  allTasks.value.filter(t => t.status === 'completed').sort((a, b) => a.id - b.id)
)
const visiblePending = computed(() =>
  pendingFolded.value ? pendingTasks.value.slice(0, 2) : pendingTasks.value
)
const hiddenPendingCount = computed(() =>
  pendingTasks.value.length - visiblePending.value.length
)
const visibleCompleted = computed(() =>
  completedFolded.value ? [] : completedTasks.value
)
const hasTasks = computed(() =>
  allTasks.value.length > 0
)
const panelHeader = computed(() => {
  const active = inProgressTasks.value[0]
  const tokenSuffix = turnUsage.value !== null
    ? '  ↓ ' + fmtTokens(turnUsage.value)
    : ''
  if (active) {
    const label = active.active_form ?? active.subject
    const elapsed = taskElapsed.value.get(active.id) ?? 0
    return label + '… (' + fmtElapsed(elapsed) + ')' + tokenSuffix
  }
  const total = allTasks.value.length
  const done = completedTasks.value.length
  return `TASKS ${done}/${total}` + tokenSuffix
})

function fmtElapsed(seconds: number): string {
  if (seconds >= 60) {
    const m = Math.floor(seconds / 60)
    const s = seconds % 60
    return `${m}m ${s}s`
  }
  return `${seconds}s`
}

function fmtTokens(n: number): string {
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k'
  return String(n)
}

function startTimer(task: Todo) {
  if (taskTimers.value.has(task.id)) return
  const startTime = new Date(task.updated_at).getTime()
  const tick = () => {
    taskElapsed.value.set(task.id, Math.floor((Date.now() - startTime) / 1000))
    taskElapsed.value = new Map(taskElapsed.value)
  }
  tick()
  taskTimers.value.set(task.id, setInterval(tick, 1000))
}

function stopTimer(taskId: number) {
  const t = taskTimers.value.get(taskId)
  if (t) { clearInterval(t); taskTimers.value.delete(taskId) }
  taskElapsed.value.delete(taskId)
  taskElapsed.value = new Map(taskElapsed.value)
}

function clearAllTimers() {
  taskTimers.value.forEach(t => clearInterval(t))
  taskTimers.value.clear()
  taskElapsed.value.clear()
}
const messagesRef = ref<HTMLElement | null>(null)
const devices = ref<DeviceStatus[]>([])
const allHosts = ref<Host[]>([])
const { selectedHostIds } = useTargetHosts()

const deviceResetTimers = new Map<string, ReturnType<typeof setTimeout>>()
const SSH_TOOLS = new Set(['RunCommand', 'RunCommandBatch'])
const executingHosts = new Set<string>()
const monitorStatuses = new Map<string, boolean>()

function setDeviceStatus(hostName: string, status: DeviceStatus['status']) {
  const idx = devices.value.findIndex(d => d.name === hostName)
  if (idx === -1) return
  devices.value = devices.value.map((d, i) => i === idx ? { ...d, status } : d)
}

function markDevicesExecuting(hostNames: string[]) {
  for (const name of hostNames) {
    const d = devices.value.find(d => d.name === name)
    if (d) executingHosts.add(d.id)
    const t = deviceResetTimers.get(name)
    if (t) { clearTimeout(t); deviceResetTimers.delete(name) }
    setDeviceStatus(name, 'executing')
  }
}

function markDevicesDone(hostNames: string[], failed: boolean) {
  const finalStatus = failed ? 'failed' : 'success'
  for (const name of hostNames) {
    setDeviceStatus(name, finalStatus)
    const d = devices.value.find(d => d.name === name)
    const t = setTimeout(() => {
      if (d) {
        executingHosts.delete(d.id)
        const monitorOnline = monitorStatuses.get(d.id)
        setDeviceStatus(name, monitorOnline === false ? 'offline' : 'online')
      }
      deviceResetTimers.delete(name)
    }, 2000)
    deviceResetTimers.set(name, t)
  }
}

let abortCtrl: AbortController | null = null
// Per-conversation EventSource subscriptions
const convSubscriptions = new Map<string, () => void>()

let scrollRafId: number | null = null
function scheduleScrollToBottom() {
  if (scrollRafId !== null) return
  scrollRafId = requestAnimationFrame(() => {
    scrollRafId = null
    scrollToBottom()
  })
}

const sidebarOpen = ref(localStorage.getItem('spider-sidebar') !== 'closed')
const sidebarWidth = ref(parseInt(localStorage.getItem('spider-sidebar-width') || '240'))
const isDragging = ref(false)
const chatPageRef = ref<HTMLElement | null>(null)


const targetOpen = ref(true)
const targetWidth = ref(280)
const isTargetDragging = ref(false)
let prefsSaveTimer: ReturnType<typeof setTimeout> | null = null

function savePrefs() {
  if (prefsSaveTimer) clearTimeout(prefsSaveTimer)
  prefsSaveTimer = setTimeout(() => {
    setUIPrefs({ target_panel_open: targetOpen.value, target_panel_width: targetWidth.value })
  }, 500)
}

function toggleTarget() {
  targetOpen.value = !targetOpen.value
  setUIPrefs({ target_panel_open: targetOpen.value, target_panel_width: targetWidth.value })
}

function startTargetDrag(e: MouseEvent) {
  isTargetDragging.value = true
  const startX = e.clientX
  const startWidth = targetWidth.value
  const onMove = (ev: MouseEvent) => {
    const delta = startX - ev.clientX
    const newWidth = Math.min(600, Math.max(180, startWidth + delta))
    targetWidth.value = newWidth
  }
  const onUp = () => {
    isTargetDragging.value = false
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
    savePrefs()
  }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

const showModelPicker = ref(false)
const availableModels = ref<{id: string, display_name: string}[]>([])
const currentModel = ref('')
const currentProvider = ref('')

const activeConv = computed(() =>
  conversations.value.find(c => c.id === activeConvId.value) || null
)

const showModeDropdown = ref(false)
const globalMode = ref('ask')

const effectiveMode = computed(() => {
  const convMode = activeConv.value?.permission_mode
  return convMode || globalMode.value
})

const editingHeaderTitle = ref(false)
const editingConvId = ref<string | null>(null)
const editTitleText = ref('')

function startEditHeaderTitle() {
  if (!activeConv.value) return
  editingHeaderTitle.value = true
  editTitleText.value = activeConv.value.title
}

async function saveHeaderTitle() {
  editingHeaderTitle.value = false
  const text = editTitleText.value.trim()
  if (!activeConv.value || !text || text === activeConv.value.title) return
  await updateTitle(activeConv.value.id, text)
  activeConv.value.title = text
}

function startEditConvTitle(id: string, title: string) {
  editingConvId.value = id
  editTitleText.value = title
}

async function saveConvTitle(id: string) {
  editingConvId.value = null
  const text = editTitleText.value.trim()
  const conv = conversations.value.find(c => c.id === id)
  if (!conv || !text || text === conv.title) return
  await updateTitle(id, text)
  conv.title = text
}

function cancelEdit() {
  editingHeaderTitle.value = false
  editingConvId.value = null
}

function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
  localStorage.setItem('spider-sidebar', sidebarOpen.value ? 'open' : 'closed')
}

function startDrag(e: MouseEvent) {
  isDragging.value = true
  const startX = e.clientX
  const startWidth = sidebarWidth.value

  function onMove(ev: MouseEvent) {
    const newWidth = Math.min(400, Math.max(180, startWidth + ev.clientX - startX))
    sidebarWidth.value = newWidth
  }

  function onUp() {
    isDragging.value = false
    localStorage.setItem('spider-sidebar-width', String(sidebarWidth.value))
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }

  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}


async function loadConversations() {
  conversations.value = await listConversations()
}

async function selectConversation(id: string) {
  clearPollTimer(id)
  const data = await getConversation(id)
  activeConvId.value = id
  const taskMap = new Map<number, Todo>()
  for (const t of data.todo_tasks ?? []) taskMap.set(t.id, t)
  todoTasksMap.value[id] = taskMap
  clearAllTimers()
  turnUsage.value = null
  completedFolded.value = true
  pendingFolded.value = true
  for (const task of taskMap.values()) {
    if (task.status === 'in_progress') startTimer(task)
  }
  localStorage.setItem('spider-last-conv', id)
  router.replace(`/chat/${id}`)

  if (data.conversation.status === 'processing' && messagesMap.value[id]) {
    // SSE is still writing into messagesMap[id] — don't overwrite it.
    isStreaming.value = true
  } else {
    messagesMap.value[id] = buildDisplayMessages(data.messages)
    isStreaming.value = data.conversation.status === 'processing'
    if (data.conversation.status === 'processing') {
      pollUntilIdle(id)
    }
  }

  if (data.conversation.status === 'processing') {
    updateAgentStatus({ conversationId: id, title: data.conversation.title || id.slice(0, 8), phase: 'thinking' })
  }

  if (!convSubscriptions.has(id)) {
    const lastMsg = data.messages[data.messages.length - 1]
    const unsub = subscribeConversation(id, (event) => handleConvEvent(id, event), lastMsg?.id)
    convSubscriptions.set(id, unsub)
  }

  await nextTick()
  scrollToBottom()
}

async function createNewConversation() {
  const conv = await createConversation()
  conversations.value.unshift(conv)
  activeConvId.value = conv.id
  messagesMap.value[conv.id] = []
  router.replace(`/chat/${conv.id}`)
  await nextTick()
  textareaRef.value?.focus()
}

function scrollToBottom() {
  if (messagesRef.value) {
    messagesRef.value.scrollTop = messagesRef.value.scrollHeight
  }
}

async function cancelSend() {
  const convId = activeConvId.value
  if (!convId) return
  abortCtrl?.abort()
  abortCtrl = null
  await cancelConversation(convId)
  isStreaming.value = false
  // Reload from DB to replace the truncated in-memory assistant message
  const data = await getConversation(convId)
  messagesMap.value[convId] = buildDisplayMessages(data.messages)
  await nextTick()
  scrollToBottom()
  flushQueue()
}

function handleConvEvent(convId: string, event: ChatEvent) {
  const convMsgs = messagesMap.value[convId]
  if (!convMsgs) return

  function setStatus(phase: AgentStatus['phase'], toolName?: string, toolInput?: unknown) {
    updateAgentStatus({
      conversationId: convId,
      title: getConvTitle(convId),
      phase,
      toolName,
      toolInput: toolInput ? JSON.stringify(toolInput) : undefined,
    })
  }

  if (event.type === 'message') {
    const msg = event.content as ChatMsg
    if (!msg || msg.role === 'user' || msg.role === 'tool_result') return
    if (!convMsgs.find(m => m.id === msg.id)) {
      convMsgs.push(...buildDisplayMessages([msg]))
      if (activeConvId.value === convId) nextTick(() => scrollToBottom())
    }
    return
  }

  // Ensure there's a streaming assistant message to receive live events.
  // Passive tabs (multi-tab sync) never call send(), so we create one on demand.
  if (['text_delta', 'tool_start', 'confirm_required'].includes(event.type)) {
    const last = convMsgs[convMsgs.length - 1]
    if (!last || last.role !== 'assistant' || !last.isStreaming) {
      const newMsg: DisplayMessage = { id: `a-${Date.now()}`, role: 'assistant', blocks: [], isStreaming: true, toolIndex: new Map() }
      convMsgs.push(newMsg)
      if (activeConvId.value === convId) {
        isStreaming.value = true
        nextTick(() => scrollToBottom())
      }
    }
  }

  const last = convMsgs[convMsgs.length - 1]
  if (!last || last.role !== 'assistant') return
  const blocks = last.blocks
  if (!last.toolIndex) last.toolIndex = new Map<string, number>()
  const toolIndex = last.toolIndex

  switch (event.type) {
    case 'text_delta': {
      const lastIdx = blocks.length - 1
      const lastBlock = blocks[lastIdx]
      if (lastBlock?.type === 'text') {
        blocks[lastIdx] = { type: 'text', content: lastBlock.content + (event.content?.text || '') }
      } else {
        blocks.push({ type: 'text', content: event.content?.text || '' })
      }
      setStatus('thinking')
      break
    }
    case 'tool_start': {
      const toolName = event.content?.name || 'unknown'
      const toolId = event.content?.id || `t-${Date.now()}`
      const idx = blocks.length
      blocks.push({ type: 'tool', call: {
        id: toolId,
        name: toolName,
        input: event.content?.input,
        hostNames: event.content?.host_names,
      }})
      toolIndex.set(toolId, idx)
      if (SSH_TOOLS.has(toolName) && event.content?.host_names?.length) {
        markDevicesExecuting(event.content.host_names)
      }
      setStatus('tool', toolName, event.content?.input)
      break
    }
    case 'tool_result': {
      const idx = toolIndex.get(event.content?.id || '')
      if (idx !== undefined && idx < blocks.length) {
        const old = (blocks[idx] as { type: 'tool'; call: ToolCallBlock }).call
        blocks[idx] = { type: 'tool', call: {
          ...old,
          input: event.content?.input ?? old.input,
          result: event.content?.result,
          summary: event.content?.summary,
          isError: event.content?.is_error,
          durationMs: event.content?.duration_ms,
        }}
        if (SSH_TOOLS.has(old.name) && old.hostNames?.length) {
          markDevicesDone(old.hostNames, !!event.content?.is_error)
        }
      }
      break
    }
    case 'confirm_required':
      last.confirm = {
        requestId: event.content?.request_id || '',
        tool: event.content?.tool || '',
        input: event.content?.input || {},
        riskLevel: event.content?.risk_level || 'moderate',
      }
      setStatus('confirm', event.content?.tool || '', event.content?.input)
      break
    case 'error': {
      clearRetryState()
      const errText = `\n\n**Error:** ${event.content?.error || 'unknown error'}`
      const lastBlk = last.blocks[last.blocks.length - 1]
      if (lastBlk?.type === 'text') {
        lastBlk.content += errText
      } else {
        last.blocks.push({ type: 'text', content: errText })
      }
      last.isStreaming = false
      for (const b of last.blocks) {
        if (b.type === 'tool' && b.call.durationMs == null) b.call.durationMs = 0
      }
      if (activeConvId.value === convId) {
        isStreaming.value = false
        nextTick(() => scrollToBottom())
        flushQueue()
      }
      break
    }
    case 'done':
      clearRetryState()
      last.isStreaming = false
      for (const b of last.blocks) {
        if (b.type === 'tool' && b.call.durationMs == null) b.call.durationMs = 0
      }
      if (activeConvId.value === convId) {
        isStreaming.value = false
        nextTick(() => scrollToBottom())
        flushQueue()
      }
      setStatus('done')
      loadConversations()
      delete todoTasksMap.value[convId]
      clearAllTimers()
      break
    case 'todo_update': {
      const task = event.content as Todo
      if (!todoTasksMap.value[convId]) todoTasksMap.value[convId] = new Map()
      todoTasksMap.value[convId].set(task.id, task)
      todoTasksMap.value[convId] = todoTasksMap.value[convId]
      if (task.status === 'in_progress') {
        startTimer(task)
      } else {
        stopTimer(task.id)
      }
      break
    }
    case 'turn_usage': {
      const u = event.content as { output_tokens: number }
      turnUsage.value = u.output_tokens
      break
    }
    case 'retrying': {
      const r = event.content as { attempt: number; max_retries: number; error: string; retry_in_ms: number }
      if (r.attempt >= 3) {
        startRetryCountdown(r.attempt, r.max_retries, r.error, r.retry_in_ms)
      }
      break
    }
  }
  if (activeConvId.value === convId) {
    scheduleScrollToBottom()
  }
}

async function send(overrideText?: string) {
  const text = (overrideText ?? inputText.value).trim()
  if (!text) return

  // slash commands only when not called from queue flush
  if (!overrideText) {
    if (text === '/model') {
      inputText.value = ''
      await handleModelCommand()
      return
    }
    if (text === '/export' || text.startsWith('/export ')) {
      const fmt = parseExportFormat(text)
      if (fmt === 'invalid') {
        addSystemMessage('用法：/export [md|json] 或 /export --format [md|json]')
        return
      }
      inputText.value = ''
      if (!activeConvId.value) {
        addSystemMessage('没有活跃的会话')
        return
      }
      await exportConversation(activeConvId.value, fmt === 'default' ? 'md' : fmt)
      return
    }
    if (text === '/rename' || text.startsWith('/rename ')) {
      inputText.value = ''
      if (!activeConvId.value) {
        addSystemMessage('没有活跃的会话')
        return
      }
      await handleRenameCommand(text)
      return
    }
  }

  // enqueue when streaming (only for direct user input, not flush)
  if (isStreaming.value && !overrideText) {
    queuedMessages.value.push(text)
    inputText.value = ''
    nextTick(() => {
      if (textareaRef.value) textareaRef.value.style.height = 'auto'
    })
    return
  }

  if (!overrideText) {
    inputText.value = ''
    nextTick(() => {
      if (textareaRef.value) textareaRef.value.style.height = 'auto'
    })
  }

  if (!activeConvId.value) {
    await createNewConversation()
  }

  const convId = activeConvId.value!
  const convMsgs = getOrInitMessages(convId)

  if (!convSubscriptions.has(convId)) {
    const unsub = subscribeConversation(convId, (event) => handleConvEvent(convId, event))
    convSubscriptions.set(convId, unsub)
  }

  const userMsg: DisplayMessage = {
    id: `u-${Date.now()}`, role: 'user', blocks: [{ type: 'text', content: text }],
  }
  convMsgs.push(userMsg)

  const assistantMsg: DisplayMessage = {
    id: `a-${Date.now()}`, role: 'assistant',
    blocks: [], isStreaming: true,
  }
  convMsgs.push(assistantMsg)
  isStreaming.value = true
  turnUsage.value = null
  await nextTick()
  scrollToBottom()

  abortCtrl = sendMessage(convId, text, selectedHostIds.value)
}

function flushQueue() {
  if (queuedMessages.value.length === 0) return
  const merged = queuedMessages.value.join('\n\n')
  queuedMessages.value = []
  send(merged)
}

function parseExportFormat(text: string): 'md' | 'json' | 'invalid' | 'default' {
  const rest = text.slice('/export'.length).trim()
  if (rest === '') return 'default'
  if (rest === 'md' || rest === 'json') return rest
  const m = rest.match(/^--format\s+(md|json)$/)
  if (m) return m[1] as 'md' | 'json'
  return 'invalid'
}

async function handleModelCommand() {
  try {
    const { provider_id, model, provider_name } = await getActiveModel()
    currentProvider.value = provider_id
    currentModel.value = model

    if (!currentProvider.value) {
      addSystemMessage('未配置模型供应商。请在 个人设置 → 模型供应商 中配置。')
      return
    }

    const res = await fetch(`/api/v1/providers/${provider_id}/models`, { headers: (await import('../api/auth')).authHeaders() })
    if (!res.ok) throw new Error('获取模型列表失败')
    const models = await res.json()
    availableModels.value = models.map((m: any) => ({ id: m.model_id, display_name: m.display_name }))
    showModelPicker.value = true
  } catch (e: any) {
    addSystemMessage(`获取模型列表失败: ${e.message}`)
  }
}

async function selectModel(modelId: string) {
  try {
    await setActiveModel(currentProvider.value, modelId)
    currentModel.value = modelId
    showModelPicker.value = false
    addSystemMessage(`模型已切换为 **${modelId}**`)
  } catch (e: any) {
    addSystemMessage(`切换模型失败: ${e.message}`)
  }
}

async function handleConfirm(requestId: string, approved: boolean) {
  if (!activeConvId.value) return
  await confirmAction(activeConvId.value, requestId, approved)
  const msg = messages.value.find(m => m.confirm?.requestId === requestId)
  if (msg) msg.confirm = null
}

async function applyConvTitle(id: string, title: string) {
  await updateTitle(id, title)
  const conv = conversations.value.find(c => c.id === id)
  if (conv) conv.title = title
}

async function handleRenameCommand(text: string) {
  const id = activeConvId.value!
  const arg = text.slice('/rename'.length).trim()
  if (arg) {
    try {
      await applyConvTitle(id, arg)
      addSystemMessage(`已重命名为 **${arg}**`)
    } catch (e: any) {
      addSystemMessage(`重命名失败: ${e.message}`)
    }
    return
  }
  try {
    addSystemMessage('正在生成命名建议…')
    const title = await suggestTitle(id)
    await applyConvTitle(id, title)
    addSystemMessage(`已重命名为 **${title}**`)
  } catch (e: any) {
    addSystemMessage(`生成命名失败: ${e.message}`)
  }
}

async function handleDeleteConversation(id: string) {
  await deleteConversation(id)
  conversations.value = conversations.value.filter(c => c.id !== id)
  clearPollTimer(id)
  const unsub = convSubscriptions.get(id)
  if (unsub) { unsub(); convSubscriptions.delete(id) }
  // 清理该会话的 tool_calls 缓存
  const msgs = messagesMap.value[id] || []
  for (const m of msgs) toolCallsCache.delete(m.id)
  if (activeConvId.value === id) {
    activeConvId.value = null
    delete messagesMap.value[id]
    router.replace('/chat')
  }
}

async function loadDevices() {
  const [hosts, statuses] = await Promise.all([listHosts(), getHostStatuses()])
  allHosts.value = hosts
  const statusMap = new Map(statuses.map(s => [s.host_id, s.online]))
  statuses.forEach(s => monitorStatuses.set(s.host_id, s.online))
  devices.value = hosts.map(h => ({
    id: h.id, name: h.name, ip: h.ip,
    vendor: '', status: (statusMap.get(h.id) === false ? 'offline' : 'online') as DeviceStatus['status'],
  }))
}

async function setConversationMode(mode: string) {
  const convId = activeConv.value?.id
  if (!convId) return
  try {
    await fetch(`/api/v1/chat/conversations/${convId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ permission_mode: mode }),
    })
    if (activeConv.value) {
      activeConv.value.permission_mode = mode
    }
  } catch (e) {
    console.error('Failed to set mode:', e)
  }
  showModeDropdown.value = false
}

function closeModeDropdown(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (!target.closest('.mode-badge-wrapper')) {
    showModeDropdown.value = false
  }
}

// @kb two-step dropdown
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const kbDropdownMode = ref<'groups' | 'docs' | null>(null)
const kbGroups = ref<DocumentGroup[]>([])
const kbDocs = ref<KbDocument[]>([])
const kbSelectedGroup = ref<DocumentGroup | null>(null)
const kbActiveIndex = ref(0)
const kbTriggerStart = ref(0)

const kbItems = computed(() =>
  kbDropdownMode.value === 'groups' ? kbGroups.value : kbDocs.value
)

function getKbItemLabel(item: DocumentGroup | KbDocument): string {
  return (item as any).name ?? (item as KbDocument).title
}

function autoResizeTextarea(el: HTMLTextAreaElement) {
  el.style.height = 'auto'
  el.style.height = el.scrollHeight + 'px'
}

async function onTextareaInput() {
  const el = textareaRef.value
  if (!el) return
  autoResizeTextarea(el)
  const pos = el.selectionStart
  const before = el.value.slice(0, pos)

  // Match @kb:groupName/ → show docs
  const docMatch = before.match(/@kb:([^/\s]+)\/$/)
  if (docMatch) {
    const groupName = docMatch[1]
    const group = kbGroups.value.find(g => g.name === groupName)
      || (await loadGroupsIfNeeded(), kbGroups.value.find(g => g.name === groupName))
    if (group) {
      kbSelectedGroup.value = group
      try {
        kbDocs.value = await listDocumentsByGroup(group.id)
      } catch (e) {
        console.error('Failed to load kb docs:', e)
        return
      }
      if (kbDocs.value.length > 0) {
        kbDropdownMode.value = 'docs'
        kbActiveIndex.value = 0
        kbTriggerStart.value = before.lastIndexOf('@kb:')
      }
      return
    }
  }

  // Match @kb or @kb: → show groups
  const groupMatch = before.match(/@kb:?$/)
  if (groupMatch) {
    await loadGroupsIfNeeded()
    if (kbGroups.value.length > 0) {
      kbDropdownMode.value = 'groups'
      kbActiveIndex.value = 0
      kbTriggerStart.value = before.lastIndexOf('@kb')
    }
    return
  }

  kbDropdownMode.value = null
}

async function loadGroupsIfNeeded() {
  if (kbGroups.value.length === 0) {
    try {
      kbGroups.value = await listGroups()
    } catch (e) {
      console.error('Failed to load kb groups:', e)
      return
    }
  }
}

function onTextareaKeydown(e: KeyboardEvent) {
  if (e.key === 'Tab' && slashHint.value && inputText.value.startsWith('/')) {
    e.preventDefault()
    const text = inputText.value
    for (const { cmd } of slashCommands) {
      if (cmd.startsWith(text) && text.length <= cmd.length) {
        inputText.value = cmd + ' '
        return
      }
    }
    return
  }
  if (!kbDropdownMode.value) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    kbActiveIndex.value = (kbActiveIndex.value + 1) % kbItems.value.length
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    kbActiveIndex.value = (kbActiveIndex.value - 1 + kbItems.value.length) % kbItems.value.length
  } else if (e.key === 'Enter') {
    e.preventDefault()
    selectKbItem(kbItems.value[kbActiveIndex.value])
    e.stopImmediatePropagation()
  } else if (e.key === 'Escape') {
    kbDropdownMode.value = null
  }
}

function selectKbItem(item: DocumentGroup | KbDocument) {
  const el = textareaRef.value
  if (!el) return
  const pos = el.selectionStart
  const before = el.value.slice(0, pos)
  const after = el.value.slice(pos)

  if (kbDropdownMode.value === 'groups') {
    const group = item as DocumentGroup
    kbSelectedGroup.value = group
    const replacement = `@kb:${group.name}/`
    const newBefore = before.slice(0, kbTriggerStart.value) + replacement
    inputText.value = newBefore + after
    kbDropdownMode.value = null
    nextTick(() => {
      el.selectionStart = el.selectionEnd = newBefore.length
      el.focus()
    })
  } else {
    if (!kbSelectedGroup.value) {
      kbDropdownMode.value = null
      return
    }
    const doc = item as KbDocument
    const replacement = `@kb:${kbSelectedGroup.value.name}/${doc.title} `
    const newBefore = before.slice(0, kbTriggerStart.value) + replacement
    inputText.value = newBefore + after
    kbDropdownMode.value = null
    nextTick(() => {
      el.selectionStart = el.selectionEnd = newBefore.length
      el.focus()
    })
  }
}

let initialized = false

async function initView() {
  await Promise.all([loadConversations(), loadDevices()])
  getActiveModel().then(m => { currentModel.value = m.model })
  const paramId = route.params.id as string | undefined
  if (paramId) {
    if (paramId !== activeConvId.value) {
      await selectConversation(paramId)
    }
  } else {
    const lastConvId = localStorage.getItem('spider-last-conv')
    if (lastConvId) {
      router.replace(`/chat/${lastConvId}`)
      await selectConversation(lastConvId)
    }
  }
  try {
    const res = await fetch('/api/v1/settings', { headers: authHeaders() })
    const data = await res.json()
    globalMode.value = data.permission_mode || 'ask'
  } catch (_) { /* use default */ }
  await loadPrefs()
  initialized = true
}

async function loadPrefs() {
  try {
    const prefs = await getUIPrefs()
    targetOpen.value = prefs.target_panel_open
    targetWidth.value = prefs.target_panel_width || 280
  } catch (_) { /* use default */ }
}

let globalEs: EventSource | null = null

function startGlobalSSE() {
  globalEs = new EventSource('/api/v1/stream')
  globalEs.onmessage = (e) => {
    try {
      const event = JSON.parse(e.data)
      if (event.type === 'host_status') {
        const { host_id, online } = event.content
        monitorStatuses.set(host_id, online)
        if (!executingHosts.has(host_id)) {
          const idx = devices.value.findIndex(d => d.id === host_id)
          if (idx !== -1 && devices.value[idx].status !== (online ? 'online' : 'offline')) {
            devices.value = devices.value.map((d, i) =>
              i === idx ? { ...d, status: online ? 'online' : 'offline' } : d
            )
          }
        }
      }
    } catch {}
  }
  globalEs.onerror = () => {}
}

onMounted(() => {
  startGlobalSSE()
  document.addEventListener('click', closeModeDropdown)
  if (!initialized) { initialized = true; initView() }
  else loadPrefs()
})
onActivated(() => {
  document.addEventListener('click', closeModeDropdown)
  if (!initialized) { initialized = true; initView() }
  else loadPrefs()
})

onDeactivated(() => {
  clearAllTimers()
  pollTimers.forEach((t) => clearTimeout(t))
  pollTimers.clear()
  document.removeEventListener('click', closeModeDropdown)
})

onUnmounted(() => {
  globalEs?.close()
  clearAllTimers()
  convSubscriptions.forEach(unsub => unsub())
  convSubscriptions.clear()
  toolCallsCache.clear()
})
</script>

<template>
  <div class="chat-page" ref="chatPageRef" :class="{ dragging: isDragging || isTargetDragging }">
    <!-- Sidebar -->
    <div class="sidebar" :class="{ collapsed: !sidebarOpen }" :style="{ width: sidebarOpen ? sidebarWidth + 'px' : '0' }">
      <div class="sidebar-header">
        <button class="sidebar-toggle" @click="toggleSidebar">≡</button>
        <div class="sidebar-tabs">
          <button class="sidebar-tab active">对话</button>
        </div>
        <button class="sidebar-new" @click="createNewConversation()">+</button>
      </div>
      <div class="sidebar-body">
        <div v-for="c in conversations" :key="c.id" class="conv-item"
             :class="{ active: c.id === activeConvId }"
             @click="selectConversation(c.id)">
          <input v-if="editingConvId === c.id" class="conv-item-input"
                 v-model="editTitleText"
                 @keydown.enter="saveConvTitle(c.id)"
                 @keydown.escape="cancelEdit"
                 @blur="saveConvTitle(c.id)"
                 @click.stop
                 @vue:mounted="($event: any) => $event.el.focus()" />
          <span v-else class="conv-item-title" @dblclick.stop="startEditConvTitle(c.id, c.title)">{{ c.title || '未命名对话' }}</span>
          <span v-if="c.status === 'processing'" class="conv-processing-dot" title="处理中"></span>
          <button class="conv-del" @click.stop="handleDeleteConversation(c.id)">×</button>
        </div>
      </div>
    </div>
    <div class="sidebar-resize-handle" @mousedown="startDrag">
      <div class="drag-indicator"></div>
    </div>

    <!-- Chat main -->
    <div class="chat-main" @click="showExportMenu = false; showModeDropdown = false">
      <div class="chat-header">
        <button v-if="!sidebarOpen" class="sidebar-toggle" @click="toggleSidebar">≡</button>
        <button v-if="!sidebarOpen" class="header-new-btn" @click="createNewConversation()">+</button>
        <input v-if="editingHeaderTitle" class="conv-title-input"
               v-model="editTitleText"
               @keydown.enter="saveHeaderTitle"
               @keydown.escape="cancelEdit"
               @blur="saveHeaderTitle"
               @vue:mounted="($event: any) => $event.el.focus()" />
        <span v-else class="conv-title" @click="startEditHeaderTitle">{{ activeConv?.title || '新对话' }}</span>
        <span class="current-model" v-if="currentModel">{{ currentModel }}</span>
        <div class="mode-badge-wrapper">
          <div class="mode-badge" :class="effectiveMode" @click.stop="showModeDropdown = !showModeDropdown">
            {{ effectiveMode }}
          </div>
          <div v-if="showModeDropdown" class="mode-dropdown">
            <div v-for="m in ['ask','auto','plan','readonly']" :key="m"
                 class="mode-option" :class="{ active: effectiveMode === m }"
                 @click.stop="setConversationMode(m)">
              {{ m }}
            </div>
            <div class="mode-option reset" @click.stop="setConversationMode('')">
              使用全局默认
            </div>
          </div>
        </div>
        <div v-if="activeConv" class="export-wrapper">
          <button class="export-btn" @click.stop="showExportMenu = !showExportMenu">导出</button>
          <div v-if="showExportMenu" class="export-menu">
            <div class="export-option" @click="doExport('md')">Markdown</div>
            <div class="export-option" @click="doExport('json')">JSON</div>
          </div>
        </div>
        <button v-if="!targetOpen" class="sidebar-toggle" @click="toggleTarget">‹</button>
      </div>

      <div class="chat-messages" ref="messagesRef">
        <ChatMessage
          v-for="msg in messages" :key="msg.id"
          :role="msg.role" :blocks="msg.blocks"
          :confirm="msg.confirm"
          :is-streaming="msg.isStreaming"
          @confirm="handleConfirm"
        />
        <div v-if="messages.length === 0" class="empty-state">
          输入消息开始对话...
        </div>
      </div>

      <div v-if="showModelPicker" class="model-picker">
        <div class="model-picker-header">
          <span>当前模型: <strong>{{ currentModel || '未选择' }}</strong></span>
          <button class="btn btn-sm" @click="showModelPicker = false">关闭</button>
        </div>
        <div class="model-picker-list">
          <div v-for="m in availableModels" :key="m.id"
               class="model-picker-item"
               :class="{ active: m.id === currentModel }"
               @click="selectModel(m.id)">
            <span>{{ m.display_name || m.id }}</span>
            <span v-if="m.id === currentModel" class="model-check">✓ 当前</span>
          </div>
        </div>
      </div>

      <div v-if="hasTasks" class="todo-panel">
        <div class="todo-header">{{ panelHeader }}</div>

        <div v-for="task in inProgressTasks" :key="task.id" class="todo-row todo-in_progress">
          <span class="todo-icon">●</span>
          <span class="todo-subject">{{ task.seq }}: {{ task.subject }}</span>
        </div>

        <div v-for="task in visiblePending" :key="task.id" class="todo-row todo-pending">
          <span class="todo-icon">○</span>
          <span class="todo-subject">{{ task.seq }}: {{ task.subject }}</span>
        </div>
        <div v-if="hiddenPendingCount > 0" class="todo-row todo-fold" @click="pendingFolded = !pendingFolded">
          <span class="todo-icon">○</span>
          <span class="todo-subject">+{{ hiddenPendingCount }} more{{ pendingFolded ? '' : ' ▲' }}</span>
        </div>

        <div v-if="completedTasks.length > 0" class="todo-row todo-fold" @click="completedFolded = !completedFolded">
          <span class="todo-icon">✓</span>
          <span class="todo-subject">+{{ completedTasks.length }} completed{{ completedFolded ? '' : ' ▲' }}</span>
        </div>
        <div v-for="task in visibleCompleted" :key="task.id" class="todo-row todo-completed todo-completed-indent">
          <span class="todo-icon">✓</span>
          <span class="todo-subject">{{ task.seq }}: {{ task.subject }}</span>
        </div>
      </div>

      <div v-if="retryState" class="retry-banner">
        <span class="retry-error">{{ retryState.error }}</span>
        <span class="retry-countdown">Retrying in {{ Math.max(0, Math.round((retryState.retryInMs - retryState.countdownMs) / 1000)) }}s… (attempt {{ retryState.attempt }}/{{ retryState.maxRetries }})</span>
      </div>

      <div v-for="(qm, i) in queuedMessages" :key="`queued-${i}`" class="queued-message">
        <span class="queued-message-text">{{ i + 1 }}: {{ qm }}</span>
      </div>

      <div class="chat-input">
        <div class="input-wrapper">
          <div v-if="kbDropdownMode" class="kb-dropdown">
            <div
              v-for="(item, i) in kbItems" :key="getKbItemLabel(item)"
              class="kb-dropdown-item"
              :class="{ active: i === kbActiveIndex }"
              @mousedown.prevent="selectKbItem(item)"
            >{{ getKbItemLabel(item) }}</div>
          </div>
          <textarea
            ref="textareaRef"
            v-model="inputText"
            @keydown.enter.exact.prevent="send()"
            @keydown="onTextareaKeydown"
            @input="onTextareaInput"
            :placeholder="isStreaming ? '排队发送...' : '输入运维指令...'"
            rows="1"
          ></textarea>
          <span v-if="slashHint" class="slash-hint" aria-hidden="true">{{ inputText }}<span class="ghost">{{ slashHint }}</span></span>
        </div>
        <button v-if="isStreaming" @click="cancelSend" class="send-btn cancel-btn">取消</button>
        <button v-if="isStreaming" @click="send()" :disabled="!inputText.trim()" class="send-btn queue-btn">排队</button>
        <button v-if="!isStreaming" @click="send()" :disabled="!inputText.trim()" class="send-btn">发送</button>
      </div>
    </div>

    <!-- Right target panel resize handle -->
    <div v-if="targetOpen" class="target-resize-handle" @mousedown="startTargetDrag">
      <div class="drag-indicator"></div>
    </div>

    <!-- Right target panel -->
    <div class="target-side" :class="{ collapsed: !targetOpen }" :style="{ width: targetOpen ? targetWidth + 'px' : '0' }">
      <div class="target-side-header">
        <span class="target-side-title">目标</span>
        <button class="target-toggle" @click="toggleTarget">›</button>
      </div>
      <div class="target-side-body">
        <TargetPanel :devices="devices" :allHosts="allHosts" v-model="selectedHostIds" />
      </div>
    </div>

  </div>
</template>

<style scoped>
.chat-page { display: flex; height: 100%; gap: 0; }
.chat-page.dragging { user-select: none; cursor: col-resize; }

/* Sidebar */
.sidebar { border-right: 1px solid var(--border); display: flex; flex-direction: column; background: var(--panel); transition: width 0.2s ease, opacity 0.2s ease; overflow: hidden; flex-shrink: 0; min-width: 0; }
.sidebar.collapsed { width: 0 !important; border-right: none; opacity: 0; }
.sidebar-header { display: flex; align-items: center; gap: 6px; padding: 8px 10px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.sidebar-toggle { background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 14px; flex-shrink: 0; }
.sidebar-toggle:hover { background: var(--row-hover); }
.sidebar-tabs { display: flex; flex: 1; gap: 2px; }
.sidebar-tab { flex: 1; background: none; border: none; color: var(--text-sub); padding: 4px 6px; border-radius: 4px; cursor: pointer; font-size: 12px; font-family: 'SF Mono', monospace; position: relative; white-space: nowrap; }
.sidebar-tab:hover { background: var(--row-hover); }
.sidebar-tab.active { color: var(--primary); background: var(--row-hover); }
.tab-badge { position: absolute; top: 1px; right: 2px; min-width: 14px; height: 14px; border-radius: 7px; font-size: 10px; display: flex; align-items: center; justify-content: center; padding: 0 3px; }
.tab-badge.failed { background: var(--red); color: #fff; }
.tab-badge.executing { background: var(--yellow); width: 7px; height: 7px; min-width: 0; border-radius: 50%; top: 3px; right: 3px; padding: 0; }
.sidebar-new { background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 13px; font-family: 'SF Mono', monospace; flex-shrink: 0; }
.sidebar-new:hover { background: var(--row-hover); }
.sidebar-body { flex: 1; overflow-y: auto; padding: 8px; }

/* Chat main */
.chat-main { flex: 1; display: flex; flex-direction: column; min-width: 300px; position: relative; }

.chat-header { display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-bottom: 1px solid var(--border); background: var(--panel); }
.header-new-btn { background: none; border: 1px solid var(--border); color: var(--text); width: 28px; height: 28px; border-radius: 4px; cursor: pointer; font-size: 16px; flex-shrink: 0; }
.header-new-btn:hover { background: var(--row-hover); }
.conv-title { flex: 1; color: var(--text); font-family: 'SF Mono', monospace; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; }
.conv-title:hover { color: var(--primary); }
.conv-title-input { flex: 1; background: var(--input-bg); border: 1px solid var(--primary); color: var(--text); font-family: 'SF Mono', monospace; font-size: 13px; padding: 2px 6px; border-radius: 4px; outline: none; }
.current-model { color: var(--muted); font-size: 11px; font-family: 'SF Mono', monospace; }

.mode-badge-wrapper { position: relative; flex-shrink: 0; }
.mode-badge {
  padding: 2px 8px; border-radius: 4px; font-size: 12px;
  cursor: pointer; font-weight: 500; text-transform: uppercase;
  letter-spacing: 0.5px; user-select: none;
  font-family: 'SF Mono', monospace;
}
.mode-badge.ask { background: color-mix(in srgb, var(--primary) 15%, transparent); color: var(--primary); }
.mode-badge.auto { background: color-mix(in srgb, var(--green, #4caf50) 15%, transparent); color: var(--green, #4caf50); }
.mode-badge.plan { background: color-mix(in srgb, var(--yellow, #f59e0b) 15%, transparent); color: var(--yellow, #f59e0b); }
.mode-badge.readonly { background: color-mix(in srgb, var(--text-sub) 15%, transparent); color: var(--text-sub); }
.mode-dropdown {
  position: absolute; top: 100%; right: 0; margin-top: 4px;
  background: var(--bg-card, #1e1e1e); border: 1px solid var(--border, #333);
  border-radius: 6px; padding: 4px; z-index: 100; min-width: 140px;
}
.mode-option {
  padding: 6px 12px; cursor: pointer; border-radius: 4px;
  font-size: 13px; font-family: 'SF Mono', monospace;
  color: var(--text); text-transform: uppercase;
}
.mode-option:hover { background: var(--row-hover, #2a2a2a); }
.mode-option.active { font-weight: 600; color: var(--primary); }
.mode-option.reset {
  color: var(--muted, #888); font-size: 12px; text-transform: none;
  border-top: 1px solid var(--border, #333);
  margin-top: 4px; padding-top: 8px;
}

.conv-item { padding: 8px 14px; cursor: pointer; color: var(--text-sub); font-size: 13px; font-family: 'SF Mono', monospace; display: flex; align-items: center; border-radius: 6px; }
.conv-item:hover { background: var(--row-hover); }
.conv-item.active { color: var(--primary); background: var(--row-hover); }
.conv-item-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conv-item-input { flex: 1; background: var(--input-bg); border: 1px solid var(--primary); color: var(--text); font-family: 'SF Mono', monospace; font-size: 13px; padding: 2px 6px; border-radius: 4px; outline: none; }
.conv-del { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 16px; padding: 0 4px; flex-shrink: 0; }
.conv-del:hover { color: var(--red); }
.conv-processing-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--primary); flex-shrink: 0; margin-right: 4px; animation: pulse-dot 1.2s ease-in-out infinite; }
@keyframes pulse-dot { 0%, 100% { opacity: 1; transform: scale(1); } 50% { opacity: 0.4; transform: scale(0.7); } }

.chat-messages { flex: 1; overflow-y: auto; padding: 16px; font-family: 'SF Mono', 'Fira Code', monospace; }
.empty-state { color: var(--muted); text-align: center; margin-top: 40%; font-size: 14px; }

.chat-input { display: flex; gap: 8px; padding: 12px 16px; border-top: 1px solid var(--border); background: var(--panel); }
.input-wrapper { flex: 1; position: relative; display: flex; flex-direction: column; }
.input-wrapper textarea { background: var(--input-bg); border: 1px solid var(--border); color: var(--text); padding: 8px 12px; border-radius: 6px; font-family: 'SF Mono', monospace; font-size: 13px; resize: none; outline: none; width: 100%; box-sizing: border-box; min-height: 36px; max-height: 200px; overflow-y: auto; line-height: 1.5; }
.input-wrapper textarea:focus { border-color: var(--primary); }
.slash-hint { position: absolute; top: 0; left: 0; padding: 8px 12px; font-family: 'SF Mono', monospace; font-size: 13px; line-height: 1.5; pointer-events: none; white-space: pre; color: transparent; }
.slash-hint .ghost { color: var(--text-muted, #666); opacity: 0.6; }
.kb-dropdown {
  position: absolute; bottom: 100%; left: 0; right: 0; margin-bottom: 4px;
  background: var(--bg-card, #1e1e1e); border: 1px solid var(--border, #333);
  border-radius: 6px; padding: 4px; z-index: 200; max-height: 200px; overflow-y: auto;
}
.kb-dropdown-item {
  padding: 6px 12px; cursor: pointer; border-radius: 4px;
  font-size: 13px; font-family: 'SF Mono', monospace; color: var(--text);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.kb-dropdown-item:hover { background: var(--row-hover, #2a2a2a); }
.kb-dropdown-item.active { background: var(--row-hover, #2a2a2a); color: var(--primary); }
.send-btn { background: var(--primary); color: #fff; border: none; padding: 8px 20px; border-radius: 6px; cursor: pointer; font-size: 13px; font-family: 'SF Mono', monospace; }
.send-btn:hover:not(:disabled) { background: var(--primary-hover); }
.send-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.queue-btn { background: var(--text-sub); }
.queue-btn:hover:not(:disabled) { background: var(--text); }
.queued-message { padding: 6px 16px; opacity: 0.45; }
.queued-message-text { font-family: 'SF Mono', monospace; font-size: 13px; color: var(--text); white-space: pre-wrap; word-break: break-word; }
.retry-banner { padding: 6px 16px; display: flex; flex-direction: column; gap: 2px; }
.retry-error { font-size: 12px; color: var(--error, #e05252); opacity: 0.8; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 100%; }
.retry-countdown { font-size: 12px; color: var(--text-muted, #888); }
.cancel-btn { background: var(--red, #e05252); }
.cancel-btn:hover { background: #c94444; }
.export-wrapper { position: relative; margin-left: auto; }
.export-btn { background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 10px; border-radius: 4px; cursor: pointer; font-size: 12px; font-family: 'SF Mono', monospace; }
.export-btn:hover { background: var(--row-hover); }
.export-menu { position: absolute; right: 0; top: calc(100% + 4px); background: var(--panel); border: 1px solid var(--border); border-radius: 6px; min-width: 120px; z-index: 100; box-shadow: 0 4px 12px rgba(0,0,0,.15); }
.export-option { padding: 8px 14px; cursor: pointer; font-size: 13px; color: var(--text); font-family: 'SF Mono', monospace; }
.export-option:hover { background: var(--row-hover); }

.model-picker { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; margin: 0 16px 8px; padding: 12px; }
.model-picker-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; color: var(--text); font-size: 13px; font-family: 'SF Mono', monospace; }
.model-picker-list { max-height: 300px; overflow-y: auto; }
.model-picker-item { padding: 8px 12px; cursor: pointer; border-radius: 6px; display: flex; justify-content: space-between; align-items: center; color: var(--text-sub); font-size: 13px; font-family: 'SF Mono', monospace; }
.model-picker-item:hover { background: var(--row-hover); }
.model-picker-item.active { color: var(--primary); font-weight: 500; }
.model-check { color: var(--green); font-size: 12px; }

/* Sidebar resize handle */
.sidebar-resize-handle { width: 5px; cursor: col-resize; background: transparent; display: flex; align-items: center; justify-content: center; flex-shrink: 0; transition: background 0.15s; }
.sidebar-resize-handle:hover, .chat-page.dragging .sidebar-resize-handle { background: rgba(108, 140, 255, 0.3); }
.drag-indicator { width: 2px; height: 32px; border-radius: 1px; background: var(--border); }

/* Todo panel */
.todo-panel { margin: 0 16px 8px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); font-family: 'SF Mono', monospace; font-size: 12px; overflow: hidden; }
.todo-header { padding: 5px 10px; color: var(--text-sub); font-size: 11px; letter-spacing: 0.05em; border-bottom: 1px solid var(--border); }
.todo-row { display: flex; align-items: center; gap: 8px; padding: 4px 10px; color: var(--text-sub); }
.todo-row + .todo-row { border-top: 1px solid var(--border); }
.todo-icon { width: 14px; text-align: center; flex-shrink: 0; }
.todo-subject { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.todo-pending .todo-icon { color: var(--text-sub); }
.todo-in_progress { border-left: 2px solid var(--primary); background: color-mix(in srgb, var(--primary) 6%, transparent); }
.todo-in_progress .todo-icon { color: var(--primary); }
.todo-in_progress .todo-subject { color: var(--text); font-weight: 500; }
.todo-completed .todo-icon { color: var(--green, #4caf50); }
.todo-completed .todo-subject { color: var(--text-sub); text-decoration: line-through; }
.todo-completed-indent { padding-left: 20px; }
.todo-fold { cursor: pointer; }
.todo-fold:hover { background: var(--surface-2, rgba(255,255,255,0.04)); }
.todo-fold .todo-icon { color: var(--text-sub); }

/* Right target panel */
.target-resize-handle { width: 5px; cursor: col-resize; background: transparent; display: flex; align-items: center; justify-content: center; flex-shrink: 0; transition: background 0.15s; }
.target-resize-handle:hover, .chat-page.dragging .target-resize-handle { background: rgba(108, 140, 255, 0.3); }
.target-side { display: flex; flex-direction: column; flex-shrink: 0; overflow: hidden; transition: width 0.2s; border-left: 1px solid var(--border); background: var(--surface); }
.target-side.collapsed { width: 0 !important; border-left: none; }
.target-side-header { display: flex; align-items: center; justify-content: space-between; padding: 8px 10px; border-bottom: 1px solid var(--border); flex-shrink: 0; min-height: 43px; box-sizing: border-box; }
.target-side-title { font-size: 12px; font-weight: 500; color: var(--text-sub); letter-spacing: 0.05em; }
.target-toggle { background: none; border: none; cursor: pointer; color: var(--text-sub); font-size: 18px; width: 24px; height: 24px; border-radius: 4px; display: flex; align-items: center; justify-content: center; line-height: 1; }
.target-toggle:hover { background: rgba(255,255,255,0.06); color: var(--text); }
.target-side-body { flex: 1; overflow-y: auto; }
</style>
