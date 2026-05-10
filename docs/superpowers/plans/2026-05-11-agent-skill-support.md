# Agent Skill Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let spider agent load and execute user-defined skills on demand, injecting skill content into the conversation when the model calls `invoke_skill`.

**Architecture:** A new `SkillManager` handles skill scanning, frontmatter parsing, hash tracking, and budget-aware list rendering. The agent injects the skill list into turn-1 user messages and re-injects on change. A new `InvokeSkillTool` returns a short `tool_result` placeholder plus a `newMessages` isMeta user message containing the full skill body.

**Tech Stack:** Go, `gopkg.in/yaml.v3` (already in go.mod), existing `internal/agent` and `internal/api` packages, Vue 3 + TypeScript frontend.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/agent/skill_manager.go` | Create | Scan skills dir, parse frontmatter, hash, budget降级, list rendering |
| `internal/agent/skill_manager_test.go` | Create | Unit tests for SkillManager |
| `internal/agent/tools_skill.go` | Create | `InvokeSkillTool` definition and Execute |
| `internal/agent/tools_skill_test.go` | Create | Unit tests for InvokeSkillTool |
| `internal/agent/agent.go` | Modify | Inject skill list into user messages; handle `newMessages` from tool result |
| `internal/agent/factory.go` | Modify | Wire SkillManager into Factory and NewAgent |
| `internal/api/skills.go` | Modify | Add frontmatter validation on upload; add status field to list response |
| `web/src/views/SkillsPanel.vue` | Modify | Health status column, upload hint text, frontend validation |

---

## Task 1: SkillManager — core struct + frontmatter parsing

**Files:**
- Create: `internal/agent/skill_manager.go`
- Create: `internal/agent/skill_manager_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/agent/skill_manager_test.go
package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillFrontmatter_Valid(t *testing.T) {
	content := "---\ndescription: Use when deploying the app.\n---\n\n# Body"
	meta, body, err := parseSkillFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Description != "Use when deploying the app." {
		t.Errorf("got description %q", meta.Description)
	}
	if body != "\n# Body" {
		t.Errorf("got body %q", body)
	}
}

func TestParseSkillFrontmatter_MissingDescription(t *testing.T) {
	content := "---\n---\n\n# Body"
	_, _, err := parseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestParseSkillFrontmatter_DescriptionTooLong(t *testing.T) {
	desc := string(make([]byte, 251))
	for i := range desc {
		desc = desc[:i] + "a" + desc[i+1:]
	}
	content := "---\ndescription: " + desc + "\n---\n\n# Body"
	_, _, err := parseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for description > 250 chars")
	}
}

func TestParseSkillFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Just a body"
	_, _, err := parseSkillFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestSkillManager_LoadSkills(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "deploy", "---\ndescription: Use when deploying.\n---\n# Deploy")
	writeSkill(t, dir, "backup", "---\ndescription: Use when backing up.\n---\n# Backup")

	sm := NewSkillManager(dir)
	skills, err := sm.LoadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func writeSkill(t *testing.T, base, name, content string) {
	t.Helper()
	dir := filepath.Join(base, name)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/agent/ -run "TestParseSkill|TestSkillManager" -v 2>&1 | head -30
```
Expected: compile error or FAIL — `parseSkillFrontmatter` and `NewSkillManager` not defined.

- [ ] **Step 3: Implement SkillManager**

```go
// internal/agent/skill_manager.go
package agent

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxDescriptionChars = 250
const skillListBudgetBytes = 8 * 1024

type skillFrontmatter struct {
	Description string `yaml:"description"`
}

type SkillEntry struct {
	Name        string
	Description string
	Status      string // "ok" | "error"
	Error       string
	bodyPath    string
}

type SkillManager struct {
	dir         string
	lastHash    string
	lastEntries []SkillEntry
}

func NewSkillManager(dir string) *SkillManager {
	return &SkillManager{dir: dir}
}
```

- [ ] **Step 4: Add parseSkillFrontmatter**

```go
// append to internal/agent/skill_manager.go
func parseSkillFrontmatter(content string) (skillFrontmatter, string, error) {
	if !strings.HasPrefix(content, "---") {
		return skillFrontmatter{}, "", fmt.Errorf("missing frontmatter: file must start with ---")
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return skillFrontmatter{}, "", fmt.Errorf("malformed frontmatter: missing closing ---")
	}
	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return skillFrontmatter{}, "", fmt.Errorf("frontmatter parse error: %w", err)
	}
	if meta.Description == "" {
		return skillFrontmatter{}, "", fmt.Errorf("description is required")
	}
	if len(meta.Description) > maxDescriptionChars {
		return skillFrontmatter{}, "", fmt.Errorf("description exceeds %d characters (%d)", maxDescriptionChars, len(meta.Description))
	}
	return meta, parts[2], nil
}
```

- [ ] **Step 5: Add LoadSkills**

```go
// append to internal/agent/skill_manager.go
func (sm *SkillManager) LoadSkills() ([]SkillEntry, error) {
	var entries []SkillEntry
	err := filepath.WalkDir(sm.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		dir := filepath.Dir(path)
		rel, _ := filepath.Rel(sm.dir, dir)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			entries = append(entries, SkillEntry{Name: rel, Status: "error", Error: readErr.Error()})
			return nil
		}
		meta, _, parseErr := parseSkillFrontmatter(string(data))
		if parseErr != nil {
			entries = append(entries, SkillEntry{Name: rel, Status: "error", Error: parseErr.Error()})
			return nil
		}
		entries = append(entries, SkillEntry{
			Name: rel, Description: meta.Description,
			Status: "ok", bodyPath: path,
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/agent/ -run "TestParseSkill|TestSkillManager" -v
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/agent/skill_manager.go internal/agent/skill_manager_test.go
git commit -m "feat: add SkillManager with frontmatter parsing"
```

---

## Task 2: SkillManager — hash tracking + skill list rendering

**Files:**
- Modify: `internal/agent/skill_manager.go`
- Modify: `internal/agent/skill_manager_test.go`

- [ ] **Step 1: Write failing tests**

```go
// append to internal/agent/skill_manager_test.go

func TestSkillManager_HashChangesOnNewSkill(t *testing.T) {
	dir := t.TempDir()
	sm := NewSkillManager(dir)

	hash1, _ := sm.ComputeHash()
	writeSkill(t, dir, "deploy", "---\ndescription: Use when deploying.\n---\n# Deploy")
	hash2, _ := sm.ComputeHash()

	if hash1 == hash2 {
		t.Error("hash should change after adding a skill")
	}
}

func TestSkillManager_RenderList_Budget(t *testing.T) {
	dir := t.TempDir()
	// Add 40 skills to exceed 8KB budget
	for i := 0; i < 40; i++ {
		desc := fmt.Sprintf("Use when doing operation number %d on the system.", i)
		writeSkill(t, dir, fmt.Sprintf("skill%02d", i), fmt.Sprintf("---\ndescription: %s\n---\n# Body", desc))
	}
	sm := NewSkillManager(dir)
	entries, _ := sm.LoadSkills()
	list := sm.RenderList(entries)
	if len(list) > skillListBudgetBytes {
		t.Errorf("rendered list %d bytes exceeds budget %d", len(list), skillListBudgetBytes)
	}
}

func TestSkillManager_RenderList_Empty(t *testing.T) {
	dir := t.TempDir()
	sm := NewSkillManager(dir)
	entries, _ := sm.LoadSkills()
	list := sm.RenderList(entries)
	if list != "" {
		t.Errorf("expected empty list for no skills, got %q", list)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/agent/ -run "TestSkillManager_Hash|TestSkillManager_Render" -v 2>&1 | head -20
```
Expected: compile error — `ComputeHash` and `RenderList` not defined.

- [ ] **Step 3: Implement ComputeHash and RenderList**

```go
// append to internal/agent/skill_manager.go

// ComputeHash returns a hash of all skill file mtimes.
// If any mtime changes, callers should reload and re-render.
func (sm *SkillManager) ComputeHash() (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(sm.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		fmt.Fprintf(h, "%s:%d\n", path, info.ModTime().UnixNano())
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// RenderList renders the skill list block for injection into user messages.
// Returns empty string if no valid skills exist.
// Applies 3-level budget degradation to stay within skillListBudgetBytes.
func (sm *SkillManager) RenderList(entries []SkillEntry) string {
	valid := make([]SkillEntry, 0, len(entries))
	for _, e := range entries {
		if e.Status == "ok" {
			valid = append(valid, e)
		}
	}
	if len(valid) == 0 {
		return ""
	}

	// Level 1: full descriptions
	lines := make([]string, len(valid))
	for i, e := range valid {
		lines[i] = fmt.Sprintf("- %s: %s", e.Name, e.Description)
	}
	body := strings.Join(lines, "\n")
	if len(body) <= skillListBudgetBytes {
		return body
	}

	// Level 2: truncate descriptions
	maxLen := skillListBudgetBytes / len(valid)
	if maxLen < 20 {
		maxLen = 20
	}
	for i, e := range valid {
		desc := e.Description
		if len(desc) > maxLen {
			desc = desc[:maxLen-1] + "…"
		}
		lines[i] = fmt.Sprintf("- %s: %s", e.Name, desc)
	}
	body = strings.Join(lines, "\n")
	if len(body) <= skillListBudgetBytes {
		return body
	}

	// Level 3: names only
	for i, e := range valid {
		lines[i] = fmt.Sprintf("- %s", e.Name)
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/agent/ -run "TestSkillManager" -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/skill_manager.go internal/agent/skill_manager_test.go
git commit -m "feat: add SkillManager hash tracking and list rendering"
```

---

## Task 3: InvokeSkillTool

**Files:**
- Create: `internal/agent/tools_skill.go`
- Create: `internal/agent/tools_skill_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/agent/tools_skill_test.go
package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInvokeSkillTool_Success(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "deploy", "---\ndescription: Use when deploying.\n---\n\n# Deploy Steps\n1. Build\n2. Upload")

	tool := NewInvokeSkillTool(dir)
	result, err := tool.Execute(context.Background(), map[string]any{"name": "deploy"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Content)
	}
	if result.Content != "Loading skill: deploy" {
		t.Errorf("unexpected tool_result content: %q", result.Content)
	}
	if result.NewMessages == nil || len(result.NewMessages) == 0 {
		t.Fatal("expected newMessages")
	}
	msg := result.NewMessages[0]
	if !msg.IsMeta {
		t.Error("expected isMeta message")
	}
	if !contains(msg.Content, "<loaded-skill name=deploy>") {
		t.Errorf("message missing loaded-skill tag: %q", msg.Content)
	}
	if !contains(msg.Content, "Base directory for this skill:") {
		t.Errorf("message missing base dir: %q", msg.Content)
	}
	if !contains(msg.Content, "# Deploy Steps") {
		t.Errorf("message missing body: %q", msg.Content)
	}
	if !contains(msg.Content, "</loaded-skill>") {
		t.Errorf("message missing closing tag: %q", msg.Content)
	}
}

func TestInvokeSkillTool_NotFound(t *testing.T) {
	dir := t.TempDir()
	tool := NewInvokeSkillTool(dir)
	result, err := tool.Execute(context.Background(), map[string]any{"name": "missing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for missing skill")
	}
}

func TestInvokeSkillTool_StripsFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "ops", "---\ndescription: Use when ops.\n---\n\n# Ops Body")
	tool := NewInvokeSkillTool(dir)
	result, _ := tool.Execute(context.Background(), map[string]any{"name": "ops"})
	msg := result.NewMessages[0].Content
	if contains(msg, "description:") {
		t.Error("frontmatter should be stripped from message")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/agent/ -run "TestInvokeSkillTool" -v 2>&1 | head -20
```
Expected: compile error — `NewInvokeSkillTool` not defined, `NewMessages` field not on `ToolResult`.

- [ ] **Step 3: Add NewMessages to ToolResult**

In `internal/agent/tools.go`, add `NewMessages` field:

```go
type InjectMessage struct {
	Content string
	IsMeta  bool
}

type ToolResult struct {
	Content     string          `json:"content"`
	IsError     bool            `json:"is_error"`
	RiskLevel   RiskLevel       `json:"risk_level"`
	NewMessages []InjectMessage `json:"-"`
}
```

- [ ] **Step 4: Implement InvokeSkillTool**

```go
// internal/agent/tools_skill.go
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InvokeSkillTool struct {
	skillsDir string
}

func NewInvokeSkillTool(skillsDir string) *InvokeSkillTool {
	return &InvokeSkillTool{skillsDir: skillsDir}
}

func (t *InvokeSkillTool) Name() string             { return "invoke_skill" }
func (t *InvokeSkillTool) DefaultRiskLevel() RiskLevel { return RiskL1 }

func (t *InvokeSkillTool) Description() string {
	return `Execute a skill within the main conversation. When user's request matches a skill's description, this is a BLOCKING REQUIREMENT: invoke this tool BEFORE generating any other response. NEVER mention a skill without calling this tool. If you see <loaded-skill name=X> in current turn, skill already loaded — follow instructions directly, do NOT call again. Read-only. No side effects.`
}

func (t *InvokeSkillTool) InputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"name"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Skill name (directory name under skills/)"},
		},
	}
}

func (t *InvokeSkillTool) Execute(_ context.Context, input map[string]any) (*ToolResult, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return &ToolResult{Content: "missing required field: name", IsError: true, RiskLevel: RiskL1}, nil
	}

	mdPath := filepath.Join(t.skillsDir, filepath.FromSlash(name), "SKILL.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{Content: fmt.Sprintf("Skill %q not found", name), IsError: true, RiskLevel: RiskL1}, nil
		}
		return &ToolResult{Content: fmt.Sprintf("failed to read skill: %v", err), IsError: true, RiskLevel: RiskL1}, nil
	}

	_, body, parseErr := parseSkillFrontmatter(string(data))
	if parseErr != nil {
		return &ToolResult{Content: fmt.Sprintf("skill %q has invalid frontmatter: %v", name, parseErr), IsError: true, RiskLevel: RiskL1}, nil
	}

	// Replace ${SKILL_DIR} variable
	skillDir := filepath.Join(t.skillsDir, filepath.FromSlash(name))
	body = strings.ReplaceAll(body, "${SKILL_DIR}", skillDir)

	msgContent := fmt.Sprintf("<loaded-skill name=%s>\nBase directory for this skill: %s\n%s\n</loaded-skill>", name, skillDir, body)

	return &ToolResult{
		Content:   fmt.Sprintf("Loading skill: %s", name),
		IsError:   false,
		RiskLevel: RiskL1,
		NewMessages: []InjectMessage{
			{Content: msgContent, IsMeta: true},
		},
	}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/agent/ -run "TestInvokeSkillTool" -v
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tools.go internal/agent/tools_skill.go internal/agent/tools_skill_test.go
git commit -m "feat: add InvokeSkillTool"
```

---

## Task 4: Wire SkillManager into agent — skill list injection

**Files:**
- Modify: `internal/agent/factory.go`
- Modify: `internal/agent/agent.go`
- Modify: `internal/api/chat.go`

- [ ] **Step 1: Add SkillsDir to Factory struct**

In `internal/agent/factory.go`, add field to `Factory`:

```go
type Factory struct {
	// ... existing fields ...
	SkillsDir string
}
```

- [ ] **Step 2: Register InvokeSkillTool in NewAgent**

In `Factory.NewAgent`, after existing `registry.Register(...)` calls:

```go
if f.SkillsDir != "" {
	registry.Register(NewInvokeSkillTool(f.SkillsDir))
}
```

- [ ] **Step 3: Add SkillManager to AgentConfig and Agent**

In `internal/agent/agent.go`:

```go
type AgentConfig struct {
	// ... existing fields ...
	SkillManager *SkillManager
}

type Agent struct {
	// ... existing fields ...
	skillManager  *SkillManager
	lastSkillHash string
}
```

In `NewAgent` function:

```go
return &Agent{
	// ... existing fields ...
	skillManager: cfg.SkillManager,
}
```

- [ ] **Step 4: Pass SkillManager from Factory.NewAgent**

In `Factory.NewAgent`, pass to `NewAgent`:

```go
return NewAgent(AgentConfig{
	// ... existing fields ...
	SkillManager: NewSkillManager(f.SkillsDir),
})
```

- [ ] **Step 5: Inject skill list into user messages**

In `internal/agent/agent.go`, in `Run()`, find `a.msgStore.Save(conversationID, "user", userMessage, "")` and replace with:

```go
finalUserMessage := userMessage
if a.skillManager != nil {
	currentHash, _ := a.skillManager.ComputeHash()
	if currentHash != a.lastSkillHash {
		entries, _ := a.skillManager.LoadSkills()
		list := a.skillManager.RenderList(entries)
		if list != "" {
			finalUserMessage = fmt.Sprintf("<skills>\nNOTE: Replaces any earlier skill list in this conversation.\n%s\n</skills>\n\n%s", list, userMessage)
		}
		a.lastSkillHash = currentHash
	}
}
a.msgStore.Save(conversationID, "user", finalUserMessage, "")
```

- [ ] **Step 6: Handle NewMessages from tool results**

In `Run()`, after `result, err := tool.Execute(ctx, tc.Input)` and error handling, before the `events <- Event{Type: EventToolResult, ...}` line, add:

```go
for _, msg := range result.NewMessages {
	history = append(history, llm.Message{Role: llm.RoleUser, Content: msg.Content})
}
```

- [ ] **Step 7: Wire SkillsDir in chat.go**

In `internal/api/chat.go`, after `factory` is created, add:

```go
factory.SkillsDir = filepath.Join(app.Config.DataDir, "skills")
```

Add `"path/filepath"` to imports if not present.

- [ ] **Step 8: Build and run all tests**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./... && go test ./internal/agent/ -v 2>&1 | grep -E "PASS|FAIL|ok"
```
Expected: build succeeds, all tests PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/agent/agent.go internal/agent/factory.go internal/api/chat.go
git commit -m "feat: wire SkillManager into agent, inject skill list and handle newMessages"
```

---

## Task 5: API — frontmatter validation on upload + status in list response

**Files:**
- Modify: `internal/api/skills.go`
- Modify: `internal/agent/skill_manager.go` (export ParseSkillFrontmatter)

- [ ] **Step 1: Export ParseSkillFrontmatter**

In `internal/agent/skill_manager.go`, rename `parseSkillFrontmatter` → `ParseSkillFrontmatter`. Update all callers in the same package:
- `skill_manager.go`: `LoadSkills` calls it
- `tools_skill.go`: `Execute` calls it

- [ ] **Step 2: Update test references**

In `internal/agent/skill_manager_test.go`, update calls from `parseSkillFrontmatter` → `ParseSkillFrontmatter`.

- [ ] **Step 3: Update skillInfo struct**

In `internal/api/skills.go`, replace `skillInfo`:

```go
type skillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}
```

- [ ] **Step 4: Update listSkillsHandler**

Replace `listSkillsHandler` body:

```go
func listSkillsHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sm := agent.NewSkillManager(filepath.Join(dataDir, "skills"))
		entries, err := sm.LoadSkills()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read skills dir")
			return
		}
		skills := make([]skillInfo, len(entries))
		for i, e := range entries {
			skills[i] = skillInfo{Name: e.Name, Description: e.Description, Status: e.Status, Error: e.Error}
		}
		writeJSON(w, http.StatusOK, skills)
	}
}
```

Add imports: `"github.com/spiderai/spider/internal/agent"` and `"path/filepath"`.

- [ ] **Step 5: Add frontmatter validation to uploadSkillHandler**

After `body, err := io.ReadAll(...)` and before `os.MkdirAll`, add:

```go
if _, _, err := agent.ParseSkillFrontmatter(string(body)); err != nil {
	writeError(w, http.StatusBadRequest, "invalid SKILL.md: "+err.Error())
	return
}
```

- [ ] **Step 6: Build and run all tests**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./... && go test ./internal/... 2>&1 | grep -E "PASS|FAIL|ok"
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/skills.go internal/agent/skill_manager.go internal/agent/tools_skill.go internal/agent/skill_manager_test.go
git commit -m "feat: add frontmatter validation on upload and status in skill list API"
```

