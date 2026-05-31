# Phase 2: SettingView Tab Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract SettingView's 14 tabs into independent components, reducing main file from 1930 lines to ~200 lines.

**Architecture:** Each tab becomes a self-contained component with its own state, API calls, and error handling. SettingView becomes a tab router that manages active tab and URL sync.

**Tech Stack:** Vue 3 Composition API, TypeScript, Vite

---

## File Structure

**New directory:**
- `web/src/components/settings/` — all tab components

**New components (14 total):**
- `PasswordSettings.vue` — password change form
- `ChatThemeSettings.vue` — theme selector (pure UI)
- `TokenSettings.vue` — token CRUD
- `SSHKeySettings.vue` — SSH key CRUD
- `LogsViewer.vue` — logs display
- `NotifyChannelSettings.vue` — notify channel CRUD
- `UsersPanel.vue` — user management (move from existing)
- `InstallPanel.vue` — install panel (move from existing)
- `SkillsPanel.vue` — skills management (move from existing)
- `PrometheusDataSourcesPanel.vue` — datasources (move from existing)
- `ProviderSettings.vue` — provider CRUD + model refresh
- `RagSettings.vue` — RAG config + model fetch + validate
- `AgentSettings.vue` — agent settings + permission rules
- `AuditLogs.vue` — audit logs (rename from AuditView)

**Modified file:**
- `web/src/views/SettingView.vue` — becomes tab router (~200 lines)

---

### Task 1: Create components/settings directory and extract PasswordSettings

**Files:**
- Create: `web/src/components/settings/PasswordSettings.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Create directory**

```bash
mkdir -p web/src/components/settings
```

- [ ] **Step 2: Extract password change section to PasswordSettings.vue**

Create `web/src/components/settings/PasswordSettings.vue`:

```vue
<template>
  <div class="password-settings">
    <h2>修改密码</h2>
    <button @click="showPwModal = true">修改密码</button>
    
    <div v-if="showPwModal" class="modal">
      <div class="modal-content">
        <h3>修改密码</h3>
        <form @submit.prevent="handleChangePassword">
          <input v-model="pw.old" type="password" placeholder="当前密码" required />
          <input v-model="pw.new1" type="password" placeholder="新密码" required />
          <input v-model="pw.new2" type="password" placeholder="确认新密码" required />
          <div v-if="pwError" class="error">{{ pwError }}</div>
          <div v-if="pwSuccess" class="success">{{ pwSuccess }}</div>
          <button type="submit" :disabled="pwLoading">{{ pwLoading ? '提交中...' : '确认' }}</button>
          <button type="button" @click="showPwModal = false">取消</button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const showPwModal = ref(false)
const pw = ref({ old: '', new1: '', new2: '' })
const pwError = ref('')
const pwSuccess = ref('')
const pwLoading = ref(false)

async function handleChangePassword() {
  pwError.value = ''
  pwSuccess.value = ''
  
  if (pw.value.new1 !== pw.value.new2) {
    pwError.value = '两次输入的新密码不一致'
    return
  }
  
  if (pw.value.new1.length < 8) {
    pwError.value = '新密码至少 8 位'
    return
  }
  
  pwLoading.value = true
  try {
    const res = await fetch('/api/v1/me/password', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        old_password: pw.value.old,
        new_password: pw.value.new1,
      }),
    })
    
    if (!res.ok) {
      const err = await res.json()
      throw new Error(err.error || '修改失败')
    }
    
    pwSuccess.value = '密码修改成功'
    pw.value = { old: '', new1: '', new2: '' }
    setTimeout(() => {
      showPwModal.value = false
      pwSuccess.value = ''
    }, 1500)
  } catch (e: any) {
    pwError.value = e.message
  } finally {
    pwLoading.value = false
  }
}
</script>

<style scoped>
.password-settings {
  padding: 20px;
}

.modal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: white;
  padding: 30px;
  border-radius: 8px;
  min-width: 400px;
}

.modal-content h3 {
  margin-top: 0;
}

.modal-content input {
  width: 100%;
  padding: 10px;
  margin-bottom: 10px;
  border: 1px solid #ddd;
  border-radius: 4px;
}

