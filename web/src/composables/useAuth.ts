import { ref, computed } from 'vue'
import type { UserInfo } from '../api/auth'
import { getMe, clearStoredToken, getStoredToken } from '../api/auth'

const currentUser = ref<UserInfo | null>(null)
const authChecked = ref(false)

export function useAuth() {
  const isLoggedIn = computed(() => currentUser.value !== null)
  const isAdmin = computed(() => currentUser.value?.role === 'admin')

  async function checkAuth(): Promise<boolean> {
    if (!getStoredToken()) {
      authChecked.value = true
      return false
    }
    try {
      currentUser.value = await getMe()
      authChecked.value = true
      return true
    } catch {
      clearStoredToken()
      currentUser.value = null
      authChecked.value = true
      return false
    }
  }

  function setUser(user: UserInfo) {
    currentUser.value = user
  }

  function clearUser() {
    currentUser.value = null
    clearStoredToken()
  }

  return { currentUser, isLoggedIn, isAdmin, authChecked, checkAuth, setUser, clearUser }
}
