# Built-in + Custom Skills Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持内置 skill（随 binary 发布，只读）和用户自定义 skill（CRUD），合并展示，agent 调用时同名 custom 优先。

**Architecture:** 双目录隔离：`skills_builtin/`（embed 写入，启动强制覆盖）+ `skills/`（用户自定义）。`SkillManager` 合并两目录加载，新增 `Source` 字段区分来源。API 路由加 `{source}` 前缀，UI 显示锁图标并禁用 builtin 的删除/编辑。

**Tech Stack:** Go 1.22, `embed.FS`, Vue 3 Composition API, existing `SkillManager` / `skills.go` / `SkillsPanel.vue`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `cmd/spider/embed.go` | Modify | 新增 `//go:embed all:skills` |
| `cmd/spider/main.go` | Modify | 启动时调 `SyncBuiltinSkills` |
| `internal/agent/skill_manager.go` | Modify | 双目录加载，`Source` 字段，`RenderList` 优先级 |
| `internal/agent/skill_manager_test.go` | Modify | 新增双目录、Source、RenderList 优先级测试 |
| `internal/api/skills.go` | Modify | 新路由 handler，`skillInfo.Source`，builtin 保护 |
| `internal/api/handler.go` | Modify | 更新路由注册 |
| `web/src/views/SkillsPanel.vue` | Modify | 锁图标，禁用按钮，复制流程，新 API 路径 |

---

## Task 1: SkillManager — Source 字段 + 双目录加载

**Files:**
- Modify: `internal/agent/skill_manager.go`
- Modify: `internal/agent/skill_manager_test.go`

- [ ] **Step 1: 写失败测试**

在 `internal/agent/skill_manager_test.go` 末尾（`writeSkillFile` 函数前）添加：

```go
func TestSkillManager_LoadSkills_Source(t *testing.T) {
	dataDir := t.TempDir()
	writeSkillFile(t, filepath.Join(dataDir, "skills_builtin"), "cron",
		"---\ndescription: Builtin cron skill.\n---\n# Cron")
	writeSkillFile(t, filepath.Join(dataDir, "skills"), "deploy",
		"---\ndescription: Custom deploy skill.\n---\n# Deploy")

	sm := NewSkillManager(dataDir)
	skills, err := sm.LoadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if skills[0].Name != "cron" || skills[0].Source != "builtin" {
		t.Errorf("expected cron/builtin, got %s/%s", skills[0].Name, skills[0].Source)
	}
	if skills[1].Name != "deploy" || skills[1].Source != "custom" {
		t.Errorf("expected deploy/custom, got %s/%s", skills[1].Name, skills[1].Source)
	}
}

func TestSkillManager_LoadSkills_SameNameBothSources(t *testing.T) {
	dataDir := t.TempDir()
	writeSkillFile(t, filepath.Join(dataDir, "skills_builtin"), "cron",
		"---\ndescription: Builtin cron.\n---\n# Cron builtin")
	writeSkillFile(t, filepath.Join(dataDir, "skills"), "cron",
		"---\ndescription: Custom cron.\n---\n# Cron custom")

	sm := NewSkillManager(dataDir)
	skills, err := sm.LoadSkills()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills (both sources), got %d", len(skills))
	}
	if skills[0].Source != "custom" {
		t.Errorf("expected custom first, got %s", skills[0].Source)
	}
	if skills[1].Source != "builtin" {
		t.Errorf("expected builtin second, got %s", skills[1].Source)
	}
}
```

确认 `"path/filepath"` 已在测试文件 import 中。

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/... -run "TestSkillManager_LoadSkills_Source|TestSkillManager_LoadSkills_SameNameBothSources" -v 2>&1 | tail -20
```

期望：FAIL（`NewSkillManager` 签名不匹配，`Source` 字段不存在）

- [ ] **Step 3: 更新 SkillEntry + SkillManager 结构体**

在 `internal/agent/skill_manager.go` 中：

将 `SkillEntry` 替换为：

```go
type SkillEntry struct {
	Name        string
	Description string
	Status      string // "ok" | "error"
	Error       string
	Source      string // "builtin" | "custom"
	bodyPath    string
}
```

将 `SkillManager` + `NewSkillManager` 替换为：

```go
type SkillManager struct {
	builtinDir string
	customDir  string
}

