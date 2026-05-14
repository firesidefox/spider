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
        <div class="detail-body">
          <div v-if="pingResult" class="ping-result" :class="pingResult.connected ? 'ping-ok' : 'ping-fail'">
            <span v-if="pingResult.connected">● 已连接 ({{ pingResult.latency_ms }}ms)</span>
            <span v-else>● 连接失败: {{ pingResult.error }}</span>
          </div>

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
                <button class="edit-link" @click="openAddFace">+ 添加</button>
              </div>
              <div class="section-body">
                <div v-if="faces.length === 0" class="tab-empty" style="padding:12px 0">暂无操作面</div>
                <div v-for="f in faces" :key="f.id" class="face-card">
                  <div class="face-header">
                    <div class="face-type">
                      <span class="badge" :class="f.type === 'ssh' ? 'badge-ssh' : 'badge-rest'">{{ f.type === 'ssh' ? 'SSH' : 'REST API' }}</span>
                      <span class="face-addr code">{{ f.type === 'ssh' ? `${f.username}@${f.ip}:${f.port}` : f.base_url || `${f.ip}:${f.port}` }}</span>
                    </div>
                    <div class="face-actions">
                      <button class="edit-link" @click="startEditFace(f)">编辑</button>
                      <button class="edit-link danger-link" @click="removeFace(f)">删除</button>
                    </div>
                  </div>
                  <div class="face-body">
                    <div v-if="f.type === 'ssh'" class="face-item"><label>认证方式</label><div class="value">{{ f.ssh_auth_type === 'password' ? '密码' : f.ssh_auth_type === 'key' ? 'SSH Key' : 'SSH Key + 密码' }}</div></div>
                    <div v-if="f.type === 'ssh' && f.ssh_key_id" class="face-item"><label>SSH Key</label><div class="value">{{ sshKeys.find(k => k.id === f.ssh_key_id)?.name || f.ssh_key_id }}</div></div>
                    <div v-if="f.type === 'ssh'" class="face-item"><label>兼容模式</label><div class="value">{{ f.ssh_legacy ? '是' : '否' }}</div></div>
                    <div v-if="f.type === 'ssh' && f.ssh_login_input" class="face-item"><label>登录后输入</label><div class="value"><code>{{ f.ssh_login_input }}</code></div></div>
                    <div v-if="f.type === 'restapi'" class="face-item">
                      <label>认证方式</label>
                      <div class="value">{{ f.rest_auth_type === 'hmac_aksk' ? `HMAC AK/SK (${f.hmac_algo || 'HMAC-SHA256'})` : f.rest_auth_type }}</div>
                    </div>
                    <div v-if="f.type === 'restapi' && f.rest_username" class="face-item"><label>用户名</label><div class="value">{{ f.rest_username }}</div></div>
                    <div v-if="f.type === 'restapi' && f.base_url" class="face-item" style="grid-column:1/-1"><label>Base URL</label><div class="value"><code>{{ f.base_url }}</code></div></div>
                  </div>
                  <div class="knowledge-row">
                    <span class="knowledge-label">知识来源：</span>
                    <span v-if="f.knowledge_sources?.some(k => k.type === 'none')" class="knowledge-tag knowledge-none">不查询</span>
                    <template v-else>
                    <span v-for="ks in f.knowledge_sources" :key="ks.type+ks.id" class="knowledge-tag">
                      <span class="at">@</span>{{ docGroupsMap.get(ks.id)?.name || ks.id }}
                      <button class="ks-remove" @click.stop="saveKnowledgeSources(f, f.knowledge_sources.filter(k => k.id !== ks.id).map(k => k.id))">×</button>
                    </span>
                    </template>
                    <div class="ks-picker-wrap">
                      <button class="add-knowledge" @click.stop="ksPickerFaceId = ksPickerFaceId === f.id ? null : f.id">+ 添加</button>
                      <div v-if="ksPickerFaceId === f.id" class="ks-dropdown" @click.stop>
                        <label v-for="g in docGroups" :key="g.id" class="ks-option">
                          <input type="checkbox"
                            :checked="f.knowledge_sources.some(k => k.id === g.id)"
                            @change="toggleKnowledgeSource(f, g.id)"
                          />
                          {{ g.name }}
                        </label>
                        <div v-if="!docGroups || docGroups.length === 0" class="ks-empty">暂无文档组</div>
                      </div>
                    </div>
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
                <option value="key">SSH Key</option>
                <option value="key_password">SSH Key + 密码</option>
              </select>
            </div>
            <div v-if="faceForm.ssh_auth_type === 'key' || faceForm.ssh_auth_type === 'key_password'" class="form-row">
              <label>SSH Key</label>
              <select v-model="faceForm.ssh_key_id" class="input">
                <option value="">— 选择 SSH Key —</option>
                <option v-for="k in sshKeys" :key="k.id" :value="k.id">{{ k.name }}</option>
              </select>
            </div>
            <div v-if="faceForm.ssh_auth_type === 'password' || faceForm.ssh_auth_type === 'key_password'" class="form-row">
              <label>{{ faceForm.ssh_auth_type === 'key_password' ? '密钥密码' : '密码' }}</label>
              <input v-model="faceForm.credential" class="input" type="password" autocomplete="new-password" />
            </div>
            <div class="form-row">
              <label>兼容模式</label>
              <label class="checkbox-label"><input type="checkbox" v-model="faceForm.ssh_legacy" /> 启用（旧版 SSH 算法）</label>
            </div>
            <div class="form-row">
              <label>登录后输入（可选）</label>
              <input v-model="faceForm.ssh_login_input" class="input" placeholder="/rsh" />
            </div>
          </template>
          <template v-if="faceForm.type === 'restapi'">
            <div class="form-row">
              <label>Base URL</label>
              <input v-model="faceForm.base_url" class="input" placeholder="http://192.168.1.1:8080" />
            </div>
            <div class="form-row">
              <label>认证方式</label>
              <select v-model="faceForm.rest_auth_type" class="input" @change="onRestAuthTypeChange">
                <option value="none">无</option>
                <option value="bearer">Bearer Token</option>
                <option value="basic">Basic</option>
                <option value="apikey">API Key</option>
                <option value="hmac_aksk">HMAC AK/SK</option>
              </select>
            </div>
            <template v-if="faceForm.rest_auth_type === 'basic'">
              <div class="form-row"><label>用户名</label><input v-model="faceForm.rest_username" class="input" /></div>
              <div class="form-row"><label>密码</label><input v-model="faceForm.credential" class="input" type="password" autocomplete="new-password" /></div>
            </template>
            <template v-if="faceForm.rest_auth_type === 'bearer'">
              <div class="form-row"><label>Token</label><input v-model="faceForm.credential" class="input" type="password" autocomplete="new-password" /></div>
            </template>
            <template v-if="faceForm.rest_auth_type === 'apikey'">
              <div class="form-row"><label>Header Name</label><input v-model="faceForm.header_name" class="input" placeholder="X-API-Key" /></div>
              <div class="form-row"><label>API Key</label><input v-model="faceForm.credential" class="input" type="password" autocomplete="new-password" /></div>
            </template>
            <template v-if="faceForm.rest_auth_type === 'hmac_aksk'">
              <div class="form-row"><label>Access Key (AK)</label><input v-model="faceForm.rest_username" class="input" placeholder="QMZ0ZENmYvwDJTz7..." /></div>
              <div class="form-row"><label>Secret Key (SK)</label><input v-model="faceForm.credential" class="input" type="password" autocomplete="new-password" /></div>
              <div class="form-row">
                <label>签名算法</label>
                <select v-model="faceForm.hmac_algo" class="input">
                  <option value="HMAC-SHA256">HMAC-SHA256</option>
                  <option value="HMAC-SM3">HMAC-SM3</option>
                </select>
              </div>
            </template>
          </template>
          <div class="form-row">
            <label>知识来源</label>
            <div class="ks-mode-tabs">
              <button type="button" class="btn btn-sm" :class="{ active: ksMode === 'global' }" @click="setKsMode('global')">全局</button>
              <button type="button" class="btn btn-sm" :class="{ active: ksMode === 'group' }" @click="setKsMode('group')">文档组</button>
              <button type="button" class="btn btn-sm" :class="{ active: ksMode === 'doc' }" @click="setKsMode('doc')">具体文档</button>
              <button type="button" class="btn btn-sm" :class="{ active: ksMode === 'none' }" @click="setKsMode('none')">不查询</button>
            </div>
            <div v-if="ksMode === 'group' && docGroups && docGroups.length > 0" class="ks-checkboxes">
              <label v-for="g in docGroups" :key="g.id" class="checkbox-label">
                <input type="checkbox"
                  :checked="faceForm.knowledge_sources.some(k => k.type === 'group' && k.id === g.id)"
                  @change="toggleKs('group', g.id)" />
                {{ g.name }}
              </label>
            </div>
            <div v-if="ksMode === 'doc' && allDocs && allDocs.length > 0" class="ks-checkboxes">
              <label v-for="d in allDocs" :key="d.id" class="checkbox-label">
                <input type="checkbox"
                  :checked="faceForm.knowledge_sources.some(k => k.type === 'doc' && k.id === d.id)"
                  @change="toggleKs('doc', d.id)" />
                {{ d.title || d.source_file }}
              </label>
            </div>
          </div>
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
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  listHosts, addHost, updateHost, deleteHost, pingHost,
  listAccessFaces, addAccessFace, updateAccessFace, deleteAccessFace,
  getFingerprint, listMemories, addMemory, deleteMemory,
  type Host, type AccessFace, type Fingerprint, type Memory,
} from '../api/hosts'
import { listSSHKeys, type SafeSSHKey } from '../api/ssh-keys'
import { listGroups, listDocuments, type DocumentGroup, type Document } from '../api/documents'

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
const ksPickerFaceId = ref<string | null>(null)


