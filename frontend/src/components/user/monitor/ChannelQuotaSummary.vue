<template>
  <section class="quota-summary">
    <header class="quota-summary-header">
      <div class="min-w-0">
        <h2>{{ title }}</h2>
        <p v-if="dashboard">{{ formattedGeneratedAt }}</p>
      </div>
      <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="emit('refresh')">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
      </UiIconButton>
    </header>

    <div class="quota-metrics">
      <div v-for="metric in metrics" :key="metric.key" class="quota-metric">
        <span>{{ metric.label }}</span>
        <strong :class="metric.tone">{{ metric.value }}</strong>
      </div>
    </div>

    <div v-if="loading && !dashboard" class="quota-state">
      <Icon name="refresh" size="md" class="animate-spin" />
    </div>
    <div v-else-if="error" class="quota-state quota-state--error">{{ loadFailedMessage }}</div>
    <div v-else-if="groups.length === 0" class="quota-state">{{ emptyMessage }}</div>

    <div v-else class="quota-groups">
      <article v-for="group in visibleGroups" :key="groupKey(group)" class="quota-group-card">
        <header class="quota-group-card__header">
          <div class="quota-group-identity">
            <span class="quota-provider-icon" aria-hidden="true">
              <PlatformIcon :platform="platformIconValue(group.platform)" size="sm" />
            </span>
            <div class="min-w-0">
              <h3>{{ group.group_name || t('admin.accounts.quotaDashboard.ungrouped') }}</h3>
              <p class="quota-group-meta">
                <span>{{ platformLabel(group.platform) }}</span>
                <span v-if="group.account_level">
                  {{ t('admin.accounts.quotaDashboard.accountLevel', { level: accountLevelLabel(group.account_level) }) }}
                </span>
                <span>
                  {{ t('admin.accounts.quotaDashboard.rateMultiplier', { rate: formatRateMultiplier(group.rate_multiplier) }) }}
                </span>
              </p>
            </div>
          </div>
          <span :class="['quota-health', `quota-health--${groupHealth(group)}`]">
            <i />
            {{ t(`admin.accounts.quotaDashboard.groupHealth.${groupHealth(group)}`) }}
          </span>
        </header>

        <div class="quota-group-facts">
          <div class="quota-group-fact">
            <span>{{ t('admin.accounts.quotaDashboard.totalAccounts') }}</span>
            <strong>{{ group.account_count }}</strong>
          </div>
          <div class="quota-group-fact quota-group-fact--success">
            <span>{{ t('admin.accounts.quotaDashboard.schedulableAccounts') }}</span>
            <strong>{{ group.schedulable_account_count }}</strong>
          </div>
          <div class="quota-group-fact">
            <span>{{ t('admin.accounts.quotaDashboard.concurrencyCapacity') }}</span>
            <strong>
              {{ group.schedulable_concurrency_capacity }}
              <small>/ {{ group.concurrency_capacity }}</small>
            </strong>
          </div>
        </div>

        <section class="quota-account-status">
          <div class="quota-account-status__header">
            <span>{{ t('admin.accounts.quotaDashboard.accountStatus') }}</span>
          </div>
          <div class="quota-status-track" :aria-label="t('admin.accounts.quotaDashboard.accountStatus')">
            <span
              v-for="segment in accountStatusSegments(group)"
              :key="segment.key"
              :class="['quota-status-segment', `quota-status-segment--${segment.key}`]"
              :style="{ width: `${segment.percent}%` }"
            />
          </div>
          <div class="quota-status-legend">
            <span v-for="segment in accountStatusSegments(group)" :key="`${segment.key}-legend`">
              <i :class="`quota-status-marker--${segment.key}`" />
              {{ segment.label }}
            </span>
          </div>
        </section>

        <div v-if="group.usage_windows?.length" class="quota-windows">
          <section v-for="window in group.usage_windows" :key="window.window" class="quota-window-card">
            <div class="quota-window-card__header">
              <span>{{ windowLabel(window.window) }}</span>
              <strong>{{ formatPercent(window.average_utilization) }}</strong>
            </div>
            <div class="quota-window-track">
              <span
                :class="quotaBarClass(window.average_utilization)"
                :style="{ width: `${progressWidth(window.average_utilization)}%` }"
              />
            </div>
            <div class="quota-window-card__meta">
              <span>{{ t('admin.accounts.quotaDashboard.schedulableSnapshots', {
                known: window.known_account_count,
                total: window.account_count
              }) }}</span>
              <span>{{ t('admin.accounts.quotaDashboard.schedulableRemainingAccountsEquivalent', {
                count: formatAccountEquivalent(window.remaining_capacity_percent)
              }) }}</span>
            </div>
          </section>
        </div>
      </article>
    </div>

    <footer v-if="totalPages > 1" class="quota-pagination">
      <UiIconButton
        :label="t('pagination.previous')"
        size="sm"
        :disabled="currentPage === 1"
        @click="currentPage -= 1"
      >
        <Icon name="chevronLeft" size="sm" />
      </UiIconButton>
      <span>{{ t('pagination.pageOf', { page: currentPage, total: totalPages }) }}</span>
      <UiIconButton
        :label="t('pagination.next')"
        size="sm"
        :disabled="currentPage === totalPages"
        @click="currentPage += 1"
      >
        <Icon name="chevronRight" size="sm" />
      </UiIconButton>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMediaQuery } from '@vueuse/core'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { UiIconButton } from '@/ui'
