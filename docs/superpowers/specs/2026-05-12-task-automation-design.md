# Spec: Task Automation

## 概述

Task 是持久化、可调度、跨对话的自动化任务。与 Todo（会话内临时进度跟踪）不同，Task 由 headless Agent 无人值守执行，执行后生成结构化报告。

---

## 需求、场景与价值

### 核心需求

- 用户在对话中描述意图，Agent 提取并创建持久化任务
- 任务支持 cron 调度（自动执行）和手动触发
- 执行由 headless Agent 完成，无需人工介入
- 执行后 LLM 生成摘要；NotifyMode = "anomaly" 时判断异常并通知
- 通知渠道：钉钉、邮件、Webhook，敏感字段加密存储
- 前端：任务管理页 + 个人设置中的通知渠道配置

### 用户场景

1. **定期巡检**：每周三凌晨自动检查所有设备磁盘使用率，异常时钉钉告警
2. **固件升级**：对话中说"下周升级固件"，Agent 提取并保存为手动任务，到时手动触发
3. **配置核查**：每天检查关键配置是否被篡改，发现异常立即通知
4. **日志清理**：每月定期清理 30 天前的日志，执行后生成摘要确认结果

### 价值

- **减少重复操作**：把对话中的一次性操作变成可复用的自动化任务
- **异步执行，不阻塞对话**：任务在后台独立运行，执行过程不占用用户当前对话的上下文窗口，用户无需等待
- **闭环可观测**：每次执行有原始输出 + LLM 摘要，历史可查
- **低门槛执行报告**：不需要配置监控系统，一句话开启执行报告通知

### 已知不足（v1）

1. **调度精度粗糙**：每分钟轮询 DB，多实例部署时依赖行锁，没有分布式调度保障
2. **headless Agent 无历史上下文**：每次执行从零开始，无法利用历史执行结果做趋势判断
3. **告警判断依赖 LLM**：异常判断由 LLM 完成，可能误报或漏报，无法设置精确阈值
4. **通知无重试**：发送失败只记录日志，没有重试机制
5. **host_ids 静态绑定**：任务创建时绑定设备 ID，设备变更后不自动更新

---

## 需求建模

### 领域模型

```
User ──── 创建 ────► Task ──── 绑定 ────► Host[]
                      │
                      └── 触发 ──► TaskRun
                                      │
                                      └── 分析 ◄── LLM

User ──── 配置 ────► NotifyChannel
                         ▲
                    TaskRun.Alerted ──── 发送 ────►
```

### 实体职责

| 实体 | 职责 | 生命周期 |
|------|------|----------|
| Task | 意图 + 调度配置，持久化 | active / paused / archived |
| TaskRun | 单次执行快照，只增不改（除状态更新） | running → success / failed |
| NotifyChannel | 通知渠道配置，与 Task 解耦 | 独立管理 |

### 角色与用例

三个 Actor：用户、Chat Agent、Scheduler。

```
用户          Chat Agent       Scheduler
 │                │                │
 ├─ 描述意图 ──►   │                │
 │            提取字段             │
 │            展示确认             │
 ├─ 确认 ──────►  │                │
 │            CreateTask           │
 │                                 │
 ├─ 手动触发 ──────────────────►  TriggerNow
 │                                 │
 │                            tick() 每分钟
 │                            isDue() 检查 cron
 │                            RunHeadless
 │                            LLM 分析
 │                            → TaskRun（Alerted=true 表示需关注）
 │                            → 推送通知（有 NotifyChannel 则发）
 │
 ├─ 查看执行记录
 └─ 配置通知渠道
```

### 状态机

**Task**
```
[新建] → active ⇄ paused → archived
```

**TaskRun**（终态不可变更）
```
[创建] → running → success
                 → failed
```

### 触发器模型

| `schedule` 值 | 触发方式 |
|---|---|
| 非空 cron 表达式 | Scheduler 自动触发（每分钟轮询） |
| 空字符串 | 用户手动触发 |

`TriggerType` 不作为独立字段，从 `schedule` 是否为空推断，避免冗余状态。

### 关键约束

1. **CreateTask 只在用户确认后调用**：Agent 提取 → 展示 → 确认 → 保存，工具本身无提取逻辑
2. **TaskRun 与 Task 解耦**：Task 更新不影响历史 TaskRun
3. **NotifyChannel 与 Task 解耦**：告警触发时遍历所有 enabled 渠道，不绑定到具体 Task
4. **LLM 只参与两个节点**：创建时提取字段、执行后分析输出；执行过程中不参与

