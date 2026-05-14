# KB 批量删除 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为知识库添加批量删除功能，支持多选文档和分组，通过侧边栏"编辑"模式操作。

**Architecture:** 后端新增两个批量删除接口（`DELETE /api/v1/documents` 和 `DELETE /api/v1/document-groups`），Store 层各加一个事务性批删方法。前端 KnowledgeView.vue 新增编辑模式状态和多选 UI，底部操作栏触发批删。

**Tech Stack:** Go (net/http, database/sql, SQLite), Vue 3 (Composition API), TypeScript

---

## File Map

| 文件 | 操作 | 内容 |
|------|------|------|
| `internal/store/document.go` | Modify | 新增 `DeleteBatch(ids []int) error` |
| `internal/store/group_store.go` | Modify | 新增 `DeleteBatch(ids []int, deleteDocuments bool) error` |
| `internal/store/document_test.go` | Modify | 新增 `TestDocumentStore_DeleteBatch` |
| `internal/store/group_store_test.go` | Create | `TestGroupStore_DeleteBatch_WithDocs` + `TestGroupStore_DeleteBatch_MoveDocs` |
| `internal/api/documents.go` | Modify | 新增 `deleteBatchDocuments` + `deleteBatchGroups` handler |
| `internal/api/handler.go` | Modify | 注册两个新路由（`DELETE /api/v1/documents` 和 `DELETE /api/v1/document-groups` 的 DELETE case） |
| `web/src/api/documents.ts` | Modify | 新增 `deleteBatchDocuments` + `deleteBatchGroups` |
| `web/src/views/KnowledgeView.vue` | Modify | 编辑模式状态、checkbox UI、底部操作栏、删除分组弹窗 |

---

## Task 1: DocumentStore.DeleteBatch — 测试先行

**Files:**
- Modify: `internal/store/document_test.go`
- Modify: `internal/store/document.go`

- [ ] **Step 1: 写失败测试**

在 `internal/store/document_test.go` 末尾追加：

```go
func TestDocumentStore_DeleteBatch(t *testing.T) {
	database := setupTestDB(t)
	ds := NewDocumentStore(database)

	ds.Save("h3c", nil, "doc1", "content1", nil, "f.md", 0, nil)
	ds.Save("h3c", nil, "doc2", "content2", nil, "f.md", 1, nil)
	ds.Save("cisco", nil, "doc3", "content3", nil, "g.md", 0, nil)

	all, _ := ds.List()
	ids := []int{all[0].ID, all[1].ID}

	err := ds.DeleteBatch(ids)
	if err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}
	remaining, _ := ds.List()
	if len(remaining) != 1 {
		t.Errorf("len = %d, want 1", len(remaining))
	}
	if remaining[0].Title != "doc3" {
		t.Errorf("remaining = %q, want doc3", remaining[0].Title)
	}
}
```

- [ ] **Step 2: 确认测试失败**

```bash
go test ./internal/store/... -run TestDocumentStore_DeleteBatch -v
```

Expected: `FAIL` — `ds.DeleteBatch undefined`

- [ ] **Step 3: 实现 DeleteBatch**

在 `internal/store/document.go` 的 `Delete` 方法后追加：

```go
func (s *DocumentStore) DeleteBatch(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	query := "DELETE FROM documents WHERE id IN (?" + strings.Repeat(",?", len(ids)-1) + ")"
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := s.db.Exec(query, args...)
	return err
}
```

在 `document.go` 的 import 块加入 `"strings"`。

- [ ] **Step 4: 确认测试通过**

```bash
go test ./internal/store/... -run TestDocumentStore_DeleteBatch -v
```

Expected: `PASS`

- [ ] **Step 5: 全量 store 测试**

```bash
go test ./internal/store/... -v
```

Expected: 全部 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/document.go internal/store/document_test.go
git commit -m "feat(store): add DocumentStore.DeleteBatch"
```

---

## Task 2: GroupStore.DeleteBatch — 测试先行

**Files:**
- Create: `internal/store/group_store_test.go`
- Modify: `internal/store/group_store.go`

- [ ] **Step 1: 创建测试文件**

新建 `internal/store/group_store_test.go`：

```go
package store

import "testing"

func TestGroupStore_DeleteBatch_WithDocs(t *testing.T) {
	database := setupTestDB(t)
	gs := NewGroupStore(database)
	ds := NewDocumentStore(database)

	g1, _ := gs.Create("group1")
	g2, _ := gs.Create("group2")
	ds.Save("h3c", nil, "doc1", "c1", nil, "f.md", 0, &g1.ID)
	ds.Save("h3c", nil, "doc2", "c2", nil, "f.md", 1, &g1.ID)

	err := gs.DeleteBatch([]int{g1.ID}, true)
	if err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}

	groups, _ := gs.List()
	if len(groups) != 1 || groups[0].ID != g2.ID {
		t.Errorf("expected 1 group (g2), got %d", len(groups))
	}
	docs, _ := ds.List()
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}

