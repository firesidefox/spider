<template>
  <div class="page-content">
    <div class="page-header">
      <h2>用户管理</h2>
      <button class="btn btn-primary btn-sm" @click="showCreate = true">+ 新建用户</button>
    </div>

    <table class="table">
      <thead>
        <tr>
          <th>用户名</th><th>角色</th><th>状态</th><th>最后登录</th><th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in users" :key="u.id">
          <td>{{ u.username }}</td>
          <td><span class="badge">{{ u.role }}</span></td>
          <td>
            <span :class="u.enabled ? 'ok' : 'err'">{{ u.enabled ? '启用' : '禁用' }}</span>
          </td>
          <td class="dim">{{ u.last_login ? new Date(u.last_login).toLocaleString() : '从未' }}</td>
          <td>
            <div class="actions">
              <button class="btn btn-sm" @click="toggleEnabled(u)" :disabled="u.id === currentUser?.id">
                {{ u.enabled ? '禁用' : '启用' }}
              </button>
              <button class="btn btn-sm btn-danger" @click="confirmDelete(u)" :disabled="u.id === currentUser?.id">删除</button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>

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
import { ref, onMounted } from 'vue'
import { listUsers, createUser, updateUser, deleteUser } from '../api/users'
import { useAuth } from '../composables/useAuth'
import type { UserInfo } from '../api/auth'

const { currentUser } = useAuth()
const users = ref<UserInfo[]>([])
const showCreate = ref(false)
const formError = ref('')
const form = ref({ username: '', password: '', role: 'operator' })

onMounted(async () => { users.value = await listUsers() })

async function toggleEnabled(u: UserInfo) {
  await updateUser(u.id, { enabled: !u.enabled })
  users.value = await listUsers()
}

async function confirmDelete(u: UserInfo) {
  if (!confirm(`确认删除用户 ${u.username}？`)) return
  await deleteUser(u.id)
  users.value = await listUsers()
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
</script>