---

## Task 6: Frontend — health status + upload hint + validation

**Files:**
- Modify: `web/src/views/SkillsPanel.vue`

- [ ] **Step 1: Update Skill interface**

In `<script setup>`, replace `interface Skill`:

```typescript
interface Skill {
  name: string
  description: string
  status: 'ok' | 'error'
  error?: string
}
```

- [ ] **Step 2: Add status badges to skill rows**

In `<template>`, replace the skill row `<div v-for="skill in skills"...>` content:

```html
<div
  v-for="skill in skills" :key="skill.name"
  class="sp-row"
  :class="{ selected: selected?.name === skill.name }"
  @click="selectSkill(skill)"
>
  <span class="sp-row-name">{{ skill.name }}</span>
  <span v-if="skill.status === 'error'" class="badge badge-error" :title="skill.error">✗</span>
  <span v-else class="badge badge-ok">✓</span>
</div>
```

- [ ] **Step 3: Add upload hint**

In `<template>`, add below the `<div class="sp-toolbar">` block:

```html
<details class="sp-hint">
  <summary>格式说明</summary>
  <pre class="sp-hint-body">Skill 供 spider agent 按需加载执行。

SKILL.md 格式:
---
description: Use when &lt;场景&gt;. &lt;能力描述&gt;. (≤250 字符)
---

# 正文
[触发条件、步骤、示例]

v1 限制: 仅支持单文件。</pre>
</details>
```

