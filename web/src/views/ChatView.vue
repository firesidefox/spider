<script setup lang="ts">
defineOptions({ name: 'ChatView' })
import { ref, onMounted, onActivated, onDeactivated, onUnmounted, nextTick, computed, provide } from 'vue'
import { useRoute, useRouter, onBeforeRouteUpdate } from 'vue-router'
import ChatMessage from '../components/ChatMessage.vue'
import RuntimeStatusBar from '../components/RuntimeStatusBar.vue'
import TargetPanel from '../components/TargetPanel.vue'
import type { DeviceStatus } from '../components/TargetPanel.vue'
import { useTargetHosts } from '../composables/useTargetHosts'
import { useConversationList } from '../composables/chat/useConversationList'
import { useTodoPanel } from '../composables/chat/useTodoPanel'
import { useChatStream } from '../composables/chat/useChatStream'
import {
  getConversation,
  getActiveModel, setActiveModel, exportConversation,
  suggestTitle,
} from '../api/chat'
import { listHosts, type Host } from '../api/hosts'
import { authHeaders, getUIPrefs, setUIPrefs } from '../api/auth'
import { useAuth } from '../composables/useAuth'
import { listGroups, listDocumentsByGroup, type DocumentGroup, type Document as KbDocument } from '../api/documents'
import { useAgentStatus } from '../composables/useAgentStatus'
import {
  chatThemes, densityPresets,
  getSavedChatTheme, saveChatTheme,
  getSavedChatDensity, saveChatDensity,
  type ChatThemeName, type ChatDensityName,
} from '../chatTheme'

const chatThemeName = ref<ChatThemeName>(getSavedChatTheme())
const chatDensityName = ref<ChatDensityName>(getSavedChatDensity())

const { currentUser } = useAuth()

provide('chatTheme', () => chatThemes[chatThemeName.value])
provide('chatDensity', () => densityPresets[chatDensityName.value])
provide('setChatTheme', (name: ChatThemeName) => {
  chatThemeName.value = name
  saveChatTheme(name)
})
provide('setChatDensity', (name: ChatDensityName) => {
  chatDensityName.value = name
  saveChatDensity(name)
})

function syncChatThemeFromStorage() {
  chatThemeName.value = getSavedChatTheme()
  chatDensityName.value = getSavedChatDensity()
}
onMounted(() => window.addEventListener('storage', syncChatThemeFromStorage))
onUnmounted(() => window.removeEventListener('storage', syncChatThemeFromStorage))

const route = useRoute()
const router = useRouter()

// Initialize conversation list composable
const conversationList = useConversationList({
  onConversationSelected: async (convId) => {
    await chatStream.loadConversationMessages(convId)
    const data = await getConversation(convId)
    todoPanel.loadTodoTasks(convId, data.todo_tasks ?? [])
    if (transitionState.value !== 'chat') transitionState.value = 'chat'
    router.replace(`/chat/${convId}`)
    await nextTick()
    scrollToBottom()
  }
})

// Initialize todo panel composable
const todoPanel = useTodoPanel({
  activeConvId: conversationList.activeConvId
})

// Initialize chat stream composable
const chatStream = useChatStream({
  activeConvId: conversationList.activeConvId,
  getConvTitle: conversationList.getConvTitle,
  onScrollToBottom: scrollToBottom,
  onDeviceStatusUpdate: (hostName, status) => {
    setDeviceStatus(hostName, status as DeviceStatus['status'])
  },
  onLoadConversations: async () => {
    await conversationList.loadConversations()
  },
  todoTasksMap: todoPanel.todoTasksMap,
  startTimer: todoPanel.startTimer,
  stopTimer: todoPanel.stopTimer,
  clearAllTimers: todoPanel.clearAllTimers,
  turnUsage: todoPanel.turnUsage,
})

const showExportMenu = ref(false)

async function doExport(format: 'md' | 'json') {
  showExportMenu.value = false
  if (!conversationList.activeConvId.value) return
  await exportConversation(conversationList.activeConvId.value, format)
}

const transitionState = ref<'welcome' | 'transitioning' | 'chat'>('welcome')
const messages = computed(() => chatStream.messages.value)
const isStreaming = computed(() => chatStream.isStreaming.value)
const queuedMessages = computed(() => chatStream.queuedMessages.value)