func TestGroupStore_DeleteBatch_MoveDocs(t *testing.T) {
	database := setupTestDB(t)
	gs := NewGroupStore(database)
	ds := NewDocumentStore(database)

	g1, _ := gs.Create("group1")
	ds.Save("h3c", nil, "doc1", "c1", nil, "f.md", 0, &g1.ID)
	ds.Save("h3c", nil, "doc2", "c2", nil, "f.md", 1, &g1.ID)

	err := gs.DeleteBatch([]int{g1.ID}, false)
	if err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}

	groups, _ := gs.List()
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
	docs, _ := ds.List()
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
	for _, d := range docs {
		if d.GroupID != nil {
			t.Errorf("doc %d still has group_id %v, want nil", d.ID, *d.GroupID)
		}
	}
}
```

- [ ] **Step 2: 确认测试失败**

```bash
go test ./internal/store/... -run TestGroupStore_DeleteBatch -v
```

Expected: `FAIL` — `gs.DeleteBatch undefined`

- [ ] **Step 3: 实现 GroupStore.DeleteBatch**

在 `internal/store/group_store.go` 的 import 块加入 `"strings"`，在 `Delete` 方法后追加：

```go
func (s *GroupStore) DeleteBatch(ids []int, deleteDocuments bool) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := "?" + strings.Repeat(",?", len(ids)-1)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if deleteDocuments {
		if _, err := tx.Exec("DELETE FROM documents WHERE group_id IN ("+placeholders+")", args...); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec("UPDATE documents SET group_id = NULL WHERE group_id IN ("+placeholders+")", args...); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM document_groups WHERE id IN ("+placeholders+")", args...); err != nil {
		return err
	}
	return tx.Commit()
}
```

- [ ] **Step 4: 确认测试通过**

```bash
go test ./internal/store/... -run TestGroupStore_DeleteBatch -v
```

Expected: 两个测试均 PASS

- [ ] **Step 5: 全量 store 测试**

```bash
go test ./internal/store/... -v
```

Expected: 全部 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/group_store.go internal/store/group_store_test.go
git commit -m "feat(store): add GroupStore.DeleteBatch"
```

---

## Task 3: 后端 API handler + 路由注册

**Files:**
- Modify: `internal/api/documents.go`
- Modify: `internal/api/handler.go:404-415` (documents route) 和 `handler.go:425-435` (document-groups route)

- [ ] **Step 1: 在 documents.go 末尾追加两个 handler**

在 `internal/api/documents.go` 末尾（`moveDocumentToGroup` 函数后）追加：

```go
func deleteBatchDocuments(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	if err := app.DocStore.DeleteBatch(req.IDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func deleteBatchGroups(app *mcppkg.App, w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs             []int `json:"ids"`
		DeleteDocuments bool  `json:"delete_documents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	if err := app.GroupStore.DeleteBatch(req.IDs, req.DeleteDocuments); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: 注册 DELETE /api/v1/documents 路由**

在 `internal/api/handler.go` 找到 `/api/v1/documents` 路由（约 line 404），在 `switch r.Method` 中的 `default:` 前加入 DELETE case：

```go
case http.MethodDelete:
    operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        deleteBatchDocuments(app, w, r)
    })).ServeHTTP(w, r)
```

- [ ] **Step 3: 注册 DELETE /api/v1/document-groups 路由**

在 `internal/api/handler.go` 找到 `/api/v1/document-groups` 路由（约 line 425），在 `switch r.Method` 中的 `default:` 前加入 DELETE case：

```go
case http.MethodDelete:
    operatorOrAbove(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        deleteBatchGroups(app, w, r)
    })).ServeHTTP(w, r)
```

- [ ] **Step 4: 构建验证**

```bash
go build ./...
```

Expected: 无错误

- [ ] **Step 5: Commit**

```bash
git add internal/api/documents.go internal/api/handler.go
git commit -m "feat(api): add batch delete endpoints for documents and groups"
```

---

## Task 4: 前端 API 客户端

**Files:**
- Modify: `web/src/api/documents.ts`

- [ ] **Step 1: 在 documents.ts 末尾追加两个函数**

在 `deleteGroup` 函数后追加：

```typescript
export async function deleteBatchDocuments(ids: number[]): Promise<void> {
  const res = await fetch('/api/v1/documents', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ ids }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function deleteBatchGroups(ids: number[], deleteDocuments: boolean): Promise<void> {
  const res = await fetch('/api/v1/document-groups', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ ids, delete_documents: deleteDocuments }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
}
```

