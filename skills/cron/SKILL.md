---
name: spider-cron
description: Use when managing crontab or scheduled tasks on remote hosts via Spider. Triggers: 定时任务、crontab、cron、计划任务、定时执行、查看定时、添加定时、删除定时任务。
---

# Spider-Cron — 远程 Crontab 管理

## 核心模型

通过 `execute_command` 操作远程主机的 crontab。所有修改操作前先展示现有 crontab，修改后验证结果。

---

## 操作速查

| 操作 | 命令 |
|------|------|
| 查看当前用户 crontab | `crontab -l` |
| 查看 root crontab | `sudo crontab -l` |
| 添加一条任务 | `(crontab -l 2>/dev/null; echo "<expr> <cmd>") \| crontab -` |
| 删除指定任务 | `crontab -l \| grep -v "<pattern>" \| crontab -` |
| 替换整个 crontab | `echo "<content>" \| crontab -` |
| 清空 crontab | `crontab -r` |

---

## Cron 表达式速查

```
┌─────── 分钟 (0-59)
│ ┌───── 小时 (0-23)
│ │ ┌─── 日 (1-31)
│ │ │ ┌─ 月 (1-12)
│ │ │ │ ┌ 星期 (0-7, 0和7都是周日)
│ │ │ │ │
* * * * *  command

常用示例：
0 * * * *        每小时整点
*/5 * * * *      每5分钟
0 2 * * *        每天凌晨2点
0 2 * * 0        每周日凌晨2点
0 2 1 * *        每月1日凌晨2点
0 2 1 1 *        每年1月1日凌晨2点
@reboot          开机时执行一次
```

---

## 操作决策

```
用户意图
  ├── 查看定时任务？ → crontab -l
  ├── 添加定时任务？ → 先展示现有 → 添加 → 验证
  ├── 删除定时任务？ → 先展示现有 → grep -v 过滤 → 验证
  └── 替换/重置？   → 先备份现有 → 写入新内容 → 验证
```

---

## 各操作详细步骤

### 查看 crontab

```bash
# 当前用户
execute_command(host_id, "crontab -l 2>/dev/null || echo '(empty)'")

# root 用户
execute_command(host_id, "sudo crontab -l 2>/dev/null || echo '(empty)'")
```

### 添加定时任务

```bash
# 第一步：展示现有 crontab
execute_command(host_id, "crontab -l 2>/dev/null || echo '(empty)'")

# 第二步：追加新任务（不覆盖现有）
execute_command(host_id, "(crontab -l 2>/dev/null; echo '0 2 * * * /path/to/script.sh >> /var/log/script.log 2>&1') | crontab -")

# 第三步：验证
execute_command(host_id, "crontab -l")
```

### 删除指定任务

```bash
# 第一步：展示现有 crontab，确认要删除的行
execute_command(host_id, "crontab -l")

# 第二步：用 grep -v 过滤掉目标行（按关键词匹配）
execute_command(host_id, "crontab -l | grep -v 'script.sh' | crontab -")

# 第三步：验证已删除
execute_command(host_id, "crontab -l")
```

---

## 常见场景

### 备份任务

```bash
# 每天凌晨3点备份数据库
0 3 * * * /opt/scripts/backup-db.sh >> /var/log/backup.log 2>&1
```

### 日志清理

```bash
# 每周日凌晨4点清理30天前的日志
0 4 * * 0 find /var/log/app -name "*.log" -mtime +30 -delete
```

### 服务健康检查

```bash
# 每5分钟检查服务，挂了自动重启
*/5 * * * * systemctl is-active myapp || systemctl restart myapp
```

---

## 安全规则

- 添加或删除前必须先执行 `crontab -l` 展示现有内容
- 修改后必须执行 `crontab -l` 验证结果符合预期
- 删除操作使用 `grep -v` 而非 `crontab -r`（避免误清空）
- 操作 root crontab 需加 `sudo`；普通用户直接使用 `crontab`
- 脚本路径使用绝对路径，输出重定向到日志文件
