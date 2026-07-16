<template>
  <AppLayout>
    <div class="usage-page">
      <UsageStatsCards :stats="usageStats" />
      <section class="usage-analytics">
        <div class="usage-analytics-toolbar">
          <div class="usage-filter-row">
            <div class="usage-filter-control usage-filter-control--date">
              <span class="usage-filter-label">{{ t('admin.dashboard.timeRange') }}</span>
              <DateRangePicker
                v-model:start-date="startDate"
                v-model:end-date="endDate"
                @change="onDateRangeChange"
              />
            </div>
            <div class="usage-filter-control usage-filter-control--granularity">
              <span class="usage-filter-label">{{ t('admin.dashboard.granularity') }}</span>
              <div class="w-28">
                <Select v-model="granularity" :options="granularityOptions" @change="loadChartData" />
              </div>
            </div>
          </div>
        </div>
        <div class="usage-analytics-grid">
          <div class="usage-analytics-panel usage-analytics-panel--trend">
            <TokenUsageTrend size="large" :trend-data="trendData" :loading="chartsLoading" />
          </div>
          <div class="usage-analytics-panel">
            <ModelDistributionChart
              v-model:source="modelDistributionSource"
              v-model:metric="modelDistributionMetric"
              :model-stats="requestedModelStats"
              :upstream-model-stats="upstreamModelStats"
              :mapping-model-stats="mappingModelStats"
              :loading="modelStatsLoading"
              :show-source-toggle="true"
              :show-metric-toggle="true"
              :start-date="startDate"
              :end-date="endDate"
              :filters="breakdownFilters"
            />
          </div>
          <div class="usage-analytics-panel">
            <GroupDistributionChart
              v-model:metric="groupDistributionMetric"
              :group-stats="groupStats"
              :loading="chartsLoading"
              :show-metric-toggle="true"
              :start-date="startDate"
              :end-date="endDate"
              :filters="breakdownFilters"
            />
          </div>
          <div class="usage-analytics-panel usage-analytics-panel--wide">
            <EndpointDistributionChart
              v-model:source="endpointDistributionSource"
              v-model:metric="endpointDistributionMetric"
              :endpoint-stats="inboundEndpointStats"
              :upstream-endpoint-stats="upstreamEndpointStats"
              :endpoint-path-stats="endpointPathStats"
              :loading="endpointStatsLoading"
              :show-source-toggle="true"
              :show-metric-toggle="true"
              :title="t('usage.endpointDistribution')"
              :start-date="startDate"
              :end-date="endDate"
              :filters="breakdownFilters"
            />
          </div>
        </div>
      </section>
      <section class="usage-records">
      <UsageFilters v-model="filters" :start-date="startDate" :end-date="endDate" :exporting="exporting" @change="applyFilters" @refresh="refreshData" @reset="resetFilters" @cleanup="openCleanupDialog" @export="exportToExcel">
        <template #after-reset>
          <div class="relative" ref="columnDropdownRef">
            <UiIconButton
              :label="t('admin.users.columnSettings')"
              @click="showColumnDropdown = !showColumnDropdown"
            >
              <Icon name="grid" size="md" />
            </UiIconButton>
            <div
              v-if="showColumnDropdown"
              class="absolute right-0 top-full z-50 mt-1 max-h-80 w-48 overflow-y-auto rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-1 shadow-lg"
            >
              <button
                v-for="col in toggleableColumns"
                :key="col.key"
                @click="toggleColumn(col.key)"
                class="flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm text-[var(--app-muted-strong)] hover:bg-[var(--app-surface-muted)] hover:text-[var(--app-text)]"
              >
                <span>{{ col.label }}</span>
                <Icon
                  v-if="isColumnVisible(col.key)"
                  name="check"
                  size="sm"
                  class="text-[var(--app-text)]"
                  :stroke-width="2"
                />
              </button>
            </div>
          </div>
        </template>
      </UsageFilters>
      <UsageTable
        :data="usageLogs"
        :loading="loading"
        :columns="visibleColumns"
        :server-side-sort="true"
        :default-sort-key="'created_at'"
        :default-sort-order="'desc'"
        @sort="handleSort"
        @userClick="handleUserClick"
        @ipGeoBatchFailed="handleIpGeoBatchFailed"
      />
      <Pagination v-if="pagination.total > 0" :page="pagination.page" :total="pagination.total" :page-size="pagination.page_size" @update:page="handlePageChange" @update:pageSize="handlePageSizeChange" />
      </section>
    </div>
  </AppLayout>
  <UsageExportProgress :show="exportProgress.show" :progress="exportProgress.progress" :current="exportProgress.current" :total="exportProgress.total" :estimated-time="exportProgress.estimatedTime" @cancel="cancelExport" />
  <UsageCleanupDialog
    :show="cleanupDialogVisible"
    :filters="filters"
    :start-date="startDate"
    :end-date="endDate"
    @close="cleanupDialogVisible = false"
  />
  <!-- Balance history modal triggered from usage table user click -->
  <UserBalanceHistoryModal
    :show="showBalanceHistoryModal"
    :user="balanceHistoryUser"
    :hide-actions="true"
    @close="showBalanceHistoryModal = false; balanceHistoryUser = null"
  />
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'; import { adminAPI } from '@/api/admin'; import { adminUsageAPI } from '@/api/admin/usage'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatReasoningEffort } from '@/utils/format'
import { resolveUsageRequestType, requestTypeToLegacyStream } from '@/utils/usageRequestType'
import AppLayout from '@/components/layout/AppLayout.vue'; import Pagination from '@/components/common/Pagination.vue'; import Select from '@/components/common/Select.vue'; import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageStatsCards from '@/components/admin/usage/UsageStatsCards.vue'; import UsageFilters from '@/components/admin/usage/UsageFilters.vue'
import UsageTable from '@/components/admin/usage/UsageTable.vue'; import UsageExportProgress from '@/components/admin/usage/UsageExportProgress.vue'
import UsageCleanupDialog from '@/components/admin/usage/UsageCleanupDialog.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'; import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'; import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import Icon from '@/components/icons/Icon.vue'
import { UiIconButton } from '@/ui'
import type { AdminUsageLog, TrendDataPoint, ModelStat, GroupStat, EndpointStat, AdminUser } from '@/types'; import type { AdminUsageStatsResponse, AdminUsageQueryParams } from '@/api/admin/usage'
import type { DashboardSnapshotV2Stats } from '@/api/admin/dashboard'

