<template>
  <div class="fullscreen-page hosts-page">
    <!-- 左侧面板 -->
    <aside class="hosts-sidebar">
      <div class="sidebar-toolbar">
        <span class="sidebar-title">主机管理</span>
        <button class="btn btn-primary btn-sm" @click="showAdd = true">+ 添加</button>
      </div>
      <div class="sidebar-search">
        <input v-model="search" class="input" placeholder="搜索主机名 / IP..." />
      </div>
      <div class="sidebar-tags">
        <span class="tag" :class="{ active: !filterTag }" @click="filterTag = ''">全部</span>
        <span v-for="t in allTags" :key="t" class="tag" :class="{ active: filterTag === t }" @click="filterTag = t">{{ t }}</span>
      </div>
      <div class="sidebar-list">
        <div
          v-for="h in filtered" :key="h.id"
          class="host-row"
          :class="{ selected: activeHost?.id === h.id }"
          @click="selectHost(h)"
        >
          <div class="host-row-left">
            <input type="checkbox" v-model="selected" :value="h.id" @click.stop />
            <div class="host-row-info">
              <span class="host-row-name">{{ h.name }}</span>
              <span class="host-row-ip">{{ h.ip }}</span>
            </div>
          </div>
          <div class="host-row-right">
            <span v-for="t in h.tags" :key="t" class="tag small">{{ t }}</span>
          </div>
        </div>
        <div v-if="filtered.length === 0" class="sidebar-empty">暂无主机</div>
      </div>
      <div v-if="selected.length" class="sidebar-bulk">
        已选 {{ selected.length }} 台
        <button class="btn btn-sm" @click="bulkExecSelected">批量执行</button>
        <button class="btn btn-sm btn-danger" @click="bulkDelete">批量删除</button>
      </div>
    </aside>

    <!-- 右侧详情 -->
    <div class="hosts-detail">
      <template v-if="activeHost">
        <div class="detail-topbar">
          <div class="detail-topbar-left">
            <span class="detail-title">{{ activeHost.name }}</span>
            <span v-if="hostSubtitle" class="detail-subtitle">{{ hostSubtitle }}</span>
          </div>
          <div class="detail-topbar-right">
            <button class="btn btn-sm" @click="goExec(activeHost)">▶ 执行</button>
            <button class="btn btn-sm" :disabled="pinging" @click="pingActive">{{ pinging ? '测试中…' : '⚡ 测试' }}</button>
            <button class="btn btn-sm btn-danger" @click="removeHost(activeHost)">删除</button>
          </div>
        </div>
        <!-- tabs -->
        <div class="detail-tabs">
          <button v-for="tab in tabs" :key="tab.key" class="tab-btn" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">{{ tab.label }}</button>
        </div>
        <div class="detail-body">
          <div v-if="pingResult" class="ping-result" :class="pingResult.connected ? 'ping-ok' : 'ping-fail'">
            <span v-if="pingResult.connected">● 已连接 ({{ pingResult.latency_ms }}ms)</span>
            <span v-else>● 连接失败: {{ pingResult.error }}</span>
          </div>

          <!-- 概览 tab -->
          <template v-if="activeTab === 'overview'">
            <!-- 基本信息 section -->
            <div class="section">
              <div class="section-header">
                <span class="section-title">📋 基本信息</span>
                <button v-if="!editingOverview" class="edit-link" @click="startOverviewEdit(activeHost)">编辑</button>
                <div v-else class="section-header-actions">
                  <button class="btn btn-primary btn-sm" :disabled="overviewSaving" @click="saveOverview">{{ overviewSaving ? '保存中…' : '保存' }}</button>
                  <button class="btn btn-sm" @click="cancelOverview">取消</button>
                </div>
              </div>
              <div class="section-body">
                <template v-if="!editingOverview">
                  <div class="info-grid">
                    <div class="info-item"><label>名称</label><div class="value">{{ activeHost.name }}</div></div>
                    <div class="info-item"><label>IP 地址</label><div class="value code">{{ activeHost.ip }}</div></div>
                    <div class="info-item">
                      <label>标签</label>
                      <div class="value">
                        <span v-if="activeHost.tags.length"><span v-for="t in activeHost.tags" :key="t" class="tag small" style="margin-right:4px">{{ t }}</span></span>
                        <span v-else class="value-muted">—</span>
                      </div>
                    </div>
                    <div class="info-item"><label>厂商</label><div class="value" :class="{'value-muted':!activeHost.vendor}">{{ activeHost.vendor || '—' }}</div></div>
                    <div class="info-item"><label>产品型号</label><div class="value" :class="{'value-muted':!activeHost.product_name}">{{ activeHost.product_name || '—' }}</div></div>
                    <div class="info-item"><label>产品版本</label><div class="value" :class="{'value-muted':!activeHost.product_version}">{{ activeHost.product_version || '—' }}</div></div>
                  </div>
                  <div v-if="activeHost.notes" class="notes-row">
                    <div class="info-item"><label>备注</label><div class="value" style="white-space:pre-wrap;font-weight:400">{{ activeHost.notes }}</div></div>
                  </div>
                </template>
                <template v-else>
                  <form class="info-grid" @submit.prevent="saveOverview">
                    <div class="info-item"><label>名称</label><input v-model="overviewForm.name" class="input info-input" required /></div>
                    <div class="info-item"><label>IP 地址</label><input v-model="overviewForm.ip" class="input info-input" required /></div>
                    <div class="info-item"><label>标签</label><input v-model="overviewForm.tagsStr" class="input info-input" placeholder="逗号分隔" /></div>
                    <div class="info-item"><label>厂商</label><input v-model="overviewForm.vendor" class="input info-input" placeholder="可选" /></div>
                    <div class="info-item"><label>产品型号</label><input v-model="overviewForm.product_name" class="input info-input" placeholder="可选" /></div>
                    <div class="info-item"><label>产品版本</label><input v-model="overviewForm.product_version" class="input info-input" placeholder="可选" /></div>
                    <div class="info-item" style="grid-column:1/-1"><label>备注</label><textarea v-model="overviewForm.notes" class="input info-input" rows="2" style="height:auto" /></div>
                  </form>
                </template>
              </div>
            </div>

            <!-- 操作面 section -->
            <div class="section">
              <div class="section-header">
                <span class="section-title">🔌 操作面</span>
                <button class="edit-link" @click="showAddFace = true">+ 添加</button>
              </div>
              <div class="section-body">
                <div v-if="faces.length === 0" class="tab-empty" style="padding:12px 0">暂无操作面</div>
                <div v-for="f in faces" :key="f.id" class="face-card">
                  <div class="face-card-header">
                    <span class="badge" :class="f.type === 'ssh' ? 'badge-ssh' : 'badge-rest'">{{ f.type === 'ssh' ? 'SSH' : 'REST API' }}</span>
                    <span class="face-addr code">{{ f.type === 'ssh' ? `${f.username}@${f.ip}:${f.port}` : f.base_url }}</span>
                    <button class="edit-link" @click="startEditFace(f)">编辑</button>
                  </div>
                  <div class="face-details">
                    <span v-if="f.ssh_auth_type">认证: <b>{{ f.ssh_auth_type }}</b></span>
                    <span v-if="f.rest_auth_type">认证: <b>{{ f.rest_auth_type }}</b></span>
                    <span v-if="f.ssh_legacy" class="badge badge-warn">兼容模式</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- 指纹 section -->
            <div class="section">
              <div class="section-header">
                <span class="section-title">🔍 指纹</span>
                <div v-if="fingerprint" style="display:flex;align-items:center;gap:8px">
                  <span class="badge" :class="'badge-fp-' + fingerprint.status">{{ fingerprint.status }}</span>
                  <span style="font-size:11px;color:var(--muted)">{{ fingerprint.collected_at ? new Date(fingerprint.collected_at).toLocaleString() : '' }}</span>
                </div>
              </div>
              <div class="section-body">
                <div v-if="!fingerprint" class="tab-empty" style="padding:12px 0">暂无指纹信息</div>
                <div v-else class="info-grid">
                  <div v-if="fingerprint.ssh_host_key" class="info-item" style="grid-column:1/-1"><label>SSH Host Key</label><div class="value code" style="font-size:12px;word-break:break-all">{{ fingerprint.ssh_host_key }}</div></div>
                  <div v-if="fingerprint.system_version" class="info-item"><label>系统版本</label><div class="value">{{ fingerprint.system_version }}</div></div>
                  <div v-if="fingerprint.hardware_id" class="info-item"><label>硬件序列号</label><div class="value code" style="font-size:12px">{{ fingerprint.hardware_id }}</div></div>
                  <div v-if="fingerprint.api_signature" class="info-item" style="grid-column:1/-1"><label>API 特征</label><div class="value code" style="font-size:12px">{{ fingerprint.api_signature }}</div></div>
                </div>
              </div>
            </div>

            <!-- 记忆 section -->
            <div class="section">
              <div class="section-header">
                <span class="section-title">🧠 记忆</span>
              </div>
              <div class="section-body">
                <div v-if="memories.length === 0" class="tab-empty" style="padding:4px 0 12px">暂无记忆</div>
                <div v-for="m in memories" :key="m.id" class="memory-item" :class="m.created_by === 'agent' ? 'memory-agent' : ''">
                  <div class="memory-meta">
                    <span class="badge" :class="m.created_by === 'agent' ? 'badge-agent' : 'badge-user'">{{ m.created_by === 'agent' ? 'Agent' : '用户' }}</span>
                    <span class="memory-date">{{ new Date(m.created_at).toLocaleString() }}</span>
                    <button class="btn btn-sm btn-danger" @click="removeMemory(m.id)">删除</button>
                  </div>
                  <div class="memory-content">{{ m.content }}</div>
                </div>
                <div class="memory-add">
                  <textarea v-model="newMemory" class="input" rows="2" placeholder="记录操作经验…" />
                  <button class="btn btn-sm btn-primary" :disabled="!newMemory.trim()" @click="submitMemory">保存</button>
                </div>
              </div>
            </div>
          </template>

          <!-- 操作面 tab -->
          <template v-if="activeTab === 'faces'">
            <div class="faces-header">
              <button class="btn btn-sm btn-primary" @click="showAddFace = true">+ 添加操作面</button>
            </div>
            <div v-if="faces.length === 0" class="tab-empty">暂无操作面</div>
            <div v-for="f in faces" :key="f.id" class="face-card">
              <div class="face-card-header">
                <span class="badge" :class="f.type === 'ssh' ? 'badge-ssh' : 'badge-rest'">{{ f.type === 'ssh' ? 'SSH' : 'REST API' }}</span>
                <span class="face-addr code">{{ f.ip }}:{{ f.port }}</span>
                <div class="face-actions">
                  <button class="btn btn-sm" @click="startEditFace(f)">编辑</button>
                  <button class="btn btn-sm btn-danger" @click="removeFace(f)">删除</button>
                </div>
              </div>
              <div class="face-details">
                <span v-if="f.username">用户: <b>{{ f.username }}</b></span>
                <span v-if="f.ssh_auth_type">认证: <b>{{ f.ssh_auth_type }}</b></span>
                <span v-if="f.rest_auth_type">认证: <b>{{ f.rest_auth_type }}</b></span>
                <span v-if="f.base_url">URL: <b class="code">{{ f.base_url }}</b></span>
                <span v-if="f.ssh_legacy" class="badge badge-warn">兼容模式</span>
              </div>
            </div>
          </template>

          <!-- 指纹 tab -->
          <template v-if="activeTab === 'fingerprint'">
            <div v-if="!fingerprint" class="tab-empty">暂无指纹信息</div>
            <template v-else>
              <div class="detail-field" style="margin-bottom:12px">
                <div class="detail-label">状态</div>
                <span class="badge" :class="'badge-fp-' + fingerprint.status">{{ fingerprint.status }}</span>
              </div>
              <div class="detail-grid">
                <div v-if="fingerprint.system_version" class="detail-field">
                  <div class="detail-label">系统版本</div>
                  <div class="detail-value">{{ fingerprint.system_version }}</div>
                </div>
                <div v-if="fingerprint.hardware_id" class="detail-field">
                  <div class="detail-label">硬件 ID</div>
                  <div class="detail-value code" style="font-size:12px;word-break:break-all">{{ fingerprint.hardware_id }}</div>
                </div>
                <div v-if="fingerprint.ssh_host_key" class="detail-field" style="grid-column:1/-1">
                  <div class="detail-label">SSH Host Key</div>
                  <div class="detail-value code" style="font-size:11px;word-break:break-all">{{ fingerprint.ssh_host_key }}</div>
                </div>
                <div v-if="fingerprint.api_signature" class="detail-field" style="grid-column:1/-1">
                  <div class="detail-label">API Signature</div>
                  <div class="detail-value code" style="font-size:11px;word-break:break-all">{{ fingerprint.api_signature }}</div>
                </div>
              </div>
            </template>
          </template>

          <!-- 记忆 tab -->
          <template v-if="activeTab === 'memories'">
            <div v-if="memories.length === 0" class="tab-empty">暂无记忆</div>
            <div v-for="m in memories" :key="m.id" class="memory-item">
              <div class="memory-meta">
                <span class="badge" :class="m.created_by === 'agent' ? 'badge-agent' : 'badge-user'">{{ m.created_by === 'agent' ? 'Agent' : '用户' }}</span>
                <span class="memory-date">{{ new Date(m.created_at).toLocaleString() }}</span>
                <button class="btn btn-sm btn-danger" @click="removeMemory(m.id)">删除</button>
              </div>
              <div class="memory-content">{{ m.content }}</div>
            </div>
            <div class="memory-add">
              <textarea v-model="newMemory" class="input" rows="2" placeholder="添加记忆…" />
              <button class="btn btn-sm btn-primary" :disabled="!newMemory.trim()" @click="submitMemory">添加</button>
            </div>
          </template>
        </div>
      </template>
      <div v-else class="detail-empty">
        <div class="detail-empty-icon">←</div>
        <div>选择左侧主机查看详情</div>
      </div>
    </div>

    <!-- 添加主机弹窗 -->
    <div v-if="showAdd" class="modal-overlay" @click.self="closeModal">
      <div class="modal">
        <h3>添加主机</h3>
        <form @submit.prevent="submitHost">
          <div class="form-row"><label>名称</label><input v-model="form.name" class="input" required /></div>
          <div class="form-row"><label>IP</label><input v-model="form.ip" class="input" required /></div>
          <div class="form-row"><label>备注</label><textarea v-model="form.notes" class="input" rows="2" /></div>
          <div class="form-row"><label>厂商</label><input v-model="form.vendor" class="input" /></div>
          <div class="form-row"><label>产品型号</label><input v-model="form.product_name" class="input" /></div>
          <div class="form-row"><label>版本</label><input v-model="form.product_version" class="input" /></div>
          <div class="form-row"><label>标签</label><input v-model="form.tagsStr" class="input" placeholder="逗号分隔，如 prod,web" /></div>
          <div class="modal-footer">
            <button type="button" class="btn" @click="closeModal">取消</button>
            <button type="submit" class="btn btn-primary">添加</button>
          </div>
        </form>
      </div>
    </div>

    <!-- 添加操作面弹窗 -->
    <div v-if="showAddFace" class="modal-overlay" @click.self="closeFaceModal">
      <div class="modal">
        <h3>{{ editFaceTarget ? '编辑操作面' : '添加操作面' }}</h3>
        <form @submit.prevent="submitFace">
          <div class="form-row"><label>类型</label>
            <select v-model="faceForm.type" class="input">
              <option value="ssh">SSH</option>
              <option value="restapi">REST API</option>
            </select>
          </div>
          <div class="form-row"><label>IP</label><input v-model="faceForm.ip" class="input" required /></div>
          <div class="form-row"><label>端口</label><input v-model.number="faceForm.port" class="input" type="number" required /></div>
          <template v-if="faceForm.type === 'ssh'">
            <div class="form-row"><label>用户名</label><input v-model="faceForm.username" class="input" /></div>
            <div class="form-row"><label>认证方式</label>
              <select v-model="faceForm.ssh_auth_type" class="input">
                <option value="password">密码</option>
                <option value="key">私钥</option>
                <option value="key_password">私钥+密码</option>
              </select>
            </div>
            <div class="form-row"><label>凭据</label><textarea v-model="faceForm.credential" class="input" rows="2" /></div>
          </template>
          <template v-if="faceForm.type === 'restapi'">
            <div class="form-row"><label>Base URL</label><input v-model="faceForm.base_url" class="input" /></div>
            <div class="form-row"><label>认证方式</label>
              <select v-model="faceForm.rest_auth_type" class="input">
                <option value="none">无</option>
                <option value="bearer">Bearer Token</option>
                <option value="basic">Basic</option>
                <option value="apikey">API Key</option>
              </select>
            </div>
            <div class="form-row"><label>用户名</label><input v-model="faceForm.rest_username" class="input" /></div>
            <div class="form-row"><label>凭据</label><textarea v-model="faceForm.credential" class="input" rows="2" /></div>
          </template>
          <div class="modal-footer">
            <button type="button" class="btn" @click="closeFaceModal">取消</button>
            <button type="submit" class="btn btn-primary">{{ editFaceTarget ? '保存' : '添加' }}</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  listHosts, addHost, updateHost, deleteHost, pingHost,
  listAccessFaces, addAccessFace, updateAccessFace, deleteAccessFace,
  getFingerprint, listMemories, addMemory, deleteMemory,
  type Host, type AccessFace, type Fingerprint, type Memory,
} from '../api/hosts'

