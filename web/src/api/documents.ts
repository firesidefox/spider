import { authHeaders } from './auth'

export interface DocumentGroup {
  id: number
  name: string
  created_at: string
}

export interface Document {
  id: number
  group_id: number | null
  vendor: string
  tags: string[]
  title: string
  content: string
  source_file: string
  chunk_index: number
  created_at: string
}

export interface IngestRequest {
  vendor: string
  content: string
  source_file: string
  chunk_index: number
  group_id?: number | null
  use_embedding?: boolean
}

export async function listDocumentsByGroup(groupId: number): Promise<Document[]> {
  const res = await fetch(`/api/v1/documents?group_id=${groupId}`, { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function listDocuments(vendor?: string, tag?: string): Promise<Document[]> {
  const params = new URLSearchParams()
  if (vendor) params.set('vendor', vendor)
  if (tag) params.set('tag', tag)
  const url = '/api/v1/documents' + (params.toString() ? '?' + params : '')
  const res = await fetch(url, { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function ingestDocument(req: IngestRequest): Promise<void> {
  const res = await fetch('/api/v1/documents', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function deleteDocument(id: number): Promise<void> {
  const res = await fetch(`/api/v1/documents/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function searchDocuments(q: string, vendor?: string, topK = 5): Promise<Document[]> {
  const params = new URLSearchParams({ q, top_k: String(topK) })
  if (vendor) params.set('vendor', vendor)
  const res = await fetch('/api/v1/documents/search?' + params, { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function moveDocument(docId: number, groupId: number | null): Promise<void> {
  const res = await fetch(`/api/v1/documents/${docId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ group_id: groupId }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function listGroups(): Promise<DocumentGroup[]> {
  const res = await fetch('/api/v1/document-groups', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function createGroup(name: string): Promise<DocumentGroup> {
  const res = await fetch('/api/v1/document-groups', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ name }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function renameGroup(id: number, name: string): Promise<void> {
  const res = await fetch(`/api/v1/document-groups/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ name }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function deleteGroup(id: number): Promise<void> {
  const res = await fetch(`/api/v1/document-groups/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function deleteBatchDocuments(ids: number[]): Promise<void> {
  const res = await fetch('/api/v1/documents', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ ids }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function deleteBatchGroups(ids: number[], deleteDocuments: boolean): Promise<void> {
  const res = await fetch('/api/v1/document-groups', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ ids, delete_documents: deleteDocuments }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
}
