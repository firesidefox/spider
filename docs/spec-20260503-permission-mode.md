# Permission Mode Design — spider.ai Agent 执行权限

**Date:** 2026-05-03  
**Status:** Draft

---

## 1. 背景

spider.ai 的智能运维 Agent 通过 MCP 工具执行命令。当前 MCP 层无任何权限检查，所有工具无限制执行。需要引入类似 Claude Code 的权限模式，根据命令风险程度分级管控。

---

## 2. 权限模式

支持四种模式：

| 模式 | 行为 | 适用场景 |
|------|------|---------|
| `ask`（默认） | L3 及以上暂停等人工批准 | 日常运维 |
| `auto` | 按风险自动执行，L4 仍需批准 | CI/CD、自动化流水线 |
| `plan` | 只生成执行计划，不实际执行 | 变更评审、演练 |
| `readonly` | 只允许 L1 读操作，其余全拒绝 | 审计巡检 |

---

## 3. 风险分级

### 3.1 级别定义

| 级别 | 名称 | 描述 | 示例 |
|------|------|------|------|
| L1 | 读 | 只读、无副作用 | `ls`, `cat`, `ps`, `df`, `ping` |
| L2 | 写 | 可逆写操作 | 写文件、重启服务、修改配置 |
| L3 | 危险 | 难以逆转 | `rm`, 停服、清空日志、kill 进程 |
| L4 | 毁灭 | 不可逆、影响范围大 | 批量删除、格式化磁盘、批量停服 |

### 3.2 判定策略（混合）

1. **静态规则优先**：维护命令前缀/模式黑白名单，快速匹配常见命令
2. **LLM 补充判断**：静态规则未命中时，由 Agent 自评风险级别并说明理由
3. **保守原则**：无法判定时默认 L3

### 3.3 静态规则示例

```yaml
rules:
  L1:
    - "^ls", "^cat", "^ps", "^df", "^du", "^ping", "^curl.*-X GET"
    - "^grep", "^find", "^tail -f", "^journalctl"
  L2:
    - "^echo.*>", "^tee", "^cp", "^mv", "^chmod", "^chown"
    - "^systemctl restart", "^service .* restart"
  L3:
    - "^rm ", "^rmdir", "^systemctl stop", "^kill", "^pkill"
    - "^truncate", "^> " # 清空文件
  L4:
    - "^rm -rf", "^dd ", "^mkfs", "^fdisk"
    - batch operations on 3+ hosts simultaneously
```

---

## 4. 模式 × 级别矩阵

| 级别 | `readonly` | `ask`（默认） | `auto` | `plan` |
|------|-----------|--------------|--------|--------|
| L1 读 | ✅ 执行 | ✅ 执行 | ✅ 执行 | 📋 计划 |
| L2 写 | ❌ 拒绝 | ✅ 执行+审计 | ✅ 执行+审计 | 📋 计划 |
| L3 危险 | ❌ 拒绝 | ⏸️ 等批准 | ✅ 执行+审计 | 📋 计划 |
| L4 毁灭 | ❌ 拒绝 | ⏸️ 等批准 | ⏸️ 等批准 | 📋 计划 |

> `auto` 模式下 L4 仍需人工批准，无"完全无人值守"的毁灭级操作。

---

## 5. 配置层级

两层配置，会话可覆盖全局：

```
全局默认（系统配置）
    └── 会话级覆盖（Agent 启动参数 permission_mode）
```

**全局配置**（`config.yaml`）：
```yaml
agent:
  permission_mode: ask   # 默认模式
```

**会话级覆盖**（创建 Agent 会话时传入）：
```json
{ "permission_mode": "auto" }
```

---

## 6. 批准交互流程

```
Agent 遇到 L3/L4 命令
    → 暂停执行
    → 向 Web UI 推送审批请求（命令内容、风险级别、理由）
    → 用户在聊天界面点击 [批准] / [拒绝]
    → Agent 收到结果继续或终止
    → 审计日志记录操作人 + 决策
```

审批请求数据结构：
```json
{
  "approval_id": "uuid",
  "session_id": "...",
  "command": "rm -rf /tmp/old_logs",
  "host": "prod-server-01",
  "risk_level": "L3",
  "risk_reason": "rm 命令删除文件，操作不可逆",
  "requested_at": "2026-05-03T10:00:00Z"
}
```

---

## 7. 架构变更

### 7.1 新增组件

- `internal/permission/classifier.go` — 风险分级器（静态规则 + LLM fallback）
- `internal/permission/enforcer.go` — 模式执行器，决定放行/暂停/拒绝
- `internal/permission/approval.go` — 审批请求管理（创建、等待、响应）

### 7.2 MCP 工具改造

在 `execute_command` / `execute_command_batch` 工具中注入权限检查：

```
收到工具调用
    → 提取命令 + 主机
    → Classifier 判定风险级别
    → Enforcer 根据当前模式决策
        → 放行：执行，记录审计
        → 暂停：创建审批请求，阻塞等待
        → 拒绝：返回错误，记录审计
```

### 7.3 审批 API

新增 HTTP 端点（需 operator 以上权限）：

```
GET  /api/v1/approvals          # 待审批列表
POST /api/v1/approvals/:id/approve
POST /api/v1/approvals/:id/reject
```

WebSocket/SSE 推送审批请求到前端。

---

## 8. 审计日志扩展

现有 `execution_logs` 表新增字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `risk_level` | string | L1-L4 |
| `permission_mode` | string | 执行时的模式 |
| `approval_id` | string | 关联审批记录（可空） |
| `approved_by` | string | 批准人 UserID（可空） |

---

## 9. 不在范围内

- 命令白名单（允许特定用户跳过风险检查）
- 主机维度的权限（某角色只能操作特定主机）
- 审批委托（A 发起，B 审批）