const router = useRouter()
const hosts = ref<Host[]>([])
const search = ref('')
const filterTag = ref('')
const selected = ref<string[]>([])
const activeHost = ref<Host | null>(null)
const showAdd = ref(false)
const editTarget = ref<Host | null>(null)
const pinging = ref(false)
const pingResult = ref<{ connected: boolean; latency_ms?: number; error?: string } | null>(null)
let pingTimer: ReturnType<typeof setTimeout> | null = null

const activeTab = ref<'overview' | 'faces' | 'fingerprint' | 'memories'>('overview')
const tabs = [
  { key: 'overview', label: '概览' },
  { key: 'faces', label: '操作面' },
  { key: 'fingerprint', label: '指纹' },
  { key: 'memories', label: '记忆' },
] as const

const faces = ref<AccessFace[]>([])
const fingerprint = ref<Fingerprint | null>(null)
const memories = ref<Memory[]>([])
const newMemory = ref('')
const showAddFace = ref(false)
const editFaceTarget = ref<AccessFace | null>(null)

const editingOverview = ref(false)
const overviewSaving = ref(false)
const overviewForm = ref({ name: '', ip: '', notes: '', vendor: '', product_name: '', product_version: '', tagsStr: '' })

function startOverviewEdit(h: Host) {
  overviewForm.value = { name: h.name, ip: h.ip, notes: h.notes ?? '', vendor: h.vendor ?? '', product_name: h.product_name ?? '', product_version: h.product_version ?? '', tagsStr: h.tags.join(',') }
  editingOverview.value = true
}

