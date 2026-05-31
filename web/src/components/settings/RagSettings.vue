<template>
  <div class="detail-topbar">
    <span class="detail-title">知识库</span>
  </div>
  <div class="detail-body">
    <div v-if="ragConfigError" class="err" style="margin-bottom:12px">{{ ragConfigError }}</div>
    <div class="edit-card emb-card">
      <!-- 卡头 -->
      <div class="emb-card-header">
        <div class="emb-card-identity">
          <div class="emb-card-icon">🧠</div>
          <div>
            <div class="emb-card-title">Embedding 模型</div>
            <div class="emb-card-subtitle dim">{{ ragConfig.name || ragConfig.model || '未配置' }}</div>
          </div>
        </div>
        <div class="emb-card-header-right">
          <span v-if="ragConfig.validated_at" class="status-badge ok">✓ 已验证</span>
          <span v-else-if="ragConfig.base_url" class="status-badge" style="border-color:var(--border)">未验证</span>
          <button v-if="!ragConfigEditing" class="btn btn-sm" @click="startEditRagConfig">编辑</button>
        </div>
      </div>

      <!-- 只读态 -->
      <template v-if="!ragConfigEditing">
        <div class="emb-divider"></div>
        <div class="emb-fields">
          <div class="emb-field">
            <span class="emb-field-label">供应商</span>
            <span class="emb-field-value">{{ ragConfig.name || '—' }}</span>
          </div>
          <div class="emb-field">
            <span class="emb-field-label">接口类型</span>
            <span class="emb-field-value">
              <span class="mc-tag-inline">{{ ragConfig.type === 'anthropic' ? 'Anthropic 兼容' : 'OpenAI 兼容' }}</span>
            </span>
          </div>
          <div class="emb-field">
            <span class="emb-field-label">请求地址</span>
            <span class="emb-field-value">{{ ragConfig.base_url || '—' }}</span>
          </div>
          <div class="emb-field">
            <span class="emb-field-label">APIKey</span>
            <span class="emb-field-value dim">{{ ragConfig.api_key_set ? '已配置' : '—' }}</span>
          </div>
        </div>
      </template>

      <!-- 编辑态 -->
      <template v-else>
        <div class="emb-divider"></div>
        <!-- 行1：供应商名称 | 接口类型 -->
        <div class="emb-form-grid">
          <div class="emb-form-col">
            <label class="emb-label">供应商名称</label>
            <input v-model="ragConfigForm.name" class="input" placeholder="如 OpenAI、MiniMax（仅标识）" />
          </div>
          <div class="emb-form-col">
            <label class="emb-label">接口类型</label>
            <select v-model="ragConfigForm.type" class="input" @change="onBaseUrlChange">
              <option value="openai">OpenAI 兼容</option>
              <option value="anthropic">Anthropic 兼容</option>
            </select>
          </div>
        </div>
        <!-- 行2：请求地址 | APIKey -->
        <div class="emb-form-grid" style="margin-bottom:4px">
          <div class="emb-form-col">
            <label class="emb-label">请求地址</label>
            <input v-model="ragConfigForm.base_url" class="input"
              placeholder="https://api.openai.com/v1"
              list="provider-urls" @change="onBaseUrlChange" @input="onBaseUrlInput" />
            <datalist id="provider-urls">
              <option v-for="p in kbProviders" :key="p.id" :value="p.base_url">{{ p.name }}</option>
            </datalist>
          </div>
          <div class="emb-form-col">
            <label class="emb-label">APIKey</label>
            <input v-model="ragConfigForm.api_key" class="input" type="password"
              :placeholder="ragConfig.api_key_set ? '已设置，留空保留原值' : 'API Key'"
              @input="clearModelCache" />
          </div>
        </div>
        <!-- URL hint + 查询按钮 -->
        <div class="emb-url-hint-row">
          <span class="emb-url-hint">
            查询接口：<span class="emb-url-hint-url">{{ ragConfigForm.base_url ? ragConfigForm.base_url.replace(/\/$/, '') + '/v1/models' : '—' }}</span>
          </span>
          <button class="btn btn-amber btn-sm" :disabled="!ragConfigForm.base_url || kbFetchingModels" @click="fetchEmbeddingModels">
            {{ kbFetchingModels ? '查询中…' : '查询模型列表' }}
          </button>
        </div>
        <!-- 模型 ID -->
        <div class="emb-form-col" style="margin-bottom:8px">
          <label class="emb-label">模型 ID</label>
          <input v-model="ragConfigForm.model" class="input" placeholder="可手动输入，或点击下方快速选择" />
        </div>
        <!-- chips -->
        <div v-if="kbModelOptions.length">
          <div class="emb-chips-label">可用模型（点击快速选择）<span v-if="kbFetchedAt" class="emb-fetched-at">{{ kbFetchedAt }}</span></div>
          <div class="emb-chips">
            <span v-for="m in kbModelOptions" :key="m"
              class="emb-chip" :class="{ active: ragConfigForm.model === m }"
              @click="ragConfigForm.model = m">{{ m }}</span>
          </div>
        </div>
        <div v-if="kbFetchModelsError" class="err" style="font-size:12px;margin-top:4px">{{ kbFetchModelsError }}</div>
        <div class="emb-edit-actions">
          <button class="btn btn-primary btn-sm" :disabled="ragConfigSaving" @click="saveRagConfig">
            {{ ragConfigSaving ? '保存中…' : '保存' }}
          </button>
          <button class="btn btn-sm" @click="cancelRagConfigEdit">取消</button>
          <button class="btn btn-sm" :disabled="kbValidating" @click="validateRagConfig">
            {{ kbValidating ? '验证中…' : '验证' }}
          </button>
          <span v-if="kbValidateResult === 'ok'" style="color:var(--green);font-size:12px">✓ 有效</span>
          <span v-else-if="kbValidateResult === 'error'" style="color:var(--red);font-size:12px">{{ kbValidateError }}</span>
        </div>
        <div v-if="ragConfigSaveError" class="err" style="margin-top:8px;font-size:13px">{{ ragConfigSaveError }}</div>
      </template>

      <div v-if="ragConfigOk" style="margin-top:8px;font-size:13px;color:var(--green)">已保存 ✓</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { authHeaders } from '../../api/auth'

