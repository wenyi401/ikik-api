<template>
  <section class="affiliate-summary">
    <div class="affiliate-period-bar">
      <div class="affiliate-period-presets">
        <button
          v-for="preset in presets"
          :key="preset"
          type="button"
          class="affiliate-period-button"
          :class="{ 'affiliate-period-button--active': periodPreset === preset }"
          @click="emit('select-period', preset)"
        >
          {{ t(`affiliate.period.presets.${preset}`) }}
        </button>
      </div>
      <div class="affiliate-date-range">
        <input
          :value="startDate"
          type="date"
          class="input affiliate-date-input"
          :aria-label="t('affiliate.period.start')"
          @change="emitStartDate"
        />
        <span class="text-[var(--ui-text-tertiary)]">-</span>
        <input
          :value="endDate"
          type="date"
          class="input affiliate-date-input"
          :aria-label="t('affiliate.period.end')"
          @change="emitEndDate"
        />
      </div>
    </div>

    <UiMetricStrip :style="{ '--metric-columns': 4 }">
      <UiMetric
        :label="t('affiliate.stats.rebateRate')"
        :value="`${rebateRate}%`"
      />
      <UiMetric :label="t('affiliate.stats.invitedUsers')" :value="inviteeCount.toLocaleString()" />
      <UiMetric :label="periodIncomeTitle" :value="formatCurrency(periodRebate)" />
      <UiMetric :label="t('affiliate.stats.totalQuota')" :value="formatCurrency(totalQuota)" />
    </UiMetricStrip>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { UiMetric, UiMetricStrip } from '@/ui'
import { formatCurrency } from '@/utils/format'

defineProps<{
  periodPreset: string
  startDate: string
  endDate: string
  rebateRate: string
  inviteeCount: number
  periodIncomeTitle: string
  periodRebate: number
  totalQuota: number
}>()

const emit = defineEmits<{
  'select-period': [preset: 'today' | 'yesterday' | 'last7']
  'update:start-date': [value: string]
  'update:end-date': [value: string]
}>()

const { t } = useI18n()
const presets = ['today', 'yesterday', 'last7'] as const

function emitStartDate(event: Event): void {
  emit('update:start-date', (event.target as HTMLInputElement).value)
}

function emitEndDate(event: Event): void {
  emit('update:end-date', (event.target as HTMLInputElement).value)
}
</script>

<style scoped>
.affiliate-summary {
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.affiliate-period-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.875rem 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.affiliate-period-presets {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.affiliate-period-button {
  min-height: 2rem;
  padding: 0.35rem 0.7rem;
  border-radius: var(--ui-radius-md);
  color: var(--ui-text-secondary);
  font-size: 0.8125rem;
  font-weight: 500;
}

.affiliate-period-button:hover {
  background: var(--ui-surface-subtle);
  color: var(--ui-text);
}

.affiliate-period-button--active,
.affiliate-period-button--active:hover {
  background: var(--ui-text);
  color: var(--ui-surface);
}

.affiliate-date-range {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.affiliate-date-input {
  width: 9.25rem;
  height: 2rem;
  font-size: 0.75rem;
}

.affiliate-summary :deep(.ui-metric + .ui-metric) {
  border-left: 1px solid var(--ui-border);
}

@media (max-width: 900px) {
  .affiliate-summary :deep(.ui-metric:nth-child(3)) {
    border-left: 0;
  }
}

@media (max-width: 640px) {
  .affiliate-period-bar {
    align-items: stretch;
    flex-direction: column;
    gap: 0.75rem;
    padding: 0.75rem;
  }

  .affiliate-period-presets {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
  }

  .affiliate-period-button {
    width: 100%;
  }

  .affiliate-date-range {
    width: 100%;
  }

  .affiliate-date-input {
    min-width: 0;
    width: 100%;
  }

  .affiliate-summary :deep(.ui-metric + .ui-metric) {
    border-left: 0;
  }

  .affiliate-summary :deep(.ui-metric:nth-child(even)) {
    border-left: 1px solid var(--ui-border);
  }

  .affiliate-summary :deep(.ui-metric:nth-child(n + 3)) {
    border-top: 1px solid var(--ui-border);
  }
}
</style>