func NewSkillManager(dataDir string) *SkillManager {
	return &SkillManager{
		builtinDir: filepath.Join(dataDir, "skills_builtin"),
		customDir:  filepath.Join(dataDir, "skills"),
	}
}
```

- [ ] **Step 4: 替换 LoadSkills**

将整个 `LoadSkills` 函数替换为：

```go
func (sm *SkillManager) LoadSkills() ([]SkillEntry, error) {
	var entries []SkillEntry
	for _, src := range []struct {
		dir    string
		source string
	}{
		{sm.builtinDir, "builtin"},
		{sm.customDir, "custom"},
	} {
		err := filepath.WalkDir(src.dir, func(path string, d fs.DirEntry, err error) error {
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
			rel, _ := filepath.Rel(src.dir, dir)
			if rel == "." {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				entries = append(entries, SkillEntry{Name: rel, Status: "error", Error: readErr.Error(), Source: src.source})
				return nil
			}
			meta, _, parseErr := ParseSkillFrontmatter(string(data))
			if parseErr != nil {
				entries = append(entries, SkillEntry{Name: rel, Status: "error", Error: parseErr.Error(), Source: src.source})
				return nil
			}
			entries = append(entries, SkillEntry{
				Name: rel, Description: meta.Description,
				Status: "ok", Source: src.source, bodyPath: path,
			})
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Source == "custom" && entries[j].Source == "builtin"
	})
	return entries, nil
}
```

- [ ] **Step 5: 更新 ComputeHash — 扫两个目录**

将整个 `ComputeHash` 函数替换为：

```go
func (sm *SkillManager) ComputeHash() (string, error) {
	var parts []string
	for _, dir := range []string{sm.builtinDir, sm.customDir} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != "SKILL.md" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			parts = append(parts, fmt.Sprintf("%s:%d\n", path, info.ModTime().UnixNano()))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(parts)
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
```

- [ ] **Step 6: 更新 RenderList — 同名时 custom 遮蔽 builtin**

将整个 `RenderList` 函数替换为：

```go
func (sm *SkillManager) RenderList(entries []SkillEntry) string {
	seen := make(map[string]bool)
	var ok []SkillEntry
	for _, e := range entries {
		if e.Status != "ok" {
			continue
		}
		if seen[e.Name] {
			continue // custom already added (custom sorts before builtin)
		}
		seen[e.Name] = true
		ok = append(ok, e)
	}
	if len(ok) == 0 {
		return ""
	}
	if s := renderLines(ok, func(e SkillEntry) string {
		return fmt.Sprintf("- %s: %s", e.Name, e.Description)
	}); len(s) <= skillListBudgetBytes {
		return s
	}
	if s := renderLines(ok, func(e SkillEntry) string {
		desc := e.Description
		if len([]rune(desc)) > 80 {
			desc = string([]rune(desc)[:79]) + "…"
		}
		return fmt.Sprintf("- %s: %s", e.Name, desc)
	}); len(s) <= skillListBudgetBytes {
		return s
	}
	s := renderLines(ok, func(e SkillEntry) string {
		return fmt.Sprintf("- %s", e.Name)
	})
	if len(s) > skillListBudgetBytes {
		s = s[:skillListBudgetBytes]
	}
	return s
}
```

- [ ] **Step 7: 修复现有测试 — NewSkillManager 签名变了**

现有测试用 `NewSkillManager(dir)` 传的是 skills 目录本身。需要更新所有现有测试：

在 `skill_manager_test.go` 中，将所有 `NewSkillManager(dir)` 改为先建 dataDir，把 skill 写入 `filepath.Join(dataDir, "skills")`，再调 `NewSkillManager(dataDir)`。

具体：`writeSkillFile` helper 签名不变，但调用处的 base 参数从 `dir` 改为 `filepath.Join(dataDir, "skills")`，`NewSkillManager` 参数从 `dir` 改为 `dataDir`。

- [ ] **Step 8: 运行所有 agent 测试**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/... -v 2>&1 | tail -30
```

期望：全部 PASS

- [ ] **Step 9: Commit**

```bash
git add internal/agent/skill_manager.go internal/agent/skill_manager_test.go
git commit -m "feat: SkillManager dual-dir loading with Source field"
```

---

## Task 2: SyncBuiltinSkills + embed

**Files:**
- Modify: `cmd/spider/embed.go`
- Create: `internal/agent/sync_builtin.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: 写失败测试**

创建 `internal/agent/sync_builtin_test.go`：

```go
package agent

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestSyncBuiltinSkills_WritesFiles(t *testing.T) {
	mockFS := fstest.MapFS{
		"skills/cron/SKILL.md":    {Data: []byte("---\ndescription: Cron skill.\n---\n# Cron")},
		"skills/monitor/SKILL.md": {Data: []byte("---\ndescription: Monitor skill.\n---\n# Monitor")},
	}
	dataDir := t.TempDir()
	if err := SyncBuiltinSkills(dataDir, mockFS); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"cron", "monitor"} {
		p := filepath.Join(dataDir, "skills_builtin", name, "SKILL.md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist: %v", p, err)
		}
	}
}

