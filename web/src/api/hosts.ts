export interface SafeHost {
  id: string
  name: string
  ip: string
  port: number
  username: string
  auth_type: string
  proxy_host_id?: string
  tags: string[]
  created_at: string
  updated_at: string
}

export interface AddHostRequest {
  name: string
  ip: string
  port: number
  username: string
  auth_type: string
  credential: string
  passphrase?: string
  proxy_host_id?: string
  tags: string[]
}

export async function listHosts(tag?: string): Promise<SafeHost[]> {
  const url = tag ? `/api/v1/hosts?tag=${encodeURIComponent(tag)}` : '/api/v1/hosts'
  const res = await fetch(url)
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function addHost(req: AddHostRequest): Promise<SafeHost> {
  const res = await fetch('/api/v1/hosts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(req) })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function updateHost(id: string, req: Partial<AddHostRequest>): Promise<SafeHost> {
  const res = await fetch(`/api/v1/hosts/${id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(req) })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteHost(id: string): Promise<void> {
  const res = await fetch(`/api/v1/hosts/${id}`, { method: 'DELETE' })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function pingHost(id: string): Promise<{ connected: boolean; latency_ms?: number; error?: string }> {
  const res = await fetch(`/api/v1/hosts/${id}/ping`, { method: 'POST' })
  return res.json()
}
