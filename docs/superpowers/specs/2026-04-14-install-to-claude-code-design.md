# 设计文档：安装到 Claude Code

**日期：** 2026-04-14
**状态：** 已批准

---

## 1. 目标

在 Spider Web Dashboard 新增 `/install` 页面，用户访问后可一键复制安装脚本链接。
安装脚本自动完成两件事：
1. 安装 Spider Skills 到 `~/.claude/plugins/spider/`
2. 将 Spider MCP Server 配置写入 `~/.claude/settings.json`

---

## 2. 架构

### 2.1 后端新增端点

| 端点 | 说明 |
|------|------|
| `GET /install.sh` | 动态生成 shell 脚本，填入当前服务 BaseURL |
| `GET /api/v1/install/skills.tar.gz` | 提供 skills 压缩包下载 |
| `GET /api/v1/skills` | 列出所有 skills |
| `PUT /api/v1/skills/:name` | 上传/更新单个 skill |
| `DELETE /api/v1/skills/:name` | 删除自定义 skill |

Skills 存储在 `<data_dir>/skills/` 磁盘目录，**不内嵌进二进制**，随 spider 二进制一起发布。管理员可通过 Web 界面上传/更新/删除 skills。

### 2.2 前端新增页面

- 路由：`/install`
- 组件：`web/src/views/InstallView.vue`
- 导航栏新增"安装"入口

### 2.3 安装脚本逻辑

```
1. mkdir -p ~/.claude/plugins/spider/
2. curl 下载 skills.tar.gz 并解压到上述目录
3. 合并写入 ~/.claude/settings.json：
   mcpServers.spider = { type: "http", url: "$SPIDER_URL/mcp" }
   （用 node 或 python3 做 JSON 合并，不覆盖已有配置）
4. 打印成功提示
```

---

## 3. 详细设计

### 3.1 后端：`/install.sh`

- Handler 读取 `app.Config.SSE.BaseURL` 填入脚本模板
- Content-Type: `text/plain; charset=utf-8`
- 脚本模板硬编码在 Go 源码中（`const installScriptTmpl`）

### 3.2 后端：`/api/v1/install/skills.tar.gz`

- 遍历 `<data_dir>/skills/` 磁盘目录，动态打包为 tar.gz 流
- Content-Type: `application/gzip`
- 每次请求实时打包

### 3.3 Skills 存储

Skills 存储在 `<data_dir>/skills/<name>/SKILL.md`，不内嵌进二进制。
随 spider 二进制一起发布（如 `spider` + `skills/` 目录）。

### 3.4 前端页面

- `curl` 命令：`curl -fsSL ${window.location.origin}/install.sh | sh`（纯前端拼接，无 API 调用）
- 脚本预览：`fetch('/install.sh')` 拉取内容，折叠默认隐藏，展开后代码高亮
- 复制按钮：点击后 2 秒内显示"已复制 ✓"

---

## 4. 安装脚本完整逻辑

```sh
#!/bin/sh
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
```

---

## 5. 文件变更清单

| 文件 | 变更 |
|------|------|
| `internal/api/install.go` | 新建，实现 `InstallScriptHandler` 和 `SkillsTarGzHandler` |
| `internal/api/skills.go` | 新建，实现 skills CRUD handler |
| `internal/api/handler.go` | 注册所有新路由 |
| `cmd/spider/main.go` | 注册 `/install.sh` 路由 |
| `web/src/views/InstallView.vue` | 新建安装页面 + skills 管理区 |
| `web/src/main.ts` | 注册 `/install` 路由 |
| `web/src/App.vue` | 导航栏加"安装"链接 |

---

## 6. 成功标准

- [ ] 访问 `/install` 页面，显示正确的 `curl` 命令（含当前服务地址）
- [ ] 点击复制按钮，命令复制到剪贴板
- [ ] 展开脚本预览，内容与 `/install.sh` 一致
- [ ] 执行安装脚本后，`~/.claude/plugins/spider/` 下有 skills 文件
- [ ] 执行安装脚本后，`~/.claude/settings.json` 中有 `mcpServers.spider` 配置
- [ ] 已有 `settings.json` 的情况下，安装脚本不覆盖其他配置

---

## 7. 不在本期范围

- 安装脚本支持 Windows（PowerShell）
- 安装脚本验证 skills 完整性（checksum）
- 卸载脚本
