## Spider 面向 Claude Code 的开发需求：智能运维平台（SSH 代理 + MCP 接口）
Spider 面向 Claude Code 的开发需求：智能运维平台（SSH 代理 + MCP 接口）

Spider 是一个以 Claude Code/Open Code 等 AI Agent 为中心的智能运维系统，具备主机管理与 SSH 代理能力，并通过 MCP（Model Context Protocol）接口向 Claude Code 提供 SSH 连接能力。

### 核心功能模块
#### 主机管理模块
支持在 web 界面进行增删改查。
主机元数据：
- 主机名称
- IP 地址
- SSH 端口（默认 22）
- 登录方式（密码 / 私钥）
- 用户名
- 主机标签/分组（如 prod/db/web）

#### SSH 代理服务
平台内部运行一个 SSH 代理网关（类似 Bastion Host），不直接暴露主机 SSH 凭据给 Claude code。当 Claude 通过 MCP 请求执行命令时：
1. 平台根据主机 ID 查找对应连接信息；
2. 通过内部安全通道（如 paramiko 或 OpenSSH 客户端）建立 SSH 会话；
3. 执行命令并返回 stdout/stderr。
