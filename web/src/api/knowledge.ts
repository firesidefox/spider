import { authHeaders } from './auth'

const BASE = '/api/v1'

export interface KnowledgeGroup {
  id: number
  name: string
  created_at: string
}

export interface KnowledgeDocument {
  id: number
  group_id: number
  name: string
  doc_type: string
  raw_content: string
  status: string
  error_msg: string
  entry_count: number
  created_at: string
  updated_at: string
}

export interface KnowledgeSection {
  id: number
  document_id: number
  name: string
  summary: string
  position: number
  entry_count: number
}

export interface KnowledgeEntry {
  id: number
  title: string
  summary: string
}

export interface KnowledgeEntryParam {
  name: string
  in?: string
  type?: string
  required: boolean
  description?: string
}

export interface KnowledgeEntryResponse {
  description?: string
  example?: any
}

export interface KnowledgeEntryDetail {
  id: number
  document_id: number
  section_id?: number | null
  title: string
  summary: string
  content: string
  position: number
  method?: string
  path?: string
  description?: string
  parameters?: KnowledgeEntryParam[]
  responses?: Record<string, KnowledgeEntryResponse>
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
  if (r.status === 204) return undefined as T

  const text = await r.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

export async function listGroups(): Promise<KnowledgeGroup[]> {
  const r = await fetch(`${BASE}/knowledge-groups`, { headers: authHeaders() })
  return handleResponse<KnowledgeGroup[]>(r)
}

export async function createGroup(name: string): Promise<KnowledgeGroup> {
  const r = await fetch(`${BASE}/knowledge-groups`, {
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

export async function deleteGroups(ids: number[]): Promise<void> {
  const r = await fetch(`${BASE}/knowledge-groups`, {
    method: 'DELETE',
    headers: authHeaders(),
    body: JSON.stringify({ ids })
  })
  await handleResponse<void>(r)
}

export async function listDocuments(groupID: number): Promise<KnowledgeDocument[]> {
  const r = await fetch(`${BASE}/knowledge-groups/${groupID}/documents`, { headers: authHeaders() })
  return handleResponse<KnowledgeDocument[]>(r)
}

export async function getDocument(docID: number): Promise<KnowledgeDocument> {
  const r = await fetch(`${BASE}/knowledge-documents/${docID}`, { headers: authHeaders() })
  return handleResponse<KnowledgeDocument>(r)
}

export async function getSections(docID: number): Promise<KnowledgeSection[]> {
  const r = await fetch(`${BASE}/knowledge-documents/${docID}/sections`, { headers: authHeaders() })
  return handleResponse<KnowledgeSection[]>(r)
}

export async function getEntries(sectionID: number): Promise<KnowledgeEntry[]> {
  const r = await fetch(`${BASE}/knowledge-sections/${sectionID}/entries`, { headers: authHeaders() })
  return handleResponse<KnowledgeEntry[]>(r)
}

export async function getEntry(entryID: number): Promise<KnowledgeEntryDetail> {
  const r = await fetch(`${BASE}/knowledge-entries/${entryID}`, { headers: authHeaders() })
  return handleResponse<KnowledgeEntryDetail>(r)
}

export async function deleteDocuments(ids: number[]): Promise<void> {
  const r = await fetch(`${BASE}/knowledge-documents`, {
    method: 'DELETE',
    headers: authHeaders(),
    body: JSON.stringify({ ids })
  })
  await handleResponse<void>(r)
}

export async function moveDocuments(ids: number[], groupID: number): Promise<void> {
  const r = await fetch(`${BASE}/knowledge-documents`, {
    method: 'PATCH',
    headers: authHeaders(),
    body: JSON.stringify({ ids, group_id: groupID })
  })
  await handleResponse<void>(r)
}

export interface ReindexResponse {
  results: ImportResult[]
  errors: Record<string, string>
}

export async function reindexDocuments(ids: number[]): Promise<ReindexResponse> {
  const r = await fetch(`${BASE}/knowledge-documents/reindex`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ ids })
  })
  return handleResponse<ReindexResponse>(r)
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
