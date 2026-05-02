<script setup lang="ts">
import { ref, onMounted, nextTick, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ChatMessage from '../components/ChatMessage.vue'
import TargetPanel from '../components/TargetPanel.vue'
import type { DeviceStatus } from '../components/TargetPanel.vue'
import {
  sendMessage, createConversation, listConversations,
  getConversation, deleteConversation, confirmAction,
  getActiveModel, setActiveModel,
  type Conversation, type ChatMessage as ChatMsg, type ChatEvent,
} from '../api/chat'
import { listHosts, type SafeHost } from '../api/hosts'

const route = useRoute()
const router = useRouter()

interface DisplayMessage {
  id: string
  role: string
  content: string
  toolCalls?: { id: string; name: string; input?: any; result?: string; isError?: boolean }[]
  confirm?: { requestId: string; tool: string; input: any; riskLevel: string } | null
  isStreaming?: boolean
}

const conversations = ref<Conversation[]>([])
const activeConvId = ref<string | null>(null)
const messages = ref<DisplayMessage[]>([])
const inputText = ref('')
const isStreaming = ref(false)
const messagesRef = ref<HTMLElement | null>(null)
const showConvList = ref(false)
const devices = ref<DeviceStatus[]>([])
let abortCtrl: AbortController | null = null

const showModelPicker = ref(false)
const availableModels = ref<{id: string, display_name: string}[]>([])
const currentModel = ref('')
const currentProvider = ref('')
const currentModelName = ref('')

const activeConv = computed(() =>
  conversations.value.find(c => c.id === activeConvId.value) || null
)

async function loadConversations() {
  conversations.value = await listConversations()
}

async function selectConversation(id: string) {
  const data = await getConversation(id)
  activeConvId.value = id
  messages.value = data.messages.map(m => ({
    id: m.id, role: m.role, content: m.content,
  }))
  router.replace(`/chat/${id}`)
  await nextTick()
  scrollToBottom()
}

async function createNewConversation() {
  const conv = await createConversation('新对话')
  conversations.value.unshift(conv)
  activeConvId.value = conv.id
  messages.value = []
  router.replace(`/chat/${conv.id}`)
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
    await createNewConversation()
  }

  const userMsg: DisplayMessage = {
    id: `u-${Date.now()}`, role: 'user', content: text,
  }
  messages.value.push(userMsg)

  const assistantMsg: DisplayMessage = {
    id: `a-${Date.now()}`, role: 'assistant',
    content: '', toolCalls: [], isStreaming: true,
  }
  messages.value.push(assistantMsg)
  isStreaming.value = true
  await nextTick()
  scrollToBottom()

  abortCtrl = sendMessage(activeConvId.value!, text, (event: ChatEvent) => {
    const last = messages.value[messages.value.length - 1]
    switch (event.type) {
      case 'text_delta':
        last.content += event.content?.text || ''
        break
      case 'tool_start':
        if (!last.toolCalls) last.toolCalls = []
        last.toolCalls.push({
          id: event.content?.id || `t-${Date.now()}`,
          name: event.content?.name || 'unknown',
          input: event.content?.input,
        })
        break
      case 'tool_result': {
        const tc = last.toolCalls?.find(t => t.id === event.content?.id)
        if (tc) {
          tc.result = event.content?.result
          tc.isError = event.content?.is_error
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
      case 'error':
        last.content += `\n\n**Error:** ${event.content?.error || 'unknown error'}`
        last.isStreaming = false
        isStreaming.value = false
        break
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
        content: '未配置模型供应商。请在 个人设置 → 模型供应商 中配置。',
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
      content: `获取模型列表失败: ${e.message}`,
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
      content: `模型已切换为 **${modelId}**`,
    })
  } catch (e: any) {
    messages.value.push({
      id: Date.now().toString(),
      role: 'assistant',
      content: `切换模型失败: ${e.message}`,
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
  <div class="chat-page">
    <div class="chat-area">
      <div class="chat-header">
        <button class="conv-toggle" @click="showConvList = !showConvList">≡</button>
        <span class="conv-title">{{ activeConv?.title || '新对话' }}</span>
        <span class="current-model" v-if="currentModelName">{{ currentModelName }}</span>
        <button class="new-conv-btn" @click="createNewConversation">+</button>
      </div>

      <div v-if="showConvList" class="conv-dropdown">
        <div v-for="c in conversations" :key="c.id" class="conv-item"
             :class="{ active: c.id === activeConvId }"
             @click="selectConversation(c.id); showConvList = false">
          <span class="conv-item-title">{{ c.title || '未命名对话' }}</span>
          <button class="conv-del" @click.stop="handleDeleteConversation(c.id)">×</button>
        </div>
      </div>

      <div class="chat-messages" ref="messagesRef">
        <ChatMessage
          v-for="msg in messages" :key="msg.id"
          :role="msg.role" :content="msg.content"
          :tool-calls="msg.toolCalls" :confirm="msg.confirm"
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

    <TargetPanel :devices="devices" class="target-side" />
  </div>
</template>

<style scoped>
.chat-page { display: flex; height: 100%; gap: 0; }
.chat-area { flex: 7; display: flex; flex-direction: column; min-width: 0; position: relative; }
.target-side { flex: 3; min-width: 280px; max-width: 400px; }

.chat-header { display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-bottom: 1px solid var(--border); background: var(--panel); }
.conv-toggle { background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 14px; }
.conv-toggle:hover { background: var(--row-hover); }
.conv-title { flex: 1; color: var(--text); font-family: 'SF Mono', monospace; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.new-conv-btn { background: var(--primary); color: #fff; border: none; width: 28px; height: 28px; border-radius: 4px; cursor: pointer; font-size: 16px; }
.new-conv-btn:hover { background: var(--primary-hover); }
.current-model { color: var(--muted); font-size: 11px; font-family: 'SF Mono', monospace; }

.conv-dropdown { position: absolute; top: 48px; left: 16px; background: var(--surface); border: 1px solid var(--border); border-radius: 6px; z-index: 10; max-height: 300px; overflow-y: auto; min-width: 250px; }
.conv-item { padding: 8px 14px; cursor: pointer; color: var(--text-sub); font-size: 13px; font-family: 'SF Mono', monospace; display: flex; align-items: center; }
.conv-item:hover { background: var(--row-hover); }
.conv-item.active { color: var(--primary); background: var(--row-hover); }
.conv-item-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
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
</style>
