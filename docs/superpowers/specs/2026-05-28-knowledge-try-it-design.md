# 知识库接口「试一试」功能设计

## 背景

知识库存储 API 文档（如 Prometheus OpenAPI spec）。运维工程师查看接口时，需要直接对已配置的数据源发真实请求验证接口可用性。

## 用户场景

运维工程师在知识库浏览 API 文档，展开某个 entry card（如 `GET /api/v1/label/__name__/values`），点击「试一试」，选择数据源，填写参数，发送请求，查看响应。

## 设计决策

- **入口**：entry card 展开后内联，参数表格下方。与现有展开/收起模式一致，无需引入新 UI 模式。
- **数据源**：选择已配置的 `PrometheusSource`（含 base URL + auth），不需要用户手填 URL。
- **代理方式**：后端代理（方案 B）。前端直连会 CORS 失败；后端代理绕过 CORS，auth 凭据不暴露给浏览器。

## 架构

### 后端

新增文件 `internal/api/knowledge_try.go`，注册路由：

```
POST /api/v1/knowledge-entries/{id}/try
```

请求体：
```json
{
  "source_id": "string",
  "params": { "key": "value" }
}
```

响应体：
```json
{
  "status": 200,
  "body": "string (raw response body)",
  "latency_ms": 38
}
```

Handler 逻辑：
1. 取 entry by id → 解析 method + path（复用现有 `splitMethodPath`）
2. 取 `PrometheusSource` by source_id → `DecryptCredentials`
3. 构造 HTTP 请求：`baseURL + path + query params`，附加 auth header
4. 执行请求，记录耗时
5. 返回原始响应体 + 状态码 + latency_ms

错误处理：
- entry 不存在 → 404
- source 不存在 → 404
- 下游请求失败（网络、超时）→ 502，body 含错误信息
- 下游返回非 2xx → 正常返回（status + body），不视为错误

权限：与现有 knowledge 路由一致，需登录用户。

### 前端

仅修改 `web/src/views/KnowledgeView.vue`。

**新增 API 函数**（`web/src/api/knowledge.ts`）：

```typescript
export interface TryEntryRequest {
  source_id: string
  params: Record<string, string>
}

export interface TryEntryResult {
  status: number
  body: string
  latency_ms: number
}

export async function tryEntry(entryID: number, req: TryEntryRequest): Promise<TryEntryResult>
```

**新增状态**（局部，不污染全局）：

```typescript
const prometheusSources = ref<PrometheusSource[]>([])  // onMounted 加载一次
const tryOpen = ref(new Set<number>())                  // try panel 是否展开
const trySourceId = ref<Record<number, string>>({})     // 每个 entry 选中的 source
const tryParams = ref<Record<number, Record<string, string>>>({})
const tryResult = ref<Record<number, TryEntryResult | null>>({})
const tryLoading = ref(new Set<number>())
const tryError = ref<Record<number, string>>({})
```

**Try panel 结构**（entry card 展开后，`inline-detail` 内，响应示例之后）：

```
[ 试一试 ▲/▼ ]  ← 折叠按钮

数据源: [下拉 PrometheusSource]
<param_name>: [input]  ← 每个 parameter 生成一行
...
[ 发送 ]

响应区（有结果时显示）:
  200 OK · 38ms
  ┌──────────────────┐
  │ { "status": ...  │
  └──────────────────┘
```

参数输入行从 `entryDetails[entry.id].parameters` 自动生成，`in === 'query'` 的参数作为 query string，`in === 'path'` 的参数替换 path 中的 `{param_name}`。

**数据源加载**：`onMounted` 时调用 `listPrometheusSources()`，结果存入 `prometheusSources`。若列表为空，try panel 显示「暂无数据源，请先在系统设置中添加」。

## 不在范围内

- POST/PUT/PATCH 请求体编辑（当前知识库主要存 Prometheus API，均为 GET）
- 请求历史记录
- 非 Prometheus 数据源类型
