<template>
  <BaseDialog
    :show="show"
    :title="provider ? t('freeModels.statusDialogTitle', { provider: provider.name }) : t('freeModels.keyStatus')"
    width="wide"
    @close="emit('close')"
  >
    <div v-if="provider" class="space-y-4">
      <div class="rounded-2xl border border-[var(--app-border)] bg-[var(--app-surface-muted)] p-4">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div class="text-sm font-semibold text-[var(--app-text)]">{{ provider.name }}</div>
            <p class="mt-1 text-sm text-[var(--app-text-muted)]">
              {{ t('freeModels.statusDialogDescription') }}
            </p>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="testingProvider"
            @click="emit('test-all')"
          >
            <Icon name="refresh" size="xs" :class="testingProvider ? 'animate-spin' : ''" />
            {{ t('freeModels.testAll') }}
          </button>
        </div>
      </div>

      <div class="space-y-3">
        <div
          v-for="account in accounts"
          :key="account.id"
          class="rounded-2xl border border-[var(--app-border)] bg-[var(--app-surface)] p-4"
        >
          <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-sm font-semibold text-[var(--app-text)]">{{ account.name }}</span>
                <span :class="accountHealthBadgeClass(account)" class="rounded-full px-2 py-0.5 text-xs font-medium">
                  {{ accountHealthLabel(account) }}
                </span>
              </div>
              <div class="mt-1 truncate font-mono text-xs text-[var(--app-text-muted)]">
                {{ accountBaseUrl(account) }}
              </div>
            </div>
            <div class="flex shrink-0 flex-wrap gap-2">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="testingAccountId === account.id"
                @click="emit('test-account', account)"
              >
                <Icon name="play" size="xs" />
                {{ testingAccountId === account.id ? t('freeModels.testing') : t('freeModels.test') }}
              </button>
              <button type="button" class="btn btn-danger btn-sm" @click="emit('delete-account', account)">
                <Icon name="trash" size="xs" />
                {{ t('freeModels.delete') }}
              </button>
            </div>
          </div>

          <div class="mt-3 flex flex-wrap gap-2">
            <span
              v-for="model in accountModelIDs(account)"
              :key="`active-${model}`"
              class="max-w-full truncate rounded-full bg-[var(--app-surface-muted)] px-2 py-1 font-mono text-[11px] text-[var(--app-text-muted)]"
              :title="model"
            >
              {{ model }}
            </span>
            <span
              v-for="model in disabledModelIDs(account)"
              :key="`disabled-${model}`"
              class="max-w-full truncate rounded-full bg-amber-50 px-2 py-1 font-mono text-[11px] text-amber-700 line-through dark:bg-amber-900/25 dark:text-amber-300"
              :title="disabledModelTitle(account, model)"
            >
              {{ model }}
            </span>
          </div>
          <p
            v-if="disabledModelIDs(account).length > 0"
            class="mt-2 text-xs leading-5 text-amber-700 dark:text-amber-300"
          >
            {{ t('freeModels.disabledModelHint') }}
          </p>

          <div v-if="accountLimitText(account) || account.error_message || testResults[account.id]?.message" class="mt-3 space-y-1 text-xs leading-5 text-[var(--app-text-muted)]">
            <p v-if="accountLimitText(account)">
              {{ accountLimitText(account) }}
            </p>
            <p v-if="account.error_message" class="text-red-600 dark:text-red-300">
              {{ account.error_message }}
            </p>
            <p v-if="testResults[account.id]?.message" :class="testResultTextClass(testResults[account.id]?.status)">
              {{ testResults[account.id]?.message }}
              <span v-if="testResults[account.id]?.latency != null">
                · {{ t('freeModels.latencyMs', { ms: Math.round(testResults[account.id]?.latency || 0) }) }}
              </span>
            </p>
          </div>
        </div>

        <div
          v-if="accounts.length === 0"
          class="rounded-2xl border border-dashed border-[var(--app-border)] p-8 text-center"
        >
          <Icon name="key" size="lg" class="mx-auto text-[var(--app-text-muted)]" />
          <p class="mt-3 text-sm font-medium text-[var(--app-text)]">{{ t('freeModels.noProviderKeys') }}</p>
          <button type="button" class="btn btn-primary btn-sm mt-4" @click="emit('connect')">
            <Icon name="plus" size="xs" />
            {{ t('freeModels.connect') }}
          </button>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { FreeModelAccount, FreeModelProvider, FreeModelTestState } from './types'

