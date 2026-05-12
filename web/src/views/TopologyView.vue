<template>
  <div class="topo-page">
    <!-- Left: topology list -->
    <aside class="topo-sidebar">
      <div class="topo-sidebar-header">
        <span class="topo-sidebar-title">拓扑</span>
        <button class="topo-add-btn" @click="showCreate = true">+</button>
      </div>
      <div
        v-for="t in topologies"
        :key="t.id"
        class="topo-item"
        :class="{ active: activeTopo?.id === t.id }"
        @click="selectTopo(t)"
      >
        {{ t.name }}
      </div>
      <div v-if="topologies.length === 0" class="topo-empty">暂无拓扑</div>

      <!-- Create topology dialog -->
      <div v-if="showCreate" class="topo-dialog-overlay" @click.self="showCreate = false">
        <div class="topo-dialog">
          <div class="topo-dialog-title">新建拓扑</div>
          <input v-model="newName" class="topo-input" placeholder="拓扑名称" @keyup.enter="doCreate" />
          <div class="topo-dialog-actions">
            <button class="topo-btn-secondary" @click="showCreate = false">取消</button>
            <button class="topo-btn-primary" @click="doCreate">创建</button>
          </div>
        </div>
      </div>
    </aside>

    <!-- Center: canvas -->
    <div class="topo-canvas-wrap">
      <div v-if="!activeTopo" class="topo-canvas-empty">选择或创建一个拓扑</div>
      <div v-else ref="cyContainer" class="topo-cy"></div>
    </div>

    <!-- Right: node detail -->
    <aside class="topo-detail" :class="{ visible: !!activeNode }">
      <template v-if="activeNode">
        <div class="topo-detail-title">{{ activeNode.host_name || activeNode.name }}</div>
        <div v-if="activeNode.host_name" class="topo-detail-sub">{{ activeNode.name }}</div>
        <div class="topo-detail-row"><span class="topo-detail-label">IP</span><span>{{ activeNode.ip || '—' }}</span></div>
        <div class="topo-detail-row"><span class="topo-detail-label">角色</span><span>{{ activeNode.role || '—' }}</span></div>
        <div class="topo-detail-row"><span class="topo-detail-label">分组</span><span>{{ groupName(activeNode.group_id) }}</span></div>
        <div class="topo-detail-section">上游</div>
        <div v-for="n in upstreamOf(activeNode.id)" :key="n.id" class="topo-detail-neighbor">{{ n.host_name || n.name }}</div>
        <div v-if="upstreamOf(activeNode.id).length === 0" class="topo-detail-empty">无</div>
        <div class="topo-detail-section">下游</div>
        <div v-for="n in downstreamOf(activeNode.id)" :key="n.id" class="topo-detail-neighbor">{{ n.host_name || n.name }}</div>
        <div v-if="downstreamOf(activeNode.id).length === 0" class="topo-detail-empty">无</div>
      </template>
    </aside>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue'
import cytoscape from 'cytoscape'
import dagre from 'cytoscape-dagre'
import type { TopologyFull, Topology, TopologyNode } from '../api/topology'
import { listTopologies, getTopologyFull, createTopology } from '../api/topology'

cytoscape.use(dagre)

const topologies = ref<Topology[]>([])
const activeTopo = ref<TopologyFull | null>(null)
const activeNode = ref<TopologyNode | null>(null)
const cyContainer = ref<HTMLElement | null>(null)
const showCreate = ref(false)
const newName = ref('')
let cy: cytoscape.Core | null = null

onMounted(async () => {
  topologies.value = await listTopologies()
  if (topologies.value.length > 0) await selectTopo(topologies.value[0])
})

async function selectTopo(t: Topology) {
  activeNode.value = null
  activeTopo.value = await getTopologyFull(t.id)
  await nextTick()
  renderGraph()
}

async function doCreate() {
  if (!newName.value.trim()) return
  const t = await createTopology(newName.value.trim())
  topologies.value.push(t)
  showCreate.value = false
  newName.value = ''
  await selectTopo(t)
}

function groupColor(groupID: string): string {
  const g = activeTopo.value?.groups.find(g => g.id === groupID)
  return g?.color ?? '#3b82f6'
}

function groupName(groupID: string): string {
  return activeTopo.value?.groups.find(g => g.id === groupID)?.name ?? ''
}

function upstreamOf(nodeID: string): TopologyNode[] {
  if (!activeTopo.value) return []
  const fromIDs = activeTopo.value.edges.filter(e => e.to_node === nodeID).map(e => e.from_node)
  return activeTopo.value.nodes.filter(n => fromIDs.includes(n.id))
}

function downstreamOf(nodeID: string): TopologyNode[] {
  if (!activeTopo.value) return []
  const toIDs = activeTopo.value.edges.filter(e => e.from_node === nodeID).map(e => e.to_node)
  return activeTopo.value.nodes.filter(n => toIDs.includes(n.id))
}

