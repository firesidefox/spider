# Spider 智能运维平台 — 产品需求规格说明书

**版本：** v0.4  
**日期：** 2026-05-23  
**状态：** 迭代中

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

Spider 是以 Claude Code 为中心的 SSH 代理网关和智能运维平台。

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

> 作为运维工程师，我希望对话一句话就能获得所有生产主机的磁盘、内存、CPU 使用率汇总，而不需要逐台 SSH 登录。

### 2.2 服务健康检查

> 作为 SRE，我希望一次性检查所有 web 节点上 nginx 和 redis 的运行状态，立刻定位异常节点。

### 2.3 应用部署

> 作为开发工程师，我希望通过一条指令完成多台服务器的应用更新，包括上传、停服、替换、重启、验证全流程。

### 2.4 配置分发

> 作为运维工程师，我希望将本地修改好的 nginx.conf 同步到所有 web 主机并安全重载，有错误立刻停止。

### 2.5 故障排查

> 作为 SRE，当某台主机响应变慢时，我希望 AI 自动收集诊断信息并给出根因分析，而不需要我手动逐条执行命令。

### 2.6 日志分析

> 作为开发工程师，我希望直接在 Claude 对话中分析远程服务器的错误日志，无需手动 scp。

### 2.7 安全审计

> 作为安全工程师，我希望批量检查所有主机上是否有非标准用户拥有 sudo 权限。

### 2.8 执行历史追溯

> 作为运维负责人，我希望能查询昨天在 db-01 上执行了哪些命令，用于事后审计。

### 2.9 AI 自然语言运维对话

> 作为运维工程师，我希望在 Chat 界面输入"检查所有华为网关的接口状态"，AI 自动检索设备文档、生成正确的 CLI 命令、批量执行并汇总结果，配置修改前需要我确认。

### 2.10 知识库辅助运维

> 作为网络工程师，我希望上传华为 VRP 命令手册后，AI 能自动查阅正确的命令语法，不再需要我手动查文档。

---

## 3. 功能需求规格

### 3.1 主机管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 添加主机 | 支持名称、IP、端口、标签、设备类型等基本信息 | P0 |
| 删除主机 | 按名称或 ID 删除，同时清理关联接入面和凭据 | P0 |
| 更新主机 | 更新 IP、标签、设备类型、厂商等字段 | P0 |
| 列出主机 | 支持按标签过滤，支持 JSON 格式输出 | P0 |
| 连通性测试 | SSH ping，返回连接状态和延迟 | P0 |
| 主机状态汇总 | 返回在线/离线/未知主机数量统计 | P0 |

**设备扩展字段：**

| 字段 | 说明 |
|------|------|
| device_type | server / gateway / switch / router |
| vendor | huawei / cisco / juniper 等 |
| model | 设备型号 |
| cli_type | vrp / ios / junos 等 |
| firmware_version | 固件版本 |

### 3.2 接入面（Access Face）

接入面是主机的访问入口抽象，一台主机可有多个接入面（如 SSH 接入面、REST API 接入面）。凭据绑定在接入面上，而非主机本身。

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 添加接入面 | 为主机添加 SSH、REST API 或 Prometheus 类型的接入面 | P0 |
| 更新接入面 | 更新认证信息、地址、知识库绑定 | P0 |
| 删除接入面 | 删除指定接入面 | P0 |
| 列出接入面 | 查看主机的所有接入面 | P0 |
| 知识库绑定 | 接入面可绑定特定知识库范围（specific）或不绑定（none） | P0 |

**SSH 接入面认证方式：**

| 类型 | 说明 |
|------|------|
| key | 引用已有 SSH Key 或内联私钥 |
| password | SSH 密码 |
| key_password | 带 passphrase 的私钥 |

### 3.3 SSH 远程执行

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 单台执行 | 在指定主机执行命令，返回 stdout/stderr/exit_code | P0 |
| 批量执行 | 按 tag 或 ID 列表并发执行，聚合结果 | P0 |
| 自定义超时 | 默认 30s，可按命令指定超时时间 | P0 |
| 连接池复用 | SSH 连接池，TTL 可配置（默认 300s） | P1 |
| 风险分级 | 命令按风险等级分类：safe / moderate / dangerous | P0 |
| 审批工作流 | moderate/dangerous 命令需用户确认后执行 | P0 |

### 3.4 文件传输

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 上传文件 | 本地文件 → 远程主机（SCP） | P0 |
| 下载文件 | 远程主机 → 本地（SCP） | P0 |

