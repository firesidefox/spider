# Spec: Userspace Installer

**状态：** 已实现 — install-arm64.sh 无 sudo 用户空间安装，uninstall.sh 同步路径变量

## uninstall.sh 改动

1. 删除 sudo 要求
2. 同步路径变量（与 install.sh 一致）
3. macOS：`launchctl bootout gui/$(id -u)/ai.fty.spider`
4. Linux：`systemctl --user disable --now spider`

---

## spider.plist 改动

模板文件保留 `__HOME__` 占位符，安装时 `sed` 替换为 `$HOME`：

```xml
<string>__HOME__/.local/bin/spider</string>
<string>--data-dir</string>
<string>__HOME__/.spider/data</string>
```

日志路径同步更新：
```xml
<string>__HOME__/.spider/logs/spider.log</string>
```

---

## 数据迁移（手动，一次性）

```bash
# 停止旧服务
sudo launchctl bootout system/ai.fty.spider 2>/dev/null || true

# 迁移数据
mkdir -p ~/.spider/data ~/.spider/logs ~/.local/bin
sudo cp -r /var/lib/spider/. ~/.spider/data/
sudo chown -R $(id -u):$(id -g) ~/.spider/data/

# 迁移日志（可选）
sudo cp /var/log/spider/spider.log ~/.spider/logs/ 2>/dev/null || true
sudo chown $(id -u):$(id -g) ~/.spider/logs/spider.log 2>/dev/null || true

# 清理旧系统文件（确认迁移成功后执行）
sudo rm -f /usr/local/bin/spider /usr/local/bin/spdctl
sudo rm -f /Library/LaunchDaemons/ai.fty.spider.plist
```

---

## 验收标准

- [ ] `./install.sh` 无需 sudo 可完整执行
- [ ] root 用户运行时报错退出
- [ ] macOS：服务注册到 LaunchAgents，登录后自动启动
- [ ] Linux：服务注册到 systemd user，`--user enable` 成功
- [ ] Linux linger 失败时打印提示而非中断安装
- [ ] `~/.local/bin` 不在 PATH 时打印提示
- [ ] `./uninstall.sh` 无需 sudo 可完整执行

---

## 不在范围内

- 系统级安装（LaunchDaemons / `/usr/local/bin`）
- 自动数据迁移
- Windows 支持
