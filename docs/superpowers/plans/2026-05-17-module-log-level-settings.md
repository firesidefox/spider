# Module Log Level Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-module log level configuration to the 偏好设置 → 日志 card in ProfileView.vue.

**Architecture:** Single-file change to `web/src/views/ProfileView.vue`. Add a `moduleLevels` ref and `LOG_MODULES` constant, update `loadSettings`/`saveSettings` to read/write module levels via the existing `/api/v1/log-level` endpoint, and replace the 日志 card template in both read and edit views with the new compact row layout.

**Tech Stack:** Vue 3 Composition API, TypeScript, existing `/api/v1/log-level` PUT endpoint (supports `{ module, level }` body).

---

## File Map

| File | Change |
|------|--------|
| `web/src/views/ProfileView.vue` | Add ref + helper, update load/save, replace 日志 card template (×2), add CSS |

---

## Task 1: Add `LOG_MODULES`, `moduleLevels` ref, and `levelLabel` helper

**Files:**
- Modify: `web/src/views/ProfileView.vue` (script section, near line 968)

- [ ] **Step 1: Insert constant, ref, and helper after `logLevelError` ref**

Find this block (around line 968–969):
```typescript
const logLevel = ref('info')
const logLevelError = ref('')
```

Replace with:
```typescript
const LOG_MODULES = ['main', 'scheduler', 'agent', 'mcp', 'ssh'] as const
const logLevel = ref('info')
const logLevelError = ref('')
const moduleLevels = ref<Record<string, string>>({})

function levelLabel(v: string): string {
  const map: Record<string, string> = {
    inherit: '继承 inherit',
    debug: '调试 debug',
    info: '信息 info',
    warn: '警告 warn',
    error: '错误 error',
  }
  return map[v] ?? v
}
```

- [ ] **Step 2: Build to verify no TypeScript errors**

```bash
cd web && npm run build 2>&1 | tail -20
```
Expected: build succeeds (exit 0).

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat(settings): add LOG_MODULES ref and levelLabel helper"
```

---

## Task 2: Update `loadSettings` to populate `moduleLevels`

**Files:**
- Modify: `web/src/views/ProfileView.vue` (loadSettings function, around line 999)

- [ ] **Step 1: Add module level population after global level load**

Find this block inside `loadSettings` (around line 997–1001):
```typescript
  if (lvlRes.ok) {
    const lvlData = await lvlRes.json()
    logLevel.value = lvlData.level || 'info'
  }
```

Replace with:
```typescript
  if (lvlRes.ok) {
    const lvlData = await lvlRes.json()
    logLevel.value = lvlData.level || 'info'
    const mods = lvlData.modules ?? {}
    for (const m of LOG_MODULES) {
      moduleLevels.value[m] = mods[m] ?? 'inherit'
    }
  }
```

- [ ] **Step 2: Build to verify**

```bash
cd web && npm run build 2>&1 | tail -20
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat(settings): load module log levels from API"
```

---

## Task 3: Update `saveSettings` to persist module levels

**Files:**
- Modify: `web/src/views/ProfileView.vue` (saveSettings function, around line 1020–1024)

- [ ] **Step 1: Add module level saves after global level save**

Find this block inside `saveSettings` (around line 1019–1024):
```typescript
  if (!lvlRes.ok) {
    logLevelError.value = (await lvlRes.json()).error
    return
  }
  settingsEditing.value = false
```

Replace with:
```typescript
  if (!lvlRes.ok) {
    logLevelError.value = (await lvlRes.json()).error
    return
  }
  for (const m of LOG_MODULES) {
    await fetch('/api/v1/log-level', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ module: m, level: moduleLevels.value[m] ?? 'inherit' }),
    })
  }
  settingsEditing.value = false
```

- [ ] **Step 2: Build to verify**

```bash
cd web && npm run build 2>&1 | tail -20
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat(settings): save module log levels on settings save"
```

---

## Task 4: Replace read-view 日志 card template

**Files:**
- Modify: `web/src/views/ProfileView.vue` (template, around lines 576–584)

- [ ] **Step 1: Replace the read-view 日志 card**

Find this block (around lines 576–584):
```html
            <div class="edit-card">
              <div class="edit-card-title">日志</div>
              <div class="detail-grid">
                <div class="detail-field">
                  <div class="detail-label">日志级别</div>
                  <div class="detail-value">{{ logLevel || '—' }}</div>
                </div>
              </div>
            </div>
```

Replace with:
```html
            <div class="edit-card">
              <div class="edit-card-title">日志</div>
              <div class="log-cfg-row">
                <span class="log-cfg-lbl">全局级别</span>
                <span :class="['log-cfg-badge', `log-cfg-badge--${logLevel}`]">{{ levelLabel(logLevel) }}</span>
              </div>
              <hr class="log-cfg-divider">
              <div v-for="m in LOG_MODULES" :key="m" class="log-cfg-row">
                <span class="log-cfg-mod">{{ m }}</span>
                <span :class="['log-cfg-badge', `log-cfg-badge--${moduleLevels[m] ?? 'inherit'}`]">{{ levelLabel(moduleLevels[m] ?? 'inherit') }}</span>
              </div>
            </div>
