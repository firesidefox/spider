---
name: spider-debug
description: Use when diagnosing system-level issues on remote hosts via Spider. Triggers: 排查、故障、CPU 高、内存不足、OOM、磁盘满、系统慢、load 高、服务挂了、debug、诊断、为什么慢、为什么挂。
---

# Spider Debug — 远程主机系统故障排查

## 核心模型

通过 `execute_command` 在远程主机上收集诊断信息，**先收集、后分析、不轻易修复**。
所有命令通过 Spider MCP 执行，凭据不暴露给 Claude。

---

## 故障场景决策树

```
用户描述故障
  ├── CPU 飙升 / 系统卡
  │     ├── execute_command(host_id, "top -bn1 | head -20")
  │     └── execute_command(host_id, "ps aux --sort=-%cpu | head -15")
  │
  ├── 内存不足 / OOM
  │     ├── execute_command(host_id, "free -m")
  │     ├── execute_command(host_id, "dmesg | grep -i oom | tail -20")
  │     └── execute_command(host_id, "ps aux --sort=-%mem | head -15")
  │
  ├── 磁盘满
  │     ├── execute_command(host_id, "df -h")
  │     └── execute_command(host_id, "du -sh /* 2>/dev/null | sort -rh | head -15")
  │
  ├── 系统负载高 (load average 异常)
  │     ├── execute_command(host_id, "uptime")
  │     ├── execute_command(host_id, "iostat -x 1 3")
  │     └── execute_command(host_id, "vmstat 1 3")
  │
  ├── 服务挂了 / 进程不在
  │     ├── execute_command(host_id, "systemctl status <service> --no-pager")
  │     └── execute_command(host_id, "journalctl -u <service> -n 50 --no-pager")
  │
  └── 端口不通 / 连接被拒
        ├── execute_command(host_id, "ss -tlnp")
        └── execute_command(host_id, "iptables -L -n --line-numbers | head -40")
```

---

## 一键全量诊断套餐

当故障原因不明时，先跑全量诊断再分析：

```
# 1. 基础资源快照
execute_command(host_id, "uptime && echo '---' && free -m && echo '---' && df -h")

# 2. 进程 Top 10（CPU + 内存）
execute_command(host_id, "ps aux --sort=-%cpu | head -11")
execute_command(host_id, "ps aux --sort=-%mem | head -11")

# 3. 系统日志最近错误
execute_command(host_id, "journalctl -p err -n 30 --no-pager")

# 4. 内核 OOM / 硬件错误
execute_command(host_id, "dmesg | grep -iE 'oom|error|fail|panic' | tail -20")
```

---

## 规则

1. **先收集，后分析**：拿到数据再给结论，不凭猜测下判断。
2. **不提前修复**：未确认根因前，不执行 kill、restart、rm 等变更操作。
3. **逐步深入**：从全量诊断 → 定位异常指标 → 针对性深挖。
4. **批量场景**：多台主机同类故障，用 `execute_command_batch` 并发收集。
5. **输出结构**：收集完毕后，按「现象 → 数据 → 根因推断 → 建议操作」格式汇报。