const { t } = useI18n()
const appStore = useAppStore()
type DistributionMetric = 'tokens' | 'actual_cost'
type EndpointSource = 'inbound' | 'upstream' | 'path'
type ModelDistributionSource = 'requested' | 'upstream' | 'mapping'
const route = useRoute()
const usageStats = ref<AdminUsageStatsResponse | null>(null); const usageLogs = ref<AdminUsageLog[]>([]); const loading = ref(false); const exporting = ref(false)
const trendData = ref<TrendDataPoint[]>([]); const requestedModelStats = ref<ModelStat[]>([]); const upstreamModelStats = ref<ModelStat[]>([]); const mappingModelStats = ref<ModelStat[]>([]); const groupStats = ref<GroupStat[]>([]); const chartsLoading = ref(false); const modelStatsLoading = ref(false); const granularity = ref<'day' | 'hour'>('hour')
const modelDistributionMetric = ref<DistributionMetric>('tokens')
const modelDistributionSource = ref<ModelDistributionSource>('requested')
const loadedModelSources = reactive<Record<ModelDistributionSource, boolean>>({
  requested: false,
  upstream: false,
  mapping: false,
})
const groupDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionSource = ref<EndpointSource>('inbound')
const inboundEndpointStats = ref<EndpointStat[]>([])
const upstreamEndpointStats = ref<EndpointStat[]>([])
const endpointPathStats = ref<EndpointStat[]>([])
const endpointStatsLoading = ref(false)
let abortController: AbortController | null = null; let exportAbortController: AbortController | null = null
let chartReqSeq = 0
let statsReqSeq = 0
let modelStatsReqSeq = 0
const exportProgress = reactive({ show: false, progress: 0, current: 0, total: 0, estimatedTime: '' })
const cleanupDialogVisible = ref(false)
// Balance history modal state
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)