function cancelOverview() {
  editingOverview.value = false
}

async function saveOverview() {
  if (!activeHost.value) return
  overviewSaving.value = true
  try {
    const tags = overviewForm.value.tagsStr.split(',').map(t => t.trim()).filter(Boolean)
    const updated = await updateHost(activeHost.value.id, {
      name: overviewForm.value.name,
      ip: overviewForm.value.ip,
      notes: overviewForm.value.notes || undefined,
      vendor: overviewForm.value.vendor || undefined,
      product_name: overviewForm.value.product_name || undefined,
      product_version: overviewForm.value.product_version || undefined,
      tags,
    })
    activeHost.value = { ...activeHost.value, ...updated }
    hosts.value = hosts.value.map(h => h.id === updated.id ? { ...h, ...updated } : h)
    editingOverview.value = false
  } finally {
    overviewSaving.value = false
  }
}

const emptyForm = () => ({ name: '', ip: '', notes: '', vendor: '', product_name: '', product_version: '', tagsStr: '' })
const form = ref(emptyForm())

const emptyFaceForm = () => ({ type: 'ssh' as 'ssh' | 'restapi', ip: '', port: 22, username: '', ssh_auth_type: 'password', credential: '', passphrase: '', ssh_key_id: '', ssh_legacy: false, base_url: '', rest_auth_type: 'none', rest_username: '', header_name: '' })
const faceForm = ref(emptyFaceForm())

