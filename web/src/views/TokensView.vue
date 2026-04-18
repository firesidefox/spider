<template>
  <div class="page-content">
    <div class="page-header">
      <h2>API Token</h2>
      <button class="btn btn-primary btn-sm" @click="showCreate = true">+ 新建 Token</button>
    </div>

    <table class="table">
      <thead>
        <tr><th>名称</th><th>创建时间</th><th>过期时间</th><th>最后使用</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="t in tokens" :key="t.id">
          <td>{{ t.name }}</td>
          <td class="dim">{{ new Date(t.created_at).toLocaleString() }}</td>
          <td class="dim">{{ t.expires_at ? new Date(t.expires_at).toLocaleString() : '永不过期' }}</td>
          <td class="dim">{{ t.last_used ? new Date(t.last_used).toLocaleString() : '从未' }}</td>
          <td>
            <button class="btn btn-sm btn-danger" @click="handleDelete(t.id)">撤销</button>
          </td>
        </tr>
        <tr v-if="tokens.length === 0">
          <td colspan="5" class="dim" style="text-align:center;padding:24px">暂无 Token</td>
        </tr>
      </tbody>
    </table>

    <!-- 新建 Token 弹窗 -->
    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h3>新建 API Token</h3>
        <div class="form-row"><label>名称</label><input v-model="form.name" class="input" placeholder="my-token" /></div>
        <div class="form-row">
          <label>过期时间（可选）</label>
          <input v-model="form.expiresAt" type="datetime-local" class="input" />
        </div>
        <div v-if="formError" class="err" style="margin-bottom:12px">{{ formError }}</div>
        <div class="modal-footer">
          <button class="btn" @click="showCreate = false">取消</button>
          <button class="btn btn-primary" @click="handleCreate">创建</button>
        </div>
      </div>
    </div>

    <!-- 明文展示弹窗（仅一次） -->
    <div v-if="newToken" class="modal-overlay">
      <div class="modal">
        <h3>Token 已创建</h3>
        <p class="dim" style="margin-bottom:12px;font-size:13px">请立即复制，此后不再显示。</p>
        <div class="token-display">
          <code class="code">{{ newToken }}</code>
          <button class="btn btn-sm" @click="copyToken">复制</button>
        </div>
        <div class="modal-footer">
          <button class="btn btn-primary" @click="newToken = ''">我已复制</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listTokens, createToken, deleteToken } from '../api/tokens'
import type { TokenInfo } from '../api/tokens'

const tokens = ref<TokenInfo[]>([])
const showCreate = ref(false)
const newToken = ref('')
const formError = ref('')
const form = ref({ name: '', expiresAt: '' })

onMounted(async () => { tokens.value = await listTokens() })

async function handleCreate() {
  formError.value = ''
  try {
    const res = await createToken(form.value.name, form.value.expiresAt || undefined)
    newToken.value = res.token
    showCreate.value = false
    form.value = { name: '', expiresAt: '' }
    tokens.value = await listTokens()
  } catch (e: any) {
    formError.value = e.message
  }
}

async function handleDelete(id: string) {
  if (!confirm('确认撤销此 Token？')) return
  await deleteToken(id)
  tokens.value = await listTokens()
}

function copyToken() {
  navigator.clipboard.writeText(newToken.value).catch(() => {})
}
</script>

<style scoped>
.token-display {
  display: flex;
  align-items: center;
  gap: 10px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px 14px;
  margin-bottom: 8px;
}
.token-display code {
  flex: 1;
  word-break: break-all;
  font-size: 12px;
  color: var(--primary);
}
</style>
