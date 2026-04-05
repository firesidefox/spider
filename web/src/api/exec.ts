export interface ExecResult {
  host: string
  command: string
  stdout: string
  stderr: string
  exit_code: number
  duration_ms: number
  error?: string
}

export async function execCommand(hostId: string, command: string, timeoutSeconds = 30): Promise<ExecResult> {
  const res = await fetch('/api/v1/exec', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ host_id: hostId, command, timeout_seconds: timeoutSeconds }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}

export interface StreamChunk {
  type: 'stdout' | 'stderr' | 'done'
  data?: string
  exit_code?: number
  duration_ms?: number
  error?: string
}

export function streamExec(
  hostId: string,
  command: string,
  timeout: number,
  onChunk: (chunk: StreamChunk) => void,
): () => void {
  const url = `/api/v1/exec/stream?host_id=${encodeURIComponent(hostId)}&command=${encodeURIComponent(command)}&timeout=${timeout}`
  const es = new EventSource(url)
  es.onmessage = (e) => {
    const chunk: StreamChunk = JSON.parse(e.data)
    onChunk(chunk)
    if (chunk.type === 'done') es.close()
  }
  es.onerror = () => {
    onChunk({ type: 'done', exit_code: -1, error: '连接中断' })
    es.close()
  }
  return () => es.close()
}

export async function execBatch(
  command: string,
  opts: { hostIds?: string; tag?: string; timeoutSeconds?: number },
): Promise<ExecResult[]> {
  const res = await fetch('/api/v1/exec/batch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ command, host_ids: opts.hostIds, tag: opts.tag, timeout_seconds: opts.timeoutSeconds ?? 30 }),
  })
  if (!res.ok) throw new Error((await res.json()).error)
  return res.json()
}
