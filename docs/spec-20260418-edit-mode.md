# Spec: 编辑模式统一规范

**日期：** 2026-04-18  
**范围：** web 所有页面

---

## 目标

所有页面默认展示只读视图，编辑操作必须通过显式的"编辑"按钮触发，避免用户误操作。

---

## 当前问题（需修改的页面）

| 页面 | 问题 |
|------|------|
| `UsersPanel.vue` | 右侧详情区直接展示角色选择器 + 密码输入框，无需点击编辑 |
| `ProfileView.vue` 基本信息 tab | 直接展示修改密码表单 |
| `ProfileView.vue` 系统设置 tab | 直接展示 MCP/SSH 配置输入框 |

**已符合规范（不需改动）：**
- `HostsView`：编辑按钮 → modal 弹窗 ✅
- `ProfileView` 访问令牌：新建按钮 → modal ✅
- `ProfileView` 操作日志：只读 ✅

---

## 交互规范

### 默认状态（只读）
- 展示字段值，使用 `detail-field` / `detail-value` 样式
- topbar 右侧显示"编辑"按钮
- 不显示任何 `<input>`、`<select>`、`<textarea>`

### 编辑状态（点击"编辑"后）
- topbar 右侧"编辑"按钮替换为"保存"+"取消"
- 字段切换为可编辑的 `<input>` / `<select>`
- 取消：恢复只读状态，丢弃未保存的修改
- 保存：调用 API，成功后回到只读状态

### 模态弹窗（新建操作）
- 新建用户、新建 Token 等保持现有 modal 方式，不变

---

## 各页面具体改动

### 1. UsersPanel — 用户详情右侧

**只读视图（默认）：**
```
detail-topbar:
  左：用户名 + role-badge + 状态
  右：[编辑] [禁用/启用] [删除]

detail-body:
  detail-grid: 用户名、最后登录（只读 detail-field）
```

**编辑视图（点击"编辑"后）：**
```
detail-topbar:
  左：用户名 + role-badge + 状态
  右：[保存] [取消] [禁用/启用] [删除]

detail-body:
  detail-grid: 用户名（只读）、最后登录（只读）
  edit-card: 角色选择器 + 新密码 + 确认密码
```

### 2. ProfileView — 基本信息 tab

**只读视图（默认）：**
```
detail-topbar:
  左：基本信息
  右：[修改密码]（按钮触发 modal）

detail-body:
  detail-grid: 用户名、角色、注册时间、上次登录（只读）
```

**修改密码：** 点击"修改密码"按钮 → modal 弹窗（旧密码 + 新密码 + 确认）

### 3. ProfileView — 系统设置 tab

**只读视图（默认）：**
```
detail-topbar:
  左：系统设置
  右：[编辑]

detail-body:
  detail-grid 展示各配置项的当前值（只读 detail-field）
```

**编辑视图（点击"编辑"后）：**
```
detail-topbar:
  左：系统设置
  右：[保存] [取消]

detail-body:
  edit-card: MCP Server 配置输入框
  edit-card: SSH 配置输入框
```

---

## 验收标准

- [ ] 所有页面默认不显示编辑表单
- [ ] 编辑按钮点击后切换到编辑态
- [ ] 取消按钮恢复只读态，数据不变
- [ ] 保存成功后回到只读态，显示最新数据
- [ ] 保存失败显示错误信息，保持编辑态
- [ ] 禁用/启用/删除按钮在只读态和编辑态均可用

---

## 不在范围内

- HostsView（已符合规范）
- 访问令牌、操作日志（只读或 modal 新建，无需改动）
- LoginView、AuditView、ExecView（无内联编辑）
