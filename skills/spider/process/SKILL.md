---
name: spider-process
description: Use when finding, killing, or managing processes on remote hosts via Spider. Triggers: 进程、process、kill、杀进程、查进程、重启进程、ps、pgrep、进程占用、端口占用、谁在用这个端口。
---

# Spider Process — 远程进程管理

## 核心模型

通过 `execute_command` 在远程主机上操作进程，不依赖 systemd。所有操作通过 Spider MCP 执行，凭据不暴露给 Claude。

---

## 操作决策树

```
用户意图
  ├── 按名称找进程？ → ps aux | grep <name>  或  pgrep -la <name>
  ├── 按端口找进程？ → lsof -i :<port>  或  ss -tlnp | grep <port>
  ├── 按 PID 操作？  → kill / top -bn1 -p <PID>
  └── 批量操作？    → pkill <name>  或  execute_command_batch
```

---

## 常用命令

### 查找进程

```bash
# 按名称查找
ps aux | grep <name>
pgrep -la <name>

# 查端口占用
lsof -i :<port>
ss -tlnp | grep <port>

# 查进程资源占用
top -bn1 -p <PID>
```

### Kill 进程

```bash
# 优雅终止（先尝试）
kill <PID>

# 强制终止
kill -9 <PID>

# 按名称批量 kill
pkill <name>
pkill -9 <name>
```

---

## 安全规则

1. kill 前必须先展示进程信息，让用户确认目标正确
2. 不得盲目 kill -9 系统进程（init、systemd、sshd、kernel 线程等）
3. 优先使用 `kill <PID>`（SIGTERM），用户确认无响应后再用 `kill -9`
4. 批量 pkill 前先用 pgrep 预览匹配列表

---

## 常见场景

### 端口被占用

```
1. execute_command: lsof -i :<port>
2. 展示结果，确认进程信息
3. 用户确认后：kill <PID>
```

### 僵尸进程

```
1. execute_command: ps aux | grep 'Z'
2. 找到僵尸进程的父进程：ps -o ppid= -p <PID>
3. kill 父进程（僵尸进程本身无法被 kill）
```

### 内存泄漏进程

```
1. execute_command: ps aux --sort=-%mem | head -10
2. 确认目标进程
3. kill -9 <PID>
```

### 批量 kill 同名进程

```
1. execute_command: pgrep -la <name>   # 预览
2. 用户确认后：pkill <name>
```
