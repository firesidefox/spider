---
name: routine-system-inspection
description: Use when asked to perform system health check, 巡检, routine inspection, or audit server status across one or more Linux hosts. Covers CPU/memory/disk/network/services/security/certificates.
---

# Routine System Inspection

## Overview

Structured checklist for Linux server health inspections. Run per-host or batch via Spider.

## Inspection Dimensions

| 类别 | 检查项 | 命令 |
|------|--------|------|
| **CPU / 负载** | 使用率、平均负载 | `top -bn1 \| head -5`, `uptime` |
| **内存** | 使用率、swap | `free -h` |
| **磁盘** | 各挂载点使用率、inode | `df -h`, `df -i` |
| **磁盘 IO** | 读写速率 | `iostat -dx 1 3` |
| **进程** | 关键服务、僵尸进程 | `systemctl is-active <svc>`, `ps aux \| grep Z` |
| **服务** | 所有 failed unit | `systemctl list-units --state=failed` |
| **端口** | 监听端口 | `ss -tlnp` |
| **网络连接** | 连接统计 | `ss -s` |
| **防火墙** | 规则状态 | `iptables -L -n \| wc -l` |
| **日志错误** | 近 1h 系统错误 | `journalctl -p err --since "1 hour ago"` |
| **登录记录** | 近期登录、失败尝试 | `last -n 20`, `lastb -n 20` |
| **SSH 配置** | root 登录、密码认证 | `grep -E "PermitRootLogin\|PasswordAuthentication" /etc/ssh/sshd_config` |
| **空密码账户** | 安全审计 | `awk -F: '($2=="")' /etc/shadow` |
| **Cron 任务** | 当前 cron 列表 | `crontab -l`, `ls /etc/cron.*` |
| **内核参数** | 关键 sysctl | `sysctl net.ipv4.tcp_tw_reuse net.core.somaxconn` |
| **证书到期** | SSL 证书剩余天数 | `openssl x509 -in /path/to/cert.pem -noout -dates` |
| **备份状态** | 备份文件更新时间 | `ls -lt /backup/ \| head -5` |
| **大文件** | 占用最大的目录 | `du -sh /* 2>/dev/null \| sort -rh \| head -10` |
| **运行时长** | uptime | `uptime` |

## Quick Start

**单机全量巡检脚本：**

```bash
echo "=== CPU / Load ===" && uptime
echo "=== Memory ===" && free -h
echo "=== Disk ===" && df -h && df -i
echo "=== Failed Services ===" && systemctl list-units --state=failed
echo "=== Listening Ports ===" && ss -tlnp
echo "=== Network Stats ===" && ss -s
echo "=== Recent Errors ===" && journalctl -p err --since "1 hour ago" --no-pager | tail -20
echo "=== Recent Logins ===" && last -n 10
echo "=== Zombie Processes ===" && ps aux | awk '$8=="Z" {print}'
echo "=== Cron Jobs ===" && crontab -l 2>/dev/null
echo "=== Top Disk Usage ===" && du -sh /var /home /tmp /opt 2>/dev/null | sort -rh
```

## 告警阈值参考

| 指标 | 警告 | 严重 |
|------|------|------|
| CPU 使用率 | >80% | >95% |
| 内存使用率 | >85% | >95% |
| 磁盘使用率 | >80% | >90% |
| inode 使用率 | >80% | >90% |
| 负载 (1min) | >CPU核数 | >CPU核数×2 |
| 证书剩余天数 | <30天 | <7天 |

## 巡检报告格式

每项结果标注状态：
- `✓ OK` — 正常
- `⚠ WARN` — 超过警告阈值
- `✗ CRIT` — 超过严重阈值 / 需立即处理

## Common Mistakes

- 忘查 inode —— 磁盘空间够但 inode 满同样导致写入失败
- 只看 `df` 不看 `df -i`
- 证书检查用域名而不是证书文件路径，需改用 `openssl s_client`：
  ```bash
  echo | openssl s_client -connect host:443 2>/dev/null | openssl x509 -noout -dates
  ```
