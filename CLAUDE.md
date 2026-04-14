# CLAUDE.md

## 1. 文件写入规范

- 需要写或者修改文件时，**单次写入操作不得超过 50 行**。
- 若内容超过 50 行，应分批写入。

## 2. 自动化部署

当用户提到"部署"、"deploy"、"发布"等意图时：

1. 读取项目根目录的 `.spider/deploy.yaml`
2. 根据用户指定的环境名（如 production、staging）找到对应配置
3. 若有 `build_cmd`，先在本地执行；失败则中止，不继续部署
4. 调用 spider MCP 工具完成部署：
   - `list_hosts` 查询目标主机
   - `execute_command` 执行 pre_deploy 命令
   - `upload_file` 上传 artifacts
   - `execute_command` 执行 chmod（若有 mode）
   - `execute_command` 执行 post_deploy 命令
5. 汇报每台主机的部署结果

**注意：** 多台主机并行部署；单台失败不影响其他台；所有操作自动记录在 spider 审计日志。

## 3. Goal-Driven Execution

**4. Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.
