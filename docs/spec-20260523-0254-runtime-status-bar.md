# Runtime Status Bar — Codex 风格运行时动画

**日期**: 2026-05-23
**作用域**: `web/src` 前端 — `ChatView` 输入框上方运行时状态指示

## 背景

当前 `ChatView` 在等待 agent 响应时，用户视觉反馈仅来自消息列表里 `*` gutter 的 pulsing 动画。当列表已滚出可视区，或 agent 长时间执行远程命令时，缺少**输入框附近**的实时状态提示。

已有 `AgentStatusBar` 组件聚合**跨对话**的 agent 状态，但不覆盖当前对话内的逐步执行细节。

需要一条 Codex 风格的运行时状态条 — 单行、紧凑、动词化、带 spinner — 显示在 `ChatView` 输入框正上方，仅当当前对话正在执行时可见。

## 目标

- 给当前对话的 agent 执行过程提供始终可见的"心跳"反馈
- 文案语义化：让用户看到正在做什么（思考 / 探测 / 执行 / 等待确认），而不是抽象的 "running…"
- 多机执行时显示主机上下文
- 视觉风格对齐 Codex / Claude Code CLI（spinner 字符循环 + 动词 + 元数据）

## 非目标

- 不替换或合并 `AgentStatusBar`（左下角跨对话状态条仍保留）
- 不展示工具结果或日志输出 — 仅状态指示
- 不增加新的后端事件类型 — 复用现有 `useAgentStatus` 数据源

## 视觉规格

### 形态

单行，输入框正上方（`<input-area>` 之上），高度 ~32px，`border-top` 与 `--nav` 背景。

```
┌─────────────────────────────────────────────────────────────┐
│ ✻ Running on xian-124, sh-201, sh-202 · tail -50 …  3.1s · esc │
├─────────────────────────────────────────────────────────────┤
│ 输入运维指令...                                       [发送]    │
└─────────────────────────────────────────────────────────────┘
```

字体：mono 12px，颜色 `--text-sub`。Spinner 用 `--primary`。

### 元素布局（左 → 右）

| 区段 | 内容 | 颜色 |
|---|---|---|
| spinner | 字符循环 | `--ct-primary` |
| verb | 动词文案 | `--ct-text` |
| context | `· {arg}` 上下文（命令、工具名等） | `--ct-text-sub` |
| (spacer) | `flex:1` 推右 | — |
| elapsed | `{n}s` | `--ct-muted` |
| sep | `·` | `--ct-muted` |
| esc hint | `esc` | `--ct-muted` |

整行 `overflow:hidden`，`text-overflow:ellipsis`，`white-space:nowrap`。

### Spinner

字符循环 `['✻','✦','✶','✷','✸','✹']`，间隔 120ms。**不用** CSS `transform:rotate` — 字符化更贴近 CLI 风格。

单 `setInterval` 在组件 `onMounted` 启动，`onUnmounted` 清除。

### 动词映射

| phase / tool | verb | context |
|---|---|---|
| `thinking` | `Processing…` | — |
| `tool` + `RunCommand` | `Running on {hosts}` | `· {cmd}` |
| `tool` + Explore (`GetHosts` / `SearchDocs` / `Verify` / `GetTopology` / `invoke_skill`) | `Exploring · {ToolName}` | `· {arg}` |
| `tool` + 其他 (Act 类) | `Working · {ToolName}` | — |
| `confirm` | `Awaiting confirm` | `· {tool}` |
| `done` | (隐藏) | — |

`Explore` 工具集合保持与 `ChatMessage.vue` 一致：`EXPLORE_TOOLS = {GetHosts, SearchDocs, Verify, GetTopology}`，外加 `invoke_skill`（用户决定纳入 Explore 分类）。

### 多 host 截断（方案 B）

```ts
function formatHosts(hosts: string[]): string {
  if (hosts.length <= 3) return hosts.join(', ')
  return hosts.slice(0, 3).join(', ') + `, +${hosts.length - 3}`
}
```

示例：
- 1 host：`Running on xian-124`
- 3 hosts：`Running on xian-124, sh-201, sh-202`
- 8 hosts：`Running on xian-124, sh-201, sh-202, +5`

### 命令/参数截断

`cmd` / explore arg / query 截断到 50 字符，超出加 `…`。整行再由 CSS `text-overflow:ellipsis` 兜底。

```ts
function truncate(s: string, n = 50): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}
```

### 计时器

`setInterval` 1000ms 更新 elapsed seconds。`startedAt` 来自 `AgentStatus.startedAt`（新字段）。`done` 或卸载时清除。

### Esc 取消

`ChatView` 已有 `cancelSend` 函数。状态条本身不绑 keydown — 在 `ChatView` 根添加 `@keydown.escape="cancelSend"`（仅当 `isStreaming`）。

## 数据流

