# Skill Frontmatter Parser Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 skill frontmatter 解析器能正确处理含冒号、中文等特殊字符的 description，同时修复现有 4 个 error 状态的 builtin skill 文件。

**Architecture:** 改用逐行提取 description 值的方式替代整块 YAML 解析，避免用户因不了解 YAML 引号规则而触发 parse error；同时将现有 skill 文件的 description 加引号以符合标准 YAML。

**Tech Stack:** Go, gopkg.in/yaml.v3

---

## 文件变更清单

- Modify: `internal/agent/skill_manager.go` — 改 `ParseSkillFrontmatter`，用预处理方式规避 YAML 冒号歧义
- Modify: `internal/agent/skill_manager_test.go` — 新增含冒号/中文 description 的测试用例
- Modify: `cmd/spider/skills/config-diff/SKILL.md` — description 加引号
- Modify: `cmd/spider/skills/disk-inspect/SKILL.md` — description 加引号
- Modify: `cmd/spider/skills/log-analysis/SKILL.md` — description 加引号
- Modify: `cmd/spider/skills/process-inspect/SKILL.md` — description 加引号

---

### Task 1: 为解析器新增失败测试

**Files:**
- Modify: `internal/agent/skill_manager_test.go`

- [ ] **Step 1: 在 `skill_manager_test.go` 末尾追加两个失败测试**

在 `writeSkillFile` 函数之前插入：

```go
func TestParseSkillFrontmatter_DescriptionWithColon(t *testing.T) {
	// description 含冒号但未加引号，当前会触发 YAML parse error
	content := "---\ndescription: Use when X. Triggers: foo、bar。\n---\n\n# Body"
	meta, body, err := ParseSkillFrontmatter(content)
	if err != nil {
		t.Fatalf("description with colon should not error: %v", err)
	}
	if meta.Description != "Use when X. Triggers: foo、bar。" {
		t.Errorf("got description %q", meta.Description)
	}
	if !strings.Contains(body, "# Body") {
		t.Errorf("got body %q", body)
	}
}

func TestParseSkillFrontmatter_DescriptionWithChineseColon(t *testing.T) {
	content := "---\ndescription: 用于对比配置。触发词：配置对比、漂移。\n---\n\n# Body"
	meta, _, err := ParseSkillFrontmatter(content)
	if err != nil {
		t.Fatalf("description with Chinese colon should not error: %v", err)
	}
	if meta.Description != "用于对比配置。触发词：配置对比、漂移。" {
		t.Errorf("got description %q", meta.Description)
	}
}
```

- [ ] **Step 2: 运行测试，确认新测试失败**

```bash
go test ./internal/agent/ -run "TestParseSkillFrontmatter_DescriptionWith" -v
```

期望：两个测试 FAIL，错误含 `frontmatter parse error: yaml`

- [ ] **Step 3: Commit**

```bash
git add internal/agent/skill_manager_test.go
git commit -m "test(skill): add failing tests for description with colon"
```

---

### Task 2: 修复解析器——预处理 description 行

**Files:**
- Modify: `internal/agent/skill_manager.go`

**思路：** 在交给 `yaml.Unmarshal` 之前，扫描 frontmatter 块，找到 `description:` 行，若值未被引号包裹则自动加双引号并转义内部双引号。其余字段保持原样。

- [ ] **Step 1: 写失败测试已在 Task 1 完成，直接修改 `ParseSkillFrontmatter`**

将 `skill_manager.go` 中的 `ParseSkillFrontmatter` 函数替换为：

