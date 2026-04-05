# Spider 智能运维平台 — 产品需求规格说明书

**版本：** v0.1  
**日期：** 2026-04-05  
**状态：** 草稿

---

## 目录

1. [产品概述](#1-产品概述)
2. [使用场景与用户故事](#2-使用场景与用户故事)
3. [功能需求规格](#3-功能需求规格)
4. [非功能性需求](#4-非功能性需求)
5. [系统架构](#5-系统架构)
6. [数据模型](#6-数据模型)
7. [接口规范](#7-接口规范)
8. [安全设计](#8-安全设计)
9. [部署方案](#9-部署方案)
10. [产品路线图](#10-产品路线图)

---

## 1. 产品概述

### 1.1 产品定位与核心价值

Spider 是一个以 Claude Code 为中心的 SSH 代理网关和智能运维平台。

**核心价值主张：**

- **AI 原生运维**：Claude Code 通过 MCP 接口直接完成运维操作，无需人工 SSH 登录，将自然语言指令转化为实际的远程操作
- **凭据安全隔离**：Claude 始终只看到主机 ID/名称，不接触任何密码或密钥，凭据在 spider 进程内加密存储和解密
- **零摩擦部署**：单二进制，无外部依赖，个人工程师 5 分钟内完成安装和接入
- **团队协作就绪**：支持多用户、权限控制和操作审计，可作为团队共享服务部署

### 1.2 目标用户画像

**用户 A — 个人运维工程师**

- 独立管理数台至数十台服务器
- 日常使用 Claude Code 辅助编码和运维
- 希望通过 AI 减少重复性 SSH 操作
- 对安全性有基本要求，不希望 AI 直接接触服务器密码

**用户 B — 团队 DevOps / SRE**

- 所在团队有 3~20 名工程师共同管理服务器资产
- 需要统一的主机资产管理和权限控制
- 需要操作审计，满足合规要求
- 希望团队成员都能通过 AI 完成标准化运维操作

### 1.3 设计原则

1. **安全第一**：凭据永不暴露给 AI 模型，所有敏感数据加密存储
2. **简单部署**：单二进制，零外部依赖，配置最小化
3. **AI 原生**：MCP 接口是一等公民，工具设计以 Claude 的使用方式为准
4. **渐进增强**：个人用户开箱即用，团队功能按需启用
5. **可审计**：所有操作留有记录，支持事后追溯

---

## 2. 使用场景与用户故事

### 2.1 批量巡检

**场景描述：** 运维工程师需要定期检查所有生产主机的资源使用情况。

**用户故事：**
> 作为运维工程师，我希望对话一句话就能获得所有生产主机的磁盘、内存、CPU 使用率汇总，而不需要逐台 SSH 登录。

**交互示例：**
```
用户：帮我检查所有生产主机的磁盘、内存和 CPU 使用率
Claude：[调用 execute_command_batch，tag=prod，命令 df -h / free -m / top -bn1]
        [汇总分析结果，标注异常主机]
```

### 2.2 服务健康检查

**场景描述：** 快速确认多台主机上关键服务的运行状态。

**用户故事：**
> 作为 SRE，我希望一次性检查所有 web 节点上 nginx 和 redis 的运行状态，立刻定位异常节点。

**交互示例：**
```
用户：检查所有 web 节点上 nginx 和 redis 是否正常运行
Claude：[批量执行 systemctl status nginx redis，聚合输出]
        [标注 web-03 上 redis 未运行，给出重启建议]
```

### 2.3 应用部署

**场景描述：** 将新版本二进制部署到多台应用服务器。

**用户故事：**
> 作为开发工程师，我希望通过一条指令完成多台服务器的应用更新，包括上传、停服、替换、重启、验证全流程。

**交互示例：**
```
用户：把 bin/app 部署到所有 app 标签的主机，替换旧版本并重启服务
Claude：[upload_file → exec stop → exec replace → exec start → exec verify]
        [逐台执行，有错误立刻停止并报告]
```

### 2.4 配置分发

**场景描述：** 将配置文件同步到多台服务器并重载服务。

**用户故事：**
> 作为运维工程师，我希望将本地修改好的 nginx.conf 同步到所有 web 主机并安全重载，有错误立刻停止。

**交互示例：**
```
用户：把本地的 nginx.conf 同步到所有 web 主机并 reload
Claude：[逐台 upload_file，执行 nginx -t && systemctl reload nginx]
        [任一主机配置检查失败则停止，不继续后续主机]
```

### 2.5 故障排查

**场景描述：** 快速定位单台主机的性能或服务异常原因。

**用户故事：**
> 作为 SRE，当某台主机响应变慢时，我希望 AI 自动收集诊断信息并给出根因分析，而不需要我手动逐条执行命令。

**交互示例：**
```
用户：web-01 响应变慢，帮我查一下原因
Claude：[自动运行 top / netstat -s / dmesg | tail / 慢查询日志]
        [分析后给出根因：内存不足导致 swap 频繁]
```

### 2.6 日志分析

**场景描述：** 下载远程日志文件并在对话中完成分析。

**用户故事：**
> 作为开发工程师，我希望直接在 Claude 对话中分析远程服务器的错误日志，无需手动 scp。

**交互示例：**
```
用户：下载 app-01 最近的错误日志并分析
Claude：[download_file 获取日志] → [在对话中分析，给出错误摘要和建议]
```

### 2.7 安全审计

**场景描述：** 批量检查主机上的安全配置。

**用户故事：**
> 作为安全工程师，我希望批量检查所有主机上是否有非标准用户拥有 sudo 权限。

**交互示例：**
```
用户：检查所有主机上是否有非标准用户有 sudo 权限
Claude：[批量读取 /etc/sudoers 和 /etc/sudoers.d/]
        [识别异常账号配置，列出需要关注的主机]
```

### 2.8 执行历史追溯

**场景描述：** 审计特定时间段内在某台主机上的操作记录。

**用户故事：**
> 作为运维负责人，我希望能查询昨天在 db-01 上执行了哪些命令，用于事后审计。

**交互示例：**
```
用户：查一下昨天在 db-01 上执行了哪些命令
Claude：[调用 get_execution_history，过滤 host=db-01，时间范围=昨天]
        [列出所有操作记录，包括命令、执行人、时间、结果]
```

---

## 3. 功能需求规格

### 3.1 基线功能（已实现）

#### 3.1.1 主机管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 添加主机 | 支持 SSH 私钥、密码、带 passphrase 私钥三种认证方式 | P0 |
| 删除主机 | 按名称或 ID 删除，同时清理关联凭据 | P0 |
| 更新主机 | 更新 IP、用户名、认证方式、标签等字段 | P0 |
| 列出主机 | 支持按标签过滤，支持 JSON 格式输出 | P0 |
| 跳板机支持 | 通过 proxy_id 指定跳板机，支持多级跳转 | P1 |
| 标签管理 | 多标签，用于分组和批量操作 | P0 |
| 连通性测试 | SSH ping，返回连接状态和延迟 | P0 |

**认证方式规格：**

| 类型 | 参数 | 说明 |
|------|------|------|
| `key` | `--key <path>` | SSH 私钥文件路径 |
| `password` | `--password <pass>` | SSH 密码 |
| `key_password` | `--key <path> --passphrase <pass>` | 带 passphrase 的私钥 |

#### 3.1.2 SSH 远程执行

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 单台执行 | 在指定主机执行命令，返回 stdout/stderr/exit_code | P0 |
| 批量执行 | 按 tag 或 ID 列表并发执行，聚合结果 | P0 |
| 自定义超时 | 默认 30s，可按命令指定超时时间 | P0 |
| 连接池复用 | SSH 连接池，TTL 可配置（默认 300s） | P1 |

#### 3.1.3 文件传输

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 上传文件 | 本地文件 → 远程主机（SCP） | P0 |
| 下载文件 | 远程主机 → 本地（SCP） | P0 |

#### 3.1.4 执行历史

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 记录执行 | 自动记录所有命令执行（命令、输出、状态、耗时） | P0 |
| 按主机过滤 | 查询指定主机的历史记录 | P0 |
| 分页查询 | 支持指定返回条数（默认 20） | P0 |

#### 3.1.5 MCP 接口（10 个工具）

| 工具名 | 说明 |
|--------|------|
| `list_hosts` | 列出主机，支持 tag 过滤 |
| `add_host` | 添加主机 |
| `remove_host` | 删除主机 |
| `update_host` | 更新主机信息 |
| `execute_command` | 在单台主机执行命令 |
| `execute_command_batch` | 按 tag 或 ID 列表批量执行 |
| `check_connectivity` | 测试 SSH 连通性 |
| `upload_file` | 上传本地文件到远程主机 |
| `download_file` | 从远程主机下载文件 |
| `get_execution_history` | 查询执行历史 |

#### 3.1.6 CLI 工具（spdctl）

| 命令 | 说明 |
|------|------|
| `spdctl host add` | 添加主机 |
| `spdctl host list` | 列出主机 |
| `spdctl host update` | 更新主机 |
| `spdctl host rm` | 删除主机 |
| `spdctl exec <host> <cmd>` | 执行命令 |
| `spdctl ping <host>` | 连通性测试 |
| `spdctl history` | 查看执行历史 |
| `spdctl mcp register` | 注册 MCP 到 Claude Code |
| `spdctl mcp unregister` | 取消注册 |
| `spdctl mcp status` | 查看注册状态 |

### 3.2 Phase 1 — Web UI 完善

目标：提供可视化管理界面，让不熟悉 CLI 的团队成员也能管理主机和查看执行记录。

#### 3.2.1 主机管理界面

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 主机列表 | 表格视图，显示名称、IP、状态、标签、最后连接时间 | P0 |
| 搜索与过滤 | 按名称/IP 搜索，按标签筛选 | P0 |
| 添加主机表单 | 图形化填写主机信息，支持三种认证方式 | P0 |
| 编辑主机 | 在线编辑主机配置 | P0 |
| 删除主机 | 单台删除，带确认弹窗 | P0 |
| 主机详情页 | 基本信息、连通状态、最近执行记录 | P1 |
| 批量操作 | 批量删除、批量打标签 | P1 |
| 连通性测试 | 界面上一键 ping，实时显示结果 | P0 |

#### 3.2.2 实时命令执行界面

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 命令输入框 | 支持多行命令输入 | P0 |
| 目标选择 | 选择单台主机或按标签批量选择 | P0 |
| 实时输出流 | 通过 SSE 实时展示命令输出 | P0 |
| 多主机并行结果 | 批量执行时分 tab 或分栏展示各主机输出 | P1 |
| 超时配置 | 界面上可设置执行超时时间 | P1 |
| 执行状态指示 | 运行中 / 成功 / 失败 / 超时 状态标识 | P0 |

#### 3.2.3 执行历史与日志

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 历史列表 | 显示时间、主机、命令摘要、状态、耗时 | P0 |
| 日志详情 | 查看完整命令输出 | P0 |
| 搜索与过滤 | 按主机、时间范围、状态过滤 | P1 |
| 分页 | 支持分页浏览 | P0 |

#### 3.2.4 文件管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 文件上传 | 拖拽或选择本地文件，指定目标主机和远程路径 | P1 |
| 上传进度 | 显示上传进度条 | P1 |
| 下载记录 | 查看历史下载记录 | P2 |

---

### 3.3 Phase 2 — 多用户与权限控制

目标：支持团队多成员共用一个 spider 实例，实现权限隔离和操作审计。

#### 3.3.1 用户账号管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 本地账号 | 用户名 + 密码登录（bcrypt 哈希存储） | P0 |
| 管理员创建账号 | Admin 可创建、禁用、删除账号 | P0 |
| 修改密码 | 用户可修改自己的密码 | P0 |
| 账号禁用 | Admin 可禁用账号，禁用后立即失效 | P0 |
| 登录会话 | JWT Token，有效期 24h，支持刷新 | P0 |

#### 3.3.2 角色与权限（RBAC）

内置三级角色，不支持自定义角色（YAGNI）：

| 角色 | 权限范围 |
|------|----------|
| `admin` | 全部权限：用户管理、主机管理、执行命令、查看审计日志 |
| `operator` | 主机管理（无删除）、执行命令、查看历史 |
| `viewer` | 只读：查看主机列表、查看执行历史 |

**主机组权限（可选）：** Admin 可限制某 operator/viewer 只能访问特定标签的主机组。

#### 3.3.3 API Token

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 生成 Token | 用户可生成命名 Token，用于 MCP 认证 | P0 |
| 权限范围 | Token 可限定 scopes（如只允许 execute） | P1 |
| 撤销 Token | 立即失效 | P0 |
| Token 列表 | 查看自己创建的所有 Token | P0 |
| 过期时间 | 可设置过期时间，默认永不过期 | P1 |

#### 3.3.4 操作审计

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 记录操作人 | 所有执行记录关联用户 ID | P0 |
| 审计日志不可删除 | 审计记录只追加，不允许删除 | P0 |
| 审计日志查询 | 按用户、时间、操作类型过滤 | P0 |
| 导出审计日志 | 导出为 CSV（Admin 权限） | P2 |

---

### 3.4 Phase 3 — 告警与监控

目标：主动监控主机状态，在异常发生时及时通知相关人员。

#### 3.4.1 主机状态监控

| 功能 | 描述 | 优先级 |
|------|------|--------|
| SSH 连通性检测 | 定期 ping 所有主机，检测在线/离线状态 | P0 |
| 状态变更通知 | 主机从在线变为离线时触发告警 | P0 |
| 检测间隔配置 | 默认 60s，可按主机组配置 | P1 |
| 状态历史 | 记录主机状态变更历史 | P1 |

#### 3.4.2 阈值告警规则

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 规则配置 | 定义检测命令、解析表达式、阈值、比较运算符 | P0 |
| 目标主机 | 按标签指定告警规则适用的主机范围 | P0 |
| 检测频率 | 可配置检测间隔（最小 60s） | P0 |
| 告警状态 | firing（触发）/ resolved（恢复）/ acknowledged（已确认） | P0 |
| 告警确认 | 运维人员可确认告警，避免重复通知 | P1 |
| 规则启用/禁用 | 临时禁用规则而不删除 | P1 |

**内置规则模板：**

| 模板 | 命令 | 默认阈值 |
|------|------|----------|
| 磁盘使用率 | `df -h / \| awk 'NR==2{print $5}'` | > 85% |
| 内存使用率 | `free \| awk '/Mem/{printf "%.0f", $3/$2*100}'` | > 90% |
| CPU 负载 | `uptime \| awk '{print $NF}'` | > 4.0 |

#### 3.4.3 通知渠道

| 渠道 | 配置项 | 优先级 |
|------|--------|--------|
| 钉钉 Webhook | Webhook URL，支持 @ 指定人 | P0 |
| Slack Webhook | Webhook URL，支持 channel 配置 | P0 |
| Email | SMTP 配置，收件人列表 | P1 |
| 通知模板 | 可自定义告警消息模板 | P2 |

通知内容包含：告警规则名、主机名、当前值、阈值、触发时间、快速链接。

#### 3.4.4 监控仪表盘

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 主机状态总览 | 在线/离线/未知主机数量，状态分布图 | P0 |
| 活跃告警列表 | 当前 firing 状态的告警，按严重程度排序 | P0 |
| 执行统计 | 近 7 天执行次数、成功率趋势图 | P1 |
| 主机健康评分 | 综合连通性和告警状态的健康评分 | P2 |

---

## 4. 非功能性需求

### 4.1 性能

| 指标 | 要求 |
|------|------|
| 并发 SSH 连接 | 单实例支持 ≥ 100 台主机并发连接 |
| 批量执行并发度 | 默认 10，可通过配置调整，上限 50 |
| API 响应时间 | 非 SSH 操作 < 200ms（P99） |
| SSH 连接复用 | 连接池 TTL 默认 300s，减少重复握手开销 |
| Web UI 首屏 | 静态资源内嵌二进制，首屏加载 < 2s（局域网） |
| 数据库写入 | 执行历史写入不阻塞命令执行主流程 |

### 4.2 安全

| 要求 | 说明 |
|------|------|
| 凭据加密 | AES-256-GCM，master.key 本地生成，不可导出 |
| 文件权限 | master.key 权限 600，spider.db 权限 600 |
| AI 隔离 | Claude 通过 MCP 调用时，凭据在 spider 进程内解密，不传递给模型 |
| 审计覆盖 | 所有 MCP 工具调用和 REST API 写操作均记录审计日志 |
| 密码存储 | 用户密码 bcrypt 哈希存储（Phase 2） |
| Token 安全 | API Token 以 SHA-256 哈希存储，明文仅在创建时展示一次（Phase 2） |

### 4.3 可用性

| 要求 | 说明 |
|------|------|
| 零外部依赖 | SQLite 内嵌，单二进制运行，无需安装数据库或中间件 |
| 优雅关闭 | 收到 SIGTERM 后等待进行中的 SSH 会话完成（最长 30s）再退出 |
| systemd 支持 | 提供标准 systemd unit 文件，支持自动重启 |
| 数据备份 | 支持配置定时备份 spider.db 到指定目录（可选） |
| 健康检查 | `GET /health` 返回服务状态，供负载均衡和监控系统使用 |

### 4.4 可维护性

| 要求 | 说明 |
|------|------|
| 结构化日志 | JSON 格式日志，包含 level、time、msg、trace_id 字段 |
| 日志级别 | 支持 debug / info / warn / error，运行时可调整 |
| 版本信息 | `GET /version` 返回 version、commit、build_time |
| 配置文件 | YAML 格式，支持环境变量覆盖，启动时校验配置合法性 |
| 数据库迁移 | 内置 schema 版本管理，升级时自动执行迁移脚本 |

---

## 5. 系统架构

### 5.1 整体架构

```
┌─────────────────────────────────────────────────────┐
│  Claude Code (MCP Client)                           │
└──────────────────────┬──────────────────────────────┘
                       │ MCP over SSE (HTTP)
┌──────────────────────▼──────────────────────────────┐
│  spider 进程                                         │
│                                                     │
│  ┌─────────────────┐   ┌──────────────────────────┐ │
│  │   MCP Layer     │   │     REST API Layer       │ │
│  │  /sse endpoint  │   │  /api/* endpoints        │ │
│  │  10 tools       │   │  Web UI 静态资源          │ │
│  └────────┬────────┘   └────────────┬─────────────┘ │
│           │                         │               │
│  ┌────────▼─────────────────────────▼─────────────┐ │
│  │                Service Layer                   │ │
│  │   HostService   ExecService   AlertService     │ │
│  └────────┬──────────────────────────┬────────────┘ │
│           │                          │              │
│  ┌────────▼────────┐   ┌─────────────▼───────────┐  │
│  │   Store Layer   │   │      SSH Layer          │  │
│  │   host_store    │   │   pool + client + scp   │  │
│  │   log_store     │   └─────────────┬───────────┘  │
│  │   SQLite DB     │                 │              │
│  └─────────────────┘                 │ SSH          │
└─────────────────────────────────────┼──────────────┘
                                      ▼
                              远程主机集群
                         (web / app / db / ...)
```

### 5.2 组件说明

| 组件 | 职责 |
|------|------|
| MCP Layer | 处理 Claude Code 的 SSE 连接和工具调用，参数校验，结果序列化 |
| REST API Layer | 为 Web UI 提供 HTTP JSON API，处理认证（Phase 2） |
| Service Layer | 业务逻辑：主机 CRUD、命令执行调度、告警规则评估 |
| Store Layer | SQLite 持久化，凭据加密/解密，查询封装 |
| SSH Layer | 连接池管理、SSH 命令执行、SCP 文件传输 |
| Web UI | Vue.js SPA，编译后嵌入 spider 二进制（embed.go） |

### 5.3 关键数据流

**MCP 工具调用流程（execute_command 为例）：**

```
Claude Code
  → SSE POST /sse (tool: execute_command, host: web01, cmd: df -h)
  → MCP Layer 解析参数，调用 ExecService.Execute()
  → ExecService 从 Store 查询主机信息，解密凭据
  → SSH Pool 获取或新建连接
  → 执行命令，收集 stdout/stderr
  → 写入执行历史（异步）
  → 返回结果给 MCP Layer
  → SSE 响应返回 Claude Code
```

---

## 6. 数据模型

### 6.1 Host（主机）

```sql
CREATE TABLE hosts (
    id          TEXT PRIMARY KEY,          -- UUID v4
    name        TEXT UNIQUE NOT NULL,      -- 主机名，全局唯一
    ip          TEXT NOT NULL,             -- IP 地址或域名
    port        INTEGER NOT NULL DEFAULT 22,
    user        TEXT NOT NULL,             -- SSH 登录用户名
    auth_type   TEXT NOT NULL,             -- key | password | key_password
    credential  BLOB NOT NULL,             -- AES-256-GCM 加密的凭据 JSON
    proxy_id    TEXT,                      -- 跳板机 host.id，可空
    tags        TEXT NOT NULL DEFAULT '[]',-- JSON 字符串数组
    status      TEXT NOT NULL DEFAULT 'unknown', -- online | offline | unknown
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);
```

凭据 JSON 结构（加密前）：
```json
// auth_type=key:          {"private_key": "-----BEGIN..."}
// auth_type=password:     {"password": "secret"}
// auth_type=key_password: {"private_key": "-----BEGIN...", "passphrase": "pass"}
```

### 6.2 Execution（执行记录）

```sql
CREATE TABLE executions (
    id          TEXT PRIMARY KEY,
    host_id     TEXT NOT NULL,
    command     TEXT NOT NULL,
    output      TEXT,                      -- stdout + stderr 合并
    exit_code   INTEGER,
    status      TEXT NOT NULL,             -- success | failed | timeout | running
    started_at  DATETIME NOT NULL,
    finished_at DATETIME,
    duration_ms INTEGER,
    user_id     TEXT                       -- Phase 2: 操作人 user.id
);
```

### 6.3 User（用户）— Phase 2

```sql
CREATE TABLE users (
    id           TEXT PRIMARY KEY,
    username     TEXT UNIQUE NOT NULL,
    password     TEXT NOT NULL,            -- bcrypt hash
    role         TEXT NOT NULL,            -- admin | operator | viewer
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL,
    last_login   DATETIME
);
```

### 6.4 ApiToken（API Token）— Phase 2

```sql
CREATE TABLE api_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    name        TEXT NOT NULL,
    token_hash  TEXT NOT NULL,             -- SHA-256(token)，明文仅展示一次
    scopes      TEXT NOT NULL DEFAULT '["*"]', -- JSON 数组
    expires_at  DATETIME,                  -- NULL 表示永不过期
    created_at  DATETIME NOT NULL,
    last_used   DATETIME
);
```

### 6.5 AlertRule（告警规则）— Phase 3

```sql
CREATE TABLE alert_rules (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    host_tags   TEXT NOT NULL,             -- JSON 数组，目标主机标签
    command     TEXT NOT NULL,             -- 检测命令
    parse_expr  TEXT NOT NULL,             -- 解析表达式（正则或 awk）
    threshold   REAL NOT NULL,
    operator    TEXT NOT NULL,             -- gt | lt | gte | lte | eq
    interval_s  INTEGER NOT NULL DEFAULT 60,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL
);
```

### 6.6 AlertEvent（告警事件）— Phase 3

```sql
CREATE TABLE alert_events (
    id          TEXT PRIMARY KEY,
    rule_id     TEXT NOT NULL,
    host_id     TEXT NOT NULL,
    value       REAL,                      -- 检测到的实际值
    status      TEXT NOT NULL,             -- firing | resolved | acknowledged
    fired_at    DATETIME NOT NULL,
    resolved_at DATETIME,
    acked_by    TEXT                       -- 确认人 user.id
);
```

---

## 7. 接口规范

### 7.1 MCP 工具详细规格

| 工具 | 必填参数 | 可选参数 | 返回 |
|------|----------|----------|------|
| `list_hosts` | — | `tag: string`, `json: bool` | 主机列表文本或 JSON |
| `add_host` | `name`, `ip`, `user`, `auth_type` | `port`, `key`, `password`, `passphrase`, `proxy`, `tag` | 成功消息 + host_id |
| `remove_host` | `name_or_id` | — | 成功消息 |
| `update_host` | `name_or_id` | `ip`, `user`, `tag`, `port` 等 | 成功消息 |
| `execute_command` | `host`, `command` | `timeout: int` | stdout/stderr/exit_code |
| `execute_command_batch` | `command` | `tag`, `hosts: []string`, `timeout: int` | 各主机结果汇总 |
| `check_connectivity` | `host` | — | `{connected, latency_ms}` |
| `upload_file` | `host`, `local_path`, `remote_path` | — | 成功消息 + 文件大小 |
| `download_file` | `host`, `remote_path`, `local_path` | — | 成功消息 + 文件大小 |
| `get_execution_history` | — | `host`, `n: int` | 执行记录列表 |

### 7.2 REST API 端点

**主机管理**
```
GET    /api/hosts                  列出主机（支持 ?tag= 过滤）
POST   /api/hosts                  添加主机
GET    /api/hosts/:id              主机详情
PUT    /api/hosts/:id              更新主机
DELETE /api/hosts/:id              删除主机
POST   /api/hosts/:id/ping         连通性测试
```

**命令执行**
```
POST   /api/exec                   执行命令（单台）
POST   /api/exec/batch             批量执行
GET    /api/exec/stream/:id        实时输出（SSE）
```

**执行历史**
```
GET    /api/logs                   历史列表（支持 ?host= ?status= ?limit= ?offset=）
GET    /api/logs/:id               日志详情
```

**系统**
```
GET    /api/settings               系统配置
PUT    /api/settings               更新配置
GET    /health                     健康检查
GET    /version                    版本信息
```

**Phase 2 新增 — 用户与认证**
```
POST   /api/auth/login             登录，返回 JWT
POST   /api/auth/logout            登出
POST   /api/auth/refresh           刷新 JWT
GET    /api/users                  用户列表（Admin）
POST   /api/users                  创建用户（Admin）
PUT    /api/users/:id              更新用户
DELETE /api/users/:id              删除用户
GET    /api/tokens                 Token 列表
POST   /api/tokens                 创建 Token
DELETE /api/tokens/:id             撤销 Token
GET    /api/audit                  审计日志（Admin）
```

**Phase 3 新增 — 告警**
```
GET    /api/alerts/rules           告警规则列表
POST   /api/alerts/rules           创建规则
PUT    /api/alerts/rules/:id       更新规则
DELETE /api/alerts/rules/:id       删除规则
GET    /api/alerts/events          告警事件列表
PUT    /api/alerts/events/:id/ack  确认告警
GET    /api/dashboard              仪表盘聚合数据
```

---

## 8. 安全设计

### 8.1 凭据安全

- **加密算法**：AES-256-GCM，提供认证加密，防止篡改
- **密钥管理**：master.key 在首次启动时本地随机生成，文件权限 600，不进入版本控制，不可通过 API 导出
- **AI 隔离**：Claude Code 通过 MCP 调用时，spider 在进程内解密凭据并直接建立 SSH 连接，凭据明文不出进程边界
- **内存安全**：凭据解密后使用完毕立即清零（Go `defer` + `bytes.Equal` 模式）

### 8.2 访问控制（Phase 2）

- **认证方式**：Web UI 使用 JWT（HS256，24h 有效期）；MCP 使用 API Token（Bearer）
- **RBAC 执行**：Service Layer 统一鉴权，MCP Layer 和 REST API Layer 不做业务逻辑判断
- **最小权限**：Viewer 角色无法触发任何写操作或命令执行

### 8.3 传输安全

- **生产环境建议**：通过 nginx 或 caddy 反向代理提供 HTTPS，spider 本身监听 localhost
- **CORS 配置**：REST API 支持配置允许的 Origin，默认仅允许同源
- **SSE 连接**：MCP SSE 端点建议在生产环境通过 HTTPS 暴露

### 8.4 审计

- 所有 MCP 工具调用写入 executions 表（含命令、输出、状态）
- Phase 2 起 executions 表关联 user_id，记录操作人
- 审计记录不提供删除 API，仅支持查询

---

## 9. 部署方案

### 9.1 单用户本地部署

适用于个人工程师，5 分钟完成安装：

```bash
# 1. 编译安装
make install   # 安装到 $GOPATH/bin

# 2. 启动 spider（后台运行）
spider &

# 3. 注册到 Claude Code
claude mcp add --transport sse spider http://localhost:8000/sse

# 4. 添加第一台主机
spdctl host add --name web01 --ip 10.0.0.1 --user root \
  --auth key --key ~/.ssh/id_rsa
```

数据存储在 `~/.spider/`，无需额外配置。

### 9.2 团队服务器部署（systemd）

适用于团队共享实例，部署在跳板机或内网服务器：

```ini
# /etc/systemd/system/spider.service
[Unit]
Description=Spider MCP Server
After=network.target

[Service]
ExecStart=/usr/local/bin/spider
Restart=always
User=spider
Group=spider
Environment=SPIDER_DATA_DIR=/var/lib/spider

[Install]
WantedBy=multi-user.target
```

```bash
systemctl enable --now spider
```

**建议配合 nginx 提供 HTTPS：**

```nginx
server {
    listen 443 ssl;
    server_name spider.internal.example.com;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Connection '';
        proxy_http_version 1.1;       # SSE 需要 HTTP/1.1
        proxy_buffering off;          # SSE 需要关闭缓冲
    }
}
```

### 9.3 Docker 部署（规划中）

```yaml
# docker-compose.yml
services:
  spider:
    image: ghcr.io/fty-ai/spider:latest
    ports:
      - "8000:8000"
    volumes:
      - spider-data:/data
    environment:
      - SPIDER_DATA_DIR=/data
    restart: unless-stopped

volumes:
  spider-data:
```

---

## 10. 产品路线图

| 阶段 | 主要内容 | 状态 |
|------|----------|------|
| **基线** | MCP SSE Server、spdctl CLI、SSH 执行、文件传输、执行历史、AES-256-GCM 凭据加密 | ✅ 已完成 |
| **Phase 1** | Web UI 完善：主机管理界面、实时命令执行、历史日志查看、文件上传 | 🔄 规划中 |
| **Phase 2** | 多用户与权限控制：账号管理、RBAC、API Token、操作审计日志 | 📋 待规划 |
| **Phase 3** | 告警与监控：SSH 状态监控、阈值告警规则、钉钉/Slack 通知、监控仪表盘 | 📋 待规划 |
| **未来** | Docker 镜像发布、多 spider 实例联邦、Webhook 触发器 | 💡 探索中 |

### 阶段依赖关系

```
基线（已完成）
    └── Phase 1（Web UI）
            └── Phase 2（多用户）
                    └── Phase 3（告警）
```

Phase 2 依赖 Phase 1 的 Web UI 框架；Phase 3 依赖 Phase 2 的用户系统（告警通知需要关联用户）。

---

*本文档由 Spider 项目团队维护，随产品迭代持续更新。*
