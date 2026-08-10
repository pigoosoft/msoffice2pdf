<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { downloadPdf } from '@/api/pdf'
import { listPdfLogs, type PdfLogItem } from '@/api/history'
import type { Envelope } from '@/api/types'
import StatusTag from '@/components/StatusTag.vue'

const props = withDefaults(
  defineProps<{
    visible: boolean
    fileid: string
    filename?: string
    fileSize?: number
    status?: string
    /** Optional override for admin pdflog endpoint (Task 9). */
    logLoader?: (fileid: string) => Promise<PdfLogItem[]>
  }>(),
  {
    filename: '',
    fileSize: 0,
    status: '',
  },
)

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const { t } = useI18n()
const loading = ref(false)
const downloading = ref(false)
const logs = ref<PdfLogItem[]>([])

const drawerTitle = computed(
  () => props.filename || props.fileid || t('pdfDetail.title'),
)

function actionLabel(action: string): string {
  const key = `pdfAction.${action}`
  const translated = t(key)
  return translated === key ? action : translated
}

function errMsg(e: unknown): string {
  return (e as Envelope)?.message || (e as Error)?.message || t('common.requestFailed')
}

function formatSize(n: number): string {
  if (!n || n < 0) return '-'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(2)} MB`
}

function formatTime(iso: string): string {
  if (!iso) return '-'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const pad = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

async function loadLogs() {
  if (!props.fileid) {
    logs.value = []
    return
  }
  loading.value = true
  try {
    if (props.logLoader) {
      logs.value = await props.logLoader(props.fileid)
    } else {
      const data = await listPdfLogs(1, 50, props.fileid)
      logs.value = data.items || []
    }
  } catch (e) {
    ElMessage.error(errMsg(e))
    logs.value = []
  } finally {
    loading.value = false
  }
}

async function onDownload() {
  if (!props.fileid) return
  downloading.value = true
  try {
    await downloadPdf(props.fileid, props.filename || undefined)
    ElMessage.success(t('pdfs.downloadStarted'))
    await loadLogs()
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    downloading.value = false
  }
}

function onClose() {
  emit('update:visible', false)
}

watch(
  () => [props.visible, props.fileid] as const,
  ([vis]) => {
    if (vis) void loadLogs()
  },
)
</script>

<template>
  <el-drawer
    :model-value="visible"
    :title="drawerTitle"
    size="420px"
    destroy-on-close
    @close="onClose"
    @update:model-value="emit('update:visible', $event)"
  >
    <div class="meta" v-loading="loading">
      <div class="meta-row">
        <span class="label">{{ t('common.filename') }}</span>
        <span class="value">{{ filename || '-' }}</span>
      </div>
      <div class="meta-row">
        <span class="label">{{ t('common.fileid') }}</span>
        <span class="value mono">{{ fileid || '-' }}</span>
      </div>
      <div class="meta-row">
        <span class="label">{{ t('common.size') }}</span>
        <span class="value">{{ formatSize(fileSize) }}</span>
      </div>
      <div class="meta-row" v-if="status">
        <span class="label">{{ t('common.status') }}</span>
        <span class="value"><StatusTag :status="status" /></span>
      </div>
      <div class="actions">
        <el-button type="primary" :loading="downloading" :disabled="!fileid" @click="onDownload">
          {{ t('pdfDetail.downloadPdf') }}
        </el-button>
        <el-button :disabled="!fileid" @click="loadLogs">{{ t('pdfDetail.refreshLogs') }}</el-button>
      </div>
    </div>

    <h3 class="section-title">{{ t('pdfDetail.operationLogs') }}</h3>
    <el-timeline v-if="logs.length" class="log-timeline">
      <el-timeline-item
        v-for="log in logs"
        :key="log.id"
        :timestamp="formatTime(log.created_at)"
        placement="top"
      >
        <div class="log-item">
          <div class="log-action">{{ actionLabel(log.action) }}</div>
          <div v-if="log.detail" class="log-detail">{{ log.detail }}</div>
          <div v-if="log.ip_address" class="log-ip">IP: {{ log.ip_address }}</div>
        </div>
      </el-timeline-item>
    </el-timeline>
    <el-empty v-else-if="!loading" :description="t('pdfDetail.noLogs')" :image-size="64" />
  </el-drawer>
</template>

<style scoped>
.meta {
  margin-bottom: 8px;
}

.meta-row {
  display: flex;
  gap: 12px;
  margin-bottom: 10px;
  font-size: 0.9rem;
  line-height: 1.4;
}

.label {
  flex: 0 0 56px;
  color: var(--el-text-color-secondary);
}

.value {
  flex: 1;
  word-break: break-all;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.85rem;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 16px 0 8px;
}

.section-title {
  margin: 20px 0 12px;
  font-size: 1rem;
  font-weight: 600;
}

.log-timeline {
  padding-left: 4px;
}

.log-item {
  font-size: 0.9rem;
}

.log-action {
  font-weight: 600;
}

.log-detail,
.log-ip {
  margin-top: 4px;
  color: var(--el-text-color-secondary);
  word-break: break-all;
}
</style>
