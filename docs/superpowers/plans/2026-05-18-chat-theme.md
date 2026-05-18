# Chat Theme System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为对话框（ChatMessage.vue）添加5套配色方案 × 3档布局密度的专属主题系统，设置入口在 ProfileView。

**Architecture:** 新建 `chatTheme.ts` 定义类型和5套主题对象；ChatView.vue 通过 `provide` 注入当前主题和密度；ChatMessage.vue 通过 `inject` 接收并在根元素上绑定 `--ct-*` CSS 变量；ProfileView 新增"对话框主题"tab 供用户切换。

**Tech Stack:** Vue 3 Composition API (provide/inject)、TypeScript、CSS custom properties、localStorage

---

## File Map

| 文件 | 操作 | 职责 |
|------|------|------|
| `web/src/chatTheme.ts` | 新建 | 类型定义、5套主题、3档密度、存取函数 |
| `web/src/views/ChatView.vue` | 修改 | provide chatTheme + chatDensity |
| `web/src/components/ChatMessage.vue` | 修改 | inject + 绑定 --ct-* CSS vars，替换硬编码颜色 |
| `web/src/views/ProfileView.vue` | 修改 | 新增 chat-theme tab，主题卡片 + 密度按钮 |

---

## Task 1: 新建 chatTheme.ts

**Files:**
- Create: `web/src/chatTheme.ts`

- [ ] **Step 1: 写类型定义和密度 presets**

```typescript
// web/src/chatTheme.ts

export type ChatThemeName = 'dark' | 'light' | 'one-dark-pro' | 'solarized-dark' | 'nord'
export type ChatDensityName = 'compact' | 'comfortable' | 'spacious'

export interface ChatDensity {
  fontSize: string
  fontSizeMono: string
  lineHeight: string
  blockPadding: string
  gutterWidth: string
  subLineGap: string
}

export interface ChatThemeTokens {
  name: ChatThemeName
  displayName: string
  msgBg: string
  codeBg: string
  codeBlockBorder: string
  text: string
  textSub: string
  muted: string
  labelColor: string
  primary: string
  accent: string
  green: string
  red: string
  yellow: string
  purple: string
}

export const densityPresets: Record<ChatDensityName, ChatDensity> = {
  compact: {
    fontSize: '13px',
    fontSizeMono: '12px',
    lineHeight: '1.55',
    blockPadding: '1px 0',
    gutterWidth: '20px',
    subLineGap: '3px',
  },
  comfortable: {
    fontSize: '14px',
    fontSizeMono: '13px',
    lineHeight: '1.65',
    blockPadding: '3px 0',
    gutterWidth: '22px',
    subLineGap: '5px',
  },
  spacious: {
    fontSize: '15px',
    fontSizeMono: '13.5px',
    lineHeight: '1.8',
    blockPadding: '6px 0',
    gutterWidth: '24px',
    subLineGap: '8px',
  },
}
```

- [ ] **Step 2: 写5套主题对象**

```typescript
export const chatThemes: Record<ChatThemeName, ChatThemeTokens> = {
  dark: {
    name: 'dark', displayName: 'Dark',
    msgBg: 'transparent', codeBg: '#12141f', codeBlockBorder: '#2c3150',
    text: '#eceef5', textSub: '#b0b8c8', muted: '#8892a4', labelColor: '#8892a4',
    primary: '#6366f1', accent: '#e94560',
    green: '#4ade80', red: '#f87171', yellow: '#fbbf24', purple: '#a78bfa',
  },
  light: {
    name: 'light', displayName: 'Light',
    msgBg: 'transparent', codeBg: '#f5f7ff', codeBlockBorder: '#d8dce8',
    text: '#111827', textSub: '#374151', muted: '#6b7280', labelColor: '#4b5563',
    primary: '#6366f1', accent: '#e94560',
    green: '#15803d', red: '#dc2626', yellow: '#b45309', purple: '#6d28d9',
  },
  'one-dark-pro': {
    name: 'one-dark-pro', displayName: 'One Dark Pro',
    msgBg: 'transparent', codeBg: '#282c34', codeBlockBorder: '#528bff',
    text: '#abb2bf', textSub: '#abb2bf', muted: '#5c6370', labelColor: '#5c6370',
    primary: '#61afef', accent: '#e06c75',
    green: '#98c379', red: '#e06c75', yellow: '#e5c07b', purple: '#c678dd',
  },
  'solarized-dark': {
    name: 'solarized-dark', displayName: 'Solarized Dark',
    msgBg: 'transparent', codeBg: '#073642', codeBlockBorder: '#268bd2',
    text: '#839496', textSub: '#93a1a1', muted: '#586e75', labelColor: '#657b83',
    primary: '#268bd2', accent: '#dc322f',
    green: '#859900', red: '#dc322f', yellow: '#b58900', purple: '#6c71c4',
  },
  nord: {
    name: 'nord', displayName: 'Nord',
    msgBg: 'transparent', codeBg: '#2e3440', codeBlockBorder: '#88c0d0',
    text: '#d8dee9', textSub: '#e5e9f0', muted: '#4c566a', labelColor: '#616e88',
    primary: '#88c0d0', accent: '#bf616a',
    green: '#a3be8c', red: '#bf616a', yellow: '#ebcb8b', purple: '#b48ead',
  },
}
```

