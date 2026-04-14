# Install to Claude Code — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Spider Web Dashboard 新增 `/install` 页面，提供一键复制的安装脚本链接，脚本自动安装 Skills 并配置 MCP Server；同时支持管理员通过页面上传/删除自定义 skills。

**Architecture:** 后端新增 `/install.sh`、`/api/v1/install/skills.tar.gz`、`/api/v1/skills` 端点；skills 来源分两层：embed.FS 内嵌（只读）+ `<data_dir>/skills/` 磁盘（可写），磁盘同名覆盖内嵌；前端 `InstallView.vue` 包含安装区和 skills 管理区。

**Tech Stack:** Go 1.21+, `embed.FS`, `archive/tar`, `compress/gzip`, `text/template`, `os`; Vue 3, TypeScript, Vue Router

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `cmd/spider/embed.go` | 修改 | 追加 `skillsFS embed.FS` |
| `internal/api/install.go` | 新建 | `/install.sh` 和 `/api/v1/install/skills.tar.gz` handler |
| `internal/api/skills.go` | 新建 | `/api/v1/skills` CRUD handler（列出/上传/删除） |
| `internal/api/handler.go` | 修改 | 注册所有新路由，接收 `skillsFS` 和 `dataDir` |
| `cmd/spider/main.go` | 修改 | 将 `skillsFS` 传给 `api.NewRouter` |
| `web/src/views/InstallView.vue` | 新建 | 安装页面 + skills 管理区 |
| `web/src/main.ts` | 修改 | 注册 `/install` 路由 |
| `web/src/App.vue` | 修改 | 导航栏加"安装"链接 |

---

## Task 1: embed skills 目录

**Files:**
- Modify: `cmd/spider/embed.go`

- [ ] **Step 1: 追加 skillsFS embed 指令**

将 `cmd/spider/embed.go` 改为：

```go
package main

import "embed"

//go:embed all:web/dist
var webFS embed.FS

//go:embed .claude/skills
var skillsFS embed.FS
```

- [ ] **Step 2: 验证编译通过**

```bash
cd /Users/cw/fty.ai/spider.ai
go build ./cmd/spider/...
```

Expected: 无报错输出。

- [ ] **Step 3: Commit**

```bash
git add cmd/spider/embed.go
git commit -m "feat: embed .claude/skills into binary"
```

---

## Task 2: 后端 install handler（/install.sh + skills.tar.gz）

**Files:**
- Create: `internal/api/install.go`

- [ ] **Step 1: 新建 `internal/api/install.go`（前半部分）**

```go
package api

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const installScriptTmpl = `#!/bin/sh
SPIDER_URL="{{.BaseURL}}"
SKILLS_DIR="$HOME/.claude/plugins/spider"
SETTINGS="$HOME/.claude/settings.json"

set -e

echo "Installing Spider Skills..."
mkdir -p "$SKILLS_DIR"
curl -fsSL "$SPIDER_URL/api/v1/install/skills.tar.gz" | tar -xz -C "$SKILLS_DIR"

echo "Configuring MCP Server..."
if command -v node >/dev/null 2>&1; then
  node -e "
    const fs=require('fs'),p='$SETTINGS';
    const c=fs.existsSync(p)?JSON.parse(fs.readFileSync(p,'utf8')):{};
    c.mcpServers=Object.assign({},c.mcpServers,{spider:{type:'http',url:'$SPIDER_URL/mcp'}});
    fs.writeFileSync(p,JSON.stringify(c,null,2));
  "
elif command -v python3 >/dev/null 2>&1; then
  python3 -c "
import json,os
p='$SETTINGS'
c=json.load(open(p)) if os.path.exists(p) else {}
c.setdefault('mcpServers',{})['spider']={'type':'http','url':'$SPIDER_URL/mcp'}
json.dump(c,open(p,'w'),indent=2)
  "
else
  echo 'Error: node or python3 is required' >&2; exit 1
fi

