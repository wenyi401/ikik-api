<template>
  <AppLayout>
    <UiPage width="wide">
      <div class="flex flex-col gap-3">
        <div class="revenue-tabs">
          <button
            v-for="tab in revenueTabs"
            :key="tab.key"
            type="button"
            class="revenue-tab"
            :class="activeRevenueTab === tab.key
              ? 'revenue-tab--active'
              : ''"
            :aria-pressed="activeRevenueTab === tab.key"
            @click="activeRevenueTab = tab.key"
          >
            {{ tab.label }}
          </button>
        </div>

        <div v-if="activeRevenueTab === 'overview'" class="flex flex-col gap-3 lg:flex-row lg:flex-wrap lg:items-end">
          <div ref="userSearchRef" class="relative w-full sm:w-[280px]">
            <label class="input-label">{{ t('common.email') }}</label>
            <div class="relative">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                v-model="userKeyword"
                type="text"
                class="input h-10 pl-9 pr-9"
                :placeholder="t('common.searchPlaceholder')"
                @input="debounceUserSearch"
                @focus="showUserDropdown = true"
              />
              <button
                v-if="selectedUserId"
                type="button"
                class="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                :title="t('common.reset')"
                @click="clearUser"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
            <div
              v-if="showUserDropdown && (userResults.length > 0 || userKeyword)"
              class="absolute z-50 mt-1 max-h-64 w-full overflow-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
            >
              <button
                v-for="user in userResults"
                :key="user.id"
                type="button"
                class="flex w-full items-center justify-between gap-3 px-3 py-2 text-left hover:bg-gray-50 dark:hover:bg-dark-700"
                @click="selectUser(user)"
              >
                <span class="truncate text-sm text-gray-900 dark:text-white">{{ user.email }}</span>
                <span class="shrink-0 text-xs text-gray-400">#{{ user.id }}</span>
              </button>
              <div v-if="!userResults.length && userKeyword" class="px-3 py-2 text-sm text-gray-500 dark:text-gray-400">
                {{ t('common.noData') }}
              </div>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-2 sm:w-[300px]">
            <div>
              <label class="input-label">{{ t('dates.startDate') }}</label>
              <input v-model="startDate" type="date" class="input h-10" @change="applyCustomDateRange" />
            </div>
            <div>
              <label class="input-label">{{ t('dates.endDate') }}</label>
              <input v-model="endDate" type="date" class="input h-10" @change="applyCustomDateRange" />
            </div>
          </div>

          <div class="revenue-segment">
            <button
              v-for="option in DAYS_OPTIONS"
              :key="option"
              type="button"
              class="revenue-segment-button"
              :class="selectedRangeDays === option
                ? 'revenue-segment-button--active'
                : ''"
              :disabled="isRangeDisabled(option)"
              @click="setRangeDays(option)"
            >
              {{ t('admin.revenue.controls.rangeDays', { days: option }) }}
            </button>
          </div>

          <div class="revenue-segment">
            <button
              v-for="option in granularityOptions"
              :key="option.value"
              type="button"
              class="revenue-segment-button"
              :class="granularity === option.value
                ? 'revenue-segment-button--active'
                : ''"
              @click="setGranularity(option.value)"
            >
              {{ option.label }}
            </button>
          </div>

          <div class="flex justify-end lg:ml-auto">
            <UiIconButton
              :label="t('common.refresh')"
              :disabled="loading"
              @click="loadSummary"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </UiIconButton>
          </div>
        </div>
      </div>

      <div v-if="activeRevenueTab === 'overview' && loading && !summary" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <template v-else-if="activeRevenueTab === 'overview' && summary">
        <div class="revenue-metrics">
          <div
            v-for="card in statCards"
            :key="card.key"
            class="revenue-metric"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
                <p class="mt-2 break-words text-2xl font-semibold text-gray-900 dark:text-white">{{ card.value }}</p>
              </div>
            </div>
            <p class="mt-3 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ card.meta }}</p>
          </div>
        </div>

        <section class="revenue-section">
          <div class="mb-4 flex items-center justify-between gap-3">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.revenue.chart.title') }}</h3>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ summary.start_date }} - {{ summary.end_date }}</span>
          </div>
          <div v-if="summary.trend.length" class="h-[320px]">
            <Line :data="chartData" :options="chartOptions" />
          </div>
          <div v-else class="flex h-[320px] items-center justify-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.revenue.noData') }}
          </div>
        </section>

        <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
          <section class="revenue-section">
            <h3 class="mb-4 text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.revenue.sections.usage') }}</h3>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-for="row in usageRows" :key="row.key" class="flex items-center justify-between gap-4 py-3">
                <span class="text-sm text-gray-500 dark:text-gray-400">{{ row.label }}</span>
                <span class="text-right text-sm font-medium text-gray-900 dark:text-white">{{ row.value }}</span>
              </div>
            </div>
          </section>

          <section class="revenue-section">
            <h3 class="mb-4 text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.revenue.sections.cash') }}</h3>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-for="row in cashRows" :key="row.key" class="flex items-center justify-between gap-4 py-3">
                <span class="text-sm text-gray-500 dark:text-gray-400">{{ row.label }}</span>
                <span class="text-right text-sm font-medium text-gray-900 dark:text-white">{{ row.value }}</span>
              </div>
            </div>
          </section>

          <section class="revenue-section">
            <h3 class="mb-4 text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.revenue.sections.adjustments') }}</h3>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-for="row in adjustmentRows" :key="row.key" class="flex items-center justify-between gap-4 py-3">
                <span class="text-sm text-gray-500 dark:text-gray-400">{{ row.label }}</span>
                <span class="text-right text-sm font-medium text-gray-900 dark:text-white">{{ row.value }}</span>
              </div>
            </div>
          </section>
        </div>

        <section class="revenue-section">
          <div class="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.revenue.sections.breakdown') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ activeBreakdownHint }}</p>
            </div>
            <div class="revenue-segment max-w-full overflow-x-auto">
              <button
                v-for="tab in breakdownTabs"
                :key="tab.key"
                type="button"
                class="revenue-segment-button whitespace-nowrap"
                :class="activeBreakdown === tab.key
                  ? 'revenue-segment-button--active'
                  : ''"
                :aria-pressed="activeBreakdown === tab.key"
                @click="activeBreakdown = tab.key"
              >
                {{ tab.label }}
              </button>
            </div>
          </div>

          <div v-if="activeBreakdownItems.length" class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
              <thead>
                <tr>
                  <th class="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ t('admin.revenue.table.name') }}
                  </th>
                  <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ t('admin.revenue.table.requests') }}
                  </th>
                  <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ t('admin.revenue.table.tokens') }}
                  </th>
                  <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ breakdownColumnLabels.primary }}
                  </th>
                  <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ breakdownColumnLabels.secondary }}
                  </th>
                  <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ breakdownColumnLabels.tertiary }}
                  </th>
                  <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ breakdownColumnLabels.quaternary }}
                  </th>
                  <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ breakdownColumnLabels.quinary }}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="item in activeBreakdownItems" :key="`${activeBreakdown}-${item.id ?? item.name}`" class="hover:bg-gray-50 dark:hover:bg-dark-800">
                  <td class="max-w-[240px] px-3 py-3">
                    <div class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ item.name }}</div>
                    <div v-if="item.secondary" class="truncate text-xs text-gray-500 dark:text-gray-400">{{ item.secondary }}</div>
                  </td>
                  <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">{{ formatInteger(item.requests) }}</td>
                  <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">{{ formatInteger(item.total_tokens) }}</td>
                  <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">{{ formatAmount(item.primary_amount) }}</td>
                  <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">{{ formatAmount(item.secondary_amount) }}</td>
                  <td class="px-3 py-3 text-right text-sm font-medium text-gray-900 dark:text-white">{{ formatAmount(item.tertiary_amount) }}</td>
                  <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">
                    {{ item.quaternary_type === 'percent' ? formatPercent(item.quaternary_amount) : formatAmount(item.quaternary_amount) }}
                  </td>
                  <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">
                    {{ item.quinary_type === 'percent' ? formatPercent(item.quinary_amount) : formatAmount(item.quinary_amount) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-else class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.revenue.noData') }}
          </div>
        </section>
      </template>

      <SharePolicyPanel v-else-if="activeRevenueTab === 'sharePolicy'" />
      <ShareSettlementsPanel v-else-if="activeRevenueTab === 'shareSettlements'" />
    </UiPage>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip
} from 'chart.js'
import type { ChartData, ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { UiIconButton, UiPage } from '@/ui'
import SharePolicyPanel from '@/components/admin/revenue/SharePolicyPanel.vue'
import ShareSettlementsPanel from '@/components/admin/revenue/ShareSettlementsPanel.vue'
import { revenueAPI } from '@/api/admin/revenue'
import type {
  RevenueBreakdownItem,
  RevenueGranularity,
  RevenueShareOwnerBreakdownItem,
  RevenueSummary
} from '@/api/admin/revenue'
import { adminUsageAPI, type SimpleUser } from '@/api/admin/usage'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

type RangeDays = 1 | 3 | 7 | 30 | 90
type BreakdownKey = 'consumers' | 'shareOwners' | 'groups' | 'accounts' | 'models'
type RevenueTab = 'overview' | 'sharePolicy' | 'shareSettlements'
type BreakdownValueType = 'amount' | 'percent'

interface RevenueBreakdownDisplayItem {
  id?: number
  name: string
  secondary?: string
  requests: number
  total_tokens: number
  primary_amount: number
  secondary_amount: number
  tertiary_amount: number
  quaternary_amount: number
  quaternary_type: BreakdownValueType
  quinary_amount: number
  quinary_type: BreakdownValueType
}

const DAYS_OPTIONS: RangeDays[] = [1, 3, 7, 30, 90]
const MAX_REVENUE_RANGE_DAYS = 366
const MAX_HOURLY_REVENUE_RANGE_DAYS = 3

const { t, locale } = useI18n()
const appStore = useAppStore()

const rangeDays = ref<RangeDays>(1)
const initialRange = getDateRange(rangeDays.value)
const startDate = ref(initialRange.start)
const endDate = ref(initialRange.end)
const selectedRangeDays = ref<RangeDays | null>(rangeDays.value)
const selectedUserId = ref<number | null>(null)
const userSearchRef = ref<HTMLElement | null>(null)
const userKeyword = ref('')
const userResults = ref<SimpleUser[]>([])
const showUserDropdown = ref(false)
const granularity = ref<RevenueGranularity>('day')
const loading = ref(false)
const summary = ref<RevenueSummary | null>(null)
const activeBreakdown = ref<BreakdownKey>('shareOwners')
const activeRevenueTab = ref<RevenueTab>('overview')
let requestSeq = 0
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null

const amountFormatter = computed(() => new Intl.NumberFormat(locale.value, {
  minimumFractionDigits: 2,
  maximumFractionDigits: 6
}))

const integerFormatter = computed(() => new Intl.NumberFormat(locale.value, {
  maximumFractionDigits: 0
}))

const percentFormatter = computed(() => new Intl.NumberFormat(locale.value, {
  style: 'percent',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2
}))

const granularityOptions = computed(() => [
  { value: 'day' as const, label: t('admin.revenue.controls.day') },
  { value: 'hour' as const, label: t('admin.revenue.controls.hour') }
])

const revenueTabs = computed(() => [
  { key: 'overview' as const, label: t('admin.revenue.tabs.overview') },
  { key: 'sharePolicy' as const, label: t('admin.revenue.tabs.sharePolicy') },
  { key: 'shareSettlements' as const, label: t('admin.revenue.tabs.shareSettlements') }
])

const statCards = computed(() => {
  const data = summary.value
  if (!data) return []

  return [
    {
      key: 'net-cash',
      label: t('admin.revenue.cards.netCash'),
      value: formatAmount(data.cash.net_paid_amount),
      meta: t('admin.revenue.cards.netCashMeta', {
        paid: formatAmount(data.cash.paid_amount),
        redeem: formatAmount(data.cash.redeem_balance_amount || 0),
        refunds: formatAmount(data.cash.refund_amount)
      }),
      borderClass: 'border-emerald-500',
      dotClass: 'bg-emerald-500'
    },
    {
      key: 'consumed',
      label: t('admin.revenue.cards.consumedRevenue'),
      value: formatAmount(data.usage.consumed_revenue),
      meta: t('admin.revenue.cards.consumedRevenueMeta', {
        requests: formatInteger(data.usage.requests),
        balance: formatAmount(data.usage.balance_consumed_amount || 0),
        points: formatAmount(data.usage.points_consumed_amount || 0)
      }),
      borderClass: 'border-sky-500',
      dotClass: 'bg-sky-500'
    },
    {
      key: 'account-cost',
      label: t('admin.revenue.cards.accountCost'),
      value: formatAmount(data.usage.account_cost),
      meta: t('admin.revenue.cards.accountCostMeta', {
        tokens: formatInteger(data.usage.total_tokens)
      }),
      borderClass: 'border-amber-500',
      dotClass: 'bg-amber-500'
    },
    {
      key: 'gross-profit',
      label: t('admin.revenue.cards.grossProfit'),
      value: formatAmount(data.profit.usage_gross_profit),
      meta: t('admin.revenue.cards.grossProfitMeta', {
        margin: formatPercent(data.profit.usage_gross_margin)
      }),
      borderClass: 'border-indigo-500',
      dotClass: 'bg-indigo-500'
    },
    {
      key: 'estimated-net',
      label: t('admin.revenue.cards.estimatedNetProfit'),
      value: formatAmount(data.profit.estimated_net_profit),
      meta: t('admin.revenue.cards.estimatedNetProfitMeta', {
        margin: formatPercent(data.profit.estimated_net_margin)
      }),
      borderClass: 'border-teal-500',
      dotClass: 'bg-teal-500'
    },
    {
      key: 'pending',
      label: t('admin.revenue.cards.pendingAmount'),
      value: formatAmount(data.cash.pending_amount),
      meta: t('admin.revenue.cards.pendingAmountMeta', {
        count: formatInteger(data.cash.pending_order_count)
      }),
      borderClass: 'border-rose-500',
      dotClass: 'bg-rose-500'
    }
  ]
})

const chartData = computed<ChartData<'line'>>(() => {
  const trend = summary.value?.trend ?? []
  return {
    labels: trend.map(point => point.date),
    datasets: [
      {
        label: t('admin.revenue.chart.paid'),
        data: trend.map(point => point.net_paid_amount),
        borderColor: '#059669',
        backgroundColor: 'rgba(5, 150, 105, 0.08)',
        pointRadius: 2,
        tension: 0.3,
        fill: false
      },
      {
        label: t('admin.revenue.chart.consumed'),
        data: trend.map(point => point.consumed_revenue),
        borderColor: '#0284c7',
        backgroundColor: 'rgba(2, 132, 199, 0.08)',
        pointRadius: 2,
        tension: 0.3,
        fill: false
      },
      {
        label: t('admin.revenue.chart.balanceConsumed'),
        data: trend.map(point => point.balance_consumed_amount || 0),
        borderColor: '#7c3aed',
        backgroundColor: 'rgba(124, 58, 237, 0.08)',
        pointRadius: 2,
        tension: 0.3,
        fill: false
      },
      {
        label: t('admin.revenue.chart.pointsConsumed'),
        data: trend.map(point => point.points_consumed_amount || 0),
        borderColor: '#dc2626',
        backgroundColor: 'rgba(220, 38, 38, 0.08)',
        pointRadius: 2,
        tension: 0.3,
        fill: false
      },
      {
        label: t('admin.revenue.chart.cost'),
        data: trend.map(point => point.account_cost),
        borderColor: '#d97706',
        backgroundColor: 'rgba(217, 119, 6, 0.08)',
        pointRadius: 2,
        tension: 0.3,
        fill: false
      },
      {
        label: t('admin.revenue.chart.netProfit'),
        data: trend.map(point => point.estimated_net_profit),
        borderColor: '#0f766e',
        backgroundColor: 'rgba(15, 118, 110, 0.08)',
        pointRadius: 2,
        tension: 0.3,
        fill: true
      }
    ]
  }
})

const chartOptions = computed<ChartOptions<'line'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index'
  },
  plugins: {
    legend: {
      position: 'bottom',
      labels: {
        boxWidth: 12,
        usePointStyle: true
      }
    },
    tooltip: {
      callbacks: {
        label: context => `${context.dataset.label}: ${formatAmount(Number(context.parsed.y || 0))}`
      }
    }
  },
  scales: {
    x: {
      grid: {
        display: false
      },
      ticks: {
        maxRotation: 0,
        autoSkip: true,
        maxTicksLimit: 10
      }
    },
    y: {
      beginAtZero: true,
      ticks: {
        callback: value => formatAmount(Number(value))
      }
    }
  }
}))