interface ProviderModel { model_id: string; display_name: string }
interface Provider {
  id: string; name: string; type: string; base_url: string
  api_key: string
  selected_model: string; is_active: boolean
  models: ProviderModel[]
  created_at: string; updated_at: string
}

const ragConfig = ref({ name: '', type: 'openai', base_url: '', model: '', api_key_set: false, validated_at: '' })
const ragConfigForm = ref({ name: '', type: 'openai', base_url: '', model: '', api_key: '' })
const ragConfigEditing = ref(false)
const ragConfigSaving = ref(false)
const ragConfigError = ref('')
const ragConfigSaveError = ref('')
const ragConfigOk = ref(false)
let ragConfigLoaded = false

const kbProviders = ref<Provider[]>([])
const kbModelOptions = ref<string[]>([])
const kbFetchingModels = ref(false)
const kbFetchModelsError = ref('')
const kbFetchedAt = ref('')
const kbValidating = ref(false)
const kbValidateResult = ref<'ok' | 'error' | ''>('')
const kbValidateError = ref('')

async function loadRagConfig() {
  if (ragConfigLoaded) return
  ragConfigLoaded = true
  ragConfigError.value = ''
  try {
    const [ragRes, provRes] = await Promise.all([
      fetch('/api/v1/rag-config', { headers: authHeaders() }),
      fetch('/api/v1/providers', { headers: authHeaders() }),
    ])
    if (ragRes.ok) {
      const data = await ragRes.json()
      ragConfig.value = data
      ragConfigForm.value = { name: data.name ?? '', type: data.type ?? 'openai', base_url: data.base_url ?? '', model: data.model ?? '', api_key: '' }
      if (data.cached_models?.length) {
        kbModelOptions.value = data.cached_models
        kbFetchedAt.value = '已缓存'
      }
    }
    if (provRes.ok) {
      kbProviders.value = await provRes.json()
    }
  } catch (e: any) {
    ragConfigError.value = e.message
  }
}

