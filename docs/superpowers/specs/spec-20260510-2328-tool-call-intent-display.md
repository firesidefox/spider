# Spec: Tool Call Intent Display

**状态：** 部分实现 — `intent` 字段已在 RunCommand/CallAPI 工具 schema 中定义并传递；前端 ChatMessage.vue 尚未渲染 intent 字段

## 背景

Agent 调用工具时，UI 目前只显示原始 JSON 参数。用户需要在执行前看到：
- 操作哪些设备
- 达成什么目标

## 目标

在工具调用 UI 中，以结构化单行格式显示 agent 意图，支持截断和折叠。

## 字段设计

Agent 调用工具时附带 `intent` 字段，值为 **goal 描述**（自然语言，简短）：

```json
{
  "host_ids": ["local110", "local201", "local7"],
  "commands": [...],
  "risk_level": "L2",
  "intent": "重启 nginx 使配置生效"
}
```

`intent` 只含目标描述，**不含设备列表**。设备列表由前端从 `input.host_ids` / `input.host_id` 提取并格式化。

## 显示格式

前端拼接完整显示行：

```
(local110, local201 +1台) -> 重启 nginx 使配置生效
```

- 设备部分：取前 2 个 host_id，超出显示 `+N台`
- `->` 分隔符
- goal 部分：`intent` 字段原文

### 截断规则

- 完整行超过 60 字符：截断 + `...`
- 点击展开完整内容

## 强制程度

| 工具 | intent 要求 |
|------|-------------|
| `RunCommand`、`RunCommandBatch`、`CallAPI` | 必填 |
| `VerifyTool`、`SearchDocs`、`ListDevices`、`GetDeviceInfo`、`TodoTask` | 不需要 |

ACT 类工具必填，EXPLORE 类和 TodoTask 不需要。

缺失时 backend warn log，前端降级：不显示 intent 行（不崩溃）。

## 实现范围

### Backend（Go）

1. `ExecuteCLITool`、`BatchExecuteTool`、`CallRESTAPITool` 的 InputSchema 新增 `intent` 字段（string，必填）
2. `Description()` 注明：必须填写 `intent`（goal 描述，不含设备）
3. System prompt 补充 intent 规范和示例：
   - 只写 goal，不写设备
   - 简短（10 字以内为佳）
   - ACT 类工具必填，不得省略

### Frontend（Vue）

修改 `ChatMessage.vue` ACT tool 渲染部分：

1. 从 `input.host_ids` 或 `input.host_id` 提取设备列表
2. 格式化设备前缀：前 2 个 + `+N台`
3. 拼接完整行：`(devices) -> intent`（无设备时只显示 `-> intent`）
4. 超 60 字符截断 + 展开按钮
5. 无 `intent` 字段时不渲染该行

## 验收标准

1. L2 工具调用显示 `(hostA, hostB +N台) -> <goal>`
2. L1 工具调用同样显示 intent 行
3. 完整行 > 60 字符时截断，点击展开
4. ACT 工具缺失 intent 时 backend 打 warn log，前端不显示该行，不崩溃
5. Go InputSchema 有 `intent` 字段，system prompt 明确 ACT 类工具必填
