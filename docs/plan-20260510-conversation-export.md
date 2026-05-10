# Conversation Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `GET /api/v1/chat/conversations/:id/export?format=md|json` endpoint and a frontend export button in the chat header.

**Architecture:** Backend handler reads conversation + messages from DB, formats as Markdown (text-only) or JSON (full structure), and returns as a file download. Frontend adds an export button to the chat header that calls the endpoint via fetch+Blob download.

**Tech Stack:** Go (net/http, no new deps), Vue 3 (Composition API), existing `authHeaders()` pattern.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/api/chat_export.go` | Create | Export handler: format Markdown or JSON, set Content-Disposition |
| `internal/api/handler.go` | Modify | Register `export` action in conversations router |
| `web/src/api/chat.ts` | Modify | Add `exportConversation(id, format)` function |
| `web/src/views/ChatView.vue` | Modify | Add export button + dropdown in chat header |

---

## Task 1: Backend — export handler

**Files:**
- Create: `internal/api/chat_export.go`

- [ ] **Step 1: Create the file with Markdown formatter**

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	mcppkg "github.com/spiderai/spider/internal/mcp"
)

var unsafeFilename = regexp.MustCompile(`[^\w\-. ]+`)

func safeFilename(title string) string {
	s := unsafeFilename.ReplaceAllString(title, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	if s == "" {
		s = "conversation"
	}
	return s
}

func buildMarkdown(title string, msgs []msgRow) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n> 导出时间：%s\n\n", title, time.Now().Format("2006-01-02 15:04"))
	for _, m := range msgs {
		if m.content == "" {
			continue
		}
		label := "User"
		if m.role == "assistant" {
			label = "Assistant"
		} else if m.role != "user" {
			continue
		}
		fmt.Fprintf(&sb, "---\n\n**%s**\n\n%s\n\n", label, m.content)
	}
	return sb.String()
}

type msgRow struct {
	role    string
	content string
}

func chatExportConversation(app *mcppkg.App, w http.ResponseWriter, r *http.Request, id string) {
	conv, err := verifyConvOwner(app, r, id)
	if err != nil {
		writeError(w, 404, "conversation not found")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "md"
	}
	if format != "md" && format != "json" {
		writeError(w, 400, "format must be md or json")
		return
	}

	msgs, err := app.MsgStore.ListByConversation(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	base := safeFilename(conv.Title)

	switch format {
	case "md":
		rows := make([]msgRow, len(msgs))
		for i, m := range msgs {
			rows[i] = msgRow{role: m.Role, content: m.Content}
		}
		body := buildMarkdown(conv.Title, rows)
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, base))
		w.WriteHeader(200)
		fmt.Fprint(w, body)

	case "json":
		payload := map[string]any{
			"conversation": conv,
			"messages":     msgs,
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, base))
		w.WriteHeader(200)
		w.Write(data)
	}
}
```

- [ ] **Step 2: Build to verify it compiles**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./internal/api/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/api/chat_export.go
git commit -m "feat(api): add chatExportConversation handler"
```

---

## Task 2: Backend — register route

**Files:**
- Modify: `internal/api/handler.go` (around line 290, the `conversations/` switch)

- [ ] **Step 1: Add `export` case to the conversations router**

In `handler.go`, find the switch block inside `/api/v1/chat/conversations/`:

```go
	case action == "cancel" && r.Method == http.MethodPost:
		chatCancel(app, w, r, id)
```

Add after it:

```go
	case action == "export" && r.Method == http.MethodGet:
		chatExportConversation(app, w, r, id)
```

- [ ] **Step 2: Build**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: no output.

- [ ] **Step 3: Smoke test**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data &
sleep 2
# get a token first (replace with real creds)
TOKEN=$(curl -s -X POST http://localhost:8002/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | jq -r .token)
# list conversations, grab first id
CONV_ID=$(curl -s http://localhost:8002/api/v1/chat/conversations \
  -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')
echo "conv: $CONV_ID"
curl -v -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8002/api/v1/chat/conversations/$CONV_ID/export?format=md"
```

Expected: response with `Content-Disposition: attachment; filename="*.md"` and Markdown body.

- [ ] **Step 4: Kill test server and commit**

```bash
pkill -f "spider serve.*8002" || true
git add internal/api/handler.go
git commit -m "feat(api): register export route for conversations"
```

---

## Task 3: Frontend API

