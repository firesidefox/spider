---
title: 网关自然语言运维对话系统
date: 2026-05-02
status: approved
---

# 网关自然语言运维对话系统 设计规格

## 1. 概述

在 spider.ai Web 前端新增 Chat 页面，用户通过自然语言对数百台网关设备进行自动化运维操作。后端内置 Agent Engine，参考 Claude Code agent loop 架构，编排 RAG 文档检索 → LLM 命令生成 → 风险分级 → 执行 → 验证的完整链路。

### 1.1 操作范围

- 查询类：接口状态、路由表、ARP 表、设备信息
- 配置类：IP、VLAN、ACL、路由策略、防火墙规则
- 批量操作：同一命令下发到数百台设备
- 巡检类：定期检查设备健康状态
- 诊断类：故障排查、连通性测试
- 固件/升级：设备固件升级

### 1.2 核心设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 架构方案 | 内置轻量 Agent | 复用现有 SSH pool，单进程部署，精细安全控制 |
| 通信协议 | SSE + REST | 复用现有 SSE 能力，确认流通过 REST POST 协调 |
| 执行方式 | 优先 CLI over SSH | 网关 CLI 文档完备，REST API 作为备选 |
| 自然语言转换 | LLM 翻译 | 用户自然语言 → LLM 根据文档生成 CLI 命令 |
| 文档存储 | sqlite-vec 向量库 | 与现有 SQLite 技术栈一致，零额外依赖 |
| LLM 调用 | Claude API 直调 | 抽象接口预留多 provider |
| 对话持久化 | 全量存储 | 用户可回溯历史对话 |

## 2. 整体架构

```
┌─────────────────────────────────────────────────────┐
│                   Vue 3 Frontend                     │
│  ┌───────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │ ChatView  │  │HostsView │  │  ExecView (现有)  │  │
│  │  (新页面)  │  │ (扩展)   │  │                  │  │
│  └─────┬─────┘  └──────────┘  └──────────────────┘  │
│        │ SSE stream                                  │
└────────┼────────────────────────────────────────────┘
         │
┌────────▼────────────────────────────────────────────┐
│                   Go Backend                         │
│  ┌──────────────────────────────────────────────┐   │
│  │              Agent Engine                     │   │
│  │  Loop Manager + Tool Registry + Hook Chain    │   │
│  │  Tools: search_docs | execute_cli | call_api  │   │
│  │         batch_exec  | get_device_info | verify │   │
│  └──────────────────────────────────────────────┘   │
│  ┌─────────────┐ ┌──────────┐ ┌────────────────┐   │
│  │ SSH Pool    │ │ RAG/Vec  │ │ LLM Client     │   │
│  │ (现有)      │ │ Store    │ │ (Claude API)   │   │
│  └─────────────┘ └──────────┘ └────────────────┘   │
│  ┌──────────────────────────────────────────────┐   │
│  │              SQLite (现有 + 扩展)              │   │
│  │  hosts│ssh_keys│users│logs│conversations│docs │   │
│  └──────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

## 3. Agent Engine

参考 Claude Code agent loop 架构。核心是 while-loop + tool dispatch + hook chain。

### 3.1 Agent Loop 流程

```
用户发消息 → 加入 message history → 调 LLM (streaming)
    │
    LLM 返回纯文本 → 流式推送前端 → 结束本轮
    LLM 返回 tool_use → BeforeTool Hook
        │
        ├── safe → 自动执行
        ├── moderate → SSE 推确认请求 → 等待 POST → 通过/拒绝
        └── dangerous → 同上 + 二次确认
        │
        Tool Execute → AfterTool Hook (审计日志)
        → tool result 加入 history → 下一轮 LLM 调用
```

### 3.2 风险分级

| 级别 | 操作类型 | 行为 |
|------|---------|------|
| safe | 查询 (show/display/get) | 自动执行 |
| moderate | 单台配置修改 | 需用户确认 |
| dangerous | 批量操作、删除、重启、升级 | 需用户确认 + 二次确认 |

LLM 生成命令时同时输出 risk_level，BeforeTool Hook 校验。

### 3.3 Tool 接口

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(ctx context.Context, input map[string]any) (*ToolResult, error)
}
```

