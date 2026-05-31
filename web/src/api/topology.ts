import { api } from '@/shared/api/client'

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

export async function listTopologies(): Promise<Topology[]> {
  return api.get<Topology[]>('/topologies')
}

export async function getTopologyFull(id: string): Promise<TopologyFull> {
  return api.get<TopologyFull>(`/topologies/${id}`)
}

export async function createTopology(name: string, notes = ''): Promise<Topology> {
  return api.post<Topology>('/topologies', { name, notes })
}

export async function deleteTopology(id: string): Promise<void> {
  return api.delete<void>(`/topologies/${id}`, undefined, { responseType: 'void' })
}

export async function createNode(
  topoID: string,
  req: { layer: string; name: string; role?: string; host_id?: string }
): Promise<TopologyNode> {
  return api.post<TopologyNode>(`/topologies/${topoID}/nodes`, req)
}

export async function updateNode(
  topoID: string,
  nodeID: string,
  req: { layer: string; name: string; role?: string; host_id?: string; notes?: string; pos_x?: number; pos_y?: number }
): Promise<TopologyNode> {
  return api.put<TopologyNode>(`/topologies/${topoID}/nodes/${nodeID}`, req)
}

export async function deleteNode(topoID: string, nodeID: string): Promise<void> {
  return api.delete<void>(`/topologies/${topoID}/nodes/${nodeID}`, undefined, { responseType: 'void' })
}

export async function createEdge(topoID: string, fromNode: string, toNode: string): Promise<TopologyEdge> {
  return api.post<TopologyEdge>(`/topologies/${topoID}/edges`, { from_node: fromNode, to_node: toNode })
}

export async function deleteEdge(topoID: string, edgeID: string): Promise<void> {
  return api.delete<void>(`/topologies/${topoID}/edges/${edgeID}`, undefined, { responseType: 'void' })
}

export async function importYAML(topoID: string, yamlText: string): Promise<TopologyFull> {
  return api.post<TopologyFull>(`/topologies/${topoID}/import`, yamlText, {
    headers: { 'Content-Type': 'application/x-yaml' }
  })
}

export async function exportYAML(topoID: string): Promise<void> {
  const blob = await api.download(`/topologies/${topoID}/export`)
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `topology-${topoID}.yaml`
  a.click()
  URL.revokeObjectURL(url)
}
