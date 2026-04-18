# Spec: Phase 2 — 用户管理与认证

**日期：** 2026-04-18  
**状态：** 草稿  
**范围：** 多用户认证、RBAC、API Token、用户管理 Web UI

---

## 1. 目标

让 spider 支持多用户团队共用，实现：
- 登录认证（Web UI 用 JWT，MCP 用 API Token）
- 三级 RBAC（admin / operator / viewer）
- Admin 可创建、禁用、删除用户
- 用户可生成/撤销 API Token 供 MCP 使用
- 所有执行记录关联操作人

**不在本 spec 范围：**
- 主机组权限（PRD 标注"可选"，推迟）
- Token scopes 细粒度控制（PRD P1，推迟）
- 审计日志导出 CSV（PRD P2，推迟）

---

## 2. 用户旅程

### 2.1 首次启动（无用户时）
1. spider 启动，检测 users 表为空
2. 自动创建默认 admin 账号（用户名 `admin`，密码随机生成，打印到 stdout 一次）
3. 后续所有 API 请求需认证

### 2.2 Web UI 登录
1. 访问任意页面 → 未登录 → 跳转 `/login`
2. 输入用户名/密码 → POST `/api/v1/auth/login` → 返回 JWT
3. JWT 存 localStorage，后续请求带 `Authorization: Bearer <jwt>`
4. JWT 过期（24h）→ 自动跳转登录页

### 2.3 Admin 管理用户
1. 导航到"用户管理"页（仅 admin 可见）
2. 查看用户列表（用户名、角色、状态、最后登录）
3. 创建用户：填写用户名、初始密码、角色
4. 禁用/启用用户（禁用后立即失效）
5. 删除用户（不能删除自己）

### 2.4 用户管理 API Token
1. 导航到"API Token"页
2. 创建 Token：填写名称，可选过期时间
3. 创建成功后展示明文一次（之后不可再查）
4. 撤销 Token

### 2.5 MCP 认证
1. 用户在 Claude Code 配置中带 Bearer Token
2. spider MCP 端点验证 Token hash，识别用户身份
3. 执行记录写入 user_id

---

## 3. 数据模型

### 3.1 新增表（迁移追加，不修改现有表）

```sql
CREATE TABLE IF NOT EXISTS users (
    id           TEXT PRIMARY KEY,
    username     TEXT UNIQUE NOT NULL,
    password     TEXT NOT NULL,        -- bcrypt hash, cost=12
    role         TEXT NOT NULL,        -- admin | operator | viewer
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL,
    last_login   DATETIME
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,  -- SHA-256(token), hex
    expires_at  DATETIME,              -- NULL = 永不过期
    created_at  DATETIME NOT NULL,
    last_used   DATETIME
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_api_tokens_token_hash ON api_tokens(token_hash);
```

### 3.2 修改现有表

```sql
-- execution_logs 追加 user_id 列（可空，兼容历史数据）
ALTER TABLE execution_logs ADD COLUMN user_id TEXT;
```

---

## 4. API 规格

### 4.1 认证端点（无需鉴权）

```
POST /api/v1/auth/login
  Body: { "username": string, "password": string }
  200:  { "token": string, "expires_at": string, "user": UserInfo }
  401:  { "error": "invalid credentials" }
  403:  { "error": "account disabled" }

POST /api/v1/auth/logout
  需要 JWT。使 token 失效（服务端维护黑名单，TTL=剩余有效期）。
  200:  { "ok": true }
```

### 4.2 用户管理（需 admin 角色）

```
GET  /api/v1/users
  200: [ UserInfo ]

POST /api/v1/users
  Body: { "username": string, "password": string, "role": string }
  201: UserInfo
  409: { "error": "username already exists" }

PUT  /api/v1/users/:id
  Body: { "role"?, "enabled"?, "password"? }
  200: UserInfo
  403: 不能修改自己的 role/enabled

DELETE /api/v1/users/:id
  204
  403: 不能删除自己
```

### 4.3 API Token（需登录，操作自己的 token）

```
GET  /api/v1/tokens
  200: [ TokenInfo ]  -- 不含 token 明文

POST /api/v1/tokens
  Body: { "name": string, "expires_at"?: string }
  201: { ...TokenInfo, "token": string }  -- 明文仅此一次

DELETE /api/v1/tokens/:id
  204
  403: 只能删除自己的 token
```

### 4.4 当前用户