const faces = ref<AccessFace[]>([])
const fingerprint = ref<Fingerprint | null>(null)
const memories = ref<Memory[]>([])
const newMemory = ref('')
const showAddFace = ref(false)
const editFaceTarget = ref<AccessFace | null>(null)
const sshKeys = ref<SafeSSHKey[]>([])
const docGroups = ref<DocumentGroup[]>([])
const ksMode = ref<'global' | 'group' | 'doc' | 'none'>('global')
const allDocs = ref<Document[]>([])

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

const emptyFaceForm = () => ({ type: 'ssh' as 'ssh' | 'restapi', ip: activeHost.value?.ip ?? '', port: 22, username: '', ssh_auth_type: 'password', credential: '', passphrase: '', ssh_key_id: '', ssh_legacy: false, ssh_login_input: '', base_url: '', rest_auth_type: 'none', rest_username: '', header_name: '', hmac_algo: 'HMAC-SHA256', knowledge_sources: [] as Array<{type:'group'|'doc';id:number}> })
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

const docGroupsMap = computed(() => new Map(docGroups.value.map(g => [g.id, g])))

async function load() { hosts.value = await listHosts() }

async function selectHost(h: Host) {
  activeHost.value = h
  pingResult.value = null
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
  } catch (e: any) {
    pingResult.value = { connected: false, error: e.message || '请求失败' }
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

async function saveKnowledgeSources(face: AccessFace, groupIds: number[]) {
  if (!activeHost.value) return
  const prev = face.knowledge_sources
  const ks = groupIds.map(id => ({ type: 'group' as const, id }))
  face.knowledge_sources = ks
  ksPickerFaceId.value = null
  try {
    await updateAccessFace(activeHost.value.id, face.id, { knowledge_sources: ks })
  } catch {
    face.knowledge_sources = prev
  }
}

function toggleKnowledgeSource(face: AccessFace, groupId: number) {
  const ids = face.knowledge_sources.some(k => k.id === groupId)
    ? face.knowledge_sources.filter(k => k.id !== groupId).map(k => k.id)
    : [...face.knowledge_sources.map(k => k.id), groupId]
  saveKnowledgeSources(face, ids)
}

function toggleFormKnowledgeSource(groupId: number) {
  const ks = faceForm.value.knowledge_sources
  const exists = ks.some(k => k.type === 'group' && k.id === groupId)
  faceForm.value.knowledge_sources = exists
    ? ks.filter(k => !(k.type === 'group' && k.id === groupId))
    : [...ks, { type: 'group', id: groupId }]
}

function setKsMode(mode: 'global' | 'group' | 'doc' | 'none') {
  ksMode.value = mode
  faceForm.value.knowledge_sources = mode === 'none' ? [{ type: 'none', id: 0 }] : []
}

function toggleKs(type: 'group' | 'doc', id: number) {
  const ks = faceForm.value.knowledge_sources
  const exists = ks.some(k => k.type === type && k.id === id)
  faceForm.value.knowledge_sources = exists
    ? ks.filter(k => !(k.type === type && k.id === id))
    : [...ks, { type, id }]
}

function onRestAuthTypeChange() {
  // 无需前端清空；后端 Update 按 auth type 清空无关字段
}

async function submitFace() {
  if (!activeHost.value) return
  const req: Record<string, unknown> = { type: faceForm.value.type, ip: faceForm.value.ip, port: faceForm.value.port, tags: [], knowledge_sources: faceForm.value.knowledge_sources }
  if (faceForm.value.type === 'ssh') {
    req.username = faceForm.value.username || undefined
    req.ssh_auth_type = faceForm.value.ssh_auth_type
    req.credential = faceForm.value.credential || undefined
    req.passphrase = faceForm.value.passphrase || undefined
    req.ssh_key_id = faceForm.value.ssh_key_id || undefined
    req.ssh_legacy = faceForm.value.ssh_legacy
    req.ssh_login_input = faceForm.value.ssh_login_input || undefined
  } else {
    req.base_url = faceForm.value.base_url || undefined
    req.rest_auth_type = faceForm.value.rest_auth_type
    req.rest_username = faceForm.value.rest_username || undefined
    req.credential = faceForm.value.credential || undefined
    req.header_name = faceForm.value.header_name || undefined
    req.hmac_algo = faceForm.value.hmac_algo || undefined
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
    ssh_login_input: face.ssh_login_input || '',
    base_url: face.base_url || '',
    rest_auth_type: face.rest_auth_type || 'none',
    rest_username: face.rest_username || '',
    header_name: face.header_name || '',
    hmac_algo: face.hmac_algo || 'HMAC-SHA256',
    knowledge_sources: face.knowledge_sources ? [...face.knowledge_sources] : [],
  }
  const ks = face.knowledge_sources ?? []
  if (ks.length === 0) {
    ksMode.value = 'global'
  } else if (ks[0].type === 'none') {
    ksMode.value = 'none'
  } else if (ks[0].type === 'doc') {
    ksMode.value = 'doc'
  } else {
    ksMode.value = 'group'
  }
  showAddFace.value = true
}

function openAddFace() {
  faceForm.value = emptyFaceForm()
  editFaceTarget.value = null
  ksMode.value = 'global'
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

function handleEsc(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (ksPickerFaceId.value) { ksPickerFaceId.value = null; return }
  if (showAddFace.value) { closeFaceModal(); return }
  if (showAdd.value) { closeModal(); return }
}

function handleOutsideClick(e: MouseEvent) {
  if (ksPickerFaceId.value && !(e.target as Element).closest('.ks-picker-wrap')) {
    ksPickerFaceId.value = null
  }
}

onMounted(async () => {
  window.addEventListener('keydown', handleEsc)
  document.addEventListener('mousedown', handleOutsideClick)
  const [, keys, groups, docs] = await Promise.all([
    load(),
    listSSHKeys().catch((): SafeSSHKey[] => []),
    listGroups().catch((): DocumentGroup[] => []),
    listDocuments().catch((): Document[] => []),
  ])
  sshKeys.value = keys
  docGroups.value = groups
  allDocs.value = docs
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleEsc)
  document.removeEventListener('mousedown', handleOutsideClick)
})
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

.detail-body { flex: 1; overflow-y: auto; padding: 20px 24px; }
.detail-empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: var(--muted); font-size: 14px; }
.detail-empty-icon { color: var(--border); font-size: 40px; }
.tab-empty { color: var(--muted); font-size: 13px; padding: 32px 0; text-align: center; }

