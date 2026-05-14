import { ref } from 'vue'

// Global singleton — persists across conversations
const selectedHostIds = ref<string[] | null>(null)

export function useTargetHosts() {
  return { selectedHostIds }
}
