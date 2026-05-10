# 导出会话 — 设计文档

**日期：** 2026-05-10  
**状态：** 已批准

---

## 背景

用户需要将会话内容导出，主要用途是**分享**（发给同事、贴入文档）。需要支持 Markdown（人类可读）和 JSON（结构化）两种格式。

---

## 目标

- 从聊天界面一键导出当前会话
- 支持 Markdown 和 JSON 两种格式
- 后端生成文件，浏览器直接下载

---

## 非目标

- 导入/恢复会话
- 批量导出
- 会话列表页入口（可后续迭代）

---

## 后端设计

### 新增路由

```
GET /api/v1/conversations/:id/export?format=md|json
```

- 鉴权：复用 `verifyConvOwner`，非会话所有者返回 404
- 参数：`format`，取值 `md` 或 `json`，默认 `md`
- 响应头：`Content-Disposition: attachment; filename="<title>.<ext>"`

### Markdown 格式

只输出 `role=user` 和 `role=assistant` 的文本消息，跳过工具调用消息（`tool_calls` 非空且 `content` 为空的消息）。

```markdown
# <会话标题>

> 导出时间：2026-05-10 14:30

---

**User**

<消息内容>

---

**Assistant**

<消息内容>
```

### JSON 格式

序列化完整数据结构：

```json
{
  "conversation": { ...Conversation },
  "messages": [ ...Message ]
}
```

包含所有消息（含工具调用），字段与 `GET /api/v1/conversations/:id` 返回一致。

### 实现位置

新增 `internal/api/chat_export.go`，handler 函数 `chatExportConversation`，在 `router.go` 中注册路由。

---

## 前端设计

### 触发入口

`ChatView.vue` 顶部工具栏（现有取消/权限模式按钮旁），新增"导出"按钮。

点击后展示两个选项：
- 导出为 Markdown
- 导出为 JSON

选择后调用 `/api/v1/conversations/:id/export?format=md|json`，浏览器触发文件下载（`window.location` 或 `<a download>` 方式）。

### API 层

在 `web/src/api/chat.ts` 新增：

```ts
export function exportConversationUrl(id: string, format: 'md' | 'json'): string
```

返回带认证参数的下载 URL，或直接用 `fetch` + `Blob` 触发下载（取决于认证方式）。

---

## 文件名规则

- Markdown：`<会话标题>.md`
- JSON：`<会话标题>.json`
- 标题中的特殊字符替换为 `-`，长度截断到 64 字符

---

## 错误处理

| 情况 | 响应 |
|---|---|
| 会话不存在或无权限 | 404 |
| format 参数非法 | 400 |
| DB 查询失败 | 500 |
