# Prometheus 集成设计

**日期：** 2026-05-24  
**状态：** 已批准

## 概述

为 spider.ai 新增 Prometheus 查询能力。Prometheus 建模为独立的监控数据源（`prometheus_sources`），与 `AccessFace`（接入面，表达"如何访问主机"）分离——Prometheus 是观测源，方向和语义不同，不应混用。数据源通过绑定关系（`prometheus_bindings`）关联到拓扑层或单台主机。新增一个 Agent 工具执行自由 PromQL 查询。

## 1. 数据模型

### 1.1 `prometheus_sources`（监控数据源定义）

存储 Prometheus 实例的连接信息，不含作用域——作用域在 bindings 表中。

```sql
CREATE TABLE prometheus_sources (
  id                  TEXT PRIMARY KEY,
  name                TEXT NOT NULL,
  base_url            TEXT NOT NULL,                  -- 如 http://prom:9090
  timeout_seconds     INTEGER NOT NULL DEFAULT 30,    -- HTTP 超时，0 表示用默认值
  auth_type           TEXT NOT NULL DEFAULT 'none',   -- none | basic | bearer
  username            TEXT NOT NULL DEFAULT '',       -- auth_type=basic 时使用
  encrypted_password  TEXT NOT NULL DEFAULT '',       -- auth_type=basic 时使用，加密存储
  encrypted_token     TEXT NOT NULL DEFAULT '',       -- auth_type=bearer 时使用，加密存储
  skip_tls_verify     INTEGER NOT NULL DEFAULT 0,     -- 1=跳过 TLS 验证（内网自签证书）
  created_at          DATETIME NOT NULL,
  updated_at          DATETIME NOT NULL
)
```

加密字段（`encrypted_password`、`encrypted_token`）均经 `crypto.Manager` 加密，模式与 `AccessFace.EncryptedCred` 一致。`username` 明文存储（非敏感）。

### 1.2 `prometheus_bindings`（作用域绑定）

将数据源绑定到作用域，支持两种类型：

| scope_type | 覆盖范围 | 唯一约束 |
|---|---|---|
| `topology_layer` | 指定拓扑中指定层的所有节点 | UNIQUE(topology_id, layer) |
| `host` | 单台主机（覆盖拓扑层绑定） | UNIQUE(host_id) |

```sql
CREATE TABLE prometheus_bindings (
  id           TEXT PRIMARY KEY,
  source_id    TEXT NOT NULL REFERENCES prometheus_sources(id) ON DELETE CASCADE,
  scope_type   TEXT NOT NULL CHECK (scope_type IN ('topology_layer', 'host')),
  topology_id  TEXT REFERENCES topologies(id) ON DELETE CASCADE, -- scope_type=topology_layer 时有值
  layer        TEXT,                                              -- scope_type=topology_layer 时有值
  host_id      TEXT REFERENCES hosts(id) ON DELETE CASCADE,      -- scope_type=host 时有值
  created_at   DATETIME NOT NULL
);
CREATE UNIQUE INDEX idx_pb_topology_layer ON prometheus_bindings(topology_id, layer)
  WHERE scope_type = 'topology_layer';
CREATE UNIQUE INDEX idx_pb_host ON prometheus_bindings(host_id)
  WHERE scope_type = 'host';
```

### 1.3 数据源查找逻辑（给定 host_id）

主机级绑定优先，拓扑层绑定兜底：

```
1. 查 bindings WHERE scope_type='host' AND host_id=?
2. 未找到 →
     查 topology_nodes WHERE host_id=? → 得到 (topology_id, layer)
     查 bindings WHERE scope_type='topology_layer' AND topology_id=? AND layer=?
3. 仍未找到 → 返回错误"该主机未配置 Prometheus 数据源"
```

## 2. 配置入口（UI）

**三层结构：**