const breakdownFilters = computed(() => {
  const f: Record<string, any> = {}
  if (filters.value.user_id) f.user_id = filters.value.user_id
  if (filters.value.api_key_id) f.api_key_id = filters.value.api_key_id
  if (filters.value.account_id) f.account_id = filters.value.account_id
  if (filters.value.group_id) f.group_id = filters.value.group_id
  if (filters.value.request_type != null) f.request_type = filters.value.request_type
  if (filters.value.billing_type != null) f.billing_type = filters.value.billing_type
  return f
})

const toFiniteNumber = (value: unknown): number => {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

const normalizeUsageStats = (
  raw: Partial<AdminUsageStatsResponse & DashboardSnapshotV2Stats> | null | undefined
): AdminUsageStatsResponse => {
  const totalInputTokens = toFiniteNumber(raw?.total_input_tokens)
  const totalOutputTokens = toFiniteNumber(raw?.total_output_tokens)
  const totalCacheTokens =
    toFiniteNumber(raw?.total_cache_tokens) ||
    toFiniteNumber(raw?.total_cache_creation_tokens) + toFiniteNumber(raw?.total_cache_read_tokens)
  return {
    total_requests: toFiniteNumber(raw?.total_requests),
    total_input_tokens: totalInputTokens,
    total_output_tokens: totalOutputTokens,
    total_cache_tokens: totalCacheTokens,
    total_tokens: toFiniteNumber(raw?.total_tokens) || totalInputTokens + totalOutputTokens + totalCacheTokens,
    total_cost: toFiniteNumber(raw?.total_cost),
    total_actual_cost: toFiniteNumber(raw?.total_actual_cost),
    total_account_cost: toFiniteNumber(raw?.total_account_cost),
    average_duration_ms: toFiniteNumber(raw?.average_duration_ms),
    endpoints: raw?.endpoints || [],
    upstream_endpoints: raw?.upstream_endpoints || [],
    endpoint_paths: raw?.endpoint_paths || []
  }
}

const isZeroUsageStats = (stats: AdminUsageStatsResponse) =>
  stats.total_requests === 0 &&
  stats.total_tokens === 0 &&
  stats.total_cost === 0 &&
  stats.total_actual_cost === 0 &&
  stats.total_account_cost === 0

const hasDetailedStatsFilters = () => {
  const f = filters.value
  return Boolean(
    f.user_id ||
    f.api_key_id ||
    f.account_id ||
    f.group_id ||
    f.model ||
    f.request_type != null ||
    f.stream != null ||
    f.billing_type != null ||
    f.billing_mode
  )
}

const buildUsageStatsParams = () => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
  return {
    start_date: filters.value.start_date || startDate.value,
    end_date: filters.value.end_date || endDate.value,
    user_id: filters.value.user_id,
    model: filters.value.model,
    api_key_id: filters.value.api_key_id,
    account_id: filters.value.account_id,
    group_id: filters.value.group_id,
    request_type: requestType,
    stream: legacyStream === null ? undefined : legacyStream,
    billing_type: filters.value.billing_type,
    billing_mode: filters.value.billing_mode
  }
}

const loadDateRangeStatsFallback = async (): Promise<AdminUsageStatsResponse | null> => {
  if (hasDetailedStatsFilters()) {
    return null
  }
  const snapshot = await adminAPI.dashboard.getSnapshotV2({
    start_date: filters.value.start_date || startDate.value,
    end_date: filters.value.end_date || endDate.value,
    granularity: granularity.value,
    include_stats: true,
    include_trend: false,
    include_model_stats: false,
    include_group_stats: false,
    include_users_trend: false
  })
  return snapshot.stats ? normalizeUsageStats(snapshot.stats) : null
}

const handleUserClick = async (userId: number) => {
  try {
    const user = await adminAPI.users.getById(userId)
    balanceHistoryUser.value = user
    showBalanceHistoryModal.value = true
  } catch {
    appStore.showError(t('admin.usage.failedToLoadUser'))
  }
}

