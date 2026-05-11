# 知识库 @ 引用 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在运维智能体对话中支持三级 `@kb` 语法，后端解析并展开为知识库内容注入消息。

**Architecture:** 后端 `chatSendMessage` 在调用 `Agent.Run()` 前解析 `@kb` / `@kb:分组名` / `@kb:分组名/文档名称`，前两种走向量检索，第三种按名称精确查文档。前端输入框 `@` 触发两步下拉（分组 → 文档）。

**Tech Stack:** Go (backend), Vue 3 + TypeScript (frontend), SQLite

---

## File Map

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/rag/store.go` | Modify | 新增 `SearchByGroup(ctx, query, groupID *int, topK int)` |
| `internal/rag/store_test.go` | Modify | 新增 group 过滤测试 |
| `internal/store/document.go` | Modify | 新增 `FindByTitle(groupID int, title string)` |
| `internal/store/document_test.go` | Modify | 新增 FindByTitle 测试 |
| `internal/api/kb_ref.go` | Create | `parseKBRefs` + `expandKBRefs` 纯函数 |
| `internal/api/kb_ref_test.go` | Create | 解析和展开逻辑单元测试 |
| `internal/api/chat.go` | Modify | `chatSendMessage` 调用 `expandKBRefs` |
| `internal/mcp/server.go` | Modify | `App` 加 `RagStore *rag.Store` 字段 |
| `cmd/spider/main.go` | Modify | 初始化 `RagStore` 并注入 `App` |
| 前端对话输入组件 | Modify | `@` 触发两步下拉（分组 → 文档） |

---

## Task 1: `rag.Store.SearchByGroup`

**Files:**
- Modify: `internal/rag/store.go`
- Modify: `internal/rag/store_test.go`

- [ ] **Step 1: 写失败测试**

在 `internal/rag/store_test.go` 末尾加：

```go
func TestSearchByGroup(t *testing.T) {
    db := setupTestDB(t)
    embedder := &fakeEmbedder{}
    s := NewStore(db, setupDocStore(t, db), embedder)
    ctx := context.Background()

    gid1 := 1
    _ = s.Ingest(ctx, "v", nil, "doc in group1", "nginx restart", "f.txt", 0, &gid1)
    gid2 := 2
    _ = s.Ingest(ctx, "v", nil, "doc in group2", "apache restart", "f.txt", 0, &gid2)

    results, err := s.SearchByGroup(ctx, "restart", &gid1, 5)
    if err != nil {
        t.Fatal(err)
    }
    if len(results) != 1 {
        t.Fatalf("expected 1 result, got %d", len(results))
    }
    if results[0].Title != "doc in group1" {
        t.Errorf("unexpected title: %s", results[0].Title)
    }

    all, err := s.SearchByGroup(ctx, "restart", nil, 5)
    if err != nil {
        t.Fatal(err)
    }
    if len(all) != 2 {
        t.Fatalf("expected 2 results, got %d", len(all))
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/rag/... -run TestSearchByGroup -v
```

期望：`FAIL` — `s.SearchByGroup undefined`

- [ ] **Step 3: 实现 `SearchByGroup`**

在 `internal/rag/store.go` 的 `Search` 函数后添加：

```go
func (s *Store) SearchByGroup(ctx context.Context, query string, groupID *int, topK int) ([]*models.Document, error) {
    qvec, err := s.embedder.Embed(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("embed query: %w", err)
    }

    var (
        rows *sql.Rows
        qErr error
    )
    if groupID != nil {
        rows, qErr = s.db.QueryContext(ctx,
            "SELECT id, vendor, tags, title, content, embedding, source_file, chunk_index, created_at, group_id FROM documents WHERE group_id = ? AND embedding IS NOT NULL",
            *groupID,
        )
    } else {
        rows, qErr = s.db.QueryContext(ctx,
            "SELECT id, vendor, tags, title, content, embedding, source_file, chunk_index, created_at, group_id FROM documents WHERE embedding IS NOT NULL",
        )
    }
    if qErr != nil {
        return nil, fmt.Errorf("query documents: %w", qErr)
    }
    defer rows.Close()

    type scored struct {
        doc   *models.Document
        score float32
    }
    var candidates []scored

    for rows.Next() {
        var d models.Document
        var tagsJSON string
        if err := rows.Scan(&d.ID, &d.Vendor, &tagsJSON, &d.Title, &d.Content, &d.Embedding, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt, &d.GroupID); err != nil {
            return nil, fmt.Errorf("scan: %w", err)
        }
        if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
            d.Tags = []string{}
        }
        dvec := deserializeVec(d.Embedding)
        if len(dvec) == 0 {
            continue
        }
        candidates = append(candidates, scored{doc: &d, score: cosineSimilarity(qvec, dvec)})
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }

    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].score > candidates[j].score
    })
    if topK > 0 && len(candidates) > topK {
        candidates = candidates[:topK]
    }
    out := make([]*models.Document, len(candidates))
    for i, c := range candidates {
        out[i] = c.doc
    }
    return out, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/rag/... -run TestSearchByGroup -v