function renderGraph() {
  if (!cyContainer.value || !activeTopo.value) return
  if (cy) { cy.destroy(); cy = null }

  const topo = activeTopo.value
  const elements: cytoscape.ElementDefinition[] = []

  for (const node of topo.nodes) {
    const color = groupColor(node.group_id)
    const bound = !!node.host_id
    const label = (node.host_name || node.name).length > 10
      ? (node.host_name || node.name).slice(0, 10) + '…'
      : (node.host_name || node.name)
    elements.push({
      data: { id: node.id, label, bound, color, nodeRef: node },
    })
  }

  for (const edge of topo.edges) {
    const fromNode = topo.nodes.find(n => n.id === edge.from_node)
    const color = fromNode ? groupColor(fromNode.group_id) : '#1f2937'
    const bound = !!fromNode?.host_id
    elements.push({
      data: { id: edge.id, source: edge.from_node, target: edge.to_node, color, bound },
    })
  }

  cy = cytoscape({
    container: cyContainer.value,
    elements,
    style: [
      {
        selector: 'node',
        style: {
          'background-color': (ele: any) => ele.data('bound') ? ele.data('color') : '#1a1a1a',
          'border-color': (ele: any) => ele.data('bound') ? ele.data('color') : '#374151',
          'border-width': 2,
          'label': 'data(label)',
          'color': (ele: any) => ele.data('bound') ? '#fff' : '#374151',
          'font-size': 11,
          'text-valign': 'center',
          'text-halign': 'center',
          'width': 100,
          'height': 36,
          'shape': 'roundrectangle',
        },
      },
      {
        selector: 'edge',
        style: {
          'line-color': 'data(color)',
          'target-arrow-color': 'data(color)',
          'target-arrow-shape': 'triangle',
          'curve-style': 'bezier',
          'line-style': (ele: any) => ele.data('bound') ? 'solid' : 'dashed',
          'opacity': 0.6,
          'width': 1.5,
        },
      },
      {
        selector: 'node:selected',
        style: { 'border-width': 3, 'border-color': '#fff' },
      },
    ],
    layout: { name: 'dagre', rankDir: 'TB', nodeSep: 40, rankSep: 60 } as any,
  })

  cy.on('tap', 'node', (evt) => {
    activeNode.value = evt.target.data('nodeRef') as TopologyNode
  })
  cy.on('tap', (evt) => {
    if (evt.target === cy) activeNode.value = null
  })
}
</script>

<style scoped>
.topo-page { display: flex; flex: 1; min-height: 0; overflow: hidden; background: #0d0d0d; }

.topo-sidebar {
  width: 180px; min-width: 140px; border-right: 1px solid #1f2937;
  display: flex; flex-direction: column; overflow-y: auto; padding: 8px 0;
}
.topo-sidebar-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 4px 12px 8px; font-size: 11px; text-transform: uppercase;
  letter-spacing: .06em; color: #6b7280;
}
.topo-add-btn {
  background: none; border: 1px solid #374151; color: #9ca3af;
  border-radius: 4px; width: 20px; height: 20px; cursor: pointer; font-size: 14px;
  display: flex; align-items: center; justify-content: center; padding: 0;
}
.topo-add-btn:hover { border-color: #6b7280; color: #fff; }
.topo-item {
  padding: 6px 12px; font-size: 13px; color: #9ca3af; cursor: pointer;
  border-radius: 4px; margin: 0 4px;
}
.topo-item:hover { background: #1f2937; color: #fff; }
.topo-item.active { background: #1f2937; color: #fff; }
.topo-empty { padding: 12px; font-size: 12px; color: #4b5563; text-align: center; }

.topo-canvas-wrap { flex: 1; min-width: 0; position: relative; }
.topo-canvas-empty { display: flex; align-items: center; justify-content: center; height: 100%; color: #4b5563; font-size: 14px; }
.topo-cy { width: 100%; height: 100%; }

.topo-detail {
  width: 0; overflow: hidden; transition: width .2s; border-left: 1px solid #1f2937;
  display: flex; flex-direction: column; padding: 0;
}
.topo-detail.visible { width: 220px; padding: 16px; overflow-y: auto; }
.topo-detail-title { font-size: 14px; font-weight: 600; color: #f9fafb; margin-bottom: 2px; }
.topo-detail-sub { font-size: 12px; color: #6b7280; margin-bottom: 12px; }
.topo-detail-row { display: flex; gap: 8px; font-size: 12px; margin-bottom: 6px; }
.topo-detail-label { color: #6b7280; min-width: 32px; }
.topo-detail-section { font-size: 11px; text-transform: uppercase; letter-spacing: .06em; color: #4b5563; margin: 12px 0 4px; }
.topo-detail-neighbor { font-size: 12px; color: #9ca3af; padding: 2px 0; }
.topo-detail-empty { font-size: 12px; color: #374151; }

.topo-dialog-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,.6);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.topo-dialog {
  background: #1a1a1a; border: 1px solid #374151; border-radius: 8px;
  padding: 20px; min-width: 280px; display: flex; flex-direction: column; gap: 12px;
}
.topo-dialog-title { font-size: 14px; font-weight: 600; color: #f9fafb; }
.topo-input {
  background: #0d0d0d; border: 1px solid #374151; border-radius: 4px;
  padding: 6px 10px; color: #f9fafb; font-size: 13px; outline: none;
}
.topo-input:focus { border-color: #3b82f6; }
.topo-dialog-actions { display: flex; gap: 8px; justify-content: flex-end; }
.topo-btn-primary {
  background: #3b82f6; color: #fff; border: none; border-radius: 4px;
  padding: 6px 14px; font-size: 13px; cursor: pointer;
}
.topo-btn-primary:hover { background: #2563eb; }
.topo-btn-secondary {
  background: none; color: #9ca3af; border: 1px solid #374151; border-radius: 4px;
  padding: 6px 14px; font-size: 13px; cursor: pointer;
}
.topo-btn-secondary:hover { border-color: #6b7280; color: #fff; }
</style>
