<template>
  <div class="settings-page">
    <div class="settings-container">
      <div class="settings-header">
        <h2>设置</h2>
        <p>配置 Spider 服务参数</p>
      </div>

      <div class="settings-block">
        <div class="block-title">MCP Server</div>
        <div class="block-grid">
          <div class="form-row">
            <label>监听地址</label>
            <input v-model="form.sse_addr" class="input" placeholder=":8000" />
          </div>
          <div class="form-row">
            <label>Base URL</label>
            <input v-model="form.sse_base_url" class="input" placeholder="http://localhost:8000" />
          </div>
        </div>
      </div>

      <div class="settings-block">
        <div class="block-title">SSH 默认配置</div>
        <div class="block-grid">
          <div class="form-row">
            <label>命令超时（秒）</label>
            <input v-model.number="form.ssh_default_timeout_seconds" class="input" type="number" />
          </div>
          <div class="form-row">
            <label>连接池 TTL（秒）</label>
            <input v-model.number="form.ssh_pool_ttl_seconds" class="input" type="number" />
          </div>
          <div class="form-row">
            <label>最大连接数</label>
            <input v-model.number="form.ssh_max_pool_size" class="input" type="number" />
          </div>
        </div>
      </div>

      <div class="settings-block">
        <div class="block-title">LLM 模型配置</div>
        <div v-for="(m, i) in form.llm.models" :key="m._uid" class="model-row">
          <label class="radio-label">
            <input type="radio" :value="m.id" v-model="form.llm.active" />
          </label>
          <input :value="m.id" @input="(e: Event) => { const old = m.id; m.id = (e.target as HTMLInputElement).value; updateLLMId(i, old, m.id) }" class="input input-sm" placeholder="ID" />
          <select v-model="m.provider" class="input input-sm">
            <option value="claude">Claude</option>
            <option value="openai">OpenAI</option>
          </select>
          <input v-model="m.model" class="input input-sm" placeholder="模型名称" />
          <input v-model="m.api_key" class="input input-sm" placeholder="API Key" />
          <input v-model.number="m.max_tokens" class="input input-sm input-num" type="number" placeholder="Max Tokens" />
          <button class="btn-icon btn-del" @click="removeLLMModel(i)">×</button>
        </div>
        <button class="btn btn-outline btn-sm" @click="addLLMModel">+ 添加模型</button>
      </div>

      <div class="settings-block">
        <div class="block-title">Embedding 模型配置</div>
        <div v-for="(m, i) in form.embedding.models" :key="m._uid" class="model-row">
          <label class="radio-label">
            <input type="radio" :value="m.id" v-model="form.embedding.active" />
          </label>
          <input :value="m.id" @input="(e: Event) => { const old = m.id; m.id = (e.target as HTMLInputElement).value; updateEmbeddingId(i, old, m.id) }" class="input input-sm" placeholder="ID" />
          <select v-model="m.provider" class="input input-sm">
            <option value="openai">OpenAI</option>
            <option value="voyage">Voyage</option>
          </select>
          <input v-model="m.model" class="input input-sm" placeholder="模型名称" />
          <input v-model="m.api_key" class="input input-sm" placeholder="API Key" />
          <input v-model.number="m.dimensions" class="input input-sm input-num" type="number" placeholder="维度" />
          <button class="btn-icon btn-del" @click="removeEmbeddingModel(i)">×</button>
        </div>
        <button class="btn btn-outline btn-sm" @click="addEmbeddingModel">+ 添加模型</button>
      </div>

      <div class="settings-footer">
        <button class="btn btn-primary" @click="save">保存设置</button>
        <span v-if="saved" class="ok">已保存</span>
        <span v-if="error" class="err">{{ error }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'

let uidCounter = 0
function uid() { return `_m${++uidCounter}` }

interface LLMModel {
  _uid: string; id: string; provider: string; api_key: string; model: string; max_tokens: number
}
interface EmbeddingModel {
  _uid: string; id: string; provider: string; api_key: string; model: string; dimensions: number
}
interface Settings {
  sse_addr: string
  sse_base_url: string
  ssh_default_timeout_seconds: number
  ssh_pool_ttl_seconds: number
  ssh_max_pool_size: number
  llm: { active: string; models: LLMModel[] }
  embedding: { active: string; models: EmbeddingModel[] }
}

const form = ref<Settings>({
  sse_addr: '', sse_base_url: '',
  ssh_default_timeout_seconds: 30, ssh_pool_ttl_seconds: 300, ssh_max_pool_size: 50,
  llm: { active: '', models: [] },
  embedding: { active: '', models: [] },
})
const saved = ref(false)
const error = ref('')

function addLLMModel() {
  form.value.llm.models.push({ _uid: uid(), id: '', provider: 'claude', api_key: '', model: '', max_tokens: 4096 })
}
function removeLLMModel(idx: number) {
  const m = form.value.llm.models[idx]
  if (m.id === form.value.llm.active) form.value.llm.active = ''
  form.value.llm.models.splice(idx, 1)
}
function addEmbeddingModel() {
  form.value.embedding.models.push({ _uid: uid(), id: '', provider: 'openai', api_key: '', model: '', dimensions: 1536 })
}
function removeEmbeddingModel(idx: number) {
  const m = form.value.embedding.models[idx]
  if (m.id === form.value.embedding.active) form.value.embedding.active = ''
  form.value.embedding.models.splice(idx, 1)
}

function updateLLMId(idx: number, oldId: string, newId: string) {
  if (form.value.llm.active === oldId) form.value.llm.active = newId
}
function updateEmbeddingId(idx: number, oldId: string, newId: string) {
  if (form.value.embedding.active === oldId) form.value.embedding.active = newId
}

async function load() {
  const res = await fetch('/api/v1/settings')
  if (!res.ok) return
  const data = await res.json()
  if (!data.llm) data.llm = { active: '', models: [] }
  if (!data.llm.models) data.llm.models = []
  if (!data.embedding) data.embedding = { active: '', models: [] }
  if (!data.embedding.models) data.embedding.models = []
  data.llm.models.forEach((m: any) => m._uid = uid())
  data.embedding.models.forEach((m: any) => m._uid = uid())
  form.value = data
}

async function save() {
  saved.value = false
  error.value = ''
  const res = await fetch('/api/v1/settings', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(form.value) })
  if (res.ok) { saved.value = true; setTimeout(() => saved.value = false, 2000) }
  else error.value = (await res.json()).error
}