```

期望：`PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/rag/store.go internal/rag/store_test.go
git commit -m "feat(rag): add SearchByGroup with optional group_id filter"
```

---

## Task 2: `store.DocumentStore.FindByTitle`

**Files:**
- Modify: `internal/store/document.go`
- Modify: `internal/store/document_test.go`

- [ ] **Step 1: 写失败测试**

在 `internal/store/document_test.go` 末尾加：

```go
func TestFindByTitle(t *testing.T) {
    db := setupTestDB(t)
    s := NewDocumentStore(db)

    gid := 1
    err := s.Save("v", nil, "nginx配置说明", "worker_processes auto;", nil, "f.txt", 0, &gid)
    if err != nil {
        t.Fatal(err)
    }

    doc, err := s.FindByTitle(gid, "nginx配置说明")
    if err != nil {
        t.Fatal(err)
    }
    if doc == nil {
        t.Fatal("expected doc, got nil")
    }
    if doc.Title != "nginx配置说明" {
        t.Errorf("unexpected title: %s", doc.Title)
    }

    missing, err := s.FindByTitle(gid, "不存在的文档")
    if err != nil {
        t.Fatal(err)
    }
    if missing != nil {
        t.Error("expected nil for missing doc")
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/... -run TestFindByTitle -v
```

期望：`FAIL` — `s.FindByTitle undefined`

- [ ] **Step 3: 实现 `FindByTitle`**

在 `internal/store/document.go` 末尾加：

```go
func (s *DocumentStore) FindByTitle(groupID int, title string) (*models.Document, error) {
    row := s.db.QueryRow(
        "SELECT id, vendor, tags, title, content, embedding, source_file, chunk_index, created_at, group_id FROM documents WHERE group_id = ? AND title = ? LIMIT 1",
        groupID, title,
    )
    var d models.Document
    var tagsJSON string
    err := row.Scan(&d.ID, &d.Vendor, &tagsJSON, &d.Title, &d.Content, &d.Embedding, &d.SourceFile, &d.ChunkIndex, &d.CreatedAt, &d.GroupID)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("FindByTitle: %w", err)
    }
    if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
        d.Tags = []string{}
    }
    return &d, nil
}
```

确保 import 有 `"database/sql"` 和 `"encoding/json"`。

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/store/... -run TestFindByTitle -v
```

期望：`PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/store/document.go internal/store/document_test.go
git commit -m "feat(store): add DocumentStore.FindByTitle for exact doc lookup"
```

---

## Task 3: `kb_ref.go` — 解析和展开三级 `@kb` 引用

**Files:**
- Create: `internal/api/kb_ref.go`
- Create: `internal/api/kb_ref_test.go`

- [ ] **Step 1: 写失败测试**

新建 `internal/api/kb_ref_test.go`：

