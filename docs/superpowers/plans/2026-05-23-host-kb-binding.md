# Host KB Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement phase 1 of `docs/spec-20260522-1442-host-kb-binding.md`: per-access-face KB binding with explicit `kb_mode`, sentinel removal, bare `@kb` removal, UI support, and agent prompt updates.

**Architecture:** Reuse `access_faces.knowledge_sources` as the persisted scope list and add `access_faces.kb_mode` as the explicit switch. Validation lives in `AccessFaceStore` so API and future callers share semantics; HTTP handlers enrich responses with KB group/doc display fields. Knowledge store delete operations clean stale access-face refs in the same transaction.

**Tech Stack:** Go, SQLite, Vue 3, TypeScript, existing spider.ai store/API patterns.

---

## File Map

- Modify `internal/models/host.go`: add `KBMode`, response enrichment DTO, request fields; remove `Host.KnowledgeSources`.
- Modify `internal/db/schema.go`: add `access_faces.kb_mode`, drop dead `host_knowledge_sources`, migrate sentinel/valid refs.
- Modify `internal/store/access_face_store.go`: persist `kb_mode`, validate final merged KB state, normalize sentinel.
- Create `internal/store/access_face_store_test.go`: test `kb_mode` merge/validation behavior.
- Modify `internal/api/hosts.go`: validate existence of KB refs and enrich host/access-face responses.
- Modify `internal/knowledge/store.go`: add group/doc lookup helpers and clean access-face refs during delete.
- Modify `internal/api/kb_ref.go` and `internal/api/kb_ref_test.go`: require `@kb:` arguments; bare `@kb` is plain text.
- Modify `internal/api/chat.go`: keep SearchDocs registered and inject face KB only when `kb_mode != "none"`.
- Modify `internal/agent/tools_docs.go`, `internal/agent/tools_list_hosts.go`, `internal/agent/tools_api.go`: update prompts for `kb_mode` and enriched `knowledge_sources`.
- Modify `web/src/api/hosts.ts`: add `kb_mode` and enriched knowledge source types.
- Modify `web/src/views/HostsView.vue`: replace sentinel UI with explicit `kb_mode` selector and source multi-select cap.

---

### Task 1: Access Face Model, Migration, and Store Validation

**Files:**
- Modify: `internal/models/host.go`
- Modify: `internal/db/schema.go`
- Modify: `internal/store/access_face_store.go`
- Create: `internal/store/access_face_store_test.go`

- [ ] **Step 1: Write failing store tests**

Add tests that create an in-memory DB via `db.Init`, construct `AccessFaceStore`, and assert:

```go
func TestAccessFaceKBModeValidation(t *testing.T) {
    // Add with default kb_mode -> "none", knowledge_sources -> [].
    // Update kb_mode="specific" with empty sources -> error string
    // "kb_mode=specific requires at least one knowledge_source".
    // Update kb_mode="specific" with 11 sources -> error string
    // "knowledge_sources exceeds limit of 10".
    // Update kb_mode="none" with non-empty sources -> persisted sources [].
}
```

```go
func TestAccessFaceKBModeMergeSemantics(t *testing.T) {
    // Start with kb_mode="specific" and one group source.
    // Update an unrelated field without kb_mode/knowledge_sources.
    // Assert kb_mode and sources are preserved.
    // Update kb_mode="none" only.
    // Assert sources are cleared.
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./internal/store -run 'TestAccessFaceKBMode'`

Expected: compile failure because `KBMode` does not exist yet.

- [ ] **Step 3: Implement model and schema**

Add `KBMode string json:"kb_mode"` to `AccessFace`, `AddAccessFaceRequest`, and `UpdateAccessFaceRequest` (`*string` for update). Remove `Host.KnowledgeSources`.

In schema initialization:

```sql
ALTER TABLE access_faces ADD COLUMN kb_mode TEXT NOT NULL DEFAULT 'none';
DROP TABLE IF EXISTS host_knowledge_sources;
```