const granularityOptions = computed(() => [{ value: 'day', label: t('admin.dashboard.day') }, { value: 'hour', label: t('admin.dashboard.hour') }])
// Use local timezone to avoid UTC timezone issues
const formatLD = (d: Date) => {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLD(start),
    end: formatLD(end)
  }
}
const getGranularityForRange = (start: string, end: string): 'day' | 'hour' => {
  const startTime = new Date(`${start}T00:00:00`).getTime()
  const endTime = new Date(`${end}T00:00:00`).getTime()
  const daysDiff = Math.ceil((endTime - startTime) / (1000 * 60 * 60 * 24))
  return daysDiff <= 1 ? 'hour' : 'day'
}
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start); const endDate = ref(defaultRange.end)
const filters = ref<AdminUsageQueryParams>({ user_id: undefined, model: undefined, group_id: undefined, request_type: undefined, billing_type: null, start_date: startDate.value, end_date: endDate.value })
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const getSingleQueryValue = (value: string | null | Array<string | null> | undefined): string | undefined => {
  if (Array.isArray(value)) return value.find((item): item is string => typeof item === 'string' && item.length > 0)
  return typeof value === 'string' && value.length > 0 ? value : undefined
}

const getNumericQueryValue = (value: string | null | Array<string | null> | undefined): number | undefined => {
  const raw = getSingleQueryValue(value)
  if (!raw) return undefined
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : undefined
}

const applyRouteQueryFilters = () => {
  const queryStartDate = getSingleQueryValue(route.query.start_date)
  const queryEndDate = getSingleQueryValue(route.query.end_date)
  const queryUserId = getNumericQueryValue(route.query.user_id)

  if (queryStartDate) {
    startDate.value = queryStartDate
  }
  if (queryEndDate) {
    endDate.value = queryEndDate
  }

  filters.value = {
    ...filters.value,
    user_id: queryUserId,
    start_date: startDate.value,
    end_date: endDate.value
  }
  granularity.value = getGranularityForRange(startDate.value, endDate.value)
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  filters.value = {
    ...filters.value,
    start_date: range.startDate,
    end_date: range.endDate
  }
  granularity.value = getGranularityForRange(range.startDate, range.endDate)
  applyFilters()
}

const buildUsageListParams = (
  page: number,
  pageSize: number,
  exactTotal: boolean
): AdminUsageQueryParams => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
  return {
    page,
    page_size: pageSize,
    exact_total: exactTotal,
    ...filters.value,
    stream: legacyStream === null ? undefined : legacyStream,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  }
}

const loadLogs = async () => {
  abortController?.abort(); const c = new AbortController(); abortController = c; loading.value = true
  try {
    const res = await adminAPI.usage.list(
      buildUsageListParams(pagination.page, pagination.page_size, false),
      { signal: c.signal }
    )
    if(!c.signal.aborted) { usageLogs.value = res.items; pagination.total = res.total }
  } catch (error: any) { if(error?.name !== 'AbortError') console.error('Failed to load usage logs:', error) } finally { if(abortController === c) loading.value = false }
}
const loadStats = async () => {
  const seq = ++statsReqSeq
  endpointStatsLoading.value = true
  try {
    const s = normalizeUsageStats(await adminAPI.usage.getStats(buildUsageStatsParams()))
    let nextStats = s
    if (isZeroUsageStats(s)) {
      const fallbackStats = await loadDateRangeStatsFallback()
      if (fallbackStats && !isZeroUsageStats(fallbackStats)) {
        nextStats = fallbackStats
      }
    }
    if (seq !== statsReqSeq) return
    usageStats.value = nextStats
    inboundEndpointStats.value = nextStats.endpoints || []
    upstreamEndpointStats.value = nextStats.upstream_endpoints || []
    endpointPathStats.value = nextStats.endpoint_paths || []
  } catch (error) {
    if (seq !== statsReqSeq) return
    console.error('Failed to load usage stats:', error)
    try {
      const fallbackStats = await loadDateRangeStatsFallback()
      if (seq !== statsReqSeq) return
      usageStats.value = fallbackStats || normalizeUsageStats(null)
      inboundEndpointStats.value = fallbackStats?.endpoints || []
      upstreamEndpointStats.value = fallbackStats?.upstream_endpoints || []
      endpointPathStats.value = fallbackStats?.endpoint_paths || []
    } catch (fallbackError) {
      if (seq !== statsReqSeq) return
      console.error('Failed to load usage stats fallback:', fallbackError)
      usageStats.value = normalizeUsageStats(null)
      inboundEndpointStats.value = []
      upstreamEndpointStats.value = []
      endpointPathStats.value = []
    }
  } finally {
    if (seq === statsReqSeq) endpointStatsLoading.value = false
  }
}