const cashRows = computed(() => {
  const data = summary.value?.cash
  if (!data) return []
  return [
    { key: 'paid', label: t('admin.revenue.fields.paidAmount'), value: formatAmount(data.paid_amount) },
    { key: 'balance', label: t('admin.revenue.fields.balancePaid'), value: formatAmount(data.balance_paid_amount) },
    { key: 'redeem-balance', label: t('admin.revenue.fields.redeemBalancePaid'), value: formatAmount(data.redeem_balance_amount || 0) },
    { key: 'subscription', label: t('admin.revenue.fields.subscriptionPaid'), value: formatAmount(data.subscription_paid_amount) },
    { key: 'refund', label: t('admin.revenue.fields.refundAmount'), value: formatAmount(data.refund_amount) },
    { key: 'pending', label: t('admin.revenue.fields.pendingAmount'), value: formatAmount(data.pending_amount) },
    { key: 'paid-count', label: t('admin.revenue.fields.paidOrders'), value: formatInteger(data.paid_order_count) },
    { key: 'redeem-count', label: t('admin.revenue.fields.redeemBalanceCount'), value: formatInteger(data.redeem_balance_count || 0) },
    { key: 'refund-count', label: t('admin.revenue.fields.refundOrders'), value: formatInteger(data.refund_order_count) },
    { key: 'pending-count', label: t('admin.revenue.fields.pendingOrders'), value: formatInteger(data.pending_order_count) }
  ]
})

