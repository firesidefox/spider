---
name: spider-nginx
description: Use when managing Nginx configuration or service on remote hosts via Spider. Triggers: nginx、nginx 配置、nginx reload、nginx restart、上传 nginx 配置、nginx -t、反向代理配置、nginx 状态。
---

# Spider Nginx — 远程 Nginx 管理

## 核心模型

`upload_file` 上传配置 + `execute_command` 验证和重载。所有操作通过 Spider MCP 执行。

---

## 标准配置更新流程

```
1. 备份现有配置  → execute_command: cp /etc/nginx/nginx.conf /tmp/nginx.conf.bak
2. 上传新配置    → upload_file: local → /etc/nginx/nginx.conf
3. 验证配置      → execute_command: nginx -t
4. 验证通过      → execute_command: nginx -s reload
   验证失败      → execute_command: cp /tmp/nginx.conf.bak /etc/nginx/nginx.conf
                   中止，报告错误，不执行 reload
```

**规则：nginx -t 失败必须中止，不得强制 reload。**

---

## 操作速查

### 查看状态

```bash
systemctl status nginx --no-pager
nginx -v
ps aux | grep nginx
```

### 验证配置

```bash
nginx -t
nginx -T   # 输出完整配置（含 include）
```

### 重载 / 重启

```bash
# 优先使用 reload（不中断现有连接）
nginx -s reload
# 或
systemctl reload nginx

# 仅在 reload 无效时使用 restart
systemctl restart nginx
```

### 查看日志

```bash
# 最近 50 行 access log
tail -50 /var/log/nginx/access.log

# 最近 50 行 error log
tail -50 /var/log/nginx/error.log

# 实时跟踪 error log
tail -f /var/log/nginx/error.log
```

---

## 回滚机制

上传前备份到 `/tmp/nginx.conf.bak`，验证失败时自动还原：

```
execute_command: cp /tmp/nginx.conf.bak /etc/nginx/nginx.conf
execute_command: nginx -t   # 确认回滚后配置有效
```

---

## 常见场景

### 上传反向代理配置

```
1. 备份：cp /etc/nginx/nginx.conf /tmp/nginx.conf.bak
2. 上传：upload_file(local_path, /etc/nginx/nginx.conf)
3. 验证：nginx -t
4. 通过 → nginx -s reload
   失败 → 还原备份，报告 nginx -t 输出
```

### 批量检查 nginx 状态

```
execute_command_batch(command="systemctl is-active nginx", tag="web")
```

### 查看某台主机 nginx 错误

```
execute_command(host_id, "tail -100 /var/log/nginx/error.log")
```
