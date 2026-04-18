---
name: spider
description: Use whenever operating on remote hosts via Spider MCP tools. Triggers on: 查看主机、执行命令、批量执行、检查服务、上传文件、下载文件、SSH 连通性、执行历史、审计日志、list hosts, run command, batch exec, check service, upload, download, connectivity, execution history. This is the primary skill for all Spider SSH operations.
---

# Spider — SSH 通道管理平台

## 核心模型

Spider 是 SSH 通道管理平台。Claude Code 通过 MCP 工具操作所有被管理主机，**凭据永不暴露给 Claude**——Claude 只使用主机名或 ID，Spider 在进程内解密并建立 SSH 连接。所有操作自动写入审计日志。

```
Claude Code  →  Spider MCP (localhost:8000/mcp)  →  SSH  →  远程主机集群
                  凭据在 Spider 进程内解密，不出进程边界
```

---

## 工具速查

| 工具 | 必填参数 | 可选参数 | 说明 |
|------|---------|---------|------|
| `list_hosts` | — | `tag` | 列出主机，支持标签过滤 |
| `add_host` | `name`, `ip`, `username`, `auth_type`, `credential` | `port`, `passphrase`, `tags` | 添加主机 |
| `remove_host` | `id` | — | 删除主机（id 可为名称） |
| `update_host` | `id` | `name`, `ip`, `port`, `username`, `auth_type`, `credential`, `passphrase`, `tags` | 更新主机 |
| `execute_command` | `host_id`, `command` | `timeout_seconds` | 单台执行 |
| `execute_command_batch` | `command` + (`tag` 或 `host_ids`) | `timeout_seconds` | 批量并发执行 |
| `check_connectivity` | `host_id` | — | 测试 SSH 连通性，返回延迟 |
| `upload_file` | `host_id`, `local_path`, `remote_path` | — | SCP 上传，5 分钟超时 |
| `download_file` | `host_id`, `remote_path`, `local_path` | — | SCP 下载，5 分钟超时 |
| `get_execution_history` | — | `host_id`, `limit`, `offset` | 查询执行历史 |

**参数说明：**
- `host_id`：主机名或 UUID，推荐用名称（可读性好）
- `auth_type`：`password` / `key` / `key_password`
- `credential`：密码明文 或 SSH 私钥 PEM 内容
- `tags`：逗号分隔，如 `prod,web`
- `host_ids`：逗号分隔的主机名或 ID 列表
- `execute_command_batch` 必须提供 `tag` 或 `host_ids` 之一，不可都省略

---

## 操作决策

```
用户意图
  ├── 操作单台主机？ → execute_command(host_id, command)
  ├── 操作多台主机？
  │     ├── 有共同标签？ → execute_command_batch(command, tag=...)
  │     └── 指定名单？  → execute_command_batch(command, host_ids=...)
  ├── 传文件到远程？ → upload_file(host_id, local_path, remote_path)
  ├── 从远程取文件？ → download_file(host_id, remote_path, local_path)
  ├── 检查连通性？  → check_connectivity(host_id)
  ├── 查看主机列表？ → list_hosts([tag])
  └── 查审计记录？  → get_execution_history([host_id], [limit])
```

---

## 常见场景

### 批量巡检（最常用）

```
# 检查所有生产主机磁盘
execute_command_batch(command="df -h /", tag="prod")

# 检查指定主机列表内存
execute_command_batch(command="free -m", host_ids="web-01,web-02,db-01")
```

### 服务健康检查

```
# 批量检查 nginx 状态
execute_command_batch(command="systemctl is-active nginx", tag="web")

# 单台详细诊断
execute_command(host_id="web-01", command="systemctl status nginx --no-pager")
```

### 文件部署（SCP + 执行）

```
# 1. 上传二进制
upload_file(host_id="app-01", local_path="./dist/app", remote_path="/usr/local/bin/app")

# 2. 设权限并重启
execute_command(host_id="app-01", command="chmod 755 /usr/local/bin/app && systemctl restart app")
```

> 完整多主机部署流程见 `spider-deploy` skill。

### 配置分发

```
# 上传配置
upload_file(host_id="web-01", local_path="./nginx.conf", remote_path="/etc/nginx/nginx.conf")

# 验证并重载（配置检查失败则不 reload）
execute_command(host_id="web-01", command="nginx -t && systemctl reload nginx")
```

### 故障排查

```
# 收集诊断信息（长命令加超时）
execute_command(
  host_id="web-01",
  command="top -bn1 | head -20; free -m; df -h; netstat -tlnp 2>/dev/null | head -20",
  timeout_seconds=60
)
```

### 日志分析

```
# 下载日志到本地分析
download_file(host_id="app-01", remote_path="/var/log/app/error.log", local_path="/tmp/app-error.log")
# 然后 Read /tmp/app-error.log 分析内容
```

### 审计查询

```
# 查最近 50 条操作记录
get_execution_history(limit=50)

# 查某台主机的操作历史
get_execution_history(host_id="db-01", limit=20)
```

---

## 主机管理

### 添加主机

```
# 密码认证
add_host(name="web-01", ip="10.0.0.1", username="root", auth_type="password", credential="mypassword", tags="prod,web")

# SSH 私钥认证
add_host(name="db-01", ip="10.0.0.2", username="ubuntu", auth_type="key", credential="-----BEGIN OPENSSH PRIVATE KEY-----\n...", tags="prod,db")

# 带 passphrase 的私钥
add_host(name="app-01", ip="10.0.0.3", username="deploy", auth_type="key_password",
         credential="-----BEGIN OPENSSH PRIVATE KEY-----\n...", passphrase="mypass", tags="prod,app")
```

### 更新主机

```
# 只更新需要改的字段
update_host(id="web-01", ip="10.0.0.10")
update_host(id="web-01", tags="prod,web,nginx")
```

---

## 规则

1. **凭据安全**：不在对话或文件中存储、展示、传递凭据明文；add_host 时直接传给 MCP 工具
2. **批量优先**：多台主机相同操作用 `execute_command_batch`，不要循环调用 `execute_command`
3. **超时设置**：长时间命令（编译、备份、日志收集）显式设置 `timeout_seconds`
4. **先检查连通性**：对新加主机或长时间未操作的主机，先 `check_connectivity` 再执行
5. **文件路径**：`upload_file` 的 `local_path` 必须是本地实际存在的文件，执行前验证
6. **审计自动记录**：所有 execute_command / execute_command_batch / upload_file / download_file 操作自动写入审计日志，无需额外处理
