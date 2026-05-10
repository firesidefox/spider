# Chat Message Style — Design Spec

## Mockup 参考

`docs/mockup-chat-style-c.html` — 6 种风格对比页，**C 方案**为目标样式（第三格）。
实现时以该文件中 `.style-c` 的 CSS 为准。

## 目标

优化 `ChatMessage.vue` 的 markdown 渲染和整体消息样式，采用 **C 风格**（混合紧凑）：prose 字体渲染文本，monospace 保留给代码块，紧凑间距。

---

## 样式方案：C（混合紧凑）

### 字体

| 区域 | 字体 |
|------|------|
| 助手文本（prose） | `-apple-system, 'Segoe UI', sans-serif` |
| 代码块、工具块 | `'SF Mono', 'Fira Code', monospace`（保持不变） |
| 用户输入 | `'SF Mono', monospace`（保持不变） |

### Markdown 文本（`.assistant-text`）

```
font-size: 13.5px
line-height: 1.65
color: var(--text-sub)  // #b0b8c8
```

**标题**
- `h1/h2`：`font-size: 14px`, `font-weight: 600`, `color: var(--text)`, `margin: 0 0 8px`
- `h3`：`font-size: 11px`, `font-weight: 700`, `color: var(--muted)`, `text-transform: uppercase`, `letter-spacing: 0.8px`, `margin: 10px 0 3px`

**段落**：`margin-bottom: 7px`，最后一个 `margin-bottom: 0`

**strong**：`color: var(--text)`

**inline code**
```
background: rgba(15,17,28,0.8)
color: var(--purple)   // #a78bfa
padding: 1px 5px
border-radius: 3px
font-family: monospace
font-size: 11.5px
```

**代码块（pre）**
```
background: #080a12
border: 1px solid var(--border)
border-left: 3px solid var(--border)
border-radius: 0 6px 6px 0
padding: 8px 12px
margin: 7px 0
```
内部 `code`：`color: var(--muted)`, `font-size: 11.5px`, `line-height: 1.55`

**无序列表**
```
padding-left: 1.3em
margin: 3px 0 7px
list-style: disc
```
`li`：`color: var(--muted)`, `margin-bottom: 3px`

**有序列表**
```
padding-left: 1.3em
margin: 3px 0 7px
list-style: decimal
```
`li::marker`：`color: var(--primary)`

**blockquote**
```
border-left: 2px solid var(--border)
padding-left: 10px
color: var(--muted)
margin: 7px 0
font-size: 13px
```

**table**
```
width: 100%
border-collapse: collapse
margin: 8px 0
font-size: 12.5px
```
`th`：`color: var(--primary)`, `font-size: 10px`, `text-transform: uppercase`, `letter-spacing: 0.5px`, `border-bottom: 1px solid var(--border)`, `padding: 5px 10px`
`td`：`padding: 5px 10px`, `border-bottom: 1px solid var(--border)`, `color: var(--text-sub)`

---

## 消息布局

### 用户消息

保持现有 `❯` 前缀 + 左对齐布局，无变化。

### 助手消息

保持现有 `*` 前缀 + 左对齐布局，无变化。

`.msg-assistant` 的 `color` 改为继承（由 `.assistant-text` 的 prose 样式控制）。

---

## 工具块（不变）

explore group、act tool、confirm bar 样式保持不变，仍使用 monospace。

---

## 变更范围

仅修改 `web/src/components/ChatMessage.vue` 的 `<style scoped>` 部分：

1. `.assistant-text` 字体从 monospace 改为 prose
2. 替换 `:deep()` 样式：`code`、`pre`、`ol`、`ul`、`li`、`h1`–`h3`、`blockquote`、`table`
3. 其余样式（gutter、工具块、confirm bar）不动

---

## Mockup CSS（精确参考）

直接从 `mockup-chat-style-c.html` 提取的 `.style-c` 定义，实现时以此为准：

```css
.style-c { font-family: -apple-system, 'Segoe UI', sans-serif; font-size: 13.5px; color: #b0b8c8; line-height: 1.65; }
.style-c h2 { font-size: 14px; font-weight: 600; color: #eceef5; margin: 0 0 8px; }
.style-c h3 { font-size: 11px; font-weight: 700; color: #8892a4; margin: 10px 0 3px; text-transform: uppercase; letter-spacing: 0.8px; }
.style-c p { margin-bottom: 7px; }
.style-c strong { color: #eceef5; }
.style-c code { background: rgba(15,17,28,0.8); color: #a78bfa; padding: 1px 5px; border-radius: 3px; font-family: 'SF Mono', monospace; font-size: 11.5px; }
.style-c pre { background: #080a12; border: 1px solid #1e2338; border-left: 3px solid #2c3150; border-radius: 0 5px 5px 0; padding: 8px 12px; margin: 7px 0; }
.style-c pre code { background: none; color: #8892a4; padding: 0; font-size: 11.5px; line-height: 1.55; }
.style-c ul { padding-left: 1.3em; margin: 3px 0 7px; }
.style-c li { margin-bottom: 3px; color: #8892a4; }
.style-c blockquote { border-left: 2px solid #4b5563; padding-left: 10px; color: #4b5563; margin: 7px 0; font-size: 13px; }
```

---

## 验证标准

1. `go build -a` 成功
2. 浏览器打开对话页，发一条包含 markdown 的消息，确认：
   - 标题、列表、代码块渲染正确
   - 工具块样式不受影响
   - 用户消息样式不受影响