### 3.5 执行历史

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 记录执行 | 自动记录所有命令执行（命令、输出、状态、耗时、操作人） | P0 |
| 按主机过滤 | 查询指定主机的历史记录 | P0 |
| 按状态过滤 | 按 success/failed/timeout 过滤 | P0 |
| 分页查询 | 支持指定返回条数（默认 20）和偏移量 | P0 |
| 审计关联 | 记录操作人 user_id、风险等级、审批 ID | P0 |

### 3.6 MCP 接口（13 个工具）

MCP 传输协议：Streamable HTTP（`/mcp` 端点）。

| 工具名 | 说明 |
|--------|------|
| `list_hosts` | 列出主机，支持 tag 过滤 |
| `add_host` | 添加主机（支持 ssh_key_id 或 credential） |
| `remove_host` | 删除主机 |
| `update_host` | 更新主机信息 |
| `execute_command` | 在单台主机执行命令 |
| `execute_command_batch` | 按 tag 或 ID 列表批量执行 |
| `check_connectivity` | 测试 SSH 连通性 |
| `upload_file` | 上传本地文件到远程主机 |
| `download_file` | 从远程主机下载文件 |
| `get_execution_history` | 查询执行历史 |
| `list_ssh_keys` | 列出当前用户的 SSH 密钥 |
| `add_ssh_key` | 添加 SSH 私钥（自动解析指纹） |
| `remove_ssh_key` | 删除 SSH 密钥（被引用时拒绝） |

### 3.7 SSH 密钥管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 添加密钥 | 上传 SSH 私钥，支持带 passphrase，自动解析 SHA256 指纹 | P0 |
| 密钥列表 | 展示名称、指纹、创建时间，个人资源（用户隔离） | P0 |
| 删除密钥 | 删除前检查引用关系，被主机引用时返回 409 | P0 |
| 主机引用 | 添加/编辑主机时可从列表选择已有密钥，与内联粘贴互斥 | P0 |
| 加密存储 | 私钥使用 AES-256-GCM 加密，复用 master.key | P0 |
| 私钥不可读 | GET 接口永远不返回私钥内容，仅创建时接收 | P0 |

### 3.8 Web UI — 主机管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 主机列表 | 表格视图，显示名称、IP、状态、标签、最后连接时间 | P0 |
| 搜索与过滤 | 按名称/IP 搜索，按标签筛选 | P0 |
| 添加/编辑主机 | 图形化填写主机信息，支持三种认证方式 | P0 |
| 删除主机 | 单台删除，带确认弹窗 | P0 |
| 连通性测试 | 界面上一键 ping，实时显示结果 | P0 |
| 接入面管理 | 查看和编辑主机的接入面列表 | P0 |
| 知识库绑定 | 在主机详情页配置接入面的知识库绑定 | P0 |

### 3.9 Web UI — 命令执行

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 命令输入框 | 支持多行命令输入 | P0 |
| 目标选择 | 选择单台主机或按标签批量选择 | P0 |
| 实时输出流 | 通过 SSE 实时展示命令输出 | P0 |
| 执行状态指示 | 运行中 / 成功 / 失败 / 超时 状态标识 | P0 |

### 3.10 Web UI — 执行历史

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 历史列表 | 显示时间、主机、命令摘要、状态、耗时 | P0 |
| 日志详情 | 查看完整命令输出 | P0 |
| 搜索与过滤 | 按主机、时间范围、状态过滤 | P0 |
| 分页 | 支持分页浏览 | P0 |

### 3.11 多用户与权限控制

#### 3.11.1 用户账号管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 本地账号 | 用户名 + 密码登录（bcrypt 哈希存储） | P0 |
| 管理员创建账号 | Admin 可创建、禁用、删除账号 | P0 |
| 修改密码 | 用户可修改自己的密码 | P0 |
| 账号禁用 | Admin 可禁用账号，禁用后立即失效 | P0 |
| 登录会话 | JWT Token，有效期 24h | P0 |

#### 3.11.2 角色与权限（RBAC）

内置三级角色，不支持自定义角色：

| 角色 | 权限范围 |
|------|----------|
| `admin` | 全部权限：用户管理、主机管理、执行命令、查看审计日志 |
| `operator` | 主机管理（无删除）、执行命令、查看历史 |
| `viewer` | 只读：查看主机列表、查看执行历史 |