const usageRows = computed(() => {
  const data = summary.value?.usage
  if (!data) return []
  return [
    { key: 'consumed', label: t('admin.revenue.fields.consumedRevenue'), value: formatAmount(data.consumed_revenue) },
    { key: 'balance-consumed', label: t('admin.revenue.fields.balanceConsumed'), value: formatAmount(data.balance_consumed_amount || 0) },
    { key: 'points-consumed', label: t('admin.revenue.fields.pointsConsumed'), value: formatAmount(data.points_consumed_amount || 0) },
    { key: 'points-issued', label: t('admin.revenue.fields.pointsIssuedCost'), value: formatAmount(data.points_issued_amount || 0) },
    { key: 'standard-cost', label: t('admin.revenue.fields.standardCost'), value: formatAmount(data.standard_cost) },
    { key: 'account-cost', label: t('admin.revenue.fields.accountCost'), value: formatAmount(data.account_cost) },
    { key: 'requests', label: t('admin.revenue.fields.requests'), value: formatInteger(data.requests) },
    { key: 'tokens', label: t('admin.revenue.fields.tokens'), value: formatInteger(data.total_tokens) }
  ]
})

const adjustmentRows = computed(() => {
  const data = summary.value?.adjustments
  if (!data) return []
  return [
    { key: 'affiliate-rebate', label: t('admin.revenue.fields.affiliateRebate'), value: formatAmount(data.affiliate_rebate) },
    { key: 'affiliate-transfer', label: t('admin.revenue.fields.affiliateTransfer'), value: formatAmount(data.affiliate_transfer) },
    { key: 'affiliate-count', label: t('admin.revenue.fields.affiliateRebateCount'), value: formatInteger(data.affiliate_rebate_count) },
    { key: 'private-group-commission', label: t('admin.revenue.fields.privateGroupCommission'), value: formatAmount(data.private_group_commission) },
    { key: 'share-consumer', label: t('admin.revenue.fields.shareConsumerCharge'), value: formatAmount(data.share_consumer_charge) },
    { key: 'share-account', label: t('admin.revenue.fields.shareAccountCost'), value: formatAmount(data.share_account_cost) },
    { key: 'share-owner', label: t('admin.revenue.fields.shareOwnerCredit'), value: formatAmount(data.share_owner_credit) },
    { key: 'share-platform', label: t('admin.revenue.fields.sharePlatformFee'), value: formatAmount(data.share_platform_fee) },
    { key: 'share-net', label: t('admin.revenue.fields.shareNetProfit'), value: formatAmount(data.share_net_profit) },
    { key: 'share-count', label: t('admin.revenue.fields.shareSettlementCount'), value: formatInteger(data.share_settlement_count) }
  ]
})

