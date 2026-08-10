<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import PagedTable, { type PagedColumn } from '@/components/PagedTable.vue'
import PdfDetailDrawer from '@/components/PdfDetailDrawer.vue'
import StatusTag from '@/components/StatusTag.vue'
import { listPdfs, downloadPdf, type PdfItem } from '@/api/pdf'
import type { Envelope } from '@/api/types'

const { t, locale } = useI18n()
const tableRef = ref<{ refresh: (resetPage?: boolean) => Promise<void> } | null>(null)

const drawerVisible = ref(false)
const selected = ref<PdfItem | null>(null)

function errMsg(e: unknown): string {
  return (e as Envelope)?.message || (e as Error)?.message || t('common.requestFailed')
}

function formatSize(n: number): string {
  if (!n || n < 0) return '-'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(2)} MB`
}

const columns = computed<PagedColumn[]>(() => {
  void locale.value
  return [
    { prop: 'filename', label: t('common.filename'), minWidth: 160, showOverflowTooltip: true },
    { prop: 'fileid', label: t('common.fileid'), minWidth: 180, showOverflowTooltip: true },
    {
      prop: 'file_size',
      label: t('common.size'),
      width: 100,
      formatter: (row) => formatSize(Number((row as PdfItem).file_size) || 0),
    },
    { prop: 'status', label: t('common.status'), width: 110, slot: 'status' },
    { label: t('common.actions'), minWidth: 140, fixed: 'right', slot: 'actions' },
  ]
})

function loader(page: number, pageSize: number) {
  return listPdfs(page, pageSize)
}

async function onDownload(row: PdfItem) {
  try {
    await downloadPdf(row.fileid, row.filename || undefined)
    ElMessage.success(t('pdfs.downloadStarted'))
  } catch (e) {
    ElMessage.error(errMsg(e))
  }
}

function onDetail(row: PdfItem) {
  selected.value = row
  drawerVisible.value = true
}

function refresh() {
  void tableRef.value?.refresh()
}
</script>

<template>
  <div class="pdfs-page">
    <div class="page-head">
      <h2 class="page-title">{{ t('pdfs.title') }}</h2>
      <el-button @click="refresh">{{ t('common.refresh') }}</el-button>
    </div>

    <PagedTable
      ref="tableRef"
      :columns="columns"
      :loader="loader"
      :empty-text="t('pdfs.empty')"
    >
      <template #status="{ row }">
        <StatusTag :status="String(row.status || '')" />
      </template>
      <template #actions="{ row }">
        <el-button link type="primary" @click="onDownload(row as PdfItem)">
          {{ t('common.download') }}
        </el-button>
        <el-button link type="primary" @click="onDetail(row as PdfItem)">
          {{ t('common.detail') }}
        </el-button>
      </template>
    </PagedTable>

    <PdfDetailDrawer
      v-model:visible="drawerVisible"
      :fileid="selected?.fileid || ''"
      :filename="selected?.filename || ''"
      :file-size="selected?.file_size || 0"
      :status="selected?.status || ''"
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
</style>
