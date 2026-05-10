# Chat Message Style Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `ChatMessage.vue` 的 markdown 渲染从 monospace 改为 C 风格（prose 字体 + monospace 代码块），同时支持 light/dark 双模式。

**Architecture:** 仅修改 `ChatMessage.vue` 的 `<style scoped>` 部分，替换 `.assistant-text` 及其 `:deep()` 子选择器。所有颜色使用 CSS 变量，不硬编码。工具块、用户消息、gutter 样式不动。

**Tech Stack:** Vue 3 SFC, CSS variables (via `theme.ts` token system), `marked` (已有)

**Spec:** `docs/spec-20260510-1844-chat-message-style.md`
**Mockup:** `docs/mockup-chat-style-c.html`（C 方案在第三格）

---

## File Map

| 文件 | 操作 | 说明 |
|------|------|------|
| `web/src/components/ChatMessage.vue` | Modify | 替换 `.assistant-text` 及 `:deep()` 样式 |

仅改一个文件，无新增文件。

---

### Task 1: 替换 `.assistant-text` prose 样式

**Files:**
- Modify: `web/src/components/ChatMessage.vue:181-184`

当前第 181 行：
```css
.msg-assistant { color: var(--text-sub); line-height: 1.6; }
.assistant-text :deep(code) { background: var(--input-bg); padding: 2px 6px; border-radius: 3px; font-size: 12px; }
.assistant-text :deep(pre) { background: var(--input-bg); padding: 12px; border-radius: 6px; overflow-x: auto; margin: 8px 0; }
.assistant-text :deep(ol), .assistant-text :deep(ul) { padding-left: 1.5em; margin: 4px 0; }
```

- [ ] **Step 1: 确认当前样式行号**

```bash
grep -n "assistant-text\|msg-assistant" web/src/components/ChatMessage.vue
```

预期输出包含 `.msg-assistant`、`.assistant-text :deep(code)`、`.assistant-text :deep(pre)`、`.assistant-text :deep(ol)` 的行号。

- [ ] **Step 2: 替换 `.msg-assistant` 的 color 声明**

找到这一行（约 181 行）：
```css
.msg-assistant { color: var(--text-sub); line-height: 1.6; }
```

改为：
```css
.msg-assistant { line-height: 1.6; }
```

（color 由 `.assistant-text` 控制，不在 `.msg-assistant` 上设置）

- [ ] **Step 3: 替换全部 `.assistant-text` 相关样式**

找到并删除现有的 4 行 `.assistant-text :deep(...)` 规则：
```css
.assistant-text :deep(code) { background: var(--input-bg); padding: 2px 6px; border-radius: 3px; font-size: 12px; }
.assistant-text :deep(pre) { background: var(--input-bg); padding: 12px; border-radius: 6px; overflow-x: auto; margin: 8px 0; }
.assistant-text :deep(ol), .assistant-text :deep(ul) { padding-left: 1.5em; margin: 4px 0; }
```

替换为以下完整样式块：
```css
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
.assistant-text :deep(blockquote) { border-left: 2px solid var(--border); padding-left: 10px; color: var(--label); margin: 7px 0; font-size: 13px; }
.assistant-text :deep(table) { width: 100%; border-collapse: collapse; margin: 8px 0; font-size: 12.5px; }
.assistant-text :deep(th) { color: var(--primary); font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid var(--border); padding: 5px 10px; text-align: left; }
.assistant-text :deep(td) { padding: 5px 10px; border-bottom: 1px solid var(--border); color: var(--text-sub); }
```

- [ ] **Step 4: 验证无硬编码颜色残留**

```bash
grep -n "#[0-9a-fA-F]\{3,6\}\|rgba(" web/src/components/ChatMessage.vue | grep "assistant-text"
```

预期：无输出（所有颜色已换成 CSS 变量）。

- [ ] **Step 5: 构建**

```bash
cd web && npm run build 2>&1 | tail -20
```

预期：`built in` 字样，无 error。

- [ ] **Step 6: 启动服务验证暗色模式**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

打开 `http://localhost:8002`，发一条包含以下 markdown 的消息：

```
# 标题一
## 标题二
### 小标题

普通段落，包含 **加粗** 和 `inline code`。

- 无序项 A
- 无序项 B

1. 有序项一
2. 有序项二

\`\`\`bash
nginx -t && systemctl reload nginx
\`\`\`

> blockquote 引用文字

| 列A | 列B |
|-----|-----|
| 值1 | 值2 |
```

确认：
- 标题层次清晰，h3 全大写小字
- 列表 bullet 正常
- 代码块左侧有竖线，背景为 `var(--panel)`
- inline code 紫色
- blockquote 左边框
- table 有分隔线

- [ ] **Step 7: 切换 light 模式验证**

在页面右上角切换到 light 主题，确认：
- 所有文字颜色适配浅色背景（无黑底黑字、无白底白字）
- 代码块背景为浅色（`var(--panel)` 在 light 模式 = `#ffffff`）
- 工具块、用户消息样式不受影响

- [ ] **Step 8: Commit**

```bash
git add web/src/components/ChatMessage.vue
git commit -m "style: update assistant text to C style (prose font, CSS vars, light/dark)"
```
