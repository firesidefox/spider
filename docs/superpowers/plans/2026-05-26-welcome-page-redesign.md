# Welcome Page Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the welcome page to "Ambient Glow" style and add a border-trace animation + layout transition on send.

**Architecture:** All changes are in `ChatView.vue`. Three concerns: (1) CSS-only style upgrade, (2) SVG border-trace animation driven by `requestAnimationFrame`, (3) layout transition using a `transitionState` ref that delays `welcome-mode` class removal. No new files.

**Tech Stack:** Vue 3 (Composition API), TypeScript, vanilla CSS, SVG, `requestAnimationFrame`

---

## File Map

- Modify: `web/src/views/ChatView.vue`
  - Template: add SVG overlay inside `.chat-input`, update `.welcome-greeting` markup
  - Script: add `transitionState` ref, `chatInputRef` templateRef, `traceRaf` cancel handle, `startBorderTrace()` function, modify `send()` to trigger animation
  - CSS: update `.welcome-greeting`, `.welcome-logo`, `.welcome-text`, add `.welcome-mode` scoped input/button overrides, add animation keyframes, add transition CSS classes

---

## Task 1: Style — Ambient Glow CSS

**Files:**
- Modify: `web/src/views/ChatView.vue` (CSS section, lines ~1475–1484)

- [ ] **Step 1: Replace welcome-mode CSS**

Find and replace the existing welcome CSS block (lines ~1475–1484):

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

Replace with:

```css
/* Welcome mode */
.chat-main.welcome-mode { justify-content: center; align-items: center; }
.chat-main.welcome-mode .chat-messages { display: none; }
.chat-main.welcome-mode .todo-panel { display: none; }
.chat-main.welcome-mode .retry-banner { display: none; }
.chat-main.welcome-mode .chat-input {
  max-width: 640px; width: 100%; position: relative;
  transition: max-width 0.35s ease;
}
.chat-main.welcome-transitioning .chat-input,
.chat-main.welcome-chat .chat-input { max-width: 100%; }

.welcome-greeting {
  display: none; flex-direction: column; align-items: center; gap: 16px;
  margin-bottom: 32px; position: relative;
  transition: opacity 0.4s ease, transform 0.4s ease, filter 0.4s ease;
}
.welcome-greeting::before {
  content: '';
  position: absolute; top: -40px; left: 50%; transform: translateX(-50%);
  width: 200px; height: 200px;
  background: radial-gradient(circle, rgba(99,102,241,0.18) 0%, transparent 70%);
  pointer-events: none;
}
.chat-main.welcome-mode .welcome-greeting { display: flex; }
.chat-main.welcome-transitioning .welcome-greeting {
  opacity: 0; transform: translateY(-20px); filter: blur(4px); pointer-events: none;
}
.chat-main.welcome-chat .welcome-greeting { display: none; }

.welcome-logo {
  font-size: 32px; color: #818cf8;
  filter: drop-shadow(0 0 14px rgba(99,102,241,0.65));
  animation: logo-float 3s ease-in-out infinite;
}
@keyframes logo-float {
  0%, 100% { transform: translateY(0); }
  50%       { transform: translateY(-4px); }
}
.welcome-text { font-size: 24px; color: #c7d2fe; }
.welcome-text .welcome-username { color: #fff; }

/* Welcome-mode input overrides */
.chat-main.welcome-mode .input-wrapper {
  background: rgba(99,102,241,0.05);
  border: 1px solid rgba(99,102,241,0.28);
  border-radius: 9px;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.04), 0 4px 20px rgba(0,0,0,0.3);
  transition: border-color 0.2s;
}
.chat-main.welcome-mode .input-wrapper:focus-within {
  border-color: rgba(99,102,241,0.55);
}
.chat-main.welcome-mode .send-btn:not(.cancel-btn):not(.queue-btn) {
  background: linear-gradient(135deg, #6366f1, #818cf8);
  box-shadow: 0 4px 14px rgba(99,102,241,0.45);
  transition: transform 0.1s, box-shadow 0.2s;
}
.chat-main.welcome-mode .send-btn:not(.cancel-btn):not(.queue-btn):hover {
  box-shadow: 0 6px 20px rgba(99,102,241,0.6);
  transform: translateY(-1px);
}
.chat-main.welcome-mode .send-btn:not(.cancel-btn):not(.queue-btn):active {
  transform: scale(0.95);
}

/* Messages fade-in after welcome exits */
.chat-main.welcome-chat .chat-messages {
  animation: messages-fadein 0.7s ease 0.5s both;
}
@keyframes messages-fadein {
  from { opacity: 0; transform: translateY(12px); }
  to   { opacity: 1; transform: translateY(0); }
}
```