```go
package api

import (
    "strings"
    "testing"
)

func TestParseKBRefs(t *testing.T) {
    tests := []struct {
        input string
        want  []kbRef
    }{
        {
            "@kb nginx重启",
            []kbRef{{raw: "@kb", displayName: "知识库", groupName: "", docTitle: ""}},
        },
        {
            "@kb:运维手册 nginx重启",
            []kbRef{{raw: "@kb:运维手册", displayName: "运维手册", groupName: "运维手册", docTitle: ""}},
        },
        {
            "@kb:运维手册/nginx配置说明 怎么限速",
            []kbRef{{raw: "@kb:运维手册/nginx配置说明", displayName: "运维手册/nginx配置说明", groupName: "运维手册", docTitle: "nginx配置说明"}},
        },
        {
            "@kb:运维手册 和 @kb:网络配置/BGP路由表",
            []kbRef{
                {raw: "@kb:运维手册", displayName: "运维手册", groupName: "运维手册", docTitle: ""},
                {raw: "@kb:网络配置/BGP路由表", displayName: "网络配置/BGP路由表", groupName: "网络配置", docTitle: "BGP路由表"},
            },
        },
        {"没有引用", nil},
    }
    for _, tt := range tests {
        got := parseKBRefs(tt.input)
        if len(got) != len(tt.want) {
            t.Errorf("input=%q: got %d refs, want %d", tt.input, len(got), len(tt.want))
            continue
        }
        for i, w := range tt.want {
            g := got[i]
            if g.raw != w.raw || g.displayName != w.displayName || g.groupName != w.groupName || g.docTitle != w.docTitle {
                t.Errorf("input=%q ref[%d]: got %+v, want %+v", tt.input, i, g, w)
            }
        }
    }
}

func TestFormatKBBlock(t *testing.T) {
    block := formatKBBlock("运维手册", []string{"nginx重启方法\nsystemctl restart nginx"})
    if !strings.Contains(block, "[知识库: 运维手册") {
        t.Errorf("block missing header, got: %s", block)
    }
}

func TestStripKBRefs(t *testing.T) {
    got := stripKBRefs("@kb:运维手册/nginx配置说明 怎么限速")
    if got != "怎么限速" {
        t.Errorf("unexpected: %q", got)
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/api/... -run "TestParseKBRefs|TestFormatKBBlock|TestStripKBRefs" -v
```

期望：`FAIL` — `parseKBRefs undefined`

- [ ] **Step 3: 实现 `kb_ref.go`（第一部分）**

新建 `internal/api/kb_ref.go`：

```go
package api

import (
    "fmt"
    "regexp"
    "strings"

    "github.com/spiderai/spider/internal/models"
)

// @kb                    — 全局向量检索
// @kb:分组名              — 分组向量检索
// @kb:分组名/文档名称      — 精确文档引用
var kbRefRe = regexp.MustCompile(`@kb(?::([^\s/]+)(?:/([^\s]+))?)?`)

type kbRef struct {
    raw         string
    displayName string
    groupName   string
    docTitle    string
}

func parseKBRefs(message string) []kbRef {
    matches := kbRefRe.FindAllStringSubmatch(message, -1)
    if len(matches) == 0 {
        return nil
    }
    seen := make(map[string]bool)
    var refs []kbRef
    for _, m := range matches {
        raw := m[0]
        if seen[raw] {
            continue
        }
        seen[raw] = true
        groupName := m[1]
        docTitle := m[2]
        displayName := "知识库"
        if groupName != "" && docTitle != "" {
            displayName = groupName + "/" + docTitle
        } else if groupName != "" {
            displayName = groupName
        }
        refs = append(refs, kbRef{raw: raw, displayName: displayName, groupName: groupName, docTitle: docTitle})
    }
    return refs
}
```

- [ ] **Step 4: 实现第二部分（format + expand + strip）**

追加到 `internal/api/kb_ref.go`：

```go
func formatKBBlock(displayName string, contents []string) string {
    if len(contents) == 0 {
        return fmt.Sprintf("[知识库: %s · 0条结果]\n", displayName)
    }
    var sb strings.Builder
    fmt.Fprintf(&sb, "[知识库: %s · %d条结果]\n---\n", displayName, len(contents))
    for _, c := range contents {
        sb.WriteString(c)
        sb.WriteString("\n\n")
    }
    sb.WriteString("---\n")
    return sb.String()
}

func stripKBRefs(message string) string {
    return strings.TrimSpace(kbRefRe.ReplaceAllString(message, ""))
}

func expandKBRefs(
    message string,
    groupLookup func(name string) *int,
    docLookup func(groupID int, title string) *models.Document,
    search func(query string, groupID *int) []*models.Document,
) string {
    refs := parseKBRefs(message)
    if len(refs) == 0 {
        return message
    }
    query := stripKBRefs(message)
    for _, ref := range refs {
        var contents []string
        if ref.docTitle != "" {
            // 精确文档引用
            groupID := groupLookup(ref.groupName)
            if groupID != nil {
                if doc := docLookup(*groupID, ref.docTitle); doc != nil {
                    contents = []string{doc.Title + "\n" + doc.Content}
                }
            }
        } else {
            // 向量检索
            groupID := groupLookup(ref.groupName)
            docs := search(query, groupID)
            for _, d := range docs {
                contents = append(contents, d.Title+"\n"+d.Content)
            }
        }
        block := formatKBBlock(ref.displayName, contents)
        message = strings.Replace(message, ref.raw, block, 1)
    }
    return message
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./internal/api/... -run "TestParseKBRefs|TestFormatKBBlock|TestStripKBRefs" -v
```