#### 3.11.3 API Token

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 生成 Token | 用户可生成命名 Token，用于 MCP 认证 | P0 |
| 撤销 Token | 立即失效 | P0 |
| Token 列表 | 查看自己创建的所有 Token | P0 |
| 过期时间 | 可设置过期时间，默认永不过期 | P1 |

#### 3.11.4 操作审计

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 记录操作人 | 所有执行记录关联用户 ID | P0 |
| 审计日志不可删除 | 审计记录只追加，不允许删除 | P0 |
| 审计日志查询 | 按用户、时间、操作类型过滤 | P0 |

### 3.12 AI Chat 智能运维对话

#### 3.12.1 Agent Engine

| 功能 | 描述 | 优先级 |
|------|------|--------|
| Agent Loop | 消息 → LLM → tool_use/文本 → 循环 | P0 |
| Tool 注册与分发 | 统一 Tool 接口，支持 RunCommand / RunCommandBatch / SearchDocs / PollUntil / CallAPI 等 | P0 |
| 风险分级 | 命令按 L1~L4 四级分类，safe（L1-2）/ moderate（L3）/ dangerous（L4） | P0 |
| Hook Chain | BeforeTool（风险拦截）/ AfterTool（审计记录） | P0 |
| 审批工作流 | moderate/dangerous 操作生成 approval 记录，等待用户确认后继续 | P0 |
| 中止执行 | 用户可随时中止当前 agent loop | P0 |
| 运行状态栏 | 输入框上方实时显示 agent 执行状态（当前工具、目标主机） | P0 |
| Explore-Plan-Act | Agent 处理任务时遵循探索→规划→执行顺序，通过 system prompt 约束 | P0 |
| 并发工具执行 | 同一 turn 内多个并发安全的工具调用并行执行，减少等待时间 | P1 |
| TodoTask 工具 | Agent 可创建和管理子任务，用户在对话中实时看到任务进度 | P1 |

**Agent 内置工具列表：**

| 工具名 | 说明 | 副作用 |
|--------|------|--------|
| `GetHosts` | 列出主机及接入面信息，支持 tag 过滤 | 只读 |
| `RunCommand` | 在单台主机执行 CLI 命令，需声明 risk_level 和 intent | 有 |
| `RunCommandBatch` | 在多台主机并行执行命令，需声明 risk_level 和 intent | 有 |
| `PollUntil` | 轮询主机直到条件满足或超时，用于部署后验证 | 只读 |
| `CallAPI` | 调用主机 REST API，GET 只读，POST/PUT/DELETE 有副作用 | 有（非 GET） |
| `SearchDocs` | 语义搜索知识库，返回相关文档片段 | 只读 |
| `Todo` | 创建和管理当前对话的子任务列表 | 有 |
| `QueryMetrics` | 对主机 Prometheus 接入面执行自由 PromQL 查询（即时或区间），返回原始 Prometheus JSON | 只读 |
| `GetTopology` | 获取网络拓扑数据（节点、边、分组） | 只读 |
| `GetTopologyContext` | 查询主机在拓扑中的位置和上下游关系 | 只读 |
| `CreateTask` | 保存已确认的自动化任务到数据库 | 有 |
| `invoke_skill` | 调用已安装的 Skill | 有 |

**工具提示词多层架构：**

每个 Agent 工具的行为规范分多层注入：

| 层 | 位置 | 内容 | 进入上下文时机 |
|----|------|------|--------------|
| `Description()` | 工具定义 | 一句话用途 + 副作用声明 + 阶段约束（极简） | 每次工具选择时 |
| `SystemPromptSection()` | `BuildSystemPrompt()` 静态段 | When to use / When NOT to use / 状态机约束 / reasoning 示例 | 每次对话开头一次（可缓存） |
| `Nudge`（运行时反馈） | 工具执行返回内容末尾 | 引导 Agent 维护任务状态或验证结果 | 每次写操作调用后 |

Nudge 规则：只读工具不加；`RunCommand`/`RunCommandBatch` 执行后追加 nudge 引导更新任务列表；`CallAPI` 非 GET 调用后追加 nudge 检查 status_code。`Todo` 工具在所有任务完成时触发 conditional nudge，要求产出可验证 artifact 而非自我评估。

**风险分级（L1~L4）：**

| 级别 | 含义 | 示例 | 默认行为 |
|------|------|------|---------|
| L1 | 只读查询 | df -h, show interface | 自动执行 |
| L2 | 标准变更 | systemctl restart nginx | 自动执行（ask 模式需确认） |
| L3 | 破坏性操作 | rm -rf, truncate table | 需用户确认 |
| L4 | 关键操作 | 删除主机、清空数据库 | 需二次确认 |

