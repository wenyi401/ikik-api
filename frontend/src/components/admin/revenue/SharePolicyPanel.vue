<template>
  <div class="space-y-6">
    <section class="card p-5">
      <div class="mb-5 flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('admin.revenue.sharePolicy.title') }}
          </h3>
          <p class="mt-1 max-w-3xl text-sm leading-6 text-gray-500 dark:text-gray-400">
            {{ t('admin.revenue.sharePolicy.description') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary h-10"
          :disabled="loading"
          :title="t('common.refresh')"
          @click="loadPolicies"
        >
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <div v-if="loading && !policies.length" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <div v-else class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="flex items-center justify-between gap-3">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.revenue.sharePolicy.currentPolicy') }}
            </h4>
            <span
              class="rounded-full px-2.5 py-1 text-xs font-medium"
              :class="effectivePolicy ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'"
            >
              {{ effectivePolicy ? t('common.enabled') : t('admin.revenue.sharePolicy.notConfigured') }}
            </span>
          </div>

          <dl class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div>
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.ownerShare') }}
              </dt>
              <dd class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
                {{ effectivePolicy ? formatPolicyPercent(effectivePolicy.owner_share_ratio) : '--' }}
              </dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.inviteShare') }}
              </dt>
              <dd class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
                {{ effectivePolicy ? formatPolicyPercent(effectivePolicy.invite_share_ratio ?? 0) : '--' }}
              </dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.platformShare') }}
              </dt>
              <dd class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
                {{ effectivePolicy ? formatPolicyPercent(platformShareRatio(effectivePolicy)) : '--' }}
              </dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.effectiveAt') }}
              </dt>
              <dd class="mt-1 text-sm text-gray-900 dark:text-white">
                {{ effectivePolicy ? formatDateTime(effectivePolicy.effective_at) : '--' }}
              </dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.version') }}
              </dt>
              <dd class="mt-1 text-sm text-gray-900 dark:text-white">
                {{ effectivePolicy ? effectivePolicy.version : '--' }}
              </dd>
            </div>
          </dl>

          <p class="mt-5 text-xs leading-5 text-gray-500 dark:text-gray-400">
            {{ t('admin.revenue.sharePolicy.currentPolicyHint') }}
          </p>
        </div>

        <form class="rounded-lg border border-gray-200 p-4 dark:border-dark-700" @submit.prevent="savePolicy">
          <div class="flex items-center justify-between gap-3">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.revenue.sharePolicy.globalConfig') }}
            </h4>
            <label class="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
              <input v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-emerald-600 focus:ring-emerald-500" />
              {{ t('admin.revenue.sharePolicy.enabled') }}
            </label>
          </div>

          <div class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.ownerSharePercent') }}
              </label>
              <div class="relative">
                <input
                  v-model="form.ownerSharePercent"
                  type="number"
                  min="0"
                  max="100"
                  step="0.01"
                  class="input w-full pr-10"
                  @blur="normalizeFormPercent"
                />
                <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-400">%</span>
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.ownerShareHint') }}
              </p>
            </div>

            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.inviteSharePercent') }}
              </label>
              <div class="relative">
                <input
                  v-model="form.inviteSharePercent"
                  type="number"
                  min="0"
                  max="100"
                  step="0.01"
                  class="input w-full pr-10"
                  @blur="normalizeFormPercent"
                />
                <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-400">%</span>
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.inviteShareHint') }}
              </p>
            </div>

            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.platformSharePercent') }}
              </label>
              <div class="input flex h-10 items-center bg-gray-50 text-gray-700 dark:bg-dark-800 dark:text-gray-300">
                {{ formatPercentFromNumber(platformSharePercent) }}
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.platformShareHint') }}
              </p>
            </div>
          </div>

          <div class="mt-5 flex flex-col gap-2 border-t border-gray-100 pt-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
            <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
              {{ t('admin.revenue.sharePolicy.saveHint') }}
            </p>
            <button type="submit" class="btn btn-primary h-10" :disabled="saving">
              {{ saving ? t('common.saving') : t('admin.revenue.sharePolicy.savePolicy') }}
            </button>
          </div>
        </form>
      </div>
    </section>

    <section class="card p-5">
      <div class="mb-4 flex items-center justify-between gap-3">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('admin.revenue.sharePolicy.history') }}
        </h3>
        <span class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.revenue.sharePolicy.globalOnly') }}
        </span>
      </div>

      <div v-if="policies.length" class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <thead>
            <tr>
              <th class="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                ID
              </th>
              <th class="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('common.status') }}
              </th>
              <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.ownerShare') }}
              </th>
              <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.inviteShare') }}
              </th>
              <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.platformShare') }}
              </th>
              <th class="px-3 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.version') }}
              </th>
              <th class="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.effectiveAt') }}
              </th>
              <th class="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.revenue.sharePolicy.updatedAt') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="policy in policies" :key="policy.id" class="hover:bg-gray-50 dark:hover:bg-dark-800">
              <td class="px-3 py-3 text-sm font-medium text-gray-900 dark:text-white">
                {{ policy.id }}
              </td>
              <td class="px-3 py-3">
                <span
                  class="rounded-full px-2.5 py-1 text-xs font-medium"
                  :class="policy.enabled ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
                >
                  {{ policy.enabled ? t('common.enabled') : t('common.disabled') }}
                </span>
              </td>
              <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">
                {{ formatPolicyPercent(policy.owner_share_ratio) }}
              </td>
              <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">
                {{ formatPolicyPercent(policy.invite_share_ratio ?? 0) }}
              </td>
              <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">
                {{ formatPolicyPercent(platformShareRatio(policy)) }}
              </td>
              <td class="px-3 py-3 text-right text-sm text-gray-700 dark:text-gray-300">
                {{ policy.version }}
              </td>
              <td class="px-3 py-3 text-sm text-gray-700 dark:text-gray-300">
                {{ formatDateTime(policy.effective_at) }}
              </td>
              <td class="px-3 py-3 text-sm text-gray-700 dark:text-gray-300">
                {{ formatDateTime(policy.updated_at) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else class="flex h-40 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.revenue.sharePolicy.noPolicy') }}
      </div>
    </section>

    <section class="card p-5">
      <div class="mb-4">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('admin.revenue.sharePolicy.privateGroupCommissionTitle') }}
        </h3>
        <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">
          {{ t('admin.revenue.sharePolicy.privateGroupCommissionDescription') }}
        </p>
      </div>

      <form class="grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,280px)_1fr] md:items-end" @submit.prevent="saveCommissionSettings">
        <div>
          <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
            {{ t('admin.revenue.sharePolicy.privateGroupCommissionRate') }}
          </label>
          <div class="relative">
            <input
              v-model="commissionForm.ratePercent"
              type="number"
              min="0"
              max="100"
              step="0.01"
              class="input w-full pr-10"
              @blur="normalizeCommissionPercent"
            />
            <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-400">%</span>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.revenue.sharePolicy.privateGroupCommissionHint') }}
          </p>
        </div>

        <div class="flex flex-col gap-2 md:items-start">
          <div class="text-sm text-gray-600 dark:text-gray-300">
            {{ t('admin.revenue.sharePolicy.privateGroupCommissionExample', { rate: formatPercentFromNumber(Number(commissionForm.ratePercent) || 0) }) }}
          </div>
          <button type="submit" class="btn btn-primary h-10" :disabled="savingCommission">
            {{ savingCommission ? t('common.saving') : t('admin.revenue.sharePolicy.saveCommission') }}
          </button>
        </div>
      </form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { accountSharePoliciesAPI } from '@/api/admin/accountSharePolicies'
