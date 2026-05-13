<template>
  <div class="page-content">
    <div class="page-header">
      <h2>设置</h2>
    </div>

    <div class="settings-card">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px;padding-bottom:12px;border-bottom:1px solid var(--border)">
        <h3 style="margin:0;padding:0;border:none">通知渠道</h3>
        <button class="btn btn-primary btn-sm" @click="showAddModal = true">添加渠道</button>
      </div>

      <p v-if="errMsg" class="err" style="margin-bottom:12px">{{ errMsg }}</p>

      <table v-if="channels.length > 0" class="table">
        <thead>
          <tr>
            <th>名称</th>
            <th>类型</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ch in channels" :key="ch.id">
            <td>{{ ch.name }}</td>
            <td>{{ ch.type === 'dingtalk' ? '钉钉' : ch.type }}</td>
            <td>
              <span :class="ch.enabled ? 'badge badge-ok' : 'badge'">
                {{ ch.enabled ? '启用' : '禁用' }}
              </span>
            </td>
            <td>{{ formatDate(ch.created_at) }}</td>
            <td>
              <div class="actions">
                <button class="btn btn-sm" @click="toggleEnabled(ch)" :disabled="toggling === ch.id">
                  {{ ch.enabled ? '禁用' : '启用' }}
                </button>
                <template v-if="pendingDeleteId === ch.id">
                  <span style="font-size:13px;color:var(--text-sub)">确认删除?</span>
                  <button class="btn btn-sm btn-danger" @click="doDelete(ch.id)" :disabled="deleting === ch.id">确认</button>
                  <button class="btn btn-sm" @click="pendingDeleteId = null">取消</button>
                </template>
                <button v-else class="btn btn-sm btn-danger" @click="pendingDeleteId = ch.id">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <p v-else class="dim" style="text-align:center;padding:24px 0">暂无通知渠道</p>
    </div>

    <!-- Add Channel Modal -->
    <div v-if="showAddModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal">
        <h3>添加通知渠道</h3>

        <div class="form-row">
          <label>名称</label>
          <input v-model="form.name" class="input" placeholder="渠道名称" required />
        </div>

        <div class="form-row">
          <label>类型</label>
          <select v-model="form.type" class="input">
            <option value="dingtalk">钉钉</option>
          </select>
        </div>

        <div class="form-row">
          <label>Webhook URL</label>
          <input v-model="form.webhook_url" class="input" placeholder="https://oapi.dingtalk.com/robot/send?access_token=..." required />
        </div>

        <div class="form-row">
          <label>Secret（可选）</label>
          <input v-model="form.secret" class="input" placeholder="加签密钥，可留空" />
        </div>

        <div class="form-row" style="flex-direction:row;align-items:center;gap:8px">
          <input type="checkbox" v-model="form.enabled" id="ch-enabled" style="width:auto" />
          <label for="ch-enabled" style="text-transform:none;letter-spacing:normal;font-size:14px;color:var(--text-sub)">启用</label>
        </div>

        <p v-if="addErrMsg" class="err" style="margin-bottom:8px">{{ addErrMsg }}</p>

        <div class="modal-footer">
          <button class="btn" @click="closeModal">取消</button>
          <button class="btn btn-primary" @click="handleAdd" :disabled="adding">
            {{ adding ? '添加中…' : '添加' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  listNotifyChannels,
  createNotifyChannel,
  updateNotifyChannel,
  deleteNotifyChannel,
  type NotifyChannel,
} from '../api/notify-channels'

const channels = ref<NotifyChannel[]>([])
const errMsg = ref('')
const toggling = ref<number | null>(null)
const deleting = ref<number | null>(null)
const pendingDeleteId = ref<number | null>(null)

const showAddModal = ref(false)
const adding = ref(false)
const addErrMsg = ref('')

const form = ref({
  name: '',
  type: 'dingtalk',
  webhook_url: '',
  secret: '',
  enabled: true,
})

async function load() {
  try {
    channels.value = await listNotifyChannels()
  } catch (e: unknown) {
    errMsg.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(load)

function formatDate(s: string): string {
  return new Date(s).toLocaleString('zh-CN', { hour12: false })
}

async function toggleEnabled(ch: NotifyChannel) {
  toggling.value = ch.id
  try {
    const updated = await updateNotifyChannel(ch.id, { enabled: !ch.enabled })
    const idx = channels.value.findIndex(c => c.id === ch.id)
    if (idx !== -1) channels.value[idx] = updated
  } catch (e: unknown) {
    errMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    toggling.value = null
  }
}

async function doDelete(id: number) {
  deleting.value = id
  try {
    await deleteNotifyChannel(id)
    channels.value = channels.value.filter(c => c.id !== id)
    pendingDeleteId.value = null
  } catch (e: unknown) {
    errMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    deleting.value = null
  }
}

function closeModal() {
  showAddModal.value = false
  addErrMsg.value = ''
  form.value = { name: '', type: 'dingtalk', webhook_url: '', secret: '', enabled: true }
}

async function handleAdd() {
  if (!form.value.name.trim()) { addErrMsg.value = '请填写名称'; return }
  if (!form.value.webhook_url.trim()) { addErrMsg.value = '请填写 Webhook URL'; return }
  adding.value = true
  addErrMsg.value = ''
  try {
    const config = JSON.stringify({ webhook_url: form.value.webhook_url, secret: form.value.secret })
    const ch = await createNotifyChannel({ type: form.value.type, name: form.value.name, config, enabled: form.value.enabled })
    channels.value.push(ch)
    closeModal()
  } catch (e: unknown) {
    addErrMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    adding.value = false
  }
}
</script>
