---
name: target-panel-unknown-state
description: Add 'unknown' default state for host status in target panel with timeout mechanism
created: 2026-06-01
---

# Target Panel Unknown State Design

## Overview

Add an `unknown` state as the default host status in the target panel. Hosts start in `unknown` state and transition to actual states (online/offline/executing/success/failed) when status updates arrive. Implement a 5-minute timeout mechanism to reset stale states back to `unknown`.

## Problem

Currently, hosts in the target panel immediately show colored status (online/offline) without distinguishing between:
- Hosts with confirmed status
- Hosts waiting for initial status report
- Hosts with stale/outdated status

This creates ambiguity about data freshness and connection state.

## Solution

### State Architecture

Implement a two-layer state model:

**Physical Layer (Global, Cross-Session):**
- `unknown`: Initial state or timeout (>5min no update)
- `online`: Host is reachable
- `offline`: Host is unreachable

**Task Layer (Session-Local):**
- `executing`: Running task
- `success`: Task completed successfully
- `failed`: Task failed

**Display Priority:**
```
Task Layer > Physical Layer > unknown
```

### Type Definitions

```typescript
// Status types
export type PhysicalStatus = 'unknown' | 'online' | 'offline'
export type TaskStatus = 'executing' | 'success' | 'failed'
export type DeviceStatusValue = PhysicalStatus | TaskStatus

// Extended DeviceStatus
export interface DeviceStatus {
  id: string
  name: string
  ip: string
  vendor: string
  status: DeviceStatusValue
  detail?: string
}

// Internal state storage
interface PhysicalState {
  status: PhysicalStatus
  lastUpdate: number  // timestamp in ms
}

interface TaskState {
  status: TaskStatus
  timestamp: number
}
```

### Data Layer

**New Composable: `useDeviceStates.ts`**

Global singleton managing physical layer states:

```typescript
const physicalStates = ref(new Map<string, PhysicalState>())

export function useDeviceStates() {
  function initPhysicalStates(hosts: Host[]) {
    hosts.forEach(h => {
      if (!physicalStates.value.has(h.id)) {
        physicalStates.value.set(h.id, {
          status: 'unknown',
          lastUpdate: Date.now()
        })
      }
    })
  }

  function updatePhysicalState(hostId: string, status: PhysicalStatus) {
    physicalStates.value.set(hostId, {
      status,
      lastUpdate: Date.now()
    })
  }

  function getPhysicalState(hostId: string): PhysicalStatus {
    const state = physicalStates.value.get(hostId)
    if (!state) return 'unknown'
    
    // Lazy timeout check: 5 minutes
    const elapsed = Date.now() - state.lastUpdate
    if (elapsed > 5 * 60 * 1000) return 'unknown'
    
    return state.status
  }

  return {
    initPhysicalStates,
    updatePhysicalState,
    getPhysicalState
  }
}
```

**ChatView Task Layer:**

Session-local task states, cleared on conversation switch:

```typescript
const taskStates = ref(new Map<string, TaskState>())

function updateTaskState(hostId: string, status: TaskStatus) {
  taskStates.value.set(hostId, { status, timestamp: Date.now() })
}

function clearTaskStates() {
  taskStates.value.clear()
}
```

### State Merging

**Display Logic in ChatView:**

```typescript
const { getPhysicalState } = useDeviceStates()

const devices = computed<DeviceStatus[]>(() => {
  return allHosts.value.map(h => {
    // Priority 1: Task layer
    const taskState = taskStates.value.get(h.id)
    if (taskState) {
      return {
        id: h.id,
        name: h.name,
        ip: h.ip,
        vendor: h.vendor || '',
        status: taskState.status,
        detail: undefined
      }
    }
    
    // Priority 2: Physical layer (with timeout check)
    return {
      id: h.id,
      name: h.name,
      ip: h.ip,
      vendor: h.vendor || '',
      status: getPhysicalState(h.id),
      detail: undefined
    }
  })
})
```

### Event Handling

**SSE `device_status_update` Dispatch:**

```typescript
function onDeviceStatusUpdate(hostName: string, status: DeviceStatusValue) {
  const host = allHosts.value.find(h => h.name === hostName)
  if (!host) return
  
  // Route to appropriate layer
  if (['executing', 'success', 'failed'].includes(status)) {
    updateTaskState(host.id, status as TaskStatus)
  } else {
    updatePhysicalState(host.id, status as PhysicalStatus)
  }
}
```

**Conversation Switch:**

