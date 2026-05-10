# Spider Agent Skill Support — Design Spec

**Date:** 2026-05-11  
**Status:** Draft

---

## 1. 背景与目标

Spider agent 当前 system prompt 写死，无法扩展。用户的运维场景高度定制化（发布流程、巡检 SOP、故障处理规范各不相同），内置提示词无法覆盖。

目标：让用户能定义 skill（Markdown 文件），spider agent 在对话中按需加载并执行，行为与 Claude Code skill 机制对齐。

---

## 2. Skill 文件格式

复用 Claude Code frontmatter 格式，兼容现有工具链：

```markdown
---
description: Use when <场景>. <能力描述>. (≤250 字符，必填)
---

# 正文
[触发条件、步骤、示例]
```

**字段规则：**
- `name` 字段废除，永远用目录名作为 skill 名称
- `description`：必填，≤ 250 字符，上传时 API 严格校验，超限返回 400
- `whenToUse`：不支持，约定将触发条件写入 `description`，格式 `"Use when <场景>. <能力描述>."`
- frontmatter 解析失败时返回明确错误消息（含行号）

**存储路径：**

```
dataDir/skills/<name>/SKILL.md
```

**v1 限制：** 仅支持单文件 skill，不支持目录内多文件引用。`${SKILL_DIR}` 变量保留语法但暂无实际用途。

---

## 3. Skill 列表注入

### 注入位置

skill 列表嵌入 user message 的 content 数组（不是独立 message）：

```json
{
  "role": "user",
  "content": [
    { "type": "text", "text": "<skills>\nNOTE: Replaces any earlier skill list in this conversation.\n- deploy: Use when ...\n</skills>" },
    { "type": "text", "text": "<实际用户输入>" }
  ]
}
```

- **Turn 1**：第一条 user message 前置 skill 列表块
- **列表变化时**：下一个 user turn 前置新版，旧版留历史不动

### 变化检测

每轮 user turn 前扫描 skill 目录：
1. 读各 `SKILL.md` 的 mtime（廉价 stat，无需读文件内容）
2. 任一 mtime 变 → 重解析 frontmatter，对内容算 hash
3. 内容 hash 变 → 下个 user turn 前置新版 skill 列表

### 预算管理

- 总上限：**8KB**（≈ 32 个 skill × 250 字符，足够）
- 每条 description 硬上限：**250 字符**
- 超预算三级降级：
  1. 全量 `description`
  2. 截断 description 到 maxLen
  3. 极端情况：只显示 name

### 无 skill 时

不注入任何内容，行为与现在完全一致，零开销。

---

## 4. `invoke_skill` 工具

### 定义

```
名称:   invoke_skill
参数:   name (string) — skill 名称，对应 dataDir/skills/<name>/SKILL.md
副作用: 只读
```

### Description（强制语气）

```
Execute a skill within the main conversation. When user's request matches
a skill's description, this is a BLOCKING REQUIREMENT: invoke this tool
BEFORE generating any other response. NEVER mention a skill without calling
this tool. If you see <loaded-skill name=X> in current turn, skill already
loaded — follow instructions directly, do NOT call again.
```

### 返回两路

1. **`tool_result`**：短占位 `"Loading skill: <name>"`（满足 API 协议）
2. **`newMessages`**：isMeta user message，含完整 skill 正文，格式：

```
<loaded-skill name=deploy>
Base directory for this skill: /path/to/dataDir/skills/deploy

[SKILL.md 正文（已剥离 frontmatter）]
</loaded-skill>
```

### 加载步骤

1. 读 `SKILL.md`，剥离 frontmatter
2. 前置 `Base directory for this skill: <path>`
3. 替换 `${SKILL_DIR}` 变量（`${SESSION_ID}` 不支持，spider 无 session 概念）
4. 包裹在 `<loaded-skill name=X>...</loaded-skill>` 标签内
5. 作为 isMeta user message 注入

### 错误处理

skill 不存在 / frontmatter 解析失败 / description 超限 → 只返 `tool_result` error，不返 `newMessages`，模型 fallback 通用流程：

```
tool_result: "Skill 'deploy' not found"
```

### 重复加载防呆

Description 明确：当前 turn 已见 `<loaded-skill name=X>` 标签 → 不再调用，直接按指令执行。

---

## 5. Compaction 处理

**已确认：** spider compactor（`internal/agent/compactor.go`）无 `noCompact` / `preserve` / `pin` 类 message 标记机制。

**MVP 策略：** 接受短会话漂移。长对话触发压缩后，`<loaded-skill>` isMeta message 会被压缩丢弃，模型可能忘记 skill 指令。用户重新提及相关任务时，agent 会自动重新调用 `invoke_skill`。

文档注明此限制，compaction restore 列入 v2。

