<template>
  <section class="subscription-panel">
    <header class="subscription-header">
      <div class="subscription-identity">
        <span class="subscription-platform-dot" :class="platformDotClass" aria-hidden="true"></span>
        <div class="min-w-0">
          <div class="subscription-name-row">
            <h2>{{ subscription.group?.name || `Group #${subscription.group_id}` }}</h2>
            <span class="subscription-platform">{{ platformLabel(subscription.group?.platform || '') }}</span>
          </div>
          <p v-if="subscription.group?.description">{{ subscription.group.description }}</p>
        </div>
      </div>

      <div class="subscription-actions">
        <span class="subscription-status">
          <span class="subscription-status-dot" :class="statusDotClass"></span>
          {{ t(`userSubscriptions.status.${subscription.status}`) }}
        </span>
        <button
          v-if="subscription.status === 'active'"
          class="btn btn-primary btn-sm"
          @click="emit('renew')"
        >
          {{ t('payment.renewNow') }}
        </button>
      </div>
    </header>

    <div class="subscription-body">
      <div class="subscription-expiration">
        <span>{{ t('userSubscriptions.expires') }}</span>
        <strong :class="expirationClass">{{ expirationText }}</strong>
      </div>

      <div v-if="usageWindows.length > 0" class="subscription-usage-grid">
        <div v-for="window in usageWindows" :key="window.key" class="subscription-usage-item">
          <div class="subscription-usage-head">
            <span>{{ window.label }}</span>
            <strong>${{ window.used.toFixed(2) }} / ${{ window.limit.toFixed(2) }}</strong>
          </div>
          <div class="subscription-progress">
            <span :class="progressTone(window.used, window.limit)" :style="{ width: progressWidth(window.used, window.limit) }"></span>
          </div>
          <p v-if="window.start">
            {{ t('userSubscriptions.resetIn', { time: formatResetTime(window.start, window.hours) }) }}
          </p>
        </div>
      </div>

      <div v-else class="subscription-unlimited">
        <strong>{{ t('userSubscriptions.unlimited') }}</strong>
        <span>{{ t('userSubscriptions.unlimitedDesc') }}</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserSubscription } from '@/types'
import { formatDateOnly } from '@/utils/format'
import { platformLabel } from '@/utils/platformColors'

const props = defineProps<{
  subscription: UserSubscription
}>()

const emit = defineEmits<{
  renew: []
}>()

const { t } = useI18n()

const platformDotClass = computed(() => {
  switch (props.subscription.group?.platform) {
    case 'anthropic': return 'subscription-platform-dot--anthropic'
    case 'openai': return 'subscription-platform-dot--openai'
    case 'antigravity': return 'subscription-platform-dot--antigravity'
    case 'gemini': return 'subscription-platform-dot--gemini'
    default: return 'subscription-platform-dot--default'
  }
})

const statusDotClass = computed(() => {
  if (props.subscription.status === 'active') return 'subscription-status-dot--active'
  if (props.subscription.status === 'expired') return 'subscription-status-dot--muted'
  return 'subscription-status-dot--danger'
})

const usageWindows = computed(() => {
  const group = props.subscription.group
  const windows = [
    {
      key: 'daily',
      label: t('userSubscriptions.daily'),
      used: props.subscription.daily_usage_usd || 0,
      limit: group?.daily_limit_usd || 0,
      start: props.subscription.daily_window_start,
      hours: 24
    },
    {
      key: 'weekly',
      label: t('userSubscriptions.weekly'),
      used: props.subscription.weekly_usage_usd || 0,
      limit: group?.weekly_limit_usd || 0,
      start: props.subscription.weekly_window_start,
      hours: 168
    },
    {
      key: 'monthly',
      label: t('userSubscriptions.monthly'),
      used: props.subscription.monthly_usage_usd || 0,
      limit: group?.monthly_limit_usd || 0,
      start: props.subscription.monthly_window_start,
      hours: 720
    }
  ]
  return windows.filter((window) => window.limit > 0)
})

const expirationText = computed(() => {
  const expiresAt = props.subscription.expires_at
  if (!expiresAt || isPermanentExpiration(expiresAt)) return t('userSubscriptions.noExpiration')

  const expires = new Date(expiresAt)
  const days = Math.ceil((expires.getTime() - Date.now()) / (1000 * 60 * 60 * 24))
  if (days < 0) return t('userSubscriptions.status.expired')

  const date = formatDateOnly(expires)
  if (days === 0) return `${date} (${t('common.today')})`
  if (days === 1) return `${date} (${t('common.tomorrow')})`
  return `${t('userSubscriptions.daysRemaining', { days })} (${date})`
})

