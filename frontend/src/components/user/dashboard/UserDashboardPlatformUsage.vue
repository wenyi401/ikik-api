<template>
  <UiSection
    class="dashboard-section"
    surface="panel"
    :title="t('dashboard.todayPlatformUsage')"
  >
    <template #actions>
      <div class="platform-summary">
        <span>{{ formatNumber(stats?.today_requests || 0) }} {{ t('dashboard.requests') }}</span>
        <span>{{ formatTokens(stats?.today_tokens || 0) }} Token</span>
        <span>${{ formatCost(stats?.today_actual_cost || 0) }}</span>
      </div>
    </template>

    <div v-if="platformRows.length > 0" class="platform-list">
      <div v-for="row in platformRows" :key="row.platform" class="platform-row">
        <div class="platform-name">
          <span :class="['h-2 w-2 shrink-0 rounded-full', platformDotClass(row.platform)]" />
          <div class="min-w-0">
            <p>{{ platformLabel(row.platform) }}</p>
            <span>{{ formatNumber(row.requests) }} {{ t('dashboard.requests') }}</span>
          </div>
        </div>

        <div class="min-w-0">
          <div class="platform-progress-label">
            <span>{{ formatTokens(row.total_tokens) }}</span>
            <span>{{ formatPercent(platformShare(row.total_tokens)) }}</span>
          </div>
          <div class="platform-progress">
            <div
              :class="platformAccentBarClass(row.platform)"
              :style="{ width: `${platformBarWidth(row.total_tokens)}%` }"
            />
          </div>
        </div>

        <div class="platform-breakdown">
          <span>{{ t('dashboard.input') }} <strong>{{ formatTokens(row.input_tokens) }}</strong></span>
          <span>{{ t('dashboard.output') }} <strong>{{ formatTokens(row.output_tokens) }}</strong></span>
          <span>{{ t('dashboard.actual') }} <strong>${{ formatCost(row.actual_cost) }}</strong></span>
        </div>
      </div>
    </div>

    <div v-else class="platform-empty">
      {{ t('dashboard.noDataAvailable') }}
    </div>
  </UiSection>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiSection } from '@/ui'
import { platformAccentBarClass, platformLabel } from '@/utils/platformColors'
import type { UserDashboardPlatformUsage, UserDashboardStats as UserStatsType } from '@/api/usage'

const props = defineProps<{ stats: UserStatsType }>()
const { t } = useI18n()

const platformRows = computed<UserDashboardPlatformUsage[]>(() =>
  [...(props.stats?.today_platforms || [])].sort((a, b) => (b.total_tokens || 0) - (a.total_tokens || 0))
)
const platformTokenBase = computed(() => {
  const totalFromRows = platformRows.value.reduce((sum, row) => sum + (row.total_tokens || 0), 0)
  return props.stats?.today_tokens || totalFromRows || 0
})

const formatNumber = (value: number) => value.toLocaleString()
const formatCost = (value: number) => value.toFixed(4)
const formatTokens = (value: number) => {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return value.toString()
}
const formatPercent = (value: number) => `${value.toFixed(value >= 10 ? 0 : 1)}%`
const platformShare = (tokens: number) => {
  if (!platformTokenBase.value || tokens <= 0) return 0
  return Math.min(100, (tokens / platformTokenBase.value) * 100)
}
const platformBarWidth = (tokens: number) => {
  const share = platformShare(tokens)
  return share <= 0 ? 0 : Math.max(3, share)
}
const platformDotClass = (platform: string) => {
  switch (platform) {
    case 'anthropic': return 'bg-orange-500'
    case 'openai': return 'bg-emerald-500'
    case 'antigravity': return 'bg-purple-500'
    case 'gemini': return 'bg-blue-500'
    case 'grok': return 'bg-slate-600 dark:bg-slate-300'
    case 'custom': return 'bg-stone-500'
    default: return 'bg-primary-500'
  }
}
</script>

<style scoped>
.platform-summary {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 1rem;
  color: var(--ui-text-secondary);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
}

.platform-list {
  display: grid;
  gap: 0.25rem;
}

.platform-row {
  display: grid;
  grid-template-columns: minmax(9rem, 0.8fr) minmax(12rem, 1.2fr) minmax(15rem, 1fr);
  align-items: center;
  gap: 1.25rem;
  padding: 0.75rem 0;
}

.platform-name {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.625rem;
}

.platform-name p {
  overflow: hidden;
  color: var(--ui-text);
  font-size: 0.875rem;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.platform-name span,
.platform-progress-label,
.platform-breakdown {
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

.platform-progress-label {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.375rem;
  font-variant-numeric: tabular-nums;
}

.platform-progress {
  height: 0.3rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--ui-surface-hover);
}

.platform-progress > div {
  height: 100%;
  border-radius: inherit;
}

.platform-breakdown {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
  text-align: right;
}

.platform-breakdown span {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.platform-breakdown strong {
  color: var(--ui-text-secondary);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.platform-empty {
  padding: 2rem 0;
  color: var(--ui-text-tertiary);
  text-align: center;
  font-size: 0.875rem;
}

@media (max-width: 760px) {
  .platform-summary {
    justify-content: flex-start;
  }

  .platform-row {
    grid-template-columns: 1fr;
    gap: 0.75rem;
  }

  .platform-breakdown {
    text-align: left;
  }
}
</style>
