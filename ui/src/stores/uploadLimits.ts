import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { fetchUploadLimits } from '@/api/upload'
import { i18n } from '@/i18n'

export const useUploadLimitsStore = defineStore('uploadLimits', () => {
  const allowedExts = ref<string[]>([])
  const maxSize = ref(0)
  const loaded = ref(false)
  const loading = ref(false)
  const error = ref('')

  const accept = computed(() => allowedExts.value.join(','))
  const restrictExts = computed(() => allowedExts.value.length > 0)

  function clear() {
    allowedExts.value = []
    maxSize.value = 0
    loaded.value = false
    loading.value = false
    error.value = ''
  }

  async function load() {
    loading.value = true
    error.value = ''
    try {
      const data = await fetchUploadLimits()
      allowedExts.value = Array.isArray(data.allowed_exts) ? data.allowed_exts : []
      maxSize.value = Number(data.max_size) || 0
      loaded.value = true
    } catch (e) {
      const msg =
        (e as { message?: string })?.message || i18n.global.t('upload.loadLimitsFailed')
      clear()
      error.value = msg
    } finally {
      loading.value = false
    }
  }

  return {
    allowedExts,
    maxSize,
    loaded,
    loading,
    error,
    accept,
    restrictExts,
    clear,
    load,
  }
})
