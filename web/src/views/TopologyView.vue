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
    </aside>

    <!-- Center: canvas + toolbar -->
    <div class="topo-canvas-wrap">
      <div v-if="activeTopo" class="topo-toolbar">
        <span class="topo-toolbar-name">{{ activeTopo.name }}</span>
        <div class="topo-toolbar-sep"></div>
        <button class="topo-btn-sm" @click="openAddNode">＋ 添加节点</button>
        <button class="topo-btn-sm" :class="{ 'topo-btn-active': linkMode }" @click="toggleLinkMode">🔗 连线</button>
        <button class="topo-btn-sm" @click="showImportYaml = true">⬆ 导入 YAML</button>
        <button class="topo-btn-sm" @click="doExportYaml">⬇ 导出 YAML</button>
        <div style="flex:1"></div>
        <button class="topo-btn-sm topo-btn-danger" @click="doDeleteTopo">删除拓扑</button>
      </div>
      <div v-if="loadError" class="topo-canvas-error">{{ loadError }}</div>
      <div v-else-if="!activeTopo" class="topo-canvas-empty">选择或创建一个拓扑</div>
      <div v-else class="topo-cy-wrap">
        <div class="topo-layer-axis" v-if="layerAxisItems.length > 0">
          <div
            v-for="item in layerAxisItems"
            :key="item.layer"
            class="topo-layer-label"
            :style="{ top: item.top + 'px', color: item.color }"
          >{{ item.layer }}</div>
        </div>
        <div ref="cyContainer" class="topo-cy"></div>
      </div>
    </div>

    <!-- Right: node detail -->
    <aside class="topo-detail" :class="{ visible: !!activeNode }">
      <template v-if="activeNode">
        <div class="topo-detail-title">{{ activeNode.host_name || activeNode.name }}</div>
        <div v-if="activeNode.host_name" class="topo-detail-sub">{{ activeNode.name }}</div>
        <div class="topo-detail-row"><span class="topo-detail-label">IP</span><span>{{ activeNode.ip || '—' }}</span></div>
        <div class="topo-detail-row"><span class="topo-detail-label">角色</span><span>{{ activeNode.role || '—' }}</span></div>
        <div class="topo-detail-row"><span class="topo-detail-label">层</span><span>{{ activeNode.layer || '—' }}</span></div>
        <div class="topo-detail-section">上游</div>
        <div v-for="n in upstreamNodes" :key="n.id" class="topo-detail-neighbor">{{ n.host_name || n.name }}</div>
        <div v-if="upstreamNodes.length === 0" class="topo-detail-empty">无</div>
        <div class="topo-detail-section">下游</div>
        <div v-for="n in downstreamNodes" :key="n.id" class="topo-detail-neighbor">{{ n.host_name || n.name }}</div>
        <div v-if="downstreamNodes.length === 0" class="topo-detail-empty">无</div>
        <div class="topo-detail-actions">
          <button class="topo-btn-sm" @click="openEditNode">编辑</button>
          <button class="topo-btn-sm topo-btn-danger" @click="doDeleteNode">删除</button>
        </div>
      </template>
    </aside>

    <!-- ── Modal: 新建拓扑 ── -->
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

    <!-- ── Modal: 添加/编辑节点 ── -->
    <div v-if="showNodeForm" class="topo-dialog-overlay" @click.self="showNodeForm = false">
      <div class="topo-dialog topo-dialog-wide">
        <div class="topo-dialog-title">{{ editingNode ? '编辑节点' : '添加节点' }}</div>
        <div class="topo-form-row">
          <label class="topo-form-label">节点名称</label>
          <input v-model="nodeForm.name" class="topo-input" placeholder="例：sw-core-01" />
        </div>
        <div class="topo-form-row">
          <label class="topo-form-label">角色</label>
          <input v-model="nodeForm.role" class="topo-input" placeholder="例：switch、router、server（可选）" />
        </div>
        <div class="topo-form-row">
          <label class="topo-form-label">层</label>
          <input v-model="nodeForm.layer" class="topo-input" placeholder="例：核心层、接入层（可选）" />
        </div>
        <div class="topo-form-row">
          <label class="topo-form-label">绑定主机 <span class="topo-form-opt">（可选）</span></label>
          <select v-model="nodeForm.host_id" class="topo-input">
            <option value="">— 不绑定 —</option>
            <option v-for="h in hosts" :key="h.id" :value="h.id">{{ h.name }} ({{ h.ip }})</option>
          </select>
          <span class="topo-form-hint">绑定后节点显示主机名和 IP，可通过智能运维直接操作</span>
        </div>
        <div v-if="dialogError" class="topo-dialog-error">{{ dialogError }}</div>
        <div class="topo-dialog-actions">
          <button class="topo-btn-secondary" @click="showNodeForm = false">取消</button>
          <button class="topo-btn-primary" :disabled="saving" @click="doSaveNode">{{ editingNode ? '保存' : '创建节点' }}</button>
        </div>
      </div>
    </div>

    <!-- ── Modal: 导入 YAML ── -->
    <div v-if="showImportYaml" class="topo-dialog-overlay" @click.self="showImportYaml = false">
      <div class="topo-dialog topo-dialog-wide">
        <div class="topo-dialog-title">导入 YAML</div>
        <div class="topo-tab-bar">
          <div class="topo-tab" :class="{ active: yamlTab === 0 }" @click="yamlTab = 0">粘贴 YAML</div>
          <div class="topo-tab" :class="{ active: yamlTab === 1 }" @click="yamlTab = 1">格式说明</div>
        </div>
        <div v-if="yamlTab === 0">
          <textarea v-model="yamlText" class="topo-input topo-textarea" placeholder="粘贴拓扑 YAML…"></textarea>
          <div class="topo-form-hint">导入为增量操作：已存在的同名节点不会重复创建，边也会自动去重。</div>
        </div>
        <div v-else class="topo-yaml-doc">
          <pre class="topo-yaml-example">name: &lt;拓扑名称&gt;          # 可选，不影响导入目标

