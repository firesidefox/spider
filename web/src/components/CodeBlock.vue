<template>
  <!-- wrap 模式：逐行渲染，行号与内容同行对齐，高度自适应 -->
  <div v-if="wrap" class="code-block--wrap">
    <template v-for="(line, i) in lines" :key="i">
      <div class="wl-num">{{ i + 1 }}</div>
      <div class="wl-text">{{ line }}</div>
    </template>
  </div>
  <!-- 非 wrap 模式：原有滚动行为 -->
  <div v-else class="code-block">
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
  wrap?: boolean
}>()

const gutterRef = ref<HTMLElement | null>(null)
const contentRef = ref<HTMLElement | null>(null)

const lines = computed(() => props.code.split('\n'))
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

/* wrap 模式：grid 两列，行号列固定宽，内容列自动换行 */
.code-block--wrap {
  display: grid;
  grid-template-columns: 44px 1fr;
  font-size: 13px;
  line-height: 1.7em;
  font-family: 'JetBrains Mono', 'SF Mono', Consolas, 'Courier New', monospace;
  padding-top: 14px;
  padding-bottom: 14px;
}

.wl-num {
  text-align: right;
  padding-right: 10px;
  color: var(--label);
  user-select: none;
  background: var(--panel);
  border-right: 1px solid var(--border);
}

.wl-num:last-of-type {
  padding-bottom: 14px;
}

.wl-text {
  padding: 0 18px;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-all;
  min-height: 1.7em;
}

.wl-text:last-of-type {
  padding-bottom: 14px;
}

.line-numbers {
  flex-shrink: 0;
  width: 44px;
  overflow: hidden;
  background: var(--panel);
  border-right: 1px solid var(--border);
  padding: 14px 0;
  user-select: none;
}

.line-number {
  height: 1.7em;
  line-height: 1.7em;
  text-align: right;
  padding-right: 10px;
  font-size: 12px;
  font-family: 'JetBrains Mono', 'SF Mono', Consolas, 'Courier New', monospace;
  color: var(--label);
}

.hl-wrap {
  flex: 1;
  overflow: auto;
  max-height: 400px;
}

.hl-wrap :deep(pre.shiki) {
  margin: 0;
  padding: 14px 18px;
  border-radius: 0;
  font-size: 13px;
  line-height: 1.7em;
  overflow-x: auto;
  overflow-y: auto;
  max-height: 400px;
  white-space: pre;
  font-family: 'JetBrains Mono', 'SF Mono', Consolas, 'Courier New', monospace;
}

.plain-output {
  flex: 1;
  margin: 0;
  padding: 14px 18px;
  font-size: 13px;
  line-height: 1.7em;
  font-family: 'JetBrains Mono', 'SF Mono', Consolas, 'Courier New', monospace;
  color: var(--text);
  background: transparent;
  border: none;
  overflow: auto;
  max-height: 400px;
  white-space: pre;
}
</style>
