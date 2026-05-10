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

const EXPLORE_TOOLS = new Set(['ListHosts', 'GetHost', 'SearchDocs', 'Verify'])

type TextItem    = { kind: 'text';    content: string }
type ExploreGroup = { kind: 'explore'; calls: ToolCallBlock[] }
type ActItem     = { kind: 'act';     call: ToolCallBlock }
type RenderItem  = TextItem | ExploreGroup | ActItem

const props = defineProps<{
  role: string
  blocks: MessageBlock[]
  confirm?: ConfirmRequest | null
  isStreaming?: boolean
}>()

const emit = defineEmits<{
  confirm: [requestId: string, approved: boolean]
}>()

const renderItems = computed<RenderItem[]>(() => {
  const items: RenderItem[] = []
  for (const block of props.blocks) {
    if (block.type === 'text') {
      if (block.content) items.push({ kind: 'text', content: block.content })
    } else if (EXPLORE_TOOLS.has(block.call.name)) {
      const last = items[items.length - 1]
      if (last?.kind === 'explore') {
        items[items.length - 1] = { kind: 'explore', calls: [...last.calls, block.call] }
      } else {
        items.push({ kind: 'explore', calls: [block.call] })
      }
    } else {
      items.push({ kind: 'act', call: block.call })
    }
  }
  return items
})

const expandedTools = ref<Set<string>>(new Set())
const collapsedGroups = ref<Set<string>>(new Set())

function toggle(set: Set<string>, key: string) {
  if (set.has(key)) set.delete(key)
  else set.add(key)
}

function toggleTool(id: string) { toggle(expandedTools.value, id) }
function toggleGroup(firstId: string) { toggle(collapsedGroups.value, firstId) }

