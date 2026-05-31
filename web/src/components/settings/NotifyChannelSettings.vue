<template>
  <div class="detail-topbar">
    <span class="detail-title">通知渠道</span>
    <button class="btn btn-primary btn-sm" @click="showAddChannelModal = true">添加渠道</button>
  </div>
  <div class="detail-body">
    <p v-if="notifyErrMsg" class="err" style="margin-bottom:12px">{{ notifyErrMsg }}</p>
    <p v-if="notifyLoading" class="dim" style="text-align:center;padding:24px 0">加载中…</p>
    <table v-else-if="notifyChannels.length > 0" class="table">
      <thead>
        <tr><th>名称</th><th>类型</th><th>状态</th><th>创建时间</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="ch in notifyChannels" :key="ch.id">
          <td>{{ ch.name }}</td>
          <td>{{ ch.type === 'dingtalk' ? '钉钉' : ch.type }}</td>
          <td><span :class="ch.enabled ? 'badge badge-ok' : 'badge'">{{ ch.enabled ? '启用' : '禁用' }}</span></td>
          <td>{{ formatNotifyDate(ch.created_at) }}</td>
          <td>
            <div class="actions">
              <button class="btn btn-sm" @click="toggleNotify(ch)" :disabled="notifyToggling === ch.id">{{ ch.enabled ? '禁用' : '启用' }}</button>
              <template v-if="notifyPendingDeleteId === ch.id">
                <span style="font-size:13px;color:var(--text-sub)">确认删除?</span>
                <button class="btn btn-sm btn-danger" @click="doDeleteNotify(ch.id)" :disabled="notifyDeleting === ch.id">确认</button>
                <button class="btn btn-sm" @click="notifyPendingDeleteId = null">取消</button>
              </template>
              <button v-else class="btn btn-sm btn-danger" @click="notifyPendingDeleteId = ch.id">删除</button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-else-if="!notifyLoading" class="dim" style="text-align:center;padding:24px 0">暂无通知渠道</p>
  </div>

  <!-- 添加通知渠道弹窗 -->
  <div v-if="showAddChannelModal" class="modal-overlay" @click.self="closeAddChannelModal">
    <div class="modal">
      <h3>添加通知渠道</h3>
      <div class="form-row"><label>名称</label><input v-model="channelForm.name" class="input" placeholder="渠道名称" required /></div>
      <div class="form-row">
        <label>类型</label>
        <select v-model="channelForm.type" class="input"><option value="dingtalk">钉钉</option></select>
      </div>
      <div class="form-row"><label>Webhook URL</label><input v-model="channelForm.webhook_url" class="input" placeholder="https://oapi.dingtalk.com/robot/send?access_token=..." required /></div>
      <div class="form-row"><label>Secret（可选）</label><input v-model="channelForm.secret" class="input" placeholder="加签密钥，可留空" /></div>
      <div class="form-row" style="flex-direction:row;align-items:center;gap:8px">
        <input type="checkbox" v-model="channelForm.enabled" id="ch-enabled" style="width:auto" />
        <label for="ch-enabled" style="text-transform:none;letter-spacing:normal;font-size:14px;color:var(--text-sub)">启用</label>
      </div>
      <p v-if="addChannelErrMsg" class="err" style="margin-bottom:8px">{{ addChannelErrMsg }}</p>
      <div class="modal-footer">
        <button class="btn" @click="closeAddChannelModal">取消</button>
        <button class="btn btn-primary" @click="handleAddChannel" :disabled="addingChannel">{{ addingChannel ? '添加中…' : '添加' }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  listNotifyChannels,
  createNotifyChannel,
  toggleNotifyChannel,
  deleteNotifyChannel,
  type NotifyChannel,
} from '../../api/notify-channels'

const notifyChannels = ref<NotifyChannel[]>([])
const notifyErrMsg = ref('')
const notifyLoading = ref(false)
const notifyToggling = ref<number | null>(null)
const notifyDeleting = ref<number | null>(null)
const notifyPendingDeleteId = ref<number | null>(null)
let notifyLoaded = false

const showAddChannelModal = ref(false)
const addingChannel = ref(false)
const addChannelErrMsg = ref('')
const channelForm = ref({ name: '', type: 'dingtalk', webhook_url: '', secret: '', enabled: true })

async function loadNotifyChannels() {
  if (notifyLoaded) return
  notifyLoaded = true
  notifyLoading.value = true
  try {
    notifyChannels.value = await listNotifyChannels()
  } catch (e: unknown) {
    notifyErrMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    notifyLoading.value = false
  }
}

function formatNotifyDate(s: string): string {
  if (!s) return ''
  return new Date(s).toLocaleString('zh-CN', { hour12: false })
}

async function toggleNotify(ch: NotifyChannel) {
  notifyToggling.value = ch.id
  try {
    const updated = await toggleNotifyChannel(ch.id, !ch.enabled)
    const idx = notifyChannels.value.findIndex(c => c.id === ch.id)
    if (idx !== -1) notifyChannels.value[idx] = updated
  } catch (e: unknown) {
    notifyErrMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    notifyToggling.value = null
  }
}

async function doDeleteNotify(id: number) {
  notifyDeleting.value = id
  try {
    await deleteNotifyChannel(id)
    notifyChannels.value = notifyChannels.value.filter(c => c.id !== id)
    notifyPendingDeleteId.value = null
  } catch (e: unknown) {
    notifyErrMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    notifyDeleting.value = null
  }
}

function closeAddChannelModal() {
  showAddChannelModal.value = false
  addChannelErrMsg.value = ''
  channelForm.value = { name: '', type: 'dingtalk', webhook_url: '', secret: '', enabled: true }
}

async function handleAddChannel() {
  if (!channelForm.value.name.trim()) { addChannelErrMsg.value = '请填写名称'; return }
  if (!channelForm.value.webhook_url.trim()) { addChannelErrMsg.value = '请填写 Webhook URL'; return }
  addingChannel.value = true
  addChannelErrMsg.value = ''
  try {
    const config = JSON.stringify({ webhook_url: channelForm.value.webhook_url, secret: channelForm.value.secret })
    const ch = await createNotifyChannel({ type: channelForm.value.type, name: channelForm.value.name, config, enabled: channelForm.value.enabled })
    notifyChannels.value.push(ch)
    closeAddChannelModal()
  } catch (e: unknown) {
    addChannelErrMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    addingChannel.value = false
  }
}

onMounted(() => {
  loadNotifyChannels()
})
</script>