function startEditRagConfig() {
  ragConfigForm.value = { name: ragConfig.value.name, type: ragConfig.value.type || 'openai', base_url: ragConfig.value.base_url, model: ragConfig.value.model, api_key: '' }
  kbFetchModelsError.value = ''
  kbValidateResult.value = ragConfig.value.validated_at ? 'ok' : ''
  kbValidateError.value = ''
  ragConfigEditing.value = true
}

function cancelRagConfigEdit() {
  ragConfigEditing.value = false
  kbValidateResult.value = ''
  kbFetchModelsError.value = ''
}

function saveCachedModels(models: string[]) {
  const body: any = {
    name: ragConfigForm.value.name,
    type: ragConfigForm.value.type || 'openai',
    base_url: ragConfigForm.value.base_url,
    model: ragConfigForm.value.model,
    cached_models: models,
  }
  fetch('/api/v1/rag-config', {
    method: 'PUT',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => {})
}

function clearModelCache() {
  const hadCache = kbModelOptions.value.length > 0
  kbModelOptions.value = []
  kbFetchedAt.value = ''
  kbFetchModelsError.value = ''
  kbValidateResult.value = ''
  kbValidateError.value = ''
  if (hadCache) saveCachedModels([])
}

function onBaseUrlChange() {
  clearModelCache()
  const url = ragConfigForm.value.base_url
  const match = kbProviders.value.find(p => p.base_url === url)
  if (match) {
    if (match.api_key) ragConfigForm.value.api_key = match.api_key
  }
}

function onBaseUrlInput() {
  clearModelCache()
  const url = ragConfigForm.value.base_url
  const match = kbProviders.value.find(p => p.base_url === url)
  if (match) {
    if (match.api_key) ragConfigForm.value.api_key = match.api_key
  }
}

async function fetchEmbeddingModels() {
  kbFetchingModels.value = true
  kbFetchModelsError.value = ''
  try {
    const res = await fetch('/api/v1/rag-config/models', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({
        type: ragConfigForm.value.type || 'openai',
        base_url: ragConfigForm.value.base_url,
        api_key: ragConfigForm.value.api_key,
      }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      kbFetchModelsError.value = err.error || '获取模型失败'
      return
    }
    const models: { id: string }[] = await res.json()
    kbModelOptions.value = models.map(m => m.id)
    kbFetchedAt.value = new Date().toLocaleTimeString()
    saveCachedModels(kbModelOptions.value)
  } catch (e: any) {
    kbFetchModelsError.value = e.message
  } finally {
    kbFetchingModels.value = false
  }
}

async function validateRagConfig() {
  kbValidating.value = true
  kbValidateResult.value = ''
  kbValidateError.value = ''
  try {
    const res = await fetch('/api/v1/rag-config/validate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({
        type: ragConfigForm.value.type || 'openai',
        base_url: ragConfigForm.value.base_url,
        api_key: ragConfigForm.value.api_key,
        model: ragConfigForm.value.model,
      }),
    })
    if (res.ok) {
      kbValidateResult.value = 'ok'
    } else {
      const err = await res.json().catch(() => ({}))
      kbValidateResult.value = 'error'
      kbValidateError.value = err.error || '验证失败'
    }
  } catch (e: any) {
    kbValidateResult.value = 'error'
    kbValidateError.value = e.message
  } finally {
    kbValidating.value = false
  }
}

