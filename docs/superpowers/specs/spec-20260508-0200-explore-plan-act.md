# Spec: Explore-Plan-Act Agent 行为约束

**日期**：2026-05-08  
**状态：** 已实现  
**范围**：纯 prompt 层，不改 `Run()` 结构，不改权限机制

---

## 背景

当前 `Agent.Run()` 是单一 turn 循环，agent 拿到任务后直接调工具执行，没有显式的"先探索、再规划、再行动"约束。导致两个问题：

1. **效率低**：反复调工具探索同一信息，浪费 turns
2. **质量差**：没有先想清楚步骤，中途改变方向或走弯路

## 目标

让 agent 在处理任务时自然遵循 Explore → Plan → Act 顺序。

## 方案

纯 prompt 层实现。改动两处：系统提示前缀、工具描述。

### 1. 系统提示前缀

在 `NewAgent` 里将以下内容拼接到用户传入 `SystemPrompt` 之前：

```
## 行为约束

按以下顺序处理任务：

Explore：先用只读工具收集信息，在没有充分了解现状之前不执行有副作用的操作。
Plan：基于探索结果，在内部推理出完整执行步骤，明确每步目的和预期结果。
Act：按计划执行，每步完成后验证结果再继续；遇到意外重新进入 Explore，不盲目继续。
```

优先级：此块在最前，高于角色定义和用户自定义指令。

### 2. 工具描述语义标注

在 `tools_docs.go` 每个工具描述里明确副作用，让 LLM 自己判断阶段归属。不使用显式标签。

**只读工具**（Explore 阶段可自由调用）：

```
Read-only. No side effects. Use freely in Explore phase.
```

**执行工具**（Act 阶段才能调用）：

```
Has side effects. Use only after confirming intent in Plan phase.
Risk depends on the command:
- Read-only commands (ls, cat, grep, ps, df, free, uname, systemctl status): safe, can use in Explore phase
- State-changing commands (rm, kill, systemctl start|stop|restart, apt, yum, chmod, chown): use only in Act phase
```

## 不改动的部分

- `Run()` turn 循环结构不变
- 权限钩子（HookChain、RequireConfirm）不变，prompt 层是额外的行为约束，两者正交
- 不新增工具或协议

## 成功标准

- agent 在执行变更类任务前，先调只读工具收集信息
- agent 不在第一个 turn 直接执行高风险命令
- 现有权限测试通过，无回归