### 3.4 内置 Tools

| Tool | 用途 |
|------|------|
| search_docs | RAG 检索命令行/API 文档 |
| execute_cli | SSH 到网关执行单条 CLI 命令 |
| call_rest_api | 调网关 REST API |
| batch_execute | 批量下发命令到多台设备 |
| get_device_info | 查设备基本信息（从 Host 表） |
| verify | 执行后验证（带超时重试轮询） |

## 4. 执行后验证 (Verify)

命令执行完后自动验证是否生效，带超时重试轮询。

### 4.1 流程

```
命令执行完 → Agent 决定验证策略 + 参数
    │
    ┌─► 执行验证检查
    │       ├── 通过 → 报告成功
    │       └── 未通过
    │           ├── 未超时 → 等待 interval → 重试
    │           └── 已超时 → 报告失败 + LLM 分析原因
    └───────────────────────────────────────────┘
```

### 4.2 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| timeout | 60s | 总超时时间 |
| interval | 5s | 重试间隔 |
| max_retries | 12 | 最大重试次数 |

LLM 可根据操作类型调整——端口监听 5s 够，固件升级可能 300s。

### 4.3 验证类型

```go
type Check struct {
    Type   string // port_open / file_exists / http_status / cli_output / custom_cmd
    Target string // 检查目标
    Expect string // 期望值
    HostID string
}
```

### 4.4 两层机制

1. **LLM 自主验证** — Agent loop 中 LLM 自己判断需要验证什么，自动调 tool 检查
2. **用户定义验证规则** — 可选预设条件（port_open:8080、file_exists:/path、http_status:200:url）

## 5. RAG 文档系统

### 5.1 文档入库流程

上传文档（markdown/PDF/文本）→ 解析 + 切片 → 打 metadata (vendor, cli_type, doc_type) → Embedding → 存入 documents 表

### 5.2 切片策略

| 文档类型 | 切片方式 |
|---------|---------|
| CLI 参考手册 | 按命令切——每条命令（名称+语法+参数+示例）为一个片段 |
| REST API 文档 | 按 endpoint 切——每个 API（路径+方法+参数+响应）为一个片段 |
| 故障排除手册 | 按问题切——每个问题+症状+解决方案为一个片段 |
| 通用文档 | 按段落/章节切，重叠 window |

### 5.3 检索流程 (search_docs tool)

用户意图 + 目标设备 vendor/cli_type → 构建查询 embedding → 向量检索 + metadata 过滤 → Top-K 片段（默认 5）→ 注入 LLM context

### 5.4 向量库

sqlite-vec（SQLite 向量扩展）。与现有 SQLite 技术栈一致，零额外依赖。

### 5.5 Embedding

抽象接口，支持多 provider：

| Provider | 模型 | 维度 |
|----------|------|------|
| OpenAI | text-embedding-3-small | 1536 |
| Anthropic Voyager | voyage-3 | 1024 |
| 本地 | ollama 等 | 可变 |

## 6. LLM Client + Prompt 设计

### 6.1 LLM Client 接口

```go
type LLMClient interface {
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
}
```

初期只实现 Claude provider。接口预留多 provider。

### 6.2 System Prompt 结构

```
你是 Spider 网关运维助手。

## 身份
帮助用户通过自然语言管理网关设备。根据用户意图生成 CLI 命令或 REST API 调用，执行后解读结果。

## 当前上下文
- 用户: {username}
- 可管理设备数: {host_count}
- 设备类型分布: {vendor/cli_type 统计}

## 安全规则
- 查询类命令可直接执行
- 配置修改、批量操作、升级必须先展示命令让用户确认
- 永远不要执行 format/erase/factory-reset 等破坏性命令
- 批量操作先在单台验证，确认无误再批量下发

## 命令生成规则
- 根据目标设备的 vendor 和 cli_type 生成对应语法
- 先调 search_docs 检索相关文档，确保命令语法正确
- 生成命令时标注 risk_level: safe/moderate/dangerous
```

