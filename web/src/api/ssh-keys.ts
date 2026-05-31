import { api } from '@/shared/api/client'

export interface SafeSSHKey {
  id: string
  name: string
  fingerprint: string
  created_at: string
  updated_at: string
}

export async function listSSHKeys(): Promise<SafeSSHKey[]> {
  return api.get('/ssh-keys')
}

export async function createSSHKey(name: string, privateKey: string, passphrase: string): Promise<SafeSSHKey> {
  return api.post('/ssh-keys', { name, private_key: privateKey, passphrase })
}

export async function deleteSSHKey(id: string): Promise<void> {
  return api.delete(`/ssh-keys/${id}`, undefined, { responseType: 'void' })
}