const allTags = computed(() => {
  const s = new Set<string>()
  hosts.value.forEach(h => h.tags.forEach(t => s.add(t)))
  return [...s]
})

const hostSubtitle = computed(() => {
  const h = activeHost.value
  if (!h) return ''
  return [h.ip, h.vendor, h.product_name, h.product_version].filter(Boolean).join(' · ')
})

const filtered = computed(() => hosts.value.filter(h => {
  const q = search.value.toLowerCase()
  const matchSearch = !q || h.name.toLowerCase().includes(q) || h.ip.includes(q)
  const matchTag = !filterTag.value || h.tags.includes(filterTag.value)
  return matchSearch && matchTag
}))

async function load() { hosts.value = await listHosts() }

async function selectHost(h: Host) {
  activeHost.value = h
  pingResult.value = null
  activeTab.value = 'overview'
  editingOverview.value = false
  faces.value = []
  fingerprint.value = null
  memories.value = []
  const [f, fp, m] = await Promise.all([
    listAccessFaces(h.id),
    getFingerprint(h.id),
    listMemories(h.id),
  ])
  faces.value = f
  fingerprint.value = fp
  memories.value = m
}

async function pingActive() {
  if (!activeHost.value || pinging.value) return
  pinging.value = true
  pingResult.value = null
  if (pingTimer) clearTimeout(pingTimer)
  try {
    pingResult.value = await pingHost(activeHost.value.id)
  } finally {
    pinging.value = false
    pingTimer = setTimeout(() => { pingResult.value = null }, 5000)
  }
}

