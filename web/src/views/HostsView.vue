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
          @click="activeHost = h"
        >
          <div class="host-row-left">
            <input type="checkbox" v-model="selected" :value="h.id" @click.stop />
            <div class="host-row-info">
              <span class="host-row-name">{{ h.name }}</span>
              <span class="host-row-ip">{{ h.ip }}:{{ h.port }}</span>
            </div>
          </div>
          <div class="host-row-right">
            <span v-for="t in h.tags" :key="t" class="tag small">{{ t }}</span>
            <span class="badge">{{ h.auth_type }}</span>
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
            <span class="badge">{{ activeHost.auth_type }}</span>
          </div>
          <div class="detail-topbar-right">
            <button class="btn btn-sm" @click="goExec(activeHost)">▶ 执行</button>
            <button class="btn btn-sm" @click="editHost(activeHost)">编辑</button>
            <button class="btn btn-sm btn-danger" @click="removeHost(activeHost)">删除</button>
          </div>
        </div>
        <div class="detail-body">
          <div class="detail-grid">
            <div class="detail-field">
              <div class="detail-label">IP 地址</div>
              <div class="detail-value code">{{ activeHost.ip }}</div>
            </div>
            <div class="detail-field">
              <div class="detail-label">端口</div>
              <div class="detail-value">{{ activeHost.port }}</div>
            </div>
            <div class="detail-field">
              <div class="detail-label">用户名</div>
              <div class="detail-value code">{{ activeHost.username }}</div>
            </div>
            <div class="detail-field">
              <div class="detail-label">认证方式</div>
              <div class="detail-value">{{ activeHost.auth_type }}</div>
            </div>
          </div>
          <div v-if="activeHost.tags.length" class="detail-field" style="margin-top:16px">
            <div class="detail-label">标签</div>
            <div class="tags" style="margin-top:6px">
              <span v-for="t in activeHost.tags" :key="t" class="tag">{{ t }}</span>
            </div>
          </div>
        </div>
      </template>
      <div v-else class="detail-empty">
        <div class="detail-empty-icon">←</div>
        <div>选择左侧主机查看详情</div>
      </div>
    </div>

    <!-- 添加/编辑弹窗 -->
    <div v-if="showAdd || editTarget" class="modal-overlay" @click.self="closeModal">
      <div class="modal">
        <h3>{{ editTarget ? '编辑主机' : '添加主机' }}</h3>
        <form @submit.prevent="submitHost">
          <div class="form-row">
            <label>名称</label>
            <input v-model="form.name" class="input" required />
          </div>
          <div class="form-row">
            <label>IP</label>
            <input v-model="form.ip" class="input" required />
          </div>
          <div class="form-row">
            <label>端口</label>
            <input v-model.number="form.port" class="input" type="number" />
          </div>
          <div class="form-row">
            <label>用户名</label>
            <input v-model="form.username" class="input" required />
          </div>
          <div class="form-row">
            <label>认证方式</label>
            <select v-model="form.auth_type" class="input">
              <option value="password">密码</option>
              <option value="key">私钥</option>
              <option value="key_password">私钥+密码</option>
            </select>
          </div>
          <div class="form-row">
            <label>{{ form.auth_type === 'password' ? '密码' : '私钥内容' }}</label>
            <textarea v-model="form.credential" class="input" rows="3" :placeholder="form.auth_type === 'password' ? '登录密码' : 'PEM 格式私钥'" />
          </div>
          <div v-if="form.auth_type === 'key_password'" class="form-row">
            <label>Passphrase</label>
            <input v-model="form.passphrase" class="input" type="password" />
          </div>
          <div class="form-row">
            <label>标签</label>
            <input v-model="form.tagsStr" class="input" placeholder="逗号分隔，如 prod,web" />
          </div>
          <div class="modal-footer">
            <button type="button" class="btn" @click="closeModal">取消</button>
            <button type="submit" class="btn btn-primary">{{ editTarget ? '保存' : '添加' }}</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { listHosts, addHost, updateHost, deleteHost, type SafeHost } from '../api/hosts'

const router = useRouter()
const hosts = ref<SafeHost[]>([])
const search = ref('')
const filterTag = ref('')
const selected = ref<string[]>([])
const activeHost = ref<SafeHost | null>(null)
const showAdd = ref(false)
const editTarget = ref<SafeHost | null>(null)

