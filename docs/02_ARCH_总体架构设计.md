# Spider 智能运维平台 — 总体架构设计

**版本：** v0.1  
**日期：** 2026-04-06  
**状态：** 草稿

---

## 目录

1. [系统概览](#1-系统概览)
2. [进程模型](#2-进程模型)
3. [模块职责](#3-模块职责)
4. [数据流](#4-数据流)
5. [关键接口](#5-关键接口)
6. [数据模型](#6-数据模型)
7. [设计决策](#7-设计决策)

---

## 1. 系统概览

Spider 是一个以 Claude Code 为中心的 SSH 代理网关。它的核心职责是：**让 AI 模型能够安全地在远程主机上执行运维操作，同时确保凭据永不暴露给 AI**。

系统由两个可执行文件组成：

| 二进制 | 职责 |
|--------|------|
| `spider` | 常驻服务进程，提供 MCP、REST API 和 Web UI |
| `spdctl` | 本地管理 CLI，直接读写本地数据库 |

### 外部交互关系

```
┌─────────────────┐        MCP (HTTP)        ┌──────────────────────┐
│   Claude Code   │ ───────────────────────► │                      │
└─────────────────┘                          │   spider 服务进程     │
                                             │   :8000              │
┌─────────────────┐        REST API          │                      │
│    Web UI       │ ───────────────────────► │                      │
└─────────────────┘                          └──────────┬───────────┘
                                                        │ SSH
┌─────────────────┐     直接访问 SQLite                 ▼
│    spdctl CLI   │ ──────────────────────►  ┌──────────────────────┐
└─────────────────┘                          │   远程主机集群        │
                                             └──────────────────────┘
```

---

## 2. 进程模型

`spider` 是单进程、单端口服务。启动时按以下顺序初始化：

```
1. 加载配置（~/.spider/config.yaml，环境变量覆盖）
2. 初始化加密模块（加载或生成 ~/.spider/master.key）
3. 打开 SQLite 数据库（~/.spider/spider.db）
4. 创建 HostStore、LogStore
5. 启动 SSH 连接池（含后台清理 goroutine）
6. 注册 HTTP 路由（/mcp、/api/v1/*、/）
7. 监听 :8000，等待信号优雅关闭
```

所有组件共享同一个 `App` 结构体，通过依赖注入传递：

```go
type App struct {
    HostStore *store.HostStore
    LogStore  *store.LogStore
    Pool      *sshpkg.Pool
    Config    *config.Config
    DB        *sql.DB
}
```

---

## 3. 模块职责

### 3.1 `internal/config` — 配置管理

从 `~/.spider/config.yaml` 加载配置，支持环境变量覆盖（`SPIDER_DATA_DIR`）。文件不存在时使用内置默认值，保证零配置启动。

关键配置项：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `data_dir` | `~/.spider` | SQLite、master.key 存放目录 |
| `sse.addr` | `:8000` | HTTP 监听地址 |
| `ssh.default_timeout_seconds` | `30` | 命令执行默认超时 |
| `ssh.pool_ttl_seconds` | `300` | SSH 连接池 TTL |
| `ssh.max_pool_size` | `50` | 连接池最大连接数 |

### 3.2 `internal/crypto` — 凭据加密

使用 AES-256-GCM 对称加密，密钥存储在 `data_dir/master.key`（32 字节随机数，文件权限 0600）。

加密流程：
```
明文 → 随机 nonce（12 字节）→ AES-256-GCM 加密 → base64(nonce + ciphertext)
```

凭据在写入数据库前加密，在建立 SSH 连接时解密，**解密后的明文仅在内存中短暂存在，不经过任何网络接口**。

### 3.3 `internal/db` — 数据库

封装 SQLite 的打开和 schema 迁移（幂等 `CREATE TABLE IF NOT EXISTS`）。数据库文件位于 `data_dir/spider.db`。

包含两张表：`hosts`（主机资产）和 `execution_logs`（执行历史）。

### 3.4 `internal/store` — 数据访问层

提供两个 Store：

**HostStore**：主机的 CRUD 操作。写入时调用 `crypto.Encrypt` 加密凭据；读取时按需调用 `crypto.Decrypt`（仅在建立 SSH 连接时）。对外暴露的 `Host.Safe()` 方法返回不含凭据的安全视图。

**LogStore**：执行日志的写入和查询，支持按 `host_id` 过滤和分页。

### 3.5 `internal/ssh` — SSH 执行层

包含三个子模块：

- **`client.go`**：封装单条 SSH 连接，支持 `password`、`key`、`key_password` 三种认证方式。
- **`pool.go`**：按 `host_id` 缓存 SSH 连接，TTL 到期或连接空闲时自动清理。后台 goroutine 每 `ttl/2` 执行一次清理。
- **`scp.go`**：基于 SCP 协议实现文件上传（`upload_file`）和下载（`download_file`）。

### 3.6 `internal/mcp` — MCP 接口层

基于 `mark3labs/mcp-go` 库，以 Streamable HTTP 模式对外提供 MCP 服务（挂载于 `/mcp`）。

注册的 MCP 工具：

| 工具名 | 说明 |
|--------|------|
| `list_hosts` | 列出主机，支持 tag 过滤 |
| `add_host` | 添加主机（含凭据加密） |
| `remove_host` | 删除主机 |
| `update_host` | 更新主机信息 |
| `execute_command` | 在单台主机执行命令 |
| `execute_command_batch` | 按 host_ids 或 tag 批量执行命令 |
| `check_connectivity` | 测试 SSH 连通性 |
| `upload_file` | 上传文件到远程主机 |
| `download_file` | 从远程主机下载文件 |
| `get_execution_history` | 查询执行历史 |

### 3.7 `internal/api` — REST API 层

为 Web UI 提供 REST API，挂载于 `/api/v1/`。所有主机相关响应使用 `SafeHost`（不含凭据字段）。

主要端点：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/hosts` | 列出主机 |
| POST | `/api/v1/hosts` | 添加主机 |
| GET | `/api/v1/hosts/:id` | 获取主机详情 |
| PUT | `/api/v1/hosts/:id` | 更新主机 |
| DELETE | `/api/v1/hosts/:id` | 删除主机 |
| POST | `/api/v1/hosts/:id/ping` | 测试连通性 |
| POST | `/api/v1/exec` | 执行命令 |
| GET | `/api/v1/logs` | 查询执行历史 |

### 3.8 `cmd/spider` — 服务入口

组装所有模块，启动 HTTP 服务。Web UI 静态资源通过 `embed.go` 编译进二进制（`web/dist`），无需外部文件。

### 3.9 `cmd/spdctl` — CLI 工具

本地管理工具，基于 `cobra`。直接连接本地 SQLite，不依赖 `spider` 服务进程运行。

子命令：`host`（主机管理）、`exec`（执行命令）、`ping`（连通性测试）、`history`（执行历史）、`mcp`（MCP server 注册/注销/状态）。

---

## 4. 数据流

### 4.1 Claude Code 执行命令（核心路径）

```
Claude Code
  │
  │  MCP: execute_command(host_id="web-01", command="df -h")
  ▼
internal/mcp/tools.go: makeExecuteCommand()
  │
  ├─► HostStore.GetByIDOrName("web-01")   # 查询主机元数据
  │     └─► SQLite hosts 表
  │
  ├─► Pool.Get(host, hostStore)           # 获取/新建 SSH 连接
  │     └─► ssh.NewClient(host, hostStore)
  │           └─► crypto.Decrypt(encrypted_credential)  # 内存解密
  │                 └─► ssh.Dial()        # 建立 TCP+SSH 连接
  │
  ├─► client.Execute("df -h", timeout)   # 执行命令
  │
  ├─► Pool.Release("web-01")             # 归还连接
  │
  ├─► LogStore.Save(executionLog)        # 写入执行日志
  │
  └─► MCP 响应: stdout + stderr + exit_code
```

### 4.2 批量执行

`execute_command_batch` 在单个 goroutine 中串行执行（当前实现），按 `host_ids` 列表或 `tag` 展开为多次 `execute_command` 调用，聚合结果后返回。

### 4.3 spdctl 添加主机

```
spdctl host add --name web-01 --ip 1.2.3.4 --auth-type password --credential "xxx"
  │
  ▼
cli/host.go → HostStore.Add(req)
  │
  ├─► crypto.Encrypt(credential)   # 加密凭据
  └─► SQLite INSERT INTO hosts     # 持久化
```

---

## 5. 关键接口

### 5.1 MCP 接入配置

在 Claude Code 的 `~/.claude/settings.json` 中注册：

```json
{
  "mcpServers": {
    "spider": {
      "type": "http",
      "url": "http://localhost:8000/mcp"
    }
  }
}
```

`spdctl mcp register` 命令自动完成此配置。

### 5.2 HostStore 接口

```go
Add(req *models.AddHostRequest) (*models.Host, error)
GetByIDOrName(idOrName string) (*models.Host, error)
List(tag string) ([]*models.Host, error)
Update(id string, req *models.UpdateHostRequest) (*models.Host, error)
Delete(id string) error
DecryptCredential(h *models.Host) (credential, passphrase string, error)
```

### 5.3 SSH Pool 接口

```go
Get(host *models.Host, hs *store.HostStore) (*Client, error)
Release(hostID string)
Close()
```

---

## 6. 数据模型

### 6.1 hosts 表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | TEXT PK | UUID |
| `name` | TEXT UNIQUE | 人类可读名称，MCP 工具中使用 |
| `ip` | TEXT | IP 地址 |
| `port` | INTEGER | SSH 端口，默认 22 |
| `username` | TEXT | SSH 用户名 |
| `auth_type` | TEXT | `password` / `key` / `key_password` |
| `encrypted_credential` | TEXT | AES-256-GCM 加密后的 base64 |
| `encrypted_passphrase` | TEXT | 私钥 passphrase（可为空） |
| `tags` | TEXT | JSON 数组，如 `["prod","web"]` |
| `created_at` / `updated_at` | DATETIME | UTC 时间戳 |

### 6.2 execution_logs 表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | TEXT PK | UUID |
| `host_id` | TEXT | 关联主机 ID |
| `command` | TEXT | 执行的命令 |
| `stdout` / `stderr` | TEXT | 命令输出 |
| `exit_code` | INTEGER | 退出码 |
| `duration_ms` | INTEGER | 执行耗时（毫秒） |
| `triggered_by` | TEXT | `mcp` / `api` / `cli` |
| `created_at` | DATETIME | UTC 时间戳 |

索引：`host_id`、`created_at`。

---

## 7. 设计决策

### 7.1 单二进制，零外部依赖

选择 SQLite 而非 PostgreSQL/MySQL，选择嵌入式 Web UI 而非独立前端服务，目标是让个人工程师能在 5 分钟内完成安装。代价是不支持多进程水平扩展，但对当前目标用户（单机或小团队）完全够用。

### 7.2 凭据隔离是核心约束

MCP 工具的所有响应均不包含凭据字段。`Host.Safe()` 方法在 API 层强制过滤。凭据解密仅发生在 `ssh.NewClient()` 内部，解密后的明文不离开该函数作用域（通过 `ssh.Dial()` 直接消费）。

### 7.3 MCP Streamable HTTP 而非 stdio

选择 HTTP 模式而非 stdio 模式，使 spider 可以作为独立服务部署，支持多个 Claude Code 实例同时连接，也便于未来扩展为团队共享服务。

### 7.4 spdctl 直连数据库

`spdctl` 不通过 HTTP API，而是直接访问本地 SQLite。这简化了本地开发和调试场景，不需要 spider 服务进程运行。代价是 spdctl 只能在与 spider 同机运行时使用。

### 7.5 SSH 连接池

连接池按 `host_id` 缓存连接，TTL 默认 300 秒。批量操作时复用连接可显著减少握手开销。连接标记 `inUse` 防止并发操作使用同一连接（当前串行执行，此机制为未来并发预留）。