onMounted(load)
</script>

<style scoped>
.settings-page {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 32px 40px;
}

.settings-container {
  max-width: 680px;
}

.settings-header {
  margin-bottom: 24px;
}

.settings-header h2 {
  font-size: 20px;
  font-weight: 700;
  color: var(--text);
  letter-spacing: -0.02em;
  margin-bottom: 4px;
}

.settings-header p {
  font-size: 13px;
  color: var(--muted);
}

.settings-block {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px 24px;
  box-shadow: var(--card-shadow);
  margin-bottom: 16px;
}

.block-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

.block-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.settings-footer {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 48px;
}

.model-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.radio-label { display: flex; align-items: center; flex-shrink: 0; }
.radio-label input[type="radio"] { accent-color: var(--primary); }
.input-sm { padding: 6px 8px !important; font-size: 12px !important; }
.input-num { width: 90px; flex-shrink: 0; }
.btn-icon { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 18px; padding: 2px 6px; }
.btn-icon:hover { color: var(--red); }
.btn-outline { background: transparent; border: 1px solid var(--border); color: var(--text-sub); padding: 5px 14px; border-radius: 6px; cursor: pointer; font-size: 12px; margin-top: 4px; }
.btn-outline:hover { border-color: var(--primary); color: var(--primary); }
.btn-sm { font-size: 12px; }
</style>
