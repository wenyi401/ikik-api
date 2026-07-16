<template>
  <UiMetricStrip class="usage-summary" :style="{ '--metric-columns': 4 }">
    <UiMetric
      :label="t('usage.totalRequests')"
      :value="normalizedStats.total_requests.toLocaleString()"
      :detail="t('usage.inSelectedRange')"
    />
    <UiMetric
      :label="t('usage.totalTokens')"
      :value="formatTokens(normalizedStats.total_tokens)"
      :detail="`${t('usage.in')} ${formatTokens(normalizedStats.total_input_tokens)} · ${t('usage.out')} ${formatTokens(normalizedStats.total_output_tokens)}`"
    />
    <UiMetric
      :label="t('usage.totalCost')"
      :value="`$${normalizedStats.total_actual_cost.toFixed(4)}`"
      :detail="`${t('usage.accountCost')} $${normalizedStats.total_account_cost.toFixed(4)} · ${t('usage.standardCost')} $${normalizedStats.total_cost.toFixed(4)}`"
    />
    <UiMetric
      :label="t('usage.avgDuration')"
      :value="formatDuration(normalizedStats.average_duration_ms)"
    />
  </UiMetricStrip>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import { UiMetric, UiMetricStrip } from '@/ui'

const props = defineProps<{ stats: AdminUsageStatsResponse | null }>()

const { t } = useI18n()

const toFiniteNumber = (value: unknown): number => {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

const normalizedStats = computed(() => {
  const raw = props.stats
  const totalInputTokens = toFiniteNumber(raw?.total_input_tokens)
  const totalOutputTokens = toFiniteNumber(raw?.total_output_tokens)
  const totalCacheTokens = toFiniteNumber(raw?.total_cache_tokens)
  return {
    total_requests: toFiniteNumber(raw?.total_requests),
    total_input_tokens: totalInputTokens,
    total_output_tokens: totalOutputTokens,
    total_cache_tokens: totalCacheTokens,
    total_tokens: toFiniteNumber(raw?.total_tokens) || totalInputTokens + totalOutputTokens + totalCacheTokens,
    total_cost: toFiniteNumber(raw?.total_cost),
    total_actual_cost: toFiniteNumber(raw?.total_actual_cost),
    total_account_cost: toFiniteNumber(raw?.total_account_cost),
    average_duration_ms: toFiniteNumber(raw?.average_duration_ms)
  }
})

const formatDuration = (ms: number) =>
  ms < 1000 ? `${ms.toFixed(0)}ms` : `${(ms / 1000).toFixed(2)}s`

const formatTokens = (value: number) => {
  if (value >= 1e9) return (value / 1e9).toFixed(2) + 'B'
  if (value >= 1e6) return (value / 1e6).toFixed(2) + 'M'
  if (value >= 1e3) return (value / 1e3).toFixed(2) + 'K'
  return value.toLocaleString()
}
</script>

<style scoped>
.usage-summary {
  padding-bottom: 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.usage-summary :deep(.ui-metric) {
  padding: 0;
}

.usage-summary :deep(.ui-metric + .ui-metric) {
  padding-left: clamp(1rem, 2.5vw, 2rem);
  border-left: 1px solid var(--ui-border);
}

@media (max-width: 900px) {
  .usage-summary :deep(.ui-metric:nth-child(3)) {
    padding-left: 0;
    border-left: 0;
  }
}

@media (max-width: 520px) {
  .usage-summary {
    padding-bottom: 1rem;
  }

  .usage-summary :deep(.ui-metric + .ui-metric) {
    padding-left: 0.875rem;
  }

  .usage-summary :deep(.ui-metric:nth-child(3)) {
    padding-left: 0;
  }
}
</style>
