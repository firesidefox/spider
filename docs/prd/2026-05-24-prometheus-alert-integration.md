# PRD: Prometheus Alert Integration

**Date:** 2026-05-24  
**Status:** Backlog  
**Depends on:** Prometheus AccessFace (see `docs/superpowers/specs/2026-05-24-prometheus-integration-design.md`)

## Problem

Firing alerts from Prometheus/Alertmanager are not visible in spider.ai. Operators must context-switch to Alertmanager UI or Grafana to see what's firing, then manually create tasks to investigate.

## Goals

1. Agent can query active alerts from a host's Prometheus instance
2. Alertmanager can push firing alerts to spider.ai, which auto-creates tasks for investigation

## Requirements

### R1: `GetAlerts` Agent Tool

- Input: `host_id` (required), `filter` label selector (optional, e.g. `severity=critical`)
- Behavior: calls `GET /api/v1/alerts` on the host's prometheus face; filters to `state=firing`; applies optional label filter client-side
- Output: list of active alerts with `labels`, `annotations`, `startsAt`
- Risk: `L1` read-only, concurrency safe

### R2: Alertmanager Webhook Endpoint

```
POST /api/webhooks/alertmanager/:face_id?token=<token>
```

- `:face_id` — prometheus AccessFace ID; ties webhook to a host without label parsing
- `token` — random 32-byte hex, generated on first use, stored in `config` table under key `webhook_alertmanager_token`
- Accepts standard Alertmanager webhook v4 payload
- Processes only `firing` alerts; ignores `resolved`

### R3: Auto-Task Creation

For each firing alert group:
1. Look up host via `face_id`
2. Create Task:
   - Title: `[Alert] <alertname> on <host_name>`
   - Description: alert `labels` + `annotations.summary` + `annotations.description`
   - `host_ids`: matched host
3. Task starts in `pending` state; agent not auto-started

### R4: Token Display

Webhook token visible in Settings UI (Settings → Webhooks) so operators can configure Alertmanager receivers.

## Out of Scope

- Alert silencing / inhibition via API
- `resolved` alert handling (auto-close task)
- Multiple Alertmanager instances per host
