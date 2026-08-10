<script setup lang="ts">
import { ref, watch, onMounted, nextTick, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import type { Envelope } from '@/api/types'
import ActionsColToggle from '@/components/ActionsColToggle.vue'

export interface PagedColumn {
  prop?: string
  label: string
  width?: string | number
  minWidth?: string | number
  slot?: string
  formatter?: (row: unknown) => string
  showOverflowTooltip?: boolean
  fixed?: boolean | 'left' | 'right'
  align?: 'left' | 'center' | 'right'
}

export interface PagedResult {
  items: unknown[]
  total: number
}

const props = withDefaults(
  defineProps<{
    columns: PagedColumn[]
    loader: (page: number, pageSize: number) => Promise<PagedResult>
    pageSize?: number
    emptyText?: string
  }>(),
  {
    pageSize: 20,
  },
)

const { t } = useI18n()
const resolvedEmpty = computed(() => props.emptyText ?? t('paged.empty'))

const tableRef = ref<{ doLayout?: () => void } | null>(null)
const rows = ref<unknown[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(props.pageSize)
const loading = ref(false)
const actionsOpen = ref(false)

function errMsg(e: unknown): string {
  return (e as Envelope)?.message || (e as Error)?.message || t('common.requestFailed')
}

function cellValue(row: unknown, prop: string): unknown {
  if (row && typeof row === 'object' && prop in row) {
    return (row as Record<string, unknown>)[prop]
  }
  return undefined
}

function isActionsSlot(slot?: string): boolean {
  return slot === 'actions'
}

function toNum(v: string | number | undefined, fallback: number): number {
  const n = Number(v)
  return Number.isFinite(n) && n > 0 ? n : fallback
}

/** Fixed columns need an explicit width for el-table horizontal lock. */
function actionsColWidth(col: PagedColumn): number {
  if (actionsOpen.value) return toNum(col.minWidth ?? col.width, 220)
  return 72
}

function colFixed(col: PagedColumn): boolean | 'left' | 'right' | undefined {
  if (isActionsSlot(col.slot)) return 'right'
  return col.fixed
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

async function load() {
  loading.value = true
  try {
    const data = await props.loader(page.value, pageSize.value)
    rows.value = data.items || []
    total.value = data.total
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    loading.value = false
    relayout()
  }
}

function onPageChange(p: number) {
  page.value = p
  void load()
}

function onSizeChange(size: number) {
  pageSize.value = size
  page.value = 1
  void load()
}

function refresh(resetPage = false) {
  if (resetPage) page.value = 1
  return load()
}

watch(
  () => props.loader,
  () => {
    page.value = 1
    actionsOpen.value = false
    void load()
  },
)

onMounted(() => {
  void load()
})

defineExpose({ refresh, load })
</script>

<template>
  <div class="paged-table">
    <el-table
      ref="tableRef"
      :data="rows"
      v-loading="loading"
      stripe
      :empty-text="resolvedEmpty"
      class="table"
    >
      <el-table-column
        v-for="(col, idx) in columns"
        :key="col.slot || col.prop || String(idx)"
        :prop="col.prop"
        :label="col.label"
        :width="isActionsSlot(col.slot) ? actionsColWidth(col) : col.width"
        :min-width="isActionsSlot(col.slot) ? undefined : col.minWidth"
        :fixed="colFixed(col)"
        :align="col.align"
        :show-overflow-tooltip="col.showOverflowTooltip"
      >
        <template v-if="isActionsSlot(col.slot)" #header>
          <ActionsColToggle :open="actionsOpen" :label="col.label" @toggle="toggleActions" />
        </template>
        <template #default="{ row }">
          <template v-if="isActionsSlot(col.slot)">
            <div v-show="actionsOpen" class="actions-cell">
              <slot :name="col.slot" :row="row" />
            </div>
          </template>
          <slot v-else-if="col.slot" :name="col.slot" :row="row" />
          <template v-else-if="col.formatter">
            {{ col.formatter(row) }}
          </template>
          <template v-else-if="col.prop">
            {{ cellValue(row, col.prop) }}
          </template>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        background
        layout="total, sizes, prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        :page-sizes="[10, 20, 50]"
        @current-change="onPageChange"
        @size-change="onSizeChange"
      />
    </div>
  </div>
</template>

<style scoped>
.paged-table {
  width: 100%;
  overflow: hidden;
}

.table {
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
