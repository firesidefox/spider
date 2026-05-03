# Permission Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 spider.ai Agent 的 MCP 工具执行引入风险分级权限模式（L1-L4），支持 auto/ask/plan/readonly 四种模式，ask 为默认，高风险命令暂停等待 Web UI 人工批准。

**Architecture:** 新增 `internal/permission` 包，包含风险分级器（静态规则 + LLM fallback）、模式执行器、审批请求管理。MCP 工具 `execute_command` / `execute_command_batch` 注入权限检查。新增审批 HTTP API，通过 SSE 推送审批请求到前端。

**Tech Stack:** Go 1.23, standard `testing`, SQLite (modernc.org/sqlite), `net/http` ServeMux, `github.com/mark3labs/mcp-go`

---

## File Map

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/permission/types.go` | 新建 | RiskLevel、PermissionMode、Decision 类型定义 |
| `internal/permission/classifier.go` | 新建 | 静态规则 + LLM fallback 风险分级器 |
| `internal/permission/classifier_test.go` | 新建 | 分级器单元测试 |
| `internal/permission/enforcer.go` | 新建 | 模式执行器，根据模式+级别返回决策 |
| `internal/permission/enforcer_test.go` | 新建 | 执行器单元测试 |
| `internal/permission/approval.go` | 新建 | 审批请求内存管理（创建/等待/响应） |
| `internal/permission/approval_test.go` | 新建 | 审批管理单元测试 |
| `internal/store/approval_store.go` | 新建 | 审批记录持久化到 SQLite |
| `internal/db/schema.go` | 修改 | 新增 approvals 表、execution_logs 新增字段 |
| `internal/config/config.go` | 修改 | Config 新增 Agent.PermissionMode 字段 |
| `internal/mcp/server.go` | 修改 | App 新增 PermissionManager 字段 |
| `internal/mcp/tools.go` | 修改 | execute_command / execute_command_batch 注入权限检查 |
| `internal/api/handler.go` | 修改 | 注册审批 API 路由 |
| `internal/api/approval.go` | 新建 | 审批 HTTP 处理器（list/approve/reject/stream） |
| `cmd/spider/main.go` | 修改 | 初始化 PermissionManager 并注入 App |

---

## Task 1: 类型定义

**Files:**
- Create: `internal/permission/types.go`

- [ ] **Step 1: 创建类型文件**

```go
package permission

// RiskLevel 命令风险级别
type RiskLevel int

const (
    L1Read      RiskLevel = 1 // 只读，无副作用
    L2Write     RiskLevel = 2 // 可逆写操作
    L3Dangerous RiskLevel = 3 // 难以逆转
    L4Destroy   RiskLevel = 4 // 不可逆，影响范围大
)

func (l RiskLevel) String() string {
    switch l {
    case L1Read:
        return "L1"
    case L2Write:
        return "L2"
    case L3Dangerous:
        return "L3"
    case L4Destroy:
        return "L4"
    default:
        return "unknown"
    }
}

// Mode 权限模式
type Mode string

const (
    ModeAsk      Mode = "ask"      // 默认：L3+ 等批准
    ModeAuto     Mode = "auto"     // 自动：L4 等批准
    ModePlan     Mode = "plan"     // 只生成计划，不执行
    ModeReadonly Mode = "readonly" // 只允许 L1
)

// Decision 执行决策
type Decision int

const (
    DecisionAllow   Decision = iota // 直接执行
    DecisionPending                 // 暂停等批准
    DecisionDeny                    // 拒绝执行
    DecisionPlan                    // 返回计划，不执行
)

// Classification 分级结果
type Classification struct {
    Level  RiskLevel
    Reason string // 判定理由
    Source string // "static" or "llm"
}
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./internal/permission/...
```

Expected: 无错误

- [ ] **Step 3: Commit**

```bash
git add internal/permission/types.go
git commit -m "feat(permission): add RiskLevel, Mode, Decision types"
```

---

## Task 2: 风险分级器（静态规则）

**Files:**
- Create: `internal/permission/classifier.go`
- Create: `internal/permission/classifier_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/permission/classifier_test.go
package permission_test

