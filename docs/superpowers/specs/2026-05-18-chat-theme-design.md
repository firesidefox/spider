# Chat Theme System Design

**Status:** 已实现 — light/dark/system 三模式（ThemeModeSelector.vue）、localStorage 持久化、matchMedia 系统偏好

## Overview

对话框（ChatMessage.vue）专属主题系统。支持5套配色方案 × 3档布局密度，独立于全局 dark/light 主题。设置入口在 ProfileView。

## Architecture

### 新文件

- `web/src/chatTheme.ts` — 类型定义、5套主题对象、3档密度 preset、存取函数

### 修改文件

- `web/src/views/ChatView.vue` — provide chatTheme + chatDensity，监听 localStorage 变化
- `web/src/components/ChatMessage.vue` — inject chatTheme + chatDensity，CSS vars 改用 chat 专属 token
- `web/src/views/ProfileView.vue` — 新增"对话框主题"设置区

## Data Model

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
  displayName: string     // 显示名，如 'One Dark Pro'
  msgBg: string
  codeBg: string
  codeBlockBorder: string
  text: string
  textSub: string
  muted: string
  labelColor: string      // 标签/参数色（避免与 displayName 字段名冲突）
  primary: string
  accent: string
  green: string
  red: string
  yellow: string
  purple: string
}
```

## Theme Palette

| Token | dark | light | one-dark-pro | solarized-dark | nord |
|-------|------|-------|--------------|----------------|------|
| codeBg | `#12141f` | `#f5f7ff` | `#282c34` | `#073642` | `#2e3440` |
| codeBlockBorder | `#2c3150` | `#d8dce8` | `#528bff` | `#268bd2` | `#88c0d0` |
| text | `#eceef5` | `#111827` | `#abb2bf` | `#839496` | `#d8dee9` |
| textSub | `#b0b8c8` | `#374151` | `#abb2bf` | `#93a1a1` | `#e5e9f0` |
| muted | `#8892a4` | `#6b7280` | `#5c6370` | `#586e75` | `#4c566a` |
| label | `#8892a4` | `#4b5563` | `#5c6370` | `#657b83` | `#616e88` |
| primary | `#6366f1` | `#6366f1` | `#61afef` | `#268bd2` | `#88c0d0` |
| accent | `#e94560` | `#e94560` | `#e06c75` | `#dc322f` | `#bf616a` |
| green | `#4ade80` | `#15803d` | `#98c379` | `#859900` | `#a3be8c` |
| red | `#f87171` | `#dc2626` | `#e06c75` | `#dc322f` | `#bf616a` |
| yellow | `#fbbf24` | `#b45309` | `#e5c07b` | `#b58900` | `#ebcb8b` |
| purple | `#a78bfa` | `#6d28d9` | `#c678dd` | `#6c71c4` | `#b48ead` |

confirm bar 背景：各主题统一用对应语义色 + 10% opacity。

## Density Presets

| Token | compact | comfortable | spacious |
|-------|---------|-------------|----------|
| fontSize | 13px | 14px | 15px |
| fontSizeMono | 12px | 13px | 13.5px |
| lineHeight | 1.55 | 1.65 | 1.8 |
| blockPadding | 1px 0 | 3px 0 | 6px 0 |
| gutterWidth | 20px | 22px | 24px |
| subLineGap | 3px | 5px | 8px |

## State Management

- 存储：`localStorage`，key `spider-chat-theme` + `spider-chat-density`
- 默认值：`dark` + `compact`
- 传递：ChatView.vue `provide('chatTheme', ...)` + `provide('chatDensity', ...)`
- ChatMessage.vue `inject` 后通过 inline style 或 scoped CSS var 应用

## CSS Variable Mapping

ChatMessage.vue 内部用 `--ct-*` 前缀的 CSS vars（chat theme），与全局 `--*` 完全隔离：

```
--ct-text, --ct-text-sub, --ct-muted, --ct-label-color
--ct-primary, --ct-accent
--ct-green, --ct-red, --ct-yellow, --ct-purple
--ct-code-bg, --ct-code-border
--ct-font-size, --ct-font-size-mono, --ct-line-height
--ct-block-padding, --ct-gutter-width, --ct-sub-gap
```

注入方式：ChatMessage.vue 根元素上 `:style` 绑定所有 `--ct-*` 变量。

## ProfileView UI

"对话框主题"设置区，包含两行：

1. **配色方案**：5个色块卡片，显示主题名 + 代表色预览，当前选中有边框高亮
2. **布局密度**：3个按钮 `紧凑 / 舒适 / 宽松`，当前选中高亮

## Affected Elements in ChatMessage.vue

所有硬编码颜色替换为 `--ct-*` vars：

- `.prompt` ❯ → `--ct-primary`
- `.dot` * → `--ct-primary` / `--ct-red`
- `.assistant-text` → `--ct-text-sub`，标题 `--ct-text`
- inline code → `--ct-purple` on `--ct-code-bg`
- pre 块 → `--ct-code-bg`，border `--ct-code-border`
- `.tool-fn` → `--ct-primary`
- `.tool-paren`, `.hook`, `.dur` → `--ct-label-color`
- `.res-ok` → `--ct-green`，`.res-err` → `--ct-red`
- `.hk-arg` → `--ct-primary`