期望：`PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/api/kb_ref.go internal/api/kb_ref_test.go
git commit -m "feat(api): add kb_ref parser supporting @kb/@kb:group/@kb:group/doc"
```

---

## Task 4: 注入 `RagStore` 并在 `chatSendMessage` 展开

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `cmd/spider/main.go`
- Modify: `internal/api/chat.go`

- [ ] **Step 1: `App` struct 加 `RagStore` 字段**

在 `internal/mcp/server.go` 的 `App` struct 中，`GroupStore` 字段后加：

```go
RagStore *rag.Store
```

在 import 中加 `"github.com/spiderai/spider/internal/rag"`。

- [ ] **Step 2: `main.go` 初始化 `RagStore`**

在 `main.go` 中初始化 `App` 的位置，`GroupStore` 初始化后加（embedder 从 active provider 取）：

```go
ragStore := rag.NewStore(db, docStore, embedder)
app.RagStore = ragStore
```

若 main.go 中 embedder 尚未初始化，先查 active provider：

```go
embedder, _ := rag.NewEmbedderFromProvider(providerStore)
```

（`NewEmbedderFromProvider` 在 Task 4 Step 3 中确认是否已有，若无则直接传 nil，`RagStore` 为 nil 时 chat handler 跳过展开）

- [ ] **Step 3: `chatSendMessage` 展开 `@kb` 引用**

在 `internal/api/chat.go` 的 `chatSendMessage` 函数中，`req.Content` 解析后、`a.Run()` 调用前插入：

```go
content := req.Content
if app.RagStore != nil {
    groupLookup := func(name string) *int {
        if name == "" {
            return nil
        }
        groups, _ := app.GroupStore.List()
        for _, g := range groups {
            if g.Name == name {
                id := g.ID
                return &id
            }
        }
        return nil
    }
    docLookup := func(groupID int, title string) *models.Document {
        doc, _ := app.DocStore.FindByTitle(groupID, title)
        return doc
    }
    search := func(query string, groupID *int) []*models.Document {
        docs, _ := app.RagStore.SearchByGroup(r.Context(), query, groupID, 3)
        return docs
    }
    content = expandKBRefs(content, groupLookup, docLookup, search)
}

events, err := a.Run(r.Context(), id, content, waiter)
```

在 import 中加 `"github.com/spiderai/spider/internal/models"`（若未有）。

- [ ] **Step 4: 编译确认**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./...
```

期望：无错误

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/server.go cmd/spider/main.go internal/api/chat.go
git commit -m "feat(chat): expand @kb/@kb:group/@kb:group/doc before agent run"
```

---

## Task 5: 前端两步下拉

**Files:**
- Modify: 对话输入组件（运行 Step 1 确认路径）

- [ ] **Step 1: 确认对话输入组件路径**

```bash
grep -r "textarea\|sendMessage\|v-model" /Users/cw/fty.ai/spider.ai/web/src --include="*.vue" -l
```

- [ ] **Step 2: 加 `listDocumentsByGroup` API 调用**

在 `web/src/api/documents.ts`（或现有文档 API 文件）中加：

```typescript
export async function listDocumentsByGroup(groupId: number): Promise<Document[]> {
  const res = await fetch(`/api/v1/documents?group_id=${groupId}`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to fetch documents')
  return res.json()
}
```

- [ ] **Step 3: 在输入组件加两步下拉逻辑**

在对话输入组件 `<script setup>` 中加：

