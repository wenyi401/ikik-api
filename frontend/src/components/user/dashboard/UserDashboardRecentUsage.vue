<template>
  <UiSection class="dashboard-section" surface="panel" :title="t('dashboard.recentUsage')">
    <template #actions>
      <span class="text-xs text-[var(--app-muted)]">{{ t('dashboard.last7Days') }}</span>
    </template>
    <div>
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner size="lg" />
      </div>
      <div v-else-if="data.length === 0" class="py-8">
        <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
      </div>
      <div v-else>
        <div v-for="log in data" :key="log.id" class="recent-usage-row">
          <div class="min-w-0">
            <p class="truncate text-sm font-medium text-[var(--app-text)]">{{ log.model }}</p>
            <p class="text-xs text-[var(--app-muted)]">{{ formatDateTime(log.created_at) }}</p>
          </div>
          <div class="text-right">
            <p class="text-sm font-medium text-[var(--app-text)]">
              <span :title="t('dashboard.actual')">${{ formatCost(log.actual_cost) }}</span>
              <span class="font-normal text-[var(--app-muted)]" :title="t('dashboard.standard')"> / ${{ formatCost(log.total_cost) }}</span>
            </p>
            <p class="text-xs text-[var(--app-muted)]">{{ (log.input_tokens + log.output_tokens).toLocaleString() }} tokens</p>
          </div>
        </div>

        <router-link to="/usage" class="mt-3 inline-flex py-2 text-sm font-medium text-[var(--app-text)] hover:underline">
          {{ t('dashboard.viewAllUsage') }}
        </router-link>
      </div>
    </div>
  </UiSection>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { UiSection } from '@/ui'
import { formatDateTime } from '@/utils/format'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()
const { t } = useI18n()
const formatCost = (c: number) => c.toFixed(4)
</script>

<style scoped>
.recent-usage-row {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--ui-border);
}

.recent-usage-row:last-of-type {
  border-bottom: 0;
}
</style>