const emptyForm = () => ({ name: '', ip: '', port: 22, username: '', auth_type: 'password', credential: '', passphrase: '', tagsStr: '' })
const form = ref(emptyForm())

const allTags = computed(() => {
  const s = new Set<string>()
  hosts.value.forEach(h => h.tags.forEach(t => s.add(t)))
  return [...s]
})

const filtered = computed(() => hosts.value.filter(h => {
  const q = search.value.toLowerCase()
  const matchSearch = !q || h.name.toLowerCase().includes(q) || h.ip.includes(q)
  const matchTag = !filterTag.value || h.tags.includes(filterTag.value)
  return matchSearch && matchTag
}))

async function load() {
  hosts.value = await listHosts()
}

function editHost(h: SafeHost) {
  editTarget.value = h
  form.value = { name: h.name, ip: h.ip, port: h.port, username: h.username, auth_type: h.auth_type, credential: '', passphrase: '', tagsStr: h.tags.join(',') }
}

function closeModal() {
  showAdd.value = false
  editTarget.value = null
  form.value = emptyForm()
}

async function submitHost() {
  const tags = form.value.tagsStr.split(',').map(t => t.trim()).filter(Boolean)
  if (editTarget.value) {
    await updateHost(editTarget.value.id, { name: form.value.name || undefined, ip: form.value.ip, port: form.value.port, username: form.value.username, auth_type: form.value.auth_type, credential: form.value.credential || undefined, passphrase: form.value.passphrase || undefined, tags })
    if (activeHost.value?.id === editTarget.value.id) {
      activeHost.value = { ...activeHost.value, ...form.value, tags }
    }
  } else {
    await addHost({ ...form.value, tags })
  }
  closeModal()
  load()
}

async function removeHost(h: SafeHost) {
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

function goExec(h: SafeHost) {
  router.push({ path: '/exec', query: { host: h.id } })
}

function bulkExecSelected() {
  router.push({ path: '/exec', query: { hosts: selected.value.join(',') } })
}

onMounted(load)
</script>

<style scoped>
.hosts-page {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* ── 左侧面板 ── */
.hosts-sidebar {
  width: 26%;
  min-width: 280px;
  max-width: 380px;
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow: hidden;
}

.sidebar-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.sidebar-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text);
}

.sidebar-search {
  padding: 10px 12px 8px;
  flex-shrink: 0;
}

.sidebar-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  padding: 0 12px 10px;
  flex-shrink: 0;
}

.sidebar-list {
  flex: 1;
  overflow-y: auto;
}

.host-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
  border-left: 3px solid transparent;
  cursor: pointer;
  transition: background 0.1s;
  gap: 8px;
}

.host-row:hover { background: var(--row-hover); }

.host-row.selected {
  border-left-color: var(--primary);
  background: rgba(99,102,241,0.1);
}

.host-row-left { display: flex; align-items: center; gap: 10px; min-width: 0; }

.host-row-info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }

.host-row-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.host-row-ip { font-size: 12px; color: var(--label); font-family: 'SF Mono', Consolas, monospace; }

.host-row-right { display: flex; align-items: center; gap: 4px; flex-shrink: 0; flex-wrap: wrap; justify-content: flex-end; }

.sidebar-bulk {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 10px 14px;
  border-top: 1px solid var(--border);
  font-size: 13px;
  color: var(--text-sub);
  background: rgba(99,102,241,0.06);
  flex-shrink: 0;
}

.sidebar-empty { color: var(--label); font-size: 13px; padding: 32px 16px; text-align: center; }

/* ── 右侧详情 ── */
.hosts-detail {
  flex: 1;
  overflow: hidden;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.detail-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  flex-shrink: 0;
}

.detail-topbar-left { display: flex; align-items: center; gap: 10px; }
.detail-topbar-right { display: flex; gap: 8px; }

.detail-title { font-size: 15px; font-weight: 700; color: var(--text); }

.detail-body {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
}

.detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.detail-field {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px 20px;
  box-shadow: var(--card-shadow);
}

.detail-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.07em;
  margin-bottom: 6px;
}

.detail-value { font-size: 15px; font-weight: 600; color: var(--text); }

.detail-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--muted);
  font-size: 14px;
}

.detail-empty-icon { color: var(--border); font-size: 40px; }
</style>