#### 3.12.2 对话管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 创建/删除对话 | 支持多轮对话，全量持久化 | P0 |
| 对话标题 | 首轮完成后 LLM 自动生成标题，支持手动重命名 | P0 |
| 对话列表 | 左侧持久侧边栏 | P0 |
| 消息历史 | 按对话 ID 加载完整消息历史 | P0 |
| 权限模式 | 对话级别的权限模式：ask（默认）/ auto / plan / readonly | P0 |
| Context 压缩 | 对话超过 token 阈值时自动将旧消息压缩为摘要，摘要缓存到 DB | P0 |
| 导出对话 | 导出当前对话为 Markdown 或 JSON 格式，浏览器直接下载 | P1 |

#### 3.12.3 Chat 前端界面

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 终端风格对话 | 等宽字体 + 深色背景 | P0 |
| 流式输出 | SSE 实时推送，文本逐字显示 + 光标动画 | P0 |
| Tool 调用渲染 | 可折叠工具卡片，显示名称、输入、结果、耗时 | P0 |
| 确认操作栏 | 内联确认/取消按钮，按 risk level 着色 | P0 |
| Markdown 渲染 | 助手消息支持 Markdown + 代码高亮 | P0 |
| 运行状态栏 | 输入框上方显示 agent 当前执行状态 | P0 |

### 3.13 知识库（Knowledge Base）

知识库采用三层结构：知识库组（Group）→ 文档（Document）→ 章节/条目（Section/Entry）。接入面可绑定特定知识库组，Agent 执行时自动检索相关内容。

#### 3.13.1 知识库组管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 创建知识库组 | 按厂商/设备类型/用途分组，支持名称和描述 | P0 |
| 更新/删除知识库组 | 管理组元数据 | P0 |
| 列出知识库组 | 支持过滤 | P0 |

#### 3.13.2 文档管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 上传文档 | 支持 Markdown/PDF/文本，自动解析切片 | P0 |
| 文档列表 | 按组过滤，显示标题、类型、状态 | P0 |
| 删除文档 | 同时清理关联章节和向量 | P0 |
| 重建索引 | 重新生成文档的向量嵌入 | P0 |

#### 3.13.3 向量检索

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 语义搜索 | sqlite-vec 向量库，Top-K 检索 | P0 |
| 范围过滤 | 按 vendor/cli_type/group 过滤检索范围 | P0 |
| 接入面绑定检索 | Agent 执行时按接入面绑定的 KB 范围自动检索 | P0 |

#### 3.13.4 Embedding 配置

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 多 Provider | 支持 OpenAI / Voyage / 本地 ollama | P0 |
| 模型列表 | 列出可用 embedding 模型 | P0 |
| 配置验证 | 保存前验证 embedding 模型可用性 | P0 |

### 3.14 LLM Provider 管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 添加 Provider | 配置 LLM 提供商（Claude / OpenAI 等）及 API Key | P0 |
| 更新/删除 Provider | 管理 Provider 配置 | P0 |
| 模型列表 | 每个 Provider 下可配置可用模型列表 | P0 |
| API Key 安全 | API Key 加密存储，展示时仅显示末 4 位 | P0 |
| 环境变量覆盖 | 环境变量优先级高于数据库配置 | P0 |

### 3.15 Skills 管理

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 列出 Skills | 列出系统内置和用户自定义 Skills | P0 |
| 上传自定义 Skill | 用户可上传自定义 Skill 文件 | P0 |
| 删除自定义 Skill | 删除用户上传的 Skill | P0 |
| Skills 安装包 | 提供 skills.tar.gz 下载，供 Claude Code 安装 | P0 |

### 3.16 网络拓扑

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 拓扑视图 | 可视化展示主机间的网络关系，节点按标签/设备类型着色 | P1 |
| 拓扑数据 API | 返回主机节点列表及连接关系，供前端渲染 | P1 |

### 3.17 CLI 工具（spdctl）

SSH 密钥管理仅通过 Web UI 和 REST API 操作，spdctl 不提供密钥子命令。

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
| 密码存储 | 用户密码 bcrypt 哈希存储 |
| Token 安全 | API Token 以 SHA-256 哈希存储，明文仅在创建时展示一次 |

### 4.3 可用性

