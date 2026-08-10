import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAppStore = defineStore('app', () => {
  const sidebarCollapsed = ref(false)
  const drawerOpen = ref(false)

  return { sidebarCollapsed, drawerOpen }
})