echo "Done. Restart Claude Code to activate spider MCP."
`

var installTmpl = template.Must(template.New("install").Parse(installScriptTmpl))

// InstallScriptHandler 返回动态生成的安装脚本。
func InstallScriptHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_ = installTmpl.Execute(w, map[string]string{"BaseURL": baseURL})
	}
}
```

- [ ] **Step 2: 追加 skillsTarGzHandler（同文件末尾）**

```go
// SkillsTarGzHandler 将内嵌 skills 与磁盘 skills 合并打包为 tar.gz 流。
// 磁盘同名 skill 覆盖内嵌版本。
func SkillsTarGzHandler(skillsFS embed.FS, dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", `attachment; filename="skills.tar.gz"`)

		gz := gzip.NewWriter(w)
		tw := tar.NewWriter(gz)

		// 收集内嵌 skills（key = skill名，value = 内容）
		embedded := map[string][]byte{}
		root := ".claude/skills"
		_ = fs.WalkDir(skillsFS, root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel := strings.TrimPrefix(path, root+"/")
			data, err := skillsFS.ReadFile(path)
			if err != nil {
				return err
			}
			embedded[rel] = data
			return nil
		})

		// 收集磁盘 skills，覆盖同名内嵌
		diskDir := filepath.Join(dataDir, "skills")
		if entries, err := os.ReadDir(diskDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					skillFile := filepath.Join(diskDir, e.Name(), "SKILL.md")
					data, err := os.ReadFile(skillFile)
					if err == nil {
						embedded[e.Name()+"/SKILL.md"] = data
					}
				}
			}
		}

		// 写入 tar
		for rel, data := range embedded {
			hdr := &tar.Header{Name: rel, Mode: 0644, Size: int64(len(data))}
			if err := tw.WriteHeader(hdr); err != nil {
				return
			}
			_, _ = tw.Write(data)
		}

		_ = tw.Close()
		_ = gz.Close()
	}
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./internal/api/...
```

Expected: 无报错。

- [ ] **Step 4: Commit**

```bash
git add internal/api/install.go
git commit -m "feat: add InstallScriptHandler and SkillsTarGzHandler"
```

---

## Task 3: 后端 skills CRUD handler

**Files:**
- Create: `internal/api/skills.go`

- [ ] **Step 1: 新建 `internal/api/skills.go`**

```go
package api

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type skillInfo struct {
	Name   string `json:"name"`
	Source string `json:"source"` // "embedded" | "custom"
}

// listSkillsHandler 列出所有 skills（内嵌 + 磁盘，磁盘同名标记为 custom）。
func listSkillsHandler(skillsFS embed.FS, dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seen := map[string]string{} // name -> source

		// 内嵌 skills
		root := ".claude/skills"
		_ = fs.WalkDir(skillsFS, root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, "/SKILL.md") {
				return err
			}
			name := strings.TrimSuffix(strings.TrimPrefix(path, root+"/"), "/SKILL.md")
			seen[name] = "embedded"
			return nil
		})

		// 磁盘 skills 覆盖
		diskDir := filepath.Join(dataDir, "skills")
		if entries, err := os.ReadDir(diskDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					if _, err := os.Stat(filepath.Join(diskDir, e.Name(), "SKILL.md")); err == nil {
						seen[e.Name()] = "custom"
					}
				}
			}
		}

		result := make([]skillInfo, 0, len(seen))
		for name, source := range seen {
			result = append(result, skillInfo{Name: name, Source: source})
		}
		writeJSON(w, http.StatusOK, result)
	}
}
```

- [ ] **Step 2: 追加 uploadSkillHandler 和 deleteSkillHandler（同文件末尾）**

```go
// uploadSkillHandler 上传单个 skill 的 SKILL.md 内容，存到 <dataDir>/skills/<name>/SKILL.md。
func uploadSkillHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" || strings.ContainsAny(name, "/\\..") {
			writeError(w, http.StatusBadRequest, "invalid skill name")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
		if err != nil || len(body) == 0 {
			writeError(w, http.StatusBadRequest, "empty body")
			return
		}
		dir := filepath.Join(dataDir, "skills", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "mkdir failed")
			return
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), body, 0644); err != nil {
			writeError(w, http.StatusInternalServerError, "write failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"name": name, "source": "custom"})
	}
}

// deleteSkillHandler 删除磁盘上的自定义 skill（内嵌 skill 不可删）。
func deleteSkillHandler(skillsFS embed.FS, dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(w, http.StatusBadRequest, "missing skill name")
			return
		}
		dir := filepath.Join(dataDir, "skills", name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "custom skill not found")
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// needed for json import
var _ = json.Marshal
```

