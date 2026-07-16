<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full sm:w-72">
            <Icon
              name="search"
              size="md"
              class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
            />
            <input
              v-model="searchQuery"
              type="text"
              class="input pl-10"
              :placeholder="t('admin.carpools.searchPlaceholder')"
              @input="handleSearch"
            />
          </div>

          <div class="w-full sm:w-44">
            <Select v-model="filters.platform" :options="platformOptions" @change="reloadFromFirstPage" />
          </div>
          <div class="w-full sm:w-44">
            <Select v-model="filters.status" :options="statusOptions" @change="reloadFromFirstPage" />
          </div>

          <div class="flex flex-1 justify-end">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPools">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="pools" :loading="loading">
          <template #cell-name="{ row }">
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-900 dark:text-gray-100">
                {{ row.pool.name }}
              </div>
              <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
                <span>#{{ row.pool.id }}</span>
                <span>{{ row.pool.platform }}</span>
                <code class="rounded bg-[#f3f3f6] px-1.5 py-0.5 text-[#565869] dark:bg-[#2f2f2f] dark:text-[#acacbe]">
                  {{ row.pool.invite_code }}
                </code>
              </div>
            </div>
          </template>

          <template #cell-owner="{ row }">
            <div class="min-w-0 text-sm">
              <div class="truncate text-gray-900 dark:text-gray-100">
                {{ row.owner_email || '-' }}
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ row.owner_username || t('admin.carpools.ownerId', { id: row.pool.owner_user_id }) }}
              </div>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span class="badge" :class="statusClass(row.pool.status)">
              {{ statusLabel(row.pool.status) }}
            </span>
          </template>

          <template #cell-seats="{ row }">
            <div class="text-sm text-gray-700 dark:text-gray-200">
              {{ row.active_members }}/{{ row.pool.target_seats }}
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('carpool.pendingApplications') }} {{ row.pending_applications }}
            </div>
          </template>

          <template #cell-quota="{ row }">
            <div class="space-y-0.5 text-sm text-gray-700 dark:text-gray-200">
              <div>{{ t('carpool.perMemberFiveHourQuota') }}: {{ formatMoney(row.pool.per_member_five_hour_limit_usd) }}</div>
              <div>{{ t('carpool.perMemberWeeklyQuota') }}: {{ formatMoney(row.pool.per_member_weekly_limit_usd) }}</div>
            </div>
          </template>

          <template #cell-accounts="{ row }">
            <div class="text-sm text-gray-700 dark:text-gray-200">
              {{ row.bound_account_count }}
            </div>
            <div class="max-w-[220px] truncate text-xs text-gray-500 dark:text-gray-400">
              {{ row.group_name || '-' }}
            </div>
          </template>

          <template #cell-created_at="{ row }">
            <span class="text-sm text-gray-600 dark:text-gray-300">
              {{ formatDate(row.pool.created_at) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex flex-wrap items-center gap-2">
              <button type="button" class="btn btn-ghost btn-sm" @click="openDetail(row)">
                <Icon name="eye" size="sm" class="mr-1" />
                {{ t('common.view') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="row.pool.status === 'closed'"
                @click="openConfirm('close', row)"
              >
                {{ t('admin.carpools.closePool') }}
              </button>
              <button type="button" class="btn btn-danger btn-sm" @click="openConfirm('delete', row)">
                <Icon name="trash" size="sm" class="mr-1" />
                {{ t('common.delete') }}
              </button>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :page-size="pagination.page_size"
          :total="pagination.total"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="showDetail"
      :title="t('admin.carpools.detailTitle')"
      width="extra-wide"
      @close="closeDetail"
    >
      <div v-if="detailLoading" class="py-12 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </div>
      <div v-else-if="detailData" class="space-y-6">
        <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div class="rounded-lg border border-[#d9d9e3] bg-[#ffffff] p-3 dark:border-[#3f3f46] dark:bg-[#2f2f2f]">
            <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.seats') }}</div>
            <div class="mt-1 text-lg font-semibold text-[#202123] dark:text-[#ececf1]">
              {{ detailData.summary.active_members }}/{{ detailData.pool.target_seats }}
            </div>
          </div>
          <div class="rounded-lg border border-[#d9d9e3] bg-[#ffffff] p-3 dark:border-[#3f3f46] dark:bg-[#2f2f2f]">
            <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.pendingApplications') }}</div>
            <div class="mt-1 text-lg font-semibold text-[#202123] dark:text-[#ececf1]">
              {{ detailData.summary.pending_applications }}
            </div>
          </div>
          <div class="rounded-lg border border-[#d9d9e3] bg-[#ffffff] p-3 dark:border-[#3f3f46] dark:bg-[#2f2f2f]">
            <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.boundAccounts') }}</div>
            <div class="mt-1 text-lg font-semibold text-[#202123] dark:text-[#ececf1]">
              {{ detailData.accounts.length }}
            </div>
          </div>
          <div class="rounded-lg border border-[#d9d9e3] bg-[#ffffff] p-3 dark:border-[#3f3f46] dark:bg-[#2f2f2f]">
            <div class="text-xs text-[#6e6e80] dark:text-[#acacbe]">{{ t('carpool.inviteCode') }}</div>
            <div class="mt-1 truncate font-mono text-sm font-semibold text-[#202123] dark:text-[#ececf1]">
              {{ detailData.pool.invite_code }}
            </div>
          </div>
        </div>

        <section class="space-y-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.carpools.poolInfo') }}</h3>
          <div class="grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-3">
            <InfoItem :label="t('carpool.name')" :value="detailData.pool.name" />
            <InfoItem :label="t('carpool.platform')" :value="detailData.pool.platform" />
            <InfoItem :label="t('carpool.owner')" :value="detailOwnerLabel" />
            <InfoItem :label="t('carpool.currentUserStatus')" :value="statusLabel(detailData.pool.status)" />
            <InfoItem :label="t('carpool.group')" :value="detailData.summary.group_name || '-'" />
            <InfoItem :label="t('carpool.seatPrice')" :value="formatMoney(detailData.pool.seat_price)" />
            <InfoItem :label="t('carpool.extraFee')" :value="formatMoney(detailData.pool.extra_fee)" />
            <InfoItem :label="t('carpool.totalFiveHourQuota')" :value="formatMoney(detailData.pool.total_five_hour_limit_usd)" />
            <InfoItem :label="t('carpool.totalWeeklyQuota')" :value="formatMoney(detailData.pool.total_weekly_limit_usd)" />
            <InfoItem :label="t('carpool.durationDays')" :value="String(detailData.pool.duration_days)" />
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('carpool.accounts') }}</h3>
          <div v-if="detailData.accounts.length === 0" class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('carpool.noAccountsBound') }}
          </div>
          <div v-else class="grid gap-2 md:grid-cols-2">
            <div
              v-for="account in detailData.accounts"
              :key="account.id"
              class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-600"
            >
              <div class="font-medium text-gray-900 dark:text-gray-100">{{ account.name || '-' }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                #{{ account.account_id }} - {{ account.platform }}
              </div>
            </div>
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('carpool.members') }}</h3>
          <div class="grid gap-2 md:grid-cols-2">
            <div
              v-for="member in detailData.members"
              :key="member.member.id"
              class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-600"
            >
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="min-w-0">
                  <div class="truncate font-medium text-gray-900 dark:text-gray-100">
                    {{ member.masked_email }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ member.username || t('admin.carpools.userId', { id: member.member.user_id }) }}
                  </div>
                </div>
                <span class="badge" :class="member.member.role === 'owner' ? 'badge-primary' : 'badge-gray'">
                  {{ member.member.role === 'owner' ? t('carpool.owner') : t('carpool.member') }}
                </span>
              </div>
              <div class="mt-2 grid gap-1 text-xs text-gray-500 dark:text-gray-400">
                <span>{{ t('carpool.fiveHourUsage') }}: {{ formatMoney(member.member.five_hour_used_usd) }} / {{ formatMoney(member.member.five_hour_limit_usd) }}</span>
                <span>{{ t('carpool.weeklyUsage') }}: {{ formatMoney(member.weekly_usage_usd) }} / {{ formatMoney(member.weekly_limit_usd) }}</span>
                <span>{{ t('carpool.totalTokens') }}: {{ formatInteger(member.total_tokens) }}</span>
                <span>{{ t('carpool.totalCost') }}: {{ formatMoney(member.total_cost_usd) }}</span>
              </div>
            </div>
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('carpool.joinRequests') }}</h3>
          <div v-if="detailData.join_requests.length === 0" class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('carpool.noJoinRequests') }}
          </div>
          <div v-else class="space-y-2">
            <div
              v-for="requestProfile in detailData.join_requests"
              :key="requestProfile.request.id"
              class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-600"
            >
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="min-w-0">
                  <div class="truncate font-medium text-gray-900 dark:text-gray-100">
                    {{ requestProfile.masked_email }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ requestProfile.username || t('admin.carpools.userId', { id: requestProfile.request.user_id }) }}
                  </div>
                </div>
                <span class="badge badge-gray">{{ requestStatusLabel(requestProfile.request.status) }}</span>
              </div>
              <div class="mt-2 grid gap-1 text-xs text-gray-500 dark:text-gray-400 sm:grid-cols-2 lg:grid-cols-4">
                <span>{{ t('carpool.totalRequests') }}: {{ formatInteger(requestProfile.usage.total_requests) }}</span>
                <span>{{ t('carpool.totalTokens') }}: {{ formatInteger(requestProfile.usage.total_tokens) }}</span>
                <span>{{ t('carpool.last7dTokens') }}: {{ formatInteger(requestProfile.usage.last_7d_tokens) }}</span>
                <span>{{ t('carpool.last30dTokens') }}: {{ formatInteger(requestProfile.usage.last_30d_tokens) }}</span>
              </div>
              <p v-if="requestProfile.request.note" class="mt-2 break-words text-xs text-gray-600 dark:text-gray-300">
                {{ requestProfile.request.note }}
              </p>
            </div>
          </div>
        </section>
      </div>

      <template #footer>
        <div class="flex flex-wrap justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeDetail">
            {{ t('common.close') }}
          </button>
          <button
            v-if="detailData && detailData.pool.status !== 'closed'"
            type="button"
            class="btn btn-secondary"
            @click="openConfirm('close', summaryFromDetail(detailData))"
          >
            {{ t('admin.carpools.closePool') }}
          </button>
          <button
            v-if="detailData"
            type="button"
            class="btn btn-danger"
            @click="openConfirm('delete', summaryFromDetail(detailData))"
          >
            {{ t('common.delete') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="confirmState.show"
      :title="confirmTitle"
      :message="confirmMessage"
      :confirm-text="confirmText"
      :danger="confirmState.type === 'delete'"
      @confirm="runConfirmedAction"
      @cancel="closeConfirm"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI, type AdminCarpoolPoolSummary } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { CarpoolPoolDetail, CarpoolPoolStatus, SelectOption } from '@/types'
import type { Column } from '@/components/common/types'

const { t } = useI18n()
const appStore = useAppStore()

const InfoItem = defineComponent({
  name: 'InfoItem',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
  },
  setup(props) {
    return () =>
      h('div', { class: 'min-w-0 rounded-lg border border-gray-200 p-3 dark:border-dark-600' }, [
        h('div', { class: 'text-xs text-gray-500 dark:text-gray-400' }, props.label),
        h('div', { class: 'mt-1 break-words text-sm font-medium text-gray-900 dark:text-gray-100' }, props.value),
      ])
  },
})

