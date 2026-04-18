<template>
  <div class="fullscreen-page users-page">
    <!-- 左侧面板 -->
    <aside class="users-sidebar">
      <div class="sidebar-toolbar">
        <span class="sidebar-title">用户管理</span>
        <button class="btn btn-primary btn-sm" @click="showCreate = true">+ 新建</button>
      </div>
      <div class="sidebar-list">
        <div
          v-for="u in users" :key="u.id"
          class="user-row"
          :class="{ selected: selectedUser?.id === u.id }"
          @click="selectUser(u)"
        >
          <div class="user-row-left">
            <div class="user-row-info">
              <span class="user-row-name">{{ u.username }}</span>
              <span class="user-row-sub">{{ u.last_login ? new Date(u.last_login).toLocaleString() : '从未登录' }}</span>
            </div>
          </div>
          <div class="user-row-right">
            <span class="role-badge" :class="u.role">{{ u.role }}</span>
            <span :class="u.enabled ? 'status-ok' : 'status-err'">{{ u.enabled ? '启用' : '禁用' }}</span>
          </div>
        </div>
        <div v-if="users.length === 0" class="sidebar-empty">暂无用户</div>
      </div>
    </aside>

    <!-- 右侧详情 -->
    <div class="users-detail">
      <template v-if="selectedUser">
        <div class="detail-topbar">
          <div class="detail-topbar-left">
            <span class="detail-title">{{ selectedUser.username }}</span>
            <span class="role-badge" :class="selectedUser.role">{{ selectedUser.role }}</span>
            <span :class="selectedUser.enabled ? 'status-ok' : 'status-err'">{{ selectedUser.enabled ? '启用' : '禁用' }}</span>
          </div>
          <div class="detail-topbar-right">
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
        <div class="detail-body">
          <div class="detail-grid">
            <div class="detail-field">
              <div class="detail-label">用户名</div>
              <div class="detail-value">{{ selectedUser.username }}</div>
            </div>
            <div class="detail-field">
              <div class="detail-label">最后登录</div>
              <div class="detail-value dim">{{ selectedUser.last_login ? new Date(selectedUser.last_login).toLocaleString() : '从未' }}</div>
            </div>
          </div>

          <div class="edit-card">
            <div class="edit-card-title">修改</div>
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
            <div v-if="detailError" class="err" style="margin-bottom:10px">{{ detailError }}</div>
            <div v-if="detailSuccess" class="ok" style="margin-bottom:10px">{{ detailSuccess }}</div>
            <button class="btn btn-primary btn-sm" @click="handleDetailSave">保存修改</button>
          </div>
        </div>
      </template>
      <div v-else class="detail-empty">
        <div class="detail-empty-icon">←</div>
        <div>选择左侧用户查看详情</div>
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
.users-page {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.users-sidebar {
  width: 26%;
  min-width: 260px;
  max-width: 360px;
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

.sidebar-title { font-size: 13px; font-weight: 700; color: var(--text); }

.sidebar-list { flex: 1; overflow-y: auto; }

.user-row {
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

.user-row:hover { background: var(--row-hover); }

.user-row.selected {
  border-left-color: var(--primary);
  background: rgba(99,102,241,0.1);
}

.user-row-left { display: flex; align-items: center; gap: 10px; min-width: 0; }

.user-row-info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }

.user-row-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-row-sub { font-size: 12px; color: var(--label); }

.user-row-right { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }

.sidebar-empty { color: var(--label); font-size: 13px; padding: 32px 16px; text-align: center; }

.users-detail {
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

.detail-body { flex: 1; overflow-y: auto; padding: 20px 24px; }

.detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 16px;
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

.edit-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 20px 24px;
  box-shadow: var(--card-shadow);
}

.edit-card-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

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

.status-ok  { font-size: 12px; font-weight: 600; color: var(--green); }
.status-err { font-size: 12px; font-weight: 600; color: var(--red); }
</style>