function exploreParam(call: ToolCallBlock): string {
  if (!call.input) return ''
  const vals = Object.values(call.input)
  if (!vals.length) return ''
  const v = vals[0]
  const s = typeof v === 'string' ? v : JSON.stringify(v)
  return s.length > 32 ? s.slice(0, 32) + '…' : s
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
      <div class="gutter"><span class="prompt">❯</span></div>
      <div class="content"><span class="user-text">{{ blocks[0]?.type === 'text' ? blocks[0].content : '' }}</span></div>
    </div>

    <div v-else class="msg-assistant-wrap">
      <div class="gutter"><span class="prompt prompt-assistant" :class="{ streaming: isStreaming }">*</span></div>
      <div class="content assistant-body">
        <template v-for="(item, idx) in renderItems" :key="idx">
          <!-- Text block -->
          <div v-if="item.kind === 'text'" class="msg-assistant">
            <div class="assistant-text" v-html="renderMd(item.content, isStreaming)"></div>
          </div>

          <!-- Explore group -->
          <div v-else-if="item.kind === 'explore'" class="explore-group">
            <div class="explore-group-header" @click="toggleGroup(item.calls[0].id)">
              <span class="tool-arrow">{{ collapsedGroups.has(item.calls[0].id) ? '▶' : '▼' }}</span>
              <span class="explore-label">Explored</span>
              <span class="explore-count">({{ item.calls.length }})</span>
            </div>
            <div v-if="!collapsedGroups.has(item.calls[0].id)" class="explore-items">
              <div v-for="call in item.calls" :key="call.id" class="explore-item">
                <span class="tree-branch">└</span>
                <span class="explore-tool-name" :class="{ 'is-error': call.isError }">{{ call.name }}</span>
                <span v-if="exploreParam(call)" class="explore-param">{{ exploreParam(call) }}</span>
                <span v-if="call.isError" class="explore-error-mark">✕</span>
                <span v-else-if="call.durationMs != null" class="tool-duration">{{ formatDuration(call.durationMs) }}</span>
                <span v-else class="explore-streaming">···</span>
              </div>
            </div>
          </div>

          <!-- Act tool -->
          <div v-else class="tool-calls">
            <div class="tool-call" :class="{ 'has-error': item.call.isError }">
              <div class="tool-header" @click="toggleTool(item.call.id)">
                <span class="tool-arrow">{{ expandedTools.has(item.call.id) ? '▼' : '▶' }}</span>
                <span class="tool-badge">Tool</span>
                <span class="tool-name">{{ item.call.name }}</span>
                <span v-if="item.call.isError" class="tool-error-badge">✕</span>
                <span v-else-if="item.call.durationMs != null" class="tool-duration">{{ formatDuration(item.call.durationMs) }}</span>
                <span v-else class="act-streaming">···</span>
              </div>
              <div v-if="expandedTools.has(item.call.id)" class="tool-detail">
                <div v-if="item.call.input && Object.keys(item.call.input).length" class="tool-section">
                  <span class="tool-section-label input-label">INPUT</span>
                  <pre class="tool-input">{{ JSON.stringify(item.call.input, null, 2) }}</pre>
                </div>
                <div v-if="item.call.result" class="tool-section">
                  <span class="tool-section-label" :class="item.call.isError ? 'error-label' : 'output-label'">{{ item.call.isError ? 'ERROR' : 'OUTPUT' }}</span>
                  <pre class="tool-result" :class="{ 'is-error': item.call.isError }">{{ item.call.result }}</pre>
                </div>
              </div>
            </div>
          </div>
        </template>

        <span v-if="isStreaming" class="cursor">▊</span>
      </div><!-- .content -->
    </div>

    <div v-if="confirm" class="confirm-bar" :class="confirm.riskLevel === 'dangerous' ? 'risk-dangerous' : confirm.riskLevel === 'safe' ? 'risk-safe' : 'risk-moderate'">
      <div class="confirm-header">
        <span class="confirm-label">{{ confirm.tool }}</span>
        <span class="risk-badge">{{ confirm.riskLevel }}</span>
        <button class="btn-confirm" @click="emit('confirm', confirm.requestId, true)">确认执行</button>
        <button class="btn-cancel" @click="emit('confirm', confirm.requestId, false)">取消</button>
      </div>
      <pre v-if="confirm.input && Object.keys(confirm.input).length" class="confirm-input">{{ JSON.stringify(confirm.input, null, 2) }}</pre>
    </div>
  </div>
</template>

<style scoped>
.chat-msg { padding: 8px 0; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 13px; }
.msg-user { display: flex; color: var(--text); }
.msg-assistant-wrap { display: flex; }
.gutter { flex-shrink: 0; width: 20px; text-align: center; }
.content { flex: 1; min-width: 0; }
.prompt { color: var(--primary); font-weight: bold; }
.prompt-assistant { display: inline-block; margin-top: 2px; }
.prompt-assistant.streaming { animation: prompt-pulse 1.5s ease-in-out infinite; }
@keyframes prompt-pulse {
  0%, 100% { opacity: 0.4; text-shadow: 0 0 0 transparent; }
  50% { opacity: 1; text-shadow: 0 0 8px var(--primary); }
}
.assistant-body { }
.msg-assistant { color: var(--text-sub); line-height: 1.6; }
.assistant-text :deep(code) { background: var(--input-bg); padding: 2px 6px; border-radius: 3px; font-size: 12px; }
.assistant-text :deep(pre) { background: var(--input-bg); padding: 12px; border-radius: 6px; overflow-x: auto; margin: 8px 0; }
.assistant-text :deep(ol), .assistant-text :deep(ul) { padding-left: 1.5em; margin: 4px 0; }
.cursor { color: var(--primary); animation: blink 1s step-end infinite; }
@keyframes blink { 50% { opacity: 0; } }

/* Explore group */
.explore-group { margin: 6px 0; }
.explore-group-header { display: flex; align-items: center; gap: 6px; padding: 3px 0; cursor: pointer; }
.explore-group-header:hover .explore-label { color: var(--text-sub); }
.explore-label { color: var(--muted); font-size: 12px; font-weight: 600; }
.explore-count { color: var(--muted); font-size: 11px; opacity: 0.6; }
.explore-items { padding-left: 14px; }
.explore-item { display: flex; align-items: center; gap: 8px; padding: 2px 0; }
.tree-branch { color: var(--muted); font-size: 12px; opacity: 0.5; }
.explore-tool-name { color: var(--text-sub); font-size: 12px; font-weight: 500; }
.explore-tool-name.is-error { color: var(--red); }
.explore-param { color: var(--muted); font-size: 11px; }
.explore-error-mark { color: var(--red); font-size: 11px; }
.explore-streaming { color: var(--muted); font-size: 11px; animation: blink 1s step-end infinite; }

/* Act tool */
.tool-calls { margin: 8px 0; }
.tool-call { border: 1px solid var(--border); border-left: 3px solid var(--primary); border-radius: 6px; margin: 4px 0; overflow: hidden; }
.tool-call.has-error { border-left-color: var(--red); }
.tool-header { padding: 6px 10px; cursor: pointer; display: flex; align-items: center; gap: 8px; background: var(--input-bg); }
.tool-header:hover { background: var(--row-hover); }
.tool-arrow { font-size: 10px; color: var(--muted); width: 12px; }
.tool-name { color: var(--primary); font-weight: 500; }
.tool-badge { font-size: 10px; padding: 1px 6px; border-radius: 3px; background: var(--primary); color: #fff; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px; }
.tool-duration { color: var(--muted); font-size: 11px; margin-left: auto; }
.tool-error-badge { background: var(--red); color: #fff; font-size: 10px; padding: 1px 6px; border-radius: 3px; margin-left: auto; }
.act-streaming { color: var(--muted); font-size: 11px; margin-left: auto; animation: blink 1s step-end infinite; }
.tool-detail { padding: 0; }
.tool-section { border-top: 1px solid var(--border); }
.tool-section-label { display: block; font-size: 10px; font-weight: 700; letter-spacing: 0.8px; padding: 4px 10px 2px; }
.input-label { color: var(--primary); }
.output-label { color: var(--green, #4ade80); }
.error-label { color: var(--red); }
.tool-input, .tool-result { font-size: 12px; margin: 0; padding: 6px 10px 8px; white-space: pre-wrap; word-break: break-all; color: var(--text-sub); }
.tool-result.is-error { color: var(--red); }

.confirm-bar { display: flex; flex-direction: column; gap: 6px; padding: 8px 12px; border-radius: 6px; margin: 8px 0; }
.confirm-header { display: flex; align-items: center; gap: 10px; }
.confirm-input { font-size: 12px; margin: 0; white-space: pre-wrap; word-break: break-all; color: var(--text-sub); background: transparent; border: none; padding: 0; font-family: inherit; }
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