```
GET /api/v1/me
  200: UserInfo
```

### 4.5 响应类型

```typescript
interface UserInfo {
  id: string
  username: string
  role: "admin" | "operator" | "viewer"
  enabled: boolean
  created_at: string
  last_login: string | null
}

interface TokenInfo {
  id: string
  name: string
  expires_at: string | null
  created_at: string
  last_used: string | null
}
```

---

## 5. 认证中间件

### 5.1 JWT（Web UI）

- 算法：HS256
- 有效期：24h
- Payload：`{ sub: user_id, role: string, exp: unix }`
- 密钥：启动时从 `~/.spider/jwt.key` 读取，不存在则随机生成并保存
- 黑名单：logout 时将 jti 写入内存 map（重启后清空，可接受）

### 5.2 API Token（MCP）

- 格式：`spd_` + 32字节随机 hex（共 68 字符）
- 存储：SHA-256(token) hex 存 DB
- 验证：计算请求 token 的 SHA-256，查 DB，检查 enabled/expires_at
- 更新 last_used（异步，不阻塞请求）

### 5.3 中间件优先级

```
请求 → 检查 Authorization header
  → "Bearer spd_..." → API Token 认证路径
  → "Bearer eyJ..." → JWT 认证路径
  → 无 header → 401
```

### 5.4 RBAC 矩阵

| 操作 | admin | operator | viewer |
|------|-------|----------|--------|
| 用户管理 | ✅ | ❌ | ❌ |
| 主机增删改 | ✅ | ✅ | ❌ |
| 主机查看 | ✅ | ✅ | ✅ |
| 执行命令 | ✅ | ✅ | ❌ |
| 查看执行历史 | ✅ | ✅ | ✅ |
| 系统设置 | ✅ | ❌ | ❌ |
| API Token 管理（自己） | ✅ | ✅ | ✅ |

---

## 6. 后端实现结构

```
internal/
  models/
    user.go          -- User, ApiToken 结构体
  store/
    user_store.go    -- CRUD + 认证查询
    token_store.go   -- Token CRUD + hash 查询
  auth/
    jwt.go           -- 生成/验证 JWT
    token.go         -- 生成/验证 API Token
    middleware.go    -- HTTP 中间件，注入 UserContext
  api/
    auth.go          -- login/logout handler
    users.go         -- 用户管理 handler
    tokens.go        -- Token 管理 handler
  db/
    schema.go        -- 追加 users/api_tokens 表 + migration
```

**无需新增 Service 层**：用户管理逻辑简单，handler 直接调用 store。

---

## 7. 前端实现结构

```
web/src/
  views/
    LoginView.vue      -- 登录页（无导航栏）
    UsersView.vue      -- 用户管理（admin only）
    TokensView.vue     -- API Token 管理
  api/
    auth.ts            -- login/logout/me
    users.ts           -- 用户 CRUD
    tokens.ts          -- Token CRUD
  composables/
    useAuth.ts         -- 当前用户状态、登录态检查
  router/
    index.ts           -- 路由守卫：未登录跳 /login，非 admin 跳 /hosts
```

**导航栏变更：**
- 添加"用户"链接（admin only）
- 添加"Token"链接（所有登录用户）
- 右上角显示当前用户名 + 登出按钮

---

## 8. 兼容性与迁移

- **单用户模式**：若 `auth.enabled: false`（config），跳过所有认证中间件，保持现有行为
- **默认关闭**：Phase 2 认证默认 **不启用**，需在 config 中显式 `auth.enabled: true`
- **数据库迁移**：schema.go 追加新表，ALTER TABLE 追加列，幂等执行

---

## 9. 验收标准

- [ ] 未登录访问 `/api/v1/hosts` 返回 401
- [ ] 正确密码登录返回 JWT，错误密码返回 401
- [ ] 禁用账号登录返回 403
- [ ] operator 调用 `DELETE /api/v1/users/:id` 返回 403
- [ ] viewer 调用 `POST /api/v1/exec` 返回 403
- [ ] API Token 认证成功后执行记录含 user_id
- [ ] 撤销 Token 后立即失效（下一请求 401）
- [ ] JWT 过期后 Web UI 跳转登录页
- [ ] Admin 可创建/禁用/删除用户（不能删自己）
- [ ] Token 明文仅在创建响应中出现一次
- [ ] `auth.enabled: false` 时所有现有功能不受影响
