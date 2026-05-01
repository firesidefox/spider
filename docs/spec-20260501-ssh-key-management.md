# SSH Key 管理

## 概述

在个人设置中管理 SSH 私钥，主机管理中通过下拉列表引用已有 key，同时保留原有的内联私钥粘贴方式。

## 数据模型

### 新增 ssh_keys 表

```sql
CREATE TABLE ssh_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    encrypted_private_key TEXT NOT NULL,
    encrypted_passphrase TEXT,
    fingerprint TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name)
);
CREATE INDEX idx_ssh_keys_user_id ON ssh_keys(user_id);
```

字段说明：
- `id`：格式 `k_<nanoid>`
- `user_id`：归属用户，每个用户只能看到和使用自己的 key
- `name`：用户自定义名称，同一用户下唯一
- `encrypted_private_key`：AES-256-GCM 加密，复用现有 crypto 模块
- `encrypted_passphrase`：可选，加密后的 passphrase
- `fingerprint`：从私钥解析的 SHA256 指纹，用于展示辨识

### hosts 表变更

```sql
ALTER TABLE hosts ADD COLUMN ssh_key_id TEXT;
```

- `encrypted_credential` 保留不废弃，已有内联私钥主机继续正常工作
- `ssh_key_id` 和 `encrypted_credential` 并存，SSH 连接时优先用 `ssh_key_id`，fallback 到 `encrypted_credential`
- 无需数据迁移

## API

### SSH Keys CRUD

所有接口操作当前登录用户自己的 key，无需额外权限。

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/me/ssh-keys` | 列出所有 key（不返回私钥内容） |
| `POST` | `/api/v1/me/ssh-keys` | 上传新 key |
| `GET` | `/api/v1/me/ssh-keys/:id` | 获取单个 key 详情 |
| `DELETE` | `/api/v1/me/ssh-keys/:id` | 删除 key（被引用时 409） |

**POST 请求体：**

```json
{
  "name": "prod-key",
  "private_key": "-----BEGIN OPENSSH PRIVATE KEY-----\n...",
  "passphrase": "optional"
}
```

**返回体 SafeSSHKey：**

```json
{
  "id": "k_abc123",
  "name": "prod-key",
  "fingerprint": "SHA256:...",
  "created_at": "2026-05-01T...",
  "updated_at": "2026-05-01T..."
}
```

私钥内容只在创建时接收，之后永远不返回给前端。

DELETE 删除时，后端检查是否有主机引用该 key。如有引用，返回 409 并附带引用主机列表。

### Host 端变更

- `POST /api/v1/hosts` 和 `PUT /api/v1/hosts/:id`：新增可选字段 `ssh_key_id`
- `ssh_key_id` 和 `credential` 互斥：同时传则报错
- 后端校验 `ssh_key_id` 存在且属于当前用户
- `GET` 返回的 SafeHost 新增 `ssh_key_id` 和 `ssh_key_name` 字段

### MCP 工具变更

- 新增工具：`list_ssh_keys`、`add_ssh_key`、`remove_ssh_key`
- `add_host` / `update_host`：新增 `ssh_key_id` 可选参数，`credential` 保留
- `ssh_key_id` 和 `credential` 互斥，同时传则报错

## 前端

### ProfileView 新增 SSH Keys tab

位置在 Tokens tab 旁边，同属 Personal 分组。

**Key 列表：**
- 表格展示：名称、指纹（截断显示）、创建时间、引用主机数
- 右上角"添加 Key"按钮
- 每行操作：删除（有引用时 disable 并 tooltip 提示）

**添加 Key 表单：**
- name：文本输入，必填
- private_key：textarea 支持粘贴 PEM，也支持文件选择器读取
- passphrase：可选密码输入框
- 提交后展示指纹确认

### HostsView 变更

添加/编辑主机时，auth_type 为 key 或 key_password：

- 上方新增下拉选择器，列出当前用户的 SSH keys（name + fingerprint 后几位）
- 下方保留原来的 credential textarea

两者互斥交互：
- 选了下拉中的 key → textarea 置灰清空
- 在 textarea 中输入内容 → 下拉重置为空

提交时：
- 选了已有 key → 发送 `ssh_key_id`
- 粘贴了私钥内容 → 走原有逻辑，私钥加密后存 `encrypted_credential`

## SSH 连接逻辑

连接主机时的凭据解析优先级：

1. `ssh_key_id` 非空 → 从 `ssh_keys` 表解密私钥（和 passphrase）
2. `encrypted_credential` 非空 → 走原有解密逻辑
3. 都为空 → 报错

## 安全考虑

- 私钥使用 AES-256-GCM 加密存储，复用现有 crypto 模块和 master.key
- 私钥内容只在 POST 创建时接收，GET 接口永远不返回
- key 归属用户隔离，后端强制校验 user_id
- 主机绑定 key 时校验 key 属于当前操作用户
