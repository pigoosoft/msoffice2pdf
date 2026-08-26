<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import type { UploadFile, UploadFiles, UploadInstance, UploadUserFile } from 'element-plus'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useUploadLimitsStore } from '@/stores/uploadLimits'

const { t } = useI18n()
const limits = useUploadLimitsStore()
const { allowedExts, maxSize, loaded, loading, error, accept, restrictExts } = storeToRefs(limits)

const files = defineModel<File[]>('files', { default: () => [] })
const props = defineProps<{ uploading?: boolean }>()
const emit = defineEmits<{ change: [files: File[]] }>()

const uploadRef = ref<UploadInstance>()
const fileList = ref<UploadUserFile[]>([])

const disabled = computed(
  () => !loaded.value || loading.value || !!error.value || !!props.uploading,
)

const hint = computed(() => {
  if (loading.value) return t('upload.loadingLimits')
  if (error.value) return error.value
  if (!loaded.value) return t('upload.limitsNotLoaded')
  if (!restrictExts.value) return t('upload.serverValidatesExt')
  const sample = allowedExts.value.join(' / ')
  const sizeHint =
    maxSize.value > 0
      ? t('upload.maxMb', { mb: Math.round(maxSize.value / (1024 * 1024)) })
      : ''
  return t('upload.limitsHint', { sample, sizeHint })
})

function extOf(name: string): string {
  const i = name.lastIndexOf('.')
  return i >= 0 ? name.slice(i).toLowerCase() : ''
}

function syncFromList(list: UploadUserFile[]) {
  const next: File[] = []
  for (const item of list) {
    if (item.raw) next.push(item.raw)
  }
  files.value = next
  emit('change', next)
}

function rejectFile(uploadFile: UploadFile) {
  nextTick(() => {
    fileList.value = fileList.value.filter((f) => f.uid !== uploadFile.uid)
    syncFromList(fileList.value)
  })
}

function onChange(uploadFile: UploadFile, list: UploadFiles) {
  const raw = uploadFile.raw
  if (!raw) {
    syncFromList(list)
    return
  }
  if (!loaded.value || loading.value) {
    ElMessage.warning(t('upload.limitsNotReady'))
    rejectFile(uploadFile)
    return
  }
  const ext = extOf(raw.name)
  if (restrictExts.value && !allowedExts.value.includes(ext)) {
    ElMessage.warning(t('upload.onlyExts', { exts: allowedExts.value.join(' ') }))
    rejectFile(uploadFile)
    return
  }
  if (maxSize.value > 0 && raw.size > maxSize.value) {
    ElMessage.error(
      t('upload.tooLarge', { mb: Math.round(maxSize.value / (1024 * 1024)) }),
    )
    rejectFile(uploadFile)
    return
  }
  syncFromList(list)
}

function onRemove(_uploadFile: UploadFile, list: UploadFiles) {
  syncFromList(list)
}

function clear() {
  fileList.value = []
  uploadRef.value?.clearFiles()
  files.value = []
  emit('change', [])
}

function removeFiles(toRemove: File[]) {
  const set = new Set(toRemove)
  fileList.value = fileList.value.filter((item) => !item.raw || !set.has(item.raw))
  syncFromList(fileList.value)
}

defineExpose({ clear, removeFiles })
</script>

<template>
  <el-upload
    ref="uploadRef"
    v-model:file-list="fileList"
    drag
    multiple
    :auto-upload="false"
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
:deep(.el-upload-dragger) {
  height: auto;
  padding: 24px 16px;
}

.drop-inner {
  padding: 0;
}

.drop-title {
  margin: 0 0 8px;
  font-size: 15px;
  color: var(--el-text-color-primary);
}

.drop-hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.65;
  color: var(--el-text-color-secondary);
  white-space: normal;
  word-break: break-word;
}
</style>