- [ ] **Step 2: Update `.welcome-text` template to split username span**

Find in template (~line 1334):
```html
<span class="welcome-text">你好，{{ currentUser?.username }}</span>
```
Replace with:
```html
<span class="welcome-text">你好，<span class="welcome-username">{{ currentUser?.username }}</span></span>
```

- [ ] **Step 3: Build and verify styles**

```bash
cd web && npm run build 2>&1 | tail -5
```
Expected: no errors. Then start dev server and visually confirm welcome page shows ambient glow, logo float, correct colors.

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```
Open http://localhost:8002, navigate to `/chat` with no conversation selected.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(welcome): ambient glow style — logo glow, gradient input border, float animation"
```

---

## Task 2: SVG Border-Trace Animation

**Files:**
- Modify: `web/src/views/ChatView.vue` (template + script)

- [ ] **Step 1: Add templateRef and animation state to script**

In the `<script setup>` block, find the section with other `ref()` declarations (around line 79). Add after existing refs:

```typescript
// Welcome border-trace animation
const chatInputRef = ref<HTMLElement | null>(null)
const traceRafId = ref<number | null>(null)
const traceAnimating = ref(false)
```

- [ ] **Step 2: Add SVG overlay to template**

Find in template:
```html
      <div class="chat-input">
        <div class="input-wrapper">
```
Replace with:
```html
      <div class="chat-input" ref="chatInputRef">
        <!-- Border-trace SVG overlay (welcome-mode only) -->
        <svg v-if="!activeConvId" class="trace-svg" xmlns="http://www.w3.org/2000/svg"
             style="position:absolute;inset:0;width:100%;height:100%;overflow:visible;pointer-events:none;z-index:2;">
          <defs>
            <filter id="trace-trail-glow" x="-30%" y="-30%" width="160%" height="160%">
              <feGaussianBlur in="SourceGraphic" stdDeviation="2" result="b"/>
              <feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge>
            </filter>
            <filter id="trace-head-glow" x="-80%" y="-80%" width="260%" height="260%">
              <feGaussianBlur in="SourceGraphic" stdDeviation="4" result="b"/>
              <feMerge><feMergeNode in="b"/><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge>
            </filter>
          </defs>
          <rect ref="trailRectRef" fill="none" stroke="#818cf8" stroke-width="1.5"
            stroke-dasharray="0 9999" stroke-opacity="0" filter="url(#trace-trail-glow)"/>
          <rect ref="headRectRef" fill="none" stroke="#e0e7ff" stroke-width="2.5"
            stroke-dasharray="18 9999" stroke-opacity="0" filter="url(#trace-head-glow)"/>
        </svg>
        <div class="input-wrapper">
```

- [ ] **Step 3: Add SVG rect refs to script**

Add alongside the other new refs from Step 1:
```typescript
const trailRectRef = ref<SVGRectElement | null>(null)
const headRectRef = ref<SVGRectElement | null>(null)
```

- [ ] **Step 4: Add `startBorderTrace()` function to script**

Add this function after the `send()` function:

