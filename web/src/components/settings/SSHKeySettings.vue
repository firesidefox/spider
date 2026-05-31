<template>
  <div class="detail-topbar">
    <span class="detail-title">SSH Keys</span>
    <button class="btn btn-primary btn-sm" @click="showAddKey = true">+ 添加 Key</button>
  </div>
  <div class="detail-body">
    <div class="edit-card">
      <p class="dim" style="margin-bottom:16px;font-size:13px">管理 SSH 私钥，可在添加主机时引用。</p>
      <table class="table">
        <thead><tr><th>名称</th><th>指纹</th><th>创建时间</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="k in sshKeys" :key="k.id">
            <td style="font-weight:500;color:var(--text)">{{ k.name }}</td>
            <td class="dim" style="font-family:'SF Mono',Consolas,monospace;font-size:12px">{{ k.fingerprint.slice(0, 24) }}…</td>
            <td class="dim">{{ new Date(k.created_at).toLocaleString() }}</td>
            <td><button class="btn btn-sm btn-danger" @click="handleDeleteKey(k.id)">删除</button></td>
          </tr>
          <tr v-if="sshKeys.length === 0">
            <td colspan="4" class="dim" style="text-align:center;padding:32px">暂无 SSH Key</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>

  <!-- 添加 SSH Key 弹窗 -->
  <div v-if="showAddKey" class="modal-overlay" @click.self="showAddKey = false">
    <div class="modal">
      <h3>添加 SSH Key</h3>
      <div class="form-row"><label>名称</label><input v-model="keyForm.name" class="input" placeholder="prod-key" /></div>
      <div class="form-row">
        <label>私钥内容</label>
        <textarea v-model="keyForm.privateKey" class="input" rows="5" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" />
      </div>
      <div class="form-row"><label>Passphrase（可选）</label><input v-model="keyForm.passphrase" type="password" class="input" /></div>
      <div v-if="keyFormError" class="err" style="margin-bottom:12px">{{ keyFormError }}</div>
      <div class="modal-footer">
        <button class="btn" @click="showAddKey = false">取消</button>
        <button class="btn btn-primary" @click="handleAddKey">添加</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listSSHKeys, createSSHKey, deleteSSHKey } from '../../api/ssh-keys'
import type { SafeSSHKey } from '../../api/ssh-keys'

const sshKeys = ref<SafeSSHKey[]>([])
const showAddKey = ref(false)
const keyForm = ref({ name: '', privateKey: '', passphrase: '' })
const keyFormError = ref('')
let sshKeysLoaded = false

async function loadSSHKeys() {
  if (sshKeysLoaded) return
  sshKeysLoaded = true
  sshKeys.value = await listSSHKeys()
}

async function handleAddKey() {
  keyFormError.value = ''
  if (!keyForm.value.name.trim()) { keyFormError.value = '请输入名称'; return }
  if (!keyForm.value.privateKey.trim()) { keyFormError.value = '请输入私钥内容'; return }
  try {
    await createSSHKey(keyForm.value.name, keyForm.value.privateKey, keyForm.value.passphrase || '')
    showAddKey.value = false
    keyForm.value = { name: '', privateKey: '', passphrase: '' }
    sshKeysLoaded = false
    sshKeys.value = await listSSHKeys()
    sshKeysLoaded = true
  } catch (e: any) { keyFormError.value = e.message }
}

async function handleDeleteKey(id: string) {
  if (!confirm('确认删除此 SSH Key？')) return
  try {
    await deleteSSHKey(id)
    sshKeys.value = await listSSHKeys()
  } catch (e: any) { alert(e.message) }
}

onMounted(() => {
  loadSSHKeys()
})
</script>
