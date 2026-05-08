export interface Host {
  id: string
  name: string
  ip: string
  notes?: string
  tags: string[]
  vendor?: string
  product_name?: string
  product_version?: string
  created_at: string
  updated_at: string
  access_faces?: AccessFace[]
  fingerprint?: Fingerprint
  memories?: Memory[]
}

export interface AccessFace {
  id: string
  host_id: string
  type: 'ssh' | 'restapi'
  ip: string
  port: number
  username?: string
  ssh_auth_type?: 'password' | 'key' | 'key_password'
  ssh_key_id?: string
  ssh_legacy?: boolean
  base_url?: string
  rest_auth_type?: 'bearer' | 'basic' | 'apikey' | 'none'
  rest_username?: string
  header_name?: string
  knowledge_sources: Array<{ type: 'group' | 'doc'; id: number }>
  created_at: string
  updated_at: string
}

export interface Fingerprint {
  host_id: string
  ssh_host_key?: string
  system_version?: string
  hardware_id?: string
  api_signature?: string
  status: 'ok' | 'changed' | 'unverified'
  snapshot_at?: string
}

export interface Memory {
  id: number
  host_id: string
  content: string
  created_by: 'user' | 'agent'
  created_at: string
}

export interface AddHostRequest {
  name: string
  ip: string
  notes?: string
  tags: string[]
  vendor?: string
  product_name?: string
  product_version?: string
}

export interface UpdateHostRequest {
  name?: string
  ip?: string
  notes?: string
  tags?: string[]
  vendor?: string
  product_name?: string
  product_version?: string
}

export interface AddAccessFaceRequest {
  type: 'ssh' | 'restapi'
  ip: string
  port: number
  username?: string
  ssh_auth_type?: string
  credential?: string
  passphrase?: string
  ssh_key_id?: string
  ssh_legacy?: boolean
  base_url?: string
  rest_auth_type?: string
  rest_username?: string
  header_name?: string
  knowledge_sources?: Array<{ type: 'group' | 'doc'; id: number }>
}

async function apiFetch(url: string, init?: RequestInit) {
  const res = await fetch(url, init)
  if (!res.ok) throw new Error((await res.json()).error)
  return res
}

const json = (body: unknown) => ({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
const put  = (body: unknown) => ({ method: 'PUT',  headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })

export async function listHosts(tag?: string): Promise<Host[]> {
  const url = tag ? `/api/v1/hosts?tag=${encodeURIComponent(tag)}` : '/api/v1/hosts'
  return (await apiFetch(url)).json()
}

export async function getHost(id: string): Promise<Host> {
  return (await apiFetch(`/api/v1/hosts/${id}`)).json()
}

export async function addHost(req: AddHostRequest): Promise<Host> {
  return (await apiFetch('/api/v1/hosts', json(req))).json()
}

export async function updateHost(id: string, req: UpdateHostRequest): Promise<Host> {
  return (await apiFetch(`/api/v1/hosts/${id}`, put(req))).json()
}

export async function deleteHost(id: string): Promise<void> {
  await apiFetch(`/api/v1/hosts/${id}`, { method: 'DELETE' })
}

export async function pingHost(id: string): Promise<{ connected: boolean; latency_ms?: number; error?: string }> {
  return (await fetch(`/api/v1/hosts/${id}/ping`, { method: 'POST' })).json()
}

export async function listAccessFaces(hostId: string): Promise<AccessFace[]> {
  return (await apiFetch(`/api/v1/hosts/${hostId}/faces`)).json()
}

export async function addAccessFace(hostId: string, req: AddAccessFaceRequest): Promise<AccessFace> {
  return (await apiFetch(`/api/v1/hosts/${hostId}/faces`, json(req))).json()
}

export async function updateAccessFace(hostId: string, faceId: string, req: Partial<AddAccessFaceRequest>): Promise<AccessFace> {
  return (await apiFetch(`/api/v1/hosts/${hostId}/faces/${faceId}`, put(req))).json()
}

export async function deleteAccessFace(hostId: string, faceId: string): Promise<void> {
  await apiFetch(`/api/v1/hosts/${hostId}/faces/${faceId}`, { method: 'DELETE' })
}

export async function getFingerprint(hostId: string): Promise<Fingerprint | null> {
  const res = await fetch(`/api/v1/hosts/${hostId}/fingerprint`)
  if (res.status === 404) return null
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function listMemories(hostId: string): Promise<Memory[]> {
  return (await apiFetch(`/api/v1/hosts/${hostId}/memories`)).json()
}

export async function addMemory(hostId: string, content: string): Promise<Memory> {
  return (await apiFetch(`/api/v1/hosts/${hostId}/memories`, json({ content }))).json()
}

export async function deleteMemory(hostId: string, memId: number): Promise<void> {
  await apiFetch(`/api/v1/hosts/${hostId}/memories/${memId}`, { method: 'DELETE' })
}
