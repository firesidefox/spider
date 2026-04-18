import { authHeaders } from './auth'
import type { UserInfo } from './auth'

export async function listUsers(): Promise<UserInfo[]> {
  const res = await fetch('/api/v1/users', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function createUser(username: string, password: string, role: string): Promise<UserInfo> {
  const res = await fetch('/api/v1/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ username, password, role }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function updateUser(id: string, data: { role?: string; enabled?: boolean; password?: string }): Promise<UserInfo> {
  const res = await fetch(`/api/v1/users/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteUser(id: string): Promise<void> {
  const res = await fetch(`/api/v1/users/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}