| 要求 | 说明 |
|------|------|
| 零外部依赖 | SQLite 内嵌，单二进制运行，无需安装数据库或中间件 |
| 优雅关闭 | 收到 SIGTERM 后等待进行中的 SSH 会话完成（最长 30s）再退出 |
| systemd 支持 | 提供标准 systemd unit 文件，支持自动重启 |
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

### 5.1 组件说明

| 组件 | 职责 |
|------|------|
| MCP Layer | 处理 Claude Code 的 Streamable HTTP 连接和工具调用，参数校验，结果序列化 |
| REST API Layer | 为 Web UI 提供 HTTP JSON API，处理认证 |
| Service Layer | 业务逻辑：主机 CRUD、命令执行调度、接入面管理 |
| Agent Engine | AI 对话核心：agent loop + tool dispatch + hook chain + 风险分级 |
| Store Layer | SQLite 持久化，凭据加密/解密，对话/消息存储，查询封装 |
| SSH Layer | 连接池管理、SSH 命令执行、SCP 文件传输 |
| LLM Client | 多 Provider LLM 调用（Claude / OpenAI），流式输出 |
| RAG / Vec | sqlite-vec 向量检索，文档切片与 Embedding 生成 |
| Web UI | Vue 3 SPA，编译后嵌入 spider 二进制 |

### 5.2 关键数据流

**MCP 工具调用（execute_command 为例）：**

Claude Code 发起工具调用 → MCP Layer 解析参数 → ExecService 查询主机信息并解密凭据 → SSH Pool 获取或新建连接 → 执行命令收集输出 → 异步写入执行历史 → 返回结果给 Claude Code。

**AI Chat 对话流：**

用户发送消息 → Agent Engine 构建上下文 → 调用 LLM → 解析 tool_use → BeforeTool Hook 风险检查 → 执行工具 → AfterTool Hook 审计记录 → 结果追加上下文 → 循环直到 LLM 返回纯文本。

---

## 6. 数据模型

### 6.1 Host（主机）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| name | string | 主机名，全局唯一 |
| ip | string | IP 地址或域名 |
| port | int | SSH 端口，默认 22 |
| tags | string[] | 标签数组 |
| status | string | online / offline / unknown |
| device_type | string | server / gateway / switch / router（可空） |
| vendor | string | 设备厂商（可空） |
| model | string | 设备型号（可空） |
| cli_type | string | vrp / ios / junos 等（可空） |
| firmware_version | string | 固件版本（可空） |

### 6.2 AccessFace（接入面）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| host_id | UUID | 所属主机 |
| face_type | string | ssh / rest_api / prometheus |
| name | string | 接入面名称 |
| address | string | 连接地址 |
| auth_type | string | key / password / key_password |
| credential | blob | AES-256-GCM 加密的凭据 |
| ssh_key_id | string | 引用的 SSH Key ID（可空） |
| kb_mode | string | specific / none |
| knowledge_sources | string | 绑定的知识库来源（JSON），kb_mode=specific 时有效 |

### 6.3 ExecutionLog（执行记录）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| host_id | UUID | 目标主机 |
| command | string | 执行的命令 |
| output | string | stdout + stderr 合并 |
| exit_code | int | 退出码 |
| status | string | success / failed / timeout / running |
| started_at | datetime | 开始时间 |
| finished_at | datetime | 结束时间 |
| duration_ms | int | 耗时毫秒 |
| user_id | string | 操作人 |
| risk_level | string | safe / moderate / dangerous |
| approval_id | string | 关联审批记录 ID（可空） |

### 6.4 User（用户）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| username | string | 用户名，唯一 |
| password | string | bcrypt 哈希 |
| role | string | admin / operator / viewer |
| enabled | bool | 是否启用 |
| created_at | datetime | 创建时间 |
| last_login | datetime | 最后登录时间 |

### 6.5 ApiToken（API Token）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| user_id | string | 归属用户 |
| name | string | Token 名称 |
| token_hash | string | SHA-256 哈希，明文仅展示一次 |
| expires_at | datetime | 过期时间，NULL 表示永不过期 |
| created_at | datetime | 创建时间 |
| last_used | datetime | 最后使用时间 |

### 6.6 SSHKey（SSH 密钥）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键，k_ 前缀 |
| user_id | string | 归属用户 |
| name | string | 用户自定义名称，同用户下唯一 |
| encrypted_private_key | string | AES-256-GCM 加密的私钥 |
| encrypted_passphrase | string | 加密的 passphrase |
| fingerprint | string | SHA256 指纹 |

### 6.7 Conversation（对话）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| user_id | string | 归属用户 |
| title | string | LLM 自动生成或用户编辑 |
| permission_mode | string | 对话级权限模式 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 最后更新时间 |