Add idempotent Go migration that scans `access_faces(id, kb_mode, knowledge_sources)`, converts sentinel `[{type:"none",id:0}]` to `kb_mode='none'` and `[]`, and upgrades valid group/doc refs to `kb_mode='specific'` when current mode is `none`.

- [ ] **Step 4: Implement store persistence and validation**

Update INSERT/SELECT/UPDATE column lists for `kb_mode`.

Add helpers:

```go
func normalizeKBMode(mode string) string
func normalizeKnowledgeSources(mode string, sources []models.KnowledgeSourceRef) []models.KnowledgeSourceRef
func validateAccessFaceKB(mode string, sources []models.KnowledgeSourceRef) error
```

Validation:
- mode must be `none` or `specific`
- `none` always persists `[]`
- `specific` requires 1 to 10 refs
- ref type must be `group` or `doc`; id must be positive

- [ ] **Step 5: Run green test**

Run: `go test ./internal/store -run 'TestAccessFaceKBMode'`

Expected: PASS.

---

### Task 2: API Response Enrichment and Ref Existence Validation

**Files:**
- Modify: `internal/models/host.go`
- Modify: `internal/api/hosts.go`
- Modify: `internal/knowledge/store.go`
- Add or modify: `internal/api/hosts_test.go`

- [ ] **Step 1: Write failing API tests**

Add tests around `updateAccessFace` or store/API boundary that assert:
- `kb_mode=specific` with non-existent group/doc returns HTTP 400.
- `GET /hosts` and `GET /hosts/:id` include enriched source fields for group (`name`) and doc (`title`, `group_id`, `group_name`).
- `kb_mode=none` responds with `knowledge_sources: []`.

- [ ] **Step 2: Run red test**

Run: `go test ./internal/api -run 'AccessFace|Host.*Knowledge'`

Expected: failure because validation/enrichment does not exist.

- [ ] **Step 3: Add knowledge lookup helpers**

In `internal/knowledge/store.go` add:

```go
func (s *Store) GetGroupsByIDs(ctx context.Context, ids []int) ([]Group, error)
func (s *Store) GetDocumentsByIDs(ctx context.Context, ids []int) ([]Document, error)
```

Use `IN` placeholders and return empty slices for empty input.

- [ ] **Step 4: Validate refs before update/create**

In `internal/api/hosts.go`, before `AddAccessFaceStore.Add` and `Update`, check every specific ref exists:
- group ref must be returned by `GetGroupsByIDs`
- doc ref must be returned by `GetDocumentsByIDs`

Return 400 with the store validation message or a concise missing-ref message.

- [ ] **Step 5: Enrich responses**

Build response DTOs in API layer:
- group ref: `type`, `id`, `name`, optional `description`
- doc ref: `type`, `id`, `title`, `group_id`, `group_name`, optional `description`

Use batch collection for `listHosts`; direct/batch helper is acceptable for `getHost` and face list.

- [ ] **Step 6: Run green test**

Run: `go test ./internal/api -run 'AccessFace|Host.*Knowledge'`

Expected: PASS.

---

### Task 3: Delete Link Cleanup, Chat Gate, and `@kb` Syntax

**Files:**
- Modify: `internal/knowledge/store.go`
- Modify: `internal/api/chat.go`
- Modify: `internal/api/kb_ref.go`
- Modify: `internal/api/kb_ref_test.go`
- Add or modify: `internal/knowledge/store_test.go`

- [ ] **Step 1: Write failing tests**

Update `kb_ref_test.go` so:
- `parseKBRefs("@kb nginx") == nil`
- `expandKBRefs("@kb nginx", ...)` leaves text unchanged and does not call search
- `@kb:组名` and `@kb:组名/文档` still work

Add store tests for:
- deleting a doc removes matching `{type:"doc", id}` from `access_faces.knowledge_sources`
- deleting a group removes matching group and child-doc refs, and downgrades `kb_mode` to `none` when sources become empty

- [ ] **Step 2: Run red tests**

Run: `go test ./internal/api -run KBRefs`

