<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import PagedTable, { type PagedColumn } from '@/components/PagedTable.vue'
import PdfDetailDrawer from '@/components/PdfDetailDrawer.vue'
import StatusTag from '@/components/StatusTag.vue'
import { listAdminUploads, listAdminPdfs } from '@/api/admin'
import { listAdminPdfLogs } from '@/api/history'
import type { UploadItem } from '@/api/upload'
import type { PdfItem } from '@/api/pdf'

const { t, locale } = useI18n()
const activeTab = ref('uploads')
const uidInput = ref('')
const uidFilter = ref('')

const uploadsRef = ref<{ refresh: (resetPage?: boolean) => Promise<void> } | null>(null)
const pdfsRef = ref<{ refresh: (resetPage?: boolean) => Promise<void> } | null>(null)

const drawerVisible = ref(false)
const selected = ref<PdfItem | null>(null)

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

/** Best-effort uid from storage path `{uid}/...` (list views omit uid). */
function uidFromPath(filePath: string | undefined): string {
  if (!filePath) return '-'
  const part = filePath.replace(/\\/g, '/').split('/').filter(Boolean)[0]
  return part || '-'
}

const uploadColumns = computed<PagedColumn[]>(() => {
  void locale.value
  return [
    {
      prop: 'file_path',
      label: t('common.uid'),
      width: 120,
      showOverflowTooltip: true,
      formatter: (row) => uidFromPath((row as UploadItem).file_path),
    },
    { prop: 'filename', label: t('common.filename'), minWidth: 140, showOverflowTooltip: true },
    { prop: 'fileid', label: t('common.fileid'), minWidth: 180, showOverflowTooltip: true },
    { label: t('common.status'), width: 110, slot: 'status' },
    { prop: 'retry_count', label: t('common.retries'), width: 70, align: 'center' },
    {
      prop: 'file_size',
      label: t('common.size'),
      width: 100,
      formatter: (row) => formatSize(Number((row as UploadItem).file_size) || 0),
    },
    {
      prop: 'upload_time',
      label: t('history.uploadTime'),
      width: 170,
      formatter: (row) => formatTime(String((row as UploadItem).upload_time || '')),
    },
  ]
})

const pdfColumns = computed<PagedColumn[]>(() => {
  void locale.value
  return [
    {
      prop: 'file_path',
      label: t('common.uid'),
      width: 120,
      showOverflowTooltip: true,
      formatter: (row) => uidFromPath((row as PdfItem).file_path),
    },
    { prop: 'filename', label: t('common.filename'), minWidth: 160, showOverflowTooltip: true },
    { prop: 'fileid', label: t('common.fileid'), minWidth: 180, showOverflowTooltip: true },
    {
      prop: 'file_size',
      label: t('common.size'),
      width: 100,
      formatter: (row) => formatSize(Number((row as PdfItem).file_size) || 0),
    },
    { label: t('common.status'), width: 110, slot: 'status' },
    { label: t('common.actions'), minWidth: 100, fixed: 'right', slot: 'actions' },
  ]
})

function uploadsLoader(page: number, pageSize: number) {
  return listAdminUploads(page, pageSize, uidFilter.value || undefined)
}

function pdfsLoader(page: number, pageSize: number) {
  return listAdminPdfs(page, pageSize, uidFilter.value || undefined)
}

async function adminLogLoader(fileid: string) {
  const data = await listAdminPdfLogs(1, 50, fileid)
  return data.items || []
}

function applyUidFilter() {
  uidFilter.value = uidInput.value.trim()
  void uploadsRef.value?.refresh(true)
  void pdfsRef.value?.refresh(true)
}

function clearUidFilter() {
  uidInput.value = ''
  uidFilter.value = ''
  void uploadsRef.value?.refresh(true)
  void pdfsRef.value?.refresh(true)
}

function refresh() {
  if (activeTab.value === 'uploads') {
    void uploadsRef.value?.refresh()
  } else {
    void pdfsRef.value?.refresh()
  }
}

function onDetail(row: PdfItem) {
  selected.value = row
  drawerVisible.value = true
}
</script>

<template>
  <div class="overview-page">
    <div class="page-head">
      <h2 class="page-title">{{ t('overview.title') }}</h2>
      <div class="head-actions">
        <el-input
          v-model="uidInput"
          clearable
          :placeholder="t('overview.uidFilter')"
          class="uid-filter"
          @keyup.enter="applyUidFilter"
        />
        <el-button type="primary" @click="applyUidFilter">{{ t('common.filter') }}</el-button>
        <el-button @click="clearUidFilter">{{ t('common.clear') }}</el-button>
        <el-button @click="refresh">{{ t('common.refresh') }}</el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="overview-tabs">
      <el-tab-pane :label="t('overview.uploadsTab')" name="uploads">
        <PagedTable
          ref="uploadsRef"
          :columns="uploadColumns"
          :loader="uploadsLoader"
          :empty-text="t('overview.uploadsEmpty')"
        >
          <template #status="{ row }">
            <StatusTag :status="String((row as UploadItem).status || '')" />
          </template>
        </PagedTable>
      </el-tab-pane>

      <el-tab-pane :label="t('overview.pdfsTab')" name="pdfs">
        <PagedTable
          ref="pdfsRef"
          :columns="pdfColumns"
          :loader="pdfsLoader"
          :empty-text="t('overview.pdfsEmpty')"
        >
          <template #status="{ row }">
            <StatusTag :status="String((row as PdfItem).status || '')" />
          </template>
          <template #actions="{ row }">
            <el-button link type="primary" @click="onDetail(row as PdfItem)">
              {{ t('common.detail') }}
            </el-button>
          </template>
        </PagedTable>
      </el-tab-pane>
    </el-tabs>

    <PdfDetailDrawer
      v-model:visible="drawerVisible"
      :fileid="selected?.fileid || ''"
      :filename="selected?.filename || ''"
      :file-size="selected?.file_size || 0"
      :status="selected?.status || ''"
      :log-loader="adminLogLoader"
    />
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

.head-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.uid-filter {
  width: 180px;
}

.overview-tabs :deep(.el-tabs__content) {
  padding-top: 8px;
}
</style>
