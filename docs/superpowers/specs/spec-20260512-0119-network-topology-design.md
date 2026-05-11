# 网络拓扑功能设计

**日期：** 2026-05-12  
**状态：** 待实现

---

## 1. 背景与目标

Spider.ai 管理混合云基础设施（云服务器、本地机器、IDC 设备）。用户需要描述这些设备之间的流量关系，并让 Agent 能够查询"某台设备的上下游是什么"。

**两个核心目标：**
1. **可视化 UI** — 用户在浏览器里看到部署拓扑，理解流量走向
2. **LLM 可查询** — Agent 通过工具获取结构化的节点+边数据，用于推理

---

## 2. 核心概念

| 概念 | 说明 |
|------|------|
| **拓扑（Topology）** | 一个命名的部署环境，如"生产环境"、"测试集群" |
| **分组（Group）** | 拓扑内的节点分类，如"防火墙"、"应用LB"，决定节点颜色 |
| **节点（Node）** | 拓扑中的一个设备，可选绑定一个 Host |
| **边（Edge）** | 节点间的有向连接，表示流量方向（from → to） |

**节点与 Host 的关系：**
- 节点可以绑定一个已有 Host（获得 IP、访问能力等）
- 节点也可以是占位符（设备尚未录入系统），只有名称和角色
- 绑定状态通过颜色区分：已绑定=分组色，未绑定=灰色

---

## 3. 数据模型

新增 3 张表，不修改现有表。

### 3.1 topologies

