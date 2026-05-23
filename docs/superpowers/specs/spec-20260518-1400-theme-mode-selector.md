# Spec: 主题模式选择器

**状态：** 已实现 — light/dark/system 三模式，localStorage 持久化，matchMedia 系统偏好检测

将顶部导航栏的 ☀️/🌙 切换按钮扩展为下拉选择器，支持三种模式：浅色、深色、跟随系统。

## 改动范围

仅修改两个文件：`web/src/theme.ts` 和 `web/src/App.vue`。不新建文件。

## theme.ts 改动

- `Theme` 类型扩展为 `'dark' | 'light' | 'system'`
- `getSavedTheme()` 默认值改为 `'system'`

## App.vue 改动

### Template

- 移除现有 `<button class="theme-toggle">`
- 替换为 dropdown 结构（复用 `.user-dropdown` 的 click-outside 模式）
- 三个选项：
  - ☀️ 浅色模式 — 始终使用浅色主题
  - 🌙 深色模式 — 始终使用深色主题
  - 🖥 自动模式 — 跟随系统主题设置
- 当前选中项加粗
- 自动模式选中时，底部显示"当前跟随系统：浅色/深色"

### Script

- `theme` ref 类型改为 `'dark' | 'light' | 'system'`
- 新增 `systemIsDark` ref：
  - `onMounted` 时从 `matchMedia('(prefers-color-scheme: dark)')` 初始化
  - 注册 `change` listener 实时响应系统切换
  - `onUnmounted` 清理 listener
- 新增 `resolvedTheme` computed：`system` 时根据 `systemIsDark` 返回 `'dark'|'light'`，否则直接返回 `theme.value`
- `watchEffect` 改用 `resolvedTheme` 注入 CSS 变量
- `isDark` computed 改为基于 `resolvedTheme`
- `provide('isDark', ...)` 保持不变，基于 `resolvedTheme`
- 新增 `showThemeMenu` ref + click-outside 关闭逻辑
- `toggleTheme()` 替换为 `setTheme(mode: Theme)`

### CSS

- `.theme-toggle` 样式保留，改为 dropdown trigger
- 新增 `.theme-dropdown` 定位样式（参考 `.dropdown-menu`）
- 选中项样式：字体加粗 + 主色高亮

## 不变的部分

- 24 个 CSS 变量注入逻辑不变，数据源从 `theme` 改为 `resolvedTheme`
- `provide('isDark', ...)` 接口不变
- localStorage key `'spider-theme'` 不变，值域扩展为包含 `'system'`

## 与对话框主题的关系

本功能控制全局暗/亮模式（App 级 CSS 变量）。对话框多主题（chat dialog 内部配色方案）是独立功能，两者正交。
