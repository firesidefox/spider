import { api } from '@/shared/api/client'
import { ApiError } from '@/shared/api/client'

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

export type KBMode = 'none' | 'specific'

export interface KnowledgeSource {
  type: 'group' | 'doc'
  id: number
  name?: string
  title?: string
  group_id?: number
  group_name?: string
  description?: string
}

export interface AccessFace {
  id: string
  host_id: string
  type: 'ssh' | 'restapi' | 'prometheus'
  ip: string
  port: number
  username?: string
  ssh_auth_type?: 'password' | 'key' | 'key_password'
  ssh_key_id?: string
  ssh_legacy?: boolean
  ssh_login_input?: string
  base_url?: string
  rest_auth_type?: 'bearer' | 'basic' | 'apikey' | 'hmac_aksk' | 'none'
  rest_username?: string
  header_name?: string
  hmac_algo?: string
  kb_mode: KBMode
  knowledge_sources: KnowledgeSource[]
  probe_port?: number
  prometheus_source_id?: string
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
  type: 'ssh' | 'restapi' | 'prometheus'
  ip: string
  port: number
  username?: string
  ssh_auth_type?: string
  credential?: string
  passphrase?: string
  ssh_key_id?: string
  ssh_legacy?: boolean
  ssh_login_input?: string
  base_url?: string
  rest_auth_type?: string
  rest_username?: string
  header_name?: string
  hmac_algo?: string
  kb_mode?: KBMode
  knowledge_sources?: Array<{ type: 'group' | 'doc'; id: number }>
  probe_port?: number
  prometheus_source_id?: string
}

export async function listHosts(tag?: string): Promise<Host[]> {
  const path = tag ? `/hosts?tag=${encodeURIComponent(tag)}` : '/hosts'
  return api.get<Host[]>(path)
}

export async function getHost(id: string): Promise<Host> {
  return api.get<Host>(`/hosts/${id}`)
}

export async function addHost(req: AddHostRequest): Promise<Host> {
  return api.post<Host>('/hosts', req)
}

export async function updateHost(id: string, req: UpdateHostRequest): Promise<Host> {
  return api.put<Host>(`/hosts/${id}`, req)
}

export async function deleteHost(id: string): Promise<void> {
  return api.delete<void>(`/hosts/${id}`, undefined, { responseType: 'void' })
}

export async function listAccessFaces(hostId: string): Promise<AccessFace[]> {
  return api.get<AccessFace[]>(`/hosts/${hostId}/faces`)
}

export async function addAccessFace(hostId: string, req: AddAccessFaceRequest): Promise<AccessFace> {
  return api.post<AccessFace>(`/hosts/${hostId}/faces`, req)
}

export async function updateAccessFace(hostId: string, faceId: string, req: Partial<AddAccessFaceRequest>): Promise<AccessFace> {
  return api.put<AccessFace>(`/hosts/${hostId}/faces/${faceId}`, req)
}

export async function deleteAccessFace(hostId: string, faceId: string): Promise<void> {
  return api.delete<void>(`/hosts/${hostId}/faces/${faceId}`, undefined, { responseType: 'void' })
}

export async function getFingerprint(hostId: string): Promise<Fingerprint | null> {
  try {
    return await api.get<Fingerprint>(`/hosts/${hostId}/fingerprint`)
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return null
    }
    throw err
  }
}

export async function listMemories(hostId: string): Promise<Memory[]> {
  return api.get<Memory[]>(`/hosts/${hostId}/memories`)
}

export async function addMemory(hostId: string, content: string): Promise<Memory> {
  return api.post<Memory>(`/hosts/${hostId}/memories`, { content })
}

export async function deleteMemory(hostId: string, memId: number): Promise<void> {
  return api.delete<void>(`/hosts/${hostId}/memories/${memId}`, undefined, { responseType: 'void' })
}