const breakdownTabs = computed(() => [
  { key: 'consumers' as const, label: t('admin.revenue.breakdown.consumers') },
  { key: 'shareOwners' as const, label: t('admin.revenue.breakdown.shareOwners') },
  { key: 'groups' as const, label: t('admin.revenue.breakdown.groups') },
  { key: 'accounts' as const, label: t('admin.revenue.breakdown.accounts') },
  { key: 'models' as const, label: t('admin.revenue.breakdown.models') }
])

const activeBreakdownHint = computed(() => {
  switch (activeBreakdown.value) {
    case 'shareOwners':
      return t('admin.revenue.breakdownHints.shareOwners')
    case 'groups':
      return t('admin.revenue.breakdownHints.groups')
    case 'accounts':
      return t('admin.revenue.breakdownHints.accounts')
    case 'models':
      return t('admin.revenue.breakdownHints.models')
    case 'consumers':
    default:
      return t('admin.revenue.breakdownHints.consumers')
  }
})

const breakdownColumnLabels = computed(() => {
  if (activeBreakdown.value === 'shareOwners') {
    return {
      primary: t('admin.revenue.table.consumerCharge'),
      secondary: t('admin.revenue.table.cost'),
      tertiary: t('admin.revenue.table.ownerCredit'),
      quaternary: t('admin.revenue.table.platformFee'),
      quinary: t('admin.revenue.table.shareRatio')
    }
  }
  return {
    primary: t('admin.revenue.table.platformRevenue'),
    secondary: t('admin.revenue.table.cost'),
    tertiary: t('admin.revenue.table.shareExpense'),
    quaternary: t('admin.revenue.table.platformNetProfit'),
    quinary: t('admin.revenue.table.platformNetMargin')
  }
})

