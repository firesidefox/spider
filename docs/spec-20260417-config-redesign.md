# Spec: 移除默认用户目录，重新设计配置与命令行

**日期：** 2026-04-17
**状态：** 草稿

---

## 1. 背景与问题

当前 `DefaultConfig()` 硬编码 `~/.spider` 作为默认 DataDir，依赖 `os.UserHomeDir()`。
这导致：

1. **系统级安装**（launchd 以 root 运行）必须通过 `SPIDER_DATA_DIR=/var/lib/spider`
   环境变量绕过，否则 `spdctl` 找不到数据（因为 root 的 `~` 是 `/var/root`）。
2. **`spdctl` 的 `--data-dir` flag 存在 bug**：flag 在 `cobra.Execute()` 之前就被读取，
   永远不生效（`dataDir` 始终为空字符串）。
3. 配置文件路径从 DataDir 推导，但 DataDir 本身依赖用户目录，逻辑耦合。

---

## 2. 目标

- 移除 `DefaultConfig()` 中对 `os.UserHomeDir()` 的依赖。
- 移除 `SPIDER_DATA_DIR` 环境变量支持。
- DataDir 默认值固定为 `/var/lib/spider`，可通过以下方式覆盖（优先级高 → 低）：
  1. 命令行 flag `--data-dir`
  2. 配置文件 `config.yaml` 中的 `data_dir` 字段
  3. 内置默认值 `/var/lib/spider`
- 修复 `spdctl` 的 `--data-dir` flag 不生效的 bug。

---

## 3. 新配置加载优先级

```
优先级（高 → 低）：
1. 命令行 --data-dir flag
2. config.yaml 中的 data_dir 字段
3. 内置默认值：/var/lib/spider
```

配置文件路径（`--config`）独立于 DataDir，不再从 DataDir 推导默认值。
`--config` 未指定时，尝试读取 `$DataDir/config.yaml`（即 `/var/lib/spider/config.yaml`）；
文件不存在时静默跳过，不打印警告。

---

## 4. 命令行接口变更

### 4.1 `spider`（主服务）

```
spider [--config <path>] [--data-dir <path>] [--addr <addr>] [serve|version]
```

| Flag | 说明 | 变更 |
|------|------|------|
| `--config` | 配置文件路径 | 不再有默认值，未指定则不加载文件 |
| `--data-dir` | 数据目录 | 覆盖配置文件和内置默认值 |
| `--addr` | 监听地址 | 不变 |

启动时若 DataDir 为空（理论上不会发生，因为有默认值），打印错误并退出：
```
错误: 未指定数据目录，请通过 --data-dir 指定
```

### 4.2 `spdctl`（管理工具）

```
spdctl --data-dir <path> <subcommand>
```

**修复**：`--data-dir` flag 必须在 `cobra.Execute()` 之后、子命令执行之前生效。
实现方式：将数据库初始化移入 `PersistentPreRunE`，而非 `run()` 顶层。

---

## 5. `config.Load()` 接口变更

```go
// Load 加载配置。path 为空时尝试读取 $DataDir/config.yaml，不存在则静默跳过。
// DataDir 始终有值（默认 /var/lib/spider），不会返回空字符串。
func Load(path string) (*Config, error)
```

`DefaultConfig()` 移除 `os.UserHomeDir()` 调用，DataDir 默认为 `/var/lib/spider`。
移除 `Validate()` 方法（不再需要，DataDir 始终有值）。

---

## 6. 受影响文件

| 文件 | 变更内容 |
|------|----------|
| `internal/config/config.go` | 移除 UserHomeDir，移除 Validate()，调整 Load() |
| `cmd/spider/main.go` | --config 无默认值，启动前无需 Validate() |
| `cmd/spdctl/main.go` | 修复 --data-dir bug，移入 PersistentPreRunE |
| `installer/spider.plist` | 移除 `SPIDER_DATA_DIR` 环境变量，改用 `--data-dir /var/lib/spider` flag |
| `internal/api/install.go` | 删除 `serverInstallScript`、`ServerInstallScriptHandler`、`BinaryDownloadHandler` 及相关路由 |
| `cmd/spider/main.go` | 移除 `/server-install.sh` 路由注册 |
| `docs/spec-20260417-macos-installer.md` | 更新数据目录说明 |

---

## 7. 验收标准

- [ ] `spider` 不带任何参数启动时，使用 `/var/lib/spider` 作为数据目录
- [ ] `spider --data-dir /tmp/test` 正常启动，DataDir 为 `/tmp/test`
- [ ] `spdctl --data-dir /var/lib/spider host list` 正确读取数据（bug 修复验证）
- [ ] `spdctl` 不带 `--data-dir` 时，使用 `/var/lib/spider`
- [ ] `SPIDER_DATA_DIR` 环境变量被忽略（不再读取）
- [ ] `config.Load("")` 不再打印 "config: ... not found" 警告
- [ ] 单元测试覆盖 Load() 的三种优先级场景

---

## 8. 边界

**Always：**
- DataDir 始终有值（内置默认 `/var/lib/spider`），不依赖运行时用户身份

**Never：**
- 不再调用 `os.UserHomeDir()`
- 不再读取 `SPIDER_DATA_DIR` 环境变量

---

## 9. 不在本期范围

- 配置文件格式变更
- 多配置文件支持
- XDG Base Directory 规范支持
