<template>
  <section class="affiliate-panel">
    <header class="affiliate-panel-header">
      <h2>{{ t('affiliate.invitees.title') }}</h2>
      <span>{{ items.length.toLocaleString() }}</span>
    </header>

    <div v-if="items.length === 0" class="affiliate-empty">
      {{ t('affiliate.invitees.empty') }}
    </div>

    <template v-else>
      <div class="affiliate-table-wrap">
        <table class="affiliate-table">
          <thead>
            <tr>
              <th>{{ t('affiliate.invitees.columns.user') }}</th>
              <th>{{ t('affiliate.invitees.columns.bindSource') }}</th>
              <th>
                <button type="button" @click="emit('sort', 'bound_at')">
                  {{ t('affiliate.invitees.columns.joinedAt') }}
                  <Icon :name="sortIcon('bound_at')" size="xs" />
                </button>
              </th>
              <th>{{ t('affiliate.invitees.columns.status') }}</th>
              <th class="text-right">
                <button type="button" @click="emit('sort', 'period_consumption')">
                  {{ t('affiliate.invitees.columns.periodConsumption') }}
                  <Icon :name="sortIcon('period_consumption')" size="xs" />
                </button>
              </th>
              <th class="text-right">
                <button type="button" @click="emit('sort', 'period_rebate')">
                  {{ t('affiliate.invitees.columns.periodRebate') }}
                  <Icon :name="sortIcon('period_rebate')" size="xs" />
                </button>
              </th>
              <th class="text-right">
                <button type="button" @click="emit('sort', 'total_rebate')">
                  {{ t('affiliate.invitees.columns.rebate') }}
                  <Icon :name="sortIcon('total_rebate')" size="xs" />
                </button>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in items" :key="item.user_id">
              <td>
                <div class="font-medium text-[var(--ui-text)]">{{ item.email || '-' }}</div>
                <div class="mt-0.5 text-xs text-[var(--ui-text-tertiary)]">{{ item.username || '-' }}</div>
              </td>
              <td>{{ formatBindSource(item.invite_bind_source) }}</td>
              <td>{{ formatDateTime(item.created_at) || '-' }}</td>
              <td>{{ formatInviteeStatus(item.status) }}</td>
              <td class="text-right">{{ formatCurrency(item.period_consumption) }}</td>
              <td class="text-right font-medium">{{ formatCurrency(item.period_rebate) }}</td>
              <td class="text-right font-medium text-[var(--ui-text)]">{{ formatCurrency(item.total_rebate) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="affiliate-mobile-list">
        <article v-for="item in items" :key="item.user_id" class="affiliate-mobile-item">
          <div class="affiliate-mobile-head">
            <div class="min-w-0">
              <p class="truncate font-medium text-[var(--ui-text)]">{{ item.email || '-' }}</p>
              <p class="truncate text-xs text-[var(--ui-text-tertiary)]">{{ item.username || '-' }}</p>
            </div>
            <span class="affiliate-status">{{ formatInviteeStatus(item.status) }}</span>
          </div>
          <dl class="affiliate-mobile-stats">
            <div>
              <dt>{{ t('affiliate.invitees.columns.periodConsumption') }}</dt>
              <dd>{{ formatCurrency(item.period_consumption) }}</dd>
            </div>
            <div>
              <dt>{{ t('affiliate.invitees.columns.periodRebate') }}</dt>
              <dd>{{ formatCurrency(item.period_rebate) }}</dd>
            </div>
            <div>
              <dt>{{ t('affiliate.invitees.columns.historyConsumption') }}</dt>
              <dd>{{ formatCurrency(item.history_consumption) }}</dd>
            </div>
            <div>
              <dt>{{ t('affiliate.invitees.columns.rebate') }}</dt>
              <dd>{{ formatCurrency(item.total_rebate) }}</dd>
            </div>
          </dl>
          <p class="affiliate-mobile-meta">
            {{ formatBindSource(item.invite_bind_source) }} · {{ formatDateTime(item.created_at) || '-' }}
          </p>
        </article>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { AffiliateInvitee } from '@/types'
import { formatCurrency, formatDateTime } from '@/utils/format'

type AffiliateSortKey =
  | 'bound_at'
  | 'period_consumption'
  | 'period_rebate'
  | 'history_consumption'
  | 'total_rebate'

const props = defineProps<{
  items: AffiliateInvitee[]
  sortKey: AffiliateSortKey
  sortDirection: 'asc' | 'desc'
}>()

const emit = defineEmits<{
  sort: [key: AffiliateSortKey]
}>()

const { t } = useI18n()

function sortIcon(key: AffiliateSortKey): 'arrowsUpDown' | 'arrowUp' | 'arrowDown' {
  if (props.sortKey !== key) return 'arrowsUpDown'
  return props.sortDirection === 'asc' ? 'arrowUp' : 'arrowDown'
}

function formatBindSource(source?: string): string {
  if (source === 'registration') return t('affiliate.invitees.bindSources.registration')
  if (source === 'admin') return t('affiliate.invitees.bindSources.admin')
  return t('affiliate.invitees.bindSources.legacy')
}

function formatInviteeStatus(status: string): string {
  if (status === 'active') return t('affiliate.invitees.status.active')
  if (status === 'disabled') return t('affiliate.invitees.status.disabled')
  return status || '-'
}
</script>

<style scoped>
.affiliate-panel {
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.affiliate-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.875rem 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.affiliate-panel-header h2 {
  color: var(--ui-text);
  font-size: 0.9375rem;
  font-weight: 600;
}

.affiliate-panel-header span {
  color: var(--ui-text-tertiary);
  font-size: 0.8125rem;
}

.affiliate-empty {
  padding: 2rem 1.25rem;
  color: var(--ui-text-tertiary);
  font-size: 0.875rem;
  text-align: center;
}

.affiliate-table-wrap {
  overflow-x: auto;
}

.affiliate-table {
  width: 100%;
  min-width: 900px;
  border-collapse: collapse;
  color: var(--ui-text-secondary);
  font-size: 0.8125rem;
  text-align: left;
}

.affiliate-table th,
.affiliate-table td {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--ui-border);
}

.affiliate-table th {
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  font-weight: 500;
}

.affiliate-table th button {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.affiliate-table tbody tr:last-child td {
  border-bottom: 0;
}

.affiliate-mobile-list {
  display: none;
}

@media (max-width: 640px) {
  .affiliate-table-wrap {
    display: none;
  }

  .affiliate-mobile-list {
    display: block;
  }

  .affiliate-mobile-item {
    padding: 1rem;
    border-bottom: 1px solid var(--ui-border);
  }

  .affiliate-mobile-item:last-child {
    border-bottom: 0;
  }

  .affiliate-mobile-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .affiliate-status {
    flex: 0 0 auto;
    color: var(--ui-text-secondary);
    font-size: 0.75rem;
  }

  .affiliate-mobile-stats {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.75rem 1rem;
    margin-top: 1rem;
  }

  .affiliate-mobile-stats dt {
    color: var(--ui-text-tertiary);
    font-size: 0.6875rem;
  }

  .affiliate-mobile-stats dd {
    margin-top: 0.2rem;
    color: var(--ui-text);
    font-size: 0.875rem;
    font-variant-numeric: tabular-nums;
    font-weight: 500;
  }

  .affiliate-mobile-meta {
    margin-top: 0.875rem;
    color: var(--ui-text-tertiary);
    font-size: 0.6875rem;
  }
}
</style>
