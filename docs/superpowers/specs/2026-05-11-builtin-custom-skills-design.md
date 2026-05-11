# Spec: Built-in + Custom Skills Support

**Date:** 2026-05-11  
**Status:** Approved

## Overview

支持两类 skill：内置 skill（随 binary 发布，只读）和用户自定义 skill（CRUD）。两类合并展示，agent 调用时同名 custom 优先。

---

## 1. 数据层

### 目录结构

```
dataDir/
  skills_builtin/       ← server 启动时从 embed 写入，强制覆盖
    cron/SKILL.md
    monitor/SKILL.md
    nginx/SKILL.md
    network/SKILL.md
    process/SKILL.md
  skills/               ← 用户自定义，CRUD API 管理
    my-skill/SKILL.md
    cron/SKILL.md       ← 同名 builtin 也存在，两条都显示
```

### SkillEntry 扩展

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

### SkillManager 扩展

```go
type SkillManager struct {
    builtinDir string // {dataDir}/skills_builtin
    customDir  string // {dataDir}/skills
}

func NewSkillManager(dataDir string) *SkillManager
```

### LoadSkills() 逻辑

1. 扫 `builtinDir` → 标 `Source: "builtin"`
2. 扫 `customDir` → 标 `Source: "custom"`
3. 合并两个列表（同名不去重，两条都保留）
4. 排序：按 `Name` 升序，同名时 `custom` 排前面

### RenderList() 优先级

同名 skill 注入 system prompt 时，只注入 custom 版本（builtin 被遮蔽）。

### 启动同步

```go
// SyncBuiltinSkills 从 embed.FS 强制写入 skills_builtin/
// 每次启动执行，覆盖已有文件
func SyncBuiltinSkills(dataDir string, fs embed.FS) error
```

embed 声明（`cmd/spider/embed.go`）：

```go
//go:embed all:skills
var builtinSkillsFS embed.FS
```

---

## 2. API 层

### 路由

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/v1/skills` | 返回合并列表（builtin + custom） |
| `GET` | `/api/v1/skills/{source}/{name}` | 读指定 source 的 skill，source = builtin \| custom |
| `PUT` | `/api/v1/skills/custom/{name}` | 写 custom skill |
| `DELETE` | `/api/v1/skills/custom/{name}` | 删 custom skill |

### skillInfo 扩展

```go
type skillInfo struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Status      string `json:"status"`
    Error       string `json:"error,omitempty"`
    Source      string `json:"source"` // "builtin" | "custom"
}
```

### 错误行为

- `DELETE /api/v1/skills/custom/{name}`：skill 不存在返回 404
- `GET /api/v1/skills/{source}/{name}`：source 非 builtin/custom 返回 400
- builtin skill 无 DELETE/PUT 路由，尝试写入返回 405

---

## 3. UI 层

### 列表展示

- 合并列表，按 name 升序排序
- 同名时 custom 排在 builtin 前面
- builtin 条目显示 🔒 锁图标

### 操作权限

| 状态 | 删除 | 编辑 | 复制 |
|------|------|------|------|
| builtin 🔒 | 禁用 | 禁用 | ✓ |
| custom | ✓ | ✓ | ✓ |

### "复制"流程

1. 点击复制 → `GET /api/v1/skills/builtin/{name}` 拉取内容
2. 打开编辑器，预填内容，name 可修改
3. 保存 → `PUT /api/v1/skills/custom/{name}`

---

## 4. 不在范围内

- install/skills.tar.gz 只打包 `skills/`（custom），不含 builtin
- 不保留旧路由（`/api/v1/skills/{name}` 无 source 前缀）
- builtin skill 不支持在线编辑（复制后变 custom 再编辑）