const activeBreakdownItems = computed<RevenueBreakdownDisplayItem[]>(() => {
  const data = summary.value
  if (!data) return []
  switch (activeBreakdown.value) {
    case 'shareOwners':
      return data.top_share_owners.map(mapShareOwnerBreakdownItem)
    case 'groups':
      return data.top_groups.map(mapUsageBreakdownItem)
    case 'accounts':
      return data.top_accounts.map(mapUsageBreakdownItem)
    case 'models':
      return data.top_models.map(mapUsageBreakdownItem)
    case 'consumers':
    default:
      return data.top_users.map(mapUsageBreakdownItem)
  }
})

function mapUsageBreakdownItem(item: RevenueBreakdownItem): RevenueBreakdownDisplayItem {
  return {
    id: item.id,
    name: item.name,
    secondary: item.secondary,
    requests: item.requests,
    total_tokens: item.total_tokens,
    primary_amount: item.consumed_revenue,
    secondary_amount: item.account_cost,
    tertiary_amount: item.share_owner_credit,
    quaternary_amount: item.net_profit,
    quaternary_type: 'amount',
    quinary_amount: item.net_margin,
    quinary_type: 'percent'
  }
}

function mapShareOwnerBreakdownItem(item: RevenueShareOwnerBreakdownItem): RevenueBreakdownDisplayItem {
  return {
    id: item.id,
    name: item.name,
    secondary: item.secondary,
    requests: item.requests,
    total_tokens: item.total_tokens,
    primary_amount: item.consumer_charge,
    secondary_amount: item.account_cost,
    tertiary_amount: item.owner_credit,
    quaternary_amount: item.platform_fee,
    quaternary_type: 'amount',
    quinary_amount: item.owner_share_ratio,
    quinary_type: 'percent'
  }
}