- [ ] **Step 3: 验证编译**

```bash
go build ./internal/api/...
```

Expected: 无报错。

- [ ] **Step 4: Commit**

```bash
git add internal/api/skills.go
git commit -m "feat: add skills CRUD handlers (list/upload/delete)"
```

---

## Task 4: 注册所有新路由

**Files:**
- Modify: `internal/api/handler.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: 修改 `NewRouter` 签名，接收 skillsFS 和 dataDir**

将 `internal/api/handler.go` 顶部 import 改为：

```go
import (
	"embed"
	"encoding/json"
	"net/http"

	mcppkg "github.com/spiderai/spider/internal/mcp"
)
```

将 `NewRouter` 签名改为：

```go
func NewRouter(app *mcppkg.App, skillsFS embed.FS) http.Handler {
```

在 `return mux` 前追加：

```go
mux.HandleFunc("GET /api/v1/install/skills.tar.gz", SkillsTarGzHandler(skillsFS, app.Config.DataDir))
mux.HandleFunc("GET /api/v1/skills", listSkillsHandler(skillsFS, app.Config.DataDir))
mux.HandleFunc("PUT /api/v1/skills/{name}", uploadSkillHandler(app.Config.DataDir))
mux.HandleFunc("DELETE /api/v1/skills/{name}", deleteSkillHandler(skillsFS, app.Config.DataDir))
```

- [ ] **Step 2: 修改 `cmd/spider/main.go`**

找到：

```go
mux.Handle("/api/", apipkg.NewRouter(app))
```

改为：

```go
mux.HandleFunc("/install.sh", apipkg.InstallScriptHandler(app.Config.SSE.BaseURL))
mux.Handle("/api/", apipkg.NewRouter(app, skillsFS))
```

- [ ] **Step 3: 验证编译**

```bash
go build ./...
```

Expected: 无报错。

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler.go cmd/spider/main.go
git commit -m "feat: register install and skills routes"
```

---

## Task 5: 手动验证后端

- [ ] **Step 1: 构建并启动**

```bash
go build -o ./spider ./cmd/spider && ./spider --addr :8000
```

- [ ] **Step 2: 验证 /install.sh**

```bash
curl -s http://localhost:8000/install.sh | head -3
```

Expected:
```
#!/bin/sh
SPIDER_URL="http://localhost:8000"
```

- [ ] **Step 3: 验证 skills 列表**

```bash
curl -s http://localhost:8000/api/v1/skills | python3 -m json.tool
```

Expected: JSON 数组，包含 `spider-deploy`、`ui-style`，source 均为 `"embedded"`。

- [ ] **Step 4: 验证上传 skill**

```bash
curl -s -X PUT http://localhost:8000/api/v1/skills/my-test-skill \
  -d '---\nname: my-test-skill\n---\nTest skill content.'
curl -s http://localhost:8000/api/v1/skills | python3 -m json.tool
```

Expected: 列表中出现 `my-test-skill`，source 为 `"custom"`。

- [ ] **Step 5: 验证 skills.tar.gz 包含自定义 skill**

```bash
curl -s http://localhost:8000/api/v1/install/skills.tar.gz | tar -tz
```

Expected: 包含 `my-test-skill/SKILL.md`。

- [ ] **Step 6: 验证删除 skill**

```bash
curl -s -X DELETE http://localhost:8000/api/v1/skills/my-test-skill
curl -s http://localhost:8000/api/v1/skills | python3 -m json.tool
```

Expected: `my-test-skill` 从列表消失。

- [ ] **Step 7: 停止服务**

`Ctrl+C`

---

## Task 6: 前端 InstallView.vue

**Files:**
- Create: `web/src/views/InstallView.vue`

- [ ] **Step 1: 新建 `web/src/views/InstallView.vue`（template + script）**

```vue
<template>
  <div class="page-content">
    <div class="page-header"><h2>安装到 Claude Code</h2></div>

    <!-- 安装区 -->
    <div class="settings-card">
      <h3>一键安装</h3>
      <p class="dim" style="margin-bottom:16px;font-size:14px;">
        在终端运行以下命令，自动安装 Spider Skills 并配置 MCP Server：
      </p>
      <div class="copy-row">
        <code class="copy-cmd">{{ curlCmd }}</code>
        <button class="btn btn-primary btn-sm" @click="copy">{{ copied ? '已复制 ✓' : '复制' }}</button>
      </div>
      <ul class="install-checklist">
        <li>✓ 安装 Spider Skills 到 <code>~/.claude/plugins/spider/</code></li>
        <li>✓ 配置 MCP Server 到 <code>~/.claude/settings.json</code></li>
      </ul>
    </div>

    <!-- 脚本预览 -->
    <div class="settings-card">
      <div style="cursor:pointer;" @click="showScript = !showScript">
        <h3 style="margin-bottom:0;">查看安装脚本 {{ showScript ? '▲' : '▼' }}</h3>
      </div>
      <pre v-if="showScript" class="output" style="margin-top:16px;max-height:400px;">{{ scriptContent || '加载中...' }}</pre>
    </div>

    <!-- Skills 管理 -->
    <div class="settings-card">
      <div class="page-header" style="margin-bottom:16px;">
        <h3 style="margin-bottom:0;">Skills 管理</h3>
        <button class="btn btn-primary btn-sm" @click="triggerUpload(null)">添加 Skill</button>
      </div>
      <input ref="fileInput" type="file" accept=".md" style="display:none" @change="onFileChange" />
      <table class="table">
        <thead><tr><th>名称</th><th>来源</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="s in skills" :key="s.name">
            <td class="code">{{ s.name }}</td>
            <td><span :class="s.source === 'custom' ? 'badge' : 'dim'">{{ s.source === 'custom' ? '自定义' : '内嵌' }}</span></td>
            <td class="actions">
              <button class="btn btn-sm" @click="triggerUpload(s.name)">上传新版本</button>
              <button v-if="s.source === 'custom'" class="btn btn-sm btn-danger" @click="deleteSkill(s.name)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 追加 script setup（同文件）**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'

interface Skill { name: string; source: 'embedded' | 'custom' }

const curlCmd = `curl -fsSL ${window.location.origin}/install.sh | sh`
const copied = ref(false)
const showScript = ref(false)
const scriptContent = ref('')
const skills = ref<Skill[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
let uploadTarget = ref<string | null>(null)

function copy() {
  navigator.clipboard.writeText(curlCmd)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

async function loadSkills() {
  const res = await fetch('/api/v1/skills')
  skills.value = await res.json()
  skills.value.sort((a, b) => a.name.localeCompare(b.name))
}

function triggerUpload(name: string | null) {
  uploadTarget.value = name
  fileInput.value?.click()
}

async function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const name = uploadTarget.value ?? file.name.replace(/\.md$/, '')
  const body = await file.text()
  await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, { method: 'PUT', body })
  ;(e.target as HTMLInputElement).value = ''
  await loadSkills()
}

async function deleteSkill(name: string) {
  if (!confirm(`删除自定义 skill "${name}"？`)) return
  await fetch(`/api/v1/skills/${encodeURIComponent(name)}`, { method: 'DELETE' })
  await loadSkills()
}

onMounted(async () => {
  const res = await fetch('/install.sh')
  scriptContent.value = await res.text()
  await loadSkills()
})
</script>
```

- [ ] **Step 3: 追加 scoped style（同文件）**

```vue
<style scoped>
.copy-row {
  display: flex; align-items: center; gap: 12px;
  background: var(--panel); border: 1px solid var(--border);
  border-radius: 8px; padding: 10px 14px; margin-bottom: 16px;
}
.copy-cmd {
  flex: 1; font-family: 'SF Mono', Consolas, monospace;
  font-size: 13px; color: var(--text); word-break: break-all;
}
.install-checklist {
  list-style: none; display: flex; flex-direction: column;
  gap: 6px; font-size: 14px; color: var(--text-sub);
}
.install-checklist code {
  font-family: 'SF Mono', Consolas, monospace; font-size: 12px;
  background: var(--surface); padding: 1px 5px; border-radius: 4px;
}
</style>
```

- [ ] **Step 4: Commit**

```bash
git add web/src/views/InstallView.vue
git commit -m "feat: add InstallView with skills management"
```

---

## Task 7: 注册路由 + 导航栏

**Files:**
- Modify: `web/src/main.ts`
- Modify: `web/src/App.vue`

- [ ] **Step 1: 在 `web/src/main.ts` 注册 /install 路由**

完整 routes 数组：

```ts
routes: [
  { path: '/', redirect: '/hosts' },
  { path: '/hosts',    component: () => import('./views/HostsView.vue') },
  { path: '/exec',     component: () => import('./views/ExecView.vue') },
  { path: '/audit',    component: () => import('./views/AuditView.vue') },
  { path: '/settings', component: () => import('./views/SettingsView.vue') },
  { path: '/install',  component: () => import('./views/InstallView.vue') },
],
```

- [ ] **Step 2: 在 `web/src/App.vue` 导航栏加"安装"链接**

找到：

```html
<RouterLink to="/settings" class="nav-item">设置</RouterLink>
```

在其后加：

```html
<RouterLink to="/install" class="nav-item">安装</RouterLink>
```

- [ ] **Step 3: 构建前端**

```bash
cd web && npm run build
```

Expected: `web/dist/` 生成，无报错。

- [ ] **Step 4: Commit**

```bash
cd ..
git add web/src/main.ts web/src/App.vue web/dist
git commit -m "feat: add /install route and nav link"
```

---

## Task 8: 端到端验证

- [ ] **Step 1: 构建完整二进制**

```bash
go build -o ./spider ./cmd/spider
```

- [ ] **Step 2: 启动服务**

```bash
./spider --addr :8000
```

- [ ] **Step 3: 浏览器验证安装区**

打开 `http://localhost:8000/install`：
- 显示 `curl -fsSL http://localhost:8000/install.sh | sh`
- 点击"复制"→ 变为"已复制 ✓"
- 展开脚本预览，内容正确

- [ ] **Step 4: 浏览器验证 skills 管理**

- Skills 表格显示 `spider-deploy`、`ui-style`，来源为"内嵌"
- 点击"添加 Skill"，选择一个 `.md` 文件，上传后表格出现新行，来源为"自定义"
- 点击"上传新版本"，替换已有 skill
- 点击"删除"，自定义 skill 消失；内嵌 skill 无删除按钮

- [ ] **Step 5: 验证安装脚本执行**

```bash
ORIG_HOME=$HOME
export HOME=$(mktemp -d)
curl -fsSL http://localhost:8000/install.sh | sh
ls "$HOME/.claude/plugins/spider/"
cat "$HOME/.claude/settings.json"
export HOME=$ORIG_HOME
```

Expected:
- `ls` 包含 `spider-deploy`、`ui-style`
- `settings.json` 含 `"spider": {"type": "http", "url": "http://localhost:8000/mcp"}`

- [ ] **Step 6: 验证不覆盖已有配置**

```bash
ORIG_HOME=$HOME
export HOME=$(mktemp -d)
mkdir -p "$HOME/.claude"
echo '{"theme":"dark","mcpServers":{"other":{"type":"http","url":"http://other/mcp"}}}' > "$HOME/.claude/settings.json"
curl -fsSL http://localhost:8000/install.sh | sh
cat "$HOME/.claude/settings.json"
export HOME=$ORIG_HOME
```

Expected: `other` 和 `spider` 均在 mcpServers，`theme` 保留。

- [ ] **Step 7: 最终 commit**

```bash
git add -A
git status
git commit -m "feat: install-to-claude-code complete — /install page, skills CRUD, /install.sh"
```