devices:                   # 节点列表
  - name: sw-core-01       # 节点名称（唯一键）
    layer: 核心层           # 层名称，相同名称颜色相同，可选
    role: switch            # 角色，可选
    ip: 10.0.0.1            # 用于自动匹配已有主机，可选
    upstream:               # 上游节点名称列表，可选
      - internet-gw</pre>
          <div class="topo-form-hint" style="margin-top:10px">
            • <code>layer</code> 相同名称自动分配相同颜色，无需预先创建<br>
            • <code>ip</code> 字段会与主机管理中的 IP 自动匹配并绑定主机<br>
            • <code>upstream</code> 定义有向边（上游 → 当前节点）<br>
            • 重复导入安全，不会产生重复数据
          </div>
        </div>
        <div v-if="dialogError" class="topo-dialog-error">{{ dialogError }}</div>
        <div class="topo-dialog-actions">
          <button class="topo-btn-secondary" @click="showImportYaml = false">取消</button>
          <button class="topo-btn-primary" :disabled="saving || yamlTab === 1" @click="doImportYaml">导入</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import cytoscape from 'cytoscape'
import dagre from 'cytoscape-dagre'
import type { TopologyFull, Topology, TopologyNode } from '../api/topology'
import {
  listTopologies, getTopologyFull, createTopology, deleteTopology,
  createNode, updateNode, deleteNode, createEdge, deleteEdge, importYAML, exportYAML,
} from '../api/topology'
import type { Host } from '../api/hosts'
import { listHosts } from '../api/hosts'

cytoscape.use(dagre)

const topologies = ref<Topology[]>([])
const activeTopo = ref<TopologyFull | null>(null)
const activeNode = ref<TopologyNode | null>(null)
const cyContainer = ref<HTMLElement | null>(null)
const loadError = ref('')
let cy: cytoscape.Core | null = null

// new-topo dialog
const showCreate = ref(false)
const newName = ref('')

// shared dialog state
const saving = ref(false)
const dialogError = ref('')

const showNodeForm = ref(false)
const editingNode = ref<TopologyNode | null>(null)
const nodeForm = ref({ name: '', role: '', layer: '', host_id: '' })
const hosts = ref<Host[]>([])

// import YAML modal
const showImportYaml = ref(false)
const yamlTab = ref(0)
const yamlText = ref('')

// link mode (edge creation)
const linkMode = ref(false)
const linkSource = ref<string | null>(null)

// layer axis
interface LayerAxisItem { layer: string; color: string; top: number }
const layerAxisItems = ref<LayerAxisItem[]>([])

let themeObserver: MutationObserver | null = null

onBeforeUnmount(() => {
  if (cy) { cy.destroy(); cy = null }
  themeObserver?.disconnect()
})

