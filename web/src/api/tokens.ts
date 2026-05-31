import { api } from '@/shared/api/client'

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
  return api.get('/tokens')
}

export async function createToken(name: string, expiresAt: string): Promise<CreateTokenResponse> {
  return api.post('/tokens', { name, expires_at: expiresAt })
}

export async function deleteToken(id: string): Promise<void> {
  return api.delete(`/tokens/${id}`, undefined, { responseType: 'void' })
}
