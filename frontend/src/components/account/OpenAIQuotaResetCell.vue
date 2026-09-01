<template>
  <div v-if="visible" class="space-y-1">
    <div class="flex flex-wrap items-center gap-1.5">
      <slot name="pre-actions" />

      <button
        type="button"
        class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium text-blue-600 transition-colors hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-50 dark:text-blue-400 dark:hover:bg-blue-900/30"
        :disabled="loading || resetting"
        :title="countButtonTitle"
        @click="handleQuery()"
      >
        <svg
          class="h-2.5 w-2.5"
          :class="{ 'animate-spin': loading }"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
        {{ t('admin.accounts.openaiQuotaReset.count') }}<span v-if="data"> {{ availableResetCount }}</span>
      </button>

      <button
        type="button"
        class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium text-orange-600 transition-colors hover:bg-orange-50 disabled:cursor-not-allowed disabled:opacity-50 dark:text-orange-400 dark:hover:bg-orange-900/30"
        :disabled="resetting || loading || !canReset"
        :title="resetButtonTitle"
        @click="handleReset"
      >
        <svg
          class="h-2.5 w-2.5"
          :class="{ 'animate-spin': resetting }"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M20 12a8 8 0 11-2.343-5.657L20 8m0 0V4m0 4h-4"
          />
        </svg>
        {{ t('admin.accounts.openaiQuotaReset.reset') }}
      </button>
    </div>

    <div
      v-if="autoResetState"
      class="flex flex-wrap items-center gap-1 text-[10px]"
      data-testid="auto-reset-credit-state"
    >
      <span
        class="inline-flex items-center rounded px-1.5 py-0.5 font-medium"
        :class="autoResetStateClass"
      >
        {{ autoResetStateLabel }}
        <span v-if="autoResetState.trigger_window" class="ml-1 tabular-nums">
          {{ autoResetState.trigger_window }}
        </span>
      </span>
      <span v-if="autoResetState.checked_at" class="text-gray-500 dark:text-gray-400">
        {{ formatResetCreditExpiry(autoResetState.checked_at, 'short') }}
      </span>
      <span
        v-if="autoResetState.error_code"
        class="max-w-full truncate text-red-600 dark:text-red-400"
        :title="autoResetState.error_code"
      >
        {{ autoResetState.error_code }}
      </span>
    </div>

    <div v-if="primaryResetCreditExpiry" class="space-y-1">
      <div class="flex flex-wrap items-center gap-1">
        <span
          class="inline-flex max-w-full items-center rounded bg-gray-100 px-1.5 py-0.5 text-[10px] leading-4 text-gray-600 tabular-nums dark:bg-dark-800 dark:text-gray-300"
          :title="t('admin.accounts.openaiQuotaReset.expiresAtFull', { time: formatResetCreditExpiry(primaryResetCreditExpiry, 'full') })"
        >
          {{ t('admin.accounts.openaiQuotaReset.expiresAt', { time: formatResetCreditExpiry(primaryResetCreditExpiry, 'short') }) }}
        </span>
        <button
          v-if="hiddenResetCreditCount > 0"
          type="button"
          data-testid="reset-credit-expiry-toggle"
          class="inline-flex items-center rounded-full bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium leading-4 text-gray-600 transition-colors hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
          :aria-expanded="showResetCreditDetails"
          :title="resetCreditDetailsTitle"
          @click="toggleResetCreditDetails"
        >
          +{{ hiddenResetCreditCount }}
        </button>
      </div>

      <div
        v-if="showResetCreditDetails && resetCreditExpirations.length > 1"
        data-testid="reset-credit-expiry-details"
        class="inline-grid max-w-full gap-0.5 rounded border border-gray-200 bg-white px-1.5 py-1 text-[10px] leading-4 text-gray-600 shadow-sm dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
      >
        <span
          v-for="(expiresAt, index) in resetCreditExpirations"
          :key="`${expiresAt}-${index}`"
          class="flex min-w-0 items-center gap-1 tabular-nums"
          :title="formatResetCreditExpiry(expiresAt, 'full')"
        >
          <span class="h-1 w-1 shrink-0 rounded-full bg-gray-400 dark:bg-dark-500" />
          <span class="truncate">{{ formatResetCreditExpiry(expiresAt, 'short') }}</span>
        </span>
      </div>
    </div>

    <div
      v-if="error"
      class="text-[10px] text-red-600 dark:text-red-400"
      :title="error"
    >
      {{ truncatedError }}
    </div>
    <div
      v-else-if="resetWarning"
      class="text-[10px] text-amber-600 dark:text-amber-400"
      :title="resetWarningDetail || resetWarning"
    >
      {{ resetWarning }}
    </div>
    <div
      v-else-if="resetMessage"
      class="text-[10px] text-emerald-600 dark:text-emerald-400"
    >
      {{ resetMessage }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import {
  queryOpenAIQuota as queryAdminOpenAIQuota,
  resetOpenAIQuota as resetAdminOpenAIQuota,
  type OpenAIQuotaResetResult,
  type OpenAIQuotaUsage
} from '@/api/admin/accounts'
import {
  queryOpenAIQuota as queryUserOpenAIQuota,
  resetOpenAIQuota as resetUserOpenAIQuota
} from '@/api/accounts'

const props = withDefaults(
  defineProps<{
    account: Account
    accountScope?: 'admin' | 'user'
  }>(),
  {
    accountScope: 'admin'
  }
)

const emit = defineEmits<{
  (e: 'reset', accountId: number, result: OpenAIQuotaResetResult): void
}>()

const { t } = useI18n()

const visible = computed(() => props.account.platform === 'openai' && props.account.type === 'oauth')

const loading = ref(false)
const resetting = ref(false)
const error = ref<string | null>(null)
const data = ref<OpenAIQuotaUsage | null>(null)
const cachedData = ref<OpenAIQuotaUsage | null>(null)
const resetMessage = ref<string | null>(null)
const resetWarning = ref<string | null>(null)
const resetWarningDetail = ref<string | null>(null)
const showResetCreditDetails = ref(false)

interface AutoResetCreditState {
  status?: string
  trigger_window?: string
  checked_at?: string
  error_code?: string
}

const validAutoResetStatuses = new Set([
  'checking',
  'available',
  'resetting',
  'success',
  'no_credit',
  'failed'
])
const autoResetState = computed<AutoResetCreditState | null>(() => {
  if (props.account.extra?.auto_reset_credit_enabled !== true) return null
  const state = props.account.extra?.codex_auto_reset_credit_state
  if (!state || typeof state !== 'object' || Array.isArray(state)) return null
  const parsed = state as AutoResetCreditState
  return parsed.status && validAutoResetStatuses.has(parsed.status) ? parsed : null
})
const autoResetStateLabel = computed(() => {
  if (!autoResetState.value?.status) return ''
  const keyByStatus: Record<string, string> = {
    checking: 'checking',
    available: 'available',
    resetting: 'resetting',
    success: 'success',
    no_credit: 'noCredit',
    failed: 'failed'
  }
  return t(`admin.accounts.openaiQuotaReset.autoStatus.${keyByStatus[autoResetState.value.status]}`)
})
const autoResetStateClass = computed(() => {
  switch (autoResetState.value?.status) {
    case 'available':
      return 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
    case 'success':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
    case 'no_credit':
    case 'failed':
      return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    case 'resetting':
      return 'bg-orange-50 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-300'
  }
})

const readCachedResetCredits = (account: Account): OpenAIQuotaUsage | null => {
  const cached = account.extra?.codex_reset_credit_snapshot
  if (!cached || typeof cached !== 'object' || Array.isArray(cached)) return null
  const { available_count: count, credits: rawCredits } = cached as {
    available_count?: unknown
    credits?: unknown
  }
  if (typeof count !== 'number' || !Number.isFinite(count)) return null
  const now = Date.now()
  const credits: { expires_at?: string }[] = []
  if (Array.isArray(rawCredits)) {
    for (const credit of rawCredits) {
      if (!credit || typeof credit !== 'object') continue
      const expiresAt = (credit as { expires_at?: unknown }).expires_at
      if (typeof expiresAt !== 'string' || !expiresAt.trim()) continue
      const expiryTime = new Date(expiresAt).getTime()
      if (!Number.isNaN(expiryTime) && expiryTime <= now) continue
      credits.push({ expires_at: expiresAt })
    }
  }
  const availableCount = Math.min(Math.max(count, 0), credits.length)
  if (count > 0 && availableCount <= 0) return null
  return {
    fetched_at: 0,
    rate_limit_reset_credits: { available_count: availableCount, credits }
  }
}

cachedData.value = readCachedResetCredits(props.account)
data.value = cachedData.value

const isShadow = computed(() => props.account.parent_account_id != null)

const availableResetCount = computed(() => data.value?.rate_limit_reset_credits?.available_count ?? 0)
// Prefer the live payload and fall back to the persisted snapshot only when the
// live state is unknown, so the count and the expirations never come from two
// different generations of the same data.
const resetCreditExpirations = computed(() =>
  ((data.value ?? cachedData.value)?.rate_limit_reset_credits?.credits ?? [])
    .map((credit) => credit.expires_at?.trim() ?? '')
    .filter((expiresAt) => expiresAt.length > 0)
    .sort(compareResetCreditExpiry)
)
const primaryResetCreditExpiry = computed(() => resetCreditExpirations.value[0] ?? '')
const hiddenResetCreditCount = computed(() => Math.max(resetCreditExpirations.value.length - 1, 0))
const canReset = computed(() => availableResetCount.value > 0 && !isShadow.value)

const resetCreditDetailsTitle = computed(() =>
  resetCreditExpirations.value
    .map((expiresAt) => formatResetCreditExpiry(expiresAt, 'full'))
    .join('\n')
)

const resetButtonTitle = computed(() => {
  if (isShadow.value) return t('admin.accounts.openaiQuotaReset.resetTooltipShadow')
  if (!data.value) return t('admin.accounts.openaiQuotaReset.resetTooltipNeedQuery')
  if (!canReset.value) return t('admin.accounts.openaiQuotaReset.resetTooltipNoCredits')
  return t('admin.accounts.openaiQuotaReset.resetTooltipReady')
})

const countButtonTitle = computed(() => {
  if (!data.value) return t('admin.accounts.openaiQuotaReset.countTooltipLoad')
  return t('admin.accounts.openaiQuotaReset.countTooltipRefresh')
})

const truncatedError = computed(() => {
  if (!error.value) return ''
  return error.value.length > 80 ? `${error.value.slice(0, 80)}…` : error.value
})

const getResetCreditExpiryTime = (value: string): number => {
  const time = new Date(value).getTime()
  return Number.isNaN(time) ? Number.POSITIVE_INFINITY : time
}

const compareResetCreditExpiry = (a: string, b: string): number => {
  const diff = getResetCreditExpiryTime(a) - getResetCreditExpiryTime(b)
  if (diff !== 0) return diff
  return a.localeCompare(b)
}

const formatResetCreditExpiry = (value: string, style: 'short' | 'full'): string => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value

  const options: Intl.DateTimeFormatOptions = {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }
  if (style === 'full') {
    options.year = 'numeric'
  }
  return new Intl.DateTimeFormat(undefined, options).format(date)
}