---

## 6. 存储与 API

复用现有路径和 API，无需改动：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/skills` | 列表（含健康状态） |
| GET | `/api/v1/skills/{name}` | 读内容 |
| PUT | `/api/v1/skills/{name}` | 上传/更新（body = SKILL.md 内容），上传时校验 frontmatter |
| DELETE | `/api/v1/skills/{name}` | 删除 |
| GET | `/api/v1/install/skills.tar.gz` | 打包所有 skill 目录为 tar.gz，供 Claude Code plugin 安装时下载；agent skill 与 Claude Code skill 共用同一存储路径，此端点同时服务两者 |

**GET `/api/v1/skills` 响应 schema：**

```json
[
  { "name": "deploy", "description": "Use when...", "status": "ok" },
  { "name": "broken", "description": "", "status": "error", "error": "line 3: missing colon after key" }
]
```

`status` 取值：`ok` | `error`（上传时严格校验，`warn` 状态理论不出现，保留供直接写文件系统的边缘情况）。

**存储路径：**

```
dataDir/skills/<name>/SKILL.md
```

---

## 7. UI（SkillsPanel.vue）

复用现有组件，无结构变化。新增三处：

### 7.1 上传提示文案

```
Skill 供 spider agent 按需加载执行。

SKILL.md 格式:
---
description: Use when <场景>. <能力描述>. (≤250 字符)
---

# 正文
[触发条件、步骤、示例]

v1 限制: 仅支持单文件，不支持目录内多文件引用。
```

### 7.2 健康状态列

列表每行显示状态（仅针对已入库 skill，正常情况只有 ✓）：
- ✓ 正常
- ✗ frontmatter 解析失败（绕过 API 直接写文件系统时可能出现）

点击 ✗ 行展开详细错误（YAML 行号、字段名），方便用户排查。

上传时 API 严格校验（description 缺失、超限、YAML 格式错误）→ 返回 400 + 错误消息，skill 不入库，UI 在上传弹窗显示错误。

### 7.3 上传时前端校验

上传时解析 frontmatter，即时反馈常见错误（description 缺失、超限、YAML 格式错误）。

---

## 8. MVP 范围

**包含：**
- skill 文件格式 + 存储（复用现有）
- skill 列表注入（turn 1 + 变化检测）
- `invoke_skill` 工具
- 预算管理 + 三级降级
- UI 健康状态 + 上传校验

**不包含（v2）：**
- 压缩复原（compaction restore）
- 权限系统（假设用户只用自己的 skill）
- 多文件 skill（目录上传；当前 `<name>/SKILL.md` 目录结构为此预留）
- `invoke_skill` fork 执行模式

---

## 9. 关键实现文件

| 文件 | 变更 |
|------|------|
| `internal/agent/skill_manager.go` | 新建：skill 扫描、frontmatter 解析、hash 计算、预算降级、列表渲染 |
| `internal/agent/factory.go` | 注入 SkillManager 依赖，对话初始化时传入 |
| `internal/agent/agent.go` | user message 构建时前置 skill 列表块；处理 `invoke_skill` 的 `newMessages` 返回路径 |
| `internal/agent/tools.go` | 新增 `invoke_skill` 工具定义 |
| `internal/api/skills.go` | 新增 frontmatter 校验（上传时）、健康状态字段 |
| `web/src/views/SkillsPanel.vue` | 健康状态列 + 上传提示 + 前端校验 |

---

## 10. 验收标准

| 场景 | 预期行为 |
|------|---------|
| 装 deploy skill（description: "Use when 部署"），用户说"帮我发布" | agent 调 `invoke_skill("deploy")`，上下文出现 `<loaded-skill name=deploy>` |
| description > 250 字符的 skill 上传 | API 返回 400，上传弹窗显示错误，skill 不入库 |
| frontmatter YAML 格式错误 | API 返回 400，上传弹窗显示错误含行号；若绕过 API 直接写文件，列表显示 ✗ |
| 删除 skill | 下一个 user turn skill 列表不含该项 |
| 新增 skill（对话进行中） | 下一个 user turn skill 列表含新项，含 replaces 标记 |
| 8KB 预算超限 | 三级降级生效：先截断 description，极端情况只显示 name |
| 同一 turn 内已有 `<loaded-skill name=X>` | 模型不再调用 `invoke_skill("X")`，直接按指令执行（靠 Description 强制语气约束；`invoke_skill` 实现上幂等，重复调用返回同内容不 crash） |
| 用户意图同时匹配多个 skill | 模型自行串联调用（`invoke_skill("deploy")` → `invoke_skill("notify")`），此为预期行为非 bug |
| 无 skill 时 | system prompt 无任何 skill 相关内容，行为与现在完全一致 |
