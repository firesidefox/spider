import { api } from '@/shared/api/client'

export interface NotifyChannel {
  id: number
  type: string
  name: string
  enabled: boolean
  created_at: string
}

export interface CreateNotifyChannelRequest {
  type: string
  name: string
  config: string
  enabled: boolean
}

export async function listNotifyChannels(): Promise<NotifyChannel[]> {
  return api.get('/notify-channels')
}

export async function createNotifyChannel(data: CreateNotifyChannelRequest): Promise<NotifyChannel> {
  return api.post('/notify-channels', data)
}

export async function updateNotifyChannel(id: number, data: Partial<NotifyChannel>): Promise<NotifyChannel> {
  return api.patch(`/notify-channels/${id}`, data)
}

export async function deleteNotifyChannel(id: number): Promise<void> {
  return api.delete(`/notify-channels/${id}`, undefined, { responseType: 'void' })
}

export async function toggleNotifyChannel(id: number, enabled: boolean): Promise<NotifyChannel> {
  return api.patch(`/notify-channels/${id}/enabled`, { enabled })
}
