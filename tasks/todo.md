# Phase 2 任务清单

## Phase A：后端基础设施

- [ ] Task 1：DB 迁移 + 数据模型（users/api_tokens 表，User/ApiToken 结构体）
- [ ] Task 2：Config 新增 Auth 配置（auth.enabled 开关）
- [ ] Task 3：JWT + API Token 生成/验证（auth/jwt.go, auth/token.go）
- [ ] Task 4：UserStore + TokenStore（CRUD + 首次启动自动创建 admin）
- [ ] Task 5：认证中间件（Bearer 解析，RBAC helper，auth.enabled 开关）

### Checkpoint A
- [ ] `go build ./...` 无错误
- [ ] `go test ./internal/...` 全部通过

## Phase B：后端 API

- [ ] Task 6：认证端点 + App 扩展（login/logout/me，路由包裹中间件）
- [ ] Task 7：用户管理 API（/api/v1/users CRUD，admin only）
- [ ] Task 8：API Token 端点（/api/v1/tokens，MCP 写 user_id）

### Checkpoint B
- [ ] `go build ./...` 无错误
- [ ] 手动验证所有 RBAC 场景
- [ ] auth.enabled=false 回归通过

## Phase C：前端

- [ ] Task 9：useAuth composable + 路由守卫
- [ ] Task 10：LoginView + 导航栏更新
- [ ] Task 11：UsersView（admin 用户管理页）
- [ ] Task 12：TokensView（API Token 管理页）

### Checkpoint C（完成）
- [ ] spec §9 全部验收标准通过
- [ ] 前端 `npm run build` 无错误
- [ ] auth.enabled=false 回归测试通过