- [ ] **Step 3: 写存取函数**

```typescript
const THEME_KEY = 'spider-chat-theme'
const DENSITY_KEY = 'spider-chat-density'

export function getSavedChatTheme(): ChatThemeName {
  return (localStorage.getItem(THEME_KEY) as ChatThemeName) || 'dark'
}

export function saveChatTheme(name: ChatThemeName) {
  localStorage.setItem(THEME_KEY, name)
}

export function getSavedChatDensity(): ChatDensityName {
  return (localStorage.getItem(DENSITY_KEY) as ChatDensityName) || 'compact'
}

export function saveChatDensity(name: ChatDensityName) {
  localStorage.setItem(DENSITY_KEY, name)
}
```

- [ ] **Step 4: 确认文件无 TypeScript 错误**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1 | head -20
```

期望：无错误输出（或只有与本次无关的已有错误）。

- [ ] **Step 5: Commit**

```bash
git add web/src/chatTheme.ts
git commit -m "feat(chat-theme): add chatTheme.ts with 5 themes and 3 density presets"
```

---

## Task 2: ChatView.vue — provide chatTheme + chatDensity

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: 在 script setup 顶部加 import**

在 ChatView.vue `<script setup>` 的 import 区末尾添加：

```typescript
import { ref, provide } from 'vue'  // ref 已存在，确认 provide 也在同一行
import {
  chatThemes, densityPresets,
  getSavedChatTheme, saveChatTheme,
  getSavedChatDensity, saveChatDensity,
  type ChatThemeName, type ChatDensityName,
} from '../chatTheme'
```

注意：ChatView.vue 已有 `import { ref, ... } from 'vue'`，只需把 `provide` 加进去，并加 chatTheme 的 import。

- [ ] **Step 2: 在 script setup 中声明响应式状态并 provide**

在 ChatView.vue script setup 中（`defineOptions` 之后，第一个 `const` 之前）添加：

```typescript
const chatThemeName = ref<ChatThemeName>(getSavedChatTheme())
const chatDensityName = ref<ChatDensityName>(getSavedChatDensity())

provide('chatTheme', () => chatThemes[chatThemeName.value])
provide('chatDensity', () => densityPresets[chatDensityName.value])
provide('setChatTheme', (name: ChatThemeName) => {
  chatThemeName.value = name
  saveChatTheme(name)
})
provide('setChatDensity', (name: ChatDensityName) => {
  chatDensityName.value = name
  saveChatDensity(name)
})
```

- [ ] **Step 3: 确认 TypeScript 无错误**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(chat-theme): ChatView provides chatTheme and chatDensity"
```

---

## Task 3: ChatMessage.vue — inject + 绑定 --ct-* CSS vars

**Files:**
- Modify: `web/src/components/ChatMessage.vue`

- [ ] **Step 1: 加 inject import 和 chatTheme 类型 import**

在 ChatMessage.vue `<script setup>` 的 import 区添加：

```typescript
import { ref, computed, inject } from 'vue'  // inject 加入已有 import
import type { ChatThemeTokens, ChatDensity } from '../chatTheme'
```

- [ ] **Step 2: inject chatTheme 和 chatDensity**

