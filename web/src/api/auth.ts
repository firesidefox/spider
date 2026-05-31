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

export interface UIPrefs {
  target_panel_open: boolean
  target_panel_width: number
}

import { api } from '@/shared/api/client'

export async function login(username: string, password: string): Promise<LoginResponse> {
  return api.post('/auth/login', { username, password })
}

export async function logout(): Promise<void> {
  return api.post('/auth/logout', undefined, { responseType: 'void' })
}

export async function getMe(): Promise<UserInfo> {
  return api.get('/me')
}

export async function getUIPrefs(): Promise<UIPrefs> {
  return api.get('/me/prefs')
}

export async function setUIPrefs(prefs: UIPrefs): Promise<void> {
  return api.put('/me/prefs', prefs, { responseType: 'void' })
}

export function authHeaders(): Record<string, string> {
  // Cookie auto-sent by browser, no need for manual header
  // Keep localStorage for token display/status only
  return {}
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