onMounted(async () => {
  themeObserver = new MutationObserver(() => { if (activeTopo.value) renderGraph() })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['style'] })
  topologies.value = await listTopologies()
  if (topologies.value.length > 0) await selectTopo(topologies.value[0])
})

async function selectTopo(t: Topology) {
  activeNode.value = null
  loadError.value = ''
  try {
    activeTopo.value = await getTopologyFull(t.id)
    await nextTick()
    renderGraph()
  } catch (e: any) {
    loadError.value = e.message ?? 'Failed to load topology'
  }
}

async function doCreate() {
  if (!newName.value.trim()) return
  try {
    const t = await createTopology(newName.value.trim())
    topologies.value.push(t)
    showCreate.value = false
    newName.value = ''
    await selectTopo(t)
  } catch (e: any) {
    loadError.value = e.message ?? 'Failed to create topology'
  }
}

async function doDeleteTopo() {
  if (!activeTopo.value) return
  if (!confirm(`确认删除拓扑「${activeTopo.value.name}」？此操作不可撤销。`)) return
  try {
    await deleteTopology(activeTopo.value.id)
    topologies.value = topologies.value.filter(t => t.id !== activeTopo.value!.id)
    activeTopo.value = null
    activeNode.value = null
    if (cy) { cy.destroy(); cy = null }
  } catch (e: any) {
    loadError.value = e.message ?? 'Failed to delete topology'
  }
}

async function openAddNode() {
  nodeForm.value = { name: '', role: '', layer: '', host_id: '' }
  editingNode.value = null
  dialogError.value = ''
  if (hosts.value.length === 0) hosts.value = await listHosts()
  showNodeForm.value = true
}

function openEditNode() {
  if (!activeNode.value) return
  nodeForm.value = {
    name: activeNode.value.name,
    role: activeNode.value.role,
    layer: activeNode.value.layer,
    host_id: activeNode.value.host_id ?? '',
  }
  editingNode.value = activeNode.value
  dialogError.value = ''
  showNodeForm.value = true
}

async function doSaveNode() {
  if (!activeTopo.value || !nodeForm.value.name.trim()) return
  saving.value = true
  dialogError.value = ''
  try {
    const req = {
      layer: nodeForm.value.layer,
      name: nodeForm.value.name.trim(),
      role: nodeForm.value.role,
      host_id: nodeForm.value.host_id || undefined,
    }
    if (editingNode.value) {
      await updateNode(activeTopo.value.id, editingNode.value.id, req)
    } else {
      await createNode(activeTopo.value.id, req)
    }
    showNodeForm.value = false
    activeTopo.value = await getTopologyFull(activeTopo.value.id)
    activeNode.value = null
    await nextTick()
    renderGraph()
  } catch (e: any) {
    dialogError.value = e.message ?? 'Failed to save node'
  } finally {
    saving.value = false
  }
}

async function doDeleteNode() {
  if (!activeTopo.value || !activeNode.value) return
  if (!confirm(`确认删除节点「${activeNode.value.name}」？`)) return
  try {
    await deleteNode(activeTopo.value.id, activeNode.value.id)
    activeTopo.value = await getTopologyFull(activeTopo.value.id)
    activeNode.value = null
    await nextTick()
    renderGraph()
  } catch (e: any) {
    loadError.value = e.message ?? 'Failed to delete node'
  }
}

async function doExportYaml() {
  if (!activeTopo.value) return
  try {
    await exportYAML(activeTopo.value.id)
  } catch (e: any) {
    loadError.value = e.message ?? 'Export failed'
  }
}

function toggleLinkMode() {
  linkMode.value = !linkMode.value
  linkSource.value = null
  if (cy) cy.container()!.style.cursor = linkMode.value ? 'crosshair' : ''
}

async function doImportYaml() {
  if (!activeTopo.value || !yamlText.value.trim()) return
  saving.value = true
  dialogError.value = ''
  const topoRef = activeTopo.value
  try {
    await importYAML(topoRef.id, yamlText.value)
    showImportYaml.value = false
    yamlText.value = ''
    await selectTopo(topoRef)
  } catch (e: any) {
    dialogError.value = e.message ?? 'Import failed'
  } finally {
    saving.value = false
  }
}