### 6.3 Context Window 管理

| 策略 | 触发条件 | 行为 |
|------|---------|------|
| 滑动窗口 | message history > 50 条 | 保留最近 30 条，旧消息摘要压缩 |
| 摘要压缩 | token 数接近限制 | 调 LLM 总结旧对话，替换原始消息 |
| RAG 结果缓存 | 同一 vendor 重复检索 | 短期缓存避免重复 embedding 查询 |

### 6.4 对话标题自动生成

首轮对话完成后，额外调一次 LLM 生成简短标题（10 字以内）。

## 7. 前端设计

### 7.1 页面布局

Chat + 目标视图（方案 B）：左侧聊天区 + 右侧目标视图面板。

对话列表通过顶部栏下拉切换：`[≡ 对话列表 ▾] [当前对话标题]`

**最终设计 Mockup**: `docs/mockups/final-design.html`

备选方案参考: `docs/mockups/chat-layout.html`（方案 A/B/C 对比，最终选择 B）

### 7.2 对话样式

参考 Claude Code 终端风格：

- 等宽字体 + 深色终端背景
- `❯` prompt 符号标识用户输入
- Tool 调用显示为可折叠条（tool 名称 + 摘要），点击展开详情
- CLI 命令用代码块高亮展示
- 确认/取消按钮内联在消息中，标注 risk level
- 流式输出文本逐字显示

备选方案参考: `docs/mockups/chat-style.html`（Claude Code 风格详细 mockup）

### 7.3 目标视图

混合方案（方案 C）：统计摘要 + 热力矩阵 + 列表，支持视图切换。

备选方案参考: `docs/mockups/target-view.html`（方案 A/B/C 对比，最终选择 C）

核心功能：
- 顶部统计摘要（在线/离线/执行中/失败计数）
- 批量操作时显示进度条 + 热力矩阵（每台设备一个色块）
- 单台操作时切列表视图显示设备详情
- 失败设备自动置顶
- 设备状态通过 `device_update` SSE 事件实时更新
- 支持搜索过滤

### 7.4 Mockup 文件索引

| 文件 | 内容 | 说明 |
|------|------|------|
| `docs/mockups/final-design.html` | 最终确认设计 | 包含所有选定方案的完整 mockup + 设计要点 |
| `docs/mockups/chat-layout.html` | 布局方案对比 | A 经典 Chat / B Chat+设备面板 / C 三栏 → **选定 B** |
| `docs/mockups/chat-style.html` | 对话样式 | Claude Code 终端风格详细 mockup |
| `docs/mockups/target-view.html` | 目标视图方案对比 | A 分组折叠 / B 热力矩阵 / C 混合 → **选定 C** |

## 8. 数据模型

### 8.1 Host 表扩展（nullable 字段）

| 字段 | 类型 | 说明 |
|------|------|------|
| device_type | string | server / gateway / switch / router |
| vendor | string | huawei / cisco / juniper 等 |
| model | string | 设备型号 |
| cli_type | string | vrp / ios / junos 等 |
| firmware_version | string | 固件版本 |

### 8.2 conversations 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string (UUID) | 主键 |
| user_id | int | 关联用户 |
| title | string | 对话标题（LLM 自动生成或用户编辑） |
| created_at | datetime | |
| updated_at | datetime | |

### 8.3 messages 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string (UUID) | 主键 |
| conversation_id | string | 关联对话 |
| role | string | user / assistant / tool / system |
| content | text | JSON，支持 text + tool_use 混合 |
| created_at | datetime | |

### 8.4 documents 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | 主键 |
| vendor | string | 厂商 |
| cli_type | string | CLI 类型 |
| doc_type | string | cli_ref / api_ref / troubleshooting |
| title | string | 片段标题 |
| content | text | 原始文本 |
| embedding | blob | 向量 |
| source_file | string | 原始文件路径 |
| chunk_index | int | 在原文件中的位置 |

