import { api } from '@/shared/api/client'
import type { UserInfo } from './auth'

export async function listUsers(): Promise<UserInfo[]> {
  return api.get('/users')
}

export async function createUser(username: string, password: string, role: string): Promise<UserInfo> {
  return api.post('/users', { username, password, role })
}

export async function updateUser(id: string, data: { role?: string; enabled?: boolean; password?: string }): Promise<UserInfo> {
  return api.patch(`/users/${id}`, data)
}

export async function deleteUser(id: string): Promise<void> {
  return api.delete(`/users/${id}`, { responseType: 'void' })
}