function layerColor(layer: string): string {
  if (!layer) return '#374151'
  const palette = ['#3b82f6', '#10b981', '#8b5cf6', '#f59e0b', '#ef4444', '#ec4899', '#06b6d4', '#f97316']
  let h = 0
  for (let i = 0; i < layer.length; i++) h = (h * 31 + layer.charCodeAt(i)) >>> 0
  return palette[h % palette.length]
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

const upstreamNodes = computed(() => activeNode.value ? upstreamOf(activeNode.value.id) : [])
const downstreamNodes = computed(() => activeNode.value ? downstreamOf(activeNode.value.id) : [])

function renderGraph() {
  if (!cyContainer.value || !activeTopo.value) return
  if (cy) { cy.destroy(); cy = null }

  const topo = activeTopo.value
  const cs = getComputedStyle(document.documentElement)
  const surfaceColor = cs.getPropertyValue('--surface').trim() || '#ffffff'
  const textColor = cs.getPropertyValue('--text').trim() || '#111827'
  const borderColor = cs.getPropertyValue('--border').trim() || '#d8dce8'
  const bgColor = cs.getPropertyValue('--bg').trim() || '#f0f2f8'

  const elements: cytoscape.ElementDefinition[] = []

  for (const node of topo.nodes) {
    const color = layerColor(node.layer)
    const bound = !!node.host_id
    const label = (node.host_name || node.name).length > 12
      ? (node.host_name || node.name).slice(0, 12) + '…'
      : (node.host_name || node.name)
    const hasPos = node.pos_x !== 0 || node.pos_y !== 0
    elements.push({
      data: { id: node.id, label, bound, color, nodeRef: node },
      ...(hasPos ? { position: { x: node.pos_x, y: node.pos_y } } : {}),
    })
  }
  const allHavePos = topo.nodes.length > 0 && topo.nodes.every(n => n.pos_x !== 0 || n.pos_y !== 0)

  for (const edge of topo.edges) {
    const fromNode = topo.nodes.find(n => n.id === edge.from_node)
    const color = fromNode ? layerColor(fromNode.layer) : borderColor
    elements.push({
      data: { id: edge.id, source: edge.from_node, target: edge.to_node, color },
    })
  }

  cy = cytoscape({
    container: cyContainer.value,
    elements,
    style: [
      {
        selector: 'node',
        style: {
          'background-color': (ele: any) => ele.data('bound') ? ele.data('color') : surfaceColor,
          'border-color': (ele: any) => ele.data('color'),
          'border-width': (ele: any) => ele.data('bound') ? 0 : 2,
          'label': 'data(label)',
          'color': (ele: any) => ele.data('bound') ? '#ffffff' : textColor,
          'font-size': 11,
          'font-weight': '500',
          'text-valign': 'center',
          'text-halign': 'center',
          'width': 110,
          'height': 38,
          'shape': 'roundrectangle',
          'text-outline-width': 0,
        },
      },
      {
        selector: 'edge',
        style: {
          'line-color': 'data(color)',
          'target-arrow-color': 'data(color)',
          'target-arrow-shape': 'triangle',
          'curve-style': 'bezier',
          'line-style': 'solid',
          'opacity': 0.75,
          'width': 1.5,
        },
      },
      {
        selector: 'node:selected',
        style: { 'border-width': 3, 'border-color': cs.getPropertyValue('--primary').trim() || '#6366f1' },
      },
    ],
    layout: allHavePos
      ? { name: 'preset' }
      : { name: 'dagre', rankDir: 'TB', nodeSep: 50, rankSep: 70 } as any,
  })

  const updateLayerAxis = () => {
    if (!cy) return
    const pan = cy.pan()
    const zoom = cy.zoom()
    const layerMap = new Map<string, { color: string; ys: number[] }>()
    cy.nodes().forEach((n: any) => {
      const nodeRef = n.data('nodeRef') as TopologyNode
      const layer = nodeRef.layer || ''
      if (!layer) return
      const renderedY = n.position().y * zoom + pan.y
      if (!layerMap.has(layer)) layerMap.set(layer, { color: layerColor(layer), ys: [] })
      layerMap.get(layer)!.ys.push(renderedY)
    })
    const items: LayerAxisItem[] = []
    layerMap.forEach(({ color, ys }, layer) => {
      const minY = Math.min(...ys)
      const maxY = Math.max(...ys)
      items.push({ layer, color, top: (minY + maxY) / 2 - 10 })
    })
    items.sort((a, b) => a.top - b.top)
    layerAxisItems.value = items
  }

  cy.on('layoutstop', updateLayerAxis)
  cy.on('pan zoom', updateLayerAxis)
  // trigger after layout runs (dagre is async via requestAnimationFrame)
  setTimeout(updateLayerAxis, 50)

  cy.on('tap', 'node', async (evt) => {
    const node = evt.target
    const nodeId = node.id()
    if (linkMode.value) {
      if (!linkSource.value) {
        linkSource.value = nodeId
        node.style('border-color', '#22c55e')
      } else if (linkSource.value !== nodeId) {
        try {
          await createEdge(activeTopo.value!.id, linkSource.value, nodeId)
          activeTopo.value = await getTopologyFull(activeTopo.value!.id)
          await nextTick()
          renderGraph()
        } catch (e: any) {
          loadError.value = e.message ?? 'Failed to create edge'
        }
        linkSource.value = null
      }
    } else {
      activeNode.value = node.data('nodeRef') as TopologyNode
    }
  })
  cy.on('tap', (evt) => {
    if (evt.target === cy) {
      activeNode.value = null
      if (linkMode.value) linkSource.value = null
    }
  })
  cy.on('cxttap', 'edge', async (evt) => {
    const edgeId = evt.target.id()
    if (!confirm('删除此连线？')) return
    try {
      await deleteEdge(activeTopo.value!.id, edgeId)
      activeTopo.value = await getTopologyFull(activeTopo.value!.id)
      await nextTick()
      renderGraph()
    } catch (e: any) {
      loadError.value = e.message ?? 'Failed to delete edge'
    }
  })
  cy.on('dragfree', 'node', async (evt) => {
    const node = evt.target
    const pos = node.position()
    const nodeRef = node.data('nodeRef') as TopologyNode
    try {
      await updateNode(activeTopo.value!.id, nodeRef.id, {
        layer: nodeRef.layer, name: nodeRef.name, role: nodeRef.role,
        host_id: nodeRef.host_id || undefined,
        pos_x: pos.x, pos_y: pos.y,
      })
      nodeRef.pos_x = pos.x
      nodeRef.pos_y = pos.y
    } catch (e: any) {
      console.warn('Failed to save position:', e)
    }
  })
}
</script>

<style scoped>
.topo-page { display: flex; flex: 1; min-height: 0; overflow: hidden; background: var(--bg); }

.topo-sidebar {
  width: 180px; min-width: 140px; border-right: 1px solid var(--border);
  display: flex; flex-direction: column; overflow-y: auto; padding: 8px 0;
}
.topo-sidebar-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 4px 12px 8px; font-size: 11px; text-transform: uppercase;
  letter-spacing: .06em; color: var(--muted);
}
.topo-add-btn {
  background: none; border: 1px solid var(--border); color: var(--muted);
  border-radius: 4px; width: 20px; height: 20px; cursor: pointer; font-size: 14px;
  display: flex; align-items: center; justify-content: center; padding: 0;
}
.topo-add-btn:hover { border-color: var(--text-sub); color: var(--text); }
.topo-item {
  padding: 6px 12px; font-size: 13px; color: var(--muted); cursor: pointer;
  border-radius: 4px; margin: 0 4px;
}
.topo-item:hover { background: var(--surface); color: var(--text); }
.topo-item.active { background: var(--surface); color: var(--text); }
.topo-empty { padding: 12px; font-size: 12px; color: var(--label); text-align: center; }

