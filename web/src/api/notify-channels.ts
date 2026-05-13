import { authHeaders } from './auth'

export interface NotifyChannel {
  id: number
  type: string
  name: string
  config: string
  enabled: boolean
  created_at: string
}

export interface DingTalkConfig {
  webhook_url: string
  secret: string
}

export async function listNotifyChannels(): Promise<NotifyChannel[]> {
  const res = await fetch('/api/v1/notify-channels', { headers: authHeaders() })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function createNotifyChannel(data: {
  type: string
  name: string
  config: string
  enabled: boolean
}): Promise<NotifyChannel> {
  const res = await fetch('/api/v1/notify-channels', {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateNotifyChannel(id: number, data: Partial<{
  type: string
  name: string
  config: string
  enabled: boolean
}>): Promise<NotifyChannel> {
  const res = await fetch(`/api/v1/notify-channels/${id}`, {
    method: 'PUT',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function deleteNotifyChannel(id: number): Promise<void> {
  const res = await fetch(`/api/v1/notify-channels/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error(await res.text())
}