import (
    "testing"
    "github.com/spiderai/spider/internal/permission"
)

func TestClassifier_StaticRules(t *testing.T) {
    c := permission.NewClassifier(nil) // nil = no LLM fallback

    tests := []struct {
        cmd      string
        wantLevel permission.RiskLevel
    }{
        {"ls -la /tmp", permission.L1Read},
        {"cat /etc/hosts", permission.L1Read},
        {"ps aux", permission.L1Read},
        {"df -h", permission.L1Read},
        {"grep -r foo /var/log", permission.L1Read},
        {"echo hello > /tmp/test.txt", permission.L2Write},
        {"cp /tmp/a /tmp/b", permission.L2Write},
        {"systemctl restart nginx", permission.L2Write},
        {"rm /tmp/test.txt", permission.L3Dangerous},
        {"systemctl stop nginx", permission.L3Dangerous},
        {"kill 1234", permission.L3Dangerous},
        {"rm -rf /tmp/old", permission.L4Destroy},
        {"dd if=/dev/zero of=/dev/sda", permission.L4Destroy},
        {"unknown-custom-tool --flag", permission.L3Dangerous}, // 保守原则
    }

    for _, tt := range tests {
        t.Run(tt.cmd, func(t *testing.T) {
            got := c.Classify(t.Context(), tt.cmd)
            if got.Level != tt.wantLevel {
                t.Errorf("Classify(%q) = %v, want %v (reason: %s)", tt.cmd, got.Level, tt.wantLevel, got.Reason)
            }
        })
    }
}
```

- [ ] **Step 2: 运行确认失败**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/permission/... 2>&1 | head -20
```

Expected: `cannot find package` 或 `undefined: permission.NewClassifier`

- [ ] **Step 3: 实现分级器**

```go
// internal/permission/classifier.go
package permission

import (
    "context"
    "regexp"
)

// LLMClassifier 可选的 LLM fallback 接口
type LLMClassifier interface {
    Classify(ctx context.Context, command string) Classification
}

type rule struct {
    pattern *regexp.Regexp
    level   RiskLevel
}

// Classifier 风险分级器
type Classifier struct {
    rules []rule
    llm   LLMClassifier
}

// NewClassifier 创建分级器，llm 可为 nil（禁用 LLM fallback）
func NewClassifier(llm LLMClassifier) *Classifier {
    return &Classifier{
        rules: buildStaticRules(),
        llm:   llm,
    }
}

// Classify 判定命令风险级别
func (c *Classifier) Classify(ctx context.Context, command string) Classification {
    // 静态规则优先，从高到低匹配（L4 先检查）
    for _, r := range c.rules {
        if r.pattern.MatchString(command) {
            return Classification{Level: r.level, Source: "static", Reason: "matched static rule: " + r.pattern.String()}
        }
    }
    // LLM fallback
    if c.llm != nil {
        return c.llm.Classify(ctx, command)
    }
    // 保守原则：未知命令默认 L3
    return Classification{Level: L3Dangerous, Source: "default", Reason: "unknown command, defaulting to L3"}
}
```

- [ ] **Step 4: 实现静态规则（续写同文件）**

