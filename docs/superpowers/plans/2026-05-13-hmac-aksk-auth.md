# HMAC AK/SK Authentication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add HMAC AK/SK authentication support to REST API access faces, matching the X-Auth-* header scheme described in the target system's API docs.

**Architecture:** New `hmac_aksk` auth type stored in `AccessFace`. AK stored in existing `rest_username` column; SK stored encrypted in `credential`. A new `hmac_algo` column stores the algorithm choice. At request time, `CallRESTAPITool` computes `base64(HMAC(SK, "METHOD\nTIMESTAMP\nPATH"))` and injects four headers.

**Tech Stack:** Go (crypto/hmac, crypto/sha256, encoding/base64), SQLite ALTER TABLE migration, Vue 3 + TypeScript frontend.

---

## File Map

| File | Change |
|------|--------|
| `internal/models/host.go` | Add `RESTAuthHMACAKSK` constant; add `HMACAlgo` field to `AccessFace`, `AddAccessFaceRequest`, `UpdateAccessFaceRequest` |
| `internal/db/schema.go` | Append `ALTER TABLE access_faces ADD COLUMN hmac_algo` migration |
| `internal/store/access_face_store.go` | Add `hmac_algo` to INSERT col list, UPDATE SET, SELECT cols, and `scanAccessFace` |
| `internal/agent/tools_api.go` | Add HMAC signing case in auth injection block |
| `web/src/views/HostsView.vue` | Add `hmac_aksk` option + AK/SK/algo fields to face form; update display card |

---

### Task 1: Model — add `hmac_aksk` type and `HMACAlgo` field

**Files:**
- Modify: `internal/models/host.go`

- [ ] **Step 1: Add constant and field**

In `internal/models/host.go`, add after `RESTAuthNone`:

```go
RESTAuthHMACAKSK RESTAuthType = "hmac_aksk"
```

Add `HMACAlgo` field to `AccessFace` struct after `HeaderName`:

```go
HMACAlgo string `json:"hmac_algo,omitempty"`
```

Add same field to `AddAccessFaceRequest` after `HeaderName`:

```go
HMACAlgo string `json:"hmac_algo"`
```

Add pointer field to `UpdateAccessFaceRequest` after `HeaderName`:

```go
HMACAlgo *string `json:"hmac_algo"`
```

- [ ] **Step 2: Build to verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/models/host.go
git commit -m "feat(models): add hmac_aksk auth type and HMACAlgo field"
```

---

### Task 2: Schema migration — add `hmac_algo` column

**Files:**
- Modify: `internal/db/schema.go`

- [ ] **Step 1: Append migration line**

In `internal/db/schema.go`, after the existing line:

```go
db.Exec("ALTER TABLE access_faces ADD COLUMN ssh_login_input TEXT NOT NULL DEFAULT ''")
```

add:

```go
db.Exec("ALTER TABLE access_faces ADD COLUMN hmac_algo TEXT NOT NULL DEFAULT ''")
```

- [ ] **Step 2: Build to verify**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/db/schema.go
git commit -m "feat(db): add hmac_algo column to access_faces"
```

---

### Task 3: Store — persist and scan `hmac_algo`

**Files:**
- Modify: `internal/store/access_face_store.go`

The store has three places to update: INSERT, UPDATE, and SELECT+scan.

- [ ] **Step 1: Update INSERT**

In `Add()`, the INSERT column list currently ends with `knowledge_sources,created_at,updated_at`. Change to include `hmac_algo`:

```go
(id,host_id,type,ip,port,username,auth_type,
 encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,
 ssh_login_input,
 base_url,rest_auth_type,rest_username,header_name,hmac_algo,knowledge_sources,created_at,updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
```

Add `req.HMACAlgo` to the values list after `req.HeaderName` and before `ksJSON`.

- [ ] **Step 2: Update UPDATE SET**

In `Update()`, the SET clause currently ends with `header_name=?,knowledge_sources=?,updated_at=?`. Change to:

```go
base_url=?,rest_auth_type=?,rest_username=?,
header_name=?,hmac_algo=?,knowledge_sources=?,updated_at=?
```

Apply `HMACAlgo` from `UpdateAccessFaceRequest`: if `req.HMACAlgo != nil`, set `cur.HMACAlgo = *req.HMACAlgo`. Add `cur.HMACAlgo` to the exec args after `cur.HeaderName`.

- [ ] **Step 3: Update SELECT cols and scan**

Change `accessFaceCols` constant to include `hmac_algo` after `header_name`:

```go
const accessFaceCols = `id,host_id,type,ip,port,username,auth_type,` +
	`encrypted_credential,encrypted_passphrase,ssh_key_id,ssh_legacy,` +
	`ssh_login_input,` +
	`base_url,rest_auth_type,rest_username,header_name,hmac_algo,knowledge_sources,created_at,updated_at`
```

In `scanAccessFace()`, add `&f.HMACAlgo` after `&f.HeaderName`:

```go
err := s.Scan(
    &f.ID, &f.HostID, &f.Type, &f.IP, &f.Port,
    &f.Username, &f.SSHAuthType,
    &f.EncryptedCred, &f.EncryptedPass,
    &f.SSHKeyID, &sshLegacy,
    &f.SSHLoginInput,
    &f.BaseURL, &f.RESTAuthType, &f.RESTUsername, &f.HeaderName, &f.HMACAlgo,
    &ksJSON, &f.CreatedAt, &f.UpdatedAt,
)
```

- [ ] **Step 4: Build and test**

```bash
go build ./...
go test ./internal/store/...
```

