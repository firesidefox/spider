import { authHeaders } from './auth'

const BASE = '/api/v1'

export interface KnowledgeBase {
  id: number
  name: string
  created_at: string
}

export interface KnowledgeGroup {
  id: number
  kb_id: number
  name: string
  created_at: string
}

export interface KnowledgeDocument {
  id: number
  group_id: number
  name: string
  doc_type: string
  status: string
  error_msg: string
  entry_count: number
  created_at: string
  updated_at: string
}

export interface ImportResult {
  document_id: number
  entry_count: number
  section_count: number
}

async function handleResponse<T>(r: Response): Promise<T> {
  if (!r.ok) {
    let msg = `HTTP ${r.status}`
    try {
      const json = await r.json()
      if (json.error) msg = json.error
    } catch {
      const text = await r.text()
      if (text) msg = text
    }
    throw new Error(msg)
  }
  return r.json()
}

export async function listKBs(): Promise<KnowledgeBase[]> {
  const r = await fetch(`${BASE}/knowledge-bases`, { headers: authHeaders() })
  return handleResponse<KnowledgeBase[]>(r)
}

export async function createKB(name: string): Promise<KnowledgeBase> {
  const r = await fetch(`${BASE}/knowledge-bases`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ name })
  })
  return handleResponse<KnowledgeBase>(r)
}

export async function deleteKB(id: number): Promise<void> {
  const r = await fetch(`${BASE}/knowledge-bases/${id}`, {
    method: 'DELETE',
    headers: authHeaders()
  })
  await handleResponse<void>(r)
}

export async function listGroups(kbID: number): Promise<KnowledgeGroup[]> {
  const r = await fetch(`${BASE}/knowledge-bases/${kbID}/groups`, { headers: authHeaders() })
  return handleResponse<KnowledgeGroup[]>(r)
}

export async function createGroup(kbID: number, name: string): Promise<KnowledgeGroup> {
  const r = await fetch(`${BASE}/knowledge-bases/${kbID}/groups`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ name })
  })
  return handleResponse<KnowledgeGroup>(r)
}

export async function deleteGroup(id: number): Promise<void> {
  const r = await fetch(`${BASE}/knowledge-groups/${id}`, {
    method: 'DELETE',
    headers: authHeaders()
  })
  await handleResponse<void>(r)
}

export async function listDocuments(groupID: number): Promise<KnowledgeDocument[]> {
  const r = await fetch(`${BASE}/knowledge-groups/${groupID}/documents`, { headers: authHeaders() })
  return handleResponse<KnowledgeDocument[]>(r)
}

export async function deleteDocuments(ids: number[]): Promise<void> {
  const r = await fetch(`${BASE}/knowledge-documents`, {
    method: 'DELETE',
    headers: authHeaders(),
    body: JSON.stringify({ ids })
  })
  await handleResponse<void>(r)
}

export async function importDocument(groupID: number, file: File): Promise<ImportResult> {
  const fd = new FormData()
  fd.append('group_id', String(groupID))
  fd.append('file', file)
  const r = await fetch(`${BASE}/knowledge-documents/import`, {
    method: 'POST',
    headers: authHeaders(),
    body: fd
  })
  return handleResponse<ImportResult>(r)
}