```go
func buildStaticRules() []rule {
    // 顺序重要：L4 必须在 L3 之前（rm -rf 比 rm 更具体）
    patterns := []struct {
        pattern string
        level   RiskLevel
    }{
        // L4 毁灭
        {`^rm\s+-[a-zA-Z]*r[a-zA-Z]*f`, L4Destroy},  // rm -rf, rm -fr
        {`^rm\s+-[a-zA-Z]*f[a-zA-Z]*r`, L4Destroy},
        {`^dd\s+`, L4Destroy},
        {`^mkfs`, L4Destroy},
        {`^fdisk\s+`, L4Destroy},
        {`^parted\s+`, L4Destroy},
        {`^shred\s+`, L4Destroy},
        // L3 危险
        {`^rm\s+`, L3Dangerous},
        {`^rmdir\s+`, L3Dangerous},
        {`^systemctl\s+stop\s+`, L3Dangerous},
        {`^service\s+\S+\s+stop`, L3Dangerous},
        {`^kill\s+`, L3Dangerous},
        {`^pkill\s+`, L3Dangerous},
        {`^killall\s+`, L3Dangerous},
        {`^truncate\s+`, L3Dangerous},
        {`^>\s+\S+`, L3Dangerous}, // > file (清空)
        {`^unlink\s+`, L3Dangerous},
        // L2 写
        {`^echo\s+.*>`, L2Write},
        {`^tee\s+`, L2Write},
        {`^cp\s+`, L2Write},
        {`^mv\s+`, L2Write},
        {`^chmod\s+`, L2Write},
        {`^chown\s+`, L2Write},
        {`^mkdir\s+`, L2Write},
        {`^touch\s+`, L2Write},
        {`^systemctl\s+restart\s+`, L2Write},
        {`^systemctl\s+start\s+`, L2Write},
        {`^service\s+\S+\s+restart`, L2Write},
        {`^service\s+\S+\s+start`, L2Write},
        {`^apt(-get)?\s+install`, L2Write},
        {`^yum\s+install`, L2Write},
        {`^pip\s+install`, L2Write},
        // L1 读
        {`^ls(\s+|$)`, L1Read},
        {`^cat\s+`, L1Read},
        {`^less\s+`, L1Read},
        {`^more\s+`, L1Read},
        {`^head\s+`, L1Read},
        {`^tail\s+`, L1Read},
        {`^ps(\s+|$)`, L1Read},
        {`^df(\s+|$)`, L1Read},
        {`^du(\s+|$)`, L1Read},
        {`^ping\s+`, L1Read},
        {`^grep\s+`, L1Read},
        {`^find\s+`, L1Read},
        {`^which\s+`, L1Read},
        {`^whoami$`, L1Read},
        {`^hostname$`, L1Read},
        {`^uname(\s+|$)`, L1Read},
        {`^uptime$`, L1Read},
        {`^free(\s+|$)`, L1Read},
        {`^top(\s+|$)`, L1Read},
        {`^htop$`, L1Read},
        {`^journalctl(\s+|$)`, L1Read},
        {`^systemctl\s+status\s+`, L1Read},
        {`^netstat(\s+|$)`, L1Read},
        {`^ss(\s+|$)`, L1Read},
        {`^curl\s+`, L1Read},
        {`^wget\s+`, L1Read},
    }

    rules := make([]rule, 0, len(patterns))
    for _, p := range patterns {
        rules = append(rules, rule{
            pattern: regexp.MustCompile(p.pattern),
            level:   p.level,
        })
    }
    return rules
}
```

- [ ] **Step 5: 运行测试**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/permission/... -v -run TestClassifier_StaticRules
```

Expected: 全部 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/permission/classifier.go internal/permission/classifier_test.go
git commit -m "feat(permission): static rule classifier with L1-L4 risk levels"
```

---

## Task 3: 模式执行器

