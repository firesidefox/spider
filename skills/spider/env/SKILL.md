---
name: spider-env
description: Use when initializing or configuring a new remote server via Spider. Triggers: 初始化服务器、配置新机器、装依赖、setup server、install docker、配置环境、新机器、环境准备、init server、安装软件、新服务器第一次配置。
---

# Spider-Env — 远程服务器初始化

## 核心模型

标准化新机器初始化流程，通过 `execute_command` 顺序执行各模块。每步执行前展示命令，执行后验证结果；任一步骤失败立即停止并报告。

---

## 初始化模块

| 模块 | 说明 | 关键命令 |
|------|------|---------|
| 系统更新 | 更新软件包索引和已安装包 | `apt update && apt upgrade -y` / `yum update -y` |
| 时区设置 | 统一设为上海时区 | `timedatectl set-timezone Asia/Shanghai` |
| 基础工具 | 常用命令行工具 | `apt install -y curl wget git vim htop unzip` |
| Docker | 官方脚本安装 | `curl -fsSL https://get.docker.com | sh` |
| Swap | 适合小内存机器（≤2GB） | 见下方详细步骤 |
| 防火墙 | 基础 UFW 规则 | 见下方详细步骤 |

---

## 操作决策

**首先询问用户需要哪些模块**，不要全量执行。推荐提问：

```
需要初始化哪些模块？
1. 系统更新
2. 时区设置（Asia/Shanghai）
3. 基础工具（curl/wget/git/vim/htop/unzip）
4. Docker
5. Swap（小内存机器推荐）
6. 防火墙基础规则
```

---

## 各模块详细步骤

### 系统更新

```bash
# Debian/Ubuntu
execute_command(host_id, "apt update && apt upgrade -y")

# CentOS/RHEL
execute_command(host_id, "yum update -y")

# 验证
execute_command(host_id, "uname -r")
```

### 时区设置

```bash
execute_command(host_id, "timedatectl set-timezone Asia/Shanghai")

# 验证
execute_command(host_id, "timedatectl | grep 'Time zone'")
```

### 基础工具

```bash
# Debian/Ubuntu
execute_command(host_id, "apt install -y curl wget git vim htop unzip")

# CentOS/RHEL
execute_command(host_id, "yum install -y curl wget git vim htop unzip")

# 验证
execute_command(host_id, "curl --version && git --version")
```

### Docker 安装

```bash
# 官方一键脚本（适用于 Ubuntu/Debian/CentOS）
execute_command(host_id, "curl -fsSL https://get.docker.com | sh", timeout_seconds=120)

# 启动并设置开机自启
execute_command(host_id, "systemctl enable docker && systemctl start docker")

# 验证
execute_command(host_id, "docker version --format '{{.Server.Version}}'")
```

### Swap 配置（小内存机器）

```bash
# 创建 2GB swap 文件
execute_command(host_id, "fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile")

# 持久化（写入 fstab）
execute_command(host_id, "echo '/swapfile none swap sw 0 0' >> /etc/fstab")

# 验证
execute_command(host_id, "free -h | grep Swap")
```

### 防火墙基础规则（UFW）

```bash
# 安装并配置
execute_command(host_id, "apt install -y ufw")
execute_command(host_id, "ufw default deny incoming && ufw default allow outgoing")
execute_command(host_id, "ufw allow ssh && ufw allow 80/tcp && ufw allow 443/tcp")
execute_command(host_id, "ufw --force enable")

# 验证
execute_command(host_id, "ufw status verbose")
```

---

## 规则

- 每步执行前展示将要运行的命令，等用户确认或直接执行
- 执行后检查返回码和输出，确认成功再进行下一步
- 任一步骤失败立即停止，报告失败原因，不继续后续模块
- 需要 root 权限的命令若当前用户非 root，自动加 `sudo`
