# Tool Call Display Design

**Date:** 2026-05-17  
**Status:** Approved

## Overview

Redesign the tool call display in `ChatMessage.vue` to use a three-tier visual weight system. Replace the current TOOL badge + bordered card pattern with an indent-based hierarchy using `⎿` characters, matching the terminal aesthetic of Claude Code.

## Tool Classification

| Type | Tools | Visual Weight |
|------|-------|---------------|
| Housekeeping | `invoke_skill`, `Todo` | Lowest — near-invisible |
| Explore | `ListHosts`, `SearchDocs`, `GetTopology`, `Verify` | Medium — collapsed group |
| Execute | `RunCommandBatch`, `RunCommand`, `CallAPI`, `PollUntil`, `CreateTask` | Highest — full detail |

`Todo` is **never rendered**.

## Display Rules

### Housekeeping — `invoke_skill`

```
· invoke_skill(log-analysis)                    0ms
  ⎿ skill "log-analysis" loaded
```

- Dot prefix `·`, function-call format `name(arg)`
- Color: `#484f58` (near-invisible grey)
- Result line: `⎿` + one-line load result, color `#484f58`
- No result line for `Todo` (not rendered at all)

### Explore — collapsed group

**Collapsed (default):**
```
▼ 探索 (3)
  └ ListHosts                          5 hosts   1ms
  └ SearchDocs   "nginx 配置"           3 docs   45ms
  └ Verify       local-201 ssh      unreachable  2001ms
```

**Expanded (click to toggle):**
```
▶ 探索 (3)
  └ ListHosts                                    1ms
    ⎿ ecs-tencent · local-110 · local-201 · local-7 · xian-124
  └ SearchDocs   "nginx 配置"                   45ms
    ⎿ nginx.conf 参考 · 反向代理配置 · SSL 证书配置
  └ Verify       local-201 ssh               2001ms
    ⎿ Connection refused: 10.37.129.201:22
```

- Group header color: `#6e7681`
- Tool name color: `#6e7681`; error: `#f85149`
- Result summary: ok `#3fb950`, error `#f85149`

### Execute — full detail

```
▶ RunCommandBatch @ ecs-tencent · local-110 · local-7   301ms
  ⎿ grep -ci "error|fail|fatal" /var/log/syslog
  ⎿ 3 hosts ok · 0 critical errors
```

- `▶` + tool name (blue `#58a6ff`) + `@` + host list + duration
- Host list: plain text dot-separated, `white-space: normal` (wraps naturally)
- Command line: `⎿` + full command, `white-space: pre-wrap; word-break: break-all`
- Result line: `⎿` + result, ok `#3fb950` / error `#f85149`
- Error state: `▶` and tool name turn red `#f85149`, host list `#f8514466`

## Streaming State

All tool types use a single `*` pulse animation while in-progress:

```
* invoke_skill(log-analysis)
* 探索 (1)
  └ ListHosts                                          ···
* RunCommandBatch @ ecs-tencent · local-110 · local-7  ···
  ⎿ grep -ci "error|fail|fatal" /var/log/syslog
  ⎿ ···
```

- `*` color: `#58a6ff`, animation: `pulse 1.5s ease-in-out infinite` (opacity 0.3 → 1)
- `···` blink: `blink 1s step-end infinite`
- On completion: `*` replaced by `·` / `▼` / `▶` depending on tool type

## Color Reference

| Token | Value | Usage |
|-------|-------|-------|
| Housekeeping text | `#484f58` | invoke_skill name, arg, result |
| Housekeeping dot/paren | `#3d444d` | `·`, `(`, `)`, `⎿` |
| Explore label | `#6e7681` | group header, tool names |
| Explore muted | `#484f58` | params, duration |
| Execute blue | `#58a6ff` | tool name, `▶` |
| Execute hosts | `#484f58` | `@` host list text |
| Ok result | `#3fb950` | success result lines |
| Error | `#f85149` | error tool name, result, hosts |
| Error hosts | `#f8514466` | host list in error state |
| Streaming star | `#58a6ff` | `*` pulse |
| Hook char | `#3d444d` | `⎿`, `└`, `·` separators |

## Changes to `ChatMessage.vue`

### `EXPLORE_TOOLS` set
Add `GetTopology` if not already present. Remove `Todo` from rendering entirely.

### `renderItems` computed
- `Todo` calls: skip (do not push to items)
- `invoke_skill`: push as new `housekeeping` kind
- Existing `explore` / `act` kinds unchanged

### Template

Replace current `tool-call` bordered card with:
- `housekeeping` block: dot + function-call format + `⎿` result
- `explore` group: unchanged structure, updated CSS
- `act` block: `▶ name @ hosts   dur` header + `⎿ cmd` + `⎿ result` body

### CSS

Remove: `.tool-badge`, `.tool-badge-error`, `.tool-call` border/card styles  
Add: `.act-at`, `.act-hosts`, `.hk-call`, `.hk-dot`, streaming `.star` pulse
