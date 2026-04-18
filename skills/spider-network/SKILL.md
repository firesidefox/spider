---
name: spider-network
description: Use when diagnosing network issues on remote hosts via Spider. Triggers: 网络、ping、连通性、traceroute、DNS、端口、网络慢、丢包、带宽、curl 不通、网络诊断、能不能访问。
---

# Spider Network — 远程主机网络诊断

## 核心模型

通过 `execute_command` 在远程主机上执行网络诊断命令。
网络命令可能阻塞，**始终显式设置 `timeout_seconds=30`**。

---

## 诊断场景速查

| 场景 | 命令 |
|------|------|
| 连通性测试 | `ping -c 4 <target>` |
| HTTP 可达性 | `curl -I --max-time 5 <url>` |
| 路由追踪 | `traceroute <target>` 或 `tracepath <target>` |
| DNS 解析 | `nslookup <domain>` / `dig <domain>` |
| 端口连通 | `nc -zv <host> <port>` / `telnet <host> <port>` |
| 本机监听端口 | `ss -tlnp` |
| 带宽测速 | `curl -o /dev/null -w "%{speed_download}" <url>` |
| 防火墙规则 | `iptables -L -n --line-numbers` |

---

## 决策树

```
网络问题方向
  ├── 远程主机 → 访问外部
  │     ├── DNS 能解析？
  │     │     execute_command(host_id, "dig google.com +short", timeout_seconds=30)
  │     ├── 能 ping 外网？
  │     │     execute_command(host_id, "ping -c 4 8.8.8.8", timeout_seconds=30)
  │     ├── HTTP 能通？
  │     │     execute_command(host_id, "curl -I --max-time 5 https://example.com", timeout_seconds=30)
  │     └── 路由正常？
  │           execute_command(host_id, "traceroute -m 15 8.8.8.8", timeout_seconds=30)
  │
  ├── 外部 → 访问远程主机
  │     ├── 端口是否监听？
  │     │     execute_command(host_id, "ss -tlnp | grep <port>", timeout_seconds=30)
  │     └── 防火墙是否放行？
  │           execute_command(host_id, "iptables -L INPUT -n | grep <port>", timeout_seconds=30)
  │
  └── 主机 A → 主机 B（内网互通）
        ├── execute_command(host_a, "ping -c 4 <host_b_ip>", timeout_seconds=30)
        ├── execute_command(host_a, "nc -zv <host_b_ip> <port>", timeout_seconds=30)
        └── execute_command(host_a, "traceroute <host_b_ip>", timeout_seconds=30)
```

---

## 常用组合诊断

```
# 全面网络快照（DNS + 外网 + 路由）
execute_command(host_id, "dig google.com +short && ping -c 3 8.8.8.8 && curl -I --max-time 5 https://google.com", timeout_seconds=30)

# 端口监听 + 防火墙
execute_command(host_id, "ss -tlnp && echo '---' && iptables -L -n | head -30", timeout_seconds=30)

# 带宽粗测（下载 10MB 文件）
execute_command(host_id, "curl -o /dev/null -w 'speed: %{speed_download} bytes/s\n' --max-time 20 http://speedtest.tele2.net/10MB.zip", timeout_seconds=30)
```

---

## 规则

1. **所有网络命令必须设置 `timeout_seconds=30`**，防止 ping/traceroute 无限阻塞。
2. `traceroute` 加 `-m 15` 限制跳数，避免超时。
3. `curl` 加 `--max-time 5`，`nc` 加 `-w 3`。
4. 先测连通性，再测 DNS，再测具体端口，逐步缩小范围。
5. 内网问题优先检查防火墙和路由，外网问题优先检查 DNS 和默认网关。