### 8.5 pending_confirmations 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string (UUID) | 即 request_id |
| conversation_id | string | |
| tool_name | string | |
| tool_input | text | JSON |
| risk_level | string | moderate / dangerous |
| status | string | pending / approved / denied / expired |
| created_at | datetime | |
| resolved_at | datetime | |

## 9. API 端点

### 9.1 Chat API

| 方法 | 端点 | 用途 |
|------|------|------|
| POST | /api/chat/conversations | 创建新对话 |
| GET | /api/chat/conversations | 列出用户对话 |
| GET | /api/chat/conversations/{id} | 获取对话详情 + 消息历史 |
| DELETE | /api/chat/conversations/{id} | 删除对话 |
| PATCH | /api/chat/conversations/{id} | 更新标题 |
| POST | /api/chat/conversations/{id}/messages | 发送消息（返回 SSE 流） |
| POST | /api/chat/conversations/{id}/confirm/{request_id} | 确认/拒绝操作 |
| POST | /api/chat/conversations/{id}/abort | 中止当前执行 |

### 9.2 文档管理 API

| 方法 | 端点 | 用途 |
|------|------|------|
| POST | /api/docs/upload | 上传文档 |
| GET | /api/docs | 列出文档 |
| DELETE | /api/docs/{id} | 删除文档 |
| POST | /api/docs/reindex | 重建索引 |

### 9.3 SSE 事件类型

| 事件 type | 用途 |
|-----------|------|
| text_delta | 助手文本流式输出 |
| tool_start | Tool 调用开始（名称、输入） |
| tool_result | Tool 执行结果 |
| confirm_required | 需用户确认（含 request_id、命令、risk_level） |
| verify_progress | 验证轮询进度 |
| batch_progress | 批量操作进度（完成数/总数/失败列表） |
| device_update | 设备状态变更（目标视图更新） |
| error | 错误信息 |
| done | 本轮对话结束 |

### 9.4 SSE 事件示例

```json
{"type":"text_delta","content":"检测到目标设备 GW-01 为华为 NE40E..."}
{"type":"confirm_required","request_id":"abc-123","tool":"execute_cli",
 "commands":["system-view","interface eth0","ip address 10.0.1.1 24"],
 "risk_level":"moderate","target_hosts":["GW-01"]}
{"type":"batch_progress","total":100,"completed":92,"failed":5,"running":3}
{"type":"verify_progress","attempt":3,"max_retries":12,
 "checks":[{"type":"port_open","target":"8080","status":"passed"}]}
{"type":"device_update","host_id":"xxx","status":"success","detail":"配置已生效"}
```

## 10. 文件结构

### 10.1 后端新增

```
internal/agent/agent.go       — Agent loop 核心
internal/agent/tools.go       — Tool 接口 + 注册
internal/agent/hooks.go       — Hook chain（风险分级）
internal/agent/verify.go      — 验证轮询逻辑
internal/agent/tools_cli.go   — execute_cli tool
internal/agent/tools_api.go   — call_rest_api tool
internal/agent/tools_docs.go  — search_docs tool (RAG)
internal/agent/tools_batch.go — batch_execute tool
internal/llm/client.go        — LLM client 接口
internal/llm/claude.go        — Claude API 实现
internal/rag/embedder.go      — Embedding 生成
internal/rag/store.go         — 向量存储 + 检索
internal/api/chat.go          — Chat HTTP handlers
internal/store/conversation.go — 对话持久化
internal/store/document.go    — 文档存储
```

### 10.2 前端新增

```
web/src/views/ChatView.vue           — 主页面（聊天区 + 目标视图）
web/src/components/ChatMessage.vue    — 单条消息渲染
web/src/components/TargetPanel.vue    — 目标视图面板
web/src/components/ConfirmBar.vue     — 确认/取消操作栏
web/src/components/VerifyProgress.vue — 验证轮询进度条
web/src/api/chat.ts                   — Chat API 客户端 + SSE 处理
```

### 10.3 数据库迁移

