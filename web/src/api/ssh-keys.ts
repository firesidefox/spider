import { authHeaders } from './auth'

export interface SafeSSHKey {
  id: string
  name: string
  fingerprint: string
  created_at: string
  updated_at: string
}

export async function listSSHKeys(): Promise<SafeSSHKey[]> {
  const res = await fetch('/api/v1/me/ssh-keys', { headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function addSSHKey(name: string, privateKey: string, passphrase?: string): Promise<SafeSSHKey> {
  const body: any = { name, private_key: privateKey }
  if (passphrase) body.passphrase = passphrase
  const res = await fetch('/api/v1/me/ssh-keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function deleteSSHKey(id: string): Promise<void> {
  const res = await fetch(`/api/v1/me/ssh-keys/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!res.ok) throw new Error((await res.json()).error)
}