const { statuses: agentStatuses } = useAgentStatus()
const currentAgentStatus = computed(() => {
  const id = conversationList.activeConvId.value
  const status = id ? agentStatuses.value.get(id) ?? null : null
  return status as any
})

const inputText = ref('')
const retryState = computed(() => chatStream.retryState.value)

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

const messagesRef = ref<HTMLElement | null>(null)
const devices = ref<DeviceStatus[]>([])
const allHosts = ref<Host[]>([])
const { selectedHostIds } = useTargetHosts()

function setDeviceStatus(hostName: string, status: DeviceStatus['status']) {
  const idx = devices.value.findIndex(d => d.name === hostName)
  if (idx === -1) return
  devices.value = devices.value.map((d, i) => i === idx ? { ...d, status } : d)
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
  conversationList.conversations.value.find(c => c.id === conversationList.activeConvId.value) || null
)

const showModeDropdown = ref(false)
const globalMode = ref('ask')

const effectiveMode = computed(() => {
  const convMode = activeConv.value?.permission_mode
  return convMode || globalMode.value
})

const editingHeaderTitle = ref(false)

function startEditHeaderTitle() {
  if (!activeConv.value) return
  editingHeaderTitle.value = true
  conversationList.editTitleText.value = activeConv.value.title
}

async function saveHeaderTitle() {
  editingHeaderTitle.value = false
  const text = conversationList.editTitleText.value.trim()
  if (!activeConv.value || !text || text === activeConv.value.title) return
  await conversationList.updateTitle(activeConv.value.id, text)
}

function cancelEditHeader() {
  editingHeaderTitle.value = false
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


async function createNewConversation() {
  const convId = await conversationList.createConversation()
  chatStream.getOrInitMessages(convId)
  chatStream.setConversationStreaming(convId, false)
  await conversationList.selectConversation(convId)
  await nextTick()
  textareaRef.value?.focus()
}

async function goNewPage() {
  transitionState.value = 'welcome'
  router.replace('/chat')
}

function scrollToBottom() {
  if (messagesRef.value) {
    messagesRef.value.scrollTop = messagesRef.value.scrollHeight
  }
}

function handleEscCancel(e: KeyboardEvent) {
  if (e.key === 'Escape' && isStreaming.value && !kbDropdownMode.value) {
    handleCancelSend()
  }
}

async function handleCancelSend() {
  await chatStream.cancelSend()
}

function triggerLayoutTransition() {
  if (transitionState.value !== 'welcome') return
  transitionState.value = 'transitioning'
  setTimeout(() => {
    if (transitionState.value === 'transitioning') transitionState.value = 'chat'
  }, 420)
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
        chatStream.addSystemMessage('用法：/export [md|json] 或 /export --format [md|json]')
        return
      }
      inputText.value = ''
      if (!conversationList.activeConvId.value) {
        chatStream.addSystemMessage('没有活跃的会话')
        return
      }
      await exportConversation(conversationList.activeConvId.value, fmt === 'default' ? 'md' : fmt)
      return
    }
    if (text === '/rename' || text.startsWith('/rename ')) {
      inputText.value = ''
      if (!conversationList.activeConvId.value) {
        chatStream.addSystemMessage('没有活跃的会话')
        return
      }
      await handleRenameCommand(text)
      return
    }
  }

  if (!overrideText) {
    inputText.value = ''
    nextTick(() => {
      if (textareaRef.value) textareaRef.value.style.height = 'auto'
    })
  }

  if (!conversationList.activeConvId.value) {
    if (!overrideText) setTimeout(triggerLayoutTransition, 0)
    await createNewConversation()
  }

  await chatStream.sendMessage(text, selectedHostIds.value)
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
    const { provider_id, model } = await getActiveModel()
    currentProvider.value = provider_id
    currentModel.value = model

    if (!currentProvider.value) {
      chatStream.addSystemMessage('未配置模型供应商。请在 个人设置 → 模型供应商 中配置。')
      return
    }

    const res = await fetch(`/api/v1/providers/${provider_id}/models`, { headers: (await import('../api/auth')).authHeaders() })
    if (!res.ok) throw new Error('获取模型列表失败')
    const models = await res.json()
    availableModels.value = models.map((m: any) => ({ id: m.model_id, display_name: m.display_name }))
    showModelPicker.value = true
  } catch (e: any) {
    chatStream.addSystemMessage(`获取模型列表失败: ${e.message}`)
  }
}

