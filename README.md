# Spider — 智能运维平台

Spider 是一个以 Claude Code 为中心的 SSH 代理网关。通过 MCP 接口向 Claude Code 提供主机管理和远程执行能力，Claude 始终只看到主机 ID/名称，不接触任何密码或密钥。

## 架构

```
Claude Code
    │  MCP (SSE over HTTP)
    ▼
 spider          ← MCP server，凭据代理（后台服务）
    │  SSH
    ▼
远程主机          ← 实际执行命令
```

两个独立二进制：

| 程序 | 用途 |
|------|------|
| `spider` | MCP SSE server，作为后台服务运行 |
| `spdctl` | 命令行管理工具，供运维人员手动操作 |

## 构建

```bash
make build        # 编译到 bin/spider 和 bin/spdctl
make install      # 安装到 $GOPATH/bin
```

## 安装到 Claude Code

Spider 以 Claude Code Plugin 形式分发，一次安装同时注册 Skills（运维技能）和 MCP Server（工具接口）。

### 前置条件

确保 spider 后台服务已启动（默认监听 `:8000`）：

```bash
bin/spider
```

### 安装插件

```bash
claude plugins add /path/to/spider.ai/skills/spider
```

安装后在 `/plugins` 界面可以看到：

```
spider Plugin · ✓ enabled
└ spider MCP · ✓ connected
```

Plugin 包含：
- **10 个 Skills**：主机管理、部署、巡检、日志分析、Nginx、Cron 等运维技能
- **10 个 MCP Tools**：`list_hosts`、`execute_command`、`upload_file` 等工具接口

### 卸载

```bash
claude plugins remove spider
```

## spdctl 使用说明

### 主机管理

```bash
# 添加主机（SSH 私钥认证）
spdctl host add --name web01 --ip 10.0.0.1 --user root --auth key --key ~/.ssh/id_rsa

# 添加主机（密码认证）
spdctl host add --name db01 --ip 10.0.0.2 --user admin --auth password --password mypass

# 添加主机（带 passphrase 的私钥）
spdctl host add --name app01 --ip 10.0.0.3 --user deploy --auth key_password \
  --key ~/.ssh/id_rsa --passphrase mypassphrase

# 添加主机（通过跳板机）
spdctl host add --name internal01 --ip 192.168.1.10 --user root --auth key \
  --key ~/.ssh/id_rsa --proxy <bastion-host-id>

# 添加标签
spdctl host add --name web02 --ip 10.0.0.4 --user root --auth key \
  --key ~/.ssh/id_rsa --tag prod,web

# 列出所有主机
spdctl host list

# 按标签过滤
spdctl host list --tag prod

# JSON 格式输出
spdctl host list --json

# 更新主机信息
spdctl host update web01 --ip 10.0.0.10 --tag prod,web,nginx

# 删除主机
spdctl host rm web01
```

### 远程执行

```bash
# 执行命令
spdctl exec web01 "df -h"
spdctl exec web01 "systemctl status nginx"

# 自定义超时（秒）
spdctl exec web01 "apt upgrade -y" --timeout 300
```

### 连通性测试

```bash
spdctl ping web01
# 输出：{"host":"web01","connected":true,"latency_ms":12}
```

### 执行历史

```bash
# 查看最近 20 条
spdctl history

# 按主机过滤
spdctl history --host web01

# 指定返回条数
spdctl history --n 50
```

## 使用场景

Spider 的核心价值在于让 Claude Code 直接完成运维操作，无需人工 SSH 登录。

### 批量巡检

```
帮我检查所有生产主机的磁盘、内存和 CPU 使用率
```

Claude 会自动对所有 `prod` 标签主机批量执行 `df -h`、`free -m`、`top -bn1`，并汇总分析结果。

### 服务健康检查

```
检查所有 web 节点上 nginx 和 redis 是否正常运行
```

批量执行 `systemctl status`，聚合输出，立刻定位异常节点。

### 应用部署

```
把 bin/app 部署到所有 app 标签的主机，替换旧版本并重启服务
```

Claude 依次执行：上传文件 → 停服 → 替换二进制 → 重启 → 验证进程。

### 配置分发

```
把本地的 nginx.conf 同步到所有 web 主机并 reload
```

逐台上传配置文件，执行 `nginx -t && systemctl reload nginx`，有错误立刻停止。

### 故障排查

```
web-01 响应变慢，帮我查一下原因
```

Claude 自动运行 `top`、`netstat -s`、`dmesg | tail`、查慢查询日志，给出根因分析。

### 日志分析

```
下载 app-01 最近的错误日志并分析
```

下载日志文件后直接在对话中完成分析，无需手动 `scp`。

### 安全审计

```
检查所有主机上是否有非标准用户有 sudo 权限
```

批量读取 `/etc/sudoers`，识别异常账号配置。

### 执行历史追溯

```
查一下昨天在 db-01 上执行了哪些命令
```

通过执行历史记录审计操作轨迹。

---

## MCP 工具列表

注册后 Claude Code 可调用以下工具：

| 工具 | 说明 |
|------|------|
| `list_hosts` | 列出主机，支持 tag 过滤 |
| `add_host` | 添加主机 |
| `remove_host` | 删除主机 |
| `update_host` | 更新主机信息 |
| `execute_command` | 在单台主机执行命令 |
| `execute_command_batch` | 按 tag 或 ID 列表批量执行 |
| `check_connectivity` | 测试 SSH 连通性 |
| `upload_file` | 上传本地文件到远程主机 |
| `download_file` | 从远程主机下载文件 |
| `get_execution_history` | 查询执行历史 |

## 数据存储

所有数据存储在 `~/.spider/`：

```
~/.spider/
├── spider.db      # SQLite 数据库（主机信息 + 执行日志）
├── master.key     # AES-256 加密主密钥（chmod 600，自动生成）
└── config.yaml    # 可选配置文件
```

`config.yaml` 示例：

```yaml
data_dir: ~/.spider
sse:
  addr: :8000
  base_url: http://localhost:8000
ssh:
  default_timeout_seconds: 30
  pool_ttl_seconds: 300
```

环境变量 `SPIDER_DATA_DIR` 可覆盖数据目录。