- [ ] **Step 4: Add validateSkillContent function**

In `<script setup>`, add:

```typescript
function validateSkillContent(content: string): string | null {
  if (!content.startsWith('---')) return 'missing frontmatter: file must start with ---'
  const parts = content.split('---')
  if (parts.length < 3) return 'malformed frontmatter: missing closing ---'
  const descMatch = parts[1].match(/description:\s*(.+)/)
  if (!descMatch) return 'description is required'
  const desc = descMatch[1].trim()
  if (desc.length > 250) return `description exceeds 250 characters (${desc.length})`
  return null
}
```

- [ ] **Step 5: Call validation in onFileChange**

In `onFileChange`, before `await uploadFile(file, name)`:

```typescript
const content = await file.text()
const validationError = validateSkillContent(content)
if (validationError) {
  setStatus({ type: 'error', msg: validationError })
  ;(e.target as HTMLInputElement).value = ''
  return
}
```

- [ ] **Step 6: Add CSS**

In `<style scoped>`, add:

```css
.badge-ok { background: var(--green, #22c55e); color: #fff; font-size: 11px; padding: 1px 5px; border-radius: 4px; }
.badge-error { background: var(--red, #ef4444); color: #fff; font-size: 11px; padding: 1px 5px; border-radius: 4px; cursor: help; }
.sp-hint { font-size: 11px; color: var(--label); padding: 6px 16px; border-bottom: 1px solid var(--border); }
.sp-hint summary { cursor: pointer; user-select: none; }
.sp-hint-body { margin: 6px 0 0; white-space: pre-wrap; font-family: monospace; font-size: 11px; line-height: 1.5; }
```