async function saveRagConfig() {
  ragConfigSaveError.value = ''
  ragConfigOk.value = false
  ragConfigSaving.value = true
  try {
    const body: any = { name: ragConfigForm.value.name, type: ragConfigForm.value.type || 'openai', base_url: ragConfigForm.value.base_url, model: ragConfigForm.value.model, cached_models: kbModelOptions.value }
    if (ragConfigForm.value.api_key) body.api_key = ragConfigForm.value.api_key
    const res = await fetch('/api/v1/rag-config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      ragConfigSaveError.value = err.error || '保存失败'
      return
    }
    const saved = await res.json()
    ragConfig.value = saved
    if (!saved.cached_models?.length) kbModelOptions.value = []
    ragConfigForm.value.api_key = ''
    ragConfigEditing.value = false
    ragConfigOk.value = true
    setTimeout(() => { ragConfigOk.value = false }, 2000)
  } catch (e: any) {
    ragConfigSaveError.value = e.message
  } finally {
    ragConfigSaving.value = false
  }
}

onMounted(() => {
  loadRagConfig()
})
</script>

<style scoped>
.detail-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  flex-shrink: 0;
}

.detail-title { font-size: 15px; font-weight: 700; color: var(--text); }

.detail-body { flex: 1; overflow-y: auto; padding: 20px 24px; }

.edit-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 20px 24px;
  box-shadow: var(--card-shadow);
  margin-bottom: 16px;
}

.status-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 4px; border: 1px solid;
}
.status-badge.ok { background: rgba(74,222,128,0.12); color: var(--green); border-color: rgba(74,222,128,0.3); }

.emb-card-header {
  display: flex; justify-content: space-between; align-items: center;
}
.emb-card-identity { display: flex; align-items: center; gap: 12px; }
.emb-card-icon {
  width: 36px; height: 36px; border-radius: 8px;
  background: rgba(99,102,241,0.12); border: 1px solid rgba(99,102,241,0.25);
  display: flex; align-items: center; justify-content: center; font-size: 18px;
  flex-shrink: 0;
}
.emb-card-title { font-size: 14px; font-weight: 600; color: var(--text); }
.emb-card-subtitle { font-size: 12px; margin-top: 2px; }
.emb-card-header-right { display: flex; align-items: center; gap: 8px; }
.emb-divider { border: none; border-top: 1px solid var(--border); margin: 14px 0; }
.emb-fields { display: flex; flex-direction: column; gap: 8px; }
.emb-field { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.emb-field-label {
  width: 64px; flex-shrink: 0;
  font-size: 11px; font-weight: 600; color: var(--muted);
  text-transform: uppercase; letter-spacing: 0.06em;
}
.emb-field-value { color: var(--text); }
.mc-tag-inline {
  display: inline-block; font-size: 11px; padding: 1px 7px; border-radius: 4px;
  background: rgba(96,165,250,0.12); color: #60a5fa; border: 1px solid rgba(96,165,250,0.25);
}
.btn-amber {
  background: #d97706; color: #fff; border: none; white-space: nowrap;
  flex-shrink: 0;
}
.btn-amber:hover:not(:disabled) { background: #b45309; }
.btn-amber:disabled { background: #d97706; opacity: 0.5; cursor: not-allowed; }
.emb-form-grid {
  display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 10px;
}
.emb-form-col {
  display: flex; flex-direction: column; gap: 4px;
}
.emb-label {
  font-size: 12px; color: var(--text-2); font-weight: 500;
}
.emb-url-hint-row {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
  margin-bottom: 10px; padding: 6px 10px; border-radius: 6px;
  background: rgba(255,255,255,0.03); border: 1px solid var(--border);
}
.emb-url-hint { font-size: 12px; color: var(--text-2); }
.emb-url-hint-url { color: var(--text-1); font-family: monospace; word-break: break-all; }
.emb-chips-label { font-size: 11px; color: var(--text-2); margin-bottom: 6px; }
.emb-fetched-at { margin-left: 8px; font-size: 10px; color: var(--text-3, #64748b); }
.emb-chips {
  display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px;
}
.emb-chip {
  padding: 3px 10px; border-radius: 12px; font-size: 12px; cursor: pointer;
  border: 1px solid var(--border); color: var(--text-2); background: transparent;
  transition: all .15s;
}
.emb-chip:hover { border-color: var(--primary); color: var(--primary); }
.emb-chip.active { background: var(--primary); color: #fff; border-color: var(--primary); }
.emb-edit-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
</style>