defineProps<{
  show: boolean
  provider: FreeModelProvider | null
  accounts: FreeModelAccount[]
  testResults: Record<number, FreeModelTestState>
  testingAccountId: number | null
  testingProvider: boolean
}>()

const emit = defineEmits<{
  close: []
  connect: []
  'test-all': []
  'test-account': [account: FreeModelAccount]
  'delete-account': [account: FreeModelAccount]
}>()

const { t } = useI18n()

function parseFutureTime(value: unknown): Date | null {
  if (typeof value !== 'string' || !value) return null
  const time = new Date(value)
  if (Number.isNaN(time.getTime())) return null
  return time.getTime() > Date.now() ? time : null
}

function accountHasActiveLimit(account: FreeModelAccount): boolean {
  if (parseFutureTime(account.rate_limit_reset_at)) return true
  if (parseFutureTime(account.temp_unschedulable_until)) return true
  if (parseFutureTime(account.overload_until)) return true
  const modelLimits = account.extra?.model_rate_limits
  if (!modelLimits || typeof modelLimits !== 'object' || Array.isArray(modelLimits)) return false
  return Object.values(modelLimits as Record<string, { rate_limit_reset_at?: string }>).some((item) => parseFutureTime(item.rate_limit_reset_at))
}

function accountHealthLabel(account: FreeModelAccount): string {
  if (accountHasActiveLimit(account)) return t('freeModels.health.limited')
  if (accountModelIDs(account).length === 0 && disabledModelIDs(account).length > 0) return t('freeModels.health.model_filtered')
  if (account.status === 'active') return t('freeModels.health.normal')
  if (account.status === 'disabled') return t('freeModels.status.disabled')
  if (account.status === 'inactive') return t('freeModels.status.inactive')
  return t('freeModels.health.error')
}

function accountHealthBadgeClass(account: FreeModelAccount): string {
  if (accountHasActiveLimit(account)) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (accountModelIDs(account).length === 0 && disabledModelIDs(account).length > 0) return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  if (account.status === 'active') return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  if (account.status === 'disabled' || account.status === 'inactive') return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300'
  return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
}

function accountModelIDs(account: FreeModelAccount): string[] {
  const mapping = account.credentials?.model_mapping
  if (!mapping || typeof mapping !== 'object' || Array.isArray(mapping)) return []
  return Object.keys(mapping as Record<string, unknown>)
}

function disabledModelMap(account: FreeModelAccount): Record<string, { disabled_at?: string; error?: string }> {
  const value = account.extra?.free_model_disabled_models
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  return value as Record<string, { disabled_at?: string; error?: string }>
}

function disabledModelIDs(account: FreeModelAccount): string[] {
  return Object.keys(disabledModelMap(account))
}

function disabledModelTitle(account: FreeModelAccount, model: string): string {
  return disabledModelMap(account)[model]?.error || model
}

function accountBaseUrl(account: FreeModelAccount): string {
  const value = account.credentials?.base_url
  return typeof value === 'string' ? value : ''
}

function accountLimitText(account: FreeModelAccount): string {
  const direct = parseFutureTime(account.rate_limit_reset_at)
  if (direct) return t('freeModels.limitedUntil', { time: direct.toLocaleString() })
  const temp = parseFutureTime(account.temp_unschedulable_until)
  if (temp) return t('freeModels.unavailableUntil', { time: temp.toLocaleString() })
  const overload = parseFutureTime(account.overload_until)
  if (overload) return t('freeModels.overloadUntil', { time: overload.toLocaleString() })
  const modelLimits = account.extra?.model_rate_limits
  if (!modelLimits || typeof modelLimits !== 'object' || Array.isArray(modelLimits)) return ''
  const limited = Object.entries(modelLimits as Record<string, { rate_limit_reset_at?: string }>).find(([, item]) => parseFutureTime(item.rate_limit_reset_at))
  if (!limited) return ''
  return t('freeModels.modelLimitedUntil', {
    model: limited[0],
    time: parseFutureTime(limited[1].rate_limit_reset_at)?.toLocaleString() || '-'
  })
}

function testResultTextClass(status: FreeModelTestState['status'] | undefined): string {
  if (status === 'success') return 'text-green-700 dark:text-green-300'
  if (status === 'warning') return 'text-amber-700 dark:text-amber-300'
  return 'text-red-600 dark:text-red-300'
}
</script>
