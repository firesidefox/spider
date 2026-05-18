<script setup lang="ts">
import { ref, computed, inject } from 'vue'
import { marked } from 'marked'
import type { ChatThemeTokens, ChatDensity } from '../chatTheme'

export interface ToolCallBlock {
  id: string
  name: string
  input?: Record<string, any>
  hostNames?: string[]
  result?: string
  summary?: string
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

const EXPLORE_TOOLS = new Set(['ListHosts', 'GetHost', 'SearchDocs', 'Verify', 'GetTopology'])

type TextItem     = { kind: 'text';        content: string }
type HkItem       = { kind: 'housekeeping'; call: ToolCallBlock }
type ExploreGroup = { kind: 'explore';     calls: ToolCallBlock[]; streaming: boolean }
type ActItem      = { kind: 'act';         call: ToolCallBlock; hosts: string[]; command: string }
type RenderItem   = TextItem | HkItem | ExploreGroup | ActItem

const chatTheme = inject<() => ChatThemeTokens>('chatTheme')
const chatDensity = inject<() => ChatDensity>('chatDensity')

const ctVars = computed(() => {
  const t = chatTheme?.()
  const d = chatDensity?.()
  if (!t || !d) return {}
  return {
    '--ct-text': t.text,
    '--ct-text-sub': t.textSub,
    '--ct-muted': t.muted,
    '--ct-label-color': t.labelColor,
    '--ct-primary': t.primary,
    '--ct-green': t.green,
    '--ct-red': t.red,
    '--ct-yellow': t.yellow,
    '--ct-purple': t.purple,
    '--ct-code-bg': t.codeBg,
    '--ct-code-border': t.codeBlockBorder,
    '--ct-font-size': d.fontSize,
    '--ct-font-size-mono': d.fontSizeMono,
    '--ct-line-height': d.lineHeight,
    '--ct-block-padding': d.blockPadding,
    '--ct-gutter-width': d.gutterWidth,
    '--ct-sub-gap': d.subLineGap,
  }
})

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
    } else if (block.call.name === 'Todo') {
    } else if (block.call.name === 'invoke_skill') {
      items.push({ kind: 'housekeeping', call: block.call })
    } else if (EXPLORE_TOOLS.has(block.call.name)) {
      const last = items[items.length - 1]
      if (last?.kind === 'explore') {
        const calls = [...last.calls, block.call]
        items[items.length - 1] = { kind: 'explore', calls, streaming: calls.some(c => c.durationMs == null) }
      } else {
        items.push({ kind: 'explore', calls: [block.call], streaming: block.call.durationMs == null })
      }
    } else {
      items.push({ kind: 'act', call: block.call, hosts: actHosts(block.call), command: actCommand(block.call) })
    }
  }
  return items
})


const collapsedGroups = ref<Set<string>>(new Set())
const expandedAct = ref<Set<string>>(new Set())

function toggleGroup(firstId: string) {
  if (collapsedGroups.value.has(firstId)) collapsedGroups.value.delete(firstId)
  else collapsedGroups.value.add(firstId)
}

function toggleAct(id: string) {
  if (expandedAct.value.has(id)) expandedAct.value.delete(id)
  else expandedAct.value.add(id)
}

function exploreParam(call: ToolCallBlock): string {
  if (!call.input) return ''
  const vals = Object.values(call.input)
  if (!vals.length) return ''
  const v = vals[0]
  const s = typeof v === 'string' ? v : JSON.stringify(v)
  return s.length > 32 ? s.slice(0, 32) + '…' : s
}

function skillArg(call: ToolCallBlock): string {
  if (!call.input) return ''
  const v = call.input['skill'] ?? call.input['name'] ?? Object.values(call.input)[0]
  return typeof v === 'string' ? v : ''
}

function actCommand(call: ToolCallBlock): string {
  if (!call.input) return ''
  const cmd = call.input['command'] ?? call.input['cmd'] ?? call.input['script']
  if (typeof cmd === 'string') return cmd
  const vals = Object.values(call.input)
  if (vals.length === 1 && typeof vals[0] === 'string') return vals[0]
  return JSON.stringify(call.input)
}

