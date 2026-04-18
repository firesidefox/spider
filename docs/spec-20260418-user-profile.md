---
title: Spec — 用户设置页重构
date: 2026-04-18
status: 草稿
---

## 1. 目标

将分散在导航栏的「改密码」按钮、独立的 `/tokens` 页面，整合为统一的「用户设置」页面，
入口为右上角用户名下拉菜单。

---

## 2. 用户旅程

```
点击右上角用户名
  └─ 下拉菜单
       ├─ 用户设置  →  /profile（新页面，含 Tab）
       └─ 登出
```

---

## 3. 功能范围

### 3.1 入口变更（App.vue）

- 用户名从纯文本改为可点击区域，点击展开下拉菜单
- 下拉菜单项：「用户设置」（跳转 /profile）、「登出」
- 移除导航栏「改密码」按钮
- 移除导航栏「Token」链接

### 3.2 用户设置页（/profile）

路由：`/profile`，组件：`ProfileView.vue`，三个 Tab：

#### Tab 1：基本信息

| 字段 | 说明 |
|------|------|
| 用户名 | 只读展示 |
| 角色 | 只读展示（admin / operator / viewer） |
| 修改密码 | 旧密码 + 新密码 + 确认新密码，提交调用 `PUT /api/v1/me/password` |

#### Tab 2：访问令牌

迁移现有 `TokensView.vue` 内容：
- 列表：名称、创建时间、过期时间、最后使用、撤销按钮
- 新建 Token 弹窗（名称 + 可选过期时间）
- 明文展示弹窗（仅一次，复制后关闭）

#### Tab 3：日志

展示当前登录用户触发的审计记录（`triggered_by = 当前用户名`）：
- 复用现有 `/api/v1/logs` 接口，前端过滤或后端新增 `?user=me` 参数
- 字段：主机、命令、退出码、耗时、时间
- 点击行展开命令输出（复用 AuditView 的详情逻辑）

---

## 4. 后端变更

- `GET /api/v1/logs?user=me`：新增 `user` 查询参数，`me` 表示当前认证用户
- 其余接口无变更

---

## 5. 前端变更清单

| 文件 | 变更 |
|------|------|
| `web/src/App.vue` | 用户名改为下拉菜单；移除「改密码」modal 和「Token」nav 链接 |
| `web/src/main.ts` | 新增 `/profile` 路由；移除 `/tokens` 路由 |
| `web/src/views/ProfileView.vue` | 新建，含三 Tab |
| `web/src/views/TokensView.vue` | 删除（内容迁移到 ProfileView） |
| `internal/api/logs.go` | 支持 `?user=me` 过滤 |

---

## 6. 验收标准

- [ ] 点击用户名弹出下拉菜单，含「用户设置」和「登出」
- [ ] 导航栏不再显示「Token」链接和「改密码」按钮
- [ ] /profile 默认打开「基本信息」Tab
- [ ] 修改密码：旧密码错误显示明确错误；两次新密码不一致前端拦截
- [ ] 访问令牌 Tab 功能与原 /tokens 页面一致
- [ ] 日志 Tab 只显示当前用户触发的记录
- [ ] `go vet` 零错误，`npm run build` 零错误
- [ ] auth.enabled=false 匿名用户可正常访问 /profile
