import { api } from '@/shared/api/client'

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
  return api.get('/tasks')
}

export async function getTask(id: string): Promise<Task> {
  return api.get(`/tasks/${id}`)
}

export async function triggerTask(id: string): Promise<{ run_id: string; status: string }> {
  return api.post(`/tasks/${id}/trigger`)
}

export async function listTaskRuns(id: string, limit = 20, offset = 0): Promise<TaskRun[]> {
  return api.get(`/tasks/${id}/runs?limit=${limit}&offset=${offset}`)
}
