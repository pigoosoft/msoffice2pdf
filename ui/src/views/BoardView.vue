<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import StatusTag from '@/components/StatusTag.vue'
import ActionsColToggle from '@/components/ActionsColToggle.vue'
import { listUploads, deleteUpload, type UploadItem } from '@/api/upload'
import { getStatus, downloadPdf } from '@/api/pdf'
import type { Envelope } from '@/api/types'

const ACTIVE = new Set(['pending', 'queued', 'converting'])

const { t } = useI18n()
const router = useRouter()
const tableRef = ref<{ doLayout?: () => void } | null>(null)
const rows = ref<UploadItem[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const actionsOpen = ref(false)

let timer: ReturnType<typeof setInterval> | null = null
let tracked = new Set<string>()
let busy = false

function errMsg(e: unknown): string {
  return (e as Envelope)?.message || (e as Error)?.message || t('common.requestFailed')
}

function relayout() {
  void nextTick(() => {
    tableRef.value?.doLayout?.()
  })
}

function toggleActions() {
  actionsOpen.value = !actionsOpen.value
  relayout()
}

function stopPoll() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function startPoll() {
  if (timer) return
  timer = setInterval(() => {
    void refresh(true)
  }, 3000)
}

function syncPoll(items: UploadItem[]) {
  const hasActive = items.some((r) => ACTIVE.has(r.status))
  if (hasActive) startPoll()
  else stopPoll()
}

async function notifyLeft(prev: Set<string>, nextItems: UploadItem[]) {
  const next = new Set(nextItems.map((r) => r.fileid))
  if (prev.size > 0) {
    for (const fileid of prev) {
      if (next.has(fileid)) continue
      try {
        const st = await getStatus(fileid)
        const status = st.final_status || st.status
        if (status === 'completed') {
          ElMessage.success({
            duration: 8000,
            message: t('board.convertDone', { name: st.pdf_filename || fileid }),
            showClose: true,
          })
        } else if (status === 'failed') {
          ElMessage.error(t('board.convertFailed', { msg: st.error_msg || fileid }))
        }
      } catch {
        // ignore status miss after archive race
      }
    }
  }
  tracked = next
}

async function refresh(silent = false) {
  if (busy) return
  busy = true
  if (!silent) loading.value = true
  const prev = tracked
  try {
    const data = await listUploads(page.value, pageSize.value)
    rows.value = data.items || []
    total.value = data.total
    await notifyLeft(prev, rows.value)
    syncPoll(rows.value)
  } catch (e) {
    if (!silent) ElMessage.error(errMsg(e))
  } finally {
    loading.value = false
    busy = false
  }
}

async function onDelete(row: UploadItem) {
  try {
    await ElMessageBox.confirm(
      t('board.deleteConfirm', { name: row.filename }),
      t('board.deleteTitle'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel'),
      },
    )
  } catch {
    return
  }
  try {
    await deleteUpload(row.fileid)
    ElMessage.success(t('board.deleted'))
    tracked.delete(row.fileid)
    await refresh()
  } catch (e) {
    ElMessage.error(errMsg(e))
  }
}

async function onDownload(row: UploadItem) {
  try {
    await downloadPdf(row.fileid, `${row.filename.replace(/\.[^.]+$/, '')}.pdf`)
  } catch (e) {
    ElMessage.error(errMsg(e))
  }
}

function onPageChange(p: number) {
  page.value = p
  void refresh()
}

onMounted(() => {
  void refresh()
})

onUnmounted(() => {
  stopPoll()
})
</script>

<template>
  <div class="board-page">
    <div class="board-head">
      <h2 class="page-title">{{ t('board.title') }}</h2>
      <div class="head-actions">
        <el-button @click="refresh()">{{ t('common.refresh') }}</el-button>
        <el-button type="primary" @click="router.push('/upload')">{{ t('nav.upload') }}</el-button>
      </div>
    </div>

    <el-table
      ref="tableRef"
      :data="rows"
      v-loading="loading"
      stripe
      :empty-text="t('board.empty')"
      class="board-table"
    >
      <el-table-column
        prop="filename"
        :label="t('common.filename')"
        min-width="160"
        show-overflow-tooltip
      />
      <el-table-column
        prop="fileid"
        :label="t('common.fileid')"
        min-width="180"
        show-overflow-tooltip
      />
      <el-table-column :label="t('common.status')" width="120">
        <template #default="{ row }">
          <StatusTag :status="row.status" />
        </template>
      </el-table-column>
      <el-table-column :label="t('common.retries')" width="80" align="center">
        <template #default="{ row }">
          {{ row.retry_count ?? 0 }}
        </template>
      </el-table-column>
      <el-table-column :width="actionsOpen ? 160 : 72" fixed="right">
        <template #header>
          <ActionsColToggle :open="actionsOpen" :label="t('common.actions')" @toggle="toggleActions" />
        </template>
        <template #default="{ row }">
          <div v-show="actionsOpen" class="actions-cell">
            <el-button
              v-if="row.status === 'completed'"
              link
              type="primary"
              @click="onDownload(row)"
            >
              {{ t('common.download') }}
            </el-button>
            <el-button link type="danger" @click="onDelete(row)">{{ t('common.delete') }}</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        background
        layout="total, prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="onPageChange"
      />
    </div>
  </div>
</template>

<style scoped>
.board-head {
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
  gap: 8px;
}

.board-table {
  width: 100%;
}

.actions-cell {
  display: inline-flex;
  flex-wrap: nowrap;
  align-items: center;
  white-space: nowrap;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 767px) {
  .pager {
    justify-content: center;
  }
}
</style>
