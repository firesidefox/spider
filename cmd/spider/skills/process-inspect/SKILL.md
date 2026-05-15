---
name: spider-process-inspect
description: Use when checking process health on remote hosts via Spider. Triggers: 进程、进程挂了、服务是否在跑、内存泄漏、CPU 飙高、僵尸进程、process、ps、top、OOM。
---

# Spider Process Inspect — 进程巡检

## 核心模型

用 `RunCommandBatch` 批量采集进程状态，本地分析后汇报异常。重启/kill 操作需用户确认。

---

## 标准流程

```
1. 确认目标       → ListHosts 或用户指定
2. 确认关键进程   → 询问用户，或从 systemd 服务列表推断
3. 批量采集       → RunCommandBatch: ps + systemctl + top
4. 分析异常       → 进程缺失、内存/CPU 超阈值、僵尸进程
5. 汇报结论       → 哪台有问题、异常类型、建议操作
6. 执行操作       → 用户确认后重启/kill
```

**规则：采集只读。重启/kill 必须用户确认，逐台执行，不批量 kill。**

---

## 操作速查

### 检查进程是否存活

```bash
pgrep -x nginx || echo "NOT RUNNING"
systemctl is-active nginx
```

### 查看内存占用 Top10

```bash
ps aux --sort=-%mem | head -11
```

### 查看 CPU 占用 Top10

```bash
ps aux --sort=-%cpu | head -11
```

### 查找僵尸进程

```bash
ps aux | awk '$8=="Z" {print $2, $11}'
```

### 查看进程详情（含启动时间、运行时长）

```bash
ps -p <pid> -o pid,ppid,user,%cpu,%mem,vsz,rss,stat,start,etime,cmd
```

### 检查 systemd 服务状态

```bash
systemctl list-units --type=service --state=failed --no-pager
systemctl status <service> --no-pager -l
```

### 查看进程打开的文件数（fd 泄漏）

```bash
ls /proc/<pid>/fd | wc -l
cat /proc/sys/fs/file-max
```

---

## 常见场景

### 批量检查关键服务是否存活

```
RunCommandBatch(tag="web", command="systemctl is-active nginx; systemctl is-active app")
```

### 批量检查内存使用率

```bash
free -m | awk 'NR==2 {printf "%.0f%%\n", $3/$2*100}'
```

```
RunCommandBatch(tag="all", command="free -m | awk 'NR==2 {printf \"%.0f%%\\n\", $3/$2*100}'")
```

### 定位 CPU 飙高进程

```
1. RunCommand: ps aux --sort=-%cpu | head -6
2. 记录 PID，查看进程详情
3. RunCommand: cat /proc/<pid>/cmdline | tr '\0' ' '
4. RunCommand: strace -p <pid> -c -e trace=all 2>&1 | head -20  # 慎用，有开销
```

### 检查内存泄漏趋势

```
RunCommand: ps -p <pid> -o rss= && date
```

间隔多次采集，观察 RSS 是否持续增长。

### 查找并清理僵尸进程

```
1. RunCommand: ps aux | awk '$8=="Z" {print $2, $3}'
2. 找到僵尸进程的父进程：ps -p <ppid> -o pid,cmd
3. 重启父进程（而非 kill 僵尸）以回收
```

---

## 汇报格式

```
进程巡检结果（共 4 台）

⚠ web03  nginx — inactive（服务未运行）
  上次退出：May 15 13:42，exit code 1
  建议：检查配置后重启

⚠ db01   mysqld — CPU 94%，持续 8 分钟
  PID: 1823，RSS: 12.4G / 16G
  建议：检查慢查询，考虑重启

⚠ web04  2 个僵尸进程
  PPID: 3021 (worker.sh)
  建议：重启 worker.sh 父进程回收僵尸

✓ web01  所有服务正常，内存 61%，CPU <5%
✓ web02  所有服务正常，内存 58%，CPU <5%
```

---

## 重启操作（用户确认后）

```bash
# 优先 systemctl
systemctl restart <service>

# 确认重启后状态
systemctl status <service> --no-pager

# 无 systemd 时
kill -HUP <pid>   # 优雅重载
kill -TERM <pid>  # 优雅退出
kill -KILL <pid>  # 强制（最后手段）
```