### 当前 `AgentStatus` 结构

```ts
interface AgentStatus {
  conversationId: string
  title: string
  phase: 'thinking' | 'tool' | 'confirm' | 'done'
  toolName?: string
  toolInput?: string  // JSON string
  updatedAt: number
}
```

### 扩展字段

```ts
interface AgentStatus {
  // ... 现有字段
  hosts?: string[]    // RunCommand 的目标主机列表
  startedAt?: number  // 当前 phase 开始时间（ms epoch）
}
```

`hosts` 由 `ChatView` 在调用 `updateAgentStatus` 时从 `tool_use` 事件的 `targets` / `host_name` 字段提取（与 `ChatMessage.vue:actHosts` 同一逻辑）。

`startedAt` 在每次 `phase` 切换时更新（`thinking → tool` 算新阶段）。

### Reactivity 注意

`useAgentStatus.ts:29` 已有"`thinking` 重复触发时跳过 Map 复制"优化。新增 `startedAt` 字段时**不破坏**该优化 — `startedAt` 仅在 phase 转换时更新，与 `text_delta` 触发的 thinking 重复无关。

## 组件契约

### `RuntimeStatusBar.vue`

**Props**:
```ts
{
  status: AgentStatus | null  // 当前对话的 agent status，null 时隐藏
}
```

**渲染条件**: `status != null && status.phase !== 'done'`

**内部状态**:
- `spinnerIdx`: ref<number>，120ms 自增
- `elapsedSec`: ref<number>，1000ms 自增

**Computed**:
- `verb`: 由 phase + toolName 派生
- `context`: 由 phase + toolName + toolInput + hosts 派生
- `spinnerChar`: `SPINNER[spinnerIdx % SPINNER.length]`

**生命周期**:
- `onMounted`: 启动两个 interval
- `onUnmounted`: 清除两个 interval
- `watch(() => status?.startedAt)`: 重置 `elapsedSec`

### `ChatView` 集成

```vue
<RuntimeStatusBar
  v-if="isStreaming"
  :status="currentStatus"
/>
<input-area>...</input-area>
```

`currentStatus` 来自 `useAgentStatus().statuses.value.get(activeConvId.value)`。

## 错误与边界

| 场景 | 行为 |
|---|---|
| `toolInput` JSON 解析失败 | 仅显示 verb，不显示 context（与 `formatToolDetail` 一致） |
| `hosts` 为空数组 | 显示 `Running` 而非 `Running on ` |
| `phase=done` | 立即隐藏（不等 3s timeout）— `done` timeout 已存在用于 `AgentStatusBar`，此处独立判断 |
| 切换对话 | `currentStatus` 变化触发重渲染，`elapsedSec` 通过 `startedAt` watch 重置 |
| 长 cmd 含 newline | `whitespace:nowrap` 自动单行渲染 |

## 测试

### 手动验证清单

1. 思考状态 → 显示 `✻ Processing… · 0s · esc`
2. RunCommand 单 host → `✻ Running on xian-124 · ps aux · 1s · esc`
3. RunCommand 3 host → `✻ Running on a, b, c · cmd · 2s · esc`
4. RunCommand 5 host → `✻ Running on a, b, c, +2 · cmd · 2s · esc`
5. Explore 工具 → `✻ Exploring · GetHosts · {arg} · 0s · esc`
6. invoke_skill → `✻ Exploring · invoke_skill · {name} · 0s · esc`
7. 等待确认 → `✻ Awaiting confirm · RunCommand · 5s · esc`
8. 完成后立即隐藏
9. Spinner 字符按 120ms 循环
10. Elapsed 每秒 +1
11. Esc 触发 `cancelSend`
12. 切换对话时状态条跟随当前对话

### 无回归

- `AgentStatusBar`（左下角跨对话）功能不变
- `ChatMessage.vue` gutter pulsing 动画保留
- `text_delta` 高频触发不导致额外 reactivity 开销

## 实现顺序

1. 扩展 `useAgentStatus.ts` — 添加 `hosts?` / `startedAt?` 字段，`updateAgentStatus` 在 phase 切换时设置 `startedAt`
2. `ChatView.vue` 在 `tool_use` 事件处理时提取 `hosts` 传入 `updateAgentStatus`
3. 创建 `RuntimeStatusBar.vue`
4. `ChatView.vue` 在 input-area 上方挂载 `RuntimeStatusBar`，绑定 `currentStatus`
5. 添加 `@keydown.escape` 取消（如果尚未绑定）
6. `npm run build` + Playwright 验证渲染

## 文件清单

**新增**:
- `web/src/components/RuntimeStatusBar.vue`

**修改**:
- `web/src/composables/useAgentStatus.ts` — 扩展接口
- `web/src/views/ChatView.vue` — 挂载组件、提取 hosts、绑定 esc