| 层级 | 入口位置 | 操作 |
|---|---|---|
| 数据源管理 | 系统设置 → Data Sources → Prometheus | 增删改 `prometheus_sources`（名称、URL、认证） |
| 拓扑层绑定 | 拓扑详情页 → 按层配置 | 从已有 sources 中选择，绑定到某业务的某层 |
| 主机覆盖绑定 | 主机详情页 → "监控源"区块（与接入面列表分离） | 从已有 sources 中选择，绑定到单台主机，覆盖拓扑层配置 |

数据源在 Settings 中统一管理，绑定关系在各自的上下文页面中配置。

## 3. Agent 工具：`ListMetrics`

列出指定主机在 Prometheus 中存在的所有指标名，供 Agent 构造 PromQL 前发现可用指标。

**输入参数：**

```json
{
  "host_id": "string（必填）— 目标主机 ID",
  "filter":  "string（可选）— 指标名前缀过滤，如 'node_cpu'"
}
```

**行为：**
- 同 §1.3 解析 Prometheus 数据源
- 调 `GET /api/v1/label/__name__/values?match[]={instance="<IP>:9100"}`
- 返回该主机所有指标名列表；有 `filter` 时做前缀过滤
- 风险等级：`L1`（只读），并发安全

**System prompt 指导：**
- 在 `QueryMetrics` 之前调用，用于不确定指标名时的发现
- 有 `filter` 时缩小结果范围，减少 token 消耗

## 4. Agent 工具：`QueryMetrics`

对指定主机执行自由 PromQL 查询。

**输入参数：**

```json
{
  "host_id": "string（必填）— 目标主机 ID",
  "query":   "string（必填）— PromQL 表达式",
  "start":   "string（可选）— RFC3339 或 Unix 时间戳",
  "end":     "string（可选）— RFC3339 或 Unix 时间戳",
  "step":    "string（可选）— 步长，如 '1m'、'30s'",
  "raw":     "bool（可选）— 返回原始 Prometheus JSON，默认 false"
}
```

**行为规则：**
- `start` 与 `end` 必须同时提供或同时省略，仅提供其一报错
- 两者省略 → 即时查询（`GET /api/v1/query`）
- 两者提供 → 区间查询（`GET /api/v1/query_range`）
- `step` 缺省值：`(end - start) / 100`，最小 1s
- 最大时间窗口：7 天；最大数据点数：10,000 — 超出直接报错，不截断
- 数据源通过 §1.3 逻辑自动解析，未找到则报错
- 风险等级：`L1`（只读），并发安全

**输出（默认）：** 摘要格式，非原始 JSON
- `result_type`、`series_count`，每条序列：`metric` 标签、`latest`、`min`、`max`、`avg`、最多 20 个样本点
- `raw: true` 时返回完整 Prometheus JSON（调试用）

**System prompt 指导：**
- 工具自动解析主机对应的数据源，无需用户提供 Prometheus URL
- PromQL 使用主机 IP 构造 label selector，如 `node_cpu_seconds_total{instance="<IP>:9100"}`
- 当前状态用即时查询，趋势分析用区间查询
- 避免在大集群上使用无 label 过滤的裸查询（如 `{}`）

## 5. 内部 Prometheus 客户端

```go
// internal/prometheus/client.go
type Client struct {
    baseURL        string
    authType       string
    username       string
    password       string // 解密后
    token          string // 解密后
    timeoutSeconds int
    skipTLSVerify  bool
}
func NewClient(source *models.PrometheusSource, decryptedPassword, decryptedToken string) *Client
func (c *Client) QueryInstant(ctx context.Context, query string, ts time.Time) (*QueryResult, error)
func (c *Client) QueryRange(ctx context.Context, query, start, end, step string) (*QueryResult, error)
func (c *Client) ListMetricNames(ctx context.Context, selector string) ([]string, error)
```

轻量 HTTP 封装。不做重试。超时 30s（可由 context 取消）。

## 6. 范围外（本次不做）

- Grafana 集成
- 告警集成（GetAlerts 工具、Alertmanager Webhook）— 见 PRD
- 单台主机多条 binding（每台主机最多一条 host 级覆盖）
