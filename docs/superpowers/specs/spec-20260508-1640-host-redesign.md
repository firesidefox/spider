# Spec: 主机模型重构

**日期：** 2026-05-08  
**状态：** 已实现（AccessFace、Fingerprint、Memory 均已落地）

---

## 1. 背景

现有 `Host` 模型为单一 SSH 访问模式，字段平铺，不支持多操作面、REST API 接入、指纹检测和操作记忆。本次重构将主机拆分为四个概念：基本信息、操作面、指纹、记忆。

---

## 2. 概念定义

### 2.1 基本信息

主机的身份标识和元数据。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | ✓ | UUID |
| name | string | ✓ | 主机名称，唯一 |
| ip | string | ✓ | 管理 IP |
| notes | string | | 备注 |
| tags | []string | | 标签，用于分组过滤 |
| vendor | string | | 厂商（如：华为、H3C、F5） |
| product_name | string | | 产品名称（如：USG6000） |
| product_version | string | | 产品版本（如：V500R005C00） |

`vendor`、`product_name`、`product_version` 为可选字段，用于检索知识库。Linux 等通用设备无需填写。

---

### 2.2 操作面（AccessFace）

操作面是访问和操作主机的通道，包含连接参数、授权方式和知识来源。一台主机可有多个操作面。

支持两种类型：**SSH** 和 **REST API**。

#### SSH 操作面

| 字段 | 类型 | 说明 |
|------|------|------|
| type | "ssh" | |
| ip | string | 连接 IP（可与主机 IP 不同） |
| port | int | 默认 22 |
| username | string | |
| auth_type | enum | password \| key \| key_password |
| credential | string | 加密存储（密码或私钥内容） |
| passphrase | string | 加密存储，key_password 时使用 |
| ssh_key_id | string | 引用已保存的 SSH Key |
| ssh_legacy | bool | 兼容旧算法设备 |

#### REST API 操作面

| 字段 | 类型 | 说明 |
|------|------|------|
| type | "restapi" | |
| ip | string | |
| port | int | |
| base_url | string | 如 https://10.0.0.1/api/v1 |
| auth_type | enum | bearer \| basic \| apikey \| none |
| credential | string | 加密存储（token / 密码 / api key） |
| username | string | basic auth 时使用 |
| header_name | string | apikey 模式自定义 header 名 |

#### 知识来源（两种操作面共有）

每个操作面可绑定若干知识来源，用于 Explore 阶段 AI 查询设备操作方法。

```json
"knowledge_sources": [
  { "type": "group", "id": 3 },
  { "type": "doc",   "id": 17 }
]
```

**设计原则：**
- 知识来源绑定在操作面级别（SSH 绑 CLI 手册，REST API 绑 API 文档）
- 主机级别也可绑定通用背景知识（跨操作面共享）
- Linux 等通用设备无需绑定知识来源

---

### 2.3 指纹（Fingerprint）

每次连接时采集，与上次快照对比，检测主机是否发生变化。

#### 采集项

| 字段 | 来源 | 说明 |
|------|------|------|
| ssh_host_key | SSH 握手 | Host key 变化 = 机器被替换或重装 |
| system_version | SSH 命令采集 | 固件/OS 版本，影响命令兼容性 |
| hardware_id | SSH 命令采集 | 序列号/MAC，变化 = 硬件替换 |
| api_signature | REST API 响应 | 版本字段或响应结构 hash |

#### 状态

| 状态 | 含义 |
|------|------|
| ok | 与上次快照一致 |
| changed | 检测到变化，需人工确认 |
| unverified | 首次采集或指纹变化后 memory 待验证 |

#### 与记忆的联动

- 指纹未变 → memory 可直接复用
- 指纹变化 → 告警用户 + 标记 memory 为 `unverified`，Explore 阶段提示 AI 重新探索

---

### 2.4 记忆（Memory）

记录该主机上的操作经验，供 Explore 阶段 AI 参考。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | |
| host_id | string | 关联主机 |
| content | string | 经验内容（自由文本） |
| created_by | enum | user \| agent |
| created_at | time | |

**写入规则：**
- 由用户或 Agent 通过显式指令触发写入，不自动写入
- 用户可手动编辑或删除

**复用规则：**
- Explore 阶段加载该主机所有 memory
- 若指纹状态为 `changed` 或 `unverified`，提示 AI memory 可能已过期

---

## 3. 数据库 Schema 变更

### 新增表

```sql
-- 操作面
CREATE TABLE access_faces (
  id           TEXT PRIMARY KEY,
  host_id      TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  type         TEXT NOT NULL CHECK(type IN ('ssh','restapi')),
  ip           TEXT NOT NULL,
  port         INTEGER NOT NULL,
  -- SSH
  username              TEXT,
  auth_type             TEXT,
  encrypted_credential  TEXT,
  encrypted_passphrase  TEXT,
  ssh_key_id            TEXT,
  ssh_legacy            INTEGER DEFAULT 0,
  -- REST API
  base_url     TEXT,
  header_name  TEXT,
  -- 知识来源 (JSON)
  knowledge_sources TEXT DEFAULT '[]',
  created_at   DATETIME NOT NULL,
  updated_at   DATETIME NOT NULL
);

-- 主机级知识来源
CREATE TABLE host_knowledge_sources (
  host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  type    TEXT NOT NULL CHECK(type IN ('group','doc')),
  ref_id  INTEGER NOT NULL,
  PRIMARY KEY (host_id, type, ref_id)
);

-- 指纹
CREATE TABLE host_fingerprints (
  host_id          TEXT PRIMARY KEY REFERENCES hosts(id) ON DELETE CASCADE,
  ssh_host_key     TEXT,
  system_version   TEXT,
  hardware_id      TEXT,
  api_signature    TEXT,
  status           TEXT NOT NULL DEFAULT 'unverified',
  snapshot_at      DATETIME
);

-- 记忆
CREATE TABLE host_memories (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  host_id    TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  content    TEXT NOT NULL,
  created_by TEXT NOT NULL CHECK(created_by IN ('user','agent')),
  created_at DATETIME NOT NULL
);
```