import type { AccountQuotaDashboard, AccountQuotaGroupSummary, GroupPlatform } from '@/types'
import { formatDateTime } from '@/utils/format'
import { platformLabel } from '@/utils/platformColors'
import { resolveAccountQuotaGroupHealth } from '@/utils/accountQuotaHealth'

type AccountStatusSegment = {
  key: 'schedulable' | 'rate-limited' | 'quota-protected' | 'error' | 'disabled' | 'unschedulable'
  label: string
  count: number
  percent: number
}

const props = withDefaults(defineProps<{
  dashboard: AccountQuotaDashboard | null
  loading: boolean
  error: boolean
  title: string
  emptyMessage: string
  loadFailedMessage: string
  desktopPageSize?: number
  mobilePageSize?: number
  prioritizeAccountLevels?: boolean
}>(), {
  desktopPageSize: 3,
  mobilePageSize: 1,
  prioritizeAccountLevels: false,
})
const emit = defineEmits<{ (event: 'refresh'): void }>()
const { t } = useI18n()
const isMobileViewport = useMediaQuery('(max-width: 480px)')
const groupsPerPage = computed(() => Math.max(
  1,
  Math.floor(isMobileViewport.value ? props.mobilePageSize : props.desktopPageSize),
))
const currentPage = ref(1)

const platformOrder: Record<string, number> = {
  openai: 0,
  anthropic: 1,
  gemini: 2,
  antigravity: 3,
  grok: 4,
  kiro: 5,
  custom: 6,
}

const accountLevelOrder: Record<string, number> = {
  free: 0,
  plus: 1,
  pro: 2,
  team: 3,
  k12: 4,
}

const totals = computed(() => props.dashboard?.totals)
const groups = computed(() => (props.dashboard?.group_summaries ?? [])
  .filter((group) => group.account_count > 0 || (group.usage_windows?.some((window) => window.account_count > 0) ?? false))
  .sort(compareGroups))
const totalPages = computed(() => Math.max(1, Math.ceil(groups.value.length / groupsPerPage.value)))
const visibleGroups = computed(() => {
  const start = (currentPage.value - 1) * groupsPerPage.value
  return groups.value.slice(start, start + groupsPerPage.value)
})
const formattedGeneratedAt = computed(() => props.dashboard
  ? t('admin.accounts.quotaDashboard.generatedAt', { time: formatDateTime(new Date(props.dashboard.generated_at)) })
  : '')
