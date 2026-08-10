<script setup lang="ts">
import { computed, ref } from 'vue'
import type { UploadFile, UploadFiles, UploadInstance } from 'element-plus'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useUploadLimitsStore } from '@/stores/uploadLimits'

const { t } = useI18n()
const limits = useUploadLimitsStore()
const { allowedExts, maxSize, loaded, loading, error, accept, restrictExts } = storeToRefs(limits)

const file = defineModel<File | null>('file', { default: null })
const emit = defineEmits<{ change: [file: File | null] }>()

const uploadRef = ref<UploadInstance>()

const disabled = computed(() => !loaded.value || loading.value || !!error.value)

const hint = computed(() => {
  if (loading.value) return t('upload.loadingLimits')
  if (error.value) return error.value
  if (!loaded.value) return t('upload.limitsNotLoaded')
  if (!restrictExts.value) return t('upload.serverValidatesExt')
  const sample = allowedExts.value
    .map((e) => e.replace(/^\./, ''))
    .slice(0, 8)
    .join(' / ')
  const more = allowedExts.value.length > 8 ? t('upload.moreExts') : ''
  const sizeHint =
    maxSize.value > 0
      ? t('upload.maxMb', { mb: Math.round(maxSize.value / (1024 * 1024)) })
      : ''
  return t('upload.limitsHint', { sample, more, sizeHint })
})

function extOf(name: string): string {
  const i = name.lastIndexOf('.')
  return i >= 0 ? name.slice(i).toLowerCase() : ''
}

function setFile(f: File | null) {
  file.value = f
  emit('change', f)
}

function onChange(uploadFile: UploadFile, fileList: UploadFiles) {
  if (fileList.length > 1) {
    fileList.splice(0, fileList.length - 1)
  }
  const raw = uploadFile.raw
  if (!raw) {
    setFile(null)
    return
  }
  if (!loaded.value || loading.value) {
    ElMessage.warning(t('upload.limitsNotReady'))
    uploadRef.value?.clearFiles()
    setFile(null)
    return
  }
  const ext = extOf(raw.name)
  if (restrictExts.value && !allowedExts.value.includes(ext)) {
    ElMessage.warning(t('upload.onlyExts', { exts: allowedExts.value.join(' ') }))
    uploadRef.value?.clearFiles()
    setFile(null)
    return
  }
  if (maxSize.value > 0 && raw.size > maxSize.value) {
    ElMessage.error(
      t('upload.tooLarge', { mb: Math.round(maxSize.value / (1024 * 1024)) }),
    )
    uploadRef.value?.clearFiles()
    setFile(null)
    return
  }
  setFile(raw)
}

function onRemove() {
  setFile(null)
}

function clear() {
  setFile(null)
  uploadRef.value?.clearFiles()
}

defineExpose({ clear })
</script>

<template>
  <el-upload
    ref="uploadRef"
    drag
    :auto-upload="false"
    :limit="1"
    :accept="accept"
    :disabled="disabled"
    :on-remove="onRemove"
    :on-change="onChange"
  >
    <div class="drop-inner">
      <p class="drop-title">{{ t('upload.dropTitle') }}</p>
      <p class="drop-hint">{{ hint }}</p>
    </div>
  </el-upload>
</template>

<style scoped>
.drop-inner {
  padding: 12px 8px;
}

.drop-title {
  margin: 0 0 8px;
  font-size: 15px;
  color: var(--el-text-color-primary);
}

.drop-hint {
  margin: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