function actHosts(call: ToolCallBlock): string[] {
  if (call.hostNames && call.hostNames.length > 0) return call.hostNames
  if (!call.input) return []
  const ids = call.input['host_ids']
  if (Array.isArray(ids)) return ids
  const id = call.input['host_id']
  if (typeof id === 'string' && id) return [id]
  return []
}

function exploreTotalMs(calls: ToolCallBlock[]): number | null {
  let total = 0
  for (const c of calls) {
    if (c.durationMs == null) return null
    total += c.durationMs
  }
  return total
}

function exploreResult(call: ToolCallBlock): string {
  if (call.summary) return call.summary
  if (!call.result) return ''
  const first = call.result.split('\n')[0]
  return first.length > 40 ? first.slice(0, 40) + '…' : first
}

function renderMd(text: string) {
  return marked.parse(text || '') as string
}

function formatDuration(ms: number) {
  return ms >= 1000 ? (ms / 1000).toFixed(1) + 's' : ms + 'ms'
}
</script>

<template>
  <div class="chat-msg" :class="[`role-${role}`]" :style="ctVars">
    <div v-if="role === 'user'" class="msg-user">
      <div class="gutter"><span class="prompt">❯</span></div>
      <div class="content"><span class="user-text">{{ blocks[0]?.type === 'text' ? blocks[0].content : '' }}</span></div>
    </div>

    <div v-else class="msg-assistant-wrap">
      <template v-for="(item, idx) in renderItems" :key="idx">

        <div v-if="item.kind === 'text'" class="block-row">
          <div class="gutter"><span class="dot" :class="{ pulsing: isStreaming && idx === renderItems.length - 1 }">*</span></div>
          <div class="block-body">
            <div class="assistant-text" v-html="renderMd(item.content)"></div>
            <span v-if="isStreaming && idx === renderItems.length - 1" class="cursor">▊</span>
          </div>
        </div>

        <div v-else-if="item.kind === 'housekeeping'" class="block-row">
          <div class="gutter"><span class="dot" :class="{ pulsing: isStreaming && item.call.durationMs == null }">*</span></div>
          <div class="block-body">
            <div class="hk-line">
              <span class="hk-fn">{{ item.call.name }}</span><span class="hk-paren">(</span><span class="hk-arg">{{ skillArg(item.call) }}</span><span class="hk-paren">)</span>
              <span class="tool-hd-spacer"></span>
              <span v-if="isStreaming && item.call.durationMs == null" class="streaming-dots">···</span>
              <span v-if="item.call.durationMs != null" class="dur">{{ formatDuration(item.call.durationMs) }}</span>
            </div>
            <div v-if="item.call.durationMs != null && item.call.summary" class="sub-line">
              <span class="hook">└</span><span class="sub-text">{{ item.call.summary }}</span>
            </div>
          </div>
        </div>

              <!-- Explore group -->
        <div v-else-if="item.kind === 'explore'" class="block-row">
          <div class="gutter"><span class="dot" :class="{ pulsing: isStreaming && item.streaming }">*</span></div>
          <div class="block-body">
            <div class="tool-hd" @click="toggleGroup(item.calls[0].id)">
              <span class="tool-fn">Explore</span><span class="tool-paren">(</span><span class="tool-arg">{{ item.calls.length }} tools</span><span class="tool-paren">)</span>
              <span class="tool-hd-spacer"></span>
              <span v-if="isStreaming && item.streaming" class="streaming-dots">···</span>
              <span v-if="exploreTotalMs(item.calls) != null" class="dur">{{ formatDuration(exploreTotalMs(item.calls)!) }}</span>
              <span class="expand-toggle">{{ collapsedGroups.has(item.calls[0].id) ? '▶' : '▼' }}</span>
            </div>
            <div v-if="!collapsedGroups.has(item.calls[0].id)" class="sub-lines">
              <div v-for="call in item.calls" :key="call.id" class="sub-line">
                <span class="hook">└</span>
                <span class="sub-fn" :class="{ 'is-error': call.isError }">{{ call.name }}</span>
                <span class="sub-param">{{ exploreParam(call) }}</span>
                <template v-if="call.durationMs != null">
                  <span :class="call.isError ? 'res-err' : 'res-ok'">{{ exploreResult(call) }}</span>
                  <span class="dur">{{ formatDuration(call.durationMs) }}</span>
                </template>
                <span v-else-if="isStreaming" class="streaming-dots">···</span>
              </div>
            </div>
          </div>
        </div>

        <div v-else class="block-row">
          <div class="gutter"><span class="dot" :class="{ pulsing: isStreaming && item.call.durationMs == null, 'dot-err': item.call.isError }">*</span></div>
          <div class="block-body">
            <div class="tool-hd" @click="item.call.durationMs != null && toggleAct(item.call.id)" :style="item.call.durationMs != null ? 'cursor:pointer' : ''">
              <span class="act-arrow" :class="{ 'act-arrow-err': item.call.isError }">{{ expandedAct.has(item.call.id) ? '▼' : '▶' }}</span>
              <span class="tool-fn" :class="{ 'tool-fn-err': item.call.isError }">{{ item.call.name }}</span>
              <template v-if="item.hosts.length">
                <span class="tool-at">@</span>
                <span class="tool-hosts" :class="{ 'tool-hosts-err': item.call.isError }">
                  <template v-for="(h, hi) in item.hosts" :key="hi"><span v-if="hi > 0" class="act-sep">·</span>{{ h }}</template>
                </span>
              </template>
              <span class="tool-hd-spacer"></span>
              <span v-if="isStreaming && item.call.durationMs == null" class="streaming-dots">···</span>
              <span v-if="item.call.durationMs != null" class="dur">{{ formatDuration(item.call.durationMs) }}</span>
            </div>
            <template v-if="item.call.durationMs != null">
              <div v-if="item.command" class="sub-line">
                <span class="hook">⎿</span><span class="act-cmd">{{ item.command }}</span>
              </div>
              <div v-if="expandedAct.has(item.call.id) && item.call.result" class="sub-line sub-line-full">
                <span class="hook">⎿</span>
                <pre :class="item.call.isError ? 'res-err' : 'res-ok'" class="act-output-pre">{{ item.call.result }}</pre>
              </div>
              <div v-else-if="item.call.summary || item.call.result" class="sub-line">
                <span class="hook">⎿</span>
                <span :class="item.call.isError ? 'res-err' : 'res-ok'">{{ item.call.summary || item.call.result?.split('\n')[0] }}</span>
              </div>
            </template>
            <div v-else-if="isStreaming" class="sub-line">
              <span class="hook">⎿</span><span class="streaming-dots">···</span>
            </div>
          </div>
        </div>

      </template>
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
.chat-msg { padding: 8px 0; font-family: 'SF Mono', 'Fira Code', monospace; font-size: var(--ct-font-size-mono, 13px); }
.msg-user { display: flex; color: var(--ct-text, var(--text)); }
.msg-assistant-wrap { display: flex; flex-direction: column; }
.gutter { flex-shrink: 0; width: var(--ct-gutter-width, 20px); text-align: center; padding-top: 2px; }
.content { flex: 1; min-width: 0; }
.prompt { color: var(--ct-primary, var(--primary)); font-weight: bold; }

