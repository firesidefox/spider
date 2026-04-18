---
name: spider-monitor
description: Use when checking host health, inspecting services, or running batch inspections via Spider. Triggers: 巡检、健康检查、检查服务状态、check service、monitor、服务是否正常、哪台机器有问题、批量检查、系统状态。
---

# Spider Monitor — 主机巡检

## 核心模型

通过 Spider MCP 工具对远程主机执行健康检查和巡检。优先使用 `execute_command_batch` 并发采集，结果按主机分组展示，异常用 ✗ 标记，正常用 ✓。

---

## 巡检维度

| 维度 | 命令 | 异常判断 |
|------|------|---------|
| CPU 负载 | `uptime` | load avg 超过 CPU 核数 |
| 内存 | `free -m` | available < 总量 10% |
| 磁盘 | `df -h` | 使用率 > 85% |
| 服务状态 | `systemctl is-active <svc>` | 输出非 `active` |

---

## 操作决策树

```
用户意图
  ├── 单台主机？
  │     ├── 全面巡检 → 依次执行 uptime / free -m / df -h
  │     └── 指定服务 → execute_command(host_id, "systemctl is-active <svc>")
  └── 多台主机？
        ├── 有共同标签？ → execute_command_batch(command, tag=...)
        └── 指定名单？  → execute_command_batch(command, host_ids=...)
```

---

## 常见场景

### 全量巡检（最常用）

```
# 所有生产主机 CPU 负载
execute_command_batch(command="uptime", tag="prod")

# 所有生产主机磁盘
execute_command_batch(command="df -h /", tag="prod")

# 所有生产主机内存
execute_command_batch(command="free -m", tag="prod")
```

### 服务健康检查

```
# 批量检查 nginx
execute_command_batch(command="systemctl is-active nginx", tag="web")

# 批量检查多个服务（一次命令）
execute_command_batch(
  command="for s in nginx redis mysql; do echo \"$s: $(systemctl is-active $s)\"; done",
  tag="prod"
)
```

### 资源告警排查

```
# 找出磁盘占用最大的目录
execute_command(host_id="web-01", command="du -sh /* 2>/dev/null | sort -rh | head -10")

# 查看内存占用 top 5 进程
execute_command(host_id="web-01", command="ps aux --sort=-%mem | head -6")

# 查看 CPU 占用 top 5 进程
execute_command(host_id="web-01", command="ps aux --sort=-%cpu | head -6")
```

---

## 输出规范

按主机分组展示，每台主机一个块：

```
web-01
  ✓ CPU   load avg: 0.42, 0.38, 0.31
  ✓ 内存  available: 2.1G / 8G
  ✗ 磁盘  /var 使用率 91%
  ✓ nginx active

web-02
  ✗ nginx inactive (dead)
  → 建议：systemctl status nginx --no-pager 查看详情
```

---

## 规则

- 优先 `execute_command_batch`，并发采集所有主机，不要逐台串行
- 单台失败不影响其他台，失败主机单独标注原因
- 发现异常时，给出下一步排查命令建议
- 不要在巡检命令中包含破坏性操作（restart / stop / kill）