const metrics = computed(() => [{
  key: 'total',
  label: t('admin.accounts.quotaDashboard.totalAccounts'),
  value: totals.value?.account_count ?? 0,
  tone: '',
}, {
  key: 'schedulable',
  label: t('admin.accounts.quotaDashboard.schedulableAccounts'),
  value: totals.value?.schedulable_account_count ?? 0,
  tone: 'quota-metric--success',
}, {
  key: 'limited',
  label: t('admin.accounts.quotaDashboard.rateLimitedAccounts'),
  value: totals.value?.rate_limited_account_count ?? 0,
  tone: 'quota-metric--warning',
}, {
  key: 'error',
  label: t('admin.accounts.quotaDashboard.exceptionAccounts'),
  value: (totals.value?.error_account_count ?? 0) + (totals.value?.disabled_account_count ?? 0),
  tone: 'quota-metric--danger',
}])

watch([() => groups.value.length, groupsPerPage], () => {
  currentPage.value = Math.min(currentPage.value, totalPages.value)
})

function groupKey(group: AccountQuotaGroupSummary): string {
  return group.group_id ? String(group.group_id) : `${group.platform}:${group.group_name}`
}

function groupHealth(group: AccountQuotaGroupSummary) {
  return resolveAccountQuotaGroupHealth(group)
}

function platformIconValue(platform: string): GroupPlatform | undefined {
  if (['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kiro', 'custom'].includes(platform)) {
    return platform as GroupPlatform
  }
  return undefined
}

function compareGroups(a: AccountQuotaGroupSummary, b: AccountQuotaGroupSummary): number {
  if (!props.prioritizeAccountLevels) {
    return a.group_name.localeCompare(b.group_name)
  }

  const platformDiff = (platformOrder[a.platform] ?? 100) - (platformOrder[b.platform] ?? 100)
  if (platformDiff !== 0) return platformDiff

  const levelDiff = (accountLevelOrder[a.account_level?.toLowerCase() ?? ''] ?? 100)
    - (accountLevelOrder[b.account_level?.toLowerCase() ?? ''] ?? 100)
  if (levelDiff !== 0) return levelDiff
  return a.group_name.localeCompare(b.group_name)
}

function accountLevelLabel(level: string): string {
  return level.trim().toUpperCase()
}

function formatRateMultiplier(value: number | undefined): string {
  if (!Number.isFinite(value)) return '1'
  return String(Number(Number(value).toFixed(4)))
}

function accountStatusSegments(group: AccountQuotaGroupSummary): AccountStatusSegment[] {
  const total = Math.max(group.account_count, 0)
  const accountedCount = group.schedulable_account_count
    + group.rate_limited_account_count
    + group.quota_protected_account_count
    + group.error_account_count
    + group.disabled_account_count
  const raw = [{
    key: 'schedulable' as const,
    label: t('admin.accounts.quotaDashboard.schedulableCount', { count: group.schedulable_account_count }),
    count: group.schedulable_account_count,
  }, {
    key: 'rate-limited' as const,
    label: t('admin.accounts.quotaDashboard.rateLimitedCount', { count: group.rate_limited_account_count }),
    count: group.rate_limited_account_count,
  }, {
    key: 'quota-protected' as const,
    label: t('admin.accounts.quotaDashboard.quotaProtectedCount', { count: group.quota_protected_account_count }),
    count: group.quota_protected_account_count,
  }, {
    key: 'error' as const,
    label: t('admin.accounts.quotaDashboard.errorCount', { count: group.error_account_count }),
    count: group.error_account_count,
  }, {
    key: 'disabled' as const,
    label: t('admin.accounts.quotaDashboard.disabledCount', { count: group.disabled_account_count }),
    count: group.disabled_account_count,
  }, {
    key: 'unschedulable' as const,
    label: t('admin.accounts.quotaDashboard.unschedulableCount', { count: Math.max(total - accountedCount, 0) }),
    count: Math.max(total - accountedCount, 0),
  }]

  return raw
    .filter(segment => segment.count > 0)
    .map(segment => ({
      ...segment,
      percent: total > 0 ? Math.max((segment.count / total) * 100, 0) : 0,
    }))
}

function windowLabel(window: string): string {
  if (window === '5h') return t('admin.accounts.quotaDashboard.window5h')
  if (window === '7d') return t('admin.accounts.quotaDashboard.window7d')
  return window
}

function formatPercent(value: number): string {
  return `${(Number.isFinite(value) ? value : 0).toFixed(1)}%`
}