.topo-detail {
  width: 0; overflow: hidden; transition: width .2s; border-left: 1px solid var(--border);
  display: flex; flex-direction: column; padding: 0;
}
.topo-detail.visible { width: 220px; padding: 16px; overflow-y: auto; }
.topo-detail-title { font-size: 14px; font-weight: 600; color: var(--text); margin-bottom: 2px; }
.topo-detail-sub { font-size: 12px; color: var(--muted); margin-bottom: 12px; }
.topo-detail-row { display: flex; gap: 8px; font-size: 12px; margin-bottom: 6px; }
.topo-detail-label { color: var(--muted); min-width: 32px; }
.topo-detail-section { font-size: 11px; text-transform: uppercase; letter-spacing: .06em; color: var(--label); margin: 12px 0 4px; }
.topo-detail-neighbor { font-size: 12px; color: var(--text-sub); padding: 2px 0; }
.topo-detail-empty { font-size: 12px; color: var(--label); }

.topo-dialog-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,.6);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.topo-dialog {
  background: var(--surface); border: 1px solid var(--border); border-radius: 8px;
  padding: 20px; min-width: 280px; display: flex; flex-direction: column; gap: 12px;
}
.topo-dialog-title { font-size: 14px; font-weight: 600; color: var(--text); }
.topo-input {
  background: var(--input-bg); border: 1px solid var(--border); border-radius: 4px;
  padding: 6px 10px; color: var(--text); font-size: 13px; outline: none;
}
.topo-input:focus { border-color: var(--border-focus); }
.topo-dialog-actions { display: flex; gap: 8px; justify-content: flex-end; }
.topo-btn-primary {
  background: var(--primary); color: #fff; border: none; border-radius: 4px;
  padding: 6px 14px; font-size: 13px; cursor: pointer;
}
.topo-btn-primary:hover { background: var(--primary-hover); }
.topo-btn-secondary {
  background: none; color: var(--muted); border: 1px solid var(--border); border-radius: 4px;
  padding: 6px 14px; font-size: 13px; cursor: pointer;
}
.topo-btn-secondary:hover { border-color: var(--text-sub); color: var(--text); }