Run: `go test ./internal/knowledge -run 'Delete.*AccessFace|KnowledgeSource'`

Expected: failures with current bare `@kb` and no cleanup.

- [ ] **Step 3: Implement syntax and chat gate**

Change regex to require `@kb:`:

```go
var kbRefRe = regexp.MustCompile(`@kb:([^\s/]+)(?:/([^\s]+))?`)
```

Change `allFacesDisableKB` to return `false` or remove its effect. Change face KB injection condition to `f.KBMode != "none"` wherever present.

- [ ] **Step 4: Implement knowledge delete cleanup**

In `DeleteDocument`, `DeleteGroup`, and `DeleteGroups`, run cleanup in the existing delete transaction:
- load candidate access faces
- unmarshal sources
- filter removed group/doc refs exactly
- if empty and mode is `specific`, set `kb_mode='none'`
- write filtered JSON and mode

- [ ] **Step 5: Run green tests**

Run:

```bash
go test ./internal/api -run KBRefs
go test ./internal/knowledge -run 'Delete.*AccessFace|KnowledgeSource'
```

Expected: PASS.

---

### Task 4: Agent Prompt Updates

**Files:**
- Modify: `internal/agent/tools_docs.go`
- Modify: `internal/agent/tools_list_hosts.go`
- Modify: `internal/agent/tools_api.go`

- [ ] **Step 1: Update prompt text**

Replace sentinel guidance with `kb_mode` semantics:
- `kb_mode='specific'` means bound source signal
- `type=group` maps to `scope_type=group`
- `type=doc` maps to `scope_type=document`
- `kb_mode='none'` means no binding signal

- [ ] **Step 2: Verify prompt references**

Run:

```bash
rg -n "sentinel|type:\\\"none\\\"|knowledge_sources\\[0\\]|kb_mode|Face KB Bindings" internal/agent internal/api
```

Expected: no sentinel guidance remains; new prompt appears in SearchDocs/GetHosts guidance.

---

### Task 5: Frontend Host Face KB Binding UI

**Files:**
- Modify: `web/src/api/hosts.ts`
- Modify: `web/src/views/HostsView.vue`

- [ ] **Step 1: Type API response**

Add:

```ts
type KBMode = 'none' | 'specific'
type KnowledgeSource = {
  type: 'group' | 'doc'
  id: number
  name?: string
  title?: string
  group_id?: number
  group_name?: string
  description?: string
}
```

Set `AccessFace.kb_mode` and request `kb_mode?: KBMode`.

- [ ] **Step 2: Replace sentinel UI**

In `HostsView.vue`:
- render `f.kb_mode === 'none'` as `不使用 KB`
- render `f.kb_mode === 'specific'` selected tags
- remove all `{ type: 'none', id: 0 }` writes
- add selector `不使用 KB` / `指定 KB`
- when mode switches to `none`, submit `{ kb_mode: 'none' }` and clear local sources
- when mode is `specific`, submit `{ kb_mode: 'specific', knowledge_sources: sources }`
- enforce max 10 selected sources in UI
- disable face form submit when specific has no source

- [ ] **Step 3: Run frontend checks**

Run:

```bash
npm --prefix web run build
```

Expected: type check/build succeeds.

---

### Task 6: Full Verification

**Files:**
- All modified files.

- [ ] **Step 1: Run targeted Go tests**

Run:

```bash
go test ./internal/store ./internal/api ./internal/knowledge ./internal/agent
```

Expected: PASS.

- [ ] **Step 2: Run full Go tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Run frontend build**

Run:

```bash
npm --prefix web run build
```

Expected: PASS.

- [ ] **Step 4: Manual spec checklist**

Confirm phase 1 items from `docs/spec-20260522-1442-host-kb-binding.md`:
- `access_face.kb_mode` added
- face mixed group/doc binding supported
- host-level KB field/table removed or ignored/dropped
- sentinel removed
- bare `@kb` removed
- host face UI supports binding
- SearchDocs prompt updated
- stage 2 description work intentionally not included
