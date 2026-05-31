<template>
  <div class="detail-topbar">
    <span class="detail-title">对话框主题</span>
    <button v-if="!chatThemeEditing" class="btn btn-sm" @click="chatThemeEditing = true">编辑</button>
    <button v-else class="btn btn-sm" @click="chatThemeEditing = false">完成</button>
  </div>
  <div class="detail-body">
    <!-- 展示模式 -->
    <template v-if="!chatThemeEditing">
      <div class="edit-card">
        <div class="field-group">
          <div class="field-label">配色方案</div>
          <div class="theme-cards">
            <div class="theme-card selected">
              <div class="theme-preview" :style="{ background: chatThemes[chatThemeName].codeBg }">
                <span class="theme-preview-dot" :style="{ color: chatThemes[chatThemeName].primary }">*</span>
                <span class="theme-preview-fn" :style="{ color: chatThemes[chatThemeName].primary }">fn</span>
                <span class="theme-preview-text" :style="{ color: chatThemes[chatThemeName].textSub }">text</span>
              </div>
              <div class="theme-name">{{ chatThemes[chatThemeName].displayName }}</div>
            </div>
          </div>
        </div>
        <div class="field-group">
          <div class="field-label">布局密度</div>
          <div class="density-btns">
            <button
              v-for="d in (['compact', 'comfortable', 'spacious'] as ChatDensityName[])"
              :key="d"
              class="density-btn"
              :class="{ selected: chatDensityName === d }"
              disabled
            >{{ densityLabels[d] }}</button>
          </div>
        </div>
      </div>
    </template>
    <!-- 编辑模式 -->
    <template v-else>
      <div class="edit-card">
        <div class="field-group">
          <div class="field-label">配色方案</div>
          <div class="theme-cards">
            <div
              v-for="t in chatThemeList"
              :key="t.name"
              class="theme-card"
              :class="{ selected: chatThemeName === t.name }"
              @click="selectChatTheme(t.name)"
            >
              <div class="theme-preview" :style="{ background: t.codeBg }">
                <span class="theme-preview-dot" :style="{ color: t.primary }">*</span>
                <span class="theme-preview-fn" :style="{ color: t.primary }">fn</span>
                <span class="theme-preview-text" :style="{ color: t.textSub }">text</span>
              </div>
              <div class="theme-name">{{ t.displayName }}</div>
            </div>
          </div>
        </div>
        <div class="field-group">
          <div class="field-label">布局密度</div>
          <div class="density-btns">
            <button
              v-for="d in (['compact', 'comfortable', 'spacious'] as ChatDensityName[])"
              :key="d"
              class="density-btn"
              :class="{ selected: chatDensityName === d }"
              @click="selectChatDensity(d)"
            >{{ densityLabels[d] }}</button>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import {
  chatThemes,
  getSavedChatTheme, saveChatTheme,
  getSavedChatDensity, saveChatDensity,
  type ChatThemeName, type ChatDensityName,
} from '../../chatTheme'

const chatThemeName = ref<ChatThemeName>(getSavedChatTheme())
const chatDensityName = ref<ChatDensityName>(getSavedChatDensity())
const chatThemeEditing = ref(false)
const chatThemeList = Object.values(chatThemes)
const densityLabels: Record<ChatDensityName, string> = { compact: '紧凑', comfortable: '舒适', spacious: '宽松' }

function selectChatTheme(name: ChatThemeName) {
  chatThemeName.value = name
  saveChatTheme(name)
}

function selectChatDensity(name: ChatDensityName) {
  chatDensityName.value = name
  saveChatDensity(name)
}
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

.theme-cards { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 8px; }
.theme-card { cursor: pointer; border: 2px solid var(--border); border-radius: 8px; overflow: hidden; width: 100px; }
.theme-card.selected { border-color: var(--primary); }
.theme-preview { display: flex; align-items: center; gap: 6px; padding: 8px 10px; font-family: 'SF Mono', monospace; font-size: 11px; }
.theme-name { font-size: 11px; color: var(--text-sub); padding: 5px 8px; text-align: center; background: var(--card-bg); }
.density-btns { display: flex; gap: 8px; margin-top: 8px; }
.density-btn { padding: 5px 16px; border: 1px solid var(--border); border-radius: 4px; background: transparent; color: var(--text-sub); cursor: pointer; font-size: 12px; }
.density-btn.selected { border-color: var(--primary); color: var(--primary); background: var(--row-hover); }
.field-group { margin-bottom: 16px; }
.field-group:last-child { margin-bottom: 0; }
.field-label { font-size: 11px; font-weight: 600; color: var(--muted); text-transform: uppercase; letter-spacing: 0.07em; margin-bottom: 4px; }
</style>
