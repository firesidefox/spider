<template>
  <div>
    <button class="btn btn-sm" @click="showPwModal = true">修改密码</button>

    <!-- 修改密码弹窗 -->
    <div v-if="showPwModal" class="modal-overlay" @click.self="closeModal()">
      <div class="modal">
        <h3>修改密码</h3>
        <div class="form-row"><label>旧密码</label><input v-model="pw.old" type="password" class="input" placeholder="当前密码" /></div>
        <div class="form-row"><label>新密码</label><input v-model="pw.new1" type="password" class="input" placeholder="至少 6 位" /></div>
        <div class="form-row"><label>确认新密码</label><input v-model="pw.new2" type="password" class="input" placeholder="再次输入新密码" /></div>
        <div v-if="pwError" class="err" style="margin-bottom:10px">{{ pwError }}</div>
        <div v-if="pwSuccess" class="ok" style="margin-bottom:10px">{{ pwSuccess }}</div>
        <div class="modal-footer">
          <button class="btn" @click="closeModal()">取消</button>
          <button class="btn btn-primary" @click="handleChangePassword" :disabled="pwLoading">{{ pwLoading ? '保存中…' : '保存密码' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { authHeaders } from '../../api/auth'

const pw = ref({ old: '', new1: '', new2: '' })
const pwError = ref('')
const pwSuccess = ref('')
const pwLoading = ref(false)
const showPwModal = ref(false)

function closeModal() {
  showPwModal.value = false
  pw.value = { old: '', new1: '', new2: '' }
  pwError.value = ''
  pwSuccess.value = ''
}

async function handleChangePassword() {
  pwError.value = ''
  pwSuccess.value = ''
  if (!pw.value.old) { pwError.value = '请输入旧密码'; return }
  if (pw.value.new1.length < 6) { pwError.value = '新密码至少 6 位'; return }
  if (pw.value.new1 !== pw.value.new2) { pwError.value = '两次新密码不一致'; return }
  pwLoading.value = true
  try {
    const res = await fetch('/api/v1/me/password', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ old_password: pw.value.old, new_password: pw.value.new1 }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      pwError.value = res.status === 403 ? '旧密码错误' : (data.error || '修改失败')
      return
    }
    pw.value = { old: '', new1: '', new2: '' }
    pwSuccess.value = '密码已修改'
    setTimeout(() => { pwSuccess.value = ''; showPwModal.value = false }, 1500)
  } catch { pwError.value = '修改失败' }
  finally { pwLoading.value = false }
}
</script>
