import { api } from '@/shared/api/client'

export interface ExecutionLog {
  id: string
  host_id: string
  host_name: string
  command: string
  stdout: string
  stderr: string
  exit_code: number
  duration_ms: number
  triggered_by: string
  created_at: string
}

export async function listLogs(opts?: { hostId?: string; limit?: number; offset?: number }): Promise<ExecutionLog[]> {
  const p = new URLSearchParams()
  if (opts?.hostId) p.set('host_id', opts.hostId)
  if (opts?.limit) p.set('limit', String(opts.limit))
  if (opts?.offset) p.set('offset', String(opts.offset))
  return api.get(`/logs?${p.toString()}`)
}

export async function getLog(id: string): Promise<ExecutionLog> {
  return api.get(`/logs/${id}`)
}