function isRangeDisabled(days: RangeDays): boolean {
  return granularity.value === 'hour' && days > MAX_HOURLY_REVENUE_RANGE_DAYS
}

function setRangeDays(days: RangeDays) {
  if (isRangeDisabled(days) || (rangeDays.value === days && selectedRangeDays.value === days)) return
  rangeDays.value = days
  selectedRangeDays.value = days
  const range = getDateRange(days)
  startDate.value = range.start
  endDate.value = range.end
  void loadSummary()
}

function setGranularity(value: RevenueGranularity) {
  if (granularity.value === value) return
  granularity.value = value
  if (value === 'hour' && getInclusiveDateSpanDays(startDate.value, endDate.value) > MAX_HOURLY_REVENUE_RANGE_DAYS) {
    rangeDays.value = 1
    selectedRangeDays.value = 1
    const range = getDateRange(1)
    startDate.value = range.start
    endDate.value = range.end
  }
  void loadSummary()
}

async function loadSummary() {
  if (!validateDateRange()) return

  const currentRequest = ++requestSeq
  loading.value = true
  try {
    const res = await revenueAPI.getSummary({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      top_limit: 10,
      user_id: selectedUserId.value ?? undefined
    })
    if (currentRequest === requestSeq) {
      summary.value = res.data
    }
  } catch (err: unknown) {
    if (currentRequest === requestSeq) {
      appStore.showError(extractI18nErrorMessage(err, t, 'admin.revenue.errors', t('admin.revenue.loadFailed')))
    }
  } finally {
    if (currentRequest === requestSeq) {
      loading.value = false
    }
  }
}