const expirationClass = computed(() => {
  const expiresAt = props.subscription.expires_at
  if (!expiresAt || isPermanentExpiration(expiresAt)) return ''
  const days = Math.ceil((new Date(expiresAt).getTime() - Date.now()) / (1000 * 60 * 60 * 24))
  if (days <= 3) return 'subscription-expiration--danger'
  if (days <= 7) return 'subscription-expiration--warning'
  return ''
})

function isPermanentExpiration(expiresAt: string): boolean {
  const expires = new Date(expiresAt)
  return !Number.isNaN(expires.getTime()) && expires.getUTCFullYear() >= 2099
}

function progressWidth(used: number, limit: number): string {
  return `${Math.min((used / limit) * 100, 100)}%`
}

function progressTone(used: number, limit: number): string {
  const percentage = (used / limit) * 100
  if (percentage >= 90) return 'subscription-progress--danger'
  if (percentage >= 70) return 'subscription-progress--warning'
  return 'subscription-progress--normal'
}

function formatResetTime(windowStart: string, windowHours: number): string {
  const end = new Date(new Date(windowStart).getTime() + windowHours * 60 * 60 * 1000)
  const diff = end.getTime() - Date.now()
  if (diff <= 0) return t('userSubscriptions.windowNotActive')

  const hours = Math.floor(diff / (1000 * 60 * 60))
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
  if (hours > 24) return `${Math.floor(hours / 24)}d ${hours % 24}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}
</script>

<style scoped>
.subscription-panel {
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.subscription-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--ui-border);
}

.subscription-identity,
.subscription-name-row,
.subscription-actions,
.subscription-status {
  display: flex;
  align-items: center;
}

.subscription-identity {
  min-width: 0;
  gap: 0.75rem;
}

.subscription-name-row {
  min-width: 0;
  gap: 0.6rem;
}

.subscription-name-row h2 {
  overflow: hidden;
  color: var(--ui-text);
  font-size: 0.9375rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.subscription-identity p {
  margin-top: 0.2rem;
  overflow: hidden;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.subscription-platform {
  flex: 0 0 auto;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
}

.subscription-platform-dot,
.subscription-status-dot {
  width: 0.45rem;
  height: 0.45rem;
  flex: 0 0 auto;
  border-radius: 999px;
}

.subscription-platform-dot--anthropic { background: #d97757; }
.subscription-platform-dot--openai { background: #10a37f; }
.subscription-platform-dot--antigravity { background: #7c3aed; }
.subscription-platform-dot--gemini { background: #4285f4; }
.subscription-platform-dot--default { background: var(--ui-text-tertiary); }

.subscription-actions {
  flex: 0 0 auto;
  gap: 0.75rem;
}

.subscription-status {
  gap: 0.35rem;
  color: var(--ui-text-secondary);
  font-size: 0.75rem;
}

.subscription-status-dot--active { background: var(--ui-success); }
.subscription-status-dot--muted { background: var(--ui-text-tertiary); }
.subscription-status-dot--danger { background: var(--ui-danger); }

.subscription-body {
  padding: 1rem 1.25rem 1.25rem;
}

.subscription-expiration {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

.subscription-expiration strong {
  color: var(--ui-text-secondary);
  font-weight: 500;
  text-align: right;
}

.subscription-expiration--warning { color: var(--ui-warning) !important; }
.subscription-expiration--danger { color: var(--ui-danger) !important; }

.subscription-usage-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1.5rem;
  margin-top: 1.25rem;
}

.subscription-usage-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  color: var(--ui-text-secondary);
  font-size: 0.75rem;
}

.subscription-usage-head strong {
  color: var(--ui-text);
  font-variant-numeric: tabular-nums;
  font-weight: 500;
}

.subscription-progress {
  height: 0.3rem;
  margin-top: 0.55rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--ui-surface-muted);
}

.subscription-progress span {
  display: block;
  height: 100%;
  border-radius: inherit;
}

.subscription-progress--normal { background: var(--ui-text); }
.subscription-progress--warning { background: var(--ui-warning); }
.subscription-progress--danger { background: var(--ui-danger); }

.subscription-usage-item p {
  margin-top: 0.45rem;
  color: var(--ui-text-tertiary);
  font-size: 0.6875rem;
}

.subscription-unlimited {
  display: flex;
  align-items: baseline;
  gap: 0.75rem;
  margin-top: 1.25rem;
  padding-top: 1rem;
  border-top: 1px solid var(--ui-border);
}

.subscription-unlimited strong {
  color: var(--ui-text);
  font-size: 0.875rem;
}

.subscription-unlimited span {
  color: var(--ui-text-tertiary);
  font-size: 0.75rem;
}

@media (max-width: 760px) {
  .subscription-header {
    align-items: stretch;
    flex-direction: column;
  }

  .subscription-actions {
    justify-content: space-between;
  }

  .subscription-usage-grid {
    grid-template-columns: 1fr;
    gap: 1rem;
  }
}
</style>
