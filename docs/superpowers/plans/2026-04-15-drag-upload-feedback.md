# Drag Upload Feedback — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Skills 管理卡片的拖拽上传补全用户提示：静止态入口暗示、上传中 loading、成功/失败反馈、非 .md 文件错误提示。

**Architecture:** 在 `InstallView.vue` 新增 `UploadStatus` 类型和 `status` ref，标题行中间插入状态展示区，`onDrop`/`onFileChange` 更新 status，success/error 状态 2-3s 后自动回 idle。`dragging` bool 保持独立，与 status 正交。

**Tech Stack:** Vue 3, TypeScript, CSS custom properties (已有 `--green`、`--red`、`--muted`、`--text-sub`)

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `web/src/views/InstallView.vue` | 修改 | 新增 status 类型、ref、标题行状态区、CSS |

---

## Task 1: 新增 UploadStatus 类型和 status ref

**Files:**
- Modify: `web/src/views/InstallView.vue`

- [ ] **Step 1: 在 script setup 顶部新增类型定义**

找到：
```ts
interface Skill { name: string; source: string }
```

改为：
```ts
interface Skill { name: string; source: string }

type UploadStatus =
  | { type: 'idle' }
  | { type: 'uploading'; name: string }
  | { type: 'success'; name: string }
  | { type: 'error'; msg: string }
```

- [ ] **Step 2: 在 `dragging` ref 下方新增 status ref**

找到：
```ts
const dragging = ref(false)
```

改为：
```ts
const dragging = ref(false)
const status = ref<UploadStatus>({ type: 'idle' })
```

- [ ] **Step 3: 新增 setStatus 辅助函数**

在 `triggerUpload` 函数前插入：
```ts
function setStatus(s: UploadStatus) {
  status.value = s
  if (s.type === 'success') setTimeout(() => { status.value = { type: 'idle' } }, 2000)
  if (s.type === 'error') setTimeout(() => { status.value = { type: 'idle' } }, 3000)
}
```

- [ ] **Step 4: 验证编译**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1
```

Expected: 无报错。

- [ ] **Step 5: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/InstallView.vue
git commit -m "feat: add UploadStatus type and status ref"
```

---

## Task 2: 更新 onDrop 和 onFileChange 写入 status

**Files:**
- Modify: `web/src/views/InstallView.vue`

- [ ] **Step 1: 替换 onDrop**

找到整个 `onDrop` 函数：
```ts
async function onDrop(e: DragEvent) {
  dragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (!file || !file.name.endsWith('.md')) return
  const name = file.name.replace(/\.md$/i, '')
  const content = await file.text()
  await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
    body: content,
  })
  await loadSkills()
}
```

替换为：
```ts
async function onDrop(e: DragEvent) {
  dragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (!file) return
  if (!file.name.endsWith('.md')) {
    setStatus({ type: 'error', msg: '仅支持 .md 文件' })
    return
  }
  const name = file.name.replace(/\.md$/i, '')
  setStatus({ type: 'uploading', name })
  const content = await file.text()
  const res = await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
    body: content,
  })
  if (res.ok) {
    setStatus({ type: 'success', name })
    await loadSkills()
  } else {
    setStatus({ type: 'error', msg: '上传失败，请重试' })
  }
}
```

- [ ] **Step 2: 替换 onFileChange**

找到整个 `onFileChange` 函数：
```ts
async function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const name = uploadTarget.value ?? file.name.replace(/\.md$/i, '')
  const content = await file.text()
  await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
    body: content,
  })
  ;(e.target as HTMLInputElement).value = ''
  await loadSkills()
}
```

替换为：
```ts
async function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const name = uploadTarget.value ?? file.name.replace(/\.md$/i, '')
  setStatus({ type: 'uploading', name })
  const content = await file.text()
  const res = await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
    body: content,
  })
  ;(e.target as HTMLInputElement).value = ''
  if (res.ok) {
    setStatus({ type: 'success', name })
    await loadSkills()
  } else {
    setStatus({ type: 'error', msg: '上传失败，请重试' })
  }
}
```

- [ ] **Step 3: 验证编译**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npx tsc --noEmit 2>&1
```

Expected: 无报错。

- [ ] **Step 4: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/InstallView.vue
git commit -m "feat: update onDrop/onFileChange with upload status feedback"
```

---

## Task 3: 标题行插入状态展示区

**Files:**
- Modify: `web/src/views/InstallView.vue`

- [ ] **Step 1: 替换 card-header-row**

找到：
```html
      <div class="card-header-row">
          <h3>Skills 管理</h3>
          <button class="btn btn-sm btn-primary" @click="triggerUpload(null)">添加 Skill</button>
        </div>
```

替换为：
```html
      <div class="card-header-row">
          <h3>Skills 管理</h3>
          <span class="upload-status"
            :class="{
              'upload-status--uploading': status.type === 'uploading',
              'upload-status--success': status.type === 'success',
              'upload-status--error': status.type === 'error',
            }"
          >
            <template v-if="status.type === 'idle'">拖拽 .md 文件到此处</template>
            <template v-else-if="status.type === 'uploading'">⟳ 上传 {{ status.name }} 中…</template>
            <template v-else-if="status.type === 'success'">✓ {{ status.name }} 已上传</template>
            <template v-else-if="status.type === 'error'">✗ {{ status.msg }}</template>
          </span>
          <button class="btn btn-sm btn-primary" @click="triggerUpload(null)">添加 Skill</button>
        </div>
```

- [ ] **Step 2: 新增 scoped CSS**

在 `.settings-card.dragging { ... }` 后追加：
```css
.upload-status {
  flex: 1;
  text-align: center;
  font-size: 12px;
  color: var(--muted);
  padding: 0 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.upload-status--uploading { color: var(--text-sub); }
.upload-status--success   { color: var(--green); font-weight: 600; }
.upload-status--error     { color: var(--red); font-weight: 600; }
```

- [ ] **Step 3: 构建验证**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build 2>&1 | tail -5
```

Expected: `✓ built in` 无报错。

- [ ] **Step 4: Commit**

```bash
cd /Users/cw/fty.ai/spider.ai
git add web/src/views/InstallView.vue
git commit -m "feat: add upload status hint to skills card header"
```