**Files:**
- Create: `internal/permission/enforcer.go`
- Create: `internal/permission/enforcer_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/permission/enforcer_test.go
package permission_test

import (
    "testing"
    "github.com/spiderai/spider/internal/permission"
)

func TestEnforcer_Decide(t *testing.T) {
    tests := []struct {
        mode  permission.Mode
        level permission.RiskLevel
        want  permission.Decision
    }{
        // readonly: 只允许 L1
        {permission.ModeReadonly, permission.L1Read, permission.DecisionAllow},
        {permission.ModeReadonly, permission.L2Write, permission.DecisionDeny},
        {permission.ModeReadonly, permission.L3Dangerous, permission.DecisionDeny},
        {permission.ModeReadonly, permission.L4Destroy, permission.DecisionDeny},
        // ask: L1/L2 放行，L3/L4 等批准
        {permission.ModeAsk, permission.L1Read, permission.DecisionAllow},
        {permission.ModeAsk, permission.L2Write, permission.DecisionAllow},
        {permission.ModeAsk, permission.L3Dangerous, permission.DecisionPending},
        {permission.ModeAsk, permission.L4Destroy, permission.DecisionPending},
        // auto: L1/L2/L3 放行，L4 等批准
        {permission.ModeAuto, permission.L1Read, permission.DecisionAllow},
        {permission.ModeAuto, permission.L2Write, permission.DecisionAllow},
        {permission.ModeAuto, permission.L3Dangerous, permission.DecisionAllow},
        {permission.ModeAuto, permission.L4Destroy, permission.DecisionPending},
        // plan: 全部返回计划
        {permission.ModePlan, permission.L1Read, permission.DecisionPlan},
        {permission.ModePlan, permission.L2Write, permission.DecisionPlan},
        {permission.ModePlan, permission.L3Dangerous, permission.DecisionPlan},
        {permission.ModePlan, permission.L4Destroy, permission.DecisionPlan},
    }

    e := permission.NewEnforcer()
    for _, tt := range tests {
        got := e.Decide(tt.mode, tt.level)
        if got != tt.want {
            t.Errorf("Decide(%v, %v) = %v, want %v", tt.mode, tt.level, got, tt.want)
        }
    }
}
```

- [ ] **Step 2: 运行确认失败**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/permission/... -run TestEnforcer_Decide 2>&1
```

Expected: `undefined: permission.NewEnforcer`

- [ ] **Step 3: 实现执行器**

```go
// internal/permission/enforcer.go
package permission

// Enforcer 根据权限模式和风险级别决定执行策略
type Enforcer struct{}

func NewEnforcer() *Enforcer { return &Enforcer{} }

// Decide 返回执行决策
func (e *Enforcer) Decide(mode Mode, level RiskLevel) Decision {
    switch mode {
    case ModePlan:
        return DecisionPlan
    case ModeReadonly:
        if level == L1Read {
            return DecisionAllow
        }
        return DecisionDeny
    case ModeAsk:
        if level >= L3Dangerous {
            return DecisionPending
        }
        return DecisionAllow
    case ModeAuto:
        if level >= L4Destroy {
            return DecisionPending
        }
        return DecisionAllow
    default:
        // 未知模式按 ask 处理
        if level >= L3Dangerous {
            return DecisionPending
        }
        return DecisionAllow
    }
}
```

- [ ] **Step 4: 运行测试**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/permission/... -v -run TestEnforcer_Decide
```

Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/permission/enforcer.go internal/permission/enforcer_test.go
git commit -m "feat(permission): mode enforcer — auto/ask/plan/readonly decision matrix"
```

---

## Task 4: 审批请求管理

**Files:**
- Create: `internal/permission/approval.go`
- Create: `internal/permission/approval_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/permission/approval_test.go
package permission_test

import (
    "context"
    "testing"
    "time"
    "github.com/spiderai/spider/internal/permission"
)

func TestApprovalManager_ApproveFlow(t *testing.T) {
    m := permission.NewApprovalManager()

    req := m.Create("session-1", "rm /tmp/test.txt", "host-1", permission.L3Dangerous, "rm 命令删除文件")

    if req.ID == "" {
        t.Fatal("expected non-empty ID")
    }
    if req.Status != permission.ApprovalPending {
        t.Fatalf("expected Pending, got %v", req.Status)
    }

    // 在 goroutine 中等待
    resultCh := make(chan permission.ApprovalStatus, 1)
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()
        resultCh <- m.Wait(ctx, req.ID)
    }()

    time.Sleep(10 * time.Millisecond)
    m.Respond(req.ID, permission.ApprovalApproved)

    result := <-resultCh
    if result != permission.ApprovalApproved {
        t.Errorf("expected Approved, got %v", result)
    }
}

