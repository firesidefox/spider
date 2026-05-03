<script setup lang="ts">
import { ref, onMounted, nextTick, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ChatMessage from '../components/ChatMessage.vue'
import type { MessageBlock, ToolCallBlock } from '../components/ChatMessage.vue'
import TargetPanel from '../components/TargetPanel.vue'
import type { DeviceStatus } from '../components/TargetPanel.vue'
import {
  sendMessage, createConversation, listConversations,
  getConversation, deleteConversation, confirmAction,
  getActiveModel, setActiveModel, updateTitle,
  type Conversation, type ChatMessage as ChatMsg, type ChatEvent,
} from '../api/chat'
import { listHosts, type SafeHost } from '../api/hosts'

const route = useRoute()
const router = useRouter()

interface DisplayMessage {
  id: string
  role: string
  blocks: MessageBlock[]
  confirm?: { requestId: string; tool: string; input: any; riskLevel: string } | null
  isStreaming?: boolean
}

const conversations = ref<Conversation[]>([])
const activeConvId = ref<string | null>(null)
const messages = ref<DisplayMessage[]>([])
const inputText = ref('')
const isStreaming = ref(false)
const messagesRef = ref<HTMLElement | null>(null)
const devices = ref<DeviceStatus[]>([])
let abortCtrl: AbortController | null = null

const sidebarOpen = ref(localStorage.getItem('spider-sidebar') !== 'closed')
const targetWidth = ref(parseInt(localStorage.getItem('spider-target-width') || '280'))
const isDragging = ref(false)
const chatPageRef = ref<HTMLElement | null>(null)

const showModelPicker = ref(false)
const availableModels = ref<{id: string, display_name: string}[]>([])
const currentModel = ref('')
const currentProvider = ref('')
const currentModelName = ref('')

const activeConv = computed(() =>
  conversations.value.find(c => c.id === activeConvId.value) || null
)

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
  const startWidth = targetWidth.value

  function onMove(ev: MouseEvent) {
    const delta = startX - ev.clientX
    const newWidth = Math.min(
      window.innerWidth * 0.5,
      Math.max(200, startWidth + delta)
    )
    targetWidth.value = newWidth
  }

  function onUp() {
    isDragging.value = false
    localStorage.setItem('spider-target-width', String(targetWidth.value))
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
  const data = await getConversation(id)
  activeConvId.value = id
  messages.value = data.messages.map(m => {
    const blocks: MessageBlock[] = []
    if (m.content) blocks.push({ type: 'text', content: m.content })
    if (m.tool_calls) {
      try {
        for (const tc of JSON.parse(m.tool_calls)) {
          blocks.push({ type: 'tool', call: {
            id: tc.id, name: tc.name, input: tc.input,
            result: tc.result, isError: tc.is_error, durationMs: tc.duration_ms,
          }})
        }
      } catch { /* ignore malformed */ }
    }
    return { id: m.id, role: m.role, blocks } as DisplayMessage
  })
  router.replace(`/chat/${id}`)
  await nextTick()
  scrollToBottom()
}

async function createNewConversation(title = '新对话') {
  const conv = await createConversation(title)
  conversations.value.unshift(conv)
  activeConvId.value = conv.id
  messages.value = []
  router.replace(`/chat/${conv.id}`)
}

function generateTitle(text: string): string {
  const now = new Date()
  const mm = String(now.getMonth() + 1).padStart(2, '0')
  const dd = String(now.getDate()).padStart(2, '0')
  const hh = String(now.getHours()).padStart(2, '0')
  const mi = String(now.getMinutes()).padStart(2, '0')
  const summary = text.length > 20 ? text.slice(0, 20) + '...' : text
  return `${mm}-${dd} ${hh}:${mi} ${summary}`
}

function scrollToBottom() {
  if (messagesRef.value) {
    messagesRef.value.scrollTop = messagesRef.value.scrollHeight
  }
}

async function send() {
  const text = inputText.value.trim()
  if (!text || isStreaming.value) return

  // Handle /model command
  if (text === '/model') {
    inputText.value = ''
    await handleModelCommand()
    return
  }

  inputText.value = ''

  if (!activeConvId.value) {
    await createNewConversation(generateTitle(text))
  }

  const userMsg: DisplayMessage = {
    id: `u-${Date.now()}`, role: 'user', blocks: [{ type: 'text', content: text }],
  }
  messages.value.push(userMsg)

  const assistantMsg: DisplayMessage = {
    id: `a-${Date.now()}`, role: 'assistant',
    blocks: [], isStreaming: true,
  }
  messages.value.push(assistantMsg)
  isStreaming.value = true
  await nextTick()
  scrollToBottom()

  abortCtrl = sendMessage(activeConvId.value!, text, (event: ChatEvent) => {
    const last = messages.value[messages.value.length - 1]
    const blocks = last.blocks
    switch (event.type) {
      case 'text_delta': {
        const lastBlock = blocks[blocks.length - 1]
        if (lastBlock?.type === 'text') {
          lastBlock.content += event.content?.text || ''
        } else {
          blocks.push({ type: 'text', content: event.content?.text || '' })
        }
        break
      }
      case 'tool_start':
        blocks.push({ type: 'tool', call: {
          id: event.content?.id || `t-${Date.now()}`,
          name: event.content?.name || 'unknown',
          input: event.content?.input,
        }})
        break
      case 'tool_result': {
        const tb = blocks.find(b => b.type === 'tool' && b.call.id === event.content?.id) as { type: 'tool'; call: ToolCallBlock } | undefined
        if (tb) {
          tb.call.result = event.content?.result
          tb.call.isError = event.content?.is_error
          tb.call.durationMs = event.content?.duration_ms
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
        break
      case 'error': {
        const errText = `\n\n**Error:** ${event.content?.error || 'unknown error'}`
        const lastBlk = last.blocks[last.blocks.length - 1]
        if (lastBlk?.type === 'text') {
          lastBlk.content += errText
        } else {
          last.blocks.push({ type: 'text', content: errText })
        }
        last.isStreaming = false
        isStreaming.value = false
        break
      }
      case 'done':
        last.isStreaming = false
        isStreaming.value = false
        loadConversations()
        break
    }
    nextTick(() => scrollToBottom())
  })
}

async function handleModelCommand() {
  try {
    const { provider_id, model, provider_name } = await getActiveModel()
    currentProvider.value = provider_id
    currentModel.value = model
    currentModelName.value = model

    if (!currentProvider.value) {
      messages.value.push({
        id: Date.now().toString(),
        role: 'assistant',
        blocks: [{ type: 'text', content: '未配置模型供应商。请在 个人设置 → 模型供应商 中配置。' }],
      })
      return
    }

    const res = await fetch(`/api/v1/providers/${provider_id}/models`, { headers: (await import('../api/auth')).authHeaders() })
    if (!res.ok) throw new Error('获取模型列表失败')
    const models = await res.json()
    availableModels.value = models.map((m: any) => ({ id: m.model_id, display_name: m.display_name }))
    showModelPicker.value = true
  } catch (e: any) {
    messages.value.push({
      id: Date.now().toString(),
      role: 'assistant',
      blocks: [{ type: 'text', content: `获取模型列表失败: ${e.message}` }],
    })
  }
}

async function selectModel(modelId: string) {
  try {
    await setActiveModel(currentProvider.value, modelId)
    currentModel.value = modelId
    currentModelName.value = modelId
    showModelPicker.value = false
    messages.value.push({
      id: Date.now().toString(),
      role: 'assistant',
      blocks: [{ type: 'text', content: `模型已切换为 **${modelId}**` }],
    })
  } catch (e: any) {
    messages.value.push({
      id: Date.now().toString(),
      role: 'assistant',
      blocks: [{ type: 'text', content: `切换模型失败: ${e.message}` }],
    })
  }
}

async function handleConfirm(requestId: string, approved: boolean) {
  if (!activeConvId.value) return
  await confirmAction(activeConvId.value, requestId, approved)
  const msg = messages.value.find(m => m.confirm?.requestId === requestId)
  if (msg) msg.confirm = null
}

async function handleDeleteConversation(id: string) {
  await deleteConversation(id)
  conversations.value = conversations.value.filter(c => c.id !== id)
  if (activeConvId.value === id) {
    activeConvId.value = null
    messages.value = []
    router.replace('/chat')
  }
}

async function loadDevices() {
  const hosts = await listHosts()
  devices.value = hosts.map(h => ({
    id: h.id, name: h.name, ip: h.ip,
    vendor: '', status: 'online' as const,
  }))
}

watch(() => messages.value.length, () => {
  nextTick(() => scrollToBottom())
})

onMounted(async () => {
  await Promise.all([loadConversations(), loadDevices()])
  getActiveModel().then(m => { currentModelName.value = m.model })
  const paramId = route.params.id as string | undefined
  if (paramId) {
    await selectConversation(paramId)
  }
})
</script>

<template>
  <div class="chat-page" ref="chatPageRef" :class="{ dragging: isDragging }">
    <!-- Sidebar -->
    <div class="sidebar" :class="{ collapsed: !sidebarOpen }">
      <div class="sidebar-header">
        <button class="sidebar-toggle" @click="toggleSidebar">≡</button>
        <button class="sidebar-new" @click="createNewConversation()">+ New</button>
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
          <button class="conv-del" @click.stop="handleDeleteConversation(c.id)">×</button>
        </div>
      </div>
    </div>

    <!-- Chat main -->
    <div class="chat-main">
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
        <span class="current-model" v-if="currentModelName">{{ currentModelName }}</span>
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

      <div class="chat-input">
        <textarea
          v-model="inputText"
          @keydown.enter.exact.prevent="send"
          placeholder="输入运维指令..."
          :disabled="isStreaming"
          rows="1"
        ></textarea>
        <button @click="send" :disabled="isStreaming || !inputText.trim()" class="send-btn">
          {{ isStreaming ? '...' : '发送' }}
        </button>
      </div>
    </div>

    <!-- Drag handle -->
    <div class="drag-handle" @mousedown="startDrag">
      <div class="drag-indicator"></div>
    </div>

    <!-- Target panel -->
    <TargetPanel :devices="devices" class="target-side" :style="{ flexBasis: targetWidth + 'px' }" />
  </div>
</template>

<style scoped>
.chat-page { display: flex; height: 100%; gap: 0; }
.chat-page.dragging { user-select: none; cursor: col-resize; }

/* Sidebar */
.sidebar { width: 240px; border-right: 1px solid var(--border); display: flex; flex-direction: column; background: var(--panel); transition: width 0.2s ease, opacity 0.2s ease; overflow: hidden; flex-shrink: 0; }
.sidebar.collapsed { width: 0; border-right: none; opacity: 0; }
.sidebar-header { display: flex; align-items: center; gap: 8px; padding: 10px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.sidebar-toggle { background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 14px; flex-shrink: 0; }
.sidebar-toggle:hover { background: var(--row-hover); }
.sidebar-new { flex: 1; background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 13px; font-family: 'SF Mono', monospace; }
.sidebar-new:hover { background: var(--row-hover); }
.sidebar-body { flex: 1; overflow-y: auto; padding: 8px; }

/* Chat main */
.chat-main { flex: 1; display: flex; flex-direction: column; min-width: 300px; position: relative; }

/* Target side */
.target-side { min-width: 200px; max-width: 50vw; flex-shrink: 0; }

.chat-header { display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-bottom: 1px solid var(--border); background: var(--panel); }
.header-new-btn { background: none; border: 1px solid var(--border); color: var(--text); width: 28px; height: 28px; border-radius: 4px; cursor: pointer; font-size: 16px; flex-shrink: 0; }
.header-new-btn:hover { background: var(--row-hover); }
.conv-title { flex: 1; color: var(--text); font-family: 'SF Mono', monospace; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; }
.conv-title:hover { color: var(--primary); }
.conv-title-input { flex: 1; background: var(--input-bg); border: 1px solid var(--primary); color: var(--text); font-family: 'SF Mono', monospace; font-size: 13px; padding: 2px 6px; border-radius: 4px; outline: none; }
.current-model { color: var(--muted); font-size: 11px; font-family: 'SF Mono', monospace; }

.conv-item { padding: 8px 14px; cursor: pointer; color: var(--text-sub); font-size: 13px; font-family: 'SF Mono', monospace; display: flex; align-items: center; border-radius: 6px; }
.conv-item:hover { background: var(--row-hover); }
.conv-item.active { color: var(--primary); background: var(--row-hover); }
.conv-item-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conv-item-input { flex: 1; background: var(--input-bg); border: 1px solid var(--primary); color: var(--text); font-family: 'SF Mono', monospace; font-size: 13px; padding: 2px 6px; border-radius: 4px; outline: none; }
.conv-del { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 16px; padding: 0 4px; flex-shrink: 0; }
.conv-del:hover { color: var(--red); }

.chat-messages { flex: 1; overflow-y: auto; padding: 16px; font-family: 'SF Mono', 'Fira Code', monospace; }
.empty-state { color: var(--muted); text-align: center; margin-top: 40%; font-size: 14px; }

.chat-input { display: flex; gap: 8px; padding: 12px 16px; border-top: 1px solid var(--border); background: var(--panel); }
.chat-input textarea { flex: 1; background: var(--input-bg); border: 1px solid var(--border); color: var(--text); padding: 8px 12px; border-radius: 6px; font-family: 'SF Mono', monospace; font-size: 13px; resize: none; outline: none; }
.chat-input textarea:focus { border-color: var(--primary); }
.send-btn { background: var(--primary); color: #fff; border: none; padding: 8px 20px; border-radius: 6px; cursor: pointer; font-size: 13px; font-family: 'SF Mono', monospace; }
.send-btn:hover:not(:disabled) { background: var(--primary-hover); }
.send-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.model-picker { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; margin: 0 16px 8px; padding: 12px; }
.model-picker-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; color: var(--text); font-size: 13px; font-family: 'SF Mono', monospace; }
.model-picker-list { max-height: 300px; overflow-y: auto; }
.model-picker-item { padding: 8px 12px; cursor: pointer; border-radius: 6px; display: flex; justify-content: space-between; align-items: center; color: var(--text-sub); font-size: 13px; font-family: 'SF Mono', monospace; }
.model-picker-item:hover { background: var(--row-hover); }
.model-picker-item.active { color: var(--primary); font-weight: 500; }
.model-check { color: var(--green); font-size: 12px; }

/* Drag handle */
.drag-handle { width: 5px; cursor: col-resize; background: transparent; display: flex; align-items: center; justify-content: center; flex-shrink: 0; transition: background 0.15s; }
.drag-handle:hover, .chat-page.dragging .drag-handle { background: rgba(108, 140, 255, 0.3); }
.drag-indicator { width: 2px; height: 32px; border-radius: 1px; background: var(--border); }
</style>
