<template>
  <div class="code-block">
    <div class="line-numbers" ref="gutterRef">
      <div v-for="n in lineCount" :key="n" class="line-number">{{ n }}</div>
    </div>
    <div
      v-if="html"
      class="hl-wrap"
      ref="contentRef"
      v-html="html"
      @scroll="syncScroll"
    />
    <pre
      v-else
      class="plain-output"
      ref="contentRef"
      @scroll="syncScroll"
    >{{ code }}</pre>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps<{
  code: string
  html?: string
}>()

const gutterRef = ref<HTMLElement | null>(null)
const contentRef = ref<HTMLElement | null>(null)

const lineCount = computed(() => Math.max(props.code.split('\n').length, 1))

function syncScroll() {
  if (gutterRef.value && contentRef.value) {
    gutterRef.value.scrollTop = contentRef.value.scrollTop
  }
}

watch(() => props.code, () => nextTick(syncScroll))
</script>

<style scoped>
.code-block {
  display: flex;
  overflow: hidden;
}

.line-numbers {
  flex-shrink: 0;
  width: 36px;
  overflow: hidden;
  background: var(--panel);
  border-right: 1px solid var(--border);
  padding: 12px 0;
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

.hl-wrap {
  flex: 1;
  overflow: auto;
  max-height: 400px;
}

.hl-wrap :deep(pre.shiki) {
  margin: 0;
  padding: 12px 14px;
  border-radius: 0;
  font-size: 13px;
  line-height: 1.6em;
  overflow-x: auto;
  overflow-y: auto;
  max-height: 400px;
  white-space: pre;
  font-family: 'SF Mono', Consolas, 'Courier New', monospace;
}

.plain-output {
  flex: 1;
  margin: 0;
  padding: 12px 14px;
  font-size: 13px;
  line-height: 1.6em;
  font-family: 'SF Mono', Consolas, 'Courier New', monospace;
  color: var(--text);
  background: transparent;
  border: none;
  overflow: auto;
  max-height: 400px;
  white-space: pre;
}
</style>
