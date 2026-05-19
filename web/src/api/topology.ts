import { authHeaders } from './auth'

export interface Topology {
  id: string
  name: string
  notes: string
  created_at: string
  updated_at: string
}

export interface TopologyNode {
  id: string
  topology_id: string
  layer: string
  name: string
  role: string
  host_id?: string
  host_name?: string
  ip?: string
  notes: string
  pos_x: number
  pos_y: number
  created_at: string
  updated_at: string
}

export interface TopologyEdge {
  id: string
  topology_id: string
  from_node: string
  to_node: string
  created_at: string
}

export interface TopologyFull extends Topology {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

const BASE = '/api/v1/topologies'

export async function listTopologies(): Promise<Topology[]> {
  const r = await fetch(BASE, { headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function getTopologyFull(id: string): Promise<TopologyFull> {
  const r = await fetch(`${BASE}/${id}`, { headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function createTopology(name: string, notes = ''): Promise<Topology> {
  const r = await fetch(BASE, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, notes }),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function deleteTopology(id: string): Promise<void> {
  const r = await fetch(`${BASE}/${id}`, { method: 'DELETE', headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
}

export async function createNode(
  topoID: string,
  req: { layer: string; name: string; role?: string; host_id?: string }
): Promise<TopologyNode> {
  const r = await fetch(`${BASE}/${topoID}/nodes`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function updateNode(
  topoID: string,
  nodeID: string,
  req: { layer: string; name: string; role?: string; host_id?: string; notes?: string; pos_x?: number; pos_y?: number }
): Promise<TopologyNode> {
  const r = await fetch(`${BASE}/${topoID}/nodes/${nodeID}`, {
    method: 'PUT',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function deleteNode(topoID: string, nodeID: string): Promise<void> {
  const r = await fetch(`${BASE}/${topoID}/nodes/${nodeID}`, { method: 'DELETE', headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
}

export async function createEdge(topoID: string, fromNode: string, toNode: string): Promise<TopologyEdge> {
  const r = await fetch(`${BASE}/${topoID}/edges`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ from_node: fromNode, to_node: toNode }),
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function deleteEdge(topoID: string, edgeID: string): Promise<void> {
  const r = await fetch(`${BASE}/${topoID}/edges/${edgeID}`, { method: 'DELETE', headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
}

export async function importYAML(topoID: string, yamlText: string): Promise<TopologyFull> {
  const r = await fetch(`${BASE}/${topoID}/import`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/x-yaml' },
    body: yamlText,
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function exportYAML(topoID: string): Promise<void> {
  const r = await fetch(`${BASE}/${topoID}/export`, { headers: authHeaders() })
  if (!r.ok) throw new Error(await r.text())
  const blob = await r.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = r.headers.get('Content-Disposition')?.match(/filename="(.+)"/)?.[1] ?? 'topology.yaml'
  a.click()
  URL.revokeObjectURL(url)
}