function applyCustomDateRange() {
  selectedRangeDays.value = null
  void loadSummary()
}

function validateDateRange(): boolean {
  if (!startDate.value || !endDate.value) {
    appStore.showError(t('common.unknownError'))
    return false
  }
  if (startDate.value > endDate.value) {
    appStore.showError(t('common.unknownError'))
    return false
  }
  if (getInclusiveDateSpanDays(startDate.value, endDate.value) > MAX_REVENUE_RANGE_DAYS) {
    appStore.showError(t('admin.revenue.errors.REVENUE_TIME_RANGE_TOO_LARGE'))
    return false
  }
  if (granularity.value === 'hour' && getInclusiveDateSpanDays(startDate.value, endDate.value) > MAX_HOURLY_REVENUE_RANGE_DAYS) {
    appStore.showError(t('admin.revenue.errors.REVENUE_HOUR_RANGE_TOO_LARGE'))
    return false
  }
  return true
}

function debounceUserSearch() {
  selectedUserId.value = null
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }

  userSearchTimeout = setTimeout(async () => {
    const keyword = userKeyword.value.trim()
    if (!keyword) {
      userResults.value = []
      showUserDropdown.value = false
      return
    }

    try {
      userResults.value = await adminUsageAPI.searchUsers(keyword)
      showUserDropdown.value = true
    } catch (err: unknown) {
      userResults.value = []
      appStore.showError(extractI18nErrorMessage(err, t, 'admin.revenue.errors', t('common.unknownError')))
    }
  }, 300)
}