---

## 数据模型

### Task

```go
type Task struct {
    ID               int64     `json:"id"`
    Name             string    `json:"name"`               // 任务名称
    Goal             string    `json:"goal"`               // 自然语言目标
    HostIDs          []int64   `json:"host_ids"`           // 目标设备
    Schedule         string    `json:"schedule"`           // cron 表达式，空 = manual only
    NotifyMode       string    `json:"notify_mode"`        // "none" | "failure" | "complete" | "anomaly"
    RunRetentionDays int       `json:"run_retention_days"` // TaskRun 保留天数，默认 30，0 = 永久保留
    TimeoutMinutes   int       `json:"timeout_minutes"`    // 执行超时（分钟），默认 30，0 = 无限制
    Status           string    `json:"status"`             // "active" | "paused" | "archived"
    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
    SourceConvID     string    `json:"source_conv_id"`     // 创建来源对话
}
```

`NotifyMode` 说明（需有 NotifyChannel 才发送）：

| 值 | 触发条件 | 发送内容 |
|----|----------|----------|
| `"none"` | 不通知 | — |
| `"failure"` | 执行失败（Agent 异常中断） | 中断原因 |
| `"complete"` | 每次执行完成（无论成功或失败） | LLM 执行摘要 |
| `"anomaly"` | 执行完成 + LLM 判断有异常 | LLM 执行摘要（含异常标注） |

说明：
- `"complete"` 覆盖所有完成状态，包含执行失败；若同时需要失败通知，选 `"complete"` 即可，无需叠加 `"failure"`
- `"anomaly"` 隐含"触发 LLM 异常判断"的语义——其他模式不做异常判断，节省 token

前端展示为 radio group：

```
执行报告通知
  ○ 不通知
  ○ 执行失败时通知
  ○ 每次完成后发摘要
  ○ 仅发现异常时发摘要
```

### TaskRun（执行记录）

```go
type TaskRun struct {
    ID         int64      `json:"id"`
    TaskID     int64      `json:"task_id"`
    StartedAt  time.Time  `json:"started_at"`
    FinishedAt *time.Time `json:"finished_at"`
    Status     string     `json:"status"`   // "running" | "success" | "failed"
    RawOutput  string     `json:"raw_output"`
    Summary    string     `json:"summary"`  // LLM 生成摘要
    Alerted    bool       `json:"alerted"`  // 本次执行有异常或失败需关注
}
```

### NotifyChannel（通知渠道配置）

通知渠道在个人设置中配置，按 `NotifyMode` 触发时遍历所有 enabled 渠道发送。

```go
type NotifyChannel struct {
    ID        int64     `json:"id"`
    Type      string    `json:"type"`    // "dingtalk" | "email" | "webhook"
    Name      string    `json:"name"`    // 用户自定义名称
    Config    string    `json:"config"`  // JSON，各渠道配置不同（见下）
    Enabled   bool      `json:"enabled"`
    CreatedAt time.Time `json:"created_at"`
}
```

各渠道 Config 结构：

```json
// dingtalk
{ "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx", "secret": "xxx" }

// email
{ "to": ["ops@example.com"], "smtp_host": "smtp.example.com", "smtp_port": 465, "username": "xxx", "password": "xxx" }

// webhook
{ "url": "https://example.com/hook", "method": "POST", "headers": {"Authorization": "Bearer xxx"} }
```

`Config` 中的敏感字段（`secret`、`password`、`headers` 中的 token）存储前使用项目现有 crypto 包加密，与 SSH 密钥、API Key 的处理方式一致。

---

## 创建流程

1. 用户在对话中说"帮我记录，这台设备下周要升级固件，每周三执行"或"每周检查磁盘，有问题发钉钉"
2. Agent（LLM）从对话上下文提取：
   - `name`：任务名称
   - `goal`：自然语言目标
   - `host_ids`：涉及设备
   - `schedule`：cron 表达式（如 `0 2 * * 3`），无调度则为空
   - `notify_mode`：通知模式，`"none"` | `"failure"` | `"complete"` | `"anomaly"`（默认 `"none"`）
   - 若用户提到通知渠道（钉钉 webhook、邮箱等），Agent 可先调用 `CreateNotifyChannel` 创建全局渠道