- [ ] **Step 2: 构建验证**

```bash
cd web && npm run build 2>&1 | tail -5
```

Expected: 无 TypeScript 错误

- [ ] **Step 3: Commit**

```bash
git add web/src/api/documents.ts
git commit -m "feat(frontend): add deleteBatchDocuments and deleteBatchGroups API functions"
```

---

## Task 5: KnowledgeView.vue — 编辑模式状态和 UI

**Files:**
- Modify: `web/src/views/KnowledgeView.vue`

此 task 分三步：状态声明、模板改造、删除逻辑。

### 5a: 新增响应式状态

- [ ] **Step 1: 在 script setup 中找到现有 ref 声明区域，追加编辑模式状态**

在 `web/src/views/KnowledgeView.vue` 的 `<script setup>` 中，找到现有 ref 声明（约 `const docs = ref...` 附近），追加：

```typescript
const editMode = ref(false)
const selectedDocIds = ref<Set<number>>(new Set())
const selectedGroupIds = ref<Set<number>>(new Set())
const showDeleteGroupConfirm = ref(false)
const deleteGroupWithDocs = ref(true)
const deleting = ref(false)
const deleteErr = ref('')

function enterEditMode() {
  editMode.value = true
}

function exitEditMode() {
  editMode.value = false
  selectedDocIds.value = new Set()
  selectedGroupIds.value = new Set()
}

function toggleDocSelect(id: number) {
  const s = new Set(selectedDocIds.value)
  s.has(id) ? s.delete(id) : s.add(id)
  selectedDocIds.value = s
}

function toggleGroupSelect(id: number) {
  const s = new Set(selectedGroupIds.value)
  s.has(id) ? s.delete(id) : s.add(id)
  selectedGroupIds.value = s
}

const hasSelection = computed(() =>
  selectedDocIds.value.size > 0 || selectedGroupIds.value.size > 0
)

const selectionLabel = computed(() => {
  const d = selectedDocIds.value.size
  const g = selectedGroupIds.value.size
  if (d > 0 && g > 0) return `已选 ${d} 个文档、${g} 个分组`
  if (d > 0) return `已选 ${d} 个文档`
  return `已选 ${g} 个分组`
})
```

- [ ] **Step 2: 构建验证**

```bash
cd web && npm run build 2>&1 | tail -5
```

Expected: 无错误

### 5b: 模板改造 — 工具栏 + checkbox + 底部操作栏

- [ ] **Step 3: 改造侧边栏工具栏**

找到 `sidebar-toolbar` div（约 line 4-9），将按钮区域改为：

```html
<div style="display:flex;gap:6px">
  <button v-if="!editMode" class="btn btn-sm" @click="enterEditMode">编辑</button>
  <button v-else class="btn btn-sm" @click="exitEditMode">完成</button>
  <template v-if="!editMode">
    <button class="btn btn-sm" @click="showNewGroup = true">+ 分组</button>
    <button class="btn btn-primary btn-sm" @click="showIngest = true">+ 导入</button>
  </template>
</div>
```

- [ ] **Step 4: 在分组 header 加 checkbox**

找到分组 header div（约 line 19-25），在 `<span class="group-chevron">` 前插入：

```html
<div v-if="editMode" class="edit-checkbox"
  :class="{ checked: selectedGroupIds.has(group.id) }"
  @click.stop="toggleGroupSelect(group.id)"></div>
```

- [ ] **Step 5: 在文档行加 checkbox**

找到分组内文档行 div（约 line 28-37），在 `<div class="doc-row-title">` 前插入：

```html
<div v-if="editMode" class="edit-checkbox"
  :class="{ checked: selectedDocIds.has(doc.id) }"
  @click.stop="toggleDocSelect(doc.id)"></div>
```

对未分组文档行（约 line 49-58）做同样处理。

- [ ] **Step 6: 在 sidebar 末尾（`</div>` 关闭 sidebar-list 后、`</aside>` 前）加底部操作栏**

```html
<div v-if="editMode" class="edit-bottom-bar">
  <span class="edit-selection-label">{{ selectionLabel }}</span>
  <button class="btn btn-sm btn-danger" :disabled="!hasSelection || deleting"
    @click="onBatchDelete">删除</button>
</div>
```

- [ ] **Step 7: 构建验证**

```bash
cd web && npm run build 2>&1 | tail -5
```

Expected: 无错误

### 5c: 删除逻辑 + 弹窗 + 样式

- [ ] **Step 8: 在 script setup 中追加删除逻辑**

在 `exitEditMode` 函数后追加（需先在 import 行加入 `deleteBatchDocuments, deleteBatchGroups`）：