func TestApprovalManager_RejectFlow(t *testing.T) {
    m := permission.NewApprovalManager()
    req := m.Create("session-1", "rm /tmp/test.txt", "host-1", permission.L3Dangerous, "rm 命令")

    go func() {
        time.Sleep(10 * time.Millisecond)
        m.Respond(req.ID, permission.ApprovalRejected)
    }()

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    result := m.Wait(ctx, req.ID)
    if result != permission.ApprovalRejected {
        t.Errorf("expected Rejected, got %v", result)
    }
}

func TestApprovalManager_Timeout(t *testing.T) {
    m := permission.NewApprovalManager()
    req := m.Create("session-1", "rm /tmp/test.txt", "host-1", permission.L3Dangerous, "rm 命令")

    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()
    result := m.Wait(ctx, req.ID)
    if result != permission.ApprovalRejected {
        t.Errorf("expected Rejected on timeout, got %v", result)
    }
}
```

- [ ] **Step 2: 运行确认失败**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/permission/... -run TestApproval 2>&1 | head -10
```

Expected: `undefined: permission.NewApprovalManager`

- [ ] **Step 3: 实现审批管理器**

```go
// internal/permission/approval.go
package permission

import (
    "context"
    "sync"
    "time"

    "github.com/google/uuid"
)

type ApprovalStatus string

const (
    ApprovalPending  ApprovalStatus = "pending"
    ApprovalApproved ApprovalStatus = "approved"
    ApprovalRejected ApprovalStatus = "rejected"
)

// ApprovalRequest 审批请求
type ApprovalRequest struct {
    ID          string         `json:"approval_id"`
    SessionID   string         `json:"session_id"`
    Command     string         `json:"command"`
    Host        string         `json:"host"`
    RiskLevel   RiskLevel      `json:"risk_level"`
    RiskReason  string         `json:"risk_reason"`
    Status      ApprovalStatus `json:"status"`
    RequestedAt time.Time      `json:"requested_at"`
    RespondedAt *time.Time     `json:"responded_at,omitempty"`
    ApprovedBy  string         `json:"approved_by,omitempty"`
}

type pendingEntry struct {
    req *ApprovalRequest
    ch  chan ApprovalStatus
}

// ApprovalManager 内存审批请求管理
type ApprovalManager struct {
    mu      sync.Mutex
    pending map[string]*pendingEntry
    // subscribers 用于 SSE 推送
    subs   []chan *ApprovalRequest
    subsMu sync.Mutex
}

func NewApprovalManager() *ApprovalManager {
    return &ApprovalManager{
        pending: make(map[string]*pendingEntry),
    }
}

// Create 创建审批请求并通知订阅者
func (m *ApprovalManager) Create(sessionID, command, host string, level RiskLevel, reason string) *ApprovalRequest {
    req := &ApprovalRequest{
        ID:          uuid.New().String(),
        SessionID:   sessionID,
        Command:     command,
        Host:        host,
        RiskLevel:   level,
        RiskReason:  reason,
        Status:      ApprovalPending,
        RequestedAt: time.Now(),
    }
    m.mu.Lock()
    m.pending[req.ID] = &pendingEntry{req: req, ch: make(chan ApprovalStatus, 1)}
    m.mu.Unlock()
    m.notify(req)
    return req
}

// Wait 阻塞等待审批结果，ctx 超时返回 Rejected
func (m *ApprovalManager) Wait(ctx context.Context, id string) ApprovalStatus {
    m.mu.Lock()
    entry, ok := m.pending[id]
    m.mu.Unlock()
    if !ok {
        return ApprovalRejected
    }
    select {
    case status := <-entry.ch:
        return status
    case <-ctx.Done():
        return ApprovalRejected
    }
}

// Respond 响应审批请求
func (m *ApprovalManager) Respond(id string, status ApprovalStatus) bool {
    m.mu.Lock()
    entry, ok := m.pending[id]
    if ok {
        now := time.Now()
        entry.req.Status = status
        entry.req.RespondedAt = &now
        delete(m.pending, id)
    }
    m.mu.Unlock()
    if !ok {
        return false
    }
    entry.ch <- status
    return true
}

// ListPending 返回所有待审批请求
func (m *ApprovalManager) ListPending() []*ApprovalRequest {
    m.mu.Lock()
    defer m.mu.Unlock()
    result := make([]*ApprovalRequest, 0, len(m.pending))
    for _, e := range m.pending {
        result = append(result, e.req)
    }
    return result
}

// Subscribe 订阅新审批请求（用于 SSE 推送）
func (m *ApprovalManager) Subscribe() chan *ApprovalRequest {
    ch := make(chan *ApprovalRequest, 8)
    m.subsMu.Lock()
    m.subs = append(m.subs, ch)
    m.subsMu.Unlock()
    return ch
}

// Unsubscribe 取消订阅
func (m *ApprovalManager) Unsubscribe(ch chan *ApprovalRequest) {
    m.subsMu.Lock()
    defer m.subsMu.Unlock()
    for i, s := range m.subs {
        if s == ch {
            m.subs = append(m.subs[:i], m.subs[i+1:]...)
            close(ch)
            return
        }
    }
}

func (m *ApprovalManager) notify(req *ApprovalRequest) {
    m.subsMu.Lock()
    defer m.subsMu.Unlock()
    for _, ch := range m.subs {
        select {
        case ch <- req:
        default:
        }
    }
}
```

