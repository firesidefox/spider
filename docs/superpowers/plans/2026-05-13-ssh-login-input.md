# SSH Login Input Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ssh_login_input` field to `AccessFace` so spider can automatically send a one-time input (e.g. `/rsh`) after SSH connection, before executing commands.

**Architecture:** Add `SSHLoginInput string` to the model and DB; call `sendLoginInput()` in `NewClientWithCredential` when the field is non-empty; propagate the field through store, API, and frontend.

**Tech Stack:** Go (`golang.org/x/crypto/ssh`), SQLite (ALTER TABLE migration), Vue 3 + TypeScript

---

### Task 1: Add `SSHLoginInput` to model and request types

**Files:**
- Modify: `internal/models/host.go:34-53` (AccessFace struct)
- Modify: `internal/models/host.go:118-149` (AddAccessFaceRequest, UpdateAccessFaceRequest)

- [ ] **Step 1: Add field to `AccessFace`**

In `internal/models/host.go`, add after `SSHLegacy`:
```go
SSHLoginInput string `json:"ssh_login_input,omitempty"`
```

- [ ] **Step 2: Add field to `AddAccessFaceRequest`**

After `SSHLegacy bool`:
```go
SSHLoginInput string `json:"ssh_login_input"`
```

- [ ] **Step 3: Add field to `UpdateAccessFaceRequest`**

After `SSHLegacy *bool`:
```go
SSHLoginInput *string `json:"ssh_login_input"`
```

- [ ] **Step 4: Build to verify no compile errors**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/models/host.go
git commit -m "feat(model): add SSHLoginInput to AccessFace and request types"
```

---

### Task 2: DB migration — add `ssh_login_input` column

**Files:**
- Modify: `internal/db/schema.go` (migrate function, around line 270)

- [ ] **Step 1: Add ALTER TABLE in `migrate()`**

In `internal/db/schema.go`, after the line:
```go
db.Exec("ALTER TABLE hosts ADD COLUMN notes TEXT NOT NULL DEFAULT ''")
```
add:
```go
db.Exec("ALTER TABLE access_faces ADD COLUMN ssh_login_input TEXT NOT NULL DEFAULT ''")
```

- [ ] **Step 2: Build to verify**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): add ssh_login_input column to access_faces"
```

---

### Task 3: Update store — include `ssh_login_input` in SQL

**Files:**
- Modify: `internal/store/access_face_store.go`

- [ ] **Step 1: Update `accessFaceCols` constant (line 217)**

Change:
```go
const accessFaceCols = `id,host_id,type,ip,port,username,auth_type,` +
	`encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,` +
	`base_url,rest_auth_type,rest_username,header_name,knowledge_sources,created_at,updated_at`
```
To:
```go
const accessFaceCols = `id,host_id,type,ip,port,username,auth_type,` +
	`encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,` +
	`ssh_login_input,` +
	`base_url,rest_auth_type,rest_username,header_name,knowledge_sources,created_at,updated_at`
```

- [ ] **Step 2: Update `scanAccessFace` to scan the new column**

In `scanAccessFace`, after `&sshLegacy,` add `&f.SSHLoginInput,`:
```go
err := s.Scan(
    &f.ID, &f.HostID, &f.Type, &f.IP, &f.Port,
    &f.Username, &f.SSHAuthType,
    &f.EncryptedCred, &f.EncryptedPass,
    &f.SSHKeyID, &sshLegacy,
    &f.SSHLoginInput,
    &f.BaseURL, &f.RESTAuthType, &f.RESTUsername, &f.HeaderName,
    &ksJSON, &f.CreatedAt, &f.UpdatedAt,
)
```

- [ ] **Step 3: Update `Add` INSERT statement**

Change the INSERT in `Add` to include `ssh_login_input`:
```go
_, err = s.db.Exec(`INSERT INTO access_faces
    (id,host_id,type,ip,port,username,auth_type,
     encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,
     ssh_login_input,
     base_url,rest_auth_type,rest_username,header_name,knowledge_sources,created_at,updated_at)
    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
    id, hostID, req.Type, req.IP, req.Port, req.Username, req.SSHAuthType,
    encCred, encPass, req.SSHKeyID, req.SSHLegacy,
    req.SSHLoginInput,
    req.BaseURL, req.RESTAuthType, req.RESTUsername, req.HeaderName, string(ksJSON), now, now)
```

- [ ] **Step 4: Update `Update` method to handle `SSHLoginInput`**

In `Update`, after the `if req.SSHLegacy != nil` block, add:
```go
if req.SSHLoginInput != nil {
    cur.SSHLoginInput = *req.SSHLoginInput
}
```

Then update the UPDATE SQL to include `ssh_login_input=?,`:
```go
_, err = s.db.Exec(`UPDATE access_faces SET
    ip=?,port=?,username=?,auth_type=?,
    encrypted_credential=?,encrypted_passphrase=?,
    ssh_key_id=?,ssh_legacy=?,ssh_login_input=?,
    base_url=?,rest_auth_type=?,rest_username=?,
    header_name=?,knowledge_sources=?,updated_at=?
    WHERE id=?`,
    cur.IP, cur.Port, cur.Username, cur.SSHAuthType,
    encCred, encPass, cur.SSHKeyID, cur.SSHLegacy, cur.SSHLoginInput,
    cur.BaseURL, cur.RESTAuthType, cur.RESTUsername, cur.HeaderName, string(ksJSON), now, id)