function closeModal() {
  showAdd.value = false
  editTarget.value = null
  form.value = emptyForm()
}

async function submitHost() {
  const tags = form.value.tagsStr.split(',').map(t => t.trim()).filter(Boolean)
  const payload = { name: form.value.name, ip: form.value.ip, notes: form.value.notes || undefined, vendor: form.value.vendor || undefined, product_name: form.value.product_name || undefined, product_version: form.value.product_version || undefined, tags }
  await addHost(payload)
  closeModal()
  load()
}

async function removeHost(h: Host) {
  if (!confirm(`确认删除主机 ${h.name}？`)) return
  await deleteHost(h.id)
  if (activeHost.value?.id === h.id) activeHost.value = null
  load()
}

async function bulkDelete() {
  if (!confirm(`确认删除 ${selected.value.length} 台主机？`)) return
  await Promise.all(selected.value.map(id => deleteHost(id)))
  selected.value = []
  if (activeHost.value && !hosts.value.find(h => h.id === activeHost.value!.id)) activeHost.value = null
  load()
}

function goExec(h: Host) { router.push({ path: '/exec', query: { host: h.id } }) }
function bulkExecSelected() { router.push({ path: '/exec', query: { hosts: selected.value.join(',') } }) }

async function submitFace() {
  if (!activeHost.value) return
  const req: Record<string, unknown> = { type: faceForm.value.type, ip: faceForm.value.ip, port: faceForm.value.port, tags: [], knowledge_sources: [] }
  if (faceForm.value.type === 'ssh') {
    req.username = faceForm.value.username || undefined
    req.ssh_auth_type = faceForm.value.ssh_auth_type
    req.credential = faceForm.value.credential || undefined
    req.passphrase = faceForm.value.passphrase || undefined
    req.ssh_key_id = faceForm.value.ssh_key_id || undefined
    req.ssh_legacy = faceForm.value.ssh_legacy
  } else {
    req.base_url = faceForm.value.base_url || undefined
    req.rest_auth_type = faceForm.value.rest_auth_type
    req.rest_username = faceForm.value.rest_username || undefined
    req.credential = faceForm.value.credential || undefined
    req.header_name = faceForm.value.header_name || undefined
  }
  if (editFaceTarget.value) {
    await updateAccessFace(activeHost.value.id, editFaceTarget.value.id, req as Parameters<typeof updateAccessFace>[2])
  } else {
    await addAccessFace(activeHost.value.id, req as Parameters<typeof addAccessFace>[1])
  }
  closeFaceModal()
  faces.value = await listAccessFaces(activeHost.value.id)
}

