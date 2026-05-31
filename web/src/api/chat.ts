import { api } from '@/shared/api/client'

export interface Conversation {
  id: string
  user_id: string
  title: string
  status: string
  permission_mode?: string
  created_at: string
  updated_at: string
}

export interface ChatMessage {
  id: string
  conversation_id: string
  role: string
  content: string
  tool_calls?: string
  created_at: string
}

export interface ChatEvent {
  type: 'text_delta' | 'tool_start' | 'tool_result' | 'confirm_required' | 'error' | 'done' | 'message' | 'todo_update' | 'turn_usage' | 'mid_turn_user_message'
  content?: Record<string, any>
}

export interface Todo {
  id: number; seq: number; conversation_id: string; subject: string; active_form?: string
  description?: string; status: string; owner?: string
  blocked_by?: number[]; created_at: string; updated_at: string
}

export async function createConversation(title?: string): Promise<Conversation> {
  return api.post<Conversation>('/chat/conversations', title ? { title } : undefined)
}

export async function listConversations(): Promise<Conversation[]> {
  return api.get<Conversation[]>('/chat/conversations')
}

export async function getConversation(id: string): Promise<{ conversation: Conversation; messages: ChatMessage[]; todo_tasks: Todo[]; queued_messages: string[] }> {
  return api.get(`/chat/conversations/${id}`)
}

export async function deleteConversation(id: string): Promise<void> {
  return api.delete(`/chat/conversations/${id}`, { responseType: 'void' })
}

export async function updateTitle(id: string, title: string): Promise<void> {
  return api.patch(`/chat/conversations/${id}`, { title }, { responseType: 'void' })
}

// subscribeConversation opens a persistent EventSource for a conversation.
// lastEventId: message UUID cursor — skip messages already loaded from DB
// Returns a cleanup function to close the connection.
export function subscribeConversation(
  conversationId: string,
  onEvent: (event: ChatEvent) => void,
  lastEventId?: string,
): () => void {
  const url = lastEventId
    ? `/api/v1/chat/conversations/${conversationId}/stream?last_event_id=${lastEventId}`
    : `/api/v1/chat/conversations/${conversationId}/stream`
  const es = new EventSource(url)
  es.onmessage = (e) => {
    try {
      const event: ChatEvent = JSON.parse(e.data)
      onEvent(event)
    } catch { /* skip malformed */ }
  }
  es.onerror = () => {
    // EventSource auto-reconnects on error — don't close
  }
  return () => es.close()
}

export async function sendMessage(
  conversationId: string,
  content: string,
  hostIds?: string[] | null,
  signal?: AbortSignal,
): Promise<{ status: 'accepted' | 'queued' }> {
  const body: Record<string, unknown> = { content }
  if (hostIds && hostIds.length > 0) body.host_ids = hostIds

  // Note: api client doesn't support AbortSignal yet, so we use fetch directly for this function
  const res = await fetch(`/api/v1/chat/conversations/${conversationId}/messages`, {
    method: 'POST',
    signal,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    const err = new Error(data.error || `HTTP ${res.status}`) as any
    err.status = res.status
    throw err
  }
  return res.json()
}

export async function suggestTitle(id: string): Promise<string> {
  const data = await api.post<{ title: string }>(`/chat/conversations/${id}/suggest-title`)
  return data.title
}

export async function getActiveModel(): Promise<{provider_id: string, model: string, provider_name: string}> {
  try {
    const providers = await api.get<any[]>('/providers')
    const active = providers.find((p: any) => p.is_active)
    return {
      provider_id: active?.id || '',
      model: active?.selected_model || '',
      provider_name: active?.name || '',
    }
  } catch {
    return { provider_id: '', model: '', provider_name: '' }
  }
}

export async function setActiveModel(providerId: string, model: string): Promise<void> {
  return api.put(`/providers/${providerId}/model`, { model }, { responseType: 'void' })
}

export async function confirmAction(
  conversationId: string,
  requestId: string,
  approved: boolean,
): Promise<void> {
  return api.post(
    `/chat/conversations/${conversationId}/confirm/${requestId}`,
    { approved },
    { responseType: 'void' }
  )
}

export async function cancelConversation(id: string): Promise<void> {
  return api.post(`/chat/conversations/${id}/cancel`, undefined, { responseType: 'void' })
}

export async function exportConversation(id: string, format: 'md' | 'json'): Promise<void> {
  // Use fetch directly to access response headers for filename
  const res = await fetch(`/api/v1/chat/conversations/${id}/export?format=${format}`)
  if (!res.ok) {
    const error = await res.json().catch(() => ({}))
    throw new Error(error.error || 'Export failed')
  }
  const blob = await res.blob()
  const disposition = res.headers.get('Content-Disposition') || ''
  const match = disposition.match(/filename="([^"]+)"/)
  const filename = match ? match[1] : `conversation.${format}`
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
