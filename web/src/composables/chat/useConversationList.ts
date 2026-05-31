import { ref, readonly, type Ref } from 'vue'
import {
  listConversations,
  createConversation as apiCreateConversation,
  deleteConversation as apiDeleteConversation,
  updateTitle as apiUpdateTitle,
  type Conversation,
} from '../../api/chat'

export interface UseConversationListOptions {
  onConversationSelected?: (convId: string) => void
}

export function useConversationList(options?: UseConversationListOptions) {
  const conversations = ref<Conversation[]>([])
  const activeConvId = ref<string | null>(null)
  const batchMode = ref(false)
  const selectedConvIds = ref<Set<string>>(new Set())
  const editingConvId = ref<string | null>(null)
  const editTitleText = ref('')
  const menuOpenConvId = ref<string | null>(null)

  async function loadConversations() {
    conversations.value = await listConversations()
  }

  async function selectConversation(id: string) {
    activeConvId.value = id
    localStorage.setItem('spider-last-conv', id)
    options?.onConversationSelected?.(id)
  }

  async function createConversation(title?: string): Promise<string> {
    const conv = await apiCreateConversation(title)
    conversations.value.unshift(conv)
    return conv.id
  }

  async function deleteConversation(id: string) {
    await apiDeleteConversation(id)
    conversations.value = conversations.value.filter(c => c.id !== id)
  }

  async function updateTitle(id: string, title: string) {
    await apiUpdateTitle(id, title)
    const conv = conversations.value.find(c => c.id === id)
    if (conv) conv.title = title
  }

  function enterBatchMode() {
    menuOpenConvId.value = null
    batchMode.value = true
    selectedConvIds.value = new Set()
  }

  function exitBatchMode() {
    batchMode.value = false
    selectedConvIds.value = new Set()
  }

  function toggleSelectConv(id: string) {
    const s = new Set(selectedConvIds.value)
    if (s.has(id)) s.delete(id)
    else s.add(id)
    selectedConvIds.value = s
  }

  function toggleSelectAll() {
    if (selectedConvIds.value.size === conversations.value.length) {
      selectedConvIds.value = new Set()
    } else {
      selectedConvIds.value = new Set(conversations.value.map(c => c.id))
    }
  }

  async function batchDelete() {
    const ids = Array.from(selectedConvIds.value)
    for (const id of ids) {
      await deleteConversation(id)
    }
    exitBatchMode()
  }

  function startEditConvTitle(id: string, title: string) {
    editingConvId.value = id
    editTitleText.value = title
  }

  async function saveConvTitle(id: string) {
    editingConvId.value = null
    const text = editTitleText.value.trim()
    const conv = conversations.value.find(c => c.id === id)
    if (!conv || !text || text === conv.title) return
    await updateTitle(id, text)
  }

  function cancelEdit() {
    editingConvId.value = null
  }

  function openConvMenu(id: string) {
    menuOpenConvId.value = menuOpenConvId.value === id ? null : id
  }

  function closeConvMenu() {
    menuOpenConvId.value = null
  }

  function getConvTitle(convId: string): string {
    return conversations.value.find(c => c.id === convId)?.title || convId.slice(0, 8)
  }

  return {
    // State (readonly)
    conversations: readonly(conversations) as Readonly<Ref<Conversation[]>>,
    activeConvId: readonly(activeConvId) as Readonly<Ref<string | null>>,
    batchMode: readonly(batchMode) as Readonly<Ref<boolean>>,
    selectedConvIds: readonly(selectedConvIds) as Readonly<Ref<Set<string>>>,
    editingConvId: readonly(editingConvId) as Readonly<Ref<string | null>>,
    editTitleText,
    menuOpenConvId: readonly(menuOpenConvId) as Readonly<Ref<string | null>>,

    // Operations
    loadConversations,
    selectConversation,
    createConversation,
    deleteConversation,
    updateTitle,

    // Batch
    enterBatchMode,
    exitBatchMode,
    toggleSelectConv,
    toggleSelectAll,
    batchDelete,

    // Title editing
    startEditConvTitle,
    saveConvTitle,
    cancelEdit,

    // Menu
    openConvMenu,
    closeConvMenu,

    // Utils
    getConvTitle,
  }
}
