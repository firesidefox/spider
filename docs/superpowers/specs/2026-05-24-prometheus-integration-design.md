# Prometheus Integration Design

**Date:** 2026-05-24  
**Status:** Approved

## Overview

Add Prometheus query and alert integration to spider.ai. Prometheus instances are configured as a new `AccessFace` type (`prometheus`) on individual hosts. Two agent tools expose free PromQL queries and active alert retrieval. An Alertmanager webhook endpoint auto-creates tasks from firing alerts.

## 1. Data Model

### New AccessFace type

```go
// internal/models/host.go
FacePrometheus AccessFaceType = "prometheus"
```

No new DB columns. Existing `AccessFace` fields cover all Prometheus needs:

| AccessFace field     | Prometheus use              |
|----------------------|-----------------------------|
| `base_url`           | Prometheus base URL (e.g. `http://prom:9090`) |
| `rest_auth_type`     | `none` \| `bearer` \| `basic` |
| `encrypted_credential` | Bearer token or Basic credentials |

### Constraints

- A prometheus face does not use `ip`, `port`, `username`, or any SSH fields.
- `base_url` is required; validation rejects empty `base_url` for prometheus faces.
- A host may have at most one prometheus face (enforced at store layer).

## 2. Agent Tools

### `QueryMetrics`

Execute a free PromQL expression against the Prometheus instance bound to a host.

**Input schema:**

```json
{
  "host_id":  "string (required) — host with a prometheus face",
  "query":    "string (required) — PromQL expression",
  "start":    "string (optional) — RFC3339 or Unix timestamp; triggers range query",
  "end":      "string (optional) — RFC3339 or Unix timestamp",
  "step":     "string (optional) — duration string, e.g. '1m', '30s'"
}
```

**Behavior:**
- If `start`/`end` omitted → instant query (`GET /api/v1/query`)
- If `start`/`end` present → range query (`GET /api/v1/query_range`)
- Returns raw Prometheus JSON (`resultType` + `result`)
- Risk level: `L1` (read-only)
- Concurrency safe: yes

**System prompt guidance:**
- Use after `GetHosts` to identify which host has the prometheus face
- Prefer instant queries for current state; range queries for trend analysis
- Do not construct queries wider than needed (avoid `{}` without label matchers on large clusters)

### `GetAlerts`

Fetch active alerts from a host's Prometheus instance.

**Input schema:**

```json
{
  "host_id": "string (required) — host with a prometheus face",
  "filter":  "string (optional) — label selector, e.g. 'severity=critical,job=node'"
}
```

**Behavior:**
- Calls `GET /api/v1/alerts`
- Filters to `state=firing` alerts only
- Applies optional `filter` label matching client-side
- Returns list of alerts with `labels`, `annotations`, `startsAt`
- Risk level: `L1` (read-only)
- Concurrency safe: yes

## 3. Alertmanager Webhook

### Endpoint

```
POST /api/webhooks/alertmanager/:face_id?token=<webhook_token>
```

- `:face_id` — ID of the prometheus `AccessFace`; ties the webhook to a specific host without label parsing
- `token` — static token stored in spider config; rejects requests without matching token

### Payload

Standard Alertmanager webhook v4 format. Only `firing` alerts are processed; `resolved` alerts are ignored (no auto-action on resolution).

### Auto-task creation

For each firing alert group received:

1. Look up host via `face_id`
2. Create a `Task` with:
   - Title: `[Alert] <alertname> on <host_name>`
   - Description: alert `labels` + `annotations.summary` + `annotations.description`
   - `host_ids`: the matched host
3. Task is created in `pending` state; agent is **not** auto-started (user triggers manually)

### Security

- Webhook token is a random 32-byte hex string generated on first webhook request, stored in the `config` table under key `webhook_alertmanager_token`
- Token is displayed once in the UI under Settings → Webhooks
- No auth header — token in query param only (Alertmanager webhook config supports this natively)

## 4. Internal Prometheus Client

Shared client used by both agent tools and the webhook handler:

```go
// internal/prometheus/client.go
type Client struct { baseURL, authType, token string }
func NewClient(face *models.AccessFace, decryptedCred string) *Client
func (c *Client) QueryInstant(ctx, query string, ts time.Time) (*QueryResult, error)
func (c *Client) QueryRange(ctx, query, start, end, step string) (*QueryResult, error)
func (c *Client) Alerts(ctx context.Context) ([]Alert, error)
```

Thin HTTP wrapper. No retry logic. Timeout: 30s default (context-cancelable).

## 5. Out of Scope

- Prometheus metric discovery / autocomplete
- Grafana integration
- Alert silencing / inhibition via API
- Multiple prometheus faces per host
- Topology-level Prometheus config (deferred)