import type { AccountSharePolicy } from '@/api/admin/accountSharePolicies'
import { getSettings, updateSettings } from '@/api/admin/settings'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t, locale } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const savingCommission = ref(false)
const policies = ref<AccountSharePolicy[]>([])
const form = reactive({
  ownerSharePercent: 70 as number | string,
  inviteSharePercent: 0 as number | string,
  enabled: true
})
const commissionForm = reactive({
  ratePercent: 0.5 as number | string
})

const effectivePolicy = computed(() => {
  const now = Date.now()
  return policies.value.find(policy => policy.enabled && parseDate(policy.effective_at) <= now) ?? null
})

const editablePolicy = computed(() => effectivePolicy.value ?? policies.value[0] ?? null)

const normalizedOwnerSharePercentValue = computed(() => {
  const value = Number(form.ownerSharePercent)
  if (!Number.isFinite(value)) return 0
  return clampPercent(value)
})

const normalizedInviteSharePercentValue = computed(() => {
  const value = Number(form.inviteSharePercent)
  if (!Number.isFinite(value)) return 0
  return clampPercent(value)
})

const platformSharePercent = computed(() => clampPercent(100 - normalizedOwnerSharePercentValue.value - normalizedInviteSharePercentValue.value))

async function loadPolicies() {
  loading.value = true
  try {
    const [result, settings] = await Promise.all([
      accountSharePoliciesAPI.list(1, 50, {
        scope_type: 'global',
        sort_by: 'effective_at',
        sort_order: 'desc'
      }),
      getSettings()
    ])
    policies.value = [...result.items].sort(comparePolicyByEffectiveAtDesc)
    commissionForm.ratePercent = roundPercent((settings.user_private_group_commission_rate ?? 0) * 100)
    syncFormFromPolicy()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.revenue.sharePolicy.errors', t('admin.revenue.sharePolicy.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function saveCommissionSettings() {
  const rateValue = Number(commissionForm.ratePercent)
  if (!Number.isFinite(rateValue) || rateValue < 0 || rateValue > 100) {
    appStore.showError(t('admin.revenue.sharePolicy.invalidRatio'))
    return
  }

  savingCommission.value = true
  try {
    await updateSettings({
      user_private_group_commission_rate: clampPercent(rateValue) / 100
    })
    commissionForm.ratePercent = roundPercent(clampPercent(rateValue))
    appStore.showSuccess(t('admin.revenue.sharePolicy.saved'))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.revenue.sharePolicy.errors', t('admin.revenue.sharePolicy.saveFailed')))
  } finally {
    savingCommission.value = false
  }
}

async function savePolicy() {
  const ownerValue = Number(form.ownerSharePercent)
  const inviteValue = Number(form.inviteSharePercent)
  if (!Number.isFinite(ownerValue) || ownerValue < 0 || ownerValue > 100 || !Number.isFinite(inviteValue) || inviteValue < 0 || inviteValue > 100 || ownerValue + inviteValue > 100) {
    appStore.showError(t('admin.revenue.sharePolicy.invalidRatio'))
    return
  }

  saving.value = true
  try {
    const payload = {
      scope_type: 'global' as const,
      owner_share_ratio: clampPercent(ownerValue) / 100,
      invite_share_ratio: clampPercent(inviteValue) / 100,
      enabled: form.enabled,
      effective_at: new Date().toISOString()
    }
    const target = editablePolicy.value
    if (target) {
      await accountSharePoliciesAPI.update(target.id, payload)
    } else {
      await accountSharePoliciesAPI.create(payload)
    }
    appStore.showSuccess(t('admin.revenue.sharePolicy.saved'))
    await loadPolicies()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.revenue.sharePolicy.errors', t('admin.revenue.sharePolicy.saveFailed')))
  } finally {
    saving.value = false
  }
}

function syncFormFromPolicy() {
  const target = editablePolicy.value
  if (!target) {
    form.ownerSharePercent = 70
    form.inviteSharePercent = 0
    form.enabled = true
    return
  }
  form.ownerSharePercent = roundPercent(target.owner_share_ratio * 100)
  form.inviteSharePercent = roundPercent((target.invite_share_ratio ?? 0) * 100)
  form.enabled = target.enabled
}

function normalizeFormPercent() {
  const ownerValue = Number(form.ownerSharePercent)
  if (!Number.isFinite(ownerValue)) {
    form.ownerSharePercent = 0
  } else {
    form.ownerSharePercent = roundPercent(clampPercent(ownerValue))
  }

  const inviteValue = Number(form.inviteSharePercent)
  if (!Number.isFinite(inviteValue)) {
    form.inviteSharePercent = 0
  } else {
    form.inviteSharePercent = roundPercent(clampPercent(inviteValue))
  }
}

function normalizeCommissionPercent() {
  const value = Number(commissionForm.ratePercent)
  if (!Number.isFinite(value)) {
    commissionForm.ratePercent = 0
  } else {
    commissionForm.ratePercent = roundPercent(clampPercent(value))
  }
}

function comparePolicyByEffectiveAtDesc(a: AccountSharePolicy, b: AccountSharePolicy) {
  const byTime = parseDate(b.effective_at) - parseDate(a.effective_at)
  if (byTime !== 0) return byTime
  return b.id - a.id
}

function clampPercent(value: number): number {
  return Math.min(100, Math.max(0, value))
}

function roundPercent(value: number): number {
  return Math.round(value * 100) / 100
}

function parseDate(value: string): number {
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function formatPolicyPercent(ratio: number): string {
  return formatPercentFromNumber(ratio * 100)
}

function platformShareRatio(policy: AccountSharePolicy): number {
  return Math.max(0, 1 - policy.owner_share_ratio - (policy.invite_share_ratio ?? 0))
}

function formatPercentFromNumber(value: number): string {
  return `${roundPercent(clampPercent(value)).toFixed(2)}%`
}

function formatDateTime(value: string): string {
  const parsed = parseDate(value)
  if (!parsed) return '--'
  return new Intl.DateTimeFormat(locale.value, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(parsed))
}

onMounted(() => {
  void loadPolicies()
})
</script>
