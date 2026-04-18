# Spec: 安装页面重设计 + Skills 独立入口

**日期：** 2026-04-18  
**范围：** `InstallPanel.vue`、`ProfileView.vue` 侧边栏、`client-install.sh`（后端）

---

## 背景与目标

当前"安装"页面将「客户端安装引导」和「Skills 管理」混在一起，职责不清。  
重设计目标：
1. **安装页面** 专注于引导用户在本机安装 Spider 客户端（Skills + MCP 注册）
2. **Skills** 独立为侧边栏一级入口，与安装解耦
3. **Token 提示**：`claude mcp add` 注册时需要 Token 做身份认证，安装页面提示用户先准备好 Token，并在命令中体现

---

## 背景：安装脚本做了什么

`client-install.sh` 在用户本机执行两件事：
1. 下载 Skills 到 `~/.claude/skills/spider`
2. 调用 `claude mcp add --scope global --transport http spider <SERVER_URL>/mcp` 注册 MCP 服务器

MCP 注册时需要 Token 认证，因此脚本需要支持 `--token` 参数，将其附加到 MCP 注册命令中。

---

## 结构变更

### 侧边栏（ProfileView 管理分组）

```
管理
├── 用户管理
├── 安装          ← 改动：移除 Skills 部分，加 Token 提示
├── Skills        ← 新增独立入口（从安装页拆出）
└── 系统设置
```

---

## 安装页面改动（InstallPanel.vue）

保留现有布局（一键安装命令 + 查看脚本折叠），移除 Skills 管理部分，新增 Token 提示：

### Token 提示区

在 curl 命令上方新增：

- 说明文字：执行安装脚本前需要一个访问令牌（用于注册 MCP 服务器时的身份认证）
- 跳转链接：点击切换到「访问令牌」tab（emit `switch-tab` 事件给 ProfileView）
- Token 输入框：用户粘贴已有 Token

### 安装命令

命令格式更新为：

```
curl -fsSL {origin}/install.sh | sh -s -- --token <YOUR_TOKEN>
```

- Token 输入框有值时：命令中 `<YOUR_TOKEN>` 替换为实际值，复制按钮可用
- Token 输入框为空时：命令保留占位符，复制按钮禁用

### 查看安装脚本（可折叠）

保留不变。

### Skills 管理

从本文件移除，迁移到 `SkillsPanel.vue`。

---

## Skills 页面（新建 SkillsPanel.vue）

内容从 `InstallPanel.vue` 直接拆出，逻辑不变：

- topbar 右侧：[添加 Skill] 按钮
- 拖拽上传区 / 文件选择
- Skills 列表表格（名称、来源、上传新版本、删除）

不需要遵循只读/编辑模式规范。

---

## 后端变更（client-install.sh）

脚本新增 `--token` 参数支持，将 Token 附加到 `claude mcp add` 命令：

```bash
# 用法
curl -fsSL .../install.sh | sh -s -- --token <TOKEN>
```

脚本改动：
```bash
TOKEN=""
while [ $# -gt 0 ]; do
  case $1 in
    --token) TOKEN="$2"; shift 2 ;;
    *) shift ;;
  esac
done

# MCP 注册命令加上 --header 传递 Token
claude mcp add --scope global --transport http \
  --header "Authorization: Bearer $TOKEN" \
  spider "$SPIDER_URL/mcp"
```

Token 为空时给出警告但不中断安装（Skills 下载不需要 Token）。

---

## 前端交互细节

### tab 切换（InstallPanel → 访问令牌）

`InstallPanel.vue` 通过 `emit('switch-tab', 'tokens')` 通知父组件 `ProfileView.vue` 切换 tab。

---

## 验收标准

- [ ] 侧边栏管理分组新增 Skills 入口
- [ ] 安装页移除 Skills 管理部分
- [ ] 安装页新增 Token 提示文字 + 输入框 + 跳转访问令牌 tab 链接
- [ ] Token 已填时命令动态替换，复制按钮可用；未填时禁用
- [ ] SkillsPanel.vue 独立，功能与原来一致
- [ ] client-install.sh 支持 `--token` 参数，附加到 MCP 注册命令

---

## 不在范围内

- agent 部署到远程主机（这个脚本是本机客户端安装）
- 多平台安装命令差异
- Token 有效性校验
