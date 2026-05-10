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
- `description`：必填，≤ 250 字符，超限加载时报错
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

每轮准备发送 user message 前，扫描 skill 目录计算 hash（基于文件 mtime，开销小）。hash 变化 → 下个 user turn 注入新版。

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
3. 替换 `${SKILL_DIR}`、`${SESSION_ID}` 变量
4. 包裹在 `<loaded-skill name=X>...</loaded-skill>` 标签内
5. 作为 isMeta user message 注入，标记 `noCompact: true`

### 错误处理

skill 不存在 / frontmatter 解析失败 / description 超限 → 只返 `tool_result` error，不返 `newMessages`，模型 fallback 通用流程：

```
tool_result: "Skill 'deploy' not found"
```

### 重复加载防呆

Description 明确：当前 turn 已见 `<loaded-skill name=X>` 标签 → 不再调用，直接按指令执行。

---

## 5. Compaction 处理

**MVP 策略：** skill content isMeta message 加 `noCompact: true` 标记。

- 若 spider compactor 支持此标记 → 直接生效
- 若不支持 → MVP 接受短会话漂移，文档注明限制：长对话触发压缩后 skill 内容可能丢失，用户重新提及相关任务时 agent 会自动重新调用 `invoke_skill`

**实现前需确认：** 查 `internal/agent/compactor.go`，搜 `noCompact` / `preserve` / `pin` 类标记。

---

## 6. 存储与 API

复用现有路径和 API，无需改动：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/skills` | 列表 |
| GET | `/api/v1/skills/{name}` | 读内容 |
| PUT | `/api/v1/skills/{name}` | 上传/更新（body = SKILL.md 内容） |
| DELETE | `/api/v1/skills/{name}` | 删除 |
| GET | `/api/v1/install/skills.tar.gz` | Claude Code 安装包（共用） |

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

列表每行显示状态：
- ✓ 正常
- ⚠ description 超限 250 字符
- ✗ frontmatter 解析失败

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
- 多文件 skill（目录上传）
- `invoke_skill` fork 执行模式

---

## 9. 关键实现文件

| 文件 | 变更 |
|------|------|
| `internal/agent/factory.go` | 新增 skill 列表加载、hash 计算、预算降级逻辑 |
| `internal/agent/agent.go` | user message 构建时前置 skill 列表块；处理 `invoke_skill` 的 `newMessages` 返回路径 |
| `internal/agent/tools.go` | 新增 `invoke_skill` 工具定义 |
| `internal/api/skills.go` | 新增 frontmatter 校验（上传时） |
| `web/src/views/SkillsPanel.vue` | 健康状态列 + 上传提示 + 前端校验 |