```

- [ ] **Step 5: Build to verify**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/store/access_face_store.go
git commit -m "feat(store): include ssh_login_input in access_faces CRUD"
```

---

### Task 4: SSH client — send login input after connect

**Files:**
- Modify: `internal/ssh/client.go`

- [ ] **Step 1: Add `sendLoginInput` function**

Add after `buildAuthMethods` (around line 112):
```go
// sendLoginInput sends a one-time input to handle interactive login menus
// (e.g. shell selectors). Opens a PTY session, writes the input, waits 500ms,
// then closes. No-op if input is empty.
func sendLoginInput(conn *gossh.Client, input string) error {
    if input == "" {
        return nil
    }
    session, err := conn.NewSession()
    if err != nil {
        return fmt.Errorf("sendLoginInput: create session: %w", err)
    }
    defer session.Close()

    modes := gossh.TerminalModes{gossh.ECHO: 0}
    if err := session.RequestPty("xterm", 24, 80, modes); err != nil {
        return fmt.Errorf("sendLoginInput: request pty: %w", err)
    }
    stdin, err := session.StdinPipe()
    if err != nil {
        return fmt.Errorf("sendLoginInput: stdin pipe: %w", err)
    }
    if err := session.Shell(); err != nil {
        return fmt.Errorf("sendLoginInput: start shell: %w", err)
    }
    if _, err := fmt.Fprintln(stdin, input); err != nil {
        return fmt.Errorf("sendLoginInput: write input: %w", err)
    }
    time.Sleep(500 * time.Millisecond)
    return nil
}
```

- [ ] **Step 2: Call `sendLoginInput` in `NewClientWithCredential`**

After `return &Client{conn: conn, face: face}, nil` (line 89), change to:
```go
c := &Client{conn: conn, face: face}
if err := sendLoginInput(conn, face.SSHLoginInput); err != nil {
    conn.Close()
    return nil, err
}
return c, nil
```

- [ ] **Step 3: Add `"io"` import if needed — check existing imports**

`fmt` and `time` are already imported. No new imports needed.

- [ ] **Step 4: Build to verify**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 5: Run existing tests**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ssh/client.go
git commit -m "feat(ssh): send login input after connect for interactive menu hosts"
```

---

### Task 5: Frontend — add `ssh_login_input` field

**Files:**
- Modify: `web/src/api/hosts.ts`
- Modify: `web/src/views/HostsView.vue`

- [ ] **Step 1: Add `ssh_login_input` to `AccessFace` interface in `hosts.ts`**

After `ssh_legacy?: boolean`:
```typescript
ssh_login_input?: string
```

- [ ] **Step 2: Add `ssh_login_input` to `AddAccessFaceRequest` interface**

After `ssh_legacy?: boolean`:
```typescript
ssh_login_input?: string
```

- [ ] **Step 3: Add field to `emptyFaceForm()` in `HostsView.vue` (line 397)**

In `emptyFaceForm`, add `ssh_login_input: ''` after `ssh_legacy: false`:
```typescript
const emptyFaceForm = () => ({
  type: 'ssh' as 'ssh' | 'restapi',
  ip: activeHost.value?.ip ?? '',
  port: 22,
  username: '',
  ssh_auth_type: 'password',
  credential: '',
  passphrase: '',
  ssh_key_id: '',
  ssh_legacy: false,
  ssh_login_input: '',
  base_url: '',
  rest_auth_type: 'none',
  rest_username: '',
  header_name: '',
  knowledge_sources: [] as Array<{type:'group'|'doc';id:number}>
})
```

- [ ] **Step 4: Populate `ssh_login_input` when editing existing face (line 560)**

After `ssh_legacy: face.ssh_legacy || false,` add:
```typescript
ssh_login_input: face.ssh_login_input || '',
```

- [ ] **Step 5: Include `ssh_login_input` in `submitFace` (line 541)**

After `req.ssh_legacy = faceForm.value.ssh_legacy` add:
```typescript
req.ssh_login_input = faceForm.value.ssh_login_input || undefined
```

- [ ] **Step 6: Add form row in template (after 兼容模式 row, line 260)**

After the `</div>` closing the 兼容模式 row:
```html
<div class="form-row">
  <label>登录后输入（可选）</label>
  <input v-model="faceForm.ssh_login_input" class="input" placeholder="/rsh" />
</div>
```

- [ ] **Step 7: Show `ssh_login_input` in face detail view (after 兼容模式 display, line 126)**

After the 兼容模式 display line:
```html
<div v-if="f.type === 'ssh' && f.ssh_login_input" class="face-item"><label>登录后输入</label><div class="value"><code>{{ f.ssh_login_input }}</code></div></div>
```

- [ ] **Step 8: Build frontend**

```bash
cd web && npm run build
```
Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add web/src/api/hosts.ts web/src/views/HostsView.vue
git commit -m "feat(frontend): add ssh_login_input field to access face form"
```

---

### Task 6: Verify end-to-end

- [ ] **Step 1: Build backend**

```bash
go build -a -o /tmp/spider-test ./cmd/spider
```
Expected: no errors.

- [ ] **Step 2: Start test server**

```bash
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 3: Open browser at http://localhost:8002**

Navigate to Hosts → select a host → edit SSH access face → verify "登录后输入（可选）" input field appears.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 5: Commit if any fixes needed, otherwise done**