### 6.8 Message（消息）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| conversation_id | UUID | 所属对话 |
| role | string | user / assistant / tool / system |
| content | string | JSON，text 类型消息的文本内容 |
| tool_calls | string | tool_use 类型消息的工具调用记录（JSON） |
| created_at | datetime | 创建时间 |

### 6.9 KnowledgeGroup（知识库组）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| name | string | 组名称 |
| description | string | 描述 |
| user_id | string | 归属用户 |
| created_at | datetime | 创建时间 |

### 6.10 Document（文档）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | 主键 |
| group_id | string | 所属知识库组 |
| vendor | string | 厂商标识 |
| cli_type | string | CLI 类型 |
| doc_type | string | cli_ref / api_ref / troubleshooting |
| title | string | 文档标题 |
| content | string | 文档内容 |
| embedding | blob | 向量 |
| source_file | string | 原始文件名 |
| chunk_index | int | 切片序号 |

### 6.11 Approval（审批记录）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| conversation_id | UUID | 所属对话 |
| tool_name | string | 工具名称 |
| tool_input | string | 工具输入（JSON） |
| risk_level | string | moderate / dangerous |
| status | string | pending / approved / denied / expired |
| created_at | datetime | 创建时间 |
| resolved_at | datetime | 处理时间 |

### 6.12 Provider（LLM Provider）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| name | string | Provider 名称 |
| provider_type | string | claude / openai 等 |
| encrypted_api_key | string | 加密存储的 API Key |
| base_url | string | 自定义 API 地址（可空） |
| enabled | bool | 是否启用 |

---

## 7. 接口规范

### 7.1 MCP 工具详细规格

| 工具 | 必填参数 | 可选参数 | 返回 |
|------|----------|----------|------|
| `list_hosts` | — | `tag: string` | 主机列表 |
| `add_host` | `name`, `ip`, `user`, `auth_type` | `port`, `key`, `password`, `passphrase`, `tag` | 成功消息 + host_id |
| `remove_host` | `name_or_id` | — | 成功消息 |
| `update_host` | `name_or_id` | `ip`, `user`, `tag`, `port` 等 | 成功消息 |
| `execute_command` | `host`, `command` | `timeout: int` | stdout/stderr/exit_code |
| `execute_command_batch` | `command` | `tag`, `hosts: []string`, `timeout: int` | 各主机结果汇总 |
| `check_connectivity` | `host` | — | `{connected, latency_ms}` |
| `upload_file` | `host`, `local_path`, `remote_path` | — | 成功消息 + 文件大小 |
| `download_file` | `host`, `remote_path`, `local_path` | — | 成功消息 + 文件大小 |
| `get_execution_history` | — | `host`, `n: int` | 执行记录列表 |

### 7.2 REST API 端点

所有 REST API 路径前缀为 `/api/v1`。

**主机管理**
```
GET    /api/v1/hosts                     列出主机（支持 ?tag= 过滤）
POST   /api/v1/hosts                     添加主机
GET    /api/v1/hosts/:id                 主机详情
PUT    /api/v1/hosts/:id                 更新主机
DELETE /api/v1/hosts/:id                 删除主机
POST   /api/v1/hosts/:id/ping            连通性测试
GET    /api/v1/hosts/statuses            主机状态汇总
GET    /api/v1/hosts/:id/faces           列出接入面
POST   /api/v1/hosts/:id/faces           添加接入面
PUT    /api/v1/hosts/:id/faces/:faceId   更新接入面
DELETE /api/v1/hosts/:id/faces/:faceId   删除接入面
```

**命令执行**
```
POST   /api/v1/exec                      执行命令（单台）
POST   /api/v1/exec/batch                批量执行
GET    /api/v1/exec/stream               实时输出（SSE）
```

**执行历史**
```
GET    /api/v1/logs                      历史列表（支持 ?host= ?status= ?limit= ?offset=）
GET    /api/v1/logs/:id                  日志详情
```

**用户与认证**
```
GET    /api/v1/me                        当前用户信息
PUT    /api/v1/me/password               修改密码
GET    /api/v1/me/prefs                  用户偏好
PUT    /api/v1/me/prefs                  更新用户偏好
GET    /api/v1/me/ssh-keys               SSH 密钥列表
POST   /api/v1/me/ssh-keys               添加 SSH 密钥
DELETE /api/v1/me/ssh-keys/:id           删除密钥（被引用时 409）
GET    /api/v1/users                     用户列表（Admin）
POST   /api/v1/users                     创建用户（Admin）
DELETE /api/v1/users/:id                 删除用户（Admin）
GET    /api/v1/tokens                    Token 列表
POST   /api/v1/tokens                    创建 Token
DELETE /api/v1/tokens/:id                撤销 Token
```

