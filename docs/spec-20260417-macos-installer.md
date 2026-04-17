# Spec: macOS 安装包

**日期：** 2026-04-17  
**状态：** 草稿

---

## 1. 目标

为 Spider 智能运维平台提供 macOS 安装包：

- 解压 zip，执行 `sudo ./install.sh` 完成安装
- 安装后 `spider` 作为 launchd 后台服务自动启动，开机自启
- `spdctl` 加入 PATH，可直接使用
- 支持卸载（提供 `uninstall.sh`）

**目标用户：** 个人工程师，macOS 12+，arm64 / amd64。

---

## 2. 交付物

| 产物 | 说明 |
|------|------|
| `dist/spider-<version>-arm64.zip` | Apple Silicon 安装包 |
| `dist/spider-<version>-x86_64.zip` | Intel 安装包 |

每个 zip 内含：`spider`、`spdctl`、`install.sh`、`uninstall.sh`、`spider.plist`

---

## 3. 安装行为规格

### 3.1 安装路径

| 文件 | 目标路径 |
|------|----------|
| `spider` 二进制 | `/usr/local/bin/spider` |
| `spdctl` 二进制 | `/usr/local/bin/spdctl` |
| launchd plist | `/Library/LaunchDaemons/ai.fty.spider.plist` |
| 日志目录 | `/var/log/spider/` |
| 数据目录 | `/var/lib/spider`（首次启动时由 spider 自动创建） |

### 3.2 install.sh 行为

```
前置检查：必须以 root 运行（检测 $EUID）
1. 若服务已运行，先 launchctl bootout 停止旧版本
2. 复制 spider / spdctl 到 /usr/local/bin/，chmod 755
3. 创建 /var/log/spider/ 目录，chmod 755
4. 复制 ai.fty.spider.plist 到 /Library/LaunchDaemons/，chmod 644
5. launchctl bootstrap system /Library/LaunchDaemons/ai.fty.spider.plist
6. 验证：curl -sf http://localhost:8000/health || 打印警告（不中止）
```

### 3.3 launchd plist 规格

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>ai.fty.spider</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/spider</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/var/log/spider/spider.log</string>
  <key>StandardErrorPath</key>
  <string>/var/log/spider/spider.err</string>
</dict>
</plist>
```

### 3.4 uninstall.sh 行为

```
前置检查：必须以 root 运行
1. launchctl bootout system/ai.fty.spider（忽略"未加载"错误）
2. rm -f /Library/LaunchDaemons/ai.fty.spider.plist
3. rm -f /usr/local/bin/spider /usr/local/bin/spdctl
4. 打印提示：数据目录 /var/lib/spider 已保留，如需删除请手动执行
```

---

## 4. 构建流程

### 4.1 工具链

- `zip`（标准系统工具，无需第三方）
- 集成到 `Makefile`：`make dist`

### 4.2 zip 内容结构

```
spider-<version>-arm64/
├── spider              # 主服务二进制（darwin/arm64）
├── spdctl              # CLI 工具（darwin/arm64）
├── install.sh          # 安装脚本
├── uninstall.sh        # 卸载脚本
└── spider.plist        # launchd plist
```

### 4.3 Makefile 新增 target

| Target | 说明 |
|--------|------|
| `make build-darwin-arm64` | 编译 darwin/arm64 二进制 |
| `make build-darwin-amd64` | 编译 darwin/amd64 二进制 |
| `make dist` | 打包两个 zip，输出到 `dist/` |

---

## 5. 验收标准

- [ ] `make dist` 输出 `dist/spider-<version>-arm64.zip` 和 `dist/spider-<version>-x86_64.zip`
- [ ] 解压后执行 `sudo ./install.sh` 无报错完成
- [ ] `/usr/local/bin/spider` 和 `/usr/local/bin/spdctl` 存在且可执行
- [ ] `launchctl print system/ai.fty.spider` 显示服务运行中
- [ ] 重启 Mac 后 spider 自动启动
- [ ] `curl http://localhost:8000/health` 返回 200
- [ ] 执行 `sudo ./uninstall.sh` 后服务停止，二进制和 plist 被删除
- [ ] 用户数据 `/var/lib/spider` 在卸载后保留
- [ ] 重复安装（升级场景）不报错，旧服务先停止再替换

---

## 6. 边界

**Always：**
- install.sh / uninstall.sh 首行检测 root，非 root 立即退出并提示 `sudo`
- launchd plist 放 `/Library/LaunchDaemons/`（系统级，所有用户可用）
- 卸载脚本保留 `/var/lib/spider` 数据目录

**Never：**
- 不修改 spider 服务端代码
- 不在安装包中存储任何凭据
- 不强制删除用户已有的 `/var/lib/spider` 数据

---

## 7. 不在本期范围

- Homebrew formula / tap
- 自动更新机制
- Windows / Linux 安装包