/* Block row: gutter + body side by side */
.block-row { display: flex; align-items: flex-start; padding: var(--ct-block-padding, 2px 0); }
.block-body { flex: 1; min-width: 0; }

/* Gutter dot (the * status icon) */
.dot { color: var(--ct-primary, var(--primary)); font-weight: bold; font-size: 18px; line-height: 1.2; display: inline-block; }
.dot.pulsing { animation: dot-pulse 1.5s ease-in-out infinite; }
.dot.dot-err { color: var(--ct-red, var(--red)); }
@keyframes dot-pulse { 0%, 100% { opacity: 0.3; } 50% { opacity: 1; } }

/* Text block */
.assistant-text { font-family: -apple-system, 'Segoe UI', sans-serif; font-size: var(--ct-font-size, 13.5px); color: var(--ct-text-sub, var(--text-sub)); line-height: var(--ct-line-height, 1.65); }
.assistant-text :deep(h1),
.assistant-text :deep(h2) { font-size: 14px; font-weight: 600; color: var(--ct-text, var(--text)); margin: 0 0 8px; }
.assistant-text :deep(h3) { font-size: 11px; font-weight: 700; color: var(--ct-label-color, var(--label)); margin: 10px 0 3px; text-transform: uppercase; letter-spacing: 0.8px; }
.assistant-text :deep(p) { margin-bottom: 7px; }
.assistant-text :deep(p:last-child) { margin-bottom: 0; }
.assistant-text :deep(strong) { color: var(--ct-text, var(--text)); }
.assistant-text :deep(code) { background: var(--ct-code-bg, var(--input-bg)); color: var(--ct-purple, var(--purple)); padding: 1px 5px; border-radius: 3px; font-family: 'SF Mono', monospace; font-size: 11.5px; }
.assistant-text :deep(pre) { background: var(--ct-code-bg, var(--panel)); border: 1px solid var(--ct-code-border, var(--border)); border-left: 3px solid var(--ct-code-border, var(--border)); border-radius: 0 5px 5px 0; padding: 8px 12px; margin: 7px 0; overflow-x: auto; }
.assistant-text :deep(pre code) { background: none; color: var(--ct-label-color, var(--label)); padding: 0; font-size: 11.5px; line-height: 1.55; }
.assistant-text :deep(ul) { padding-left: 1.3em; margin: 3px 0 7px; }
.assistant-text :deep(ol) { padding-left: 1.3em; margin: 3px 0 7px; }
.assistant-text :deep(li) { margin-bottom: 3px; color: var(--ct-label-color, var(--label)); }
.assistant-text :deep(ol li::marker) { color: var(--ct-primary, var(--primary)); }
.assistant-text :deep(ul li::marker) { color: var(--ct-primary, var(--primary)); }
.assistant-text :deep(blockquote) { border-left: 2px solid var(--ct-code-border, var(--border)); padding-left: 10px; color: var(--ct-label-color, var(--label)); margin: 7px 0; font-size: 13px; }
.assistant-text :deep(table) { width: 100%; border-collapse: collapse; margin: 8px 0; font-size: 12.5px; }
.assistant-text :deep(th) { color: var(--ct-primary, var(--primary)); font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid var(--ct-code-border, var(--border)); padding: 5px 10px; text-align: left; }
.assistant-text :deep(td) { padding: 5px 10px; border-bottom: 1px solid var(--ct-code-border, var(--border)); color: var(--ct-text-sub, var(--text-sub)); }
.cursor { color: var(--ct-primary, var(--primary)); animation: blink 1s step-end infinite; }
@keyframes blink { 50% { opacity: 0; } }