```typescript
import { ref } from 'vue'
import { listGroups, listDocumentsByGroup } from '@/api/documents'
import type { DocumentGroup, Document } from '@/api/documents'

type PickerStep = 'group' | 'doc'

const showPicker = ref(false)
const pickerStep = ref<PickerStep>('group')
const groups = ref<DocumentGroup[]>([])
const docs = ref<Document[]>([])
const selectedGroup = ref<DocumentGroup | null>(null)
const atTriggerPos = ref(-1)

async function onInputKeyup(e: KeyboardEvent) {
  const input = e.target as HTMLTextAreaElement
  const val = input.value
  const pos = input.selectionStart ?? val.length
  if (val[pos - 1] === '@') {
    atTriggerPos.value = pos - 1
    groups.value = await listGroups()
    pickerStep.value = 'group'
    showPicker.value = true
  } else if (showPicker.value && val[pos - 1] === ' ') {
    showPicker.value = false
  }
}

async function selectGroup(group: DocumentGroup | null) {
  if (group === null) {
    insertRef('@kb ')
    showPicker.value = false
    return
  }
  selectedGroup.value = group
  docs.value = await listDocumentsByGroup(group.id)
  pickerStep.value = 'doc'
}

function selectDoc(doc: Document | null) {
  const g = selectedGroup.value
  if (!g) return
  const ref = doc ? `@kb:${g.name}/${doc.title} ` : `@kb:${g.name} `
  insertRef(ref)
  showPicker.value = false
}

function insertRef(text: string) {
  const textarea = document.querySelector('textarea') as HTMLTextAreaElement
  if (!textarea) return
  const val = textarea.value
  textarea.value = val.slice(0, atTriggerPos.value) + text + val.slice(atTriggerPos.value + 1)
  textarea.dispatchEvent(new Event('input'))
}
```

- [ ] **Step 4: 在模板中加两步下拉**

在 `<textarea>` 上加 `@keyup="onInputKeyup"`，后面加：

```html
<div v-if="showPicker" class="kb-picker">
  <!-- 第一步：选分组 -->
  <template v-if="pickerStep === 'group'">
    <div class="kb-picker-item" @click="selectGroup(null)">
      @kb — 全局检索
    </div>
    <div
      v-for="g in groups"
      :key="g.id"
      class="kb-picker-item"
      @click="selectGroup(g)"
    >
      {{ g.name }} ›
    </div>
  </template>
  <!-- 第二步：选文档 -->
  <template v-else>
    <div class="kb-picker-item kb-picker-back" @click="pickerStep = 'group'">
      ‹ 返回
    </div>
    <div class="kb-picker-item" @click="selectDoc(null)">
      全部文档（向量检索）
    </div>
    <div
      v-for="d in docs"
      :key="d.id"
      class="kb-picker-item"
      @click="selectDoc(d)"
    >
      {{ d.title }}
    </div>
  </template>
</div>
```

- [ ] **Step 5: 加样式**

```css
.kb-picker {
  position: absolute;
  background: var(--color-bg-elevated, #fff);
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  z-index: 100;
  min-width: 200px;
  max-height: 240px;
  overflow-y: auto;
}
.kb-picker-item {
  padding: 8px 12px;
  cursor: pointer;
  font-size: 14px;
}
.kb-picker-item:hover {
  background: var(--color-bg-hover, #f3f4f6);
}
.kb-picker-back {
  color: var(--color-text-secondary, #6b7280);
  border-bottom: 1px solid var(--color-border, #e5e7eb);
}
```

- [ ] **Step 6: 启动前端确认**

```bash
cd /Users/cw/fty.ai/spider.ai/web && npm run dev
```

输入 `@` → 分组列表出现 → 选分组 → 文档列表出现 → 选文档 → 插入 `@kb:分组名/文档名称`。

- [ ] **Step 7: Commit**

```bash
git add web/src/
git commit -m "feat(ui): two-step @kb picker — group then doc"
```

---

## Task 6: 端到端验证

- [ ] **Step 1: 运行全部后端测试**

```bash
cd /Users/cw/fty.ai/spider.ai && go test ./...
```

期望：全部 PASS

- [ ] **Step 2: 手动测试 `@kb` 全局检索**

输入 `@kb nginx怎么重启`，确认 LLM 回答引用了知识库内容。

- [ ] **Step 3: 手动测试 `@kb:分组名` 分组检索**

输入 `@kb:运维手册 nginx怎么重启`，确认只检索该分组文档。

- [ ] **Step 4: 手动测试 `@kb:分组名/文档名称` 精确引用**

输入 `@kb:运维手册/nginx配置说明 怎么限速`，确认 LLM 回答基于该文档内容。

- [ ] **Step 5: 手动测试无 `@` 时 LLM 自主检索**

输入 `nginx怎么重启`（不带 `@`），确认 LLM 仍可自主调用 `search_docs` tool。

- [ ] **Step 6: 最终 Commit**

```bash
git add .
git commit -m "feat: knowledge base @kb reference — end-to-end complete"
```
