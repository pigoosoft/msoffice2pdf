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
const selected = ref<File | null>(null)
const watermark = ref('')
const uploading = ref(false)
const progress = ref(0)

function onFileChange(f: File | null) {
  selected.value = f
  progress.value = 0
}

async function onSubmit() {
  if (!selected.value) {
    ElMessage.warning(t('upload.needFile'))
    return
  }
  uploading.value = true
  progress.value = 0
  try {
    const res = await uploadFile(selected.value, watermark.value, (p) => {
      progress.value = p
    })
    ElMessage.success(t('upload.success', { name: res.filename }))
    dropzone.value?.clear()
    selected.value = null
    watermark.value = ''
    progress.value = 0
    await router.push('/board')
  } catch (e) {
    const msg = (e as Envelope)?.message || (e as Error)?.message || t('upload.failed')
    ElMessage.error(msg)
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <div class="upload-page">
    <h2 class="page-title">{{ t('upload.title') }}</h2>
    <el-card shadow="never" class="upload-card">
      <UploadDropzone ref="dropzone" v-model:file="selected" @change="onFileChange" />

      <el-form label-position="top" class="upload-form" @submit.prevent="onSubmit">
        <el-form-item :label="t('upload.watermark')">
          <el-input
            v-model="watermark"
            maxlength="255"
            show-word-limit
            clearable
            :placeholder="t('upload.watermarkPlaceholder')"
          />
        </el-form-item>

        <div v-if="uploading || progress > 0" class="progress-wrap">
          <el-progress :percentage="progress" :stroke-width="10" />
        </div>

        <el-button type="primary" :loading="uploading" :disabled="!selected" @click="onSubmit">
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