```sql
CREATE TABLE topologies (
    id         TEXT PRIMARY KEY,
    name       TEXT UNIQUE NOT NULL,
    notes      TEXT DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

### 3.2 topology_groups

```sql
CREATE TABLE topology_groups (
    id          TEXT PRIMARY KEY,
    topology_id TEXT NOT NULL REFERENCES topologies(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT '#3b82f6',
    sort_order  INTEGER DEFAULT 0,
    created_at  DATETIME NOT NULL
);
```

`color` 存 hex 值，由用户创建分组时指定或系统自动分配。

### 3.3 topology_nodes

```sql
CREATE TABLE topology_nodes (
    id          TEXT PRIMARY KEY,
    topology_id TEXT NOT NULL REFERENCES topologies(id) ON DELETE CASCADE,
    group_id    TEXT NOT NULL REFERENCES topology_groups(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    role        TEXT DEFAULT '',
    host_id     TEXT REFERENCES hosts(id) ON DELETE SET NULL,
    notes       TEXT DEFAULT '',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);
```

`host_id` 为 NULL 表示占位节点。

### 3.4 topology_edges

```sql
CREATE TABLE topology_edges (
    id          TEXT PRIMARY KEY,
    topology_id TEXT NOT NULL REFERENCES topologies(id) ON DELETE CASCADE,
    from_node   TEXT NOT NULL REFERENCES topology_nodes(id) ON DELETE CASCADE,
    to_node     TEXT NOT NULL REFERENCES topology_nodes(id) ON DELETE CASCADE,
    created_at  DATETIME NOT NULL
);
```

---

## 4. YAML 导入格式

支持用户提供的模板格式，导入时：

1. 按 `layers` 创建分组，`name` 作为分组名
2. 按 `devices` 创建节点，`role` 字段写入节点 role
3. 按 `upstream` 字段创建边（upstream 中的每个名称 → 当前节点）
4. `known_devices` 中的设备按 `name` 或 `ip` 匹配现有 Host，匹配成功则绑定 `host_id`

导入是幂等的：同名拓扑再次导入时更新而非重复创建。

---

## 5. API

遵循现有 `mux.HandleFunc` 模式，新增以下路由：

```
GET    /api/v1/topologies              列出所有拓扑
POST   /api/v1/topologies              创建拓扑
GET    /api/v1/topologies/{id}         获取拓扑（含分组、节点、边）
PUT    /api/v1/topologies/{id}         更新拓扑基本信息
DELETE /api/v1/topologies/{id}         删除拓扑

POST   /api/v1/topologies/{id}/import  YAML 导入

GET    /api/v1/topologies/{id}/groups              列出分组
POST   /api/v1/topologies/{id}/groups              创建分组
PUT    /api/v1/topologies/{id}/groups/{gid}        更新分组
DELETE /api/v1/topologies/{id}/groups/{gid}        删除分组

GET    /api/v1/topologies/{id}/nodes               列出节点
POST   /api/v1/topologies/{id}/nodes               创建节点
PUT    /api/v1/topologies/{id}/nodes/{nid}         更新节点（含绑定/解绑 host）
DELETE /api/v1/topologies/{id}/nodes/{nid}         删除节点

GET    /api/v1/topologies/{id}/edges               列出边
POST   /api/v1/topologies/{id}/edges               创建边
DELETE /api/v1/topologies/{id}/edges/{eid}         删除边
```

`GET /api/v1/topologies/{id}` 返回完整拓扑数据：

```json
{
  "id": "...",
  "name": "生产环境",
  "groups": [
    { "id": "...", "name": "防火墙", "color": "#3b82f6" }
  ],
  "nodes": [
    { "id": "...", "name": "fw-01", "role": "防火墙", "group_id": "...", "host_id": "...", "host_name": "aisg-v706", "ip": "10.2.6.247" }
  ],
  "edges": [
    { "id": "...", "from_node": "...", "to_node": "..." }
  ]
}
```

---

## 6. Agent 工具

新增 `GetTopology` 工具，供 Agent 查询拓扑关系：

- **Description:** `Get topology data including groups, nodes, and edges. Read-only. Use in Explore phase.`
- **参数：** `topology_id` 或 `topology_name`
- **返回：** 同上述 GET 接口的 JSON 结构

---

## 7. UI 设计

### 7.1 入口

主机管理页新增"拓扑"Tab（与现有 hosts/access-faces 等 Tab 并列）。

### 7.2 布局

```
┌─────────────┬──────────────────────────────┬──────────────┐
│  拓扑列表   │         拓扑画布              │  节点详情    │
│             │                               │  （点击后）  │
│  生产环境 ◀ │  [分组色条] 防火墙            │              │
│  测试集群   │    [aisg-v706] [ecs-tencen…]  │  主机名      │
│  IDC-B      │                               │  节点名      │
│             │  [分组色条] 链路LB            │  IP          │
│  + 新建     │    [local-110]                │  角色        │
│             │                               │  主机 ↗      │
│             │  [分组色条] 应用LB            │  ──────      │
│             │    [local-201] [app-lb-02]    │  上游        │
│             │                               │  下游        │
└─────────────┴──────────────────────────────┴──────────────┘
```

### 7.3 节点视觉规则

| 状态 | 方块边框 | 方块背景 | 文字 |
|------|----------|----------|------|
| 已绑定 Host | 分组色 | 分组色（深色调） | 分组色（浅色调），显示主机名 |
| 未绑定（占位） | `#374151` | `#1a1a1a` | `#374151`，30% 透明度，显示节点名 |

主机名超过 10 字符截断为 `前10字符…`，详情面板显示完整名称。

### 7.4 分组视觉规则

每个分组左侧一条 3px 竖色条（颜色 = 分组色），分组名在色条右侧以小号大写字母显示。

### 7.5 连线规则

- 已绑定节点发出的边：实线，颜色为 from 节点分组色（低透明度）
- 未绑定节点发出的边：虚线，`#1f2937`

### 7.6 布局引擎

使用 **Cytoscape.js + dagre 布局**，自动按边方向排层级。同层的多个分组横向并排，布局引擎自动处理，不需要手动指定层级。

### 7.7 节点详情面板

点击节点后右侧面板展示：
- 主标题：主机名（已绑定）或节点名（未绑定）
- 副标题：节点名（已绑定时）
- IP、角色、主机跳转链接
- 上游节点列表、下游节点列表
- 操作：绑定主机 / 解除绑定

---

## 8. 不在本期范围内

- 节点拖拽保存坐标（布局引擎自动排，不持久化位置）
- 实时连通性状态（ping 状态不在拓扑图上显示）
- 拓扑版本历史
- 多拓扑间的节点共享（同一 Host 可出现在多个拓扑，但节点记录独立）
