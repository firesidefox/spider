import { ref, computed, type Ref, type ComputedRef } from 'vue'
import type { Todo } from '../../api/chat'

export interface UseTodoPanelOptions {
  activeConvId: Ref<string | null>
}

export interface UseTodoPanelReturn {
  // State
  allTasks: ComputedRef<Todo[]>
  inProgressTasks: ComputedRef<Todo[]>
  pendingTasks: ComputedRef<Todo[]>
  completedTasks: ComputedRef<Todo[]>
  visiblePending: ComputedRef<Todo[]>
  visibleCompleted: ComputedRef<Todo[]>
  hiddenPendingCount: ComputedRef<number>
  hasTasks: ComputedRef<boolean>
  panelHeader: ComputedRef<string>
  completedFolded: Ref<boolean>
  pendingFolded: Ref<boolean>
  taskElapsed: Ref<Map<number, number>>
  turnUsage: Ref<number | null>

  // Operations
  updateTodoFromEvent(convId: string, task: Todo): void
  loadTodoTasks(convId: string, tasks: Todo[]): void
  setTurnUsage(tokens: number | null): void
  clearAllTimers(): void

  // Utils
  fmtElapsed(seconds: number): string
  fmtTokens(n: number): string
}

export function useTodoPanel(options: UseTodoPanelOptions): UseTodoPanelReturn {
  const { activeConvId } = options

  // State
  const todoTasksMap = ref<Record<string, Map<number, Todo>>>({})
  const taskTimers = ref<Map<number, ReturnType<typeof setInterval>>>(new Map())
  const taskElapsed = ref<Map<number, number>>(new Map())
  const completedFolded = ref(true)
  const pendingFolded = ref(true)
  const turnUsage = ref<number | null>(null)

  // Computed properties
  const allTasks = computed(() =>
    Array.from((todoTasksMap.value[activeConvId.value ?? ''] ?? new Map<number, Todo>()).values())
  )

  const inProgressTasks = computed(() =>
    allTasks.value.filter(t => t.status === 'in_progress').sort((a, b) => a.id - b.id)
  )

  const pendingTasks = computed(() =>
    allTasks.value.filter(t => t.status === 'pending').sort((a, b) => a.id - b.id)
  )

  const completedTasks = computed(() =>
    allTasks.value.filter(t => t.status === 'completed').sort((a, b) => a.id - b.id)
  )

  const visiblePending = computed(() =>
    pendingFolded.value ? pendingTasks.value.slice(0, 2) : pendingTasks.value
  )

  const hiddenPendingCount = computed(() =>
    pendingTasks.value.length - visiblePending.value.length
  )

  const visibleCompleted = computed(() =>
    completedFolded.value ? [] : completedTasks.value
  )

  const hasTasks = computed(() =>
    allTasks.value.length > 0
  )

  const panelHeader = computed(() => {
    const active = inProgressTasks.value[0]
    const tokenSuffix = turnUsage.value !== null
      ? '  ↓ ' + fmtTokens(turnUsage.value)
      : ''
    if (active) {
      const label = active.active_form ?? active.subject
      const elapsed = taskElapsed.value.get(active.id) ?? 0
      return label + '… (' + fmtElapsed(elapsed) + ')' + tokenSuffix
    }
    const total = allTasks.value.length
    const done = completedTasks.value.length
    return `TASKS ${done}/${total}` + tokenSuffix
  })

  // Timer management
  function startTimer(task: Todo) {
    if (taskTimers.value.has(task.id)) return
    const startTime = new Date(task.updated_at).getTime()
    const tick = () => {
      taskElapsed.value.set(task.id, Math.floor((Date.now() - startTime) / 1000))
      taskElapsed.value = new Map(taskElapsed.value)
    }
    tick()
    taskTimers.value.set(task.id, setInterval(tick, 1000))
  }

  function stopTimer(taskId: number) {
    const t = taskTimers.value.get(taskId)
    if (t) { clearInterval(t); taskTimers.value.delete(taskId) }
    taskElapsed.value.delete(taskId)
    taskElapsed.value = new Map(taskElapsed.value)
  }

  function clearAllTimers() {
    taskTimers.value.forEach(t => clearInterval(t))
    taskTimers.value.clear()
    taskElapsed.value.clear()
  }

  // Operations
  function updateTodoFromEvent(convId: string, task: Todo) {
    if (!todoTasksMap.value[convId]) todoTasksMap.value[convId] = new Map()
    todoTasksMap.value[convId].set(task.id, task)
    todoTasksMap.value[convId] = todoTasksMap.value[convId]
    if (task.status === 'in_progress') {
      startTimer(task)
    } else {
      stopTimer(task.id)
    }
  }

  function loadTodoTasks(convId: string, tasks: Todo[]) {
    const taskMap = new Map<number, Todo>()
    for (const t of tasks) taskMap.set(t.id, t)
    todoTasksMap.value[convId] = taskMap
    clearAllTimers()
    turnUsage.value = null
    completedFolded.value = true
    pendingFolded.value = true
    for (const task of taskMap.values()) {
      if (task.status === 'in_progress') startTimer(task)
    }
  }

  function setTurnUsage(tokens: number | null) {
    turnUsage.value = tokens
  }

  // Formatting utilities
  function fmtElapsed(seconds: number): string {
    if (seconds >= 60) {
      const m = Math.floor(seconds / 60)
      const s = seconds % 60
      return `${m}m ${s}s`
    }
    return `${seconds}s`
  }

  function fmtTokens(n: number): string {
    if (n >= 1000) return (n / 1000).toFixed(1) + 'k'
    return String(n)
  }

  return {
    // State
    allTasks,
    inProgressTasks,
    pendingTasks,
    completedTasks,
    visiblePending,
    visibleCompleted,
    hiddenPendingCount,
    hasTasks,
    panelHeader,
    completedFolded,
    pendingFolded,
    taskElapsed,
    turnUsage,

    // Operations
    updateTodoFromEvent,
    loadTodoTasks,
    setTurnUsage,
    clearAllTimers,

    // Utils
    fmtElapsed,
    fmtTokens,
  }
}
