export interface UserInfo {
  id: string
  username: string
  role: 'admin' | 'operator' | 'viewer'
  enabled: boolean
  created_at: string
  last_login: string | null
}

export interface LoginResponse {
  token: string
  expires_at: string
  user: UserInfo
}

export async function login(username: string, password: string): Promise<LoginResponse> {
  const res = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function logout(): Promise<void> {
  await fetch('/api/v1/auth/logout', {
    method: 'POST',
    headers: authHeaders(),
  })
}

export async function getMe(): Promise<UserInfo> {
  const res = await fetch('/api/v1/me', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('spider_token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export function getStoredToken(): string | null {
  return localStorage.getItem('spider_token')
}

export function setStoredToken(token: string): void {
  localStorage.setItem('spider_token', token)
}

export function clearStoredToken(): void {
  localStorage.removeItem('spider_token')
}