type ConfirmAction = 'close' | 'delete'

const pools = ref<AdminCarpoolPoolSummary[]>([])
const loading = ref(false)
const detailLoading = ref(false)
const showDetail = ref(false)
const detailData = ref<CarpoolPoolDetail | null>(null)
const detailSummary = ref<AdminCarpoolPoolSummary | null>(null)
const searchQuery = ref('')
const filters = reactive({
  platform: '',
  status: '',
})
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 1,
})
const confirmState = reactive<{
  show: boolean
  type: ConfirmAction
  target: AdminCarpoolPoolSummary | null
}>({
  show: false,
  type: 'close',
  target: null,
})

let abortController: AbortController | null = null
let searchTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('carpool.name') },
  { key: 'owner', label: t('carpool.owner') },
  { key: 'status', label: t('carpool.currentUserStatus') },
  { key: 'seats', label: t('carpool.seats') },
  { key: 'quota', label: t('carpool.quotaSummary') },
  { key: 'accounts', label: t('carpool.boundAccounts') },
  { key: 'created_at', label: t('admin.carpools.createdAt') },
  { key: 'actions', label: t('common.actions') },
])

const platformOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.carpools.allPlatforms') },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Claude' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'grok', label: 'Grok' },
])

const statusOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.carpools.allStatus') },
  { value: 'recruiting', label: t('carpool.statusRecruiting') },
  { value: 'full', label: t('carpool.statusFull') },
  { value: 'closed', label: t('carpool.statusClosed') },
])

