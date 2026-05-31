import { api } from '@/shared/api/client'

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

export async function listGroups(): Promise<KnowledgeGroup[]> {
  return api.get('/knowledge-groups')
}

export async function createGroup(name: string): Promise<KnowledgeGroup> {
  return api.post('/knowledge-groups', { name })
}

export async function deleteGroup(id: number): Promise<void> {
  return api.delete(`/knowledge-groups/${id}`, undefined, { responseType: 'void' })
}

export async function deleteGroups(ids: number[]): Promise<void> {
  return api.delete('/knowledge-groups', { ids }, { responseType: 'void' })
}

export async function listDocuments(groupID: number): Promise<KnowledgeDocument[]> {
  return api.get(`/knowledge-groups/${groupID}/documents`)
}

export async function getDocument(docID: number): Promise<KnowledgeDocument> {
  return api.get(`/knowledge-documents/${docID}`)
}

export async function deleteDocument(id: number): Promise<void> {
  return api.delete(`/knowledge-documents/${id}`, undefined, { responseType: 'void' })
}

export async function reindexDocument(id: number): Promise<void> {
  return api.post(`/knowledge-documents/${id}/reindex`, undefined, { responseType: 'void' })
}

export async function getSections(docID: number): Promise<KnowledgeSection[]> {
  return api.get(`/knowledge-documents/${docID}/sections`)
}

export async function getEntries(sectionID: number): Promise<KnowledgeEntry[]> {
  return api.get(`/knowledge-sections/${sectionID}/entries`)
}

export async function getEntry(entryID: number): Promise<KnowledgeEntryDetail> {
  return api.get(`/knowledge-entries/${entryID}`)
}

export async function deleteDocuments(ids: number[]): Promise<void> {
  return api.delete('/knowledge-documents', { ids }, { responseType: 'void' })
}

export async function moveDocuments(ids: number[], groupID: number): Promise<void> {
  return api.patch('/knowledge-documents', { ids, group_id: groupID }, { responseType: 'void' })
}

export interface ReindexResponse {
  results: ImportResult[]
  errors: Record<string, string>
}

export async function reindexDocuments(ids: number[]): Promise<ReindexResponse> {
  return api.post('/knowledge-documents/reindex', { ids })
}

export async function importDocument(groupID: number, file: File): Promise<ImportResult> {
  const fd = new FormData()
  fd.append('group_id', String(groupID))
  fd.append('file', file)
  return api.upload('/knowledge-documents/import', fd)
}

export interface TryEntryRequest {
  source_id: string
  params: Record<string, string>
}

export interface TryEntryResult {
  status: number
  body: string
  latency_ms: number
}

export async function tryEntry(entryID: number, req: TryEntryRequest): Promise<TryEntryResult> {
  return api.post(`/knowledge-entries/${entryID}/try`, req)
}
