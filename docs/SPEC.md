# Spec: 自动化部署配置文件（.spider/deploy.yaml）

## 1. 目标

在项目根目录定义 `.spider/deploy.yaml`，声明式描述"如何将本地产物部署到远程主机"。
Claude Code 读取该文件，先可选地在本地执行构建，再调用 spider 现有 MCP 工具
（`upload_file` + `execute_command`）完成部署。

**spider 服务端无需任何改动。**

**目标用户：** 使用 Claude Code 的开发/运维人员，通过一句话（"帮我部署到 prod"）
触发完整的"构建 → 上传 → 部署"流程。

---

## 2. 配置文件格式

文件名：`.spider/deploy.yaml`，放在项目根目录下的 `.spider/` 隐藏目录中（加入 `.gitignore`，不提交）。

```yaml
# .spider/deploy.yaml — 自动化部署配置
# Claude Code 读取此文件，调用 spider MCP 工具完成部署

deployments:
  production:
    # 【可选】本地构建命令，部署前先执行；构建失败则中止
    build_cmd: make build

    # 目标主机：name 和 tag 可同时存在，取并集
    target:
      tag: prod            # 匹配所有带 prod 标签的主机（并行部署）
      name: extra-01       # 额外指定的单台主机

    # 上传的文件列表
    artifacts:
      - local: ./dist/spider          # 相对于项目根目录
        remote: /usr/local/bin/spider # 远程绝对路径
        mode: "0755"                  # 上传后 chmod（可选）

    # 上传前在每台远程主机执行（可选）
    pre_deploy:
      - systemctl stop spider

    # 上传后在每台远程主机执行（可选）
    post_deploy:
      - systemctl start spider
      - systemctl status spider --no-pager

  staging:
    build_cmd: make build
    target:
      name: staging-01     # 单台主机
    artifacts:
      - local: ./dist/spider
        remote: /usr/local/bin/spider
        mode: "0755"
    post_deploy:
      - systemctl restart spider
```

---

## 3. 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `deployments.<name>` | map | 是 | 部署配置名称，可定义多个 |
| `build_cmd` | string | 否 | 本地构建命令；失败则中止，不执行部署 |
| `target.tag` | string | 至少一个 | 按 tag 匹配主机（并行部署） |
| `target.name` | string | 至少一个 | 指定单台主机名（可与 tag 共存） |
| `artifacts[].local` | string | 是 | 本地路径，相对于项目根目录 |
| `artifacts[].remote` | string | 是 | 远程目标绝对路径 |
| `artifacts[].mode` | string | 否 | 文件权限，如 `"0755"` |
| `pre_deploy` | []string | 否 | 上传前在每台主机执行的命令（顺序执行） |
| `post_deploy` | []string | 否 | 上传后在每台主机执行的命令（顺序执行） |

---

## 4. 执行流程

```
用户：「帮我部署到 production」
          │
          ▼
 Claude 读取 `.spider/deploy.yaml` → 找到 production 配置
          │
          ▼
 [有 build_cmd？]
   是 → 本地执行 build_cmd
        失败？→ 报错，中止，不部署
   否 → 跳过
          │
          ▼
 list_hosts(tag) + get_host(name) → 合并去重，得到主机列表
          │
          ▼
 对每台主机【并行】执行：
   1. pre_deploy 命令（顺序，execute_command）
   2. 上传每个 artifact（upload_file）
   3. 若有 mode → chmod（execute_command）
   4. post_deploy 命令（顺序，execute_command）
          │
          ▼
 汇总所有主机结果 → 报告成功/失败
```

---

## 5. 使用的 MCP 工具（现有，无需新增）

| 步骤 | MCP 工具 | 关键参数 |
|------|----------|---------|
| 查询主机列表 | `list_hosts` | `tag` |
| 查询单台主机 | `list_hosts` 或 `execute_command` | `host_id=name` |
| 执行 pre/post 命令 | `execute_command` | `host_id`, `command` |
| 上传文件 | `upload_file` | `host_id`, `local_path`, `remote_path` |
| 设置权限 | `execute_command` | `host_id`, `command: chmod 0755 <path>` |

---

## 6. 完整示例：部署 spider 本身

```yaml
# spider.ai/.spider/deploy.yaml
deployments:
  production:
    build_cmd: make build
    target:
      tag: prod
    artifacts:
      - local: ./dist/spider
        remote: /usr/local/bin/spider
        mode: "0755"
    pre_deploy:
      - systemctl stop spider
    post_deploy:
      - systemctl start spider
      - systemctl status spider --no-pager

  staging:
    build_cmd: make build
    target:
      name: staging-01
    artifacts:
      - local: ./dist/spider
        remote: /usr/local/bin/spider
        mode: "0755"
    post_deploy:
      - systemctl restart spider
```

**触发示例：**

| 用户说 | Claude 执行 |
|--------|------------|
| 帮我部署到 production | make build → 上传 → 部署所有 prod 主机 |
| 只部署到 staging，不用构建 | 跳过 build_cmd → 上传 → 部署 staging-01 |
| 先 build 再部署到所有环境 | production + staging 依次执行 |

---

## 7. 成功标准

- [ ] 项目 `.spider/` 目录下有 `deploy.yaml`，格式正确
- [ ] Claude Code 能读取配置并识别部署目标
- [ ] `build_cmd` 失败时，Claude 报错并中止，不执行上传
- [ ] 多台主机并行部署，单台失败不影响其他台
- [ ] 每台主机的部署操作自动记录在 spider 审计日志
- [ ] 用户可通过主机名或 tag 灵活指定部署范围

---

## 8. 边界

- **Always：** `local` 路径相对于项目根目录；`build_cmd` 在本地执行，不是远程
- **Ask first：** 是否需要 `timeout` 字段；是否需要多 artifact 并行上传
- **Never：** `.spider/deploy.yaml` 中不存储任何凭据；不修改 spider 服务端代码

---

## 9. 不在本期范围

- Web UI 部署触发入口
- 回滚配置（`rollback_cmd`）
- 部署历史持久化（spider 审计日志已覆盖）
- 跨平台交叉编译参数（由 `build_cmd` 自行处理）
- 定时自动部署