const confirmTitle = computed(() =>
  confirmState.type === 'delete'
    ? t('admin.carpools.deleteTitle')
    : t('admin.carpools.closeTitle')
)

const confirmText = computed(() =>
  confirmState.type === 'delete'
    ? t('common.delete')
    : t('admin.carpools.closePool')
)

const confirmMessage = computed(() => {
  const name = confirmState.target?.pool.name || ''
  return confirmState.type === 'delete'
    ? t('admin.carpools.deleteConfirm', { name })
    : t('admin.carpools.closeConfirm', { name })
})

const detailOwnerLabel = computed(() => {
  if (detailSummary.value?.owner_email) {
    return detailSummary.value.owner_username
      ? `${detailSummary.value.owner_email} (${detailSummary.value.owner_username})`
      : detailSummary.value.owner_email
  }
  if (detailData.value?.pool.owner_user_id) {
    return t('admin.carpools.ownerId', { id: detailData.value.pool.owner_user_id })
  }
  return '-'
})

async function loadPools() {
  if (abortController) {
    abortController.abort()
  }
  const current = new AbortController()
  abortController = current
  loading.value = true
  try {
    const response = await adminAPI.carpools.list(
      pagination.page,
      pagination.page_size,
      {
        search: searchQuery.value.trim() || undefined,
        platform: filters.platform || undefined,
        status: filters.status || undefined,
      },
      { signal: current.signal }
    )
    if (current.signal.aborted || abortController !== current) return
    pools.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error) {
    if (!isAbortError(error)) {
      appStore.showError(extractApiErrorMessage(error, t('admin.carpools.loadFailed')))
    }
  } finally {
    if (abortController === current) {
      loading.value = false
      abortController = null
    }
  }
}

