<script setup lang="ts">
import { ref, computed } from 'vue'
import { marked } from 'marked'

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
  <div class="chat-msg" :class="[`role-${role}`]">
    <div v-if="role === 'user'" class="msg-user">
      <div class="gutter"><span class="prompt">❯</span></div>
      <div class="content"><span class="user-text">{{ blocks[0]?.type === 'text' ? blocks[0].content : '' }}</span></div>
    </div>

    <div v-else class="msg-assistant-wrap">
      <div class="content assistant-body">
        <div class="assistant-lead">
          <span class="prompt-assistant" :class="{ streaming: isStreaming }">*</span>
          <div class="assistant-lead-body">
            <template v-for="(item, idx) in renderItems" :key="idx">
              <!-- Text block -->
              <div v-if="item.kind === 'text'" class="msg-assistant">
                <div class="assistant-text" v-html="renderMd(item.content, isStreaming)"></div>
              </div>

              <!-- Housekeeping: invoke_skill -->
              <div v-else-if="item.kind === 'housekeeping'" class="hk">
                <span v-if="item.call.durationMs == null" class="star">*</span>
                <span v-else class="hk-dot">·</span>
                <span class="hk-call">
                  <span class="hk-fn">{{ item.call.name }}</span><span class="hk-paren">(</span><span class="hk-arg">{{ skillArg(item.call) }}</span><span class="hk-paren">)</span>
                </span>
                <span v-if="item.call.durationMs != null" class="hk-dur">{{ formatDuration(item.call.durationMs) }}</span>
                <div v-if="item.call.durationMs != null && item.call.summary" class="hk-sub">
                  <span class="hook">⎿</span><span class="hk-result">{{ item.call.summary }}</span>
                </div>
              </div>

              <!-- Explore group -->
              <div v-else-if="item.kind === 'explore'" class="explore-group">
                <div class="explore-group-header" @click="toggleGroup(item.calls[0].id)">
                  <span v-if="item.streaming" class="star" style="width:10px">*</span>
                  <span v-else class="ex-arrow">{{ collapsedGroups.has(item.calls[0].id) ? '▶' : '▼' }}</span>
                  <span class="explore-label">Explore</span>
                  <span class="explore-count">({{ item.calls.length }})</span>
                </div>
                <div v-if="!collapsedGroups.has(item.calls[0].id)" class="explore-items">
                  <div v-for="call in item.calls" :key="call.id" class="explore-item">
                    <span class="tree-branch">└</span>
                    <span class="explore-tool-name" :class="{ 'is-error': call.isError }">{{ call.name }}</span>
                    <span class="explore-param">{{ exploreParam(call) }}</span>
                    <template v-if="call.durationMs != null">
                      <span v-if="call.isError" class="ex-err">{{ exploreResult(call) }}</span>
                      <span v-else class="ex-ok">{{ exploreResult(call) }}</span>
                      <span class="tool-duration">{{ formatDuration(call.durationMs) }}</span>
                    </template>
                    <span v-else class="explore-streaming">···</span>
                  </div>
                </div>
              </div>

              <!-- Act tool -->
              <div v-else class="act-block">
                <div class="act-hd" @click="item.call.durationMs != null && toggleAct(item.call.id)" :style="item.call.durationMs != null ? 'cursor:pointer' : ''">
                  <span v-if="item.call.durationMs == null" class="star">*</span>
                  <span v-else class="act-arrow" :class="{ 'act-arrow-err': item.call.isError }">{{ expandedAct.has(item.call.id) ? '▼' : '▶' }}</span>
                  <span class="act-name" :class="{ 'act-name-err': item.call.isError }">{{ item.call.name }}</span>
                  <template v-if="item.hosts.length">
                    <span class="act-at">@</span>
                    <span class="act-hosts" :class="{ 'act-hosts-err': item.call.isError }">
                      <template v-for="(h, hi) in item.hosts" :key="hi">
                        <span v-if="hi > 0" class="act-sep">·</span>{{ h }}
                      </template>
                    </span>
                  </template>
                  <span v-if="item.call.durationMs != null" class="act-dur">{{ formatDuration(item.call.durationMs) }}</span>
                  <span v-else class="act-streaming">···</span>
                </div>
                <div class="act-sub">
                  <div v-if="item.command" class="act-cmd-row">
                    <span class="hook">⎿</span><span class="act-cmd">{{ item.command }}</span>
                  </div>
                  <template v-if="item.call.durationMs != null">
                    <div v-if="expandedAct.has(item.call.id) && item.call.result" class="act-res-row act-output-full">
                      <span class="hook">⎿</span>
                      <pre :class="item.call.isError ? 'res-err' : 'res-ok'" class="act-output-pre">{{ item.call.result }}</pre>
                    </div>
                    <div v-else-if="item.call.summary || item.call.result" class="act-res-row">
                      <span class="hook">⎿</span>
                      <span :class="item.call.isError ? 'res-err' : 'res-ok'">{{ item.call.summary || item.call.result?.split('\n')[0] }}</span>
                    </div>
                  </template>
                  <div v-else-if="item.command" class="act-res-row">
                    <span class="hook">⎿</span><span class="act-streaming">···</span>
                  </div>
                </div>
              </div>
            </template>

            <span v-if="isStreaming && renderItems.length > 0 && renderItems[renderItems.length - 1].kind === 'text'" class="cursor">▊</span>
          </div><!-- .assistant-lead-body -->
        </div><!-- .assistant-lead -->
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
.assistant-lead { display: flex; flex-direction: row; align-items: flex-start; gap: 6px; }
.assistant-lead-body { flex: 1; min-width: 0; }
.prompt-assistant { flex-shrink: 0; color: var(--primary); font-weight: bold; margin-top: 2px; width: 14px; text-align: center; }
.prompt-assistant.streaming { animation: prompt-pulse 1.5s ease-in-out infinite; }
@keyframes prompt-pulse {
  0%, 100% { opacity: 0.4; text-shadow: 0 0 0 transparent; }
  50% { opacity: 1; text-shadow: 0 0 8px var(--primary); }
}
.assistant-body { }
.msg-assistant { line-height: 1.6; }
.assistant-text { font-family: -apple-system, 'Segoe UI', sans-serif; font-size: 13.5px; color: var(--text-sub); line-height: 1.65; }
.assistant-text :deep(h1),
.assistant-text :deep(h2) { font-size: 14px; font-weight: 600; color: var(--text); margin: 0 0 8px; }
.assistant-text :deep(h3) { font-size: 11px; font-weight: 700; color: var(--label); margin: 10px 0 3px; text-transform: uppercase; letter-spacing: 0.8px; }
.assistant-text :deep(p) { margin-bottom: 7px; }
.assistant-text :deep(p:last-child) { margin-bottom: 0; }
.assistant-text :deep(strong) { color: var(--text); }
.assistant-text :deep(code) { background: var(--input-bg); color: var(--purple); padding: 1px 5px; border-radius: 3px; font-family: 'SF Mono', monospace; font-size: 11.5px; }
.assistant-text :deep(pre) { background: var(--panel); border: 1px solid var(--border); border-left: 3px solid var(--border); border-radius: 0 5px 5px 0; padding: 8px 12px; margin: 7px 0; overflow-x: auto; }
.assistant-text :deep(pre code) { background: none; color: var(--label); padding: 0; font-size: 11.5px; line-height: 1.55; }
.assistant-text :deep(ul) { padding-left: 1.3em; margin: 3px 0 7px; }
.assistant-text :deep(ol) { padding-left: 1.3em; margin: 3px 0 7px; }
.assistant-text :deep(li) { margin-bottom: 3px; color: var(--label); }
.assistant-text :deep(ol li::marker) { color: var(--primary); }
.assistant-text :deep(ul li::marker) { color: var(--primary); }
.assistant-text :deep(blockquote) { border-left: 2px solid var(--border); padding-left: 10px; color: var(--label); margin: 7px 0; font-size: 13px; }
.assistant-text :deep(table) { width: 100%; border-collapse: collapse; margin: 8px 0; font-size: 12.5px; }
.assistant-text :deep(th) { color: var(--primary); font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid var(--border); padding: 5px 10px; text-align: left; }
.assistant-text :deep(td) { padding: 5px 10px; border-bottom: 1px solid var(--border); color: var(--text-sub); }
.cursor { color: var(--primary); animation: blink 1s step-end infinite; }
@keyframes blink { 50% { opacity: 0; } }