- [ ] **Step 7: Build frontend and Go binary**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run build && cd .. && go build -a -o /tmp/spider-skill-test ./cmd/spider
```
Expected: no errors.

- [ ] **Step 8: Smoke test UI**

```bash
/tmp/spider-skill-test serve --addr :8003 --data-dir ~/.spider/data
```

Open http://localhost:8003 → Skills panel:
- Existing skills show ✓
- Upload valid SKILL.md → ✓ appears
- Upload SKILL.md with description > 250 chars → error shown in status bar, not uploaded
- Upload SKILL.md with broken YAML → error shown in status bar, not uploaded

- [ ] **Step 9: Commit**

```bash
git add web/src/views/SkillsPanel.vue
git commit -m "feat: add skill health status, upload hint, and frontend validation"
```

---

## Task 7: Integration smoke test

**Files:** none (manual verification)

- [ ] **Step 1: Create a test skill**

```bash
mkdir -p ~/.spider/data/skills/deploy
cat > ~/.spider/data/skills/deploy/SKILL.md << 'EOF'
---
description: Use when user asks to deploy, release, or publish the application.
---

# Deploy Skill

When the user asks to deploy:
1. Ask which environment (staging or production)
2. Run `make build-linux` locally
3. Upload binary to target hosts
4. Restart the service
EOF
```

- [ ] **Step 2: Start spider**

```bash
/tmp/spider-skill-test serve --addr :8003 --data-dir ~/.spider/data
```

- [ ] **Step 3: Verify skill list injection in DB**

Open http://localhost:8003, start a new conversation, send any message. Then:

```bash
sqlite3 ~/.spider/data/spider.db "SELECT content FROM messages ORDER BY created_at DESC LIMIT 5;"
```
Expected: first user message contains `<skills>` block listing `deploy`.

- [ ] **Step 4: Verify invoke_skill trigger**

In the conversation, type: "帮我发布应用"

Expected: agent calls `invoke_skill("deploy")`, response uses deploy skill instructions.

- [ ] **Step 5: Verify skill list update on new skill**

```bash
mkdir -p ~/.spider/data/skills/backup
cat > ~/.spider/data/skills/backup/SKILL.md << 'EOF'
---
description: Use when user asks to backup or restore data.
---
# Backup Skill
EOF
```

Send another message. Check DB — new user message should contain updated `<skills>` block with both `deploy` and `backup`.

- [ ] **Step 6: Final commit**

```bash
git add .
git commit -m "feat: agent skill support complete"
```