.topo-canvas-wrap { flex: 1; min-width: 0; position: relative; display: flex; flex-direction: column; }
.topo-cy-wrap { flex: 1; min-height: 0; position: relative; display: flex; }
.topo-layer-axis {
  position: absolute; left: 0; top: 0; bottom: 0; width: 72px;
  pointer-events: none; z-index: 10;
}
.topo-layer-label {
  position: absolute; left: 8px; right: 4px;
  font-size: 11px; font-weight: 600; letter-spacing: .04em;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  opacity: 0.85;
}
.topo-cy { flex: 1; min-height: 0; }
.topo-toolbar {
  display: flex; align-items: center; gap: 8px; padding: 8px 14px;
  border-bottom: 1px solid var(--border); background: var(--surface); flex-shrink: 0;
}
.topo-toolbar-name { font-size: 13px; font-weight: 600; color: var(--text); margin-right: 4px; }
.topo-toolbar-sep { width: 1px; height: 20px; background: var(--border); margin: 0 4px; }
.topo-btn-sm {
  background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
  padding: 4px 10px; font-size: 12px; font-weight: 500; color: var(--text-sub); cursor: pointer;
  white-space: nowrap;
}
.topo-btn-sm:hover { background: var(--hover); color: var(--text); }
.topo-btn-active { background: var(--primary) !important; color: #fff !important; border-color: var(--primary) !important; }
.topo-btn-danger { color: var(--red); border-color: rgba(248,113,113,.3); background: none; }
.topo-btn-danger:hover { background: rgba(248,113,113,.08); border-color: var(--red); }
.topo-detail-actions { display: flex; gap: 6px; margin-top: 16px; padding-top: 12px; border-top: 1px solid var(--border); }
.topo-dialog-wide { min-width: 400px; }
.topo-dialog-error { font-size: 12px; color: var(--red); }
.topo-form-row { display: flex; flex-direction: column; gap: 4px; }
.topo-form-label { font-size: 11px; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: .06em; }
.topo-form-opt { font-weight: 400; text-transform: none; color: var(--label); }
.topo-form-hint { font-size: 11px; color: var(--muted); line-height: 1.5; }
.topo-form-hint code { background: var(--input-bg); border: 1px solid var(--border); border-radius: 3px; padding: 1px 4px; }
.topo-tab-bar { display: flex; border-bottom: 1px solid var(--border); margin-bottom: 12px; }
.topo-tab { padding: 5px 12px; font-size: 13px; color: var(--muted); cursor: pointer; border-bottom: 2px solid transparent; margin-bottom: -1px; }
.topo-tab.active { color: var(--primary); border-bottom-color: var(--primary); }
.topo-textarea { resize: vertical; min-height: 160px; font-family: 'SF Mono', Consolas, monospace; font-size: 12px; line-height: 1.6; width: 100%; }
.topo-yaml-example {
  background: var(--input-bg); border: 1px solid var(--border); border-radius: 6px;
  padding: 10px 12px; font-family: 'SF Mono', Consolas, monospace; font-size: 11px;
  color: var(--muted); line-height: 1.7; white-space: pre; overflow-x: auto;
}
.topo-yaml-doc { display: flex; flex-direction: column; gap: 8px; }
</style>