```

- [ ] **Step 2: Build to verify**

```bash
cd web && npm run build 2>&1 | tail -20
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat(settings): read-view module log level table"
```

---

## Task 5: Replace edit-view 日志 card template

**Files:**
- Modify: `web/src/views/ProfileView.vue` (template, around lines 604–616)

- [ ] **Step 1: Replace the edit-view 日志 card**

Find this block (around lines 604–616):
```html
            <div class="edit-card">
              <div class="edit-card-title">日志</div>
              <div class="form-row">
                <label>日志级别</label>
                <select v-model="logLevel" class="input" style="max-width:160px">
                  <option value="debug">debug</option>
                  <option value="info">info</option>
                  <option value="warn">warn</option>
                  <option value="error">error</option>
                </select>
              </div>
              <div v-if="logLevelError" class="err" style="margin-top:4px;font-size:12px">{{ logLevelError }}</div>
            </div>
```

Replace with:
```html
            <div class="edit-card">
              <div class="edit-card-title">日志</div>
              <div class="log-cfg-row">
                <span class="log-cfg-lbl">全局级别</span>
                <select v-model="logLevel" class="input log-cfg-select">
                  <option value="debug">调试 debug</option>
                  <option value="info">信息 info</option>
                  <option value="warn">警告 warn</option>
                  <option value="error">错误 error</option>
                </select>
              </div>
              <div v-if="logLevelError" class="err" style="margin-top:4px;font-size:12px">{{ logLevelError }}</div>
              <hr class="log-cfg-divider">
              <div v-for="m in LOG_MODULES" :key="m" class="log-cfg-row">
                <span class="log-cfg-mod">{{ m }}</span>
                <select v-model="moduleLevels[m]" class="input log-cfg-select">
                  <option value="inherit">继承 inherit</option>
                  <option value="debug">调试 debug</option>
                  <option value="info">信息 info</option>
                  <option value="warn">警告 warn</option>
                  <option value="error">错误 error</option>
                </select>
              </div>
            </div>
```

- [ ] **Step 2: Build to verify**

```bash
cd web && npm run build 2>&1 | tail -20
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat(settings): edit-view module log level dropdowns"
```

---

## Task 6: Add CSS for log-cfg classes

**Files:**
- Modify: `web/src/views/ProfileView.vue` (style section, after `.log-expand` block around line 1620)

- [ ] **Step 1: Add CSS after the `.log-expand` block**

Find this line in the style section (around line 1620):
```css
.log-expand td { padding: 0 !important; }
```

Insert after it:
```css
.log-cfg-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 0;
  border-bottom: 1px solid var(--border);
}
.log-cfg-row:last-child { border-bottom: none; }
.log-cfg-lbl { font-size: 12px; color: var(--muted); }
.log-cfg-mod { font-size: 12px; font-family: 'SF Mono', Consolas, monospace; color: var(--text); }
.log-cfg-divider { border: none; border-top: 1px solid var(--border); margin: 8px 0 4px; }
.log-cfg-select { width: 140px; }
.log-cfg-badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid transparent;
}
.log-cfg-badge--inherit { background: rgba(124,133,162,0.1); color: var(--muted); border-color: rgba(124,133,162,0.2); }
.log-cfg-badge--debug   { background: rgba(74,222,128,0.1);  color: var(--green); border-color: rgba(74,222,128,0.25); }
.log-cfg-badge--info    { background: rgba(99,102,241,0.1);  color: var(--primary); border-color: rgba(99,102,241,0.25); }
.log-cfg-badge--warn    { background: rgba(234,179,8,0.1);   color: var(--yellow); border-color: rgba(234,179,8,0.25); }
.log-cfg-badge--error   { background: rgba(248,113,113,0.1); color: var(--red);  border-color: rgba(248,113,113,0.25); }
```

- [ ] **Step 2: Build to verify**

```bash
cd web && npm run build 2>&1 | tail -20
```
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat(settings): CSS for module log level display"
```

---

## Task 7: End-to-end browser verification

- [ ] **Step 1: Start dev server with production data**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: Open browser and navigate to 偏好设置**

Navigate to `http://localhost:8002`, log in, open 系统设置 → 偏好设置.

- [ ] **Step 3: Verify read view**

Check:
- 日志 card shows 全局级别 badge with correct color and 「中文 英文」 label
- 5 module rows visible (main, scheduler, agent, mcp, ssh)
- Each row shows correct level badge

- [ ] **Step 4: Verify edit view**

Click 编辑:
- 全局级别 shows dropdown with 4 options (调试 debug / 信息 info / 警告 warn / 错误 error)
- Each module row shows dropdown with 5 options (继承 inherit + 4 levels)
- Current values pre-selected correctly

- [ ] **Step 5: Verify save round-trip**

1. Change `scheduler` to `调试 debug`
2. Click 保存
3. Reload page, re-open 偏好设置
4. Verify `scheduler` still shows `调试 debug`

- [ ] **Step 6: Final commit if any fixes needed**

```bash
git add web/src/views/ProfileView.vue
git commit -m "fix(settings): <describe fix>"
```
