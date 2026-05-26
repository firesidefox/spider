# New Conversation Welcome Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a centered welcome page when the user clicks "+" instead of immediately creating a conversation — the conversation is only created when the user sends their first message.

**Architecture:** CSS-class toggle approach. A `.welcome-mode` class on `.chat-main` hides the header/messages and centers the input. The `send()` function already lazily creates a conversation when `activeConvId` is null, so no change to send logic is needed.

**Tech Stack:** Vue 3 (script setup), Vue Router, scoped CSS

---

## File Map

| File | Change |
|------|--------|
| `web/src/views/ChatView.vue` | All changes — script, template, CSS |

---

### Task 1: Import useAuth and expose currentUser

**Files:**
- Modify: `web/src/views/ChatView.vue:19`

- [ ] **Step 1: Add import for useAuth**

In `web/src/views/ChatView.vue`, after line 19 (`import { authHeaders, getUIPrefs, setUIPrefs } from '../api/auth'`), add:

```ts
import { useAuth } from '../composables/useAuth'
```

- [ ] **Step 2: Destructure currentUser**

After the existing `const chatThemeName = ref(...)` block (around line 29), add:

```ts
const { currentUser } = useAuth()
```

- [ ] **Step 3: Verify no TypeScript errors**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors (or same errors as before — do not introduce new ones).

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): expose currentUser for welcome page greeting"
```

---

### Task 2: Add goNewPage() and rewire "+" buttons

**Files:**
- Modify: `web/src/views/ChatView.vue:581–590` (after `createNewConversation`)
- Modify: `web/src/views/ChatView.vue:1239` (sidebar "+" button)
- Modify: `web/src/views/ChatView.vue:1289` (header "+" button)

- [ ] **Step 1: Add goNewPage() after createNewConversation()**

After the closing brace of `createNewConversation()` (around line 590), insert:

```ts
function goNewPage() {
  activeConvId.value = null
  router.replace('/chat')
}
```

- [ ] **Step 2: Update sidebar "+" button (line ~1239)**

Change:
```html
<button class="sidebar-new" @click="createNewConversation()">+</button>
```
To:
```html
<button class="sidebar-new" @click="goNewPage()">+</button>
```

- [ ] **Step 3: Update header "+" button (line ~1289)**

Change:
```html
<button v-if="!sidebarOpen" class="header-new-btn" @click="createNewConversation()">+</button>
```
To:
```html
<button v-if="!sidebarOpen" class="header-new-btn" @click="goNewPage()">+</button>
```

- [ ] **Step 4: Verify no TypeScript errors**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1 | head -20
```

Expected: no new errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): add goNewPage(), wire + buttons to show welcome page"
```

---

### Task 3: Clear activeConvId on route change to /chat

**Files:**
- Modify: `web/src/views/ChatView.vue:1212–1217`

- [ ] **Step 1: Update onBeforeRouteUpdate**

Change the existing hook (lines 1212–1217):
```ts
onBeforeRouteUpdate(async (to) => {
  const newId = to.params.id as string | undefined
  if (newId && newId !== activeConvId.value) {
    await selectConversation(newId)
  }
})
```
To:
```ts
onBeforeRouteUpdate(async (to) => {
  const newId = to.params.id as string | undefined
  if (newId && newId !== activeConvId.value) {
    await selectConversation(newId)
  } else if (!newId) {
    activeConvId.value = null
  }
})
```

- [ ] **Step 2: Verify no TypeScript errors**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1 | head -20
```

Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "fix(chat): clear activeConvId when navigating to /chat with no id"
```

---

### Task 4: Update template — welcome-mode class, greeting, hide header

**Files:**
- Modify: `web/src/views/ChatView.vue` (template section)

- [ ] **Step 1: Add welcome-mode class to chat-main (line ~1286)**

Change:
```html
<div class="chat-main" @click="showExportMenu = false; showModeDropdown = false; closeConvMenu()">
```
To:
```html
<div class="chat-main" :class="{ 'welcome-mode': !activeConvId }" @click="showExportMenu = false; showModeDropdown = false; closeConvMenu()">
```

- [ ] **Step 2: Add v-if="activeConvId" to chat-header (line ~1287)**

Change:
```html
<div class="chat-header">
```
To:
```html
<div v-if="activeConvId" class="chat-header">
```

- [ ] **Step 3: Insert welcome-greeting before chat-messages (line ~1323)**

Insert this block between the closing `</div>` of `chat-header` and the opening `<div class="chat-messages"`:

```html
<div class="welcome-greeting">
  <span class="welcome-logo">✦</span>
  <span class="welcome-text">你好，{{ currentUser?.username }}</span>
</div>
```

- [ ] **Step 4: Verify no TypeScript errors**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1 | head -20
```

Expected: no new errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): add welcome-mode template — greeting and conditional header"
```

---

### Task 5: Add CSS for welcome-mode layout

**Files:**
- Modify: `web/src/views/ChatView.vue` (`<style scoped>` section, after existing `.chat-main` rule)

- [ ] **Step 1: Add welcome-mode CSS rules**

In the `<style scoped>` section, after the line:
```css
.chat-main { flex: 1; display: flex; flex-direction: column; min-width: 300px; position: relative; }
```

Add:
```css
/* Welcome mode */
.chat-main.welcome-mode { justify-content: center; align-items: center; }
.chat-main.welcome-mode .chat-messages { display: none; }
.chat-main.welcome-mode .todo-panel { display: none; }
.chat-main.welcome-mode .retry-banner { display: none; }
.chat-main.welcome-mode .chat-input { max-width: 640px; width: 100%; }
.welcome-greeting { display: none; flex-direction: column; align-items: center; gap: 16px; margin-bottom: 32px; }
.chat-main.welcome-mode .welcome-greeting { display: flex; }
.welcome-logo { font-size: 32px; color: var(--primary); }
.welcome-text { font-size: 24px; color: var(--text); font-family: 'SF Mono', monospace; }
```

- [ ] **Step 2: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat): add CSS for welcome-mode centered layout"
```

---

### Task 6: Build and verify

**Files:** none (build + browser verification)

- [ ] **Step 1: Build frontend**

```bash
cd /Users/cw/fty.ai/spider.ai && npm run build --prefix web
```

Expected: build succeeds with no errors.

- [ ] **Step 2: Start server**

```bash
go build -a -o /tmp/spider-test ./cmd/spider && /tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 3: Verify welcome page on "+" click**

Open browser at `http://localhost:8002/chat`. Log in if needed.

1. Click "+" button in sidebar → URL becomes `/chat`, greeting `你好，[username]` appears, input is centered, no header shown
2. No API call to `POST /api/v1/chat/conversations` is made (check Network tab)

- [ ] **Step 4: Verify lazy conversation creation**

1. On welcome page, type a message and press Enter
2. URL changes to `/chat/<new-id>`, normal chat layout appears, message is sent and response streams in
3. API call to `POST /api/v1/chat/conversations` happens exactly once

- [ ] **Step 5: Verify existing conversation selection**

1. Click an existing conversation in the sidebar → normal chat layout, correct messages shown, URL `/chat/<id>`
2. Click "+" → back to welcome page at `/chat`
3. Click same conversation again → normal layout restored

- [ ] **Step 6: Verify browser refresh on /chat**

1. While on welcome page, press F5/Cmd+R
2. Welcome page still shown (no last-conv fallback since we navigated away from the conv)

> Note: if there IS a `spider-last-conv` entry in localStorage from a previous session, `initView()` will restore that conversation instead. This is expected behavior per the spec — `initView()` restoration is unchanged.
