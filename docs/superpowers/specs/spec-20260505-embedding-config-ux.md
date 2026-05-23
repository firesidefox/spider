# Embedding 配置 UX 改版设计文档

**状态：** 已实现 — combobox 字段、validate 端点（POST /api/v1/rag-config/validate）、模型列表获取均已落地

## 背景

个人设置 → 知识库 tab，当前 Embedding 配置卡片字段顺序为：请求地址 | 模型 | API Key。
本次改版调整字段顺序、将请求地址改为 combobox（可从现有供应商选择或手动输入）、模型改为 combobox（支持获取后下拉选择或手动输入），并新增验证功能。

---

## 最终布局

```
请求地址  [combobox________________________]
API Key   [________________________________]
模型      [combobox________________] [获取模型]
          [验证]
```

---

## 字段说明

### 请求地址（combobox）

- 下拉列表从 `GET /api/v1/providers` 获取，展示每个 provider 的 `name + base_url`
- 用户可选择已有供应商，也可直接手动输入任意 URL
- 选中已有供应商时：
  - `base_url` 自动填入请求地址
  - `api_key` 自动填入 API Key 字段（若 provider 有 api_key）
  - 记录 `selectedProviderId`（用于"获取模型"按钮）
- 手动输入时：`selectedProviderId = null`，API Key 不自动填充

### API Key

- 普通文本输入框（type="password"）
- 从供应商自动填充后可手动覆盖
- 后端存储时若字段为空字符串，保留原有 key（不覆盖）

### 模型（combobox + [获取模型]）

- 初始为空下拉列表，用户可手动输入
- [获取模型] 按钮：
  - 仅当 `selectedProviderId != null` 时可点击
  - 调用 `GET /api/v1/providers/:id/models`
  - 返回 `[{ id, provider_id, model_id, display_name }]`，填充下拉列表
  - 获取后用户可从列表选择，也可手动输入
- 获取中显示 loading 状态，失败显示错误提示

### [验证] 按钮

- 位于模型字段下方
- 调用 `POST /api/v1/rag-config/validate`（新增后端接口）
- 后端用当前配置（base_url、api_key、model）发送一次测试 embedding 请求
- 成功：显示绿色"✓ 配置有效"
- 失败：显示红色错误信息

---

## 后端新增接口

### POST /api/v1/rag-config/validate

**请求体：**
```json
{
  "type": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "model": "text-embedding-3-small"
}
```

**响应：**
- 200 OK：`{ "ok": true }`
- 400/503：`{ "error": "..." }`

后端逻辑：用传入参数构造 embedder，发送一条测试文本（如 `"test"`）的 embedding 请求，成功则返回 ok。

---

## 前端状态

```typescript
const providers = ref<Provider[]>([])
const selectedProviderId = ref<number | null>(null)
const modelOptions = ref<string[]>([])
const fetchingModels = ref(false)
const fetchModelsError = ref('')
const validating = ref(false)
const validateResult = ref<'ok' | 'error' | null>(null)
const validateError = ref('')
```

---

## 交互流程

1. 组件挂载：并行加载 `loadRagConfig()` + `loadProviders()`
2. 用户选择供应商：自动填充 base_url 和 api_key，记录 selectedProviderId
3. 用户手动修改请求地址：清空 selectedProviderId
4. 用户点击[获取模型]：调用 providers/:id/models，填充 modelOptions
5. 用户点击[验证]：调用 validate 接口，展示结果
6. 用户点击[保存]：调用 PUT /api/v1/rag-config

---

## 受影响文件

- `web/src/views/ProfileView.vue` — 改版 kb tab 的 Embedding 配置卡片
- `web/src/api/providers.ts` — 已有，确认 Provider 类型包含 api_key 和 models
- `internal/api/rag_config.go` — 新增 validateRagConfig handler
- `internal/api/handler.go` — 注册 POST /api/v1/rag-config/validate 路由