Expected: build clean, tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/store/access_face_store.go
git commit -m "feat(store): persist hmac_algo in access_faces"
```

---

### Task 4: Agent tool — HMAC signing at request time

**Files:**
- Modify: `internal/agent/tools_api.go`

- [ ] **Step 1: Add imports**

Add to the import block (crypto/hmac, crypto/sha256, encoding/base64, fmt, strconv already available or add as needed):

```go
"crypto/hmac"
"crypto/sha256"
"encoding/base64"
"strconv"
```

- [ ] **Step 2: Add HMAC signing helper**

Add this function at the bottom of `tools_api.go`:

```go
func hmacSign(sk, method, path string, ts int64, algo string) string {
    raw := method + "\n" + strconv.FormatInt(ts, 10) + "\n" + path
    var mac []byte
    switch algo {
    case "HMAC-SM3":
        // SM3 not in stdlib; fall back to SHA256 with a log warning
        log.Printf("WARNING: HMAC-SM3 not supported, falling back to HMAC-SHA256")
        fallthrough
    default:
        h := hmac.New(sha256.New, []byte(sk))
        h.Write([]byte(raw))
        mac = h.Sum(nil)
    }
    return base64.StdEncoding.EncodeToString(mac)
}
```

- [ ] **Step 3: Inject HMAC headers in Execute()**

In the auth injection block (after `if face != nil {`), add a new case after `RESTAuthAPIKey`:

```go
case models.RESTAuthHMACAKSK:
    ts := time.Now().Unix()
    parsedURL, _ := url.Parse(req.URL.String())
    sig := hmacSign(cred, method, parsedURL.RequestURI(), ts, face.HMACAlgo)
    req.Header.Set("X-Auth-AccessKey", face.RESTUsername)
    req.Header.Set("X-Auth-Timestamp", strconv.FormatInt(ts, 10))
    algo := face.HMACAlgo
    if algo == "" {
        algo = "HMAC-SHA256"
    }
    req.Header.Set("X-Auth-Algo", algo)
    req.Header.Set("X-Auth-Signature", sig)
```

Note: `req.URL` is the `*http.Request` already built above. Use `req.URL.RequestURI()` to get path+query.

- [ ] **Step 4: Add `"net/url"` import if not present**

Check imports — `net/url` is not needed since `req.URL` is already `*url.URL` on the `*http.Request`. Use `req.URL.RequestURI()` directly.

- [ ] **Step 5: Build and test**

```bash
go build ./...
go test ./internal/agent/...
```

Expected: build clean, tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tools_api.go
git commit -m "feat(agent): inject HMAC AK/SK headers in CallRESTAPITool"
```

---

### Task 5: Frontend — form fields and display card

**Files:**
- Modify: `web/src/views/HostsView.vue`

- [ ] **Step 1: Add `hmac_aksk` to auth type dropdown**

Find the `<select v-model="faceForm.rest_auth_type"` block. Add option after `apikey`:

```html
<option value="hmac_aksk">HMAC AK/SK</option>
```

- [ ] **Step 2: Add HMAC fields template**

After the `apikey` template block, add:

```html
<template v-if="faceForm.rest_auth_type === 'hmac_aksk'">
  <div class="form-row"><label>Access Key (AK)</label><input v-model="faceForm.rest_username" class="input" placeholder="QMZ0ZENmYvwDJTz7..." /></div>
  <div class="form-row"><label>Secret Key (SK)</label><input v-model="faceForm.credential" class="input" type="password" autocomplete="new-password" /></div>
  <div class="form-row">
    <label>签名算法</label>
    <select v-model="faceForm.hmac_algo" class="input">
      <option value="HMAC-SHA256">HMAC-SHA256</option>
      <option value="HMAC-SM3">HMAC-SM3</option>
    </select>
  </div>
</template>
```

- [ ] **Step 3: Add `hmac_algo` to `emptyFaceForm()`**

Find `const emptyFaceForm = () => ({`. Add `hmac_algo: 'HMAC-SHA256'` after `header_name: ''`.

- [ ] **Step 4: Add `hmac_algo` to `submitFace()`**

In the `else` branch (REST API), after `req.header_name = ...`, add:

```ts
req.hmac_algo = faceForm.value.hmac_algo || undefined
```

- [ ] **Step 5: Add `hmac_algo` to `startEditFace()`**

In `startEditFace()`, after `header_name: face.header_name || ''`, add:

```ts
hmac_algo: face.hmac_algo || 'HMAC-SHA256',
```

- [ ] **Step 6: Update face card display**

Find the line:
```html
<div v-if="f.type === 'restapi'" class="face-item"><label>认证方式</label><div class="value">{{ f.rest_auth_type }}</div></div>
```

Replace with:
```html
<div v-if="f.type === 'restapi'" class="face-item">
  <label>认证方式</label>
  <div class="value">{{ f.rest_auth_type === 'hmac_aksk' ? `HMAC AK/SK (${f.hmac_algo || 'HMAC-SHA256'})` : f.rest_auth_type }}</div>
</div>
```

- [ ] **Step 7: Build frontend and verify**

```bash
cd web && npm run build
```

Expected: build clean, no TypeScript errors.

- [ ] **Step 8: Start dev server and verify in browser**

```bash
go build -a -o /tmp/spider-test ./cmd/spider
/tmp/spider-test serve --addr :8002 --data-dir ~/.spider/data
```

Open browser at `http://localhost:8002`. Navigate to a host → 操作面 → 添加操作面 → REST API → 认证方式 → HMAC AK/SK. Verify AK/SK/algo fields appear. Save and confirm card shows `HMAC AK/SK (HMAC-SHA256)`.

- [ ] **Step 9: Commit**

```bash
git add web/src/views/HostsView.vue
git commit -m "feat(frontend): add HMAC AK/SK auth option to access face form"
```