**Files:**
- Modify: `web/src/api/chat.ts`

- [ ] **Step 1: Add exportConversation function**

Append to `web/src/api/chat.ts`:

```ts
export async function exportConversation(id: string, format: 'md' | 'json'): Promise<void> {
  const res = await fetch(`/api/v1/chat/conversations/${id}/export?format=${format}`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  const blob = await res.blob()
  const disposition = res.headers.get('Content-Disposition') || ''
  const match = disposition.match(/filename="([^"]+)"/)
  const filename = match ? match[1] : `conversation.${format}`
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/api/chat.ts
git commit -m "feat(web): add exportConversation API helper"
```

---

## Task 4: Frontend UI — export button

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Import exportConversation**

In `ChatView.vue`, find the import from `../api/chat`:

```ts
import {
  sendMessage, subscribeConversation, createConversation, listConversations,
  getConversation, deleteConversation, confirmAction, cancelConversation,
  getActiveModel, setActiveModel, updateTitle,
  type Conversation, type ChatMessage as ChatMsg, type ChatEvent,
} from '../api/chat'
```

Replace with:

```ts
import {
  sendMessage, subscribeConversation, createConversation, listConversations,
  getConversation, deleteConversation, confirmAction, cancelConversation,
  getActiveModel, setActiveModel, updateTitle, exportConversation,
  type Conversation, type ChatMessage as ChatMsg, type ChatEvent,
} from '../api/chat'
```

- [ ] **Step 2: Add reactive state and handler**

Find `const conversations = ref<Conversation[]>([])` and add after it:

```ts
const showExportMenu = ref(false)

async function doExport(format: 'md' | 'json') {
  showExportMenu.value = false
  if (!activeConvId.value) return
  await exportConversation(activeConvId.value, format)
}
```

- [ ] **Step 3: Add export button to chat header template**

In the template, find the `<div class="chat-header">` block. After the `mode-badge-wrapper` div, add:

```html
        <div v-if="activeConv" class="export-wrapper">
          <button class="export-btn" @click.stop="showExportMenu = !showExportMenu">导出</button>
          <div v-if="showExportMenu" class="export-menu">
            <div class="export-option" @click="doExport('md')">Markdown</div>
            <div class="export-option" @click="doExport('json')">JSON</div>
          </div>
        </div>
```

Also add a click-outside handler to close the menu. Find the existing `@click.stop` on `showModeDropdown` and note the pattern — add to the root `<div class="chat-main">`:

```html
<div class="chat-main" @click="showExportMenu = false; showModeDropdown = false">
```

(Find `<div class="chat-main">` and add the `@click` attribute.)

- [ ] **Step 4: Add CSS**

Find the `.cancel-btn` style block and add after it:

```css
.export-wrapper { position: relative; margin-left: auto; }
.export-btn { background: none; border: 1px solid var(--border); color: var(--text); padding: 4px 10px; border-radius: 4px; cursor: pointer; font-size: 12px; font-family: 'SF Mono', monospace; }
.export-btn:hover { background: var(--row-hover); }
.export-menu { position: absolute; right: 0; top: calc(100% + 4px); background: var(--panel); border: 1px solid var(--border); border-radius: 6px; min-width: 120px; z-index: 100; box-shadow: 0 4px 12px rgba(0,0,0,.15); }
.export-option { padding: 8px 14px; cursor: pointer; font-size: 13px; color: var(--text); font-family: 'SF Mono', monospace; }
.export-option:hover { background: var(--row-hover); }
```

- [ ] **Step 5: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build
```

Expected: build succeeds with no errors.

- [ ] **Step 6: Build Go with fresh embed**

```bash
cd /Users/cw/fty.ai/spider.ai && go build -a -o /tmp/spider-export-test ./cmd/spider
```

Expected: no errors.

- [ ] **Step 7: Verify in browser**

```bash
/tmp/spider-export-test serve --addr :8002 --data-dir ~/.spider/data &
```

Open http://localhost:8002, open a conversation, click "导出" in the header. Verify:
- Dropdown shows "Markdown" and "JSON"
- Clicking "Markdown" downloads a `.md` file with user/assistant messages
- Clicking "JSON" downloads a `.json` file with full conversation structure
- Dropdown closes when clicking outside

```bash
pkill -f "spider-export-test" || true
```

- [ ] **Step 8: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(web): add export button to chat header"
```
