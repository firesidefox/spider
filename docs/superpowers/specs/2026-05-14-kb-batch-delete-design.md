# KB 批量删除设计

**日期:** 2026-05-14  
**状态:** 已批准

## 概述

为知识库（KB）添加批量删除功能，支持同时删除多个文档和多个分组。用户通过侧边栏"编辑"按钮进入多选模式，选中后执行批量删除。

## 后端

### 新增 API 接口

**批量删除文档**
```
DELETE /api/v1/documents
Content-Type: application/json
Body: {"ids": [1, 2, 3]}
Response: 204 No Content
```

**批量删除分组**
```
DELETE /api/v1/document-groups
Content-Type: application/json
Body: {"ids": [1, 2], "delete_documents": true}
Response: 204 No Content
```

`delete_documents` 为 `true` 时删除组内所有文档；为 `false` 时将组内文档 `group_id` 置 NULL（移至未分组）。

### Store 层

**`DocStore.DeleteBatch(ids []int) error`**
- 单事务执行 `DELETE FROM documents WHERE id IN (?)`

**`GroupStore.DeleteBatch(ids []int, deleteDocuments bool) error`**
- 单事务内：
  - 若 `deleteDocuments=true`：先 `DELETE FROM documents WHERE group_id IN (?)` 再删分组
  - 若 `deleteDocuments=false`：先 `UPDATE documents SET group_id=NULL WHERE group_id IN (?)` 再删分组

## 前端

### 状态变更（KnowledgeView.vue）

| 新增状态 | 类型 | 说明 |
|---------|------|------|
| `editMode` | `boolean` | 是否处于多选编辑模式 |
| `selectedDocIds` | `Set<number>` | 已选文档 ID 集合 |
| `selectedGroupIds` | `Set<number>` | 已选分组 ID 集合 |

退出编辑模式时清空两个 Set。

### UI 变化

**编辑模式关闭时（默认）**
- 侧边栏顶部显示"编辑"按钮

**编辑模式开启时**
- "编辑"变为"完成"按钮
- 每个文档/分组条目左侧出现 checkbox
- 底部固定操作栏：显示"已选 N 项" + 删除按钮（无选中时禁用）

### 删除流程

**删除文档**
1. 点击底部删除按钮
2. 弹确认框："确认删除 N 个文档？"
3. 确认 → 调 `deleteBatchDocuments(ids)`
4. 成功后刷新文档列表，退出编辑模式

**删除分组**
1. 点击底部删除按钮（已选分组）
2. 弹确认框，含两个选项：
   - "同时删除组内所有文档"
   - "将文档移至未分组"
3. 确认 → 调 `deleteBatchGroups(ids, deleteDocuments)`
4. 成功后刷新分组和文档列表，退出编辑模式

### API 客户端（documents.ts）

```typescript
deleteBatchDocuments(ids: number[]): Promise<void>
deleteBatchGroups(ids: number[], deleteDocuments: boolean): Promise<void>
```

## 错误处理

- 后端：`ids` 为空返回 400；事务失败返回 500
- 前端：失败时 toast 提示错误，不退出编辑模式，保留选中状态供重试

## 不在范围内

- 跨分组混选（文档和分组同时选中批删）— 分两次操作
- 撤销删除
- 批量移动文档到其他分组
