<template>
  <UiMetricStrip class="payment-summary" :style="{ '--metric-columns': 4 }">
    <UiMetric
      :label="t('payment.admin.todayRevenue')"
      :value="`$${formatMoney(stats.today_amount)}`"
      :detail="`${stats.today_count} ${t('payment.admin.entries')}`"
    />
    <UiMetric
      :label="t('payment.admin.totalRevenue')"
      :value="`$${formatMoney(stats.total_amount)}`"
      :detail="`${stats.total_count} ${t('payment.admin.entries')}`"
    />
    <UiMetric
      :label="t('payment.admin.todayOrders')"
      :value="stats.today_count"
    />
    <UiMetric
      :label="t('payment.admin.avgAmount')"
      :value="`$${formatMoney(stats.avg_amount)}`"
    />
  </UiMetricStrip>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { DashboardStats } from '@/types/payment'
import { UiMetric, UiMetricStrip } from '@/ui'

const { t } = useI18n()

defineProps<{
  stats: DashboardStats
}>()

function formatMoney(value: number): string {
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value)
}
</script>

<style scoped>
.payment-summary {
  padding-block: 0.25rem 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.payment-summary :deep(.ui-metric) {
  padding: 0;
}

.payment-summary :deep(.ui-metric + .ui-metric) {
  padding-left: clamp(1rem, 2.5vw, 2rem);
  border-left: 1px solid var(--ui-border);
}

@media (max-width: 900px) {
  .payment-summary :deep(.ui-metric:nth-child(3)) {
    padding-left: 0;
    border-left: 0;
  }
}

@media (max-width: 520px) {
  .payment-summary :deep(.ui-metric + .ui-metric) {
    padding-left: 0.875rem;
  }

  .payment-summary :deep(.ui-metric:nth-child(3)) {
    padding-left: 0;
  }
}
</style>
