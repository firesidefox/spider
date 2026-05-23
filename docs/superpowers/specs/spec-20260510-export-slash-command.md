# /export 斜杠命令 — 设计文档

**日期：** 2026-05-10  
**状态：** 已批准

---

## 背景

聊天输入框已支持 `/model` 斜杠命令。本需求在此基础上增加 `/export` 命令，让用户无需点击按钮即可通过键盘触发会话导出。

---

## 语法

```
/export [format] [--format <format>]
```

| 输入 | 行为 |
|---|---|
| `/export` | 导出为 Markdown（默认） |
| `/export md` | 导出为 Markdown |
| `/export json` | 导出为 JSON |
| `/export --format md` | 导出为 Markdown |
| `/export --format json` | 导出为 JSON |
| `/export xml` | 错误提示 |
| `/export --format xml` | 错误提示 |

---

## 非目标

- 不支持 `--title` 或其他参数（可后续迭代）
- 不修改后端
- 不修改导出按钮 UI

---

## 实现

### 修改文件

只改 `web/src/views/ChatView.vue`，无后端改动。

### 解析函数

新增 `parseExportFormat(text: string): 'md' | 'json' | 'invalid' | 'default'`：

- 输入为完整命令字符串（如 `"/export json"`）
- 解析逻辑：
  1. 去掉 `/export` 前缀，trim 剩余部分
  2. 若剩余为空 → 返回 `'default'`（即 `'md'`）
  3. 若剩余为 `md` 或 `json` → 返回对应值
  4. 若剩余为 `--format md` 或 `--format json` → 返回对应值
  5. 其他 → 返回 `'invalid'`

### send() 修改

在现有 `/model` 分支之后，`inputText.value = ''` 之前，插入：

```ts
if (text.startsWith('/export')) {
  inputText.value = ''
  const fmt = parseExportFormat(text)
  if (fmt === 'invalid') {
    addSystemMessage('用法：/export [md|json] 或 /export --format [md|json]')
    return
  }
  if (!activeConvId.value) {
    addSystemMessage('没有活跃的会话')
    return
  }
  await exportConversation(activeConvId.value, fmt === 'default' ? 'md' : fmt)
  return
}
```

### 错误提示

复用现有 `addSystemMessage(msg: string)` 函数，输出系统消息到聊天界面，不发送给 LLM。

---

## 错误处理

| 情况 | 行为 |
|---|---|
| 格式非法 | `addSystemMessage('用法：/export [md|json] 或 /export --format [md|json]')` |
| 无活跃会话 | `addSystemMessage('没有活跃的会话')` |
| 导出失败（网络/500） | `exportConversation` 抛出异常，不处理（与现有导出按钮行为一致） |