3. Agent 向用户展示提取结果，等待确认
4. 用户确认后，Agent 调用 `CreateTask` 工具保存（所有字段已确定）

---

## 触发器

| `schedule` 值 | 类型 | 说明 |
|--------------|------|------|
| 非空 cron 表达式 | cron | 标准 5 字段，DB 轮询调度（每分钟检查） |
| 空字符串 | manual | 用户点"立即执行"或对话中触发 |

`TriggerType` 字段不需要，从 `schedule` 是否为空推断。

调度器：进程内 goroutine，每分钟轮询 DB，找到到期 active Task 启动执行。

**多实例互斥（行锁）：**
```sql
BEGIN;
SELECT * FROM tasks WHERE id = ? AND status = 'active' FOR UPDATE NOWAIT;
-- 拿到锁 → 检查 cron 是否到期 → 创建 TaskRun
COMMIT;
```
- 使用 `SELECT FOR UPDATE NOWAIT`（SQLite 3.37+）
- 其他实例拿不到锁时立即返回，跳过本次调度
- 锁超时依赖 SQLite 的 `busy_timeout`（默认 5s）

### 并发执行策略

| 触发方式 | 已有 running TaskRun | 行为 |
|---------|---------------------|------|
| Cron 到期 | 存在 | **跳过**，记录日志 "skipped: previous run still running" |
| 手动触发 | 存在 | **拒绝**，返回错误 "task is already running" |
| Cron 到期 | 不存在 | 创建新 TaskRun，正常执行 |
| 手动触发 | 不存在 | 创建新 TaskRun，正常执行 |

实现：触发前查询 `SELECT COUNT(*) FROM task_runs WHERE task_id = ? AND status = 'running'`。

---

## 执行

1. 调度器触发，创建 `TaskRun` 记录（status: running）
2. **过滤无效 Host ID**：
   - 查询 `SELECT id FROM hosts WHERE id IN (task.host_ids)` 得到有效 ID 列表
   - 若全部无效 → 更新 `TaskRun.Status = failed`，`RawOutput = "all hosts invalid: [...]"`，跳过后续步骤
   - 若部分无效 → 在 `RawOutput` 开头记录 "skipped invalid hosts: [...]"，继续执行有效 ID
3. **启动 headless Agent（带超时）**：
   - 创建 context，超时时长 = `Task.TimeoutMinutes`（0 = 无限制）
   - 无对话历史
   - System prompt 包含：任务目标 + 任务类型 + 有效目标设备信息
   - 可使用全部 Agent 工具（SSH、CLI、ListHosts 等）
4. Agent 自主完成多步骤执行
5. **超时处理**：
   - 若 context 超时 → 取消 Agent 执行
   - `TaskRun.Status = failed`，`RawOutput` 追加 "execution timeout after Xm"
   - `TaskRun.Alerted = true`
   - 跳过后续 LLM 分析，直接发送通知（若 NotifyMode 匹配）
6. 执行完成，原始输出写入 `TaskRun.RawOutput`（前端展示截断至 10KB，完整内容保留在 DB）
7. 轻量 LLM 调用分析输出（使用系统默认 provider）：
   - 生成执行摘要，写入 `TaskRun.Summary`
   - 若 `NotifyMode = "anomaly"`：LLM 判断是否异常；异常则 `TaskRun.Alerted = true`
8. 更新 `TaskRun.Status` 为 success / failed
   - 若 status = failed：`TaskRun.Alerted = true`
9. 按 `NotifyMode` 发送通知（需有 NotifyChannel）：
   - `"failure"` 且 status = failed → 发中断原因
   - `"complete"` → 发摘要
   - `"anomaly"` 且 `TaskRun.Alerted = true` → 发摘要

LLM 参与两个节点：创建时（Agent 提取信息）、执行后（分析报告，用系统默认 provider）。执行中不参与。

### TaskRun 保留策略

调度器每天凌晨执行一次清理：删除每个 Task 中 `started_at` 早于 `NOW() - run_retention_days` 的 TaskRun 记录。

- 默认保留 30 天
- `run_retention_days = 0` 表示永久保留
- 清理在独立 goroutine 中执行，不影响任务调度

---

## Agent 工具

### CreateTask