const resetModelStatsCache = () => {
  requestedModelStats.value = []
  upstreamModelStats.value = []
  mappingModelStats.value = []
  loadedModelSources.requested = false
  loadedModelSources.upstream = false
  loadedModelSources.mapping = false
}

const loadModelStats = async (source: ModelDistributionSource, force = false) => {
  if (!force && loadedModelSources[source]) {
    return
  }

  const seq = ++modelStatsReqSeq
  modelStatsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const baseParams = {
      start_date: filters.value.start_date || startDate.value,
      end_date: filters.value.end_date || endDate.value,
      user_id: filters.value.user_id,
      model: filters.value.model,
      api_key_id: filters.value.api_key_id,
      account_id: filters.value.account_id,
      group_id: filters.value.group_id,
      request_type: requestType,
      stream: legacyStream === null ? undefined : legacyStream,
      billing_type: filters.value.billing_type,
    }

    const response = await adminAPI.dashboard.getModelStats({ ...baseParams, model_source: source })

    if (seq !== modelStatsReqSeq) return

    const models = response.models || []
    if (source === 'requested') {
      requestedModelStats.value = models
    } else if (source === 'upstream') {
      upstreamModelStats.value = models
    } else {
      mappingModelStats.value = models
    }
    loadedModelSources[source] = true
  } catch (error) {
    if (seq !== modelStatsReqSeq) return
    console.error('Failed to load model stats:', error)
    if (source === 'requested') {
      requestedModelStats.value = []
    } else if (source === 'upstream') {
      upstreamModelStats.value = []
    } else {
      mappingModelStats.value = []
    }
    loadedModelSources[source] = false
  } finally {
    if (seq === modelStatsReqSeq) modelStatsLoading.value = false
  }
}

const loadChartData = async () => {
  const seq = ++chartReqSeq
  chartsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const snapshot = await adminAPI.dashboard.getSnapshotV2({
      start_date: filters.value.start_date || startDate.value,
      end_date: filters.value.end_date || endDate.value,
      granularity: granularity.value,
      user_id: filters.value.user_id,
      model: filters.value.model,
      api_key_id: filters.value.api_key_id,
      account_id: filters.value.account_id,
      group_id: filters.value.group_id,
      request_type: requestType,
      stream: legacyStream === null ? undefined : legacyStream,
      billing_type: filters.value.billing_type,
      include_stats: false,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: true,
      include_users_trend: false
    })
    if (seq !== chartReqSeq) return
    trendData.value = snapshot.trend || []
    groupStats.value = snapshot.groups || []
  } catch (error) { console.error('Failed to load chart data:', error) } finally { if (seq === chartReqSeq) chartsLoading.value = false }
}
const applyFilters = () => {
  pagination.page = 1
  resetModelStatsCache()
  loadLogs()
  loadStats()
  loadModelStats(modelDistributionSource.value, true)
  loadChartData()
}
const refreshData = () => {
  resetModelStatsCache()
  loadLogs()
  loadStats()
  loadModelStats(modelDistributionSource.value, true)
  loadChartData()
}
const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = { start_date: startDate.value, end_date: endDate.value, request_type: undefined, billing_type: null, billing_mode: undefined }
  granularity.value = getGranularityForRange(startDate.value, endDate.value)
  applyFilters()
}
const handlePageChange = (p: number) => { pagination.page = p; loadLogs() }
const handlePageSizeChange = (s: number) => { pagination.page_size = s; pagination.page = 1; loadLogs() }
const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadLogs()
}
const handleIpGeoBatchFailed = () => {
  appStore.showError(t('usage.ipGeo.batchFailed'))
}
const cancelExport = () => exportAbortController?.abort()
const openCleanupDialog = () => { cleanupDialogVisible.value = true }
const getRequestTypeLabel = (log: AdminUsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

const formatReasoningTokens = (value?: number | null): string => {
  if (!value || value <= 0) return '-'
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`
  return value.toLocaleString()
}

type CsvCell = string | number | boolean | null | undefined

const CSV_FORMULA_PREFIX_PATTERN = /^\s*[=+\-@]/
const CSV_ESCAPE_PATTERN = /[",\r\n]/
const CSV_BOM = '\uFEFF'

const escapeCsvCell = (value: CsvCell): string => {
  if (value == null) return ''
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)

  let text = value
  if (CSV_FORMULA_PREFIX_PATTERN.test(text)) {
    text = `'${text}`
  }

  if (CSV_ESCAPE_PATTERN.test(text)) {
    return `"${text.replace(/"/g, '""')}"`
  }

  return text
}