function selectUser(user: SimpleUser) {
  selectedUserId.value = user.id
  userKeyword.value = user.email
  userResults.value = []
  showUserDropdown.value = false
  void loadSummary()
}

function clearUser() {
  selectedUserId.value = null
  userKeyword.value = ''
  userResults.value = []
  showUserDropdown.value = false
  void loadSummary()
}

function onDocumentClick(event: MouseEvent) {
  const target = event.target as Node | null
  if (target && !(userSearchRef.value?.contains(target) ?? false)) {
    showUserDropdown.value = false
  }
}

function getDateRange(days: RangeDays): { start: string; end: string } {
  const end = new Date()
  const start = new Date()
  start.setDate(end.getDate() - days + 1)
  return {
    start: formatDateParam(start),
    end: formatDateParam(end)
  }
}

function formatDateParam(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function getInclusiveDateSpanDays(start: string, end: string): number {
  const startTime = parseDateParam(start).getTime()
  const endTime = parseDateParam(end).getTime()
  return Math.floor((endTime - startTime) / 86_400_000) + 1
}

function parseDateParam(value: string): Date {
  const [year, month, day] = value.split('-').map(Number)
  return new Date(year, month - 1, day)
}

function formatAmount(value: number): string {
  return amountFormatter.value.format(Number.isFinite(value) ? value : 0)
}

function formatInteger(value: number): string {
  return integerFormatter.value.format(Number.isFinite(value) ? value : 0)
}

function formatPercent(value: number): string {
  return percentFormatter.value.format(Number.isFinite(value) ? value : 0)
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick)
  void loadSummary()
})

onUnmounted(() => {
  document.removeEventListener('click', onDocumentClick)
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }
})
</script>

<style scoped>
.revenue-tabs {
  display: flex;
  max-width: 100%;
  gap: 1.25rem;
  overflow-x: auto;
  border-bottom: 1px solid var(--ui-border);
}

.revenue-tab {
  margin-bottom: -1px;
  padding: 0.75rem 0;
  border-bottom: 2px solid transparent;
  color: var(--ui-text-tertiary);
  font-size: 0.875rem;
  font-weight: 500;
  white-space: nowrap;
}

.revenue-tab:hover,
.revenue-tab--active {
  color: var(--ui-text);
}

.revenue-tab--active {
  border-bottom-color: var(--ui-text);
}

.revenue-segment {
  display: inline-flex;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  padding: 0.1875rem;
  background: transparent;
}

.revenue-segment-button {
  min-width: 4rem;
  border-radius: var(--ui-radius-md);
  padding: 0.375rem 0.75rem;
  color: var(--ui-text-secondary);
  font-size: 0.875rem;
  font-weight: 500;
  transition: background-color 150ms ease, color 150ms ease;
}

.revenue-segment-button:hover {
  background: var(--ui-surface-hover);
  color: var(--ui-text);
}

.revenue-segment-button--active {
  background: var(--ui-brand);
  color: var(--ui-brand-contrast);
}

.revenue-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1.5rem;
  padding-bottom: 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.revenue-metric {
  min-width: 0;
  min-height: 6.5rem;
}

.revenue-section {
  min-width: 0;
  padding: 1.25rem 0;
  border-top: 1px solid var(--ui-border);
}

.revenue-section :deep(th) {
  letter-spacing: 0;
  text-transform: none;
}

@media (max-width: 900px) {
  .revenue-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 1rem;
  }
}

@media (max-width: 520px) {
  .revenue-metrics {
    grid-template-columns: 1fr;
  }
}
</style>