async function selectModel(modelId: string) {
  try {
    await setActiveModel(currentProvider.value, modelId)
    currentModel.value = modelId
    showModelPicker.value = false
    chatStream.addSystemMessage(`模型已切换为 **${modelId}**`)
  } catch (e: any) {
    chatStream.addSystemMessage(`切换模型失败: ${e.message}`)
  }
}

async function handleConfirm(requestId: string, approved: boolean) {
  await chatStream.handleConfirm(requestId, approved)
}

async function applyConvTitle(id: string, title: string) {
  await conversationList.updateTitle(id, title)
}

async function handleRenameCommand(text: string) {
  const id = conversationList.activeConvId.value!
  const arg = text.slice('/rename'.length).trim()
  if (arg) {
    try {
      await applyConvTitle(id, arg)
      chatStream.addSystemMessage(`已重命名为 **${arg}**`)
    } catch (e: any) {
      chatStream.addSystemMessage(`重命名失败: ${e.message}`)
    }
    return
  }
  try {
    chatStream.addSystemMessage('正在生成命名建议…')
    const title = await suggestTitle(id)
    await applyConvTitle(id, title)
    chatStream.addSystemMessage(`已重命名为 **${title}**`)
  } catch (e: any) {
    chatStream.addSystemMessage(`生成命名失败: ${e.message}`)
  }
}

async function handleDeleteConversation(id: string) {
  await conversationList.deleteConversation(id)
  if (conversationList.activeConvId.value === id) {
    transitionState.value = 'welcome'
    await goNewPage()
  }
}

