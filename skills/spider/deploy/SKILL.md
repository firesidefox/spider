---
name: spider-deploy
description: Use when user mentions deploying, releasing, or publishing to remote hosts via spider.ai. Triggers on: 部署、deploy、发布、上线、推送到生产、配置部署、查看部署、更新部署配置。Covers setup, execution, update, and status of deployments.
---

# Spider 自动化部署

## 核心模型

部署 = 本地构建 + SCP 上传 + 远程执行。Claude 负责本地构建，Spider MCP 工具负责所有 SSH 操作。凭据永不暴露给 Claude。

```
本地构建 (build_cmd)
  → upload_file (SCP 上传 artifacts)
  → execute_command (chmod + pre/post 命令)
  → 汇报结果
```

---

## 两条安装路径

| 场景 | 环境 | 前提 |
|------|------|------|
| 新服务器首次安装 | `bootstrap` | 主机已在 spider 列表，本机 spider 运行中 |
| 已有服务升级 | `production` / `staging` | 目标服务器已有 systemd 服务 |

**首次安装必须用 `bootstrap`**：`production` 的 `pre_deploy` 会 stop 服务，新机器虽有 `|| true` 不会中止，但语义上 `bootstrap` 更清晰。

---

## 操作一：执行部署

**触发：** 用户说"部署到 X"，且 `.spider/deploy.yaml` 存在对应环境。

```
1. 读取 .spider/deploy.yaml → 找到目标环境
2. [有 build_cmd？] → 本地执行；失败则报错中止，不继续
3. 解析 target：
   - target.tag  → list_hosts(tag=...) 获取主机列表
   - target.name → 直接用该主机名（bootstrap 时由用户指定）
4. 对每台主机【并行】执行：
   a. pre_deploy 命令（顺序执行，逐条 execute_command）
   b. upload_file(local, remote)
   c. [有 mode？] execute_command("chmod {mode} {remote}")
   d. post_deploy 命令（顺序执行，逐条 execute_command）
5. 汇总：成功 N 台 / 失败 N 台，失败列出错误
```

**关键规则：**
- 单台主机失败不影响其他台（并行继续）
- `build_cmd` 失败必须中止，不上传
- `bootstrap` 的 `target.name` 为空时，询问用户指定主机名

**MCP 工具参数：**

| 工具 | 关键参数 |
|------|---------|
| `list_hosts` | `tag` |
| `execute_command` | `host_id`, `command` |
| `upload_file` | `host_id`, `local_path`, `remote_path` |

---

## 操作二：首次创建配置

**触发：** `.spider/deploy.yaml` 不存在，或用户要配置新环境。

| 步骤 | 操作 |
|------|------|
| 1 | 检查 `Makefile`：找 `build-linux` target → `build_cmd: "make build-linux"` |
| 2 | 确认产物路径：`./bin/spider-linux-amd64` |
| 3 | 调用 `list_hosts` 展示可用主机，让用户选择 |
| 4 | 询问远程路径（默认 `/usr/local/bin/spider`） |
| 5 | 推断 pre/post 命令（二进制服务 → systemctl stop/start） |
| 6 | **展示完整配置，等待用户确认** |
| 7 | 写入 `.spider/deploy.yaml` |
| 8 | 执行部署（按操作一） |

**步骤 6 必须等用户确认，不得跳过。**

---

## 操作三：更新配置

读取 `.spider/deploy.yaml` → 展示目标环境现有配置 → 按用户描述修改 → 展示 diff → 等确认 → 写入。

---

## 操作四：查看配置

直接读取并展示 `.spider/deploy.yaml` 相关内容。

---

## .spider/deploy.yaml 格式

```yaml
deployments:
  bootstrap:                          # 首次安装
    build_cmd: "make build-linux"
    target:
      name: ""                        # 部署时由用户指定主机名
    artifacts:
      - local: ./bin/spider-linux-amd64
        remote: /usr/local/bin/spider
        mode: "0755"
    post_deploy:
      - "useradd --system --no-create-home --shell /usr/sbin/nologin spider || true"
      - "mkdir -p /var/lib/spider && chown spider:spider /var/lib/spider && chmod 700 /var/lib/spider"
      - "printf '[Unit]\\nDescription=Spider MCP Server\\n...' | tee /etc/systemd/system/spider.service"
      - "systemctl daemon-reload && systemctl enable spider && systemctl start spider"
      - "systemctl status spider --no-pager"

  production:                         # 升级部署
    build_cmd: "make build-linux"
    target:
      tag: prod                       # 按标签批量部署
    parallel: true
    artifacts:
      - local: ./bin/spider-linux-amd64
        remote: /usr/local/bin/spider
        mode: "0755"
    pre_deploy:
      - "systemctl stop spider || true"
    post_deploy:
      - "systemctl start spider"
      - "systemctl status spider --no-pager"
```

---

## 安全规则

- `build_cmd` 失败必须中止，不得继续上传
- 首次创建配置必须用户确认，不得静默写入
- 不在配置文件中存储凭据（密码、私钥）
- `local_path` 执行前验证文件存在
- 所有 SSH 操作自动记录在 spider 审计日志
