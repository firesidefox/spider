# System Prompt Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update `BuildSystemPrompt` in `internal/agent/factory.go` — fix `<exemple>` typos, remove `<reasoning>` blocks, trim redundant examples, and add a new `complexTaskPrompt` section.

**Architecture:** Three static string constants (`toolBehaviorPrompt`, `todoTaskPrompt`, `complexTaskPrompt`) concatenated in `BuildSystemPrompt`. Only the constants change — no structural refactor.

**Tech Stack:** Go

---

### Task 1: Replace `toolBehaviorPrompt`

**Files:**
- Modify: `internal/agent/factory.go:118-226`

- [ ] **Step 1: Replace the constant**

Replace lines 118–226 with:

```go
const toolBehaviorPrompt = `

## Tool Usage Guidelines

### ListDevices / GetDeviceInfo / SearchDocs (read-only, no side effects)

**When to use:** Call these freely at the start of any task to understand the environment.

<example>
User: Check disk usage on all web servers.
Assistant: Calls ListDevices to find web servers before running any commands.
</example>

### VerifyTool (read-only, has retry semantics)

**When to use:** After a deployment or config change, to poll until a service is ready.
**When NOT to use:** Don't use for a one-shot check — use RunCommand instead. VerifyTool retries on failure, adding latency when you just need a single result.

<example>
User: Restart nginx and confirm it's up.
Assistant: Calls RunCommand to restart, then VerifyTool to poll until nginx responds.
</example>

<example>
User: Is port 80 open on web-01?
Assistant: Calls RunCommand with "ss -tlnp | grep :80". Does NOT use VerifyTool.
</example>

### RunCommand / RunCommandBatch (has side effects)

**When to use:**
- Explore phase: read-only commands (ls, cat, grep, ps, df, systemctl status) — use freely
- Act phase: state-changing commands (rm, kill, systemctl restart, apt, chmod) — only after confirming intent

**When NOT to use:** Do not run state-changing commands before the user has confirmed the plan.

<example>
User: Clean up logs older than 30 days on all app servers.
Assistant: First calls RunCommandBatch with "find /var/log -mtime +30" to preview what would be deleted. Confirms with user. Then runs the delete command.
</example>

<example>
User: Restart the database service.
Assistant: Confirms the target host and service name, then calls RunCommand with "systemctl restart postgresql".
</example>

### CallAPI (GET: read-only; POST/PUT/DELETE: has side effects)

**When to use:**
- GET: use freely in Explore phase
- POST/PUT/DELETE: only in Act phase after confirming intent

<example>
User: Push a new ACL rule via the firewall API.
Assistant: Shows the request body to the user, confirms, then calls CallAPI with POST.
</example>

### Intent Field (RunCommand / RunCommandBatch / CallAPI)

Always set the intent field. This field is shown to the user in the UI.

**Rules:**
- Write the goal only — do not include device names (the UI adds those automatically)
- Keep it short: 10 Chinese characters or fewer is ideal

<example>
Good: "重启 nginx 使配置生效"
Good: "清理 30 天前的日志"
Bad: "在 local110 和 local201 上重启 nginx" — device names belong in host_ids, not intent
</example>`
```

- [ ] **Step 2: Build to verify no syntax errors**

```bash
cd /Users/cw/fty.ai/spider.ai && go build ./internal/agent/...
```

Expected: no output (clean build)

- [ ] **Step 3: Commit**

```bash
git add internal/agent/factory.go
git commit -m "refactor: simplify toolBehaviorPrompt, fix exemple typos, remove reasoning blocks"
```

---

### Task 2: Replace `todoTaskPrompt`

**Files:**
- Modify: `internal/agent/factory.go:228-285`

- [ ] **Step 1: Replace the constant**

Replace lines 228–285 with:

```go
const todoTaskPrompt = `

## Task Management (TodoTask tool)

Use the TodoTask tool proactively to track progress on complex tasks.

**When to use:**
- Task requires 3 or more distinct steps
- User provides multiple tasks to complete

**When NOT to use:**
- Single, straightforward task
- Purely conversational or informational response

**Rules:**
- Mark a task in_progress BEFORE beginning work on it
- Only ONE task in_progress at a time
- Mark completed IMMEDIATELY after finishing — do not batch completions
- Only mark completed when fully done; if blocked, keep in_progress and create a new task describing the blocker

<example>
User: Check disk usage on all web servers, clean up logs older than 30 days, and restart nginx if free space is below 20%.
Assistant: Creates tasks: 1) Check disk usage 2) Clean up logs 3) Restart nginx if space < 20%
</example>

<example>
User: What is the IP address of host web-01?
Assistant: Calls GetDeviceInfo directly. No todo list.
</example>`
```

- [ ] **Step 2: Build**

```bash
go build ./internal/agent/...
```

Expected: clean

- [ ] **Step 3: Commit**

```bash
git add internal/agent/factory.go
git commit -m "refactor: simplify todoTaskPrompt, remove redundant examples"
```

---

### Task 3: Add `complexTaskPrompt` and wire it in

**Files:**
- Modify: `internal/agent/factory.go` — insert new constant after `todoTaskPrompt`, update `BuildSystemPrompt`

- [ ] **Step 1: Insert new constant after `todoTaskPrompt`**

After the closing backtick of `todoTaskPrompt`, add:

```go
const complexTaskPrompt = `

## Complex Multi-Step Tasks

**Explore → Plan → Confirm → Act → Verify**

**Dependency chain:** If a step fails, stop. Report what failed before asking how to continue.

**Conditional branching:** Gather facts in Explore phase first. Pick one path based on data — do not execute branches speculatively.

<example>
User: Optimize the web server response time.
Assistant: Collects CPU, memory, and I/O metrics first. Then picks one optimization path based on the bottleneck — does not apply all optimizations at once.
</example>

**Verification:** After each Act step, verify before marking completed. If verification fails, keep in_progress and offer rollback if available.`
```

- [ ] **Step 2: Wire into `BuildSystemPrompt`**

Both return statements in `BuildSystemPrompt` currently end with `+ toolBehaviorPrompt + todoTaskPrompt`. Change both to:

```go
+ toolBehaviorPrompt + todoTaskPrompt + complexTaskPrompt
```

- [ ] **Step 3: Build**

```bash
go build ./internal/agent/...
```

Expected: clean

- [ ] **Step 4: Run existing tests**

```bash
go test ./internal/agent/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/factory.go
git commit -m "feat: add complexTaskPrompt — dependency chain, conditional branching, verification"
```
