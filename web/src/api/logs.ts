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
  const res = await fetch('/api/v1/logs?' + p.toString())
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export async function getLog(id: string): Promise<ExecutionLog> {
  const res = await fetch(`/api/v1/logs/${id}`)
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}