```go
// ParseSkillFrontmatter splits YAML frontmatter from body and validates required fields.
// description 值中的冒号、中文等特殊字符无需手动加引号。
func ParseSkillFrontmatter(content string) (skillFrontmatter, string, error) {
	if !strings.HasPrefix(content, "---") {
		return skillFrontmatter{}, "", fmt.Errorf("missing frontmatter: file must start with ---")
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return skillFrontmatter{}, "", fmt.Errorf("malformed frontmatter: missing closing ---")
	}
	normalized := normalizeDescriptionLine(parts[1])
	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(normalized), &meta); err != nil {
		return skillFrontmatter{}, "", fmt.Errorf("frontmatter parse error: %w", err)
	}
	if meta.Description == "" {
		return skillFrontmatter{}, "", fmt.Errorf("description is required")
	}
	if len([]rune(meta.Description)) > maxDescriptionChars {
		return skillFrontmatter{}, "", fmt.Errorf("description exceeds %d characters (%d)", maxDescriptionChars, len(meta.Description))
	}
	body := strings.TrimPrefix(parts[2], "\n")
	return meta, body, nil
}

// normalizeDescriptionLine 找到 frontmatter 中的 description: 行，
// 若值未被引号包裹则自动加双引号，防止值中的冒号触发 YAML 解析错误。
func normalizeDescriptionLine(frontmatter string) string {
	lines := strings.Split(frontmatter, "\n")
	for i, line := range lines {
		key, val, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) != "description" {
			continue
		}
		val = strings.TrimLeft(val, " ")
		// 已有引号则不处理
		if strings.HasPrefix(val, `"`) || strings.HasPrefix(val, `'`) {
			break
		}
		// 转义值中的双引号，然后包裹
		escaped := strings.ReplaceAll(val, `"`, `\"`)
		lines[i] = strings.TrimRight(key, " ") + `: "` + escaped + `"`
		break
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 2: 运行新增测试，确认通过**

```bash
go test ./internal/agent/ -run "TestParseSkillFrontmatter" -v
```

期望：所有 `TestParseSkillFrontmatter_*` 测试 PASS

- [ ] **Step 3: 运行全部 agent 测试，确认无回归**

```bash
go test ./internal/agent/ -v
```

期望：全部 PASS

- [ ] **Step 4: Commit**

```bash
git add internal/agent/skill_manager.go
git commit -m "fix(skill): harden frontmatter parser — auto-quote description with colons"
```

---

### Task 3: 修复现有 builtin skill 文件

**Files:**
- Modify: `cmd/spider/skills/config-diff/SKILL.md`
- Modify: `cmd/spider/skills/disk-inspect/SKILL.md`
- Modify: `cmd/spider/skills/log-analysis/SKILL.md`
- Modify: `cmd/spider/skills/process-inspect/SKILL.md`

给 description 加引号，使文件符合标准 YAML（解析器修复后这步可选，但保持文件规范）。

- [ ] **Step 1: 修改 `cmd/spider/skills/config-diff/SKILL.md` 的 frontmatter**

将第 3 行改为：

```
description: "Use when comparing configuration files across multiple remote hosts via Spider. Triggers: 配置对比、配置一致性、配置漂移、config diff、哪台机器配置不一样、配置是否同步。"
```

- [ ] **Step 2: 修改 `cmd/spider/skills/disk-inspect/SKILL.md` 的 frontmatter**

将 description 行改为：

```
description: "Use when checking disk usage, finding large files, or cleaning up space on remote hosts via Spider. Triggers: 磁盘、磁盘使用率、磁盘满了、disk、df、du、大文件、清理磁盘、inode。"
```

- [ ] **Step 3: 修改 `cmd/spider/skills/log-analysis/SKILL.md` 的 frontmatter**

将 description 行改为：

```
description: "Use when analyzing logs on remote hosts via Spider. Triggers: 日志、报错、error、exception、异常、日志分析、log、查日志、错误频率、哪台机器有问题。"
```

- [ ] **Step 4: 修改 `cmd/spider/skills/process-inspect/SKILL.md` 的 frontmatter**

将 description 行改为：

```
description: "Use when checking process health on remote hosts via Spider. Triggers: 进程、进程挂了、服务是否在跑、内存泄漏、CPU 飙高、僵尸进程、process、ps、top、OOM。"
```

- [ ] **Step 5: 验证四个文件都能正确解析**

```bash
go test ./internal/agent/ -run "TestSkillManager_LoadSkills" -v
go build ./...
```

期望：测试 PASS，build 无错误

- [ ] **Step 6: Commit**

```bash
git add cmd/spider/skills/config-diff/SKILL.md \
        cmd/spider/skills/disk-inspect/SKILL.md \
        cmd/spider/skills/log-analysis/SKILL.md \
        cmd/spider/skills/process-inspect/SKILL.md
git commit -m "fix(skills): quote description values to comply with YAML spec"
```
