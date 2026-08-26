<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import UploadDropzone from '@/components/UploadDropzone.vue'
import { uploadFile } from '@/api/upload'
import type { Envelope } from '@/api/types'

const { t } = useI18n()
const router = useRouter()
const dropzone = ref<InstanceType<typeof UploadDropzone> | null>(null)
const selected = ref<File[]>([])
const watermark = ref('')
const docPassword = ref('')
const uploading = ref(false)
const progress = ref(0)

function onFilesChange(list: File[]) {
  selected.value = list
  if (!uploading.value) {
    progress.value = 0
  }
}

async function onSubmit() {
  if (!selected.value.length) {
    ElMessage.warning(t('upload.needFile'))
    return
  }
  const queue = [...selected.value]
  const total = queue.length
  uploading.value = true
  progress.value = 0
  let ok = 0
  let fail = 0
  try {
    for (let i = 0; i < queue.length; i++) {
      const file = queue[i]
      try {
        const res = await uploadFile(file, watermark.value, docPassword.value, (p) => {
          progress.value = Math.round(((i + p / 100) / total) * 100)
        })
        ok++
        dropzone.value?.removeFiles([file])
        if (total === 1) {
          ElMessage.success(t('upload.success', { name: res.filename }))
        }
      } catch (e) {
        fail++
        const msg = (e as Envelope)?.message || (e as Error)?.message || t('upload.failed')
        ElMessage.error(t('upload.fileFailed', { name: file.name, msg }))
      }
    }
    if (total > 1 && fail === 0) {
      ElMessage.success(t('upload.successAll', { n: ok }))
    } else if (fail > 0 && ok > 0) {
      ElMessage.warning(t('upload.successPartial', { ok, fail }))
    }
    if (selected.value.length === 0) {
      dropzone.value?.clear()
      watermark.value = ''
      docPassword.value = ''
      progress.value = 0
      await router.push('/board')
    }
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <div class="upload-page">
    <h2 class="page-title">{{ t('upload.title') }}</h2>
    <el-card shadow="never" class="upload-card">
      <UploadDropzone
        ref="dropzone"
        v-model:files="selected"
        :uploading="uploading"
        @change="onFilesChange"
      />

      <el-form label-position="top" class="upload-form" @submit.prevent="onSubmit">
        <el-form-item :label="t('upload.watermark')">
          <el-input
            v-model="watermark"
            maxlength="255"
            show-word-limit
            clearable
            :disabled="uploading"
            :placeholder="t('upload.watermarkPlaceholder')"
          />
        </el-form-item>

        <el-form-item :label="t('upload.docPassword')">
          <el-input
            v-model="docPassword"
            type="password"
            autocomplete="new-password"
            show-password
            clearable
            :disabled="uploading"
            :placeholder="t('upload.docPasswordPlaceholder')"
          />
        </el-form-item>

        <div v-if="uploading || progress > 0" class="progress-wrap">
          <el-progress :percentage="progress" :stroke-width="10" />
        </div>

        <el-button type="primary" :loading="uploading" :disabled="!selected.length" @click="onSubmit">
          {{ t('upload.submit') }}
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.upload-page {
  max-width: 720px;
}

.page-title {
  margin: 0 0 16px;
  font-size: 1.25rem;
  font-weight: 600;
}

.upload-card {
  background: var(--el-bg-color);
}

.upload-form {
  margin-top: 16px;
}

.progress-wrap {
  margin-bottom: 16px;
}
</style>
