<template>
  <UiMetricStrip class="dashboard-metric-strip" :style="{ '--metric-columns': isSimple ? 4 : 5 }">
    <UiMetric
      v-if="!isSimple"
      :label="t('dashboard.balance')"
      :value="`$${formatBalance(totalBalance)}`"
      :detail="t('common.available')"
    />
    <UiMetric
      :label="t('dashboard.todayRequests')"
      :value="formatNumber(stats?.today_requests || 0)"
      :detail="`${t('common.total')}: ${formatNumber(stats?.total_requests || 0)}`"
    />
    <UiMetric
      :label="t('dashboard.todayTokens')"
      :value="formatTokens(stats?.today_tokens || 0)"
      :detail="`${t('dashboard.input')} ${formatTokens(stats?.today_input_tokens || 0)} · ${t('dashboard.output')} ${formatTokens(stats?.today_output_tokens || 0)}`"
    />
    <UiMetric
      :label="t('dashboard.todayCost')"
      :value="`$${formatCost(stats?.today_actual_cost || 0)}`"
      :detail="`${t('common.total')}: $${formatCost(stats?.total_actual_cost || 0)}`"
    />
    <UiMetric
      :label="t('dashboard.avgResponse')"
      :value="formatDuration(stats?.average_duration_ms || 0)"
      :detail="`${formatTokens(stats?.rpm || 0)} RPM · ${formatTokens(stats?.tpm || 0)} TPM`"
    />
  </UiMetricStrip>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiMetric, UiMetricStrip } from '@/ui'
import type { UserDashboardStats as UserStatsType } from '@/api/usage'
import type { User } from '@/types'

const props = defineProps<{
  stats: UserStatsType
  user: User | null | undefined
  isSimple: boolean
}>()

const { t } = useI18n()
const totalBalance = computed(() => Number(props.user?.balance || 0))

const formatBalance = (value: number) => new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2
}).format(value)

const formatNumber = (value: number) => value.toLocaleString()
const formatCost = (value: number) => value.toFixed(4)
const formatTokens = (value: number) => {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return value.toString()
}
const formatDuration = (milliseconds: number) => (
  milliseconds >= 1000 ? `${(milliseconds / 1000).toFixed(2)}s` : `${milliseconds.toFixed(0)}ms`
)
</script>

<style scoped>
.dashboard-metric-strip :deep(.ui-metric) {
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.035);
}
</style>