**AI Chat**
```
POST   /api/v1/chat/conversations                     创建新对话
GET    /api/v1/chat/conversations                     列出用户对话
GET    /api/v1/chat/conversations/:id                 获取对话详情 + 消息历史
DELETE /api/v1/chat/conversations/:id                 删除对话
PUT    /api/v1/chat/conversations/:id                 更新标题
GET    /api/v1/stream                                 SSE 流（全局，含 approval 待确认推送）
POST   /api/v1/approval/approve                       确认操作
POST   /api/v1/approval/reject                        拒绝操作
```

**知识库**
```
GET    /api/v1/knowledge-groups          列出知识库组
POST   /api/v1/knowledge-groups          创建知识库组
GET    /api/v1/knowledge-groups/:id      知识库组详情
PUT    /api/v1/knowledge-groups/:id      更新知识库组
DELETE /api/v1/knowledge-groups/:id      删除知识库组
GET    /api/v1/knowledge-documents/:id   文档详情
PUT    /api/v1/knowledge-documents/:id   更新文档
DELETE /api/v1/knowledge-documents/:id   删除文档
GET    /api/v1/knowledge-sections/:id    章节详情
PUT    /api/v1/knowledge-sections/:id    更新章节
DELETE /api/v1/knowledge-sections/:id    删除章节
GET    /api/v1/documents                 列出文档
POST   /api/v1/documents                 上传文档
GET    /api/v1/documents/search          语义搜索
```

**LLM Provider**
```
GET    /api/v1/providers                 列出 Provider
POST   /api/v1/providers                 添加 Provider
GET    /api/v1/providers/:id             Provider 详情
PUT    /api/v1/providers/:id             更新 Provider
DELETE /api/v1/providers/:id             删除 Provider
GET    /api/v1/rag-config                RAG embedding 配置
POST   /api/v1/rag-config                更新 RAG 配置
POST   /api/v1/rag-config/validate       验证 embedding 模型
GET    /api/v1/rag-config/models         列出可用 embedding 模型
```

**Skills**
```
GET    /api/v1/skills                    列出 Skills
GET    /api/v1/skills/:source/:name      Skill 详情
PUT    /api/v1/skills/custom/:name       上传自定义 Skill
DELETE /api/v1/skills/custom/:name       删除自定义 Skill
GET    /api/v1/install/skills.tar.gz     下载 Skills 安装包
```

**系统**
```
GET    /api/v1/settings                  系统配置
GET    /api/v1/topology                  网络拓扑数据
GET    /api/v1/notify-channels           通知渠道列表
GET    /health                           健康检查
GET    /version                          版本信息
```

### 7.3 SSE 事件类型

| 事件 type | 用途 |
|-----------|------|
| text_delta | 助手文本流式输出 |
| tool_start | Tool 调用开始（名称、输入） |
| tool_result | Tool 执行结果 |
| confirm_required | 需用户确认（含 approval_id、命令、risk_level） |
| batch_progress | 批量操作进度 |
| device_update | 设备状态变更 |
| error | 错误信息 |
| done | 本轮对话结束 |

---

## 8. 安全设计

### 8.1 凭据安全

- **加密算法**：AES-256-GCM，提供认证加密，防止篡改
- **密钥管理**：master.key 在首次启动时本地随机生成，文件权限 600，不进入版本控制，不可通过 API 导出
- **AI 隔离**：Claude Code 通过 MCP 调用时，spider 在进程内解密凭据并直接建立 SSH 连接，凭据明文不出进程边界

### 8.2 访问控制

- **认证方式**：Web UI 使用 JWT（HS256，24h 有效期）；MCP 使用 API Token（Bearer）
- **RBAC 执行**：Service Layer 统一鉴权，MCP Layer 和 REST API Layer 不做业务逻辑判断
- **最小权限**：Viewer 角色无法触发任何写操作或命令执行

### 8.3 传输安全

- **生产环境建议**：通过 nginx 或 caddy 反向代理提供 HTTPS，spider 本身监听 localhost
- **CORS 配置**：REST API 支持配置允许的 Origin，默认仅允许同源

### 8.4 审计

- 所有 MCP 工具调用写入执行历史（含命令、输出、状态、操作人）
- 审计记录不提供删除 API，仅支持查询

---

## 9. 部署方案

### 9.1 单用户本地部署

适用于个人工程师，5 分钟完成安装：

1. 编译安装：`make install`（安装到 `$GOPATH/bin`）
2. 启动服务：`spider serve`（数据存储在 `~/.spider/`）
3. 注册 MCP：`claude mcp add --transport http spider http://localhost:8000/mcp`
4. 添加主机：通过 `spdctl host add` 或 Web UI

### 9.2 团队服务器部署

适用于团队共享实例，部署在跳板机或内网服务器：

- 使用 systemd 管理进程，支持自动重启
- 通过环境变量 `SPIDER_DATA_DIR` 指定数据目录
- 建议通过 nginx/caddy 反向代理提供 HTTPS

### 9.3 Docker 部署（规划中）

提供官方 Docker 镜像，支持 docker-compose 一键部署，数据目录通过 volume 挂载持久化。

---

## 10. 产品路线图

### 10.1 已完成功能

| 阶段 | 主要内容 |
|------|----------|
| **基线** | MCP Streamable HTTP Server、spdctl CLI、SSH 执行、文件传输、执行历史、AES-256-GCM 凭据加密 |
| **Phase 1** | Web UI：主机管理界面、实时命令执行、历史日志查看 |
| **Phase 2** | 多用户与权限控制：账号管理、RBAC、API Token、操作审计、SSH 密钥管理 |
| **Phase 2.5** | AI Chat：Agent Engine、知识库系统、多模型配置、对话管理、终端风格 Chat UI、接入面 KB 绑定、运行状态栏、LLM Provider 管理、Skills 管理 |

### 10.2 规划中功能

#### Phase 3 — 告警与监控

目标：主动监控主机状态，在异常发生时及时通知相关人员。

**主机状态监控**

| 功能 | 描述 | 优先级 |
|------|------|--------|
| SSH 连通性检测 | 定期 ping 所有主机，检测在线/离线状态 | P0 |
| 状态变更通知 | 主机从在线变为离线时触发告警 | P0 |
| 检测间隔配置 | 默认 60s，可按主机组配置 | P1 |

**阈值告警规则**

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 规则配置 | 定义检测命令、解析表达式、阈值、比较运算符 | P0 |
| 目标主机 | 按标签指定告警规则适用的主机范围 | P0 |
| 告警状态 | firing（触发）/ resolved（恢复）/ acknowledged（已确认） | P0 |
| 规则启用/禁用 | 临时禁用规则而不删除 | P1 |

**通知渠道**

| 渠道 | 优先级 |
|------|--------|
| 钉钉 Webhook | P0 |
| Slack Webhook | P0 |
| Email（SMTP） | P1 |

**监控仪表盘**

| 功能 | 描述 | 优先级 |
|------|------|--------|
| 主机状态总览 | 在线/离线/未知主机数量，状态分布 | P0 |
| 活跃告警列表 | 当前 firing 状态的告警，按严重程度排序 | P0 |
| 执行统计 | 近 7 天执行次数、成功率趋势 | P1 |

**Prometheus 告警集成**

前提：主机已配置 prometheus 类型接入面（`base_url` 指向 Prometheus 实例）。

| 功能 | 描述 | 优先级 |
|------|------|--------|
| `GetAlerts` Agent 工具 | 查询主机 Prometheus 活跃 firing 告警，支持 label 过滤（如 `severity=critical`） | P0 |
| Alertmanager Webhook 接收 | `POST /api/webhooks/alertmanager/:face_id?token=<token>`，接收标准 Alertmanager v4 webhook payload | P0 |
| 告警自动创建任务 | firing 告警自动创建 Task（标题 `[Alert] <alertname> on <host_name>`，含 labels + annotations），状态 pending，不自动启动 agent | P0 |
| Webhook Token 管理 | 首次请求时生成 32 字节随机 token，存入 config 表；在 Settings 页展示，供 Alertmanager receiver 配置 | P0 |

#### 未来探索

- Docker 镜像发布
- 多 spider 实例联邦
- Webhook 触发器

### 10.3 阶段依赖关系

基线 → Phase 1（Web UI）→ Phase 2（多用户）→ Phase 2.5（AI Chat）→ Phase 3（告警）

Phase 3 依赖 Phase 2 的用户系统（告警通知需要关联用户）。

---

*本文档由 Spider 项目团队维护，随产品迭代持续更新。*
