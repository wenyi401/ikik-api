<template>
  <AppLayout>
    <UiPage width="wide">
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-[var(--ui-border-strong)] border-t-[var(--ui-text)]"></div>
      </div>

      <template v-else-if="detail">
        <AffiliateSummaryPanel
          :period-preset="periodPreset"
          :start-date="periodStartDate"
          :end-date="periodEndDate"
          :rebate-rate="formattedRebateRate"
          :invitee-count="detail.aff_count"
          :period-income-title="periodIncomeTitle"
          :period-rebate="detail.period_rebate"
          :total-quota="detail.aff_history_quota"
          @select-period="setPeriodPreset"
          @update:start-date="updatePeriodDate('start', $event)"
          @update:end-date="updatePeriodDate('end', $event)"
        />

        <AffiliateInvitePanel
          :code="detail.aff_code"
          :invite-link="inviteLink"
          :rebate-rate="formattedRebateRate"
          @copy-code="copyCode"
          @copy-link="copyInviteLink"
        />

        <AffiliateInviteesPanel
          :items="sortedInvitees"
          :sort-key="sortKey"
          :sort-direction="sortDirection"
          @sort="toggleSort"
        />
      </template>
    </UiPage>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import AffiliateInvitePanel from '@/components/user/affiliate/AffiliateInvitePanel.vue'
import AffiliateInviteesPanel from '@/components/user/affiliate/AffiliateInviteesPanel.vue'
import AffiliateSummaryPanel from '@/components/user/affiliate/AffiliateSummaryPanel.vue'
import userAPI from '@/api/user'
import type { AffiliateInvitee, UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { UiPage } from '@/ui'
import { extractApiErrorMessage } from '@/utils/apiError'

type PeriodPreset = 'today' | 'yesterday' | 'last7' | 'custom'
type SortKey = 'bound_at' | 'period_consumption' | 'period_rebate' | 'history_consumption' | 'total_rebate'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const detail = ref<UserAffiliateDetail | null>(null)
const periodPreset = ref<PeriodPreset>('today')
const periodStartDate = ref(toDateInputValue(startOfLocalDay(new Date())))
const periodEndDate = ref(toDateInputValue(startOfLocalDay(new Date())))
const sortKey = ref<SortKey>('bound_at')
const sortDirection = ref<'asc' | 'desc'>('desc')

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

const formattedRebateRate = computed(() => {
  const value = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(value * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

const periodIncomeTitle = computed(() => {
  if (periodPreset.value === 'today') return t('affiliate.stats.todayQuota')
  if (periodPreset.value === 'yesterday') return t('affiliate.stats.yesterdayQuota')
  if (periodPreset.value === 'last7') return t('affiliate.stats.last7Quota')
  return t('affiliate.stats.periodQuota')
})

const sortedInvitees = computed(() => {
  const rows = [...(detail.value?.invitees ?? [])]
  const direction = sortDirection.value === 'asc' ? 1 : -1
  return rows.sort((leftItem, rightItem) => {
    const left = sortableValue(leftItem, sortKey.value)
    const right = sortableValue(rightItem, sortKey.value)
    if (left === right) return 0
    return left > right ? direction : -direction
  })
})

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) loading.value = true
  try {
    detail.value = await userAPI.getAffiliateDetail(buildPeriodParams())
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) loading.value = false
  }
}

function buildPeriodParams(): { period_start_at?: string; period_end_at?: string } {
  const start = parseDateInputStart(periodStartDate.value)
  const end = parseDateInputStart(periodEndDate.value)
  return {
    period_start_at: start?.toISOString(),
    period_end_at: end ? addDays(end, 1).toISOString() : undefined
  }
}

function setPeriodPreset(preset: 'today' | 'yesterday' | 'last7'): void {
  periodPreset.value = preset
  const today = startOfLocalDay(new Date())
  if (preset === 'today') {
    periodStartDate.value = toDateInputValue(today)
    periodEndDate.value = toDateInputValue(today)
  } else if (preset === 'yesterday') {
    periodStartDate.value = toDateInputValue(addDays(today, -1))
    periodEndDate.value = toDateInputValue(addDays(today, -1))
  } else {
    periodStartDate.value = toDateInputValue(addDays(today, -6))
    periodEndDate.value = toDateInputValue(today)
  }
  void loadAffiliateDetail(true)
}

function updatePeriodDate(target: 'start' | 'end', value: string): void {
  if (target === 'start') periodStartDate.value = value
  else periodEndDate.value = value
  periodPreset.value = 'custom'

  const start = parseDateInputStart(periodStartDate.value)
  const end = parseDateInputStart(periodEndDate.value)
  if (!start || !end || start > end) {
    appStore.showError(t('affiliate.period.invalid'))
    return
  }
  void loadAffiliateDetail(true)
}

function toggleSort(key: SortKey): void {
  if (sortKey.value === key) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDirection.value = 'desc'
}

function sortableValue(item: AffiliateInvitee, key: SortKey): number {
  if (key === 'bound_at') {
    const time = item.created_at ? new Date(item.created_at).getTime() : 0
    return Number.isFinite(time) ? time : 0
  }
  return item[key] ?? 0
}

function startOfLocalDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date)
  next.setDate(next.getDate() + days)
  return next
}

function toDateInputValue(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function parseDateInputStart(value: string): Date | null {
  if (!value) return null
  const [year, month, day] = value.split('-').map(Number)
  if (!year || !month || !day) return null
  const date = new Date(year, month - 1, day)
  return Number.isNaN(date.getTime()) ? null : date
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

onMounted(() => {
  void loadAffiliateDetail()
})
</script>