function startEditFace(face: AccessFace) {
  editFaceTarget.value = face
  faceForm.value = {
    type: face.type,
    ip: face.ip,
    port: face.port,
    username: face.username || '',
    ssh_auth_type: face.ssh_auth_type || 'password',
    credential: '',
    passphrase: '',
    ssh_key_id: face.ssh_key_id || '',
    ssh_legacy: face.ssh_legacy || false,
    base_url: face.base_url || '',
    rest_auth_type: face.rest_auth_type || 'none',
    rest_username: face.rest_username || '',
    header_name: face.header_name || '',
  }
  showAddFace.value = true
}

function closeFaceModal() {
  showAddFace.value = false
  editFaceTarget.value = null
  faceForm.value = emptyFaceForm()
}

async function removeFace(f: AccessFace) {
  if (!activeHost.value || !confirm('确认删除此操作面？')) return
  await deleteAccessFace(activeHost.value.id, f.id)
  faces.value = faces.value.filter(x => x.id !== f.id)
}

async function submitMemory() {
  if (!activeHost.value || !newMemory.value.trim()) return
  const m = await addMemory(activeHost.value.id, newMemory.value.trim())
  memories.value.push(m)
  newMemory.value = ''
}

async function removeMemory(id: number) {
  if (!activeHost.value || !confirm('确认删除此记忆？')) return
  await deleteMemory(activeHost.value.id, id)
  memories.value = memories.value.filter(m => m.id !== id)
}