```
Name: CreateTask
Description: Save a confirmed automated task. Has side effects. Call only after user has confirmed all fields.
```

InputSchema（所有字段 Agent 调用前已确定）：
- `name` (string, required)
- `goal` (string, required)
- `host_ids` ([]int64, required)
- `schedule` (string) — cron 表达式，空 = manual only
- `notify_mode` (string) — `"none"` | `"failure"` | `"complete"` | `"anomaly"`，默认 `"none"`
- `run_retention_days` (int) — 默认 30，0 = 永久保留
- `timeout_minutes` (int) — 默认 30，0 = 无限制

### NotifyChannel 管理工具

Agent 可管理全局通知渠道（所有 enabled 渠道接收通知）。

**CreateNotifyChannel**
```
Name: CreateNotifyChannel
Description: Create a global notification channel. Has side effects.
```

InputSchema：
- `type` (string, required) — `"dingtalk"` | `"email"` | `"webhook"`
- `name` (string, required) — 用户自定义名称
- `config` (object, required) — 渠道配置（见 NotifyChannel 模型）
- `enabled` (bool) — 默认 true

**ListNotifyChannels**
```
Name: ListNotifyChannels
Description: List all notification channels. Read-only.
```

**UpdateNotifyChannel**
```
Name: UpdateNotifyChannel
Description: Update a notification channel. Has side effects.
```

InputSchema：
- `id` (int64, required)
- `name` (string)
- `config` (object)
- `enabled` (bool)

**DeleteNotifyChannel**
```
Name: DeleteNotifyChannel
Description: Delete a notification channel. Has side effects.
```

InputSchema：
- `id` (int64, required)

---

## 前端界面

### 页面结构

左侧列表 + 右侧详情，与主机管理页风格一致。

**左侧：**
- 顶部：标题"任务" + "新建"按钮
- 任务列表：名称、调度摘要、状态徽章（活跃/暂停）、上次执行结果；配置了任意通知项的任务显示通知图标

**右侧顶部（配置摘要）：**
- 任务名 + 状态徽章
- 操作按钮：立即执行（后端检查并发）/ 编辑 / 暂停
- 目标（goal）
- 调度、设备、通知模式、创建来源对话

**右侧主体（执行记录）：**
- 按时间倒序列表，分页加载（每页 20 条）
- 每条记录：时间、耗时、状态图标；`Alerted = true` 的记录显示标注
- 可展开：LLM 摘要 + 原始输出（monospace）

### 新建/编辑

弹窗表单，字段：名称、目标、设备（多选）、调度（cron 表达式）、通知模式（radio：不通知 / 执行失败时通知 / 每次完成后发摘要 / 仅发现异常时发摘要）、执行记录保留天数（数字输入，默认 30）。

---

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/tasks` | 列表 |
| POST | `/api/tasks` | 创建 |
| GET | `/api/tasks/:id` | 详情 |
| PUT | `/api/tasks/:id` | 更新 |
| DELETE | `/api/tasks/:id` | 删除 |
| POST | `/api/tasks/:id/trigger` | 手动触发 |
| GET | `/api/tasks/:id/runs?limit=20&offset=0` | 执行记录（分页，默认 limit=20） |
| GET | `/api/tasks/:id/runs/:run_id` | 单次执行详情 |
| GET | `/api/notify-channels` | 通知渠道列表 |
| POST | `/api/notify-channels` | 创建通知渠道 |
| PUT | `/api/notify-channels/:id` | 更新通知渠道 |
| DELETE | `/api/notify-channels/:id` | 删除通知渠道 |
| POST | `/api/notify-channels/:id/test` | 测试发送 |

---

## 范围边界

**v1 包含：**
- Task 模型（`NotifyMode` 控制通知行为）+ CRUD API
- cron + manual 触发器
- headless Agent 执行
- 执行后 LLM 摘要；`NotifyMode = "anomaly"` 时做异常判断
- TaskRun.Alerted 标记（执行失败 或 LLM 判断异常）
- NotifyChannel 模型（钉钉 / 邮件 / Webhook）+ 配置 API
- 按 NotifyMode 发送通知（需配置 NotifyChannel）
- 前端 Task 管理页
- 个人设置：通知渠道管理

**v1 不包含：**
- 设备事件触发
- 指标阈值触发
- 跨任务依赖
- API 触发（外部 webhook）