- [ ] **Step 4: 添加 uuid 依赖**

```bash
cd /Users/cw/fty.ai/spider.ai && go get github.com/google/uuid
```

- [ ] **Step 5: 运行测试**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/permission/... -v -run TestApproval
```

Expected: 全部 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/permission/approval.go internal/permission/approval_test.go go.mod go.sum
git commit -m "feat(permission): approval manager with SSE subscribe/notify"
```

---

## Task 5: DB Schema 扩展

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: 读取当前 schema**

读取 `internal/db/schema.go` 确认 `execution_logs` 表结构和 `migrate()` 函数位置。

- [ ] **Step 2: 新增 approvals 表和 execution_logs 字段**

在 `schema.go` 的建表 SQL 中新增 `approvals` 表：

```sql
CREATE TABLE IF NOT EXISTS approvals (
    id           TEXT PRIMARY KEY,
    session_id   TEXT NOT NULL,
    command      TEXT NOT NULL,
    host         TEXT NOT NULL DEFAULT '',
    risk_level   TEXT NOT NULL,
    risk_reason  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending',
    requested_at DATETIME NOT NULL,
    responded_at DATETIME,
    approved_by  TEXT
);
```

在 `migrate()` 函数中新增 ALTER TABLE（幂等）：

```go
alterations := []string{
    `ALTER TABLE execution_logs ADD COLUMN risk_level TEXT NOT NULL DEFAULT ''`,
    `ALTER TABLE execution_logs ADD COLUMN permission_mode TEXT NOT NULL DEFAULT ''`,
    `ALTER TABLE execution_logs ADD COLUMN approval_id TEXT`,
    `ALTER TABLE execution_logs ADD COLUMN approved_by TEXT`,
}
for _, sql := range alterations {
    if _, err := db.Exec(sql); err != nil {
        // SQLite: "duplicate column name" = already migrated, ignore
        if !strings.Contains(err.Error(), "duplicate column name") {
            return fmt.Errorf("migrate: %w", err)
        }
    }
}
```

- [ ] **Step 3: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./internal/db/...
```

Expected: 无错误

- [ ] **Step 4: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): add approvals table, extend execution_logs with permission fields"
```

---

## Task 6: Config 扩展

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: 新增 AgentConfig**

在 `Config` struct 后新增：

```go
type AgentConfig struct {
    PermissionMode string `yaml:"permission_mode"` // default: "ask"
}
```

