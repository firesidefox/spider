<script setup lang="ts">
import { ref, computed } from 'vue'
import { marked } from 'marked'

export interface ToolCallBlock {
  id: string
  name: string
  input?: Record<string, any>
  result?: string
  isError?: boolean
  durationMs?: number
}

export interface TextBlock { type: 'text'; content: string }
export interface ToolBlock { type: 'tool'; call: ToolCallBlock }
export type MessageBlock = TextBlock | ToolBlock

interface ConfirmRequest {
  requestId: string
  tool: string
  input: Record<string, any>
  riskLevel: string
}

const props = defineProps<{
  role: string
  blocks: MessageBlock[]
  confirm?: ConfirmRequest | null
  isStreaming?: boolean
}>()

const emit = defineEmits<{
  confirm: [requestId: string, approved: boolean]
}>()

const expandedTools = ref<Set<string>>(new Set())

function toggleTool(id: string) {
  if (expandedTools.value.has(id)) {
    expandedTools.value.delete(id)
  } else {
    expandedTools.value.add(id)
  }
}

function renderMd(text: string) {
  return marked.parse(text || '') as string
}

function formatDuration(ms: number) {
  return ms >= 1000 ? (ms / 1000).toFixed(1) + 's' : ms + 'ms'
}
</script>

<template>
  <div class="chat-msg" :class="[`role-${role}`]">
    <div v-if="role === 'user'" class="msg-user">
      <span class="prompt">❯</span>
      <span class="user-text">{{ blocks[0]?.type === 'text' ? blocks[0].content : '' }}</span>
    </div>

    <div v-else class="msg-assistant-wrap">
      <span class="prompt prompt-assistant" :class="{ streaming: isStreaming }">*</span>
      <div class="assistant-body">
        <template v-for="(block, i) in blocks" :key="i">
          <div v-if="block.type === 'text' && block.content" class="msg-assistant">
            <div class="assistant-text" v-html="renderMd(block.content)"></div>
          </div>
          <div v-else-if="block.type === 'tool'" class="tool-calls">
            <div class="tool-call">
              <div class="tool-header" @click="toggleTool(block.call.id)">
                <span class="tool-arrow">{{ expandedTools.has(block.call.id) ? '▼' : '▶' }}</span>
                <span class="tool-badge">Tool</span>
                <span class="tool-name">{{ block.call.name }}</span>
                <span v-if="block.call.durationMs != null" class="tool-duration">{{ formatDuration(block.call.durationMs) }}</span>
                <span v-if="block.call.isError" class="tool-error-badge">error</span>
              </div>
              <div v-if="expandedTools.has(block.call.id)" class="tool-detail">
                <pre v-if="block.call.input" class="tool-input">{{ JSON.stringify(block.call.input, null, 2) }}</pre>
                <pre v-if="block.call.result" class="tool-result" :class="{ 'is-error': block.call.isError }">{{ block.call.result }}</pre>
              </div>
            </div>
          </div>
        </template>
        <span v-if="isStreaming" class="cursor">▊</span>
      </div>
    </div>

    <div v-if="confirm" class="confirm-bar" :class="confirm.riskLevel === 'dangerous' ? 'risk-dangerous' : confirm.riskLevel === 'safe' ? 'risk-safe' : 'risk-moderate'">
      <span class="confirm-label">{{ confirm.tool }}</span>
      <span class="risk-badge">{{ confirm.riskLevel }}</span>
      <button class="btn-confirm" @click="emit('confirm', confirm.requestId, true)">确认执行</button>
      <button class="btn-cancel" @click="emit('confirm', confirm.requestId, false)">取消</button>
    </div>
  </div>
</template>

<style scoped>
.chat-msg { padding: 8px 0; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 13px; }
.msg-user { display: flex; gap: 8px; color: var(--text); }
.msg-assistant-wrap { display: flex; gap: 8px; }
.prompt { color: var(--primary); font-weight: bold; flex-shrink: 0; }
.prompt-assistant { align-self: flex-start; margin-top: 2px; }
.prompt-assistant.streaming { animation: prompt-pulse 1.5s ease-in-out infinite; }
@keyframes prompt-pulse {
  0%, 100% { opacity: 0.4; text-shadow: 0 0 0 transparent; }
  50% { opacity: 1; text-shadow: 0 0 8px var(--primary); }
}
.assistant-body { flex: 1; min-width: 0; }
.msg-assistant { color: var(--text-sub); line-height: 1.6; }
.assistant-text :deep(code) { background: var(--input-bg); padding: 2px 6px; border-radius: 3px; font-size: 12px; }
.assistant-text :deep(pre) { background: var(--input-bg); padding: 12px; border-radius: 6px; overflow-x: auto; margin: 8px 0; }
.assistant-text :deep(ol), .assistant-text :deep(ul) { padding-left: 1.5em; margin: 4px 0; }
.cursor { color: var(--primary); animation: blink 1s step-end infinite; }
@keyframes blink { 50% { opacity: 0; } }

.tool-calls { margin: 8px 0; }
.tool-call { border: 1px solid var(--border); border-left: 3px solid var(--primary); border-radius: 6px; margin: 4px 0; overflow: hidden; }
.tool-header { padding: 6px 10px; cursor: pointer; display: flex; align-items: center; gap: 8px; background: var(--input-bg); }
.tool-header:hover { background: var(--row-hover); }
.tool-arrow { font-size: 10px; color: var(--muted); width: 12px; }
.tool-name { color: var(--primary); font-weight: 500; }
.tool-badge { font-size: 10px; padding: 1px 6px; border-radius: 3px; background: var(--primary); color: #fff; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px; }
.tool-duration { color: var(--muted); font-size: 11px; }
.tool-error-badge { background: var(--red); color: #fff; font-size: 10px; padding: 1px 6px; border-radius: 3px; }
.tool-detail { padding: 8px 10px; }
.tool-input, .tool-result { font-size: 12px; margin: 4px 0; white-space: pre-wrap; word-break: break-all; color: var(--text-sub); }
.tool-result.is-error { color: var(--red); }

.confirm-bar { display: flex; align-items: center; gap: 10px; padding: 8px 12px; border-radius: 6px; margin: 8px 0; }
.confirm-bar.risk-safe { background: rgba(74, 222, 128, 0.1); border: 1px solid var(--green); }
.confirm-bar.risk-moderate { background: rgba(251, 191, 36, 0.1); border: 1px solid var(--yellow); }
.confirm-bar.risk-dangerous { background: rgba(248, 113, 113, 0.1); border: 1px solid var(--red); }
.confirm-label { color: var(--text); font-weight: 500; }
.risk-badge { font-size: 11px; padding: 2px 8px; border-radius: 3px; }
.risk-safe .risk-badge { background: var(--green); color: #000; }
.risk-moderate .risk-badge { background: var(--yellow); color: #000; }
.risk-dangerous .risk-badge { background: var(--red); color: #fff; }
.btn-confirm { background: var(--primary); color: #fff; border: none; padding: 4px 14px; border-radius: 4px; cursor: pointer; font-size: 12px; }
.btn-confirm:hover { background: var(--primary-hover); }
.btn-cancel { background: transparent; color: var(--muted); border: 1px solid var(--border); padding: 4px 14px; border-radius: 4px; cursor: pointer; font-size: 12px; }
.btn-cancel:hover { background: var(--row-hover); }
</style>
