<template>
  <div>
    <div class="page-header">
      <h2>主机管理</h2>
      <button class="btn btn-primary" @click="showAdd = true">+ 添加主机</button>
    </div>

    <div class="toolbar">
      <input v-model="search" class="input" placeholder="搜索主机名 / IP..." style="width:220px" />
      <div class="tags">
        <span class="tag" :class="{ active: !filterTag }" @click="filterTag = ''">全部</span>
        <span v-for="t in allTags" :key="t" class="tag" :class="{ active: filterTag === t }" @click="filterTag = t">{{ t }}</span>
      </div>
    </div>

    <table class="table">
      <thead>
        <tr>
          <th><input type="checkbox" @change="toggleAll" /></th>
          <th>名称</th><th>IP</th><th>端口</th><th>用户</th><th>认证</th><th>标签</th><th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="h in filtered" :key="h.id">
          <td><input type="checkbox" v-model="selected" :value="h.id" /></td>
          <td>{{ h.name }}</td>
          <td>{{ h.ip }}</td>
          <td>{{ h.port }}</td>
          <td>{{ h.username }}</td>
          <td><span class="badge">{{ h.auth_type }}</span></td>
          <td><span v-for="t in h.tags" :key="t" class="tag small">{{ t }}</span></td>
          <td class="actions">
            <button class="btn btn-sm" @click="goExec(h)">执行</button>
            <button class="btn btn-sm" @click="editHost(h)">编辑</button>
            <button class="btn btn-sm btn-danger" @click="removeHost(h)">删除</button>
          </td>
        </tr>
        <tr v-if="filtered.length === 0">
          <td colspan="8" style="text-align:center;color:#999;padding:32px">暂无主机</td>
        </tr>
      </tbody>
    </table>

    <div v-if="selected.length" class="bulk-bar">
      已选 {{ selected.length }} 台
      <button class="btn btn-sm" @click="bulkExecSelected">批量执行</button>
      <button class="btn btn-sm btn-danger" @click="bulkDelete">批量删除</button>
    </div>

    <!-- 添加/编辑弹窗 -->
    <div v-if="showAdd || editTarget" class="modal-overlay" @click.self="closeModal">
      <div class="modal">
        <h3>{{ editTarget ? '编辑主机' : '添加主机' }}</h3>
        <form @submit.prevent="submitHost">
          <div class="form-row">
            <label>名称</label>
            <input v-model="form.name" class="input" required :disabled="!!editTarget" />
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

function toggleAll(e: Event) {
  selected.value = (e.target as HTMLInputElement).checked ? filtered.value.map(h => h.id) : []
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
    await updateHost(editTarget.value.id, { ip: form.value.ip, port: form.value.port, username: form.value.username, auth_type: form.value.auth_type, credential: form.value.credential || undefined, passphrase: form.value.passphrase || undefined, tags })
  } else {
    await addHost({ ...form.value, tags })
  }
  closeModal()
  load()
}

async function removeHost(h: SafeHost) {
  if (!confirm(`确认删除主机 ${h.name}？`)) return
  await deleteHost(h.id)
  load()
}

async function bulkDelete() {
  if (!confirm(`确认删除 ${selected.value.length} 台主机？`)) return
  await Promise.all(selected.value.map(id => deleteHost(id)))
  selected.value = []
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