在 `Config` struct 中新增字段：

```go
Agent AgentConfig `yaml:"agent"`
```

在 `Load()` 函数返回默认值时设置：

```go
cfg.Agent.PermissionMode = "ask"
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./internal/config/...
```

Expected: 无错误

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add agent.permission_mode config field, default ask"
```

---

## Task 7: PermissionManager 注入 App

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `cmd/spider/main.go`

- [ ] **Step 1: App struct 新增字段**

在 `internal/mcp/server.go` 的 `App` struct 中新增：

```go
ApprovalManager *permission.ApprovalManager
Classifier      *permission.Classifier
Enforcer        *permission.Enforcer
PermissionMode  permission.Mode // 全局默认模式
```

- [ ] **Step 2: main.go 初始化**

在 `cmd/spider/main.go` 的 `serve()` 函数中，App 初始化后添加：

```go
classifier := permission.NewClassifier(nil) // LLM fallback 后续接入
enforcer := permission.NewEnforcer()
approvalMgr := permission.NewApprovalManager()

app.Classifier = classifier
app.Enforcer = enforcer
app.ApprovalManager = approvalMgr
app.PermissionMode = permission.Mode(cfg.Agent.PermissionMode)
if app.PermissionMode == "" {
    app.PermissionMode = permission.ModeAsk
}
```

- [ ] **Step 3: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无错误

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/server.go cmd/spider/main.go
git commit -m "feat(mcp): inject PermissionManager into App"
```

---

## Task 8: MCP 工具注入权限检查

**Files:**
- Modify: `internal/mcp/tools.go`

- [ ] **Step 1: 读取 execute_command 工具实现**

读取 `internal/mcp/tools.go` 中 `makeExecuteCommand` 和 `makeExecuteCommandBatch` 函数的完整实现。

- [ ] **Step 2: 在 execute_command 中注入权限检查**

在 `makeExecuteCommand(app *App)` 返回的闭包中，命令执行前插入：

```go
command := getString(args, "command")
hostID  := getString(args, "host_id")

// 权限检查
mode := app.PermissionMode
// 会话级覆盖（从 ctx 中读取，后续 Task 9 实现）
if sessionMode, ok := ctx.Value(sessionModeKey).(permission.Mode); ok && sessionMode != "" {
    mode = sessionMode
}

classification := app.Classifier.Classify(ctx, command)
decision := app.Enforcer.Decide(mode, classification.Level)

switch decision {
case permission.DecisionDeny:
    return toolError(fmt.Sprintf("command denied by permission mode %q (risk: %s)", mode, classification.Level))
case permission.DecisionPlan:
    return toolText(fmt.Sprintf("[PLAN] Would execute on %s: %s\nRisk: %s — %s", hostID, command, classification.Level, classification.Reason))
case permission.DecisionPending:
    sessionID, _ := ctx.Value(sessionIDKey).(string)
    req := app.ApprovalManager.Create(sessionID, command, hostID, classification.Level, classification.Reason)
    status := app.ApprovalManager.Wait(ctx, req.ID)
    if status != permission.ApprovalApproved {
        return toolError(fmt.Sprintf("command rejected by user (risk: %s)", classification.Level))
    }
    // 批准后继续执行（fall through）
}
// DecisionAllow 或批准后：继续原有执行逻辑
```

- [ ] **Step 3: 在 execute_command_batch 中注入权限检查**

在 `makeExecuteCommandBatch` 中，对每个命令执行相同检查。批量操作额外规则：3 台以上主机的 L3 命令升级为 L4：

```go
// 批量操作风险升级
if len(hostIDs) >= 3 && classification.Level >= permission.L3Dangerous {
    classification.Level = permission.L4Destroy
    classification.Reason = fmt.Sprintf("batch operation on %d hosts: %s", len(hostIDs), classification.Reason)
}
```

- [ ] **Step 4: 定义 context key 类型**