const toCsvRow = (row: CsvCell[]): string => row.map(escapeCsvCell).join(',')

const exportToExcel = async () => {
  if (exporting.value) return; exporting.value = true; exportProgress.show = true
  const c = new AbortController(); exportAbortController = c
  try {
    let p = 1; let total = pagination.total; let exportedCount = 0
    const headers = [
      t('usage.time'), t('admin.usage.user'), t('usage.apiKeyFilter'),
      t('admin.usage.account'), t('usage.model'), t('usage.upstreamModel'), t('usage.reasoningEffort'), t('usage.reasoningTokens'), t('admin.usage.group'),
      t('usage.inboundEndpoint'), t('usage.upstreamEndpoint'),
      t('usage.type'),
      t('admin.usage.inputTokens'), t('admin.usage.outputTokens'),
      t('admin.usage.cacheReadTokens'), t('admin.usage.cacheCreationTokens'),
      t('admin.usage.inputCost'), t('admin.usage.outputCost'),
      t('admin.usage.cacheReadCost'), t('admin.usage.cacheCreationCost'),
      t('usage.rate'), t('usage.accountMultiplier'), t('usage.original'), t('usage.userBilled'), t('usage.accountBilled'),
      t('usage.firstToken'), t('usage.duration'),
      t('admin.usage.requestId'), t('usage.userAgent'), t('admin.usage.ipAddress')
    ]
    const csvRows = [toCsvRow(headers)]
    while (true) {
      const res = await adminUsageAPI.list(
        buildUsageListParams(p, 100, true),
        { signal: c.signal }
      )
      if (c.signal.aborted) break; if (p === 1) { total = res.total; exportProgress.total = total }
      const rows = (res.items || []).map((log: AdminUsageLog) => [
        log.created_at, log.user?.email || '', log.api_key?.name || '', log.account?.name || '', log.model,
        log.upstream_model || '', formatReasoningEffort(log.reasoning_effort), formatReasoningTokens(log.reasoning_tokens), log.group?.name || '',
        log.inbound_endpoint || '', log.upstream_endpoint || '', getRequestTypeLabel(log),
        log.input_tokens, log.output_tokens, log.cache_read_tokens, log.cache_creation_tokens,
        log.input_cost?.toFixed(6) || '0.000000', log.output_cost?.toFixed(6) || '0.000000',
        log.cache_read_cost?.toFixed(6) || '0.000000', log.cache_creation_cost?.toFixed(6) || '0.000000',
        log.rate_multiplier?.toPrecision(4) || '1.00', (log.account_rate_multiplier ?? 1).toPrecision(4),
        log.total_cost?.toFixed(6) || '0.000000', log.actual_cost?.toFixed(6) || '0.000000',
        ((log.account_stats_cost ?? log.total_cost) * (log.account_rate_multiplier ?? 1)).toFixed(6), log.first_token_ms ?? '', log.duration_ms,
        log.request_id || '', log.user_agent || '', log.ip_address || ''
      ])
      if (rows.length) {
        csvRows.push(...rows.map(toCsvRow))
      }
      exportedCount += rows.length
      exportProgress.current = exportedCount
      exportProgress.progress = total > 0 ? Math.min(100, Math.round(exportedCount / total * 100)) : 0
      if (exportedCount >= total || res.items.length < 100) break; p++
    }
    if(!c.signal.aborted) {
      saveAs(new Blob([CSV_BOM, csvRows.join('\r\n')], { type: 'text/csv;charset=utf-8' }), `usage_${filters.value.start_date}_to_${filters.value.end_date}.csv`)
      appStore.showSuccess(t('usage.exportSuccess'))
    }
  } catch (error) { console.error('Failed to export:', error); appStore.showError('Export Failed') }
  finally { if(exportAbortController === c) { exportAbortController = null; exporting.value = false; exportProgress.show = false } }
}

