<template>
  <div class="detail-topbar">
    <span class="detail-title">访问令牌</span>
    <button class="btn btn-primary btn-sm" @click="showCreate = true">+ 新建 Token</button>
  </div>
  <div class="detail-body">
    <div class="edit-card">
      <p class="dim" style="margin-bottom:16px;font-size:13px">Token 可用于 MCP 工具或 API 调用，权限与账号角色一致。</p>
      <table class="table">
        <thead><tr><th>名称</th><th>创建时间</th><th>过期时间</th><th>最后使用</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="t in tokens" :key="t.id">
            <td style="font-weight:500;color:var(--text)">{{ t.name }}</td>
            <td class="dim">{{ new Date(t.created_at).toLocaleString() }}</td>
            <td>
              <span v-if="t.expires_at" :class="isExpired(t.expires_at) ? 'err' : 'dim'">
                {{ new Date(t.expires_at).toLocaleString() }}
              </span>
              <span v-else class="dim">永不过期</span>
            </td>
            <td class="dim">{{ t.last_used ? new Date(t.last_used).toLocaleString() : '从未' }}</td>
            <td>
              <button class="btn btn-sm" @click="handleCopyToken(t.id)">{{ copiedTokenId === t.id ? '已复制 ✓' : '复制' }}</button>
              <button class="btn btn-sm btn-danger" style="margin-left:6px" @click="handleDelete(t.id)">撤销</button>
            </td>
          </tr>
          <tr v-if="tokens.length === 0">
            <td colspan="5" class="dim" style="text-align:center;padding:32px">暂无 Token</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>

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

  <!-- Token 明文展示弹窗 -->
  <div v-if="newToken" class="modal-overlay">
    <div class="modal">
      <h3>Token 已创建</h3>
      <p class="dim" style="margin-bottom:12px;font-size:13px">请立即复制，此后不再显示。</p>
      <div class="token-display">
        <code class="code token-code">{{ newToken }}</code>
        <button class="btn btn-sm" :class="{ 'btn-copied': copied }" @click="copyToken">{{ copied ? '✓ 已复制' : '复制' }}</button>
      </div>
      <div class="modal-footer">
        <button class="btn btn-primary" @click="newToken = ''; copied = false">我已复制</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listTokens, createToken, deleteToken } from '../../api/tokens'
import type { TokenInfo } from '../../api/tokens'

const tokens = ref<TokenInfo[]>([])
const showCreate = ref(false)
const newToken = ref('')
const copied = ref(false)
const copiedTokenId = ref('')
const formError = ref('')
const form = ref({ name: '', expiresAt: '' })
let tokensLoaded = false

async function loadTokens() {
  if (tokensLoaded) return
  tokensLoaded = true
  tokens.value = await listTokens()
}

onMounted(() => {
  loadTokens()
})

async function handleCreate() {
  formError.value = ''
  if (!form.value.name.trim()) { formError.value = '请输入名称'; return }
  try {
    const res = await createToken(form.value.name, form.value.expiresAt || '')
    newToken.value = res.token
    showCreate.value = false
    form.value = { name: '', expiresAt: '' }
    tokensLoaded = false
    tokens.value = await listTokens()
    tokensLoaded = true
  } catch (e: any) { formError.value = e.message }
}

async function handleCopyToken(id: string) {
  await navigator.clipboard.writeText(id)
  copiedTokenId.value = id
  setTimeout(() => { copiedTokenId.value = '' }, 2000)
}

async function handleDelete(id: string) {
  if (!confirm('确认撤销此 Token？撤销后立即失效。')) return
  await deleteToken(id)
  tokens.value = await listTokens()
}

async function copyToken() {
  try {
    await navigator.clipboard.writeText(newToken.value)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch {
    // clipboard 不可用时（HTTP 环境/权限拒绝），静默失败，用户可手动复制
  }
}

function isExpired(expiresAt: string) { return new Date(expiresAt) < new Date() }
</script>

<style scoped>
.token-display {
  display: flex; align-items: center; gap: 8px;
  background: var(--panel); border: 1px solid var(--border);
  border-radius: 8px; padding: 10px 12px; margin-bottom: 16px;
}
.token-code { flex: 1; word-break: break-all; font-size: 12px; color: var(--green); }
.btn-copied { background: rgba(74,222,128,0.15) !important; color: var(--green) !important; border-color: rgba(74,222,128,0.4) !important; }
</style>