onMounted(load)
</script>

<style scoped>
.hosts-page { display: flex; flex: 1; min-height: 0; overflow: hidden; }

.hosts-sidebar {
  width: 26%; min-width: 280px; max-width: 380px;
  background: var(--panel); border-right: 1px solid var(--border);
  display: flex; flex-direction: column; flex-shrink: 0; overflow: hidden;
}
.sidebar-toolbar { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px 12px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
.sidebar-title { font-size: 13px; font-weight: 700; color: var(--text); }
.sidebar-search { padding: 10px 12px 8px; flex-shrink: 0; }
.sidebar-tags { display: flex; gap: 6px; flex-wrap: wrap; padding: 0 12px 10px; flex-shrink: 0; }
.sidebar-list { flex: 1; overflow-y: auto; }
.sidebar-empty { color: var(--label); font-size: 13px; padding: 32px 16px; text-align: center; }
.sidebar-bulk { display: flex; gap: 8px; align-items: center; padding: 10px 14px; border-top: 1px solid var(--border); font-size: 13px; color: var(--text-sub); background: rgba(99,102,241,0.06); flex-shrink: 0; }

.host-row { display: flex; align-items: center; justify-content: space-between; padding: 10px 16px; border-bottom: 1px solid var(--border); border-left: 3px solid transparent; cursor: pointer; transition: background 0.1s; gap: 8px; }
.host-row:hover { background: var(--row-hover); }
.host-row.selected { border-left-color: var(--primary); background: rgba(99,102,241,0.1); }
.host-row-left { display: flex; align-items: center; gap: 10px; min-width: 0; }
.host-row-info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.host-row-name { font-size: 14px; font-weight: 500; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.host-row-ip { font-size: 12px; color: var(--label); font-family: 'SF Mono', Consolas, monospace; }
.host-row-right { display: flex; align-items: center; gap: 4px; flex-shrink: 0; flex-wrap: wrap; justify-content: flex-end; }

.hosts-detail { flex: 1; overflow: hidden; min-width: 0; display: flex; flex-direction: column; }
.detail-topbar { display: flex; align-items: center; justify-content: space-between; padding: 12px 20px; border-bottom: 1px solid var(--border); background: var(--surface); flex-shrink: 0; }
.detail-topbar-left { display: flex; align-items: center; gap: 10px; }
.detail-topbar-right { display: flex; gap: 8px; }
.detail-title { font-size: 15px; font-weight: 700; color: var(--text); }

.detail-tabs { display: flex; gap: 0; border-bottom: 1px solid var(--border); background: var(--surface); flex-shrink: 0; }
.tab-btn { padding: 8px 18px; font-size: 13px; font-weight: 500; color: var(--text-sub); background: none; border: none; border-bottom: 2px solid transparent; cursor: pointer; transition: color 0.15s, border-color 0.15s; }
.tab-btn:hover { color: var(--text); }
.tab-btn.active { color: var(--primary); border-bottom-color: var(--primary); }

.detail-body { flex: 1; overflow-y: auto; padding: 20px 24px; }
.detail-empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: var(--muted); font-size: 14px; }
.detail-empty-icon { color: var(--border); font-size: 40px; }
.tab-empty { color: var(--muted); font-size: 13px; padding: 32px 0; text-align: center; }

.section { background: var(--card-bg); border: 1px solid var(--border); border-radius: 10px; overflow: hidden; margin-bottom: 16px; }
.section-header { padding: 10px 16px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border); background: var(--surface); }
.section-title { font-size: 12px; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
.section-header-actions { display: flex; gap: 6px; }
.section-body { padding: 16px; }
.edit-link { font-size: 12px; color: var(--primary); cursor: pointer; background: none; border: none; padding: 0; }
.edit-link:hover { text-decoration: underline; }

.info-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 14px; }
.info-item label { font-size: 11px; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.04em; display: block; margin-bottom: 5px; }
.info-item .value { font-size: 14px; color: var(--text); }
.info-item .value.code { font-family: 'SF Mono', Consolas, monospace; }
.value-muted { color: var(--muted); font-style: italic; }
.info-input { width: 100%; height: 30px; font-size: 13px; padding: 0 8px; }
.notes-row { margin-top: 14px; padding-top: 14px; border-top: 1px solid var(--border); }