func TestSyncBuiltinSkills_OverwritesExisting(t *testing.T) {
	mockFS := fstest.MapFS{
		"skills/cron/SKILL.md": {Data: []byte("---\ndescription: New cron.\n---\n# New")},
	}
	dataDir := t.TempDir()
	// pre-write old content
	dir := filepath.Join(dataDir, "skills_builtin", "cron")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("old content"), 0o644)

	if err := SyncBuiltinSkills(dataDir, mockFS); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if string(data) != "---\ndescription: New cron.\n---\n# New" {
		t.Errorf("expected overwrite, got: %s", data)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/... -run "TestSyncBuiltinSkills" -v 2>&1 | tail -10
```

期望：FAIL（`SyncBuiltinSkills` 未定义）

- [ ] **Step 3: 实现 SyncBuiltinSkills**

创建 `internal/agent/sync_builtin.go`：

```go
package agent

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func SyncBuiltinSkills(dataDir string, fsys fs.FS) error {
	destBase := filepath.Join(dataDir, "skills_builtin")
	return fs.WalkDir(fsys, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "skills/")
		dest := filepath.Join(destBase, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/... -run "TestSyncBuiltinSkills" -v 2>&1 | tail -10
```

期望：PASS

- [ ] **Step 5: 更新 embed.go**

在 `cmd/spider/embed.go` 中添加：

```go
//go:embed all:skills
var builtinSkillsFS embed.FS
```

完整文件变为：

```go
package main

import "embed"

//go:embed all:dist
var webFS embed.FS

//go:embed all:skills
var builtinSkillsFS embed.FS
```

- [ ] **Step 6: 在 serve() 中调用 SyncBuiltinSkills**

在 `cmd/spider/main.go` 的 `serve()` 函数中，`cfg.EnsureDataDir()` 调用之后添加：

```go
if err := agent.SyncBuiltinSkills(cfg.DataDir, builtinSkillsFS); err != nil {
    return fmt.Errorf("同步内置 skills 失败: %w", err)
}
```

确认 `agent` 包已在 import 中（搜索 `"github.com/spiderai/spider/internal/agent"`）。

- [ ] **Step 7: 编译确认**

```bash
cd /Users/cw/fty.ai/spider.ai
go build ./cmd/spider/... 2>&1
```

期望：无错误

- [ ] **Step 8: Commit**

```bash
git add cmd/spider/embed.go cmd/spider/main.go internal/agent/sync_builtin.go internal/agent/sync_builtin_test.go
git commit -m "feat: SyncBuiltinSkills writes embedded skills to skills_builtin/ on startup"
```

---

## Task 3: API 层 — 新路由 + Source 字段

**Files:**
- Modify: `internal/api/skills.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: 更新 skillInfo 结构体**

在 `internal/api/skills.go` 中，将 `skillInfo` 替换为：

```go
type skillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	Source      string `json:"source"`
}
```

- [ ] **Step 2: 更新 listSkillsHandler**

将 `listSkillsHandler` 替换为：

```go
func listSkillsHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sm := agent.NewSkillManager(dataDir)
		entries, err := sm.LoadSkills()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read skills dir")
			return
		}
		skills := make([]skillInfo, len(entries))
		for i, e := range entries {
			skills[i] = skillInfo{
				Name:        e.Name,
				Description: e.Description,
				Status:      e.Status,
				Error:       e.Error,
				Source:      e.Source,
			}
		}
		writeJSON(w, http.StatusOK, skills)
	}
}
```

- [ ] **Step 3: 新增 getSkillBySourceHandler**

将旧的 `getSkillHandler` 替换为：

```go
func getSkillBySourceHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source := r.PathValue("source")
		name := r.PathValue("name")
		if source != "builtin" && source != "custom" {
			writeError(w, http.StatusBadRequest, "source must be builtin or custom")
			return
		}
		if !isValidSkillName(name) {
			writeError(w, http.StatusBadRequest, "invalid skill name")
			return
		}
		var dir string
		if source == "builtin" {
			dir = filepath.Join(dataDir, "skills_builtin")
		} else {
			dir = filepath.Join(dataDir, "skills")
		}
		mdPath := filepath.Join(dir, filepath.FromSlash(name), "SKILL.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "skill not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to read skill")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	}
}
```

- [ ] **Step 4: 更新 uploadSkillHandler — 只写 custom**

将 `uploadSkillHandler` 中的路径从 `filepath.Join(dataDir, "skills", name)` 确认不变（已经是 custom 目录），函数名改为 `uploadCustomSkillHandler`：

```go
func uploadCustomSkillHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !isValidSkillName(name) {
			writeError(w, http.StatusBadRequest, "invalid skill name")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read body")
			return
		}
		if _, _, err := agent.ParseSkillFrontmatter(string(body)); err != nil {
			writeError(w, http.StatusBadRequest, "invalid SKILL.md: "+err.Error())
			return
		}
		dir := filepath.Join(dataDir, "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create skill dir")
			return
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), body, 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to write SKILL.md")
			return
		}
		writeJSON(w, http.StatusOK, skillInfo{Name: name, Status: "ok", Source: "custom"})
	}
}
```

- [ ] **Step 5: 更新 deleteSkillHandler — 只删 custom，builtin 返回 403**

将 `deleteSkillHandler` 改名为 `deleteCustomSkillHandler`：

```go
func deleteCustomSkillHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !isValidSkillName(name) {
			writeError(w, http.StatusBadRequest, "invalid skill name")
			return
		}
		dir := filepath.Join(dataDir, "skills", name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete skill")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
```

- [ ] **Step 6: 更新 handler.go 路由注册**

在 `internal/api/handler.go` 中，将旧的 skills 路由：

```go
mux.HandleFunc("GET /api/v1/skills", listSkillsHandler(app.Config.DataDir))
mux.HandleFunc("GET /api/v1/skills/{name...}", getSkillHandler(app.Config.DataDir))
mux.HandleFunc("PUT /api/v1/skills/{name...}", uploadSkillHandler(app.Config.DataDir))
mux.HandleFunc("DELETE /api/v1/skills/{name...}", deleteSkillHandler(app.Config.DataDir))
```

替换为：

```go
mux.HandleFunc("GET /api/v1/skills", listSkillsHandler(app.Config.DataDir))
mux.HandleFunc("GET /api/v1/skills/{source}/{name...}", getSkillBySourceHandler(app.Config.DataDir))
mux.HandleFunc("PUT /api/v1/skills/custom/{name...}", uploadCustomSkillHandler(app.Config.DataDir))
mux.HandleFunc("DELETE /api/v1/skills/custom/{name...}", deleteCustomSkillHandler(app.Config.DataDir))
```

- [ ] **Step 7: 编译确认**

```bash
cd /Users/cw/fty.ai/spider.ai
go build ./... 2>&1
```

期望：无错误

- [ ] **Step 8: Commit**

```bash
git add internal/api/skills.go internal/api/handler.go
git commit -m "feat: RESTful skills API with source prefix and Source field"
```

---

## Task 4: UI — 锁图标 + 禁用按钮 + 复制流程

**Files:**
- Modify: `web/src/views/SkillsPanel.vue`

- [ ] **Step 1: 更新 Skill 接口 + selectSkill API 路径**

在 `<script setup>` 中，将 `Skill` 接口改为：

```ts
interface Skill { name: string; status: string; error?: string; source: 'builtin' | 'custom' }
```

将 `selectSkill` 中的 fetch 路径从：
```ts
const res = await fetch(`/api/v1/skills/${encodeSkillName(skill.name)}`)
```
改为：
```ts
const res = await fetch(`/api/v1/skills/${skill.source}/${encodeSkillName(skill.name)}`)
```

- [ ] **Step 2: 更新 uploadFile + deleteSkill API 路径**

将 `uploadFile` 中的 fetch 路径从：
```ts
const res = await fetch(`/api/v1/skills/${encodeSkillName(name)}`, {
  method: 'PUT', ...
})
```
改为：
```ts
const res = await fetch(`/api/v1/skills/custom/${encodeSkillName(name)}`, {
  method: 'PUT', ...
})
```

将 `deleteSkill` 中的 fetch 路径从：
```ts
await fetch(`/api/v1/skills/${encodeSkillName(name)}`, { method: 'DELETE' })
```
改为：
```ts
await fetch(`/api/v1/skills/custom/${encodeSkillName(name)}`, { method: 'DELETE' })
```

- [ ] **Step 3: 添加 copySkill 函数**

在 `deleteSkill` 函数后添加：

```ts
async function copySkill(skill: Skill) {
  loading.value = true
  rawContent.value = ''
  try {
    const res = await fetch(`/api/v1/skills/${skill.source}/${encodeSkillName(skill.name)}`)
    if (!res.ok) { setStatus({ type: 'error', msg: '复制失败' }); return }
    const content = await res.text()
    copyContent.value = content
    copySourceName.value = skill.name
    showCopyEditor.value = true
  } finally {
    loading.value = false
  }
}

async function saveCopy() {
  const name = copyTargetName.value.trim()
  if (!name) { setStatus({ type: 'error', msg: '请输入 Skill 名称' }); return }
  await uploadFile(new File([copyContent.value], `${name}.md`, { type: 'text/plain' }), name)
  showCopyEditor.value = false
}
```

在 `ref` 声明区添加：

```ts
const showCopyEditor = ref(false)
const copyContent = ref('')
const copySourceName = ref('')
const copyTargetName = ref('')
```

- [ ] **Step 4: 更新模板 — 列表行加锁图标**

将列表行 `<div v-for="skill in skills"...>` 内容替换为：

```html
<div
  v-for="skill in skills" :key="skill.source + ':' + skill.name"
  class="sp-row"
  :class="{ selected: selected?.name === skill.name && selected?.source === skill.source }"
  @click="selectSkill(skill)"
>
  <span class="sp-row-name">
    <span v-if="skill.source === 'builtin'" class="sp-lock" title="内置 Skill，只读">🔒</span>
    {{ skill.name }}
  </span>
  <span class="badge" :class="skill.status === 'ok' ? 'badge-ok' : 'badge-err'"
    :title="skill.error || undefined">
    {{ skill.status === 'ok' ? 'ok' : 'error' }}
  </span>
</div>
```

- [ ] **Step 5: 更新模板 — 详情栏按钮**

将详情栏 `<div class="sp-topbar-right">` 内容替换为：

```html
<div class="sp-topbar-right">
  <button class="btn btn-sm btn-secondary" @click="copySkill(selected)">复制</button>
  <button
    class="btn btn-sm btn-primary"
    :disabled="selected.source === 'builtin'"
    @click="selected.source === 'custom' && triggerUpload(selected.name)"
  >上传新版本</button>
  <button
    class="btn btn-sm btn-danger"
    :disabled="selected.source === 'builtin'"
    @click="selected.source === 'custom' && deleteSkill(selected.name)"
  >删除</button>
</div>
```

- [ ] **Step 6: 添加复制编辑器弹窗**

在 `<input ref="fileInput".../>` 前添加：

```html
<!-- 复制编辑器 -->
<div v-if="showCopyEditor" class="sp-copy-overlay" @click.self="showCopyEditor = false">
  <div class="sp-copy-modal">
    <div class="sp-copy-header">
      <span>复制 "{{ copySourceName }}"</span>
      <button class="btn btn-sm" @click="showCopyEditor = false">✕</button>
    </div>
    <div class="sp-copy-body">
      <label class="sp-copy-label">新名称</label>
      <input v-model="copyTargetName" class="sp-copy-input" placeholder="my-skill" />
      <label class="sp-copy-label">内容</label>
      <textarea v-model="copyContent" class="sp-copy-textarea" rows="16" />
    </div>
    <div class="sp-copy-footer">
      <button class="btn btn-primary btn-sm" @click="saveCopy">保存为自定义</button>
      <button class="btn btn-sm" @click="showCopyEditor = false">取消</button>
    </div>
  </div>
</div>
```

- [ ] **Step 7: 添加复制弹窗样式**

在 `<style scoped>` 末尾添加：

```css
.sp-lock { font-size: 11px; margin-right: 4px; }
.sp-copy-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,0.4);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.sp-copy-modal {
  background: var(--panel); border: 1px solid var(--border); border-radius: 8px;
  width: 560px; max-height: 80vh; display: flex; flex-direction: column;
}
.sp-copy-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 16px; border-bottom: 1px solid var(--border); font-weight: 600; font-size: 13px;
}
.sp-copy-body { padding: 16px; display: flex; flex-direction: column; gap: 8px; overflow-y: auto; }
.sp-copy-label { font-size: 12px; color: var(--label); }
.sp-copy-input {
  width: 100%; padding: 6px 10px; border: 1px solid var(--border); border-radius: 4px;
  background: var(--input-bg, var(--bg)); color: var(--text); font-size: 13px;
}
.sp-copy-textarea {
  width: 100%; padding: 8px 10px; border: 1px solid var(--border); border-radius: 4px;
  background: var(--input-bg, var(--bg)); color: var(--text); font-size: 12px;
  font-family: monospace; resize: vertical;
}
.sp-copy-footer {
  display: flex; gap: 8px; padding: 12px 16px; border-top: 1px solid var(--border);
  justify-content: flex-end;
}
```

- [ ] **Step 8: Commit**

```bash
git add web/src/views/SkillsPanel.vue
git commit -m "feat: SkillsPanel builtin lock icon, disabled buttons, copy editor"
```

---

## Task 5: 端到端验证

- [ ] **Step 1: 构建并启动测试服务器**

```bash
cd /Users/cw/fty.ai/spider.ai
npm --prefix web run build
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :18765 --data-dir /tmp/spider-test-data &
sleep 1
```

- [ ] **Step 2: 验证 skills_builtin 目录已创建**

```bash
ls /tmp/spider-test-data/skills_builtin/
```

期望：`cron  monitor  network  nginx  process`

- [ ] **Step 3: 验证 API 返回 source 字段**

```bash
curl -s http://localhost:18765/api/v1/skills | python3 -m json.tool | grep -A2 '"name"'
```

期望：每个 skill 有 `"source": "builtin"` 或 `"source": "custom"`

- [ ] **Step 4: 验证 GET by source**

```bash
curl -s http://localhost:18765/api/v1/skills/builtin/cron | head -5
```

期望：返回 cron SKILL.md 内容

- [ ] **Step 5: 验证 builtin 不可删除（路由不存在）**

```bash
curl -s -o /dev/null -w "%{http_code}" -X DELETE http://localhost:18765/api/v1/skills/builtin/cron
```

期望：`405`（Method Not Allowed）

- [ ] **Step 6: 停止测试服务器**

```bash
kill %1 2>/dev/null; rm -f /tmp/spider-test
```

- [ ] **Step 7: 运行全部测试**

```bash
cd /Users/cw/fty.ai/spider.ai
go test ./internal/agent/... ./internal/api/... -v 2>&1 | tail -30
```

期望：全部 PASS

- [ ] **Step 8: Final commit**

```bash
git add -A
git commit -m "feat: builtin + custom skills support complete"
```