const extractErrorMessage = (e: unknown): string => {
  const err = e as {
    message?: string
    reason?: string
    response?: { data?: { message?: string; error?: string } }
  }
  return (
    err?.message ||
    err?.reason ||
    err?.response?.data?.message ||
    err?.response?.data?.error ||
    t('common.error')
  )
}

const toggleResetCreditDetails = () => {
  if (hiddenResetCreditCount.value <= 0) return
  showResetCreditDetails.value = !showResetCreditDetails.value
}

const handleQuery = async () => {
  if (loading.value) return
  loading.value = true
  error.value = null
  resetMessage.value = null
  resetWarning.value = null
  resetWarningDetail.value = null
  showResetCreditDetails.value = false
  try {
    data.value = await (props.accountScope === 'user'
      ? queryUserOpenAIQuota(props.account.id)
      : queryAdminOpenAIQuota(props.account.id))
  } catch (e) {
    error.value = extractErrorMessage(e)
  } finally {
    loading.value = false
  }
}

const handleReset = async () => {
  if (resetting.value) return
  if (!canReset.value) {
    error.value = t('admin.accounts.openaiQuotaReset.noCreditsAvailable')
    return
  }
  resetting.value = true
  error.value = null
  resetMessage.value = null
  resetWarning.value = null
  resetWarningDetail.value = null
  try {
    const result: OpenAIQuotaResetResult = await (props.accountScope === 'user'
      ? resetUserOpenAIQuota(props.account.id)
      : resetAdminOpenAIQuota(props.account.id))
    await handleQuery()
    if (result.runtime_state_warning || result.runtime_state_cleared === false) {
      error.value = null
      resetWarning.value = t('admin.accounts.openaiQuotaReset.runtimeStateWarning')
      resetWarningDetail.value = result.runtime_state_warning || null
    } else {
      resetMessage.value = t('admin.accounts.openaiQuotaReset.resetSuccess', {
        windows: result.windows_reset
      })
    }
    emit('reset', props.account.id, result)
  } catch (e) {
    error.value = extractErrorMessage(e)
  } finally {
    resetting.value = false
  }
}

watch(
  () => props.account.id,
  () => {
    cachedData.value = readCachedResetCredits(props.account)
    data.value = cachedData.value
    error.value = null
    resetMessage.value = null
    resetWarning.value = null
    resetWarningDetail.value = null
    loading.value = false
    resetting.value = false
    showResetCreditDetails.value = false
  }
)

watch(
  resetCreditExpirations,
  () => {
    if (hiddenResetCreditCount.value <= 0) {
      showResetCreditDetails.value = false
    }
  }
)
</script>