在 `const props = defineProps<...>()` 之前添加：

```typescript
const chatTheme = inject<() => ChatThemeTokens>('chatTheme')
const chatDensity = inject<() => ChatDensity>('chatDensity')

const ctVars = computed(() => {
  const t = chatTheme?.() 
  const d = chatDensity?.()
  if (!t || !d) return {}
  return {
    '--ct-text': t.text,
    '--ct-text-sub': t.textSub,
    '--ct-muted': t.muted,
    '--ct-label-color': t.labelColor,
    '--ct-primary': t.primary,
    '--ct-accent': t.accent,
    '--ct-green': t.green,
    '--ct-red': t.red,
    '--ct-yellow': t.yellow,
    '--ct-purple': t.purple,
    '--ct-code-bg': t.codeBg,
    '--ct-code-border': t.codeBlockBorder,
    '--ct-font-size': d.fontSize,
    '--ct-font-size-mono': d.fontSizeMono,
    '--ct-line-height': d.lineHeight,
    '--ct-block-padding': d.blockPadding,
    '--ct-gutter-width': d.gutterWidth,
    '--ct-sub-gap': d.subLineGap,
  }
})
```

- [ ] **Step 3: 在根元素绑定 ctVars**

将 template 中 `.chat-msg` 根元素改为：

```html
<div class="chat-msg" :class="[`role-${role}`]" :style="ctVars">
```

- [ ] **Step 4: 替换 CSS 中的硬编码颜色为 --ct-* vars**

将 `<style scoped>` 中所有 `var(--xxx)` 全局颜色替换为对应 `--ct-*`：

| 原来 | 替换为 |
|------|--------|
| `var(--text)` (在 .assistant-text 内) | `var(--ct-text)` |
| `var(--text-sub)` | `var(--ct-text-sub)` |
| `var(--label)` | `var(--ct-label-color)` |
| `var(--primary)` | `var(--ct-primary)` |
| `var(--purple)` | `var(--ct-purple)` |
| `var(--input-bg)` (在 code 背景) | `var(--ct-code-bg)` |
| `var(--panel)` (在 pre 背景) | `var(--ct-code-bg)` |
| `var(--border)` (在 pre 边框) | `var(--ct-code-border)` |
| `var(--green)` | `var(--ct-green)` |
| `var(--red)` | `var(--ct-red)` |
| `var(--yellow)` | `var(--ct-yellow)` |
| `var(--muted)` | `var(--ct-muted)` |
| `var(--row-hover)` (在 .btn-cancel) | `var(--row-hover)` (保留全局，confirm bar 不属于 chat theme) |

注意：`.msg-user` 的 `color: var(--text)` 和 `.prompt` 的 `color: var(--primary)` 也要替换为 `--ct-text` / `--ct-primary`。

- [ ] **Step 5: 替换 CSS 中的字号/行高为 --ct-* vars**

```css
.chat-msg { font-size: var(--ct-font-size-mono); }
.block-row { padding: var(--ct-block-padding); }
.gutter { width: var(--ct-gutter-width); }
.assistant-text { font-size: var(--ct-font-size); line-height: var(--ct-line-height); }
.sub-lines { gap: var(--ct-sub-gap); }  /* 若 sub-lines 用 gap */
```

- [ ] **Step 6: 确认 TypeScript 无错误**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 7: Build 并验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -10
```

期望：build 成功，无 error。

- [ ] **Step 8: Commit**

```bash
git add web/src/components/ChatMessage.vue
git commit -m "feat(chat-theme): ChatMessage injects and applies --ct-* CSS vars"
```

---

## Task 4: ProfileView.vue — 新增 chat-theme tab

**Files:**
- Modify: `web/src/views/ProfileView.vue`

- [ ] **Step 1: 加 import**

在 ProfileView.vue script setup import 区添加：

```typescript
import {
  chatThemes, densityPresets,
  getSavedChatTheme, saveChatTheme,
  getSavedChatDensity, saveChatDensity,
  type ChatThemeName, type ChatDensityName,
} from '../chatTheme'
```

- [ ] **Step 2: 声明响应式状态**

在 ProfileView.vue script setup 中添加：

```typescript
const chatThemeName = ref<ChatThemeName>(getSavedChatTheme())
const chatDensityName = ref<ChatDensityName>(getSavedChatDensity())

