import { authHeaders } from './auth'

export interface Task {
  id: string
  name: string
  goal: string
  host_ids: string[]
  schedule: string
  notify_mode: string
  run_retention_days: number
  timeout_minutes: number
  status: string
  created_at: string
  updated_at: string
  source_conv_id: string
}

export interface TaskRun {
  id: string
  task_id: string
  started_at: string
  finished_at?: string
  status: string
  raw_output: string
  summary: string
  alerted: boolean
}

export async function listTasks(): Promise<Task[]> {
  const res = await fetch('/api/v1/tasks', { headers: authHeaders() })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getTask(id: string): Promise<Task> {
  const res = await fetch(`/api/v1/tasks/${id}`, { headers: authHeaders() })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function triggerTask(id: string): Promise<{ run_id: string; status: string }> {
  const res = await fetch(`/api/v1/tasks/${id}/trigger`, {
    method: 'POST',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function listTaskRuns(id: string, limit = 20, offset = 0): Promise<TaskRun[]> {
  const res = await fetch(`/api/v1/tasks/${id}/runs?limit=${limit}&offset=${offset}`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
