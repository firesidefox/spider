# SSH Login Input — Design Spec

**Date:** 2026-05-13  
**Feature:** SSH Login Input (`ssh_login_input`)  
**Status:** Approved

## Problem

Some SSH hosts present an interactive menu immediately after login (e.g., UOS Server's `/bash` / `/rsh` / `q` selector). The current `Execute()` implementation uses `session.Run(command)` which assumes a clean shell is available immediately after connection. These hosts require a one-time input to be sent before any command can run.

## Solution

Add a `SSHLoginInput` field to `AccessFace`. When non-empty, send this string (plus `\n`) once after the SSH connection is established, before any command execution.

## Data Model

Add to `AccessFace`:

```go
SSHLoginInput string `json:"ssh_login_input,omitempty"`
```

- Empty string = no login input needed (default, backward-compatible)
- Non-empty = send this string + `\n` once after connection

Database: add column `ssh_login_input TEXT NOT NULL DEFAULT ''` to `access_faces` table via schema migration.

## Execution Flow

In `NewClientWithCredential` (or immediately after connection), if `face.SSHLoginInput != ""`:

1. Open a temporary SSH session
2. Request a PTY (pseudo-terminal) so the server's menu prompt is triggered
3. Write `SSHLoginInput + "\n"` to stdin
4. Wait ~500ms for the server to process the selection
5. Close the session

Subsequent `Execute()` calls use normal `session.Run(command)` — no change needed there.

### Why PTY for the init session?

Some restricted shells only present the menu when a TTY is detected. Requesting a PTY in the init session ensures the menu appears and the input is accepted.

## Connection Pool

`pool.go` caches `*Client` objects. The login input is sent once when a new `Client` is created. Reused connections skip this step — no change needed in pool logic.

## Backend Changes

| File | Change |
|------|--------|
| `internal/models/host.go` | Add `SSHLoginInput string` to `AccessFace` |
| `internal/db/schema.go` | Add `ssh_login_input` column, migration |
| `internal/ssh/client.go` | Add `sendLoginInput()` called in `NewClientWithCredential` |
| `internal/store/access_face_store.go` | Include new field in INSERT/UPDATE/SELECT |
| `internal/api/hosts.go` | Accept `ssh_login_input` in create/update handlers |

## Frontend Changes

In the SSH access face configuration panel, add an optional text input:

- Label: `登录后输入（可选）`
- Placeholder: `/rsh`
- Help text: `SSH 连接建立后自动发送，用于处理登录菜单`
- Bound to `ssh_login_input` field

## Error Handling

- If `sendLoginInput()` fails (session error), surface as connection error — do not silently ignore
- Timeout for the init wait is fixed at 500ms; not configurable (sufficient for menu selection)

## Testing

- Unit: `sendLoginInput()` with mock SSH server that presents a menu
- Integration: connect to a test host with login menu, verify commands execute correctly
- Backward compat: hosts with empty `ssh_login_input` behave identically to current behavior