```typescript
function startBorderTrace() {
  if (traceRafId.value !== null) {
    cancelAnimationFrame(traceRafId.value)
    traceRafId.value = null
  }

  const inputEl = chatInputRef.value
  const trail = trailRectRef.value
  const head = headRectRef.value
  if (!inputEl || !trail || !head) return

  const { width: w, height: h } = inputEl.getBoundingClientRect()
  const rx = 9
  const PERIM = 2 * (w - 2 * rx) + 2 * (h - 2 * rx) + 2 * Math.PI * rx
  // start at right-center: top edge (w-rx) + TR arc (PI/2*rx) + half right edge (h/2-rx)
  const START = (w - 2 * rx) + (Math.PI / 2 * rx) + (h / 2 - rx)

  const TRACE_DUR = 2400
  const HOLD = 600
  const FADE_DUR = 700

  function setRect(el: SVGRectElement) {
    el.setAttribute('x', '1')
    el.setAttribute('y', '1')
    el.setAttribute('width', String(w - 2))
    el.setAttribute('height', String(h - 2))
    el.setAttribute('rx', String(rx))
    el.setAttribute('ry', String(rx))
  }
  setRect(trail)
  setRect(head)

  trail.setAttribute('stroke-dasharray', `0 ${PERIM + 100}`)
  trail.setAttribute('stroke-dashoffset', String(-START))
  trail.setAttribute('stroke-opacity', '0')
  head.setAttribute('stroke-dashoffset', String(-START))
  head.setAttribute('stroke-opacity', '0')

  function easeInOut(t: number) { return t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t }

  let startTs: number | null = null
  let phase: 'draw' | 'hold' | 'fade' = 'draw'
  let holdStart = 0
  let fadeStart = 0

  function step(ts: number) {
    if (!startTs) startTs = ts
    const elapsed = ts - startTs

    if (phase === 'draw') {
      const t = Math.min(elapsed / TRACE_DUR, 1)
      const ease = easeInOut(t)
      const grown = ease * PERIM
      trail.setAttribute('stroke-dasharray', `${grown} ${PERIM + 100}`)
      trail.setAttribute('stroke-dashoffset', String(-START))
      trail.setAttribute('stroke-opacity', String(Math.min(t / 0.12, 1) * 0.65))
      head.setAttribute('stroke-dashoffset', String(-(START + grown)))
      head.setAttribute('stroke-opacity', String(Math.min(t / 0.04, 1)))
      if (t >= 1) {
        trail.setAttribute('stroke-dasharray', `${PERIM} ${PERIM + 100}`)
        trail.setAttribute('stroke-opacity', '0.65')
        head.setAttribute('stroke-opacity', '0')
        phase = 'hold'
        holdStart = ts
      }
    } else if (phase === 'hold') {
      if (ts - holdStart >= HOLD) { phase = 'fade'; fadeStart = ts }
    } else {
      const ft = Math.min((ts - fadeStart) / FADE_DUR, 1)
      trail.setAttribute('stroke-opacity', String((1 - ft) * 0.65))
      if (ft >= 1) {
        trail.setAttribute('stroke-opacity', '0')
        traceRafId.value = null
        traceAnimating.value = false
        return
      }
    }
    traceRafId.value = requestAnimationFrame(step)
  }

  traceAnimating.value = true
  traceRafId.value = requestAnimationFrame(step)
}
```

- [ ] **Step 5: Call `startBorderTrace()` from `send()`**

In `send()`, find the line that calls `createNewConversation()` when there's no active conv:
```typescript
  if (!activeConvId.value) {
    await createNewConversation()
  }
```
Add the trace call just before this block (only fires when in welcome mode, i.e. `!activeConvId.value`):
```typescript
  if (!activeConvId.value) {
    startBorderTrace()
    await createNewConversation()
  }
```

- [ ] **Step 6: Build and verify animation**