function formatAccountEquivalent(value: number): string {
  return ((Number.isFinite(value) ? value : 0) / 100).toFixed(2)
}

function progressWidth(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0
  return Math.min(100, value)
}

function quotaBarClass(value: number): string {
  if (value >= 100) return 'quota-window-bar--danger'
  if (value >= 80) return 'quota-window-bar--warning'
  return 'quota-window-bar--success'
}
</script>

<style scoped>
.quota-summary {
  min-width: 0;
  padding: 1rem 1.125rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.quota-summary-header {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.quota-summary-header h2,
.quota-group-identity h3 {
  overflow: hidden;
  color: var(--ui-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.quota-summary-header h2 {
  font-size: 0.9375rem;
  font-weight: 650;
}

.quota-summary-header p {
  margin-top: 0.2rem;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
}

.quota-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1rem;
  margin-top: 1rem;
}

.quota-metric {
  min-width: 0;
}

.quota-metric span {
  display: block;
  overflow: hidden;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.quota-metric strong {
  display: block;
  margin-top: 0.2rem;
  color: var(--ui-text);
  font-size: 1.125rem;
  font-variant-numeric: tabular-nums;
  font-weight: 650;
}

.quota-metric .quota-metric--success {
  color: var(--ui-success);
}

.quota-metric .quota-metric--warning {
  color: var(--ui-warning);
}

.quota-metric .quota-metric--danger {
  color: var(--ui-danger);
}

.quota-state {
  padding: 1.25rem 0 0.25rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  line-height: 1.45;
}

.quota-state--error {
  color: var(--ui-danger);
}

.quota-groups {
  display: grid;
  gap: 1.125rem;
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--ui-border);
}

.quota-group-card {
  min-width: 0;
}

.quota-group-card + .quota-group-card {
  padding-top: 1.125rem;
  border-top: 1px solid var(--ui-border);
}

.quota-group-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.quota-group-identity {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 0.625rem;
}

.quota-provider-icon {
  display: inline-flex;
  width: 1.25rem;
  height: 1.25rem;
  flex: none;
  align-items: center;
  justify-content: center;
  margin-top: 0.0625rem;
  line-height: 1;
}

.quota-provider-icon :deep(svg) {
  display: block;
}

.quota-group-identity h3 {
  font-size: 0.9375rem;
  font-weight: 650;
}

.quota-group-meta {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.2rem 0.4rem;
  margin-top: 0.18rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

.quota-group-meta span + span::before {
  margin-right: 0.4rem;
  content: '\00b7';
}

.quota-health {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 0.375rem;
  padding: 0.25rem 0.5rem;
  border-radius: 999px;
  background: color-mix(in srgb, currentColor 12%, transparent);
  color: var(--ui-text-secondary);
  font-size: 0.6875rem;
  font-weight: 650;
  white-space: nowrap;
}

.quota-health i {
  width: 0.35rem;
  height: 0.35rem;
  border-radius: 50%;
  background: currentColor;
}

.quota-health--normal {
  color: var(--ui-success);
}

.quota-health--degraded,
.quota-health--constrained {
  color: var(--ui-warning);
}

.quota-health--unavailable {
  color: var(--ui-danger);
}

.quota-group-facts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-top: 0.875rem;
  padding: 0.75rem 0;
  border-top: 1px solid var(--ui-border);
  border-bottom: 1px solid var(--ui-border);
}

.quota-group-fact {
  min-width: 0;
  padding: 0 0.875rem;
}

.quota-group-fact:first-child {
  padding-left: 0;
}

.quota-group-fact:last-child {
  padding-right: 0;
}

.quota-group-fact + .quota-group-fact {
  border-left: 1px solid var(--ui-border);
}

.quota-group-fact span {
  display: block;
  overflow: hidden;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.quota-group-fact strong {
  display: block;
  margin-top: 0.3rem;
  color: var(--ui-text);
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
  font-weight: 650;
  line-height: 1.2;
}

.quota-group-fact strong small {
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  font-weight: 500;
}

.quota-group-fact--success strong {
  color: var(--ui-success);
}

.quota-account-status {
  margin-top: 0.875rem;
}

.quota-account-status__header,
.quota-window-card__header,
.quota-window-card__meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.quota-account-status__header {
  color: var(--ui-text-secondary);
  font-size: 0.75rem;
  font-weight: 650;
}

.quota-status-track {
  display: flex;
  height: 0.625rem;
  overflow: hidden;
  margin-top: 0.625rem;
  border-radius: 999px;
  background: var(--ui-surface-hover);
}

.quota-status-segment {
  display: block;
  min-width: 2px;
  height: 100%;
}

.quota-status-segment--schedulable,
.quota-status-marker--schedulable {
  background: #31c6b5;
}

.quota-status-segment--rate-limited,
.quota-status-marker--rate-limited {
  background: #f59e0b;
}

.quota-status-segment--quota-protected,
.quota-status-marker--quota-protected {
  background: #facc15;
}

.quota-status-segment--error,
.quota-status-marker--error {
  background: #ef4444;
}

.quota-status-segment--disabled,
.quota-status-marker--disabled {
  background: #94a3b8;
}

.quota-status-segment--unschedulable,
.quota-status-marker--unschedulable {
  background: var(--ui-text-tertiary);
}

.quota-status-legend {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(7.25rem, 1fr));
  gap: 0.5rem 0.75rem;
  margin-top: 0.75rem;
}

.quota-status-legend span {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.4rem;
  color: var(--ui-text-secondary);
  font-size: 0.6875rem;
  white-space: nowrap;
}

.quota-status-legend i {
  width: 0.5rem;
  height: 0.5rem;
  flex: none;
  border-radius: 2px;
}

.quota-windows {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0;
  overflow: hidden;
  margin-top: 0.875rem;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
  background: var(--ui-surface-subtle);
}

.quota-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  margin-top: 1rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--ui-border);
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
  font-variant-numeric: tabular-nums;
}

.quota-window-card {
  min-width: 0;
  padding: 0.75rem;
}

.quota-window-card + .quota-window-card {
  border-left: 1px solid var(--ui-border);
}

.quota-window-card__header {
  color: var(--ui-text-secondary);
  font-size: 0.75rem;
  font-weight: 650;
}

.quota-window-card__header strong {
  color: var(--ui-text);
  font-variant-numeric: tabular-nums;
}

.quota-window-track {
  height: 0.375rem;
  overflow: hidden;
  margin-top: 0.625rem;
  border-radius: 999px;
  background: var(--ui-border);
}

.quota-window-track span {
  display: block;
  height: 100%;
  border-radius: inherit;
}

.quota-window-bar--success {
  background: var(--ui-success);
}

.quota-window-bar--warning {
  background: var(--ui-warning);
}

.quota-window-bar--danger {
  background: var(--ui-danger);
}

.quota-window-card__meta {
  margin-top: 0.5rem;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
  line-height: 1.35;
}

.quota-window-card__meta span:last-child {
  text-align: right;
}

@media (max-width: 720px) {
  .quota-summary {
    padding: 0.875rem;
  }

  .quota-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.75rem 1rem;
  }

  .quota-group-card__header {
    align-items: center;
  }

  .quota-window-card__meta {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.125rem;
  }

  .quota-window-card__meta span:last-child {
    text-align: left;
  }
}

@media (max-width: 480px) {
  .quota-summary {
    padding: 0.75rem;
  }

  .quota-metrics {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0.5rem;
    margin-top: 0.875rem;
  }

  .quota-metric span {
    font-size: 0.625rem;
  }

  .quota-metric strong {
    font-size: 1rem;
  }

  .quota-groups {
    gap: 0.875rem;
    margin-top: 0.75rem;
    padding-top: 0.75rem;
  }

  .quota-group-facts {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .quota-group-fact {
    padding: 0 0.5rem;
  }

  .quota-group-fact:first-child {
    padding-left: 0;
  }

  .quota-group-fact:last-child {
    padding-right: 0;
  }

  .quota-group-fact span {
    font-size: 0.625rem;
  }

  .quota-status-legend {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .quota-windows {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .quota-window-card {
    padding: 0.625rem;
  }

  .quota-window-card__header,
  .quota-window-card__meta {
    font-size: 0.625rem;
  }
}
</style>
