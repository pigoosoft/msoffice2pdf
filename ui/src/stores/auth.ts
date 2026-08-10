import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as authApi from '@/api/auth'
import type { Role } from '@/api/types'
import { useUploadLimitsStore } from '@/stores/uploadLimits'

export const useAuthStore = defineStore('auth', () => {
  const uid = ref('')
  const role = ref<Role | ''>('')
  const ready = ref(false)

  const isAdmin = computed(() => role.value === 'admin')
  const isLoggedIn = computed(() => !!uid.value)

  function clear() {
    uid.value = ''
    role.value = ''
    useUploadLimitsStore().clear()
  }

  async function bootstrap() {
    try {
      const u = await authApi.verify()
      uid.value = u.uid
      role.value = u.role
      await useUploadLimitsStore().load()
    } catch {
      clear()
    } finally {
      ready.value = true
    }
  }

  async function login(u: string, pwd: string) {
    const res = await authApi.login(u, pwd)
    uid.value = res.uid
    role.value = res.role
    // ignore res.token in storage — Cookie only
    await useUploadLimitsStore().load()
  }

  async function logout() {
    try {
      await authApi.logout()
    } finally {
      clear()
    }
  }

  return { uid, role, ready, isAdmin, isLoggedIn, clear, bootstrap, login, logout }
})