在 `tools.go` 顶部新增：

```go
type ctxKey string

const (
    sessionIDKey   ctxKey = "session_id"
    sessionModeKey ctxKey = "permission_mode"
)
```

- [ ] **Step 5: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无错误

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/tools.go
git commit -m "feat(mcp): inject permission check into execute_command and execute_command_batch"
```

---

## Task 9: 审批 HTTP API

**Files:**
- Create: `internal/api/approval.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: 创建审批处理器**

```go
// internal/api/approval.go
package api

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"

    "github.com/spiderai/spider/internal/mcp"
    "github.com/spiderai/spider/internal/permission"
)

// listApprovals GET /api/v1/approvals
func listApprovals(app *mcp.App) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        pending := app.ApprovalManager.ListPending()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]any{"approvals": pending})
    }
}

// respondApproval POST /api/v1/approvals/{id}/approve|reject
func respondApproval(app *mcp.App, status permission.ApprovalStatus) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Extract ID from path: /api/v1/approvals/{id}/approve
        parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/approvals/"), "/")
        if len(parts) == 0 || parts[0] == "" {
            http.Error(w, "missing approval id", http.StatusBadRequest)
            return
        }
        id := parts[0]
        ok := app.ApprovalManager.Respond(id, status)
        if !ok {
            http.Error(w, "approval not found", http.StatusNotFound)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": string(status)})
    }
}

// streamApprovals GET /api/v1/approvals/stream (SSE)
func streamApprovals(app *mcp.App) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        ch := app.ApprovalManager.Subscribe()
        defer app.ApprovalManager.Unsubscribe(ch)

        flusher, ok := w.(http.Flusher)
        if !ok {
            http.Error(w, "streaming not supported", http.StatusInternalServerError)
            return
        }

        for {
            select {
            case req, open := <-ch:
                if !open {
                    return
                }
                data, _ := json.Marshal(req)
                fmt.Fprintf(w, "data: %s\n\n", data)
                flusher.Flush()
            case <-r.Context().Done():
                return
            }
        }
    }
}
```

- [ ] **Step 2: 注册路由**

在 `internal/api/handler.go` 的 `NewRouter` 函数中，在现有路由后新增：

```go
// 审批 API（operator 以上）
mux.Handle("/api/v1/approvals", operatorOrAbove(http.HandlerFunc(listApprovals(app))))
mux.Handle("/api/v1/approvals/stream", operatorOrAbove(http.HandlerFunc(streamApprovals(app))))
mux.Handle("/api/v1/approvals/", operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/approve") {
        respondApproval(app, permission.ApprovalApproved)(w, r)
    } else if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reject") {
        respondApproval(app, permission.ApprovalRejected)(w, r)
    } else {
        http.NotFound(w, r)
    }
})))
```

在 `handler.go` 顶部 import 中新增：

```go
"strings"
"github.com/spiderai/spider/internal/permission"
```

- [ ] **Step 3: 编译验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

Expected: 无错误

- [ ] **Step 4: Commit**

```bash
git add internal/api/approval.go internal/api/handler.go
git commit -m "feat(api): approval endpoints — list, approve, reject, SSE stream"
```

---

## Task 10: 集成验证

- [ ] **Step 1: 运行全部测试**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./...
```

Expected: 全部 PASS，无编译错误

- [ ] **Step 2: 启动服务验证**

```bash
cd /Users/cw/fty.ai/spider.ai && go run ./cmd/spider serve
```

Expected: 服务启动，无 panic

- [ ] **Step 3: 验证审批 API**

```bash
# 需要先登录获取 token（假设 auth 已启用）
curl -s http://localhost:9090/api/v1/approvals \
  -H "Authorization: Bearer <token>" | jq .
```

Expected: `{"approvals": []}`

- [ ] **Step 4: 最终 Commit**

```bash
git add -A
git commit -m "feat(permission): complete permission mode implementation — L1-L4, auto/ask/plan/readonly"
```