.modal-content button {
  padding: 10px 20px;
  margin-right: 10px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.modal-content button[type="submit"] {
  background: #007bff;
  color: white;
}

.modal-content button[type="button"] {
  background: #6c757d;
  color: white;
}

.error {
  color: red;
  margin-bottom: 10px;
}

.success {
  color: green;
  margin-bottom: 10px;
}
</style>
```

- [ ] **Step 3: Update SettingView to import and use PasswordSettings**

In `web/src/views/SettingView.vue`, add import:

```typescript
import PasswordSettings from '@/components/settings/PasswordSettings.vue'
```

Replace password section in template with:

```vue
<PasswordSettings v-if="activeTab === 'info'" />
```

Remove password-related state and functions from SettingView script.

- [ ] **Step 4: Build and test**

```bash
cd web && npm run build
```

Open `/setting?tab=info`, test password change.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/settings/PasswordSettings.vue web/src/views/SettingView.vue
git commit -m "refactor(settings): extract PasswordSettings component"
```

---

### Task 2: Extract ChatThemeSettings

**Files:**
- Create: `web/src/components/settings/ChatThemeSettings.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Extract chat theme section**

Create `web/src/components/settings/ChatThemeSettings.vue` with theme selector logic from SettingView (lines ~903-918).

- [ ] **Step 2: Update SettingView**

Import and use `<ChatThemeSettings v-else-if="activeTab === 'chat-theme'" />`.

- [ ] **Step 3: Build and test**

Test at `/setting?tab=chat-theme`.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/settings/ChatThemeSettings.vue web/src/views/SettingView.vue
git commit -m "refactor(settings): extract ChatThemeSettings component"
```

---

### Task 3: Extract TokenSettings

**Files:**
- Create: `web/src/components/settings/TokenSettings.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Extract token CRUD section**

Create `web/src/components/settings/TokenSettings.vue` with token list, create, delete logic from SettingView (lines ~950-1008).

- [ ] **Step 2: Update SettingView**

Import and use `<TokenSettings v-else-if="activeTab === 'tokens'" />`.

- [ ] **Step 3: Build and test**

Test at `/setting?tab=tokens`, create/delete token.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/settings/TokenSettings.vue web/src/views/SettingView.vue
git commit -m "refactor(settings): extract TokenSettings component"
```

---

### Task 4: Extract SSHKeySettings

**Files:**
- Create: `web/src/components/settings/SSHKeySettings.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Extract SSH key CRUD section**

Create `web/src/components/settings/SSHKeySettings.vue` with SSH key list, add, delete logic from SettingView (lines ~1015-1047).

- [ ] **Step 2: Update SettingView**

Import and use `<SSHKeySettings v-else-if="activeTab === 'ssh-keys'" />`.

- [ ] **Step 3: Build and test**

Test at `/setting?tab=ssh-keys`.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/settings/SSHKeySettings.vue web/src/views/SettingView.vue
git commit -m "refactor(settings): extract SSHKeySettings component"
```

---

### Task 5: Extract LogsViewer

**Files:**
- Create: `web/src/components/settings/LogsViewer.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Extract logs viewer section**

Create `web/src/components/settings/LogsViewer.vue` with logs list and expand logic from SettingView (lines ~1053-1069).

- [ ] **Step 2: Update SettingView**

Import and use `<LogsViewer v-else-if="activeTab === 'logs'" />`.

- [ ] **Step 3: Build and test**

Test at `/setting?tab=logs`.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/settings/LogsViewer.vue web/src/views/SettingView.vue
git commit -m "refactor(settings): extract LogsViewer component"
```

---

### Task 6: Extract NotifyChannelSettings

**Files:**
- Create: `web/src/components/settings/NotifyChannelSettings.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Extract notify channel CRUD section**

Create `web/src/components/settings/NotifyChannelSettings.vue` with channel list, add, toggle, delete logic from SettingView (lines ~1537-1620).

- [ ] **Step 2: Update SettingView**

Import and use `<NotifyChannelSettings v-else-if="activeTab === 'notify'" />`.

- [ ] **Step 3: Build and test**

Test at `/setting?tab=notify`, add/toggle/delete channel.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/settings/NotifyChannelSettings.vue web/src/views/SettingView.vue
git commit -m "refactor(settings): extract NotifyChannelSettings component"
```

---

### Task 7: Move existing panel components to settings/

**Files:**
- Move: `web/src/components/UsersPanel.vue` → `web/src/components/settings/UsersPanel.vue`
- Move: `web/src/components/InstallPanel.vue` → `web/src/components/settings/InstallPanel.vue`
- Move: `web/src/components/SkillsPanel.vue` → `web/src/components/settings/SkillsPanel.vue`
- Move: `web/src/components/PrometheusDataSourcesPanel.vue` → `web/src/components/settings/PrometheusDataSourcesPanel.vue`
- Move: `web/src/views/AuditView.vue` → `web/src/components/settings/AuditLogs.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Move files**

```bash
mv web/src/components/UsersPanel.vue web/src/components/settings/
mv web/src/components/InstallPanel.vue web/src/components/settings/
mv web/src/components/SkillsPanel.vue web/src/components/settings/
mv web/src/components/PrometheusDataSourcesPanel.vue web/src/components/settings/
mv web/src/views/AuditView.vue web/src/components/settings/AuditLogs.vue
```

- [ ] **Step 2: Update imports in SettingView**

Change import paths:

```typescript
import UsersPanel from '@/components/settings/UsersPanel.vue'
import InstallPanel from '@/components/settings/InstallPanel.vue'
import SkillsPanel from '@/components/settings/SkillsPanel.vue'
import PrometheusDataSourcesPanel from '@/components/settings/PrometheusDataSourcesPanel.vue'
import AuditLogs from '@/components/settings/AuditLogs.vue'
```

- [ ] **Step 3: Build and test**

```bash
cd web && npm run build
```

Test all moved tabs: users, install, skills, datasources, audit.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(settings): move existing panel components to settings/"
```

---

### Task 8: Extract ProviderSettings

**Files:**
- Create: `web/src/components/settings/ProviderSettings.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Extract provider CRUD section**

Create `web/src/components/settings/ProviderSettings.vue` with provider list, add, edit, delete, enable, model refresh logic from SettingView (lines ~1112-1253).

This is complex: includes provider CRUD, model refresh, enable/disable, settings save.

- [ ] **Step 2: Update SettingView**

Import and use `<ProviderSettings v-else-if="activeTab === 'settings'" />`.

- [ ] **Step 3: Build and test**

Test at `/setting?tab=settings`, add/edit/delete provider, refresh models.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/settings/ProviderSettings.vue web/src/views/SettingView.vue
git commit -m "refactor(settings): extract ProviderSettings component"
```

---

### Task 9: Extract RagSettings

**Files:**
- Create: `web/src/components/settings/RagSettings.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Extract RAG config section**

Create `web/src/components/settings/RagSettings.vue` with RAG config form, model fetch, validate logic from SettingView (lines ~1262-1454).

This is complex: includes model fetch, validation, save.

- [ ] **Step 2: Update SettingView**

Import and use `<RagSettings v-else-if="activeTab === 'kb'" />`.

- [ ] **Step 3: Build and test**

Test at `/setting?tab=kb`, fetch models, validate, save config.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/settings/RagSettings.vue web/src/views/SettingView.vue
git commit -m "refactor(settings): extract RagSettings component"
```

---

### Task 10: Extract AgentSettings

**Files:**
- Create: `web/src/components/settings/AgentSettings.vue`
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Extract agent settings section**

Create `web/src/components/settings/AgentSettings.vue` with agent settings form, permission rules CRUD logic from SettingView (lines ~1456-1535).

This is complex: includes permission mode, timeout, custom rules CRUD.

- [ ] **Step 2: Update SettingView**

Import and use `<AgentSettings v-else-if="activeTab === 'agent'" />`.

- [ ] **Step 3: Build and test**

Test at `/setting?tab=agent`, change mode, add/delete rules.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/settings/AgentSettings.vue web/src/views/SettingView.vue
git commit -m "refactor(settings): extract AgentSettings component"
```

---

### Task 11: Refactor SettingView to tab router

**Files:**
- Modify: `web/src/views/SettingView.vue`

- [ ] **Step 1: Remove all extracted state and functions**

After all tabs extracted, SettingView should only have:
- Tab routing logic
- `activeTab` ref
- `allowedTabs` computed
- `tabTitle` computed
- `watch(activeTab)` for URL sync
- Component imports

- [ ] **Step 2: Simplify template**

Template should only have:
- Tab buttons
- Component switches (`v-if`/`v-else-if` for each tab)

- [ ] **Step 3: Verify line count**

```bash
wc -l web/src/views/SettingView.vue
```

Expected: ~200 lines (down from 1930).

- [ ] **Step 4: Build and test**

```bash
cd web && npm run build
```

Test all 14 tabs, verify tab switching and URL sync.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/SettingView.vue
git commit -m "refactor(settings): simplify SettingView to tab router"
```

---

### Task 12: Full Verification

**Files:**
- All settings components

- [ ] **Step 1: Clean build**

```bash
cd web
rm -rf dist node_modules/.vite
npm run build
```

Expected: No TypeScript errors, build succeeds.

- [ ] **Step 2: Type check**

```bash
npx vue-tsc --noEmit
```

Expected: No type errors.

- [ ] **Step 3: Manual test all tabs**

Open `/setting` and test each tab:

1. `info` - change password
2. `tokens` - create/delete token
3. `ssh-keys` - add/delete SSH key
4. `logs` - view logs
5. `chat-theme` - change theme
6. `settings` - add/edit provider
7. `kb` - configure RAG
8. `agent` - change permission mode
9. `notify` - add/delete channel
10. `audit` - view audit logs (admin)
11. `users` - manage users (admin)
12. `install` - view install panel (admin)
13. `skills` - manage skills (admin)
14. `datasources` - manage datasources (admin)

- [ ] **Step 4: Check browser console**

Verify no errors during tab switching.

- [ ] **Step 5: Verify URL sync**

Switch tabs, verify URL query parameter updates.

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "test: verify Phase 2 SettingView extraction complete

All 14 tabs extracted to independent components.
SettingView reduced from 1930 to ~200 lines.
Manual testing passed:
- All tabs functional
- URL sync working
- No console errors

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- ✅ Extract 14 tabs into independent components
- ✅ SettingView becomes tab router (~200 lines)
- ✅ Each component has own state, API calls, error handling
- ✅ Tab switching with `v-if`
- ✅ URL query sync
- ✅ Build and manual testing

**Placeholders:** None. All tasks have concrete steps.

**Type consistency:** Component names consistent across all tasks.

**Missing:** None. All spec requirements covered.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-01-phase2-settingview-extraction.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
