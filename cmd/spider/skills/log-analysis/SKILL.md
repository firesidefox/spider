---
name: spider-log-analysis
description: Use when analyzing logs on remote hosts via Spider. Triggers: 日志、报错、error、exception、异常、日志分析、log、查日志、错误频率、哪台机器有问题。
---

# Spider Log Analysis — 日志分析

## 核心模型

用 `RunCommand` / `RunCommandBatch` 远程 grep + 统计，结果返回本地分析。不下载完整日志文件。

---

## 标准流程

```
1. 确认目标       → ListHosts 或用户指定
2. 确认日志路径   → 询问用户，或用常见路径探测
3. 批量采集       → RunCommandBatch: grep + tail + awk
4. 本地汇总分析   → 统计频率、定位时间段、提取关键上下文
5. 汇报结论       → 哪台有问题、错误模式、建议下一步
```

---

## 操作速查

### 查看最近错误

```bash
tail -200 /var/log/app/app.log | grep -i "error\|exception\|fatal"
```

### 统计错误频率（按小时）

```bash
grep -i "error" /var/log/app/app.log | awk '{print $1, $2}' | cut -c1-13 | sort | uniq -c | sort -rn | head -20
```

### 查看某时间段日志

```bash
awk '/2026-05-15 14:00/,/2026-05-15 14:30/' /var/log/app/app.log
```

### 提取错误上下文（前后 3 行）

```bash
grep -i "exception" /var/log/app/app.log | tail -5
grep -B2 -A3 "OutOfMemory" /var/log/app/app.log | tail -30
```

### 统计错误类型分布

```bash
grep -i "error" /var/log/app/app.log | grep -oP '(?<=ERROR )\w+' | sort | uniq -c | sort -rn | head -10
```

### 查看 systemd 服务日志

```bash
journalctl -u <service> --since "1 hour ago" --no-pager | grep -i "error\|fail"
journalctl -u <service> -n 100 --no-pager
```

### 探测常见日志路径

```bash
ls /var/log/nginx/ /var/log/app/ /opt/app/logs/ /home/*/logs/ 2>/dev/null
```

---

## 常见场景

### 批量扫描所有主机是否有 ERROR

```
RunCommandBatch(tag="all", command="grep -c 'ERROR' /var/log/app/app.log 2>/dev/null || echo 0")
```

返回每台主机的错误行数，快速定位问题主机。

### 定位某台主机最近异常时间段

```
1. RunCommand: grep -i "error\|exception" /var/log/app/app.log | tail -100
2. 从输出提取时间戳，判断异常集中在哪个时间窗口
3. RunCommand: awk '/TIME_START/,/TIME_END/' /var/log/app/app.log | grep -i "error"
```

### 对比多台主机同一时间段日志

```
RunCommandBatch(command="awk '/2026-05-15 14:00/,/2026-05-15 14:05/' /var/log/app/app.log | grep -i error")
```

### 查找 OOM / 内核错误

```bash
dmesg | grep -i "oom\|killed\|segfault" | tail -20
journalctl -k --since "24 hours ago" | grep -i "error\|fail\|oom"
```

---

## 汇报格式

```
日志分析结果：/var/log/app/app.log（过去 1 小时）

⚠ web03  ERROR 142 次 — 异常集中在 14:02–14:08
  最频繁错误：ConnectionRefused to db01:5432（89 次）
  首次出现：14:02:31
  建议：检查 db01 数据库连接状态

⚠ web04  ERROR 3 次 — 偶发
  错误类型：TimeoutException（3 次）
  建议：观察，暂不处理

✓ web01  无 ERROR
✓ web02  无 ERROR
```

---

## 注意事项

- 日志文件可能很大，**不要直接 cat**，用 tail / grep / awk 限制输出量
- 时间戳格式因应用而异，awk 时间段过滤前先确认格式
- 敏感日志（含密码/token）只提取错误行，不返回完整上下文
