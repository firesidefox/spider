# 权限模式设置 — 设计规格

**Date:** 2026-05-04
**Status:** Approved

---

## 1. 概述

为 spider.ai 权限模式系统添加 Web UI 配置能力。用户可通过"个人设置 → 智能体"页面设置权限模式、审批超时，以及管理自定义静态规则。所有配置持久化到 config.yaml。

---

## 2. 配置结构

config.yaml 中 `agent` 段扩展：

```yaml
agent:
  permission_mode: ask          # ask | auto | plan | readonly
  approval_timeout: 300         # 审批超时秒数
  rules:                        # 用户自定义规则（优先于内置）
    - pattern: "^docker\\s+rm"
      level: L3
      description: "docker 删除容器"
    - pattern: "^ansible-playbook"
      level: L2
      description: "Ansible 执行"
```

每条规则三个字段：
- `pattern`：正则表达式
- `level`：L1 / L2 / L3 / L4
- `description`：可选，人类可读说明

Go 结构：

```go
type RuleConfig struct {
    Pattern     string `yaml:"pattern" json:"pattern"`
    Level       string `yaml:"level" json:"level"`
    Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type AgentConfig struct {
    PermissionMode  string       `yaml:"permission_mode"`
    ApprovalTimeout int          `yaml:"approval_timeout"`
    Rules           []RuleConfig `yaml:"rules,omitempty"`
}
```

---

## 3. 匹配优先级

用户自定义规则 → 内置 96 条规则 → LLM fallback → 默认 L3

---

## 4. API 设计

### 4.1 权限模式设置

融入现有 Settings API：

```
GET  /api/v1/settings   → 返回中增加 permission_mode, approval_timeout
PUT  /api/v1/settings   → 支持修改 permission_mode, approval_timeout
```

### 4.2 自定义规则 CRUD

```
GET    /api/v1/permission/rules          → 用户自定义规则列表
POST   /api/v1/permission/rules          → 添加一条规则
PUT    /api/v1/permission/rules/:index   → 修改指定规则（index 为数组下标）
DELETE /api/v1/permission/rules/:index   → 删除指定规则
GET    /api/v1/permission/builtin-rules  → 内置规则列表（只读）
```

每次写操作流程：验证输入 → 更新 config 内存 → 写回 config.yaml → 调用 Classifier.Reload()

### 4.3 请求/响应示例

**POST /api/v1/permission/rules**
```json
{
  "pattern": "^docker\\s+rm",
  "level": "L3",
  "description": "docker 删除容器"
}
```

**GET /api/v1/permission/rules**
```json
[
  {"pattern": "^docker\\s+rm", "level": "L3", "description": "docker 删除容器"},
  {"pattern": "^ansible-playbook", "level": "L2", "description": "Ansible 执行"}
]
```

---

## 5. Classifier 热更新

给 Classifier 添加 `Reload` 方法和读写锁：

```go
type Classifier struct {
    mu    sync.RWMutex
    rules []rule
    llm   LLMClassifier
}

func (c *Classifier) Reload(userRules []RuleConfig) {
    combined := buildUserRules(userRules)
    combined = append(combined, buildStaticRules()...)
    c.mu.Lock()
    c.rules = combined
    c.mu.Unlock()
}

func (c *Classifier) Classify(ctx context.Context, command string) Classification {
    c.mu.RLock()
    defer c.mu.RUnlock()
    // ... 遍历 c.rules
}
```

启动时从 config.yaml 加载用户规则，调用 Reload 初始化。

---

## 6. Web UI

### 6.1 页面位置

个人设置 → 侧边栏"管理"区新增 `🧠 智能体` tab。

### 6.2 布局

```
┌─ 权限模式 ───────────────────────────────┐
│  权限模式      [ ask         ▼]           │
│  审批超时（秒）  [ 300         ]           │
└───────────────────────────────────────────┘

┌─ 自定义规则 ─────────────────────────────┐
│  # │ 正则模式          │ 级别 │ 描述     │
│  1 │ ^docker\s+rm      │ L3   │ ...     │
│  2 │ ^ansible-playbook │ L2   │ ...     │
│  [+ 添加规则]                             │
└───────────────────────────────────────────┘

┌─ 内置规则（只读）── ▶ 展开 ──────────────┐
│  96 条内置规则，折叠面板                   │
└───────────────────────────────────────────┘
```

### 6.3 交互

- 权限模式：下拉框（ask/auto/plan/readonly），修改后保存
- 审批超时：数字输入框
- 自定义规则：表格展示，行内删除按钮，添加按钮弹出表单
- 添加规则表单：pattern 输入框 + level 下拉（L1-L4）+ description 输入框
- 内置规则：折叠面板，展开后只读表格展示

---

## 7. 输入验证

- `permission_mode`：必须为 ask/auto/plan/readonly 之一
- `approval_timeout`：正整数，范围 30-3600
- `pattern`：必须是合法正则表达式（`regexp.Compile` 验证）
- `level`：必须为 L1/L2/L3/L4 之一

---

## 8. 会话级权限模式覆盖

### 8.1 概述

每个 Chat 会话可独立设置权限模式，覆盖全局配置。未设置时使用全局默认值。

### 8.2 数据存储

`conversations` 表新增字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `permission_mode` | TEXT DEFAULT '' | 空 = 使用全局默认 |

### 8.3 API

```
PATCH /api/v1/chat/conversations/:id
Body: { "permission_mode": "auto" }
```

传空字符串恢复为全局默认。复用现有会话更新端点。

### 8.4 执行逻辑

`checkPermission` 优先级：会话级 mode（非空时）→ 全局 mode

### 8.5 UI 入口

两个入口：

1. **会话头部模式徽章**：聊天界面顶部显示当前模式徽章（如 `[ask]`），点击弹出下拉切换
2. **会话设置面板**：会话设置中增加权限模式下拉框

```
┌─ ChatView 顶部 ──────────────────────────┐
│  会话标题              [ask ▼]  [⚙️]      │
└───────────────────────────────────────────┘
```

徽章颜色区分模式：ask=蓝、auto=绿、plan=黄、readonly=灰。

---

## 9. 不在范围内

- 规则导入/导出
- 规则排序拖拽
- 规则测试（输入命令预览匹配结果）
