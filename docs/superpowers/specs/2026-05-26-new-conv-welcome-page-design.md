# New Conversation Welcome Page

**Date:** 2026-05-26  
**Scope:** `web/src/views/ChatView.vue`

## Goal

When the user clicks "+" to start a new conversation, show a centered welcome page instead of immediately creating a conversation via API. The conversation is only created when the user sends their first message.

## State Model

| State | `activeConvId` | URL |
|-------|---------------|-----|
| Welcome page | `null` | `/chat` |
| Active conversation | `"<uuid>"` | `/chat/<uuid>` |

## Changes

### 1. Logic (`<script setup>`)

**Add `currentUser` from `useAuth()`:**
```js
const { currentUser } = useAuth()
```

**Add `goNewPage()` function** (replaces direct `createNewConversation()` call on button click):
```js
function goNewPage() {
  activeConvId.value = null
  router.replace('/chat')
}
```

The existing `createNewConversation()` function is **not changed** — it remains called only from `send()` via the existing `if (!activeConvId.value)` guard.

**Update `onBeforeRouteUpdate`** — clear `activeConvId` when navigating to `/chat` (no ID):
```js
onBeforeRouteUpdate(async (to) => {
  const newId = to.params.id as string | undefined
  if (newId && newId !== activeConvId.value) {
    await selectConversation(newId)
  } else if (!newId) {
    activeConvId.value = null
  }
})
```

**`initView()` behavior (unchanged):**
- Has `paramId` → select that conversation
- No `paramId`, has `lastConvId` in localStorage → restore last conversation
- No `paramId`, no `lastConvId` → `activeConvId` stays `null` → welcome page shown

### 2. Template

**`chat-main` gets conditional class:**
```html
<div class="chat-main" :class="{ 'welcome-mode': !activeConvId }" ...>
```

**Welcome greeting** — inserted before `.chat-messages`, hidden by default, shown via CSS in welcome-mode:
```html
<div class="welcome-greeting">
  <span class="welcome-logo">✦</span>
  <span class="welcome-text">你好，{{ currentUser?.username }}</span>
</div>
```

**`chat-header`** — add `v-if="activeConvId"` so it hides on welcome page.

**"+" buttons** (sidebar and header) — change `@click` from `createNewConversation()` to `goNewPage()`.

### 3. CSS

```css
/* Welcome mode: center content vertically */
.chat-main.welcome-mode { justify-content: center; align-items: center; }

/* Hide conversation-specific UI */
.chat-main.welcome-mode .chat-header { display: none; }
.chat-main.welcome-mode .chat-messages { display: none; }
.chat-main.welcome-mode .todo-panel { display: none; }
.chat-main.welcome-mode .retry-banner { display: none; }

/* Greeting — hidden by default, flex in welcome-mode */
.welcome-greeting { display: none; flex-direction: column; align-items: center; gap: 16px; margin-bottom: 32px; }
.chat-main.welcome-mode .welcome-greeting { display: flex; }
.welcome-logo { font-size: 32px; color: var(--primary); }
.welcome-text { font-size: 24px; color: var(--text); font-family: 'SF Mono', monospace; }

/* Input centered with max width */
.chat-main.welcome-mode .chat-input { max-width: 640px; width: 100%; }
```

## What Does NOT Change

- `createNewConversation()` internals — untouched
- `send()` function — already has `if (!activeConvId.value) await createNewConversation()`, lazy creation works as-is
- Sidebar, target panel, all other views
- `initView()` last-conv restore behavior

## Acceptance Criteria

1. Clicking "+" navigates to `/chat`, shows greeting + centered input, no API call made
2. Typing and sending from welcome page creates conversation and sends message normally
3. Selecting an existing conversation from sidebar works as before
4. After sending first message, URL changes to `/chat/<new-id>` and normal chat layout shows
5. Refreshing `/chat` (no ID, no last-conv in localStorage) shows welcome page