```typescript
async function selectConversation(convId: string) {
  clearTaskStates()  // Clear task layer, physical layer persists
  // ... load conversation
}
```

### UI Updates

**Visual Style for `unknown`:**
- Color: `#6e6e6e` (medium gray)
- Border: `1px dashed #888`
- No animation

**CSS:**

```css
.hc-unknown {
  background: #6e6e6e;
  border: 1px dashed #888;
  box-sizing: border-box;
}
```

**Status Color Function:**

```typescript
function statusColor(hostId: string): string {
  const d = props.devices.find(x => x.id === hostId)
  if (!d) return 'var(--muted)'
  
  switch (d.status) {
    case 'online': case 'success': return 'var(--green)'
    case 'executing': return 'var(--yellow)'
    case 'failed': return 'var(--red)'
    case 'offline': return '#3a3a3a'
    case 'unknown': return '#6e6e6e'
    default: return 'var(--muted)'
  }
}
```

**Stats Bar Update:**

Add `unknown` count to statistics:

```typescript
const stats = computed(() => {
  const s = { online: 0, offline: 0, executing: 0, failed: 0, unknown: 0 }
  for (const d of props.devices) {
    if (d.status === 'online' || d.status === 'success') s.online++
    else if (d.status === 'offline') s.offline++
    else if (d.status === 'executing') s.executing++
    else if (d.status === 'failed') s.failed++
    else if (d.status === 'unknown') s.unknown++
  }
  return s
})
```

**Template:**

```html
<div class="stats-bar">
  <span class="stat"><span class="sdot" style="background:var(--green)"></span>{{ stats.online }}</span>
  <span class="stat"><span class="sdot" style="background:#6e6e6e"></span>{{ stats.unknown }}</span>
  <span class="stat"><span class="sdot" style="background:#3a3a3a"></span>{{ stats.offline }}</span>
  <span class="stat"><span class="sdot" style="background:var(--yellow)"></span>{{ stats.executing }}</span>
  <span class="stat"><span class="sdot" style="background:var(--red)"></span>{{ stats.failed }}</span>
</div>
```

### Timeout Mechanism

**Lazy Evaluation:**
- No timers or intervals
- Timeout check happens on-demand during `getPhysicalState()` calls
- Triggered by: rendering, user interaction, state updates

**Timeout Duration:**
- 5 minutes (300,000 ms)
- Fixed, not configurable

**Behavior:**
- If `Date.now() - lastUpdate > 5min`, return `unknown`
- Does not proactively update UI; waits for next render cycle
- Acceptable: 5-minute timeout does not require second-level precision

## State Transitions

```
Initial Load:
  allHosts → physicalStates (all 'unknown')

SSE Event (physical):
  unknown/online/offline → updatePhysicalState() → lastUpdate refreshed

SSE Event (task):
  executing/success/failed → updateTaskState() → overlays physical layer

Timeout:
  any physical state + 5min elapsed → 'unknown' (lazy check)

Conversation Switch:
  taskStates.clear() → display falls back to physical layer

New Host Added:
  initPhysicalStates() → new host starts as 'unknown'
```

## Files to Modify

1. **`web/src/composables/useDeviceStates.ts`** (new)
   - Physical layer state management
   - Timeout logic

2. **`web/src/views/ChatView.vue`**
   - Import `useDeviceStates`
   - Add task layer state management
   - Update `devices` computed
   - Update `onDeviceStatusUpdate` handler
   - Clear task states on conversation switch

3. **`web/src/components/TargetPanel.vue`**
   - Update `DeviceStatus` type export
   - Update `statusColor()` function
   - Update `stats` computed
   - Add CSS for `.hc-unknown`
   - Update stats bar template

## Testing Scenarios

1. **Initial Load:**
   - All hosts show gray dashed cells
   - Stats bar shows all hosts in `unknown` count

2. **Status Update:**
   - SSE event arrives → cell changes to colored status
   - Stats update accordingly

3. **Task Execution:**
   - Host shows `executing` (yellow, pulsing)
   - Transitions to `success` (green flash) or `failed` (red shake)

4. **Conversation Switch:**
   - Task states clear
   - Hosts revert to physical layer (online/offline/unknown)

5. **Timeout:**
   - Wait 5+ minutes without updates
   - Next render shows host as `unknown`

6. **New Host:**
   - Add host via UI
   - Immediately shows as `unknown`
   - Transitions when first status arrives

## Non-Goals

- Configurable timeout duration
- Proactive timeout notifications (no timers)
- Separate timeout values for different state types
- Backend changes (frontend-only feature)