### hosts 表变更

删除字段：`cli_type`、`device_type`、`vendor`、`model`、`firmware_version`、`ssh_legacy`、`username`、`auth_type`、`encrypted_credential`、`encrypted_passphrase`、`ssh_key_id`

新增字段：`vendor`、`product_name`、`product_version`、`notes`

> SSH 连接参数迁移至 `access_faces` 表。

---

## 4. Go 模型变更

### 删除字段（Host）

`Username`、`AuthType`、`EncryptedCredential`、`EncryptedPassphrase`、`SSHKeyID`、`SSHLegacy`、`DeviceType`、`CLIType`、`FirmwareVersion`、`Vendor`、`Model`

### 新增结构

```go
type AccessFaceType string
const (
    FaceSSH     AccessFaceType = "ssh"
    FaceRESTAPI AccessFaceType = "restapi"
)

type SSHAuthType string
const (
    SSHAuthPassword    SSHAuthType = "password"
    SSHAuthKey         SSHAuthType = "key"
    SSHAuthKeyPassword SSHAuthType = "key_password"
)

type RESTAuthType string
const (
    RESTAuthBearer RESTAuthType = "bearer"
    RESTAuthBasic  RESTAuthType = "basic"
    RESTAuthAPIKey RESTAuthType = "apikey"
    RESTAuthNone   RESTAuthType = "none"
)

type KnowledgeSourceRef struct {
    Type  string `json:"type"` // "group" | "doc"
    ID    int    `json:"id"`
}

type AccessFace struct {
    ID                  string               `json:"id"`
    HostID              string               `json:"host_id"`
    Type                AccessFaceType       `json:"type"`
    IP                  string               `json:"ip"`
    Port                int                  `json:"port"`
    // SSH
    Username            string               `json:"username,omitempty"`
    SSHAuthType         SSHAuthType          `json:"ssh_auth_type,omitempty"`
    SSHKeyID            string               `json:"ssh_key_id,omitempty"`
    SSHLegacy           bool                 `json:"ssh_legacy,omitempty"`
    // REST API
    BaseURL             string               `json:"base_url,omitempty"`
    RESTAuthType        RESTAuthType         `json:"rest_auth_type,omitempty"`
    RESTUsername        string               `json:"rest_username,omitempty"`
    HeaderName          string               `json:"header_name,omitempty"`
    // 共有
    KnowledgeSources    []KnowledgeSourceRef `json:"knowledge_sources"`
    CreatedAt           time.Time            `json:"created_at"`
    UpdatedAt           time.Time            `json:"updated_at"`
}

type FingerprintStatus string
const (
    FingerprintOK          FingerprintStatus = "ok"
    FingerprintChanged     FingerprintStatus = "changed"
    FingerprintUnverified  FingerprintStatus = "unverified"
)

type Fingerprint struct {
    HostID        string            `json:"host_id"`
    SSHHostKey    string            `json:"ssh_host_key,omitempty"`
    SystemVersion string            `json:"system_version,omitempty"`
    HardwareID    string            `json:"hardware_id,omitempty"`
    APISignature  string            `json:"api_signature,omitempty"`
    Status        FingerprintStatus `json:"status"`
    SnapshotAt    *time.Time        `json:"snapshot_at,omitempty"`
}

type Memory struct {
    ID        int       `json:"id"`
    HostID    string    `json:"host_id"`
    Content   string    `json:"content"`
    CreatedBy string    `json:"created_by"` // "user" | "agent"
    CreatedAt time.Time `json:"created_at"`
}
```

### Host 结构更新

```go
type Host struct {
    ID             string     `json:"id"`
    Name           string     `json:"name"`
    IP             string     `json:"ip"`
    Notes          string     `json:"notes,omitempty"`
    Tags           []string   `json:"tags"`
    Vendor         string     `json:"vendor,omitempty"`
    ProductName    string     `json:"product_name,omitempty"`
    ProductVersion string     `json:"product_version,omitempty"`
    CreatedAt      time.Time  `json:"created_at"`
    UpdatedAt      time.Time  `json:"updated_at"`
    // 关联数据（按需加载）
    KnowledgeSources []KnowledgeSourceRef `json:"knowledge_sources,omitempty"` // 主机级通用知识
    AccessFaces      []AccessFace         `json:"access_faces,omitempty"`
    Fingerprint      *Fingerprint         `json:"fingerprint,omitempty"`
    Memories         []Memory             `json:"memories,omitempty"`
}
```

---

## 5. UI 设计

主机详情页采用左侧列表 + 右侧详情布局，详情区分四个 Tab：概览、操作面、指纹、记忆。

Mockup 文件：`docs/mockups/host-detail-v1.html`

---

## 6. 迁移策略

1. 现有 hosts 表数据迁移：为每台主机自动创建一个 SSH 操作面，字段从 hosts 表复制
2. 迁移完成后删除 hosts 表中的 SSH 相关字段
3. 前端同步更新主机表单和详情页

---

## 7. 范围外

- 指纹采集的具体命令（由操作面知识库决定，不在本 spec 内）
- 记忆的 AI 自动提炼（后续迭代）
- 操作面连通性测试 UI