function isAbortError(error: unknown) {
  if (!error || typeof error !== 'object') return false
  const maybeError = error as { name?: string; code?: string }
  return maybeError.name === 'AbortError' || maybeError.code === 'ERR_CANCELED'
}

function handleSearch() {
  if (searchTimer) {
    clearTimeout(searchTimer)
  }
  searchTimer = setTimeout(() => {
    reloadFromFirstPage()
  }, 300)
}

function reloadFromFirstPage() {
  pagination.page = 1
  loadPools()
}

function handlePageChange(page: number) {
  pagination.page = page
  loadPools()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadPools()
}

async function openDetail(target: AdminCarpoolPoolSummary | number) {
  const poolId = typeof target === 'number' ? target : target.pool.id
  detailSummary.value = typeof target === 'number' ? null : target
  showDetail.value = true
  detailLoading.value = true
  try {
    detailData.value = await adminAPI.carpools.get(poolId)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.carpools.loadDetailFailed')))
    showDetail.value = false
  } finally {
    detailLoading.value = false
  }
}

function closeDetail() {
  showDetail.value = false
  detailData.value = null
  detailSummary.value = null
}

function openConfirm(type: ConfirmAction, target: AdminCarpoolPoolSummary) {
  confirmState.type = type
  confirmState.target = target
  confirmState.show = true
}

function closeConfirm() {
  confirmState.show = false
  confirmState.target = null
}

async function runConfirmedAction() {
  const target = confirmState.target
  if (!target) return
  try {
    if (confirmState.type === 'delete') {
      await adminAPI.carpools.deletePool(target.pool.id)
      appStore.showSuccess(t('admin.carpools.deleteSuccess'))
      if (detailData.value?.pool.id === target.pool.id) {
        closeDetail()
      }
    } else {
      const detail = await adminAPI.carpools.close(target.pool.id)
      appStore.showSuccess(t('admin.carpools.closeSuccess'))
      if (detailData.value?.pool.id === target.pool.id) {
        detailData.value = detail
      }
    }
    closeConfirm()
    await loadPools()
  } catch (error) {
    appStore.showError(
      extractApiErrorMessage(
        error,
        confirmState.type === 'delete' ? t('admin.carpools.deleteFailed') : t('admin.carpools.closeFailed')
      )
    )
  }
}

function summaryFromDetail(detail: CarpoolPoolDetail): AdminCarpoolPoolSummary {
  if (detailSummary.value?.pool.id === detail.pool.id) {
    return detailSummary.value
  }
  return {
    ...detail.summary,
    owner_email: '',
    owner_username: '',
  }
}

function statusLabel(status: CarpoolPoolStatus | string) {
  switch (status) {
    case 'recruiting':
      return t('carpool.statusRecruiting')
    case 'full':
      return t('carpool.statusFull')
    case 'closed':
      return t('carpool.statusClosed')
    default:
      return status || '-'
  }
}

function statusClass(status: CarpoolPoolStatus | string) {
  switch (status) {
    case 'recruiting':
      return 'badge-success'
    case 'full':
      return 'badge-warning'
    case 'closed':
      return 'badge-gray'
    default:
      return 'badge-gray'
  }
}

function requestStatusLabel(status: string) {
  switch (status) {
    case 'pending':
      return t('carpool.requestPending')
    case 'approved':
      return t('carpool.requestApproved')
    case 'rejected':
      return t('carpool.requestRejected')
    case 'activated':
      return t('carpool.requestActivated')
    default:
      return status || '-'
  }
}

function formatMoney(value: number | null | undefined) {
  const amount = typeof value === 'number' && Number.isFinite(value) ? value : 0
  return `$${amount.toFixed(2)}`
}

function formatInteger(value: number | null | undefined) {
  const amount = typeof value === 'number' && Number.isFinite(value) ? value : 0
  return amount.toLocaleString()
}

function formatDate(value?: string | null) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

onMounted(loadPools)

onUnmounted(() => {
  if (abortController) {
    abortController.abort()
  }
  if (searchTimer) {
    clearTimeout(searchTimer)
  }
})
</script>
