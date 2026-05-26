# Welcome Page Redesign — Design Spec

**Date:** 2026-05-26  
**Scope:** `web/src/views/ChatView.vue` — welcome-mode styles + send button animation

---

## Goals

1. Upgrade welcome page visual style from flat/plain to "Ambient Glow" direction.
2. Add a border-trace animation triggered on send button click.

---

## Style: Ambient Glow

### Ambient backdrop
- Radial gradient blob behind the logo: `radial-gradient(circle, rgba(99,102,241,0.18) 0%, transparent 70%)`, 200×200px, centered above the logo.
- The blob is a pseudo-element (`::before`) on `.welcome-greeting`.

### Logo (`✦`)
- Color: `#818cf8` (was `var(--primary)` = `#6366f1`).
- Glow: `filter: drop-shadow(0 0 14px rgba(99,102,241,0.65))`.
- Subtle float: `animation: logo-float 3s ease-in-out infinite` (0px → -4px → 0px translateY).

### Greeting text
- Font-size stays 24px.
- Color: `#c7d2fe` for "你好，", `#fff` for `{{ username }}`.
- Remove `font-family: 'SF Mono'` — use body font for greeting (more approachable). Username part keeps mono if desired; keep as-is for now.

### Input box (`.chat-input` wrapper in welcome-mode)
- Background: `rgba(99,102,241,0.05)`.
- Border: `1px solid rgba(99,102,241,0.28)` (was solid `var(--border)`).
- Border-radius: `9px` (was `6px`).
- Inner shadow: `box-shadow: inset 0 1px 0 rgba(255,255,255,0.04), 0 4px 20px rgba(0,0,0,0.3)`.
- On `:focus-within`: border-color → `rgba(99,102,241,0.55)`.

### Send button (`.send-btn`) — welcome-mode only
- Background: `linear-gradient(135deg, #6366f1, #818cf8)`.
- Box-shadow: `0 4px 14px rgba(99,102,241,0.45)`.
- Hover: shadow intensifies to `0 6px 20px rgba(99,102,241,0.6)` + `translateY(-1px)`.
- Active: `scale(0.95)`.

---

## Send Animation: Border Trace

Triggered when user clicks 发送 (and message is sent — not on disabled state).

### Mechanism
An SVG overlay (`position: absolute`, `pointer-events: none`, `z-index: 2`) is placed over `.chat-input` in welcome-mode. It contains two `<rect>` paths tracing the input box border.

**SVG rect geometry:** A `<svg>` is placed `position: absolute; inset: 0; overflow: visible; pointer-events: none` over `.chat-input`. The SVG has no `viewBox` — it uses `width="100%" height="100%"`. The two `<rect>` elements read actual pixel dimensions from a Vue `templateRef` via `getBoundingClientRect()` in `onMounted` + a `ResizeObserver`, storing `{ w, h }` in a reactive ref. `rx` is fixed at 9 (matches CSS `border-radius: 9px`). Perimeter is computed: `2*(w-18) + 2*(h-18) + 2*Math.PI*9`.

### Two layers

| Layer | Role | Stroke | Width | Dasharray |
|-------|------|--------|-------|-----------|
| `trail` | Growing lit border | `#818cf8`, `stroke-opacity` animated | 1.5px | grows 0→PERIM |
| `head` | Bright moving point | `#e0e7ff`, `stroke-opacity` animated | 2.5px | fixed 18px segment |

Both have `filter` for glow (feGaussianBlur). Head uses stronger blur (stdDeviation 4) than trail (2).

### Animation parameters
- **Perimeter** ≈ 1202px (computed from rect dimensions).
- **Start position:** right-center of box (~571px along clockwise path from top-left) — aligns visually with the send button.
- **Duration:** 2400ms full circle, `easeInOut`.
- **Hold:** 600ms at full glow after circle completes.
- **Fade:** 700ms linear fade-out.

### Timing sequence
1. Click → head appears instantly at start position, trail begins growing.
2. 0–2400ms: head leads, trail grows behind it (trail fades in quickly over first 15% then stays at 0.65 opacity).
3. At 2400ms: head disappears, trail snaps to full perimeter.
4. 600ms hold.
5. 700ms fade-out → animation done, SVG invisible.

### Vue implementation
- `isAnimating: boolean` ref on the component.
- `send()` sets `isAnimating = true` after calling the existing send logic (only when `!isStreaming && inputText.trim()`).
- `requestAnimationFrame` loop runs in a composable or inline in `send()`.
- Animation is idempotent — clicking again while animating cancels and restarts.
- SVG only rendered (or only visible) in `welcome-mode`.

---

## Files Changed

- `web/src/views/ChatView.vue` — CSS rules for `.welcome-greeting`, `.welcome-logo`, `.welcome-text`, `.chat-main.welcome-mode .chat-input`, `.send-btn` (welcome scoped), plus template: add SVG overlay, add animation JS.

No new files required.

---

## Out of Scope

- Non-welcome-mode send button appearance — unchanged.
- Mobile/responsive layout — unchanged.
- Animation on cancel/queue buttons — unchanged.
