---
title: Spec: 目录结构优化
date: 2026-04-17
status: draft
---

# Spec: 目录结构优化

## 1. 目标

清理 spider.ai 项目目录，消除四类问题：
1. 编译产物混入 git 追踪（`bin/`、根目录 `spider` 二进制）
2. 前端嵌入路径绕（`cmd/spider/web/dist`）
3. `docs/` 结构混乱（工具产物混入文档）
4. 根目录杂文件（`excalidraw.log` 等）

**目标用户：** 项目开发者，改善日常 `git status` / `git diff` 体验。

---

## 2. 技术约束

Go `//go:embed` 不允许路径中含 `..`，因此无法从 `cmd/spider/` 嵌入模块根目录的 `web/dist`。
最简方案：将嵌入目录从 `cmd/spider/web/dist` 改为 `cmd/spider/dist`（去掉中间的 `web/` 层）。

---

## 3. 变更清单

### 3.1 从 git 移除编译产物

| 操作 | 对象 |
|------|------|
| `git rm --cached` | `bin/spider`, `bin/spd`, `bin/spider-darwin-*`, `bin/spd-darwin-*`, `bin/spider-linux-amd64` |
| `git rm --cached` | 根目录 `spider`（二进制文件） |
| 更新 `.gitignore` | 确认 `bin/**` 和 `/spider` 已覆盖 |

### 3.2 前端嵌入路径简化

| 文件 | 变更 |
|------|------|
| `web/vite.config.ts` | `outDir` 从 `../cmd/spider/web/dist` 改为 `../cmd/spider/dist` |
| `cmd/spider/embed.go` | `//go:embed all:web/dist` 改为 `//go:embed all:dist` |
| `cmd/spider/main.go` | `fs.Sub(webFS, "web/dist")` 改为 `fs.Sub(webFS, "dist")` |
| `.gitignore` | `cmd/spider/web/dist/` 改为 `cmd/spider/dist/` |
| `cmd/spider/web/` | 删除空目录（仅含 `dist/`，已 gitignore） |

### 3.3 根目录杂文件

| 操作 | 对象 |
|------|------|
| 追加 `.gitignore` | `excalidraw.log`、`*.log`（根目录日志） |

### 3.4 docs/ 结构

| 操作 | 对象 |
|------|------|
| 追加 `.gitignore` | `docs/superpowers/`（工具产物，非项目文档） |

---

## 4. 目标目录结构

```
spider.ai/
├── cmd/
│   ├── spider/
│   │   ├── dist/          ← 前端构建产物（gitignore）
│   │   ├── embed.go       ← //go:embed all:dist
│   │   └── main.go
│   └── spd/
│       └── main.go
├── internal/              ← Go 内部包（不变）
├── web/                   ← 前端源码（不变）
│   ├── src/
│   ├── vite.config.ts     ← outDir 改为 ../cmd/spider/dist
│   └── ...
├── installer/             ← 安装脚本（不变）
├── skills/                ← Claude Code skills（不变）
├── docs/                  ← 项目文档
│   ├── 01_PRD_需求规格说明书.md
│   ├── 02_ARCH_总体架构设计.md
│   └── spec-*.md / plan-*.md
├── bin/                   ← 本地构建输出（gitignore，不提交）
├── dist/                  ← 发布包（gitignore，不提交）
├── .spider/               ← 部署配置（gitignore，不提交）
├── go.mod / go.sum
├── Makefile
└── CLAUDE.md
```

---

## 5. 验收标准

- [ ] `git status` 不再显示 `bin/` 下的二进制文件
- [ ] `git status` 不再显示根目录 `spider` 二进制
- [ ] `make web && make spider-only` 构建成功，Web UI 可正常访问
- [ ] `cmd/spider/web/` 目录不再存在
- [ ] `excalidraw.log` 不被 git 追踪
- [ ] `docs/superpowers/` 不被 git 追踪

---

## 6. 边界

- **Always：** 只改路径和 gitignore，不改业务逻辑
- **Ask first：** Makefile 中 `WEB_DIST` 变量是否需要同步更新
- **Never：** 不删除任何源码文件；不修改 `internal/` 包结构