```typescript
import { deleteBatchDocuments, deleteBatchGroups } from '../api/documents'

async function onBatchDelete() {
  if (selectedGroupIds.value.size > 0) {
    showDeleteGroupConfirm.value = true
    return
  }
  await doDeleteDocs()
}

async function doDeleteDocs() {
  deleting.value = true
  deleteErr.value = ''
  try {
    await deleteBatchDocuments([...selectedDocIds.value])
    await load()
    exitEditMode()
  } catch (e: any) {
    deleteErr.value = e.message
  } finally {
    deleting.value = false
  }
}

async function doDeleteGroups() {
  deleting.value = true
  deleteErr.value = ''
  try {
    await deleteBatchGroups([...selectedGroupIds.value], deleteGroupWithDocs.value)
    await load()
    exitEditMode()
    showDeleteGroupConfirm.value = false
  } catch (e: any) {
    deleteErr.value = e.message
  } finally {
    deleting.value = false
  }
}
```

注意：`load()` 是现有的数据刷新函数，确认其名称与文件中一致（若不同则替换）。

- [ ] **Step 9: 在模板末尾（导入弹窗后）加删除分组确认弹窗**

```html
<!-- 批量删除分组确认弹窗 -->
<div v-if="showDeleteGroupConfirm" class="modal-overlay" @click.self="showDeleteGroupConfirm = false">
  <div class="modal" style="max-width:360px">
    <h3>删除分组</h3>
    <p style="color:var(--text-muted);margin-bottom:12px">
      将删除 {{ selectedGroupIds.size }} 个分组，请选择组内文档处理方式：
    </p>
    <div class="form-row" style="flex-direction:column;gap:8px">
      <label class="radio-option" :class="{ active: deleteGroupWithDocs }">
        <input type="radio" :value="true" v-model="deleteGroupWithDocs" />
        同时删除组内所有文档
      </label>
      <label class="radio-option" :class="{ active: !deleteGroupWithDocs }">
        <input type="radio" :value="false" v-model="deleteGroupWithDocs" />
        将文档移至未分组
      </label>
    </div>
    <div v-if="deleteErr" class="err" style="margin-top:8px">{{ deleteErr }}</div>
    <div class="modal-footer">
      <button class="btn" @click="showDeleteGroupConfirm = false">取消</button>
      <button class="btn btn-danger" :disabled="deleting" @click="doDeleteGroups">
        {{ deleting ? '删除中…' : '确认删除' }}
      </button>
    </div>
  </div>
</div>
```

- [ ] **Step 10: 在 `<style>` 块末尾追加新样式**

```css
.edit-checkbox {
  width: 14px; height: 14px; border-radius: 3px;
  border: 1.5px solid var(--border); background: var(--bg-sidebar);
  flex-shrink: 0; cursor: pointer;
}
.edit-checkbox.checked {
  background: var(--accent); border-color: var(--accent);
}
.edit-checkbox.checked::after {
  content: '✓'; font-size: 9px; color: #fff;
  display: flex; align-items: center; justify-content: center; height: 100%;
}
.edit-bottom-bar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 12px; border-top: 1px solid var(--border);
  background: var(--bg-sidebar);
}
.edit-selection-label { font-size: 11px; color: var(--text-muted); }
.radio-option { display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 13px; }
.radio-option.active { color: var(--accent); }
```

- [ ] **Step 11: 构建验证**

```bash
cd web && npm run build 2>&1 | tail -10
```

Expected: 无错误

- [ ] **Step 12: Commit**

```bash
git add web/src/views/KnowledgeView.vue web/src/api/documents.ts
git commit -m "feat(frontend): KB batch delete — edit mode, multi-select, batch delete UI"
```

---

## Task 6: 端到端验证

- [ ] **Step 1: 启动测试服务器**

```bash
go build -a -o /tmp/spider-test ./cmd/spider && \
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 2: 打开浏览器验证**

访问 `http://localhost:8002`，进入知识库页面，验证：

1. 侧边栏顶部有"编辑"按钮
2. 点击"编辑"后按钮变"完成"，"+ 分组"和"+ 导入"隐藏
3. 每个文档和分组左侧出现 checkbox
4. 勾选文档后底部出现"已选 N 个文档 / 删除"操作栏
5. 点删除 → 确认 → 文档消失，退出编辑模式
6. 勾选分组后点删除 → 弹出确认弹窗，含两个 radio 选项
7. 选"同时删除组内所有文档"确认 → 分组和文档均消失
8. 选"将文档移至未分组"确认 → 分组消失，文档出现在未分组

- [ ] **Step 3: 全量测试**

```bash
go test ./...
```

Expected: 全部 PASS

- [ ] **Step 4: 最终 commit（如有遗漏文件）**

```bash
git status
# 确认无遗漏，若有则 add + commit
```