```sql
ALTER TABLE hosts ADD COLUMN device_type TEXT;
ALTER TABLE hosts ADD COLUMN vendor TEXT;
ALTER TABLE hosts ADD COLUMN model TEXT;
ALTER TABLE hosts ADD COLUMN cli_type TEXT;
ALTER TABLE hosts ADD COLUMN firmware_version TEXT;

CREATE TABLE conversations (...);
CREATE TABLE messages (...);
CREATE TABLE documents (...);
CREATE TABLE pending_confirmations (...);
```

## 11. LLM 与 Embedding 配置

Spider Chat Agent 需要连接 LLM 才能具备智能能力。在现有 Config 结构中扩展 LLM 和 Embedding 配置段。

### 11.1 配置文件 (config.yaml)

```yaml
llm:
  active: claude-sonnet        # 当前启用的模型 ID
  models:
    - id: claude-sonnet
      provider: claude
      api_key: sk-ant-xxx      # 或环境变量 SPIDER_LLM_APIKEY_claude-sonnet
      model: claude-sonnet-4-6
      max_tokens: 4096
    - id: claude-opus
      provider: claude
      api_key: sk-ant-xxx
      model: claude-opus-4-7
      max_tokens: 8192
    - id: gpt4o
      provider: openai
      api_key: sk-xxx
      model: gpt-4o
      max_tokens: 4096

embedding:
  active: openai-small         # 当前启用的 embedding 模型 ID
  models:
    - id: openai-small
      provider: openai
      api_key: sk-xxx
      model: text-embedding-3-small
      dimensions: 1536
    - id: voyage3
      provider: voyage
      api_key: pa-xxx
      model: voyage-3
      dimensions: 1024
```

### 11.2 Go 配置结构

```go
type LLMModelConfig struct {
    ID        string `yaml:"id"`
    Provider  string `yaml:"provider"`
    APIKey    string `yaml:"api_key"`
    Model     string `yaml:"model"`
    MaxTokens int    `yaml:"max_tokens"`
}

type LLMConfig struct {
    Active string           `yaml:"active"`
    Models []LLMModelConfig `yaml:"models"`
}

type EmbeddingModelConfig struct {
    ID         string `yaml:"id"`
    Provider   string `yaml:"provider"`
    APIKey     string `yaml:"api_key"`
    Model      string `yaml:"model"`
    Dimensions int    `yaml:"dimensions"`
}

type EmbeddingConfig struct {
    Active string                `yaml:"active"`
    Models []EmbeddingModelConfig `yaml:"models"`
}
```

加入现有 `Config` 结构：

```go
type Config struct {
    DataDir   string          `yaml:"data_dir"`
    LogLevel  string          `yaml:"log_level"`
    SSH       SSHConfig       `yaml:"ssh"`
    SSE       SSEConfig       `yaml:"sse"`
    Auth      AuthConfig      `yaml:"auth"`
    LLM       LLMConfig       `yaml:"llm"`       // 新增
    Embedding EmbeddingConfig `yaml:"embedding"`  // 新增
}
```

### 11.3 API Key 优先级

1. 环境变量 `SPIDER_LLM_APIKEY_{model_id}` / `SPIDER_EMBEDDING_APIKEY_{model_id}`（最高）
2. config.yaml 中模型的 `api_key` 字段
3. 未配置则该模型不可用

### 11.4 前端 SettingsView 扩展

SettingsView 新增 LLM 配置面板：
- 模型列表：显示所有已配置模型，标记当前启用的
- 添加模型：Provider 下拉 + Model 名称 + API Key（密文显示末 4 位）+ Max Tokens
- 切换启用：点击模型行的"启用"按钮，切换 `active` 字段
- 删除模型：删除非 active 的模型配置
- Embedding 模型同理，独立列表管理
- 保存后调 `PUT /api/settings` 更新

## 12. 导航变更

现有导航新增 Chat 入口。路由：`/chat`，可选带对话 ID：`/chat/{conversation_id}`。

HostsView 添加/编辑主机时，当 device_type 选为 gateway/switch/router 时展开设备属性字段。
