<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import {
  getAdminMetrics,
  getAdminMetricsHistory,
  type MetricsCurrent,
  type MetricsRange,
  type MetricsSample,
} from '@/api/admin'
import type { Envelope } from '@/api/types'

use([CanvasRenderer, LineChart, GridComponent, LegendComponent, TooltipComponent])

const { t } = useI18n()

const range = ref<MetricsRange>('1h')
const current = ref<MetricsCurrent | null>(null)
const points = ref<MetricsSample[]>([])
const loading = ref(false)
const error = ref('')

let timer: ReturnType<typeof setInterval> | null = null
let histTimer: ReturnType<typeof setInterval> | null = null

function errMsg(e: unknown): string {
  return (e as Envelope)?.message || (e as Error)?.message || t('common.requestFailed')
}

function fmtMiB(n: number): string {
  return `${(Number(n) / 1024 / 1024).toFixed(1)} MiB`
}

function fmtGiB(n: number): string {
  return `${(Number(n) / 1024 / 1024 / 1024).toFixed(2)} GiB`
}

function degradeLabel(reason: string): string {
  if (!reason) return ''
  const key = `perf.reason_${reason}`
  const label = t(key)
  if (label === key) return reason
  return t('perf.degrade', { reason: label })
}

async function loadCurrent() {
  try {
    current.value = await getAdminMetrics()
    error.value = ''
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function loadHistory() {
  loading.value = true
  try {
    const data = await getAdminMetricsHistory(range.value)
    points.value = data.points || []
    error.value = ''
  } catch (e) {
    error.value = errMsg(e)
    points.value = []
  } finally {
    loading.value = false
  }
}

const chartPoints = computed(() => {
  const pts = [...points.value]
  if (current.value) {
    pts.push(current.value)
  }
  return pts
})

const times = computed(() =>
  chartPoints.value.map((p) => {
    const d = new Date(p.sampled_at)
    if (Number.isNaN(d.getTime())) return p.sampled_at
    const pad = (x: number) => String(x).padStart(2, '0')
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  }),
)

function lineOption(
  series: { name: string; data: number[]; yAxisIndex?: number }[],
  yAxes: Record<string, unknown>[],
) {
  return {
    title: { show: false },
    tooltip: { trigger: 'axis' },
    legend: { top: 0 },
    grid: { left: 48, right: series.some((s) => s.yAxisIndex === 1) ? 56 : 24, top: 36, bottom: 32 },
    xAxis: { type: 'category', data: times.value, boundaryGap: false },
    yAxis: yAxes,
    series: series.map((s) => ({
      type: 'line',
      showSymbol: false,
      name: s.name,
      data: s.data,
      yAxisIndex: s.yAxisIndex || 0,
    })),
  }
}

const queueOption = computed(() =>
  lineOption(
    [
      { name: t('perf.pending'), data: chartPoints.value.map((p) => p.pending) },
      { name: t('perf.queued'), data: chartPoints.value.map((p) => p.queued) },
      { name: t('perf.converting'), data: chartPoints.value.map((p) => p.converting) },
    ],
    [{ type: 'value', minInterval: 1 }],
  ),
)

const workersOption = computed(() =>
  lineOption(
    [
      { name: t('perf.workersCur'), data: chartPoints.value.map((p) => p.workers_cur) },
      { name: t('perf.workersMax'), data: chartPoints.value.map((p) => p.workers_max) },
    ],
    [{ type: 'value', minInterval: 1 }],
  ),
)

const resourceOption = computed(() =>
  lineOption(
    [
      { name: t('perf.logMiB'), data: chartPoints.value.map((p) => p.log_backlog_bytes / 1024 / 1024) },
      { name: t('perf.heapMiB'), data: chartPoints.value.map((p) => p.heap_alloc / 1024 / 1024) },
      { name: t('perf.ramMiB'), data: chartPoints.value.map((p) => p.ram_avail / 1024 / 1024) },
      {
        name: t('perf.diskGiB'),
        data: chartPoints.value.map((p) => p.disk_free_min / 1024 / 1024 / 1024),
        yAxisIndex: 1,
      },
    ],
    [
      { type: 'value', name: 'MiB' },
      { type: 'value', name: 'GiB' },
    ],
  ),
)

const waiting = computed(() => (current.value?.pending || 0) + (current.value?.queued || 0))
const degraded = computed(() => Boolean(current.value?.degrade_reason))

watch(range, () => {
  void loadHistory()
})

onMounted(() => {
  void loadCurrent()
  void loadHistory()
  timer = setInterval(() => {
    void loadCurrent()
  }, 2_000)
  histTimer = setInterval(() => {
    void loadHistory()
  }, 10_000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  if (histTimer) clearInterval(histTimer)
})
</script>

<template>
  <div class="perf-page">
    <div class="page-head">
      <h2 class="page-title">{{ t('perf.title') }}</h2>
      <div class="head-actions">
        <el-radio-group v-model="range" size="small">
          <el-radio-button value="1h">{{ t('perf.range1h') }}</el-radio-button>
          <el-radio-button value="24h">{{ t('perf.range24h') }}</el-radio-button>
          <el-radio-button value="7d">{{ t('perf.range7d') }}</el-radio-button>
        </el-radio-group>
        <el-button @click="loadCurrent(); loadHistory()">{{ t('common.refresh') }}</el-button>
      </div>
    </div>

    <el-alert
      v-if="error"
      :title="error"
      type="error"
      show-icon
      :closable="false"
      class="banner"
    />
    <el-alert
      v-else-if="degraded"
      :title="degradeLabel(current?.degrade_reason || '')"
      type="warning"
      show-icon
      :closable="false"
      class="banner"
    />

    <el-row :gutter="12" class="cards">
      <el-col :xs="12" :sm="8" :md="6" :lg="3">
        <div class="mini">
          <div class="mini-label">{{ t('perf.waiting') }}</div>
          <div class="mini-value">{{ waiting }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="6" :lg="3">
        <div class="mini">
          <div class="mini-label">{{ t('perf.converting') }}</div>
          <div class="mini-value">{{ current?.converting || 0 }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="6" :lg="3">
        <div class="mini">
          <div class="mini-label">{{ t('perf.failed') }}</div>
          <div class="mini-value">{{ current?.failed || 0 }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="6" :lg="3">
        <div class="mini">
          <div class="mini-label">{{ t('perf.workers') }}</div>
          <div class="mini-value" :class="{ warn: degraded }">{{ current?.workers_cur ?? 0 }} / {{ current?.workers_max ?? 0 }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="6" :lg="3">
        <div class="mini">
          <div class="mini-label">{{ t('perf.logBacklog') }}</div>
          <div class="mini-value">{{ fmtMiB(current?.log_backlog_bytes || 0) }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="6" :lg="3">
        <div class="mini">
          <div class="mini-label">{{ t('perf.heap') }}</div>
          <div class="mini-value">{{ fmtMiB(current?.heap_alloc || 0) }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="6" :lg="3">
        <div class="mini">
          <div class="mini-label">{{ t('perf.disk') }}</div>
          <div class="mini-value">{{ fmtGiB(current?.disk_free_min || 0) }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="6" :lg="3">
        <div class="mini">
          <div class="mini-label">{{ t('perf.ram') }}</div>
          <div class="mini-value">{{ fmtGiB(current?.ram_avail || 0) }}</div>
        </div>
      </el-col>
    </el-row>

    <el-card class="chart-card" shadow="never" v-loading="loading">
      <template #header>{{ t('perf.chartQueue') }}</template>
      <v-chart class="chart" :option="queueOption" :update-options="{ notMerge: true }" autoresize />
    </el-card>
    <el-card class="chart-card" shadow="never" v-loading="loading">
      <template #header>{{ t('perf.chartWorkers') }}</template>
      <v-chart class="chart" :option="workersOption" :update-options="{ notMerge: true }" autoresize />
    </el-card>
    <el-card class="chart-card" shadow="never" v-loading="loading">
      <template #header>{{ t('perf.chartResources') }}</template>
      <v-chart class="chart" :option="resourceOption" :update-options="{ notMerge: true }" autoresize />
    </el-card>
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
.banner {
  margin-bottom: 12px;
}
.cards {
  margin-bottom: 16px;
}
.cards :deep(.el-col) {
  margin-bottom: 12px;
}
.mini {
  background: var(--el-fill-color-light);
  border-radius: 8px;
  padding: 12px;
  min-height: 72px;
}
.mini-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 6px;
}
.mini-value {
  font-size: 18px;
  font-weight: 600;
  word-break: break-all;
}
.mini-value.warn {
  color: var(--el-color-warning);
}
.chart-card {
  margin-bottom: 16px;
}
.chart {
  height: 280px;
}
</style>