function selectChatTheme(name: ChatThemeName) {
  chatThemeName.value = name
  saveChatTheme(name)
}

function selectChatDensity(name: ChatDensityName) {
  chatDensityName.value = name
  saveChatDensity(name)
}
```

- [ ] **Step 3: 在侧边栏 nav 添加 chat-theme 入口**

在 ProfileView.vue 侧边栏 `<nav class="sidebar-list">` 的"个人"section 中，`logs` 行之后添加：

```html
<div class="nav-row" :class="{ selected: activeTab === 'chat-theme' }" @click="activeTab = 'chat-theme'">
  <span class="nav-icon">🎨</span><span class="nav-label">对话框主题</span>
</div>
```

- [ ] **Step 4: 在 profile-detail 区添加 chat-theme tab 内容**

在 ProfileView.vue template 的 `<div class="profile-detail">` 内，找到最后一个 `<template v-else-if>` 之后添加：

```html
<template v-else-if="activeTab === 'chat-theme'">
  <div class="section-card">
    <div class="section-title">对话框主题</div>
    <div class="field-group">
      <div class="field-label">配色方案</div>
      <div class="theme-cards">
        <div
          v-for="t in Object.values(chatThemes)"
          :key="t.name"
          class="theme-card"
          :class="{ selected: chatThemeName === t.name }"
          @click="selectChatTheme(t.name)"
        >
          <div class="theme-preview" :style="{ background: t.codeBg, borderColor: t.primary }">
            <span class="theme-preview-dot" :style="{ color: t.primary }">*</span>
            <span class="theme-preview-fn" :style="{ color: t.primary }">fn</span>
            <span class="theme-preview-text" :style="{ color: t.textSub }">text</span>
          </div>
          <div class="theme-name">{{ t.displayName }}</div>
        </div>
      </div>
    </div>
    <div class="field-group">
      <div class="field-label">布局密度</div>
      <div class="density-btns">
        <button
          v-for="d in (['compact', 'comfortable', 'spacious'] as ChatDensityName[])"
          :key="d"
          class="density-btn"
          :class="{ selected: chatDensityName === d }"
          @click="selectChatDensity(d)"
        >{{ { compact: '紧凑', comfortable: '舒适', spacious: '宽松' }[d] }}</button>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 5: 添加 CSS**

在 ProfileView.vue `<style scoped>` 末尾添加：

```css
.theme-cards { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 8px; }
.theme-card { cursor: pointer; border: 2px solid var(--border); border-radius: 8px; overflow: hidden; width: 100px; }
.theme-card.selected { border-color: var(--primary); }
.theme-preview { display: flex; align-items: center; gap: 6px; padding: 8px 10px; font-family: 'SF Mono', monospace; font-size: 11px; border-bottom: 1px solid rgba(255,255,255,0.08); }
.theme-name { font-size: 11px; color: var(--text-sub); padding: 5px 8px; text-align: center; background: var(--card-bg); }
.density-btns { display: flex; gap: 8px; margin-top: 8px; }
.density-btn { padding: 5px 16px; border: 1px solid var(--border); border-radius: 4px; background: transparent; color: var(--text-sub); cursor: pointer; font-size: 12px; }
.density-btn.selected { border-color: var(--primary); color: var(--primary); background: var(--row-hover); }
```

- [ ] **Step 6: 确认 TypeScript 无错误**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 7: Build**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -10
```

- [ ] **Step 8: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat(chat-theme): add chat-theme tab in ProfileView"
```

---

## Task 5: 端到端验证

- [ ] **Step 1: 启动开发服务器**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: 打开浏览器验证**

1. 访问 `http://localhost:8002/profile`，点击"对话框主题"tab
2. 切换5套主题，确认卡片高亮正确
3. 切换3档密度，确认按钮高亮正确
4. 访问 `http://localhost:8002/chat`，确认对话框颜色/字号随主题变化
5. 刷新页面，确认主题/密度设置持久化（localStorage）
6. 切换全局 dark/light 主题，确认对话框主题不受影响

- [ ] **Step 3: 最终 commit（如有遗漏修复）**

```bash
git add -p
git commit -m "fix(chat-theme): post-verification fixes"
```