// Column visibility
const ALWAYS_VISIBLE = ['user', 'created_at']
const DEFAULT_HIDDEN_COLUMNS = ['user_agent']
const HIDDEN_COLUMNS_KEY = 'usage-hidden-columns-v2'

const allColumns = computed(() => [
  { key: 'user', label: t('admin.usage.user'), sortable: false },
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'account', label: t('admin.usage.account'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'reasoning_tokens', label: t('usage.reasoningTokens'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'group', label: t('admin.usage.group'), sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'first_token', label: t('usage.firstToken'), sortable: false },
  { key: 'duration', label: t('usage.duration'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
  { key: 'ip_address', label: t('admin.usage.ipAddress'), sortable: false }
])

const hiddenColumns = reactive<Set<string>>(new Set())

const toggleableColumns = computed(() =>
  allColumns.value.filter(col => !ALWAYS_VISIBLE.includes(col.key))
)

const visibleColumns = computed(() =>
  allColumns.value.filter(col =>
    ALWAYS_VISIBLE.includes(col.key) || !hiddenColumns.has(col.key)
  )
)

const isColumnVisible = (key: string) => !hiddenColumns.has(key)

const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      (JSON.parse(saved) as string[]).forEach((key) => {
        hiddenColumns.add(key)
      })
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach((key) => {
        hiddenColumns.add(key)
      })
    }
  } catch {
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => {
      hiddenColumns.add(key)
    })
  }
}

const showColumnDropdown = ref(false)
const columnDropdownRef = ref<HTMLElement | null>(null)

const handleColumnClickOutside = (event: MouseEvent) => {
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(event.target as HTMLElement)) {
    showColumnDropdown.value = false
  }
}

onMounted(() => {
  applyRouteQueryFilters()
  loadLogs()
  loadStats()
  loadModelStats(modelDistributionSource.value, true)
  window.setTimeout(() => {
    void loadChartData()
  }, 120)
  loadSavedColumns()
  document.addEventListener('click', handleColumnClickOutside)
})
onUnmounted(() => { abortController?.abort(); exportAbortController?.abort(); document.removeEventListener('click', handleColumnClickOutside) })

watch(modelDistributionSource, (source) => {
  void loadModelStats(source)
})
</script>

<style scoped>
.usage-page {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 1.5rem;
}

.usage-analytics {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: transparent;
}

.usage-analytics-toolbar {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--ui-border);
}

.usage-filter-row,
.usage-filter-control {
  display: flex;
  min-width: 0;
  align-items: center;
}

.usage-filter-row {
  justify-content: space-between;
  gap: 0.75rem;
}

.usage-filter-control {
  gap: 0.5rem;
}

.usage-filter-label {
  color: var(--ui-text-secondary);
  font-size: 0.8125rem;
  font-weight: 500;
  white-space: nowrap;
}

.usage-analytics-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.usage-analytics-panel {
  min-width: 0;
  padding: 1rem 1.125rem;
  border-top: 1px solid var(--ui-border);
}

.usage-analytics-panel:nth-child(3) {
  border-left: 1px solid var(--ui-border);
}

.usage-analytics-panel--trend {
  grid-column: 1 / -1;
  border-top: 0;
}

.usage-analytics-panel--wide {
  grid-column: 1 / -1;
}

.usage-records {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 1rem;
}

@media (max-width: 900px) {
  .usage-analytics-grid {
    grid-template-columns: 1fr;
  }

  .usage-analytics-panel,
  .usage-analytics-panel--trend,
  .usage-analytics-panel--wide {
    grid-column: auto;
  }

  .usage-analytics-panel:nth-child(3) {
    border-left: 0;
  }
}

@media (max-width: 640px) {
  .usage-page {
    gap: 1rem;
  }

  .usage-analytics-toolbar,
  .usage-analytics-panel {
    padding-inline: 0.875rem;
  }

  .usage-filter-row {
    align-items: stretch;
  }

  .usage-filter-label {
    display: none;
  }

  .usage-filter-control--date {
    flex: 1 1 auto;
  }

  .usage-filter-control--granularity {
    flex: 0 0 auto;
  }
}
</style>
