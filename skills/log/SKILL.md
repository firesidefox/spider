---
name: spider-log
description: Use when viewing, searching, downloading, or analyzing logs on remote hosts via Spider. Triggers: 看日志、查日志、tail log、排查报错、下载日志、grep 日志、log、error log、access log、journal、journalctl、日志分析、最近错误、应用报错。
---

# Spider Log — 远程日志查看与分析

## 核心模型

两种模式：

| 模式 | 工具 | 适用场景 |
|------|------|---------|
| 远程 grep | `execute_command` | 快速搜索、日志文件大、只需关键词 |
| 下载分析 | `download_file` + Read | 深度分析、多文件关联、需要完整上下文 |

---

## 决策规则

```
日志文件大小？
  ├── > 10MB → 远程 grep，不要整个下载
  │     └── execute_command(host_id, "grep -n 'ERROR' /var/log/app.log | tail -100")
  └── ≤ 10MB → 可下载到 /tmp/ 后用 Read 分析
        └── download_file(host_id, remote_path, "/tmp/<host>-<filename>")

需要深度分析？
  ├── 否（快速确认）→ 远程 grep
  └── 是（关联多文件、统计、时间段）→ 下载后分析
```

先用 `execute_command` 检查文件大小再决策：

```
execute_command(host_id, "ls -lh /var/log/app.log")
```

---

## 常用命令模板

### tail 最新日志

```
# 最新 100 行
execute_command(host_id="web-01", command="tail -n 100 /var/log/nginx/error.log")

# 实时（取最新 200 行代替 tail -f）
execute_command(host_id="web-01", command="tail -n 200 /var/log/app/app.log")
```

### grep 关键词

```
# 搜索 ERROR，带行号和上下文
execute_command(host_id="web-01", command="grep -n 'ERROR' /var/log/app.log | tail -50")

# 带上下文（前后 3 行）
execute_command(host_id="web-01", command="grep -n -C 3 'NullPointerException' /var/log/app.log | tail -100")

# 统计错误数量
execute_command(host_id="web-01", command="grep -c 'ERROR' /var/log/app.log")
```

### journalctl 查服务日志

```
# 最近 1 小时
execute_command(host_id="web-01", command="journalctl -u nginx --since '1 hour ago' --no-pager | tail -100")

# 只看错误级别
execute_command(host_id="web-01", command="journalctl -u nginx -p err --since today --no-pager")

# 最新 50 条
execute_command(host_id="web-01", command="journalctl -u nginx -n 50 --no-pager")
```

### 下载日志分析

```
# 下载到 /tmp/，文件名带主机前缀避免冲突
download_file(host_id="web-01", remote_path="/var/log/app/error.log", local_path="/tmp/web-01-error.log")

# 下载后用 Read 工具读取分析
Read("/tmp/web-01-error.log")
```

---

## 常见场景

### 排查服务报错

1. 先确认日志路径和大小：`ls -lh /var/log/<service>/`
2. 文件 > 10MB → 远程 grep 最近错误：`grep -n 'ERROR\|FATAL' /var/log/app.log | tail -50`
3. 文件 ≤ 10MB → 下载到 `/tmp/` 后 Read 分析

### 查 access log 异常请求

```
# 找 5xx 错误
execute_command(host_id="web-01", command="grep ' 5[0-9][0-9] ' /var/log/nginx/access.log | tail -50")

# 统计各状态码数量
execute_command(host_id="web-01", command="awk '{print $9}' /var/log/nginx/access.log | sort | uniq -c | sort -rn")
```

### 批量查多台主机日志关键词

```
execute_command_batch(
  command="grep -c 'ERROR' /var/log/app.log 2>/dev/null || echo '0'",
  tag="prod"
)
```

---

## 规则

- 日志文件 > 10MB 优先远程 grep，不要整个下载
- 下载文件存 `/tmp/`，命名格式 `<host_id>-<filename>` 避免冲突
- 远程命令加 `| tail -N` 限制输出行数，避免返回过多内容
- 不要对日志文件执行写操作（> 重定向、truncate 等）
