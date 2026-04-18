<template>
  <div class="fullscreen-page">
    <!-- 左侧用户列表 -->
    <div class="user-list-panel">
      <div class="panel-toolbar">
        <span class="panel-title">用户管理</span>
        <button class="btn btn-primary btn-sm" @click="showCreate = true">+ 新建</button>
      </div>
      <div class="user-list">
        <div
          v-for="u in users"
          :key="u.id"
          class="user-item"
          :class="{ selected: selectedUser?.id === u.id }"
          @click="selectUser(u)"
        >
          <div class="user-item-top">
            <span class="user-item-name">{{ u.username }}</span>
            <span class="role-badge" :class="u.role">{{ u.role }}</span>
          </div>
          <div class="user-item-bottom">
            <span :class="u.enabled ? 'ok' : 'err'">{{ u.enabled ? '启用' : '禁用' }}</span>
            <span class="dim">{{ u.last_login ? new Date(u.last_login).toLocaleString() : '从未' }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 右侧详情区 -->
    <div class="user-detail-panel">
      <div v-if="!selectedUser" class="empty-state">
        <span class="dim">← 选择左侧用户</span>
      </div>
      <div v-else>
        <div class="settings-card">
          <h3>账号信息</h3>
          <div class="info-grid">
            <div class="info-item"><span class="info-label">用户名</span><span>{{ selectedUser.username }}</span></div>
            <div class="info-item">
              <span class="info-label">角色</span>
              <span class="role-badge" :class="selectedUser.role">{{ selectedUser.role }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">状态</span>
              <span :class="selectedUser.enabled ? 'ok' : 'err'">{{ selectedUser.enabled ? '启用' : '禁用' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">最后登录</span>
              <span class="dim">{{ selectedUser.last_login ? new Date(selectedUser.last_login).toLocaleString() : '从未' }}</span>
            </div>
          </div>
        </div>

        <div class="settings-card">
          <h3>操作</h3>
          <div class="form-row">
            <label>角色</label>
            <select v-model="detailForm.role" class="input" :disabled="selectedUser.id === currentUser?.id">
              <option value="admin">admin</option>
              <option value="operator">operator</option>
              <option value="viewer">viewer</option>
            </select>
          </div>
          <div class="form-row">
            <label>新密码</label>
            <input v-model="detailForm.password" type="password" class="input" placeholder="留空不修改" />
          </div>
          <div class="form-row">
            <label>确认新密码</label>
            <input v-model="detailForm.confirmPassword" type="password" class="input" placeholder="留空不修改" />
          </div>
          <div v-if="detailError" class="err" style="margin-bottom:12px">{{ detailError }}</div>
          <div v-if="detailSuccess" class="ok" style="margin-bottom:12px">{{ detailSuccess }}</div>
          <div class="detail-actions">
            <button class="btn btn-primary btn-sm" @click="handleDetailSave">保存修改</button>
            <button
              class="btn btn-sm"
              @click="toggleEnabled(selectedUser)"
              :disabled="selectedUser.id === currentUser?.id"
            >{{ selectedUser.enabled ? '禁用' : '启用' }}</button>
            <button
              class="btn btn-sm btn-danger"
              @click="confirmDelete(selectedUser)"
              :disabled="selectedUser.id === currentUser?.id"
            >删除</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 新建用户弹窗 -->
    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h3>新建用户</h3>
        <div class="form-row"><label>用户名</label><input v-model="form.username" class="input" /></div>
        <div class="form-row"><label>密码</label><input v-model="form.password" type="password" class="input" /></div>
        <div class="form-row">
          <label>角色</label>
          <select v-model="form.role" class="input">
            <option value="admin">admin</option>
            <option value="operator">operator</option>
            <option value="viewer">viewer</option>
          </select>
        </div>
        <div v-if="formError" class="err" style="margin-bottom:12px">{{ formError }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showCreate = false">取消</button>
          <button class="btn btn-primary" @click="handleCreate">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { listUsers, createUser, updateUser, deleteUser } from '../api/users'
import { useAuth } from '../composables/useAuth'
import type { UserInfo } from '../api/auth'

const { currentUser } = useAuth()
const users = ref<UserInfo[]>([])
const selectedUser = ref<UserInfo | null>(null)

const showCreate = ref(false)
const formError = ref('')
const form = ref({ username: '', password: '', role: 'operator' })

const detailForm = ref({ role: 'operator', password: '', confirmPassword: '' })
const detailError = ref('')
const detailSuccess = ref('')

onMounted(async () => { users.value = await listUsers() })

function selectUser(u: UserInfo) {
  selectedUser.value = u
  detailForm.value = { role: u.role, password: '', confirmPassword: '' }
  detailError.value = ''
  detailSuccess.value = ''
}

watch(selectedUser, (u) => {
  if (u) detailForm.value.role = u.role
})

async function toggleEnabled(u: UserInfo) {
  await updateUser(u.id, { enabled: !u.enabled })
  users.value = await listUsers()
  const updated = users.value.find(x => x.id === u.id)
  if (updated) selectedUser.value = updated
}

async function confirmDelete(u: UserInfo) {
  if (!confirm(`确认删除用户 ${u.username}？`)) return
  await deleteUser(u.id)
  users.value = await listUsers()
  selectedUser.value = null
}

async function handleCreate() {
  formError.value = ''
  try {
    await createUser(form.value.username, form.value.password, form.value.role)
    showCreate.value = false
    form.value = { username: '', password: '', role: 'operator' }
    users.value = await listUsers()
  } catch (e: any) {
    formError.value = e.message
  }
}

async function handleDetailSave() {
  detailError.value = ''
  detailSuccess.value = ''
  if (detailForm.value.password !== detailForm.value.confirmPassword) {
    detailError.value = '两次密码不一致'
    return
  }
  const data: { role?: string; password?: string } = {}
  if (selectedUser.value?.id !== currentUser.value?.id) {
    data.role = detailForm.value.role
  }
  if (detailForm.value.password) {
    data.password = detailForm.value.password
  }
  try {
    await updateUser(selectedUser.value!.id, data)
    detailForm.value.password = ''
    detailForm.value.confirmPassword = ''
    detailSuccess.value = '保存成功'
    users.value = await listUsers()
    const updated = users.value.find(x => x.id === selectedUser.value?.id)
    if (updated) selectedUser.value = updated
  } catch (e: any) {
    detailError.value = e.message
  }
}
</script>

<style scoped>
.user-list-panel {
  width: 280px;
  flex-shrink: 0;
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
}
.panel-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border-bottom: 1px solid var(--border);
}
.panel-title { font-size: 14px; font-weight: 700; color: var(--text); }
.user-list { overflow-y: auto; flex: 1; }
.user-item {
  padding: 12px 16px;
  cursor: pointer;
  border-left: 3px solid transparent;
  border-bottom: 1px solid var(--border);
}
.user-item:hover { background: var(--row-hover); }
.user-item.selected {
  border-left-color: var(--primary);
  background: var(--row-hover);
  color: var(--primary);
}
.user-item-top {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.user-item-name { font-weight: 600; font-size: 13px; }
.user-item-bottom { display: flex; gap: 10px; font-size: 12px; }
.user-detail-panel { flex: 1; overflow-y: auto; padding: 24px; min-width: 0; }
.empty-state {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
}
.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 24px;
}
.info-item { display: flex; flex-direction: column; gap: 4px; }
.info-label { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
.role-badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid transparent;
}
.role-badge.admin    { background: rgba(99,102,241,0.12); color: var(--primary); border-color: rgba(99,102,241,0.3); }
.role-badge.operator { background: rgba(74,222,128,0.12); color: var(--green);   border-color: rgba(74,222,128,0.3); }
.role-badge.viewer   { background: rgba(167,139,250,0.1); color: var(--purple);  border-color: rgba(167,139,250,0.25); }
.detail-actions { display: flex; gap: 8px; margin-top: 16px; }
</style>
