# 模块日志级别配置 — 设计文档

**日期：** 2026-05-17  
**位置：** 系统设置 → 偏好设置 → 日志 card

---

## 背景

后端已支持 per-module 动态日志级别（`/api/v1/log-level`，`module` 字段），但前端偏好设置只有全局级别下拉，无法配置各模块的覆盖级别。

---

## 目标

在偏好设置「日志」card 中，全局级别下方增加模块级别覆盖表格，支持查看和编辑各模块的日志级别。

---

## 已知模块

固定列表（与后端 `ForModule` 调用一致）：

| 模块 | 说明 |
|------|------|
| `main` | 主进程 |
| `scheduler` | 任务调度器 |
| `agent` | Agent 执行 |
| `mcp` | MCP Server |
| `ssh` | SSH 客户端 |

---

## 级别映射

| 英文值（API） | 中文显示 |
|--------------|---------|
| `inherit`    | 继承     |
| `debug`      | 调试 debug |
| `info`       | 信息 info  |
| `warn`       | 警告 warn  |
| `error`      | 错误 error |

只读 badge 只显示中文（继承 / 调试 / 信息 / 警告 / 错误）。  
编辑下拉显示「中文 英文」格式，`继承` 选项无英文后缀。

---

## UI 设计

### 只读视图（`settingsEditing === false`）

日志 card 结构：

```
全局级别   [信息]（彩色 badge）

模块级别覆盖
模块        级别
main        继承（灰色 badge）
scheduler   调试（绿色 badge）
agent       继承
mcp         警告（橙色 badge）
ssh         继承
```

Badge 颜色：继承=灰、调试=绿、信息=蓝、警告=橙、错误=红。

### 编辑视图（`settingsEditing === true`）

```
全局级别   [信息 info ▼]

模块级别覆盖
模块        级别
main        [继承 ▼]
scheduler   [调试 debug ▼]
agent       [继承 ▼]
mcp         [警告 warn ▼]
ssh         [继承 ▼]
```

---

## 数据流

### 加载（`loadSettings`）

`GET /api/v1/log-level` 返回：
```json
{ "level": "info", "modules": { "scheduler": "debug", "mcp": "warn" } }
```

前端初始化 `moduleLevels` ref（`Record<string, string>`），未出现的模块默认 `"inherit"`。

### 保存（`saveSettings`）

对每个模块，若值为 `"inherit"` 则发 `{ module, level: "inherit" }`（后端 `ClearModuleLevel`），否则发 `{ module, level }`。

全量发送 5 个模块（简单，避免 diff 逻辑）。

---

## 实现范围

仅修改 `web/src/views/ProfileView.vue`：

1. 新增 `moduleLevels` ref：`ref<Record<string, string>>({})` 
2. `loadSettings` — 从 `lvlData.modules` 填充 `moduleLevels`，未出现的模块设为 `"inherit"`
3. `saveSettings` — 循环 5 个模块，逐个 PUT `/api/v1/log-level`
4. 只读视图 — 日志 card 加模块表格（彩色 badge）
5. 编辑视图 — 日志 card 加模块下拉行

不修改后端。

---

## Mockup

`docs/superpowers/plans/mockup-log-modules-zh.png`（见浏览器截图）
