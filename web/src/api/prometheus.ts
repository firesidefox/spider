import { api } from '@/shared/api/client'

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
  return api.get<PrometheusSource[]>('/prometheus/sources')
}

export async function addPrometheusSource(req: AddPrometheusSourceRequest): Promise<PrometheusSource> {
  return api.post<PrometheusSource>('/prometheus/sources', req)
}

export async function updatePrometheusSource(id: string, req: UpdatePrometheusSourceRequest): Promise<PrometheusSource> {
  return api.put<PrometheusSource>(`/prometheus/sources/${id}`, req)
}

export async function deletePrometheusSource(id: string): Promise<void> {
  return api.delete<void>(`/prometheus/sources/${id}`, { responseType: 'void' })
}

export async function testPrometheusConnection(id: string): Promise<{ ok: boolean; latency_ms?: number; error?: string }> {
  return api.get<{ ok: boolean; latency_ms?: number; error?: string }>(`/prometheus/sources/${id}/test`)
}

export async function addPrometheusBinding(req: {
  source_id: string
  scope_type: PrometheusScopeType
  topology_id?: string
  layer?: string
  host_id?: string
}): Promise<PrometheusBinding> {
  return api.post<PrometheusBinding>('/prometheus/bindings', req)
}

export async function deletePrometheusBinding(id: string): Promise<void> {
  return api.delete<void>(`/prometheus/bindings/${id}`, { responseType: 'void' })
}
