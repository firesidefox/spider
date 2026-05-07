import { authHeaders } from './auth'

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
  type: 'text_delta' | 'tool_start' | 'tool_result' | 'confirm_required' | 'error' | 'done' | 'message'
  content?: Record<string, any>
}

export async function createConversation(title?: string): Promise<Conversation> {
  const res = await fetch('/api/v1/chat/conversations', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: title ? JSON.stringify({ title }) : undefined,
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function listConversations(): Promise<Conversation[]> {
  const res = await fetch('/api/v1/chat/conversations', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function getConversation(id: string): Promise<{ conversation: Conversation; messages: ChatMessage[] }> {
  const res = await fetch(`/api/v1/chat/conversations/${id}`, { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteConversation(id: string): Promise<void> {
  const res = await fetch(`/api/v1/chat/conversations/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function updateTitle(id: string, title: string): Promise<void> {
  const res = await fetch(`/api/v1/chat/conversations/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ title }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
}

// subscribeConversation opens a persistent EventSource for a conversation.
// lastEventId: skip messages already loaded from DB (pass msgs.length - 1)
// Returns a cleanup function to close the connection.
export function subscribeConversation(
  conversationId: string,
  onEvent: (event: ChatEvent) => void,
  lastEventId?: number,
): () => void {
  const url = lastEventId !== undefined
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

export function sendMessage(
  conversationId: string,
  content: string,
): AbortController {
  const ctrl = new AbortController()
  fetch(`/api/v1/chat/conversations/${conversationId}/messages`, {
    method: 'POST',
    signal: ctrl.signal,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  }).catch(() => { /* errors come via EventSource */ })
  return ctrl
}

export async function getActiveModel(): Promise<{provider_id: string, model: string, provider_name: string}> {
  const res = await fetch('/api/v1/providers', { headers: authHeaders() })
  if (!res.ok) return { provider_id: '', model: '', provider_name: '' }
  const providers = await res.json()
  const active = providers.find((p: any) => p.is_active)
  return {
    provider_id: active?.id || '',
    model: active?.selected_model || '',
    provider_name: active?.name || '',
  }
}

export async function setActiveModel(providerId: string, model: string): Promise<void> {
  await fetch(`/api/v1/providers/${providerId}/model`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ model }),
  })
}

export async function confirmAction(
  conversationId: string,
  requestId: string,
  approved: boolean,
): Promise<void> {
  const res = await fetch(
    `/api/v1/chat/conversations/${conversationId}/confirm/${requestId}`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ approved }),
    },
  )
  if (!res.ok) throw new Error((await res.json()).error)
}

export async function cancelConversation(id: string): Promise<void> {
  await fetch(`/api/v1/chat/conversations/${id}/cancel`, {
    method: 'POST',
    headers: authHeaders(),
  })
}
