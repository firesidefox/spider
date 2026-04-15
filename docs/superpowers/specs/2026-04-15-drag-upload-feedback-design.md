# Drag Upload Feedback Design

**Date:** 2026-04-15
**Scope:** `web/src/views/InstallView.vue` — Skills 管理卡片

---

## Problem

拖拽上传功能已实现，但缺少用户提示：

1. 静止态无任何拖拽入口暗示
2. 上传中无 loading 状态
3. 上传成功/失败无反馈
4. 拖入非 .md 文件时静默忽略

---

## Design

### 状态机

```
idle → dragging → uploading → success / error → idle
                                   ↑ 2-3s 后自动回 idle
```

`status` ref 类型：

```ts
type UploadStatus =
  | { type: 'idle' }
  | { type: 'uploading'; name: string }
  | { type: 'success'; name: string }
  | { type: 'error'; msg: string }
```

`dragging` bool 独立存在，控制卡片蓝边和 drop-hint 覆盖，与 status 正交。

### 标题行布局

```
┌──────────────────────────────────────────────────────┐
│ Skills 管理   [拖拽 .md 到此处]   ⟳ 上传中…   [添加] │
└──────────────────────────────────────────────────────┘
```

| 状态 | 中间区域内容 | 颜色 |
|------|------------|------|
| idle | "拖拽 .md 文件到此处" | `--muted` |
| uploading | "⟳ 上传 `name` 中…" | `--text-sub` |
| success | "✓ `name` 已上传" | `--green`，2s 后回 idle |
| error | "✗ `msg`" | `--red`，3s 后回 idle |

### 错误场景

- 拖入非 .md 文件 → `status = { type: 'error', msg: '仅支持 .md 文件' }`，不上传
- 上传请求失败（非 2xx）→ `status = { type: 'error', msg: '上传失败，请重试' }`

---

## Scope

- 只改 `InstallView.vue`
- 不新建组件，不改后端
- 新增 `status` ref，更新 `onDrop` / `onFileChange`，新增 3 条 scoped CSS
