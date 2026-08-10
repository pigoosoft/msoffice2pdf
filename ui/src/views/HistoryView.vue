<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import PagedTable, { type PagedColumn } from '@/components/PagedTable.vue'
import StatusTag from '@/components/StatusTag.vue'
import {
  listUploadHistory,
  listPdfLogs,
  type UploadHistoryItem,
  type PdfLogItem,
} from '@/api/history'

const { t, locale } = useI18n()
const activeTab = ref('uploads')
const uploadsRef = ref<{ refresh: (resetPage?: boolean) => Promise<void> } | null>(null)
const pdflogsRef = ref<{ refresh: (resetPage?: boolean) => Promise<void> } | null>(null)

function actionLabel(action: string): string {
  const key = `pdfAction.${action}` as const
  const translated = t(key)
  return translated === key ? action || '-' : translated
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

const uploadColumns = computed<PagedColumn[]>(() => {
  void locale.value
  return [
    { prop: 'fileid', label: t('common.fileid'), minWidth: 180, showOverflowTooltip: true },
    { prop: 'filename', label: t('common.filename'), minWidth: 140, showOverflowTooltip: true },
    { label: t('history.finalStatus'), width: 100, slot: 'final_status' },
    { prop: 'error_code', label: t('history.errorCode'), width: 160, showOverflowTooltip: true },
    { prop: 'error_msg', label: t('history.errorMsg'), minWidth: 140, showOverflowTooltip: true },
    { prop: 'retry_count', label: t('common.retries'), width: 70, align: 'center' },
    {
      prop: 'file_size',
      label: t('common.size'),
      width: 100,
      formatter: (row) => formatSize(Number((row as UploadHistoryItem).file_size) || 0),
    },
    {
      prop: 'upload_time',
      label: t('history.uploadTime'),
      width: 170,
      formatter: (row) => formatTime(String((row as UploadHistoryItem).upload_time || '')),
    },
    {
      prop: 'finished_at',
      label: t('history.finishedAt'),
      width: 170,
      formatter: (row) => formatTime(String((row as UploadHistoryItem).finished_at || '')),
    },
  ]
})

const pdflogColumns = computed<PagedColumn[]>(() => {
  void locale.value
  return [
    { label: t('history.action'), width: 90, slot: 'action' },
    { prop: 'fileid', label: t('common.fileid'), minWidth: 180, showOverflowTooltip: true },
    { prop: 'detail', label: t('history.detail'), minWidth: 160, showOverflowTooltip: true },
    { prop: 'ip_address', label: 'IP', width: 130, showOverflowTooltip: true },
    {
      prop: 'created_at',
      label: t('common.time'),
      width: 170,
      formatter: (row) => formatTime(String((row as PdfLogItem).created_at || '')),
    },
  ]
})

function uploadsLoader(page: number, pageSize: number) {
  return listUploadHistory(page, pageSize)
}

function pdflogsLoader(page: number, pageSize: number) {
  return listPdfLogs(page, pageSize)
}

function refresh() {
  if (activeTab.value === 'uploads') {
    void uploadsRef.value?.refresh()
  } else {
    void pdflogsRef.value?.refresh()
  }
}
</script>

<template>
  <div class="history-page">
    <div class="page-head">
      <h2 class="page-title">{{ t('history.title') }}</h2>
      <el-button @click="refresh">{{ t('common.refresh') }}</el-button>
    </div>

    <el-tabs v-model="activeTab" class="history-tabs">
      <el-tab-pane :label="t('history.uploadsTab')" name="uploads">
        <PagedTable
          ref="uploadsRef"
          :columns="uploadColumns"
          :loader="uploadsLoader"
          :empty-text="t('history.uploadsEmpty')"
        >
          <template #final_status="{ row }">
            <StatusTag :status="String((row as UploadHistoryItem).final_status || '')" />
          </template>
        </PagedTable>
      </el-tab-pane>

      <el-tab-pane :label="t('history.pdflogsTab')" name="pdflogs">
        <PagedTable
          ref="pdflogsRef"
          :columns="pdflogColumns"
          :loader="pdflogsLoader"
          :empty-text="t('history.pdflogsEmpty')"
        >
          <template #action="{ row }">
            {{ actionLabel((row as PdfLogItem).action) }}
          </template>
        </PagedTable>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.page-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.page-title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
}

.history-tabs :deep(.el-tabs__content) {
  padding-top: 8px;
}
</style>
