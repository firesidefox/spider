<template>
  <div class="code-editor" @click="focusTextarea">
    <div class="line-numbers" ref="gutterRef">
      <div v-for="n in lineCount" :key="n" class="line-number">{{ n }}</div>
    </div>
    <textarea
      ref="taRef"
      :value="modelValue"
      :placeholder="placeholder"
      class="code-textarea"
      spellcheck="false"
      autocomplete="off"
      autocorrect="off"
      autocapitalize="off"
      @input="onInput"
      @scroll="syncScroll"
      @keydown="$emit('keydown', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, watch } from 'vue'

const props = defineProps<{
  modelValue: string
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'keydown': [event: KeyboardEvent]
}>()

const taRef = ref<HTMLTextAreaElement | null>(null)
const gutterRef = ref<HTMLDivElement | null>(null)

const lineCount = computed(() => {
  const lines = props.modelValue.split('\n').length
  return Math.max(lines, 1)
})

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLTextAreaElement).value)
}

function syncScroll() {
  if (gutterRef.value && taRef.value) {
    gutterRef.value.scrollTop = taRef.value.scrollTop
  }
}

function focusTextarea() {
  taRef.value?.focus()
}

watch(() => props.modelValue, () => {
  nextTick(syncScroll)
})
</script>

<style scoped>
.code-editor {
  display: flex;
  flex: 1;
  min-height: 72px;
  max-height: 200px;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  background: var(--input-bg);
  cursor: text;
  transition: border-color 0.15s;
}

.code-editor:focus-within {
  border-color: var(--border-focus);
}

.line-numbers {
  flex-shrink: 0;
  width: 36px;
  overflow: hidden;
  background: var(--panel);
  border-right: 1px solid var(--border);
  padding: 8px 0;
  user-select: none;
}

.line-number {
  height: 1.6em;
  line-height: 1.6em;
  text-align: right;
  padding-right: 8px;
  font-size: 12px;
  font-family: 'SF Mono', Consolas, 'Courier New', monospace;
  color: var(--muted);
}

.code-textarea {
  flex: 1;
  resize: none;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text);
  font-size: 13px;
  line-height: 1.6em;
  font-family: 'SF Mono', Consolas, 'Courier New', monospace;
  padding: 8px 10px;
  overflow-y: auto;
  white-space: pre;
  word-break: normal;
  overflow-x: auto;
}

.code-textarea::placeholder {
  color: var(--muted);
}
</style>