/* Explore group */
.explore-group { margin: 3px 0; }
.explore-group-header { display: flex; align-items: center; gap: 6px; padding: 2px 0; cursor: pointer; }
.ex-arrow { color: #484f58; font-size: 10px; width: 10px; }
.explore-label { color: #6e7681; font-size: 11.5px; font-weight: 600; }
.explore-count { color: #484f58; font-size: 11px; }
.explore-items { padding-left: 16px; }
.explore-item { display: flex; align-items: baseline; gap: 7px; padding: 1px 0; }
.tree-branch { color: #3d444d; font-size: 11px; }
.explore-tool-name { color: #6e7681; font-size: 11.5px; font-weight: 500; }
.explore-tool-name.is-error { color: #f85149; }
.explore-param { color: #484f58; font-size: 11px; flex: 1; }
.ex-ok { color: #3fb950; font-size: 10.5px; }
.ex-err { color: #f85149; font-size: 10.5px; }
.tool-duration { color: #3d444d; font-size: 10.5px; margin-left: auto; }
.explore-streaming { color: #484f58; font-size: 11px; margin-left: auto; animation: blink 1s step-end infinite; }

/* Housekeeping */
.hk { display: flex; align-items: center; flex-wrap: wrap; padding: 1px 0; gap: 0; }
.hk-dot { color: #3d444d; font-size: 9px; margin-right: 8px; }
.hk-call { color: #484f58; font-size: 11.5px; }
.hk-fn { color: #484f58; font-weight: 500; }
.hk-paren { color: #3d444d; }
.hk-arg { color: #3d444d; }
.hk-dur { margin-left: auto; color: #2d333b; font-size: 10.5px; }
.hk-sub { width: 100%; padding: 0 0 2px 18px; }
.hk-result { color: #484f58; font-size: 11px; }

/* Act tool */
.act-block { margin: 3px 0; }
.act-hd { display: flex; flex-wrap: wrap; align-items: baseline; padding: 3px 0; row-gap: 0; column-gap: 0; }
.act-arrow { font-size: 11px; margin-right: 6px; flex-shrink: 0; align-self: center; color: #58a6ff; }
.act-arrow-err { color: #f85149; }
.act-name { font-weight: 600; font-size: 12px; flex-shrink: 0; color: #58a6ff; }
.act-name-err { color: #f85149; }
.act-at { color: #3d444d; font-size: 11px; margin: 0 5px; flex-shrink: 0; }
.act-hosts { color: #484f58; font-size: 11px; flex: 1; white-space: normal; word-break: break-word; }
.act-hosts-err { color: #f8514466; }
.act-sep { margin: 0 4px; color: #3d444d; }
.act-dur { color: #484f58; font-size: 10.5px; margin-left: auto; flex-shrink: 0; padding-left: 12px; }
.act-streaming { color: #484f58; font-size: 11px; margin-left: auto; animation: blink 1s step-end infinite; }
.act-sub { padding: 0 0 5px 18px; }
.act-cmd-row { display: flex; align-items: flex-start; }
.act-res-row { display: flex; align-items: flex-start; }
.hook { color: #3d444d; font-size: 11px; margin-right: 5px; flex-shrink: 0; }
.act-cmd { color: #6e7681; font-size: 11px; white-space: pre-wrap; word-break: break-all; }
.res-ok { color: #3fb950; font-size: 11.5px; }
.res-err { color: #f85149; font-size: 11.5px; }
.act-output-pre { margin: 0; font-family: inherit; font-size: 11px; white-space: pre-wrap; word-break: break-all; line-height: 1.55; }

/* Streaming star */
.star { color: #58a6ff; font-weight: bold; font-size: 11px; margin-right: 6px; animation: star-pulse 1.5s ease-in-out infinite; display: inline-block; flex-shrink: 0; }
@keyframes star-pulse { 0%, 100% { opacity: 0.3; } 50% { opacity: 1; } }

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
