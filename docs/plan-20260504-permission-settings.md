# Permission Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Web UI configuration for permission mode, approval timeout, and custom static rules, plus session-level mode override in ChatView.

**Architecture:** Extend existing Settings API with permission fields, add CRUD endpoints for custom rules, add Classifier.Reload() for hot-update, add "智能体" tab to ProfileView, add mode badge to ChatView header.

**Tech Stack:** Go (backend API + config), Vue 3 (frontend), YAML (config persistence), SQLite (conversation permission_mode field)

---

### Task 1: Extend AgentConfig with Rules field

**Files:**
- Modify: `internal/config/config.go:22-25`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for RuleConfig parsing**

```go
func TestLoadConfigWithRules(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
data_dir: /tmp/test
agent:
  permission_mode: auto
  approval_timeout: 120
  rules:
    - pattern: "^docker\\s+rm"
      level: L3
      description: "docker remove"
    - pattern: "^ansible"
      level: L2
`
	os.WriteFile(cfgPath, []byte(content), 0644)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Agent.Rules) != 2 {
		t.Fatalf("want 2 rules, got %d", len(cfg.Agent.Rules))
	}
	if cfg.Agent.Rules[0].Pattern != "^docker\\s+rm" {
		t.Errorf("rule[0].Pattern = %q", cfg.Agent.Rules[0].Pattern)
	}
	if cfg.Agent.Rules[0].Level != "L3" {
		t.Errorf("rule[0].Level = %q", cfg.Agent.Rules[0].Level)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadConfigWithRules -v`
Expected: FAIL — `cfg.Agent.Rules` is nil (field doesn't exist yet)

- [ ] **Step 3: Add RuleConfig struct and Rules field to AgentConfig**

```go
// In internal/config/config.go

type RuleConfig struct {
	Pattern     string `yaml:"pattern" json:"pattern"`
	Level       string `yaml:"level" json:"level"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type AgentConfig struct {
	PermissionMode  string       `yaml:"permission_mode"`
	ApprovalTimeout int          `yaml:"approval_timeout"`
	Rules           []RuleConfig `yaml:"rules,omitempty" json:"rules,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestLoadConfigWithRules -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add RuleConfig and Rules field to AgentConfig"
```

---

### Task 2: Classifier hot-reload with RWMutex

**Files:**
- Modify: `internal/permission/classifier.go`
- Create: `internal/permission/classifier_reload_test.go`

- [ ] **Step 1: Write failing test for Reload**

```go
// internal/permission/classifier_reload_test.go
package permission_test

import (
	"context"
	"testing"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/permission"
)

func TestClassifier_Reload(t *testing.T) {
	c := permission.NewClassifier(nil)

	// Before reload: "docker rm" matches built-in L3 (generic rm)
	got := c.Classify(context.Background(), "docker rm abc")
	if got.Level != permission.L3Dangerous {
		t.Fatalf("before reload: got %s, want L3", got.Level)
	}

	// Reload with user rule: "docker rm" → L2
	c.Reload([]config.RuleConfig{
		{Pattern: `^docker\s+rm`, Level: "L2", Description: "docker remove"},
	})

	got = c.Classify(context.Background(), "docker rm abc")
	if got.Level != permission.L2Write {
		t.Fatalf("after reload: got %s, want L2", got.Level)
	}
	if got.Source != permission.SourceStatic {
		t.Fatalf("source = %s, want static", got.Source)
	}

	// Built-in rules still work for non-overridden commands
	got = c.Classify(context.Background(), "ls -la")
	if got.Level != permission.L1Read {
		t.Fatalf("ls after reload: got %s, want L1", got.Level)
	}
}

func TestClassifier_ReloadInvalidPattern(t *testing.T) {
	c := permission.NewClassifier(nil)
	// Invalid regex should be skipped, not crash
	c.Reload([]config.RuleConfig{
		{Pattern: `^valid`, Level: "L2"},
		{Pattern: `[invalid`, Level: "L3"},
	})
	got := c.Classify(context.Background(), "valid cmd")
	if got.Level != permission.L2Write {
		t.Fatalf("got %s, want L2", got.Level)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/permission/ -run TestClassifier_Reload -v`
Expected: FAIL — `Reload` method doesn't exist

- [ ] **Step 3: Add RWMutex and Reload to Classifier**

Modify `internal/permission/classifier.go`:

```go
import (
	"context"
	"log"
	"regexp"
	"sync"

	"github.com/spiderai/spider/internal/config"
)

type Classifier struct {
	mu    sync.RWMutex
	rules []rule
	llm   LLMClassifier
}

func NewClassifier(llm LLMClassifier) *Classifier {
	c := &Classifier{rules: buildStaticRules(), llm: llm}
	return c
}

func parseLevelString(s string) RiskLevel {
	switch s {
	case "L1":
		return L1Read
	case "L2":
		return L2Write
	case "L3":
		return L3Dangerous
	case "L4":
		return L4Destroy
	default:
		return L3Dangerous
	}
}

func (c *Classifier) Reload(userRules []config.RuleConfig) {
	var combined []rule
	for _, ur := range userRules {
		re, err := regexp.Compile(ur.Pattern)
		if err != nil {
			log.Printf("WARNING: invalid rule pattern %q: %v", ur.Pattern, err)
			continue
		}
		combined = append(combined, rule{pattern: re, level: parseLevelString(ur.Level)})
	}
	combined = append(combined, buildStaticRules()...)
	c.mu.Lock()
	c.rules = combined
	c.mu.Unlock()
}

func (c *Classifier) Classify(ctx context.Context, command string) Classification {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, r := range c.rules {
		if r.pattern.MatchString(command) {
			return Classification{Level: r.level, Source: SourceStatic, Reason: "matched: " + r.pattern.String()}
		}
	}
	if c.llm != nil {
		return c.llm.Classify(ctx, command)
	}
	return Classification{Level: L3Dangerous, Source: SourceDefault, Reason: "unknown command, defaulting to L3"}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/permission/ -v -race`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/permission/classifier.go internal/permission/classifier_reload_test.go
git commit -m "feat(permission): add Classifier.Reload() with RWMutex for hot-update"
```

---

### Task 3: Extend Settings API with permission fields

**Files:**
- Modify: `internal/api/settings.go`
- Modify: `internal/mcp/server.go` (App.PermissionMode update on save)

- [ ] **Step 1: Add permission fields to settingsResponse**

In `internal/api/settings.go`, add fields to `settingsResponse`:

```go
type settingsResponse struct {
	SSEAddr         string `json:"sse_addr"`
	SSEBaseURL      string `json:"sse_base_url"`
	SSHTimeout      int    `json:"ssh_default_timeout_seconds"`
	SSHPoolTTL      int    `json:"ssh_pool_ttl_seconds"`
	SSHMaxPool      int    `json:"ssh_max_pool_size"`
	PermissionMode  string `json:"permission_mode"`
	ApprovalTimeout int    `json:"approval_timeout"`
}
```

- [ ] **Step 2: Update buildSettingsResponse**

```go
func buildSettingsResponse(app *mcppkg.App) settingsResponse {
	return settingsResponse{
		SSEAddr:         app.Config.SSE.Addr,
		SSEBaseURL:      app.Config.SSE.BaseURL,
		SSHTimeout:      app.Config.SSH.DefaultTimeout,
		SSHPoolTTL:      app.Config.SSH.PoolTTL,
		SSHMaxPool:      app.Config.SSH.MaxPoolSize,
		PermissionMode:  app.Config.Agent.PermissionMode,
		ApprovalTimeout: app.Config.Agent.ApprovalTimeout,
	}
}
```

- [ ] **Step 3: Update updateSettings to handle permission fields**

Add to `updateSettings` in `internal/api/settings.go`, before `saveConfig`:

```go
if req.PermissionMode != "" {
	app.Config.Agent.PermissionMode = req.PermissionMode
	app.PermissionMode = permission.Mode(req.PermissionMode)
}
if req.ApprovalTimeout > 0 {
	app.Config.Agent.ApprovalTimeout = req.ApprovalTimeout
}
```

Add import for `"github.com/spiderai/spider/internal/permission"`.

- [ ] **Step 4: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add internal/api/settings.go
git commit -m "feat(api): extend settings API with permission_mode and approval_timeout"
```

---

### Task 4: Permission rules CRUD API

**Files:**
- Create: `internal/api/permission_rules.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: Create permission_rules.go with all handlers**

```go
// internal/api/permission_rules.go
package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/spiderai/spider/internal/config"
	mcppkg "github.com/spiderai/spider/internal/mcp"
)

func listRules(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	rules := app.Config.Agent.Rules
	if rules == nil {
		rules = []config.RuleConfig{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func addRule(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var rule config.RuleConfig
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	if err := validateRule(rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app.Config.Agent.Rules = append(app.Config.Agent.Rules, rule)
	if err := saveAndReload(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func updateRule(app *mcppkg.App, w http.ResponseWriter, r *http.Request, idx int) {
	if idx < 0 || idx >= len(app.Config.Agent.Rules) {
		writeError(w, http.StatusNotFound, "规则不存在")
		return
	}
	var rule config.RuleConfig
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	if err := validateRule(rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app.Config.Agent.Rules[idx] = rule
	if err := saveAndReload(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func deleteRule(app *mcppkg.App, w http.ResponseWriter, _ *http.Request, idx int) {
	if idx < 0 || idx >= len(app.Config.Agent.Rules) {
		writeError(w, http.StatusNotFound, "规则不存在")
		return
	}
	app.Config.Agent.Rules = append(
		app.Config.Agent.Rules[:idx],
		app.Config.Agent.Rules[idx+1:]...,
	)
	if err := saveAndReload(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listBuiltinRules(app *mcppkg.App, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, app.Classifier.BuiltinRules())
}

func validateRule(r config.RuleConfig) error {
	if r.Pattern == "" {
		return fmt.Errorf("pattern 不能为空")
	}
	if _, err := regexp.Compile(r.Pattern); err != nil {
		return fmt.Errorf("pattern 不是合法正则: %v", err)
	}
	switch strings.ToUpper(r.Level) {
	case "L1", "L2", "L3", "L4":
	default:
		return fmt.Errorf("level 必须为 L1/L2/L3/L4")
	}
	return nil
}

func saveAndReload(app *mcppkg.App) error {
	if err := saveConfig(app); err != nil {
		return err
	}
	app.Classifier.Reload(app.Config.Agent.Rules)
	return nil
}
```

Add missing `"fmt"` import.

- [ ] **Step 2: Add BuiltinRules() method to Classifier**

In `internal/permission/classifier.go`:

```go
type BuiltinRule struct {
	Pattern string `json:"pattern"`
	Level   string `json:"level"`
}

func (c *Classifier) BuiltinRules() []BuiltinRule {
	static := buildStaticRules()
	out := make([]BuiltinRule, len(static))
	for i, r := range static {
		out[i] = BuiltinRule{Pattern: r.pattern.String(), Level: r.level.String()}
	}
	return out
}
```

- [ ] **Step 3: Register routes in handler.go**

Add to `NewRouter` in `internal/api/handler.go`, before the auth middleware section:

```go
// Permission rules API (admin only)
mux.HandleFunc("/api/v1/permission/rules", func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listRules(app, w, r)
	case http.MethodPost:
		adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			addRule(app, w, r)
		})).ServeHTTP(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
})
mux.HandleFunc("/api/v1/permission/rules/", func(w http.ResponseWriter, r *http.Request) {
	idxStr := r.URL.Path[len("/api/v1/permission/rules/"):]
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			updateRule(app, w, r, idx)
		})).ServeHTTP(w, r)
	case http.MethodDelete:
		adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deleteRule(app, w, r, idx)
		})).ServeHTTP(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
})
mux.HandleFunc("/api/v1/permission/builtin-rules", func(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		listBuiltinRules(app, w, r)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
})
```

Add `"strconv"` import to handler.go.

- [ ] **Step 4: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add internal/api/permission_rules.go internal/api/handler.go internal/permission/classifier.go
git commit -m "feat(api): add permission rules CRUD API with validation and hot-reload"
```

---

### Task 5: Load user rules on startup

**Files:**
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: Call Classifier.Reload with config rules on startup**

In `cmd/spider/main.go`, after `app.Classifier = permission.NewClassifier(nil)`:

```go
app.Classifier = permission.NewClassifier(nil)
if len(cfg.Agent.Rules) > 0 {
	app.Classifier.Reload(cfg.Agent.Rules)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add cmd/spider/main.go
git commit -m "feat(main): load user permission rules from config on startup"
```

---

### Task 6: Session-level permission mode — backend

**Files:**
- Modify: `internal/models/conversation.go`
- Modify: `internal/store/conversation.go`
- Modify: `internal/db/schema.go`
- Modify: `internal/mcp/tools.go`

- [ ] **Step 1: Add PermissionMode field to Conversation model**

In `internal/models/conversation.go`:

```go
type Conversation struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Title          string    `json:"title"`
	PermissionMode string    `json:"permission_mode,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Add ALTER TABLE migration**

In `internal/db/schema.go`, add to `migrate()`:

```go
db.Exec("ALTER TABLE conversations ADD COLUMN permission_mode TEXT NOT NULL DEFAULT ''")
```

- [ ] **Step 3: Add UpdatePermissionMode to ConversationStore**

In `internal/store/conversation.go`:

```go
func (s *ConversationStore) UpdatePermissionMode(id, mode string) error {
	_, err := s.db.Exec(
		`UPDATE conversations SET permission_mode = ?, updated_at = ? WHERE id = ?`,
		mode, time.Now().UTC(), id,
	)
	return err
}
```

Update `GetByID` and `ListByUser` queries to include `permission_mode` in SELECT and Scan.

- [ ] **Step 4: Update checkPermission to accept session mode**

In `internal/mcp/tools.go`, change `checkPermission` signature:

```go
func checkPermission(ctx context.Context, app *App, command, hostDisplay string, sessionMode permission.Mode) (*permission.Classification, *mcpgo.CallToolResult, error) {
	c := app.Classifier.Classify(ctx, command)
	mode := app.PermissionMode
	if sessionMode != "" && sessionMode.IsValid() {
		mode = sessionMode
	}
	decision := app.Enforcer.Decide(mode, c.Level)
	// ... rest unchanged
}
```

Update all callers of `checkPermission` to pass `""` as sessionMode (existing behavior preserved).

- [ ] **Step 5: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add internal/models/conversation.go internal/store/conversation.go internal/db/schema.go internal/mcp/tools.go
git commit -m "feat: session-level permission mode — model, store, migration, checkPermission"
```

---

### Task 7: Session permission mode — Chat API

**Files:**
- Modify: `internal/api/chat.go` (or wherever chatUpdateTitle lives)

- [ ] **Step 1: Extend PATCH conversation endpoint to accept permission_mode**

The existing `chatUpdateTitle` handler at `PATCH /api/v1/chat/conversations/:id` currently only handles title. Extend it to also accept `permission_mode`:

```go
// In the PATCH handler for conversations
var req struct {
	Title          *string `json:"title"`
	PermissionMode *string `json:"permission_mode"`
}
// ... decode body
if req.PermissionMode != nil {
	mode := *req.PermissionMode
	if mode != "" {
		m := permission.Mode(mode)
		if !m.IsValid() {
			writeError(w, http.StatusBadRequest, "无效的权限模式")
			return
		}
	}
	app.ConvStore.UpdatePermissionMode(id, mode)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/api/chat.go
git commit -m "feat(api): PATCH conversation supports permission_mode update"
```

---

### Task 8: Frontend — 智能体 tab (ProfileView)

**Files:**
- Modify: `web/src/views/ProfileView.vue`

- [ ] **Step 1: Add sidebar nav item**

In the admin `<template v-if="isAdmin">` section, add before "系统设置":

```html
<div class="nav-row" :class="{ selected: activeTab === 'agent' }" @click="activeTab = 'agent'; loadAgentSettings()">
  <span class="nav-icon">🧠</span><span class="nav-label">智能体</span>
</div>
```

- [ ] **Step 2: Add agent tab template**

Add a new `<template v-if="activeTab === 'agent'">` section with:
- Permission mode dropdown (ask/auto/plan/readonly)
- Approval timeout number input
- Custom rules table with add/delete
- Builtin rules collapsible panel (readonly)

- [ ] **Step 3: Add data and methods**

Add reactive state:
```js
const agentSettings = ref({ permission_mode: 'ask', approval_timeout: 300 })
const customRules = ref([])
const builtinRules = ref([])
const showAddRule = ref(false)
const newRule = ref({ pattern: '', level: 'L3', description: '' })
```

Add methods:
```js
async function loadAgentSettings() {
  const res = await fetch('/api/v1/settings')
  const data = await res.json()
  agentSettings.value = { permission_mode: data.permission_mode, approval_timeout: data.approval_timeout }
  const rulesRes = await fetch('/api/v1/permission/rules')
  customRules.value = await rulesRes.json()
  const builtinRes = await fetch('/api/v1/permission/builtin-rules')
  builtinRules.value = await builtinRes.json()
}

async function saveAgentSettings() {
  await fetch('/api/v1/settings', { method: 'PUT', headers: {'Content-Type':'application/json'}, body: JSON.stringify(agentSettings.value) })
}

async function addRule() { /* POST /api/v1/permission/rules */ }
async function deleteRule(idx) { /* DELETE /api/v1/permission/rules/:idx */ }
```

- [ ] **Step 4: Test in browser**

Run dev server, navigate to 个人设置 → 智能体, verify:
- Mode dropdown works
- Rules table loads
- Add/delete rules works
- Builtin rules panel expands

- [ ] **Step 5: Commit**

```bash
git add web/src/views/ProfileView.vue
git commit -m "feat(ui): add 智能体 tab with permission mode and rules management"
```

---

### Task 9: Frontend — ChatView mode badge

**Files:**
- Modify: `web/src/views/ChatView.vue`

- [ ] **Step 1: Add mode badge to chat header**

In the ChatView header area, add a clickable badge showing current conversation permission mode:

```html
<div class="mode-badge" :class="currentMode" @click="showModeDropdown = !showModeDropdown">
  {{ currentMode || globalMode }}
</div>
<div v-if="showModeDropdown" class="mode-dropdown">
  <div v-for="m in ['ask','auto','plan','readonly']" :key="m"
       class="mode-option" :class="{ active: currentMode === m }"
       @click="setConversationMode(m)">
    {{ m }}
  </div>
  <div class="mode-option reset" @click="setConversationMode('')">
    使用全局默认
  </div>
</div>
```

- [ ] **Step 2: Add styles**

```css
.mode-badge {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  font-weight: 500;
}
.mode-badge.ask { background: #dbeafe; color: #1d4ed8; }
.mode-badge.auto { background: #dcfce7; color: #166534; }
.mode-badge.plan { background: #fef9c3; color: #854d0e; }
.mode-badge.readonly { background: #f3f4f6; color: #4b5563; }
```

- [ ] **Step 3: Add logic**

```js
const showModeDropdown = ref(false)
const currentMode = computed(() => currentConversation.value?.permission_mode || '')
const globalMode = ref('ask')

async function setConversationMode(mode) {
  await fetch(`/api/v1/chat/conversations/${currentConversation.value.id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ permission_mode: mode })
  })
  currentConversation.value.permission_mode = mode
  showModeDropdown.value = false
}

onMounted(async () => {
  const settings = await fetch('/api/v1/settings').then(r => r.json())
  globalMode.value = settings.permission_mode
})
```

- [ ] **Step 4: Test in browser**

Verify:
- Badge shows current mode with correct color
- Click opens dropdown
- Selecting mode calls PATCH API
- "使用全局默认" resets to empty

- [ ] **Step 5: Commit**

```bash
git add web/src/views/ChatView.vue
git commit -m "feat(ui): add permission mode badge to ChatView header"
```

---

### Task 10: Integration verification

**Files:** None (verification only)

- [ ] **Step 1: Run all backend tests**

```bash
go test ./... -race
```
Expected: ALL PASS

- [ ] **Step 2: Build binary**

```bash
go build ./cmd/spider
```
Expected: No errors

- [ ] **Step 3: Start server and test API flow**

```bash
./spider serve --data-dir /tmp/spider-test
```

Test sequence:
1. `GET /api/v1/settings` → verify permission_mode and approval_timeout present
2. `PUT /api/v1/settings` with `{"permission_mode":"auto"}` → verify saved
3. `POST /api/v1/permission/rules` → add a rule
4. `GET /api/v1/permission/rules` → verify rule listed
5. `DELETE /api/v1/permission/rules/0` → verify deleted
6. `GET /api/v1/permission/builtin-rules` → verify 96+ rules returned

- [ ] **Step 4: Test frontend**

Open browser, verify:
- 智能体 tab loads and displays settings
- Mode change persists after page refresh
- Rules CRUD works end-to-end
- ChatView badge shows and switches mode

- [ ] **Step 5: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: integration test fixes for permission settings"
```