async function loadDevices() {
  const hosts = await listHosts()
  allHosts.value = hosts
  devices.value = hosts.map(h => ({
    id: h.id, name: h.name, ip: h.ip,
    vendor: '', status: 'online' as DeviceStatus['status'],
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

// @kb: two-step dropdown
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

  // Match @kb: → show groups
  const groupMatch = before.match(/@kb:$/)
  if (groupMatch) {
    await loadGroupsIfNeeded()
    if (kbGroups.value.length > 0) {
      kbDropdownMode.value = 'groups'
      kbActiveIndex.value = 0
      kbTriggerStart.value = before.lastIndexOf('@kb:')
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
  await Promise.all([conversationList.loadConversations(), loadDevices()])
  getActiveModel().then(m => { currentModel.value = m.model })
  const paramId = route.params.id as string | undefined
  if (paramId) {
    if (paramId !== conversationList.activeConvId.value) {
      await conversationList.selectConversation(paramId)
    }
  } else {
    const lastConvId = localStorage.getItem('spider-last-conv')
    if (lastConvId) {
      router.replace(`/chat/${lastConvId}`)
      await conversationList.selectConversation(lastConvId)
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
  globalEs.onmessage = (_e) => {}
  globalEs.onerror = () => {}
}

onMounted(() => {
  startGlobalSSE()
  document.addEventListener('click', closeModeDropdown)
  window.addEventListener('keydown', handleEscCancel)
  if (!initialized) { initialized = true; initView() }
  else loadPrefs()
})
onActivated(() => {
  document.addEventListener('click', closeModeDropdown)
  window.addEventListener('keydown', handleEscCancel)
  chatThemeName.value = getSavedChatTheme()
  chatDensityName.value = getSavedChatDensity()
  if (!initialized) { initialized = true; initView() }
  else loadPrefs()
})

onDeactivated(() => {
  todoPanel.clearAllTimers()
  document.removeEventListener('click', closeModeDropdown)
  window.removeEventListener('keydown', handleEscCancel)
})

onBeforeRouteUpdate(async (to) => {
  const newId = to.params.id as string | undefined
  if (newId && newId !== conversationList.activeConvId.value) {
    await conversationList.selectConversation(newId)
  }
})

onUnmounted(() => {
  globalEs?.close()
  chatStream.cleanup()
  todoPanel.clearAllTimers()
  window.removeEventListener('keydown', handleEscCancel)
})
</script>

<template>
  <div class="chat-page" ref="chatPageRef" :class="{ dragging: isDragging || isTargetDragging }">
    <!-- Sidebar -->
    <div class="sidebar" :class="{ collapsed: !sidebarOpen }" :style="{ width: sidebarOpen ? sidebarWidth + 'px' : '0' }">
      <div class="sidebar-header">
        <template v-if="!conversationList.batchMode.value">
          <button class="sidebar-toggle" @click="toggleSidebar">≡</button>
          <div class="sidebar-tabs">
            <button class="sidebar-tab active">对话</button>
          </div>
          <button class="sidebar-new" @click="goNewPage()">+</button>
        </template>
        <template v-else>
          <span class="batch-mode-label">批量管理</span>
          <span style="flex:1"></span>
          <button class="batch-select-all" @click="conversationList.toggleSelectAll">{{ conversationList.selectedConvIds.value.size === conversationList.conversations.value.length ? '取消全选' : '全选' }}</button>
          <button class="batch-cancel" @click="conversationList.exitBatchMode">取消</button>
        </template>
      </div>
      <div class="sidebar-body" @click="conversationList.closeConvMenu()">
        <div v-for="c in conversationList.conversations.value" :key="c.id" class="conv-item"
             :class="{ active: c.id === conversationList.activeConvId.value, 'batch-selected': conversationList.batchMode.value && conversationList.selectedConvIds.value.has(c.id) }"
             @click="conversationList.batchMode.value ? conversationList.toggleSelectConv(c.id) : conversationList.selectConversation(c.id)">
          <input type="checkbox" v-if="conversationList.batchMode.value" class="conv-checkbox"
                 :checked="conversationList.selectedConvIds.value.has(c.id)"
                 @click.stop="conversationList.toggleSelectConv(c.id)" />
          <span v-if="conversationList.batchMode.value" class="conv-item-title">{{ c.title || '未命名对话' }}</span>
          <input v-else-if="conversationList.editingConvId.value === c.id" class="conv-item-input"
                 v-model="conversationList.editTitleText.value"
                 @keydown.enter="conversationList.saveConvTitle(c.id)"
                 @keydown.escape="conversationList.cancelEdit"
                 @blur="conversationList.saveConvTitle(c.id)"
                 @click.stop
                 @vue:mounted="($event: any) => $event.el.focus()" />
          <span v-else class="conv-item-title" @dblclick.stop="conversationList.startEditConvTitle(c.id, c.title)">{{ c.title || '未命名对话' }}</span>
          <span v-if="c.status === 'processing'" class="conv-processing-dot" title="处理中"></span>
          <div v-if="!conversationList.batchMode.value" class="conv-menu-wrap">
            <button class="conv-more" @click.stop="conversationList.openConvMenu(c.id)" title="更多">⋯</button>
            <div v-if="conversationList.menuOpenConvId.value === c.id" class="conv-menu" @click.stop>
              <button class="conv-menu-item" @click="conversationList.startEditConvTitle(c.id, c.title); conversationList.closeConvMenu()">✏ 重命名</button>
              <button class="conv-menu-item" @click="conversationList.enterBatchMode()">☑ 批量管理</button>
              <div class="conv-menu-divider"></div>
              <button class="conv-menu-item conv-menu-item--danger" @click="handleDeleteConversation(c.id); conversationList.closeConvMenu()">✕ 删除</button>
            </div>
          </div>
        </div>
      </div>
      <div v-if="conversationList.batchMode.value" class="batch-action-bar">
        <span class="batch-count">已选 {{ conversationList.selectedConvIds.value.size }}</span>
        <button class="batch-delete-btn" :disabled="conversationList.selectedConvIds.value.size === 0" @click="conversationList.batchDelete">删除选中</button>
      </div>
    </div>
    <div class="sidebar-resize-handle" @mousedown="startDrag">
      <div class="drag-indicator"></div>
    </div>

    <!-- Chat main -->
    <div class="chat-main" :class="{
      'welcome-mode': transitionState === 'welcome',
      'welcome-transitioning': transitionState === 'transitioning',
      'welcome-chat': transitionState === 'chat',
    }" @click="showExportMenu = false; showModeDropdown = false; conversationList.closeConvMenu()">
      <div v-if="conversationList.activeConvId.value && transitionState === 'chat'" class="chat-header">
        <button v-if="!sidebarOpen" class="sidebar-toggle" @click="toggleSidebar">≡</button>
        <button v-if="!sidebarOpen" class="header-new-btn" @click="goNewPage()">+</button>
        <input v-if="editingHeaderTitle" class="conv-title-input"
               v-model="conversationList.editTitleText.value"
               @keydown.enter="saveHeaderTitle"
               @keydown.escape="cancelEditHeader"
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

      <div class="welcome-greeting">
        <span class="welcome-logo">✦</span>
        <span class="welcome-text">你好，{{ currentUser?.username }}</span>
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

      <div v-if="todoPanel.hasTasks.value" class="todo-panel">
        <div class="todo-header">{{ todoPanel.panelHeader.value }}</div>

        <div v-for="task in todoPanel.inProgressTasks.value" :key="task.id" class="todo-row todo-in_progress">
          <span class="todo-icon">●</span>
          <span class="todo-subject">{{ task.seq }}: {{ task.subject }}</span>
        </div>

        <div v-for="task in todoPanel.visiblePending.value" :key="task.id" class="todo-row todo-pending">
          <span class="todo-icon">○</span>
          <span class="todo-subject">{{ task.seq }}: {{ task.subject }}</span>
        </div>
        <div v-if="todoPanel.hiddenPendingCount.value > 0" class="todo-row todo-fold" @click="todoPanel.pendingFolded.value = !todoPanel.pendingFolded.value">
          <span class="todo-icon">○</span>
          <span class="todo-subject">+{{ todoPanel.hiddenPendingCount.value }} more{{ todoPanel.pendingFolded.value ? '' : ' ▲' }}</span>
        </div>

        <div v-if="todoPanel.completedTasks.value.length > 0" class="todo-row todo-fold" @click="todoPanel.completedFolded.value = !todoPanel.completedFolded.value">
          <span class="todo-icon">✓</span>
          <span class="todo-subject">+{{ todoPanel.completedTasks.value.length }} completed{{ todoPanel.completedFolded.value ? '' : ' ▲' }}</span>
        </div>
        <div v-for="task in todoPanel.visibleCompleted.value" :key="task.id" class="todo-row todo-completed todo-completed-indent">
          <span class="todo-icon">✓</span>
          <span class="todo-subject">{{ task.seq }}: {{ task.subject }}</span>
        </div>
      </div>

      <div v-if="retryState" class="retry-banner">
        <span class="retry-error">{{ retryState.error }}</span>
        <span class="retry-countdown">Retrying in {{ Math.max(0, Math.round((retryState.retryInMs - retryState.countdownMs) / 1000)) }}s… (attempt {{ retryState.attempt }}/{{ retryState.maxRetries }})</span>
      </div>

      <div v-for="(qm, i) in (queuedMessages.get(conversationList.activeConvId.value ?? '') ?? [])" :key="`queued-${i}`" class="queued-message">
        <span class="queued-message-text">{{ i + 1 }}: {{ qm }}</span>
      </div>

      <RuntimeStatusBar v-if="isStreaming" :status="currentAgentStatus" />

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
        <button v-if="isStreaming" @click="handleCancelSend" class="send-btn cancel-btn">取消</button>
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
.conv-menu-wrap { position: relative; flex-shrink: 0; }
.conv-more { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 16px; padding: 0 4px; opacity: 0; transition: opacity 0.1s; }
.conv-item:hover .conv-more, .conv-item.active .conv-more { opacity: 1; }
.conv-more:hover { color: var(--text); }
.conv-menu { position: absolute; right: 0; top: 100%; background: var(--panel); border: 1px solid var(--border); border-radius: 6px; min-width: 140px; z-index: 100; padding: 4px 0; box-shadow: 0 4px 12px rgba(0,0,0,0.3); }
.conv-menu-item { display: block; width: 100%; background: none; border: none; color: var(--text); text-align: left; padding: 6px 14px; cursor: pointer; font-size: 12px; font-family: 'SF Mono', monospace; }
.conv-menu-item:hover { background: var(--row-hover); }
.conv-menu-item--danger { color: var(--red); }
.conv-menu-divider { height: 1px; background: var(--border); margin: 2px 0; }
.conv-checkbox { accent-color: var(--primary); width: 13px; height: 13px; flex-shrink: 0; margin-right: 4px; cursor: pointer; }
.conv-item.batch-selected { background: var(--row-hover); }
.batch-mode-label { color: var(--primary); font-size: 12px; font-family: 'SF Mono', monospace; }
.batch-select-all { background: none; border: 1px solid var(--border); color: var(--text); padding: 2px 8px; border-radius: 4px; font-size: 11px; font-family: 'SF Mono', monospace; cursor: pointer; }
.batch-select-all:hover { background: var(--row-hover); }
.batch-cancel { background: none; border: none; color: var(--text-sub); padding: 2px 6px; font-size: 11px; font-family: 'SF Mono', monospace; cursor: pointer; }
.batch-cancel:hover { color: var(--text); }
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

/* Batch action bar */
.batch-action-bar { display: flex; align-items: center; gap: 8px; padding: 8px 10px; border-top: 1px solid var(--border); background: var(--surface); flex-shrink: 0; }
.batch-count { flex: 1; color: var(--text-sub); font-size: 12px; font-family: 'SF Mono', monospace; }
.batch-delete-btn { background: var(--red, #e05252); color: #fff; border: none; padding: 4px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-family: 'SF Mono', monospace; }
.batch-delete-btn:hover:not(:disabled) { background: #c94444; }
.batch-delete-btn:disabled { opacity: 0.5; cursor: not-allowed; }

/* Welcome mode */
.chat-main.welcome-mode { justify-content: center; align-items: center; }
.chat-main.welcome-mode .chat-messages { display: none; }
.chat-main.welcome-mode .todo-panel { display: none; }
.chat-main.welcome-mode .retry-banner { display: none; }
.chat-main.welcome-mode .chat-input {
  max-width: 640px; width: 100%; position: relative;
  transition: max-width 0.35s ease;
  border-top: none; background: transparent; padding: 0;
}
.chat-main.welcome-transitioning .chat-input,
.chat-main.welcome-chat .chat-input { max-width: 100%; }

.welcome-greeting {
  display: none; flex-direction: column; align-items: center; gap: 16px;
  margin-bottom: 32px; position: relative;
  transition: opacity 0.4s ease, transform 0.4s ease, filter 0.4s ease;
}
.welcome-greeting::before {
  content: '';
  position: absolute; top: -40px; left: 50%; transform: translateX(-50%);
  width: 200px; height: 200px;
  background: radial-gradient(circle, rgba(99,102,241,0.18) 0%, transparent 70%);
  pointer-events: none;
}
.chat-main.welcome-mode .welcome-greeting { display: flex; }
.chat-main.welcome-transitioning .welcome-greeting {
  opacity: 0; transform: translateY(-20px); filter: blur(4px); pointer-events: none;
}
.chat-main.welcome-chat .welcome-greeting { display: none; }

.welcome-logo {
  font-size: 32px; color: color-mix(in srgb, var(--primary) 70%, #fff);
  filter: drop-shadow(0 0 14px color-mix(in srgb, var(--primary) 65%, transparent));
  animation: logo-float 3s ease-in-out infinite;
}
@keyframes logo-float {
  0%, 100% { transform: translateY(0); }
  50%       { transform: translateY(-4px); }
}
.welcome-text { font-size: 24px; color: color-mix(in srgb, var(--primary) 40%, var(--text)); }

.chat-main.welcome-mode .input-wrapper {
  background: rgba(99,102,241,0.05);
  border: 1px solid rgba(99,102,241,0.28);
  border-radius: 9px;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.04), 0 2px 12px rgba(0,0,0,0.08);
  transition: border-color 0.2s;
}
.chat-main.welcome-mode .input-wrapper:focus-within {
  border-color: rgba(99,102,241,0.55);
}
.chat-main.welcome-mode .input-wrapper textarea {
  border: none; background: transparent; border-radius: 0;
}
.chat-main.welcome-mode .send-btn:not(.cancel-btn):not(.queue-btn) {
  background: linear-gradient(135deg, #6366f1, #818cf8);
  box-shadow: 0 4px 14px rgba(99,102,241,0.45);
  transition: transform 0.1s, box-shadow 0.2s;
}
.chat-main.welcome-mode .send-btn:not(.cancel-btn):not(.queue-btn):hover {
  box-shadow: 0 6px 20px rgba(99,102,241,0.6);
  transform: translateY(-1px);
}
.chat-main.welcome-mode .send-btn:not(.cancel-btn):not(.queue-btn):active {
  transform: scale(0.95);
}

.chat-main.welcome-chat .chat-messages {
  animation: messages-fadein 0.7s ease 0.5s both;
}
@keyframes messages-fadein {
  from { opacity: 0; transform: translateY(12px); }
  to   { opacity: 1; transform: translateY(0); }
}
</style>
