import { authHeaders } from './auth'

export interface TokenInfo {
  id: string
  name: string
  expires_at: string | null
  created_at: string
  last_used: string | null
}

export interface CreateTokenResponse extends TokenInfo {
  token: string  // 明文，仅此一次
}

export async function listTokens(): Promise<TokenInfo[]> {
  const res = await fetch('/api/v1/tokens', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function createToken(name: string, expiresAt?: string): Promise<CreateTokenResponse> {
  const body: any = { name }
  if (expiresAt) body.expires_at = expiresAt
  const res = await fetch('/api/v1/tokens', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteToken(id: string): Promise<void> {
  const res = await fetch(`/api/v1/tokens/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}