/* Tool header line: ▶ ToolName @host ··· dur */
.tool-hd { display: flex; align-items: baseline; flex-wrap: wrap; padding: 1px 0; }
.act-arrow { font-size: 11px; margin-right: 6px; flex-shrink: 0; align-self: center; color: var(--ct-primary, var(--primary)); }
.act-arrow-err { color: var(--ct-red, var(--red)); }
.tool-fn { color: var(--ct-primary, var(--primary)); font-weight: 600; font-size: 12.5px; flex-shrink: 0; }
.tool-fn-err { color: var(--ct-red, var(--red)); }
.tool-paren { color: var(--ct-label-color, var(--label)); font-size: 12px; }
.tool-arg { color: var(--ct-text-sub, var(--text-sub)); font-size: 11.5px; max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tool-at { color: var(--ct-label-color, var(--label)); font-size: 11px; margin: 0 4px; }
.tool-hosts { color: var(--ct-text-sub, var(--text-sub)); font-size: 11px; white-space: normal; }
.tool-hosts-err { color: var(--ct-red, var(--red)); opacity: 0.5; }
.act-sep { margin: 0 4px; color: var(--ct-label-color, var(--label)); }
.act-cmd { color: var(--ct-text-sub, var(--text-sub)); font-size: 11px; white-space: pre-wrap; word-break: break-all; }
.tool-hd-spacer { flex: 1; }
.dur { color: var(--ct-label-color, var(--label)); font-size: 10.5px; margin-left: 8px; flex-shrink: 0; }
.expand-toggle { color: var(--ct-label-color, var(--label)); font-size: 10px; margin-left: 6px; flex-shrink: 0; }
.streaming-dots { color: var(--ct-label-color, var(--label)); font-size: 11px; margin-left: 8px; animation: blink 1s step-end infinite; }

/* Sub-lines (tool output rows) */
.sub-lines { padding: 0; }
.sub-line { display: flex; align-items: flex-start; gap: 5px; padding: 0; }
.sub-line-full { align-items: flex-start; }
.hook { color: var(--ct-label-color, var(--label)); font-size: 11px; flex-shrink: 0; margin-top: 1px; }
.sub-text { color: var(--ct-text-sub, var(--text-sub)); font-size: 11px; }
.sub-fn { color: var(--ct-text-sub, var(--text-sub)); font-size: 11.5px; font-weight: 500; flex-shrink: 0; }
.sub-fn.is-error { color: var(--ct-red, var(--red)); }
.sub-param { color: var(--ct-label-color, var(--label)); font-size: 11px; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.hk-line { display: flex; align-items: baseline; flex-wrap: wrap; gap: 0; }
.hk-fn { color: var(--ct-label-color, var(--label)); font-weight: 500; font-size: 12px; }
.hk-paren { color: var(--ct-label-color, var(--label)); font-size: 12px; }
.hk-arg { color: var(--ct-primary, var(--primary)); font-size: 11.5px; }
.res-ok { color: var(--ct-green, var(--green)); font-size: 11px; }
.res-err { color: var(--ct-red, var(--red)); font-size: 11px; }
.act-output-pre { margin: 0; font-family: inherit; font-size: 11px; white-space: pre-wrap; word-break: break-all; line-height: 1.55; }

/* confirm-bar uses global theme vars intentionally — system UI, not chat content */
.confirm-bar { display: flex; flex-direction: column; gap: 6px; padding: 8px 12px; border-radius: 6px; margin: 8px 0; }
.confirm-header { display: flex; align-items: center; gap: 10px; }
.confirm-input { font-size: 12px; margin: 0; white-space: pre-wrap; word-break: break-all; color: var(--ct-text-sub, var(--text-sub)); background: transparent; border: none; padding: 0; font-family: inherit; }
.confirm-bar.risk-safe { background: rgba(74, 222, 128, 0.1); border: 1px solid var(--ct-green, var(--green)); }
.confirm-bar.risk-moderate { background: rgba(251, 191, 36, 0.1); border: 1px solid var(--ct-yellow, var(--yellow)); }
.confirm-bar.risk-dangerous { background: rgba(248, 113, 113, 0.1); border: 1px solid var(--ct-red, var(--red)); }
.confirm-label { color: var(--ct-text, var(--text)); font-weight: 500; }
.risk-badge { font-size: 11px; padding: 2px 8px; border-radius: 3px; }
.risk-safe .risk-badge { background: var(--ct-green, var(--green)); color: #000; }
.risk-moderate .risk-badge { background: var(--ct-yellow, var(--yellow)); color: #000; }
.risk-dangerous .risk-badge { background: var(--ct-red, var(--red)); color: #fff; }
.btn-confirm { background: var(--ct-primary, var(--primary)); color: #fff; border: none; padding: 4px 14px; border-radius: 4px; cursor: pointer; font-size: 12px; }
.btn-confirm:hover { background: var(--primary-hover); }
.btn-cancel { background: transparent; color: var(--ct-muted, var(--muted)); border: 1px solid var(--border); padding: 4px 14px; border-radius: 4px; cursor: pointer; font-size: 12px; }
.btn-cancel:hover { background: var(--row-hover); }
</style>