```bash
cd web && npm run build 2>&1 | tail -5
```
Start server, open welcome page, type something, click 发送. Confirm:
- Light point appears at right-center of input box
- Border lights up as point travels clockwise
- Full circle holds 0.6s then fades

- [ ] **Step 7: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(welcome): SVG border-trace animation on send"
```

---

## Task 3: Layout Transition (Welcome → Chat)

**Files:**
- Modify: `web/src/views/ChatView.vue` (script + template class binding)

- [ ] **Step 1: Add `transitionState` ref**

Add to the refs block (alongside `chatInputRef` from Task 2):
```typescript
const transitionState = ref<'welcome' | 'transitioning' | 'chat'>('welcome')
```

- [ ] **Step 2: Update `.chat-main` class binding**

Find in template:
```html
    <div class="chat-main" :class="{ 'welcome-mode': !activeConvId }" @click="...">
```
Replace with:
```html
    <div class="chat-main"
         :class="{
           'welcome-mode': transitionState === 'welcome',
           'welcome-transitioning': transitionState === 'transitioning',
           'welcome-chat': transitionState === 'chat',
         }"
         @click="showExportMenu = false; showModeDropdown = false; closeConvMenu()">
```

- [ ] **Step 3: Add `triggerLayoutTransition()` function**

Add after `startBorderTrace()`:
```typescript
function triggerLayoutTransition() {
  if (transitionState.value !== 'welcome') return
  transitionState.value = 'transitioning'
  // after welcome fadeUp completes (0.4s + small buffer), switch to chat
  setTimeout(() => {
    transitionState.value = 'chat'
  }, 420)
}
```

- [ ] **Step 4: Wire transition into `send()`**

Update the block added in Task 2 Step 5:
```typescript
  if (!activeConvId.value) {
    startBorderTrace()
    setTimeout(triggerLayoutTransition, 500)
    await createNewConversation()
  }
```

- [ ] **Step 5: Reset `transitionState` when returning to welcome page**

Find `goNewPage()` (~line 595):
```typescript
function goNewPage() {
  activeConvId.value = null
```
Add reset after:
```typescript
function goNewPage() {
  activeConvId.value = null
  transitionState.value = 'welcome'
```

Also reset when `selectConversation` is called from welcome state. Find `selectConversation` (~line 545):
```typescript
  activeConvId.value = id
```
Add after:
```typescript
  activeConvId.value = id
  if (transitionState.value !== 'chat') transitionState.value = 'chat'
```

- [ ] **Step 6: Build and verify full sequence**

```bash
cd web && npm run build 2>&1 | tail -5
```
Start server. Test full flow:
1. Open welcome page — ambient glow visible, logo floating
2. Type a message, click 发送
3. Confirm: border trace starts → 0.5s later welcome area blurs up → input expands → messages fade in slowly
4. Click `+` (new conversation) — welcome page returns cleanly

- [ ] **Step 7: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(welcome): layout transition — fadeUp exit + messages fade-in on send"
```

---

## Self-Review Notes

- Spec requires `transitionState` to drive classes — covered in Task 3.
- Spec says SVG only visible in welcome-mode — covered via `v-if="!activeConvId"` on SVG (Task 2 Step 2). Note: SVG uses `!activeConvId` not `transitionState` — this is correct because the SVG should be present during `transitioning` state too (trace is still running). Change `v-if` to `v-if="transitionState !== 'chat'"`.
- `welcome-chat` class needs to show `.chat-messages` (currently hidden by `welcome-mode`). The `welcome-chat` class does NOT apply `display:none` to `.chat-messages`, so messages are visible. Correct.
- `messages-fadein` animation applies to `.welcome-chat .chat-messages` — this fires once when class is applied. Correct.
- `ResizeObserver` mentioned in spec but not in plan — `getBoundingClientRect()` at animation start is sufficient since the input width is stable at click time. Omitted per YAGNI.
