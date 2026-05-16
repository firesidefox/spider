---
name: spider-config-diff
description: "Use when comparing configuration files across multiple remote hosts via Spider. Triggers: 配置对比、配置一致性、配置漂移、config diff、哪台机器配置不一样、配置是否同步。"
---

# Spider Config Diff — 多主机配置对比

## 核心模型

用 `RunCommandBatch` 批量读取配置内容，本地 diff 后汇报差异。所有操作只读，无副作用。

---

## 标准流程

```
1. 确认目标主机   → ListHosts（按 tag 过滤，或用户直接指定）
2. 批量读取配置   → RunCommandBatch: cat <config_file>
3. 本地对比内容   → 以第一台为基准，逐台 diff
4. 汇报结果       → 列出差异行，标注哪台不同
5. 用户决定是否同步（不主动执行写操作）
```

**规则：对比阶段全程只读。用户明确要求同步时才进入写操作。**

---

## 操作速查

### 读取单文件

```bash
cat /etc/nginx/nginx.conf
cat /etc/sysctl.conf
cat /etc/hosts
```

### 读取多文件（目录结构）

```bash
find /etc/nginx/conf.d -name "*.conf" | sort
cat /etc/nginx/conf.d/<filename>
```

### 读取关键值（不需要全文对比时）

```bash
# sysctl 单项
sysctl net.ipv4.tcp_max_syn_backlog

# 环境变量文件某行
grep "^MAX_CONN" /etc/app/config.env
```

---

## 常见场景

### 对比所有 web 主机的 nginx.conf

```
1. ListHosts(tag="web")  → 得到主机列表
2. RunCommandBatch(hosts=web_ids, command="cat /etc/nginx/nginx.conf")
3. 以第一台输出为基准，对比其余台
4. 输出：
   - ✓ web02 与 web01 一致
   - ✗ web03 第 42 行不同：worker_processes 4 vs 8
```

### 对比 sysctl 内核参数

```
RunCommandBatch(command="sysctl -a 2>/dev/null | sort")
```

对比输出，找出值不同的参数行。

### 对比 /etc/hosts

```
RunCommandBatch(command="cat /etc/hosts | grep -v '^#' | sort")
```

排序后对比，忽略注释行顺序差异。

---

## 汇报格式

差异存在时：

```
配置对比结果：/etc/nginx/nginx.conf

基准主机：web01
对比主机：web02 web03 web04

✓ web02 — 与基准一致
✗ web03 — 2 处差异：
  第 12 行  基准: worker_processes 4;
            web03: worker_processes 8;
  第 38 行  基准: keepalive_timeout 65;
            web03: keepalive_timeout 30;
✓ web04 — 与基准一致

建议：确认 web03 的差异是否为预期配置，如需同步请告知。
```

无差异时：

```
所有主机 /etc/nginx/nginx.conf 一致。
```

---

## 同步操作（用户确认后）

用户要求将某台同步为基准配置时：

```
1. 备份目标主机现有配置
   RunCommand(host=web03, command="cp /etc/nginx/nginx.conf /tmp/nginx.conf.bak.$(date +%Y%m%d%H%M%S)")
2. 从基准主机读取配置内容
   RunCommand(host=web01, command="cat /etc/nginx/nginx.conf")
3. 将内容写入目标主机（通过 upload_file 或 heredoc）
4. 验证（如适用，如 nginx -t）
5. 重载服务（如适用，如 nginx -s reload）
```
