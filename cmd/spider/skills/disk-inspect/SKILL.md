---
name: spider-disk-inspect
description: "Use when checking disk usage, finding large files, or cleaning up space on remote hosts via Spider. Triggers: 磁盘、磁盘使用率、磁盘满了、disk、df、du、大文件、清理磁盘、inode。"
---

# Spider Disk Inspect — 磁盘巡检

## 核心模型

用 `RunCommandBatch` 批量采集磁盘数据，汇总后给出清理建议。清理操作需用户确认后执行。

---

## 标准流程

```
1. 确认目标主机   → ListHosts（按 tag 或用户指定）
2. 批量采集数据   → RunCommandBatch: df + du
3. 汇总分析       → 标出使用率 ≥80% 的挂载点
4. 定位大文件     → 对高危主机深入查找
5. 给出清理建议   → 不主动删除，等用户确认
```

**规则：采集阶段只读。删除/清理操作必须用户明确确认，逐台执行。**

---

## 操作速查

### 磁盘使用率总览

```bash
df -h
```

### inode 使用率（小文件堆积场景）

```bash
df -i
```

### 目录占用 Top10

```bash
du -sh /* 2>/dev/null | sort -rh | head -10
du -sh /var/* 2>/dev/null | sort -rh | head -10
```

### 大文件查找（>100MB）

```bash
find / -xdev -size +100M -printf "%s\t%p\n" 2>/dev/null | sort -rn | head -20
```

### 日志目录占用

```bash
du -sh /var/log/* 2>/dev/null | sort -rh | head -10
```

### 已删除但未释放的文件（进程占用）

```bash
lsof +L1 2>/dev/null | awk 'NR>1 {print $7, $9}' | sort -rn | head -10
```

---

## 常见场景

### 批量巡检所有主机磁盘

```
RunCommandBatch(tag="all", command="df -h | awk 'NR>1 && $5+0>=80 {print $5, $6, $1}'")
```

输出只含使用率 ≥80% 的行，快速定位高危主机。

### 定位某台主机磁盘占用来源

```
1. RunCommand: du -sh /var/log/* | sort -rh | head -10
2. RunCommand: du -sh /tmp/* | sort -rh | head -10
3. RunCommand: find /var/log -name "*.log" -size +50M
```

### 检查 inode 耗尽

```
RunCommandBatch(command="df -i | awk 'NR>1 && $5+0>=80 {print $5, $6}'")
```

inode 耗尽时 df -h 显示空间充足但无法创建文件，需单独检查。

---

## 汇报格式

```
磁盘巡检结果（共 6 台）

⚠ web03  /var  91%  — 建议清理
  /var/log: 12G（最大: app.log 8.2G）
  建议：轮转或清理 /var/log/app.log

⚠ db01   /data 85%  — 关注
  /data/mysql: 42G
  建议：检查慢查询日志、binlog 保留策略

✓ web01  最高 62%  — 正常
✓ web02  最高 58%  — 正常
✓ web04  最高 71%  — 正常
✓ web05  最高 44%  — 正常
```

---

## 清理操作（用户确认后）

### 清理旧日志

```bash
# 查看 30 天前的日志文件
find /var/log -name "*.log.*" -mtime +30 -ls

# 确认后删除
find /var/log -name "*.log.*" -mtime +30-delete
```

### 清理 journald 日志

```bash
journalctl --disk-usage
journalctl --vacuum-time=7d
```

### 清理 /tmp

```bash
find /tmp -mtime +7 -delete
```

### 释放已删除未关闭文件

```bash
# 找到占用进程
lsof +L1 | awk 'NR>1 {print $2, $7, $9}'
# 重启对应服务释放句柄
systemctl restart <service>
```