.detail-subtitle { font-size: 12px; color: var(--muted); margin-left: 8px; }

.detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.detail-field { background: var(--card-bg); border: 1px solid var(--border); border-radius: 10px; padding: 14px 20px; box-shadow: var(--card-shadow); }
.detail-label { font-size: 11px; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.07em; margin-bottom: 6px; }
.detail-value { font-size: 15px; font-weight: 600; color: var(--text); }
.detail-value.code, .code { font-family: 'SF Mono', Consolas, monospace; }

.ping-result { font-size: 13px; font-weight: 500; padding: 8px 14px; border-radius: 8px; margin-bottom: 14px; }
.ping-ok { background: rgba(34,197,94,0.12); color: #16a34a; }
.ping-fail { background: rgba(239,68,68,0.12); color: #dc2626; }

.faces-header { margin-bottom: 14px; }
.face-card { background: var(--card-bg); border: 1px solid var(--border); border-radius: 10px; padding: 14px 18px; margin-bottom: 10px; }
.face-card-header { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.face-addr { font-size: 13px; color: var(--text); flex: 1; }
.face-actions { display: flex; gap: 6px; }
.face-details { display: flex; gap: 12px; flex-wrap: wrap; font-size: 13px; color: var(--text-sub); }

.memory-item { background: var(--card-bg); border: 1px solid var(--border); border-radius: 10px; padding: 12px 16px; margin-bottom: 10px; }
.memory-meta { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.memory-date { font-size: 12px; color: var(--muted); flex: 1; }
.memory-content { font-size: 13px; color: var(--text); white-space: pre-wrap; }
.memory-add { display: flex; gap: 8px; align-items: flex-end; margin-top: 16px; }
.memory-add .input { flex: 1; resize: vertical; }

.badge { display: inline-flex; align-items: center; padding: 2px 8px; border-radius: 6px; font-size: 11px; font-weight: 600; }
.badge-ssh { background: rgba(99,102,241,0.15); color: var(--primary); }
.badge-rest { background: rgba(16,185,129,0.15); color: #059669; }
.badge-warn { background: rgba(245,158,11,0.15); color: #d97706; }
.badge-agent { background: rgba(99,102,241,0.15); color: var(--primary); }
.badge-user { background: rgba(107,114,128,0.15); color: #6b7280; }
.badge-fp-ok { background: rgba(34,197,94,0.15); color: #16a34a; }
.badge-fp-changed { background: rgba(239,68,68,0.15); color: #dc2626; }
.badge-fp-unverified { background: rgba(107,114,128,0.15); color: #6b7280; }

.tag { display: inline-flex; align-items: center; padding: 2px 8px; border-radius: 6px; font-size: 11px; font-weight: 500; background: rgba(99,102,241,0.1); color: var(--primary); cursor: pointer; border: 1px solid transparent; }
.tag.active { background: var(--primary); color: #fff; }
.tag.small { font-size: 10px; padding: 1px 6px; }

.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--panel); border: 1px solid var(--border); border-radius: 14px; padding: 24px; width: 420px; max-width: 95vw; max-height: 90vh; overflow-y: auto; }
.modal h3 { margin: 0 0 18px; font-size: 16px; font-weight: 700; color: var(--text); }
.form-row { display: flex; flex-direction: column; gap: 4px; margin-bottom: 12px; }
.form-row label { font-size: 12px; font-weight: 600; color: var(--muted); }
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 18px; }

.btn { padding: 6px 14px; border-radius: 8px; font-size: 13px; font-weight: 500; cursor: pointer; border: 1px solid var(--border); background: var(--surface); color: var(--text); transition: background 0.15s; }
.btn:hover { background: var(--row-hover); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-sm { padding: 4px 10px; font-size: 12px; }
.btn-primary { background: var(--primary); color: #fff; border-color: var(--primary); }
.btn-primary:hover { opacity: 0.9; }
.btn-danger { background: rgba(239,68,68,0.1); color: #dc2626; border-color: rgba(239,68,68,0.3); }
.btn-danger:hover { background: rgba(239,68,68,0.2); }

.input { width: 100%; padding: 7px 10px; border-radius: 8px; border: 1px solid var(--border); background: var(--surface); color: var(--text); font-size: 13px; box-sizing: border-box; }
.input:focus { outline: none; border-color: var(--primary); }
.tags { display: flex; gap: 6px; flex-wrap: wrap; }
</style>