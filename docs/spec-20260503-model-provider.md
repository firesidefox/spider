# 模型供应商配置优化 — 设计规格

## 概述

将模型供应商配置从 config.yaml 迁移到 DB，优化交互流程为"添加即可用"，新增 OpenAI 兼容 LLM client，清理 Embedding 配置。

## 数据模型

### providers 表

```sql
CREATE TABLE IF NOT EXISTS providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,  -- 'anthropic' | 'openai'
    encrypted_api_key TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL DEFAULT '',
    selected_model TEXT NOT NULL DEFAULT '',
    is_active INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

### provider_models 表

```sql
CREATE TABLE IF NOT EXISTS provider_models (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_provider_models_provider_id ON provider_models(provider_id);
```

## API 端点

| 方法 | 端点 | 用途 |
|------|------|------|
| GET | /api/v1/providers | 列出供应商（含 selected_model、is_active、models 列表） |
| POST | /api/v1/providers | 创建供应商 → 自动获取模型 → 自动选第一个 → 首个自动启用 |
| PUT | /api/v1/providers/{id} | 更新供应商（name, type, api_key, base_url） |
| DELETE | /api/v1/providers/{id} | 删除供应商及其模型列表 |
| POST | /api/v1/providers/{id}/refresh | 重新获取模型列表（手动刷新） |
| PUT | /api/v1/providers/{id}/activate | 启用该供应商（停用其他） |
| PUT | /api/v1/providers/{id}/model | 切换该供应商的选中模型 `{model: "xxx"}` |

### GET /api/v1/providers 响应

```json
[
  {
    "id": "uuid",
    "name": "My Claude",
    "type": "anthropic",
    "base_url": "https://api.anthropic.com",
    "selected_model": "claude-sonnet-4-6",
    "is_active": true,
    "models": [
      {"model_id": "claude-opus-4-7", "display_name": "Claude Opus 4.7"},
      {"model_id": "claude-sonnet-4-6", "display_name": "Claude Sonnet 4.6"}
    ],
    "created_at": "...",
    "updated_at": "..."
  }
]
```

API Key 不在列表响应中返回。编辑时单独获取（或始终不返回，只接受写入）。

### POST /api/v1/providers 请求

```json
{
  "name": "My Claude",
  "type": "anthropic",
  "api_key": "sk-ant-xxx",
  "base_url": ""
}
```

后端处理流程：
1. 生成 UUID，加密 API Key，存入 providers 表
2. 调用供应商 API 获取模型列表，存入 provider_models 表
3. 如果获取到模型，selected_model 设为第一个
4. 如果是唯一供应商，自动设 is_active = 1
5. 返回完整供应商对象（含 models 列表）

## 前端交互

### 个人设置 → 模型供应商

供应商列表表格：

| 名称 | 类型 | 请求地址 | 模型 | 状态 | 操作 |
|------|------|----------|------|------|------|
| My Claude | Anthropic 兼容 | 默认 | [claude-sonnet-4-6 ▾] | 已启用 | 编辑 获取模型 |
| My OpenAI | OpenAI 兼容 | 默认 | [gpt-4o ▾] | 未启用 | 启用 编辑 获取模型 |

- **模型下拉框**：从 provider_models 读取，选中立即调 `PUT /providers/{id}/model` 生效
- **编辑**：点击后该行变为输入框（名称、类型、API Key、请求地址），保存/取消/删除
- **获取模型**：调 `POST /providers/{id}/refresh`，刷新下拉框
- **启用**：调 `PUT /providers/{id}/activate`，其他供应商自动停用
- **添加供应商**：右上角按钮，创建后自动获取模型、自动选第一个、首个自动启用

### Chat 页面

- 顶部栏显示当前模型名（只读），如 `claude-sonnet-4-6`
- `/model` 命令：显示当前模型 + 可用模型列表，点击切换

## 后端改动

### 新增

- `internal/store/provider_store.go` — ProviderStore CRUD + ProviderModelStore
- `internal/llm/openai.go` — OpenAI 兼容 LLM client（ChatStream 实现）
- `internal/models/provider.go` — Provider、ProviderModel 结构体

### 修改

- `internal/db/schema.go` — 新增 providers、provider_models 表
- `internal/api/providers.go` — 重写，从 config 操作改为 DB 操作
- `internal/api/handler.go` — 更新路由注册
- `internal/api/settings.go` — 移除 Model 相关代码
- `internal/agent/factory.go` — 从 DB 读取活跃供应商创建 LLM client
- `internal/config/config.go` — 移除 ModelConfig、ProviderConfig、EmbeddingConfig
- `internal/mcp/server.go` — App 添加 ProviderStore，移除 ConfigMu
- `cmd/spider/main.go` — 初始化 ProviderStore

### 删除

- config.go 中的 `ModelConfig`、`ProviderConfig`、`EmbeddingConfig` 及相关方法
- config.yaml 中的 `model` 和 `embedding` 配置段

## OpenAI 兼容 LLM Client

`internal/llm/openai.go`：

- 实现 `Client` 接口的 `ChatStream` 方法
- POST `{base_url}/v1/chat/completions` with `stream: true`
- Header: `Authorization: Bearer {api_key}`
- SSE 解析：`data: {"choices":[{"delta":{"content":"..."}}]}` 格式
- 工具调用：OpenAI function calling 格式转换为内部 ToolCall 格式
- 默认 base_url: `https://api.openai.com`

## 模型列表获取

`internal/llm/models.go`（已有，需更新）：

- `ListModels(providerType, apiKey, baseURL)` 保持不变
- Anthropic: GET `{base_url}/v1/models`
- OpenAI: GET `{base_url}/v1/models`
- 返回结果存入 provider_models 表（替换旧数据）
