import { authHeaders } from './auth'

export interface Conversation {
  id: string
  user_id: string
  title: string
  created_at: string
  updated_at: string
}

export interface ChatMessage {
  id: string
  conversation_id: string
  role: string
  content: string
  created_at: string
}

export interface ChatEvent {
  type: 'text_delta' | 'tool_start' | 'tool_result' | 'confirm_required' | 'error' | 'done'
  content?: Record<string, any>
}

export async function createConversation(title: string): Promise<Conversation> {
  const res = await fetch('/api/v1/chat/conversations', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ title }),
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

export function sendMessage(
  conversationId: string,
  content: string,
  onEvent: (event: ChatEvent) => void,
): AbortController {
  const ctrl = new AbortController()
  const run = async () => {
    const res = await fetch(`/api/v1/chat/conversations/${conversationId}/messages`, {
      method: 'POST',
      signal: ctrl.signal,
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ content }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'request failed' }))
      onEvent({ type: 'error', content: { error: err.error } })
      return
    }
    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop()!
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          try {
            const event: ChatEvent = JSON.parse(line.slice(6))
            onEvent(event)
          } catch { /* skip malformed */ }
        }
      }
    }
  }
  run().catch((err) => {
    if (err.name !== 'AbortError') {
      onEvent({ type: 'error', content: { error: err.message } })
    }
  })
  return ctrl
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
