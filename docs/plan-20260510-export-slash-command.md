# /export Slash Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `/export [md|json] [--format md|json]` slash command to the chat input that triggers conversation export.

**Architecture:** Single-file change in `ChatView.vue`. Add `parseExportFormat()` pure function and a `/export` branch in `send()`, following the existing `/model` command pattern. No backend changes needed — reuses `exportConversation()` from `web/src/api/chat.ts`.

**Tech Stack:** Vue 3, TypeScript

---

## File Map

| File | Action |
|---|---|
| `web/src/views/ChatView.vue` | Modify — add `parseExportFormat`, add `/export` branch in `send()` |

---

## Task 1: Implement /export slash command

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Add `parseExportFormat` function**

Find the line `async function handleModelCommand() {` in `ChatView.vue` (around line 418). Insert the following function **before** it:

```ts
function parseExportFormat(text: string): 'md' | 'json' | 'invalid' | 'default' {
  const rest = text.slice('/export'.length).trim()
  if (rest === '') return 'default'
  if (rest === 'md' || rest === 'json') return rest
  const m = rest.match(/^--format\s+(md|json)$/)
  if (m) return m[1] as 'md' | 'json'
  return 'invalid'
}
```

- [ ] **Step 2: Add `/export` branch in `send()`**

In `send()`, find the existing `/model` branch:

```ts
  if (text === '/model') {
    inputText.value = ''
    await handleModelCommand()
    return
  }
```

Add the `/export` branch **after** it:

```ts
  if (text.startsWith('/export')) {
    inputText.value = ''
    const fmt = parseExportFormat(text)
    if (fmt === 'invalid') {
      addSystemMessage('用法：/export [md|json] 或 /export --format [md|json]')
      return
    }
    if (!activeConvId.value) {
      addSystemMessage('没有活跃的会话')
      return
    }
    await exportConversation(activeConvId.value, fmt === 'default' ? 'md' : fmt)
    return
  }
```

- [ ] **Step 3: Build frontend to verify no TypeScript errors**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -20
```

Expected: build succeeds, no errors.

- [ ] **Step 4: Build Go with fresh embed**

```bash
cd /Users/cw/fty.ai/spider.ai && go build -a -o /tmp/spider-export-cmd-test ./cmd/spider
```

Expected: no errors.

- [ ] **Step 5: Verify in browser**

```bash
/tmp/spider-export-cmd-test serve --addr :8002 --data-dir ~/.spider/data &
sleep 2
```

Open http://localhost:8002 and verify:

1. Type `/export` + Enter → downloads `.md` file
2. Type `/export json` + Enter → downloads `.json` file
3. Type `/export md` + Enter → downloads `.md` file
4. Type `/export --format json` + Enter → downloads `.json` file
5. Type `/export xml` + Enter → system message: `用法：/export [md|json] 或 /export --format [md|json]`
6. Type `/export --format xml` + Enter → same error message

```bash
pkill -f "spider-export-cmd-test" || true
```

- [ ] **Step 6: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/ChatView.vue
git commit -m "feat(web): add /export slash command with format parameter"
```
