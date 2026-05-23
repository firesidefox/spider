# Queued Input Design

**Date:** 2026-05-15  
**Status:** 已实现 — queuedMessages ref、streaming 时入队、done 后自动 flush（ChatView.vue）

## Overview

Agent 运行时输入框保持可用。用户提交的消息进入队列，以 dim 样式显示在消息列表末尾。Run 完成（或取消）后，所有排队消息合并为一条发出，触发新 run。

## Scope

前端改动，后端无需修改。

## Data Model

`ChatView` 新增：

```ts
const queuedMessages = ref<string[]>([])
```

FIFO 队列，存排队中的消息文本。

## Data Flow

```
用户输入 → send()
  ├─ isStreaming=false → 正常发送（现有逻辑）
  └─ isStreaming=true  → queuedMessages.push(text)
                         输入框清空
                         dim 消息列表更新

run 完成（EventDone / EventError）
  └─ queuedMessages.length > 0
       → merged = queuedMessages.join('\n\n')
       → queuedMessages = []
       → send(merged)

cancelSend()
  → 取消当前 run
  └─ queuedMessages.length > 0
       → merged = queuedMessages.join('\n\n')
       → queuedMessages = []
       → send(merged)
```

## UI

### 输入框

- Streaming 时不再 `disabled`
- Placeholder 改为 `"排队发送..."` （仅 streaming 时）
- 发送按钮：streaming 时文字改为 `"排队"`，始终可点击（支持多条入队）

### Dim 消息

- 排队消息渲染在消息列表末尾，`queuedMessages` 每条单独一行
- 样式：`opacity: 0.45`，无其他标记
- Run 完成后 dim 消息消失，由真实消息替代

### 取消按钮

- 现有"取消"按钮行为不变，取消后若有排队消息则立即合并发出

## Component Changes

| 文件 | 改动 |
|------|------|
| `web/src/views/ChatView.vue` | 新增 `queuedMessages` ref；修改 `send()`；修改 run 完成/取消逻辑；渲染 dim 消息 |

## Non-Goals

- 不支持取消单条排队消息
- 不支持排队消息编辑
- 不支持图片/附件排队（文本消息只）
