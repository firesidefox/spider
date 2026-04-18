<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-brand">🕷 Spider</div>
      <h2>登录</h2>
      <form @submit.prevent="handleLogin">
        <div class="form-row">
          <label>用户名</label>
          <input v-model="username" class="input" placeholder="admin" autocomplete="username" />
        </div>
        <div class="form-row">
          <label>密码</label>
          <input v-model="password" type="password" class="input" placeholder="••••••••" autocomplete="current-password" />
        </div>
        <div v-if="error" class="login-error">{{ error }}</div>
        <button type="submit" class="btn btn-primary login-btn" :disabled="loading">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { login, setStoredToken } from '../api/auth'
import { useAuth } from '../composables/useAuth'

const router = useRouter()
const { setUser } = useAuth()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    const res = await login(username.value, password.value)
    setStoredToken(res.token)
    setUser(res.user)
    router.push('/hosts')
  } catch (e: any) {
    error.value = e.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg);
}
.login-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 36px 40px;
  width: 380px;
  box-shadow: var(--card-shadow);
}
.login-brand {
  font-size: 22px;
  font-weight: 700;
  margin-bottom: 8px;
}
h2 { font-size: 18px; font-weight: 600; margin-bottom: 24px; color: var(--text); }
.login-error {
  color: var(--red);
  font-size: 13px;
  margin-bottom: 12px;
  padding: 8px 12px;
  background: rgba(248,113,113,0.08);
  border-radius: 6px;
  border: 1px solid rgba(248,113,113,0.2);
}
.login-btn { width: 100%; margin-top: 8px; padding: 10px; }
</style>
