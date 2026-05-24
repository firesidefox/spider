import { authHeaders } from './auth'

export type PrometheusAuthType = 'none' | 'basic' | 'bearer'
export type PrometheusScopeType = 'topology_layer' | 'host'

export interface PrometheusSource {
  id: string
  name: string
  base_url: string
  timeout_seconds: number
  auth_type: PrometheusAuthType
  username?: string
  skip_tls_verify: boolean
  created_at: string
  updated_at: string
}

export interface PrometheusBinding {
  id: string
  source_id: string
  scope_type: PrometheusScopeType
  topology_id?: string
  layer?: string
  host_id?: string
  created_at: string
}

export interface AddPrometheusSourceRequest {
  name: string
  base_url: string
  timeout_seconds?: number
  auth_type: PrometheusAuthType
  username?: string
  password?: string
  token?: string
  skip_tls_verify?: boolean
}

export interface UpdatePrometheusSourceRequest {
  name?: string
  base_url?: string
  timeout_seconds?: number
  auth_type?: PrometheusAuthType
  username?: string
  password?: string
  token?: string
  skip_tls_verify?: boolean
}

export async function listPrometheusSources(): Promise<PrometheusSource[]> {
  const res = await fetch('/api/v1/prometheus/sources', { headers: authHeaders() })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function addPrometheusSource(req: AddPrometheusSourceRequest): Promise<PrometheusSource> {
  const res = await fetch('/api/v1/prometheus/sources', {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updatePrometheusSource(id: string, req: UpdatePrometheusSourceRequest): Promise<PrometheusSource> {
  const res = await fetch(`/api/v1/prometheus/sources/${id}`, {
    method: 'PUT',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deletePrometheusSource(id: string): Promise<void> {
  const res = await fetch(`/api/v1/prometheus/sources/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function testPrometheusConnection(id: string): Promise<{ ok: boolean; latency_ms?: number; error?: string }> {
  const res = await fetch(`/api/v1/prometheus/sources/${id}/test`, { headers: authHeaders() })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function addPrometheusBinding(req: {
  source_id: string
  scope_type: PrometheusScopeType
  topology_id?: string
  layer?: string
  host_id?: string
}): Promise<PrometheusBinding> {
  const res = await fetch('/api/v1/prometheus/bindings', {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deletePrometheusBinding(id: string): Promise<void> {
  const res = await fetch(`/api/v1/prometheus/bindings/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error(await res.text())
}