.section { background: var(--card-bg); border: 1px solid var(--border); border-radius: 10px; margin-bottom: 16px; }
.section-header { padding: 10px 16px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border); background: var(--surface); }
.section-title { font-size: 12px; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
.section-header-actions { display: flex; gap: 6px; }
.section-body { padding: 16px; }
.edit-link { font-size: 12px; color: var(--primary); cursor: pointer; background: none; border: none; padding: 0; }
.edit-link:hover { text-decoration: underline; }
.danger-link { color: var(--danger, #e53e3e); }

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
.face-card { border: 1px solid var(--border); border-radius: 8px; margin-bottom: 10px; }
.face-header { padding: 10px 14px; background: var(--surface); display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border); }
.face-type { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 13px; }
.face-addr { font-size: 13px; color: var(--text); }
.face-body { padding: 12px 14px; display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.face-item label { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.04em; display: block; margin-bottom: 3px; }
.face-item .value { font-size: 13px; color: var(--text); }
.face-item .value code { font-family: 'SF Mono', Consolas, monospace; font-size: 12px; }
.face-actions { display: flex; gap: 6px; }
.face-details { display: flex; gap: 12px; flex-wrap: wrap; font-size: 13px; color: var(--text-sub); }
.knowledge-row { padding: 8px 14px; border-top: 1px solid var(--border); display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.knowledge-label { font-size: 11px; color: var(--muted); }
.knowledge-tag { font-size: 11px; padding: 2px 8px; border-radius: 4px; background: var(--surface); border: 1px solid var(--border); color: var(--text-sub); display: flex; align-items: center; gap: 3px; }
.knowledge-none { color: var(--muted); border-style: dashed; }
.knowledge-tag .at { color: var(--primary); }
.add-knowledge { font-size: 11px; color: var(--primary); cursor: pointer; padding: 2px 8px; border: 1px dashed var(--primary); border-radius: 4px; background: none; }
.add-knowledge:hover { background: rgba(99,102,241,0.08); }
.ks-remove { background: none; border: none; color: var(--muted); cursor: pointer; padding: 0 0 0 4px; font-size: 12px; line-height: 1; }
.ks-remove:hover { color: var(--danger, #ef4444); }
.ks-picker-wrap { position: relative; display: inline-flex; }
.ks-dropdown { position: absolute; top: calc(100% + 4px); left: 0; z-index: 100; background: var(--card-bg); border: 1px solid var(--border); border-radius: 6px; box-shadow: 0 4px 12px rgba(0,0,0,0.15); padding: 6px 0; min-width: 160px; }
.ks-option { display: flex; align-items: center; gap: 8px; padding: 6px 12px; font-size: 13px; color: var(--text); cursor: pointer; white-space: nowrap; }
.ks-option:hover { background: var(--surface); }
.ks-option input { cursor: pointer; }
.ks-empty { padding: 8px 12px; font-size: 12px; color: var(--muted); }
.ks-checkboxes { display: flex; flex-direction: column; gap: 6px; }

.memory-item { padding: 10px 12px; background: var(--card-bg); border-radius: 6px; border-left: 3px solid var(--border); margin-bottom: 8px; }
.memory-item.memory-agent { border-left-color: var(--primary); }
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
.checkbox-label { display: flex; align-items: center; gap: 6px; font-size: 13px; cursor: pointer; }
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

.ks-mode-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 8px;
}
.ks-mode-tabs .btn.active {
  background: var(--accent);
  color: #fff;
}
</style>