<template>
  <section
    class="card min-w-0 overflow-hidden border border-amber-200 bg-white shadow-sm dark:border-amber-900/50 dark:bg-dark-900/60"
    data-testid="experimental-prompt-card"
  >
    <div class="border-b border-amber-100 bg-amber-50/70 px-5 py-4 dark:border-amber-900/40 dark:bg-amber-950/20">
      <div class="flex items-start gap-3">
        <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
          <Icon name="sparkles" size="md" />
        </div>
        <div class="min-w-0">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('profile.experimentalPrompt.title') }}
          </h3>
          <p class="mt-1 text-sm leading-5 text-gray-600 dark:text-gray-400">
            {{ t('profile.experimentalPrompt.description') }}
          </p>
        </div>
      </div>
    </div>

    <div v-if="loading" class="space-y-3 p-5">
      <div class="h-12 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
      <div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
    </div>

    <div v-else-if="status" class="space-y-4 p-5">
      <div class="grid grid-cols-2 gap-2">
        <div class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 dark:border-dark-700 dark:bg-dark-800">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('profile.experimentalPrompt.serviceStatus') }}
          </p>
          <p
            class="mt-1 text-sm font-semibold"
            :class="status.configured ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-600 dark:text-gray-300'"
          >
            {{ status.configured ? t('profile.experimentalPrompt.configured') : t('profile.experimentalPrompt.notConfigured') }}
          </p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 dark:border-dark-700 dark:bg-dark-800">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('profile.experimentalPrompt.entitlementStatus') }}
          </p>
          <p
            class="mt-1 text-sm font-semibold"
            :class="status.unlocked ? 'text-emerald-700 dark:text-emerald-300' : 'text-amber-700 dark:text-amber-300'"
          >
            {{ status.unlocked ? t('profile.experimentalPrompt.unlocked') : t('profile.experimentalPrompt.locked') }}
          </p>
        </div>
      </div>

      <div v-if="status.unlocked" class="rounded-lg border border-emerald-200 bg-emerald-50 px-3.5 py-3 dark:border-emerald-900/50 dark:bg-emerald-950/20">
        <div class="flex items-start gap-2.5">
          <Icon name="check" size="sm" class="mt-0.5 shrink-0 text-emerald-700 dark:text-emerald-300" />
          <p class="text-sm leading-5 text-emerald-800 dark:text-emerald-200">
            {{ t('profile.experimentalPrompt.unlockedHint') }}
          </p>
        </div>
      </div>

      <template v-else>
        <div>
          <div class="mb-2 flex items-end justify-between gap-3">
            <div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('profile.experimentalPrompt.price') }}
              </p>
              <p class="mt-0.5 text-xl font-semibold text-gray-900 dark:text-white">
                {{ formattedPrice }}
              </p>
            </div>
            <button
              data-testid="experimental-prompt-purchase"
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="purchasing || !status.configured"
              @click="purchase"
            >
              {{ purchasing ? t('common.processing') : t('profile.experimentalPrompt.purchase') }}
            </button>
          </div>
          <p v-if="!status.configured" class="text-xs leading-5 text-amber-700 dark:text-amber-300">
            {{ t('profile.experimentalPrompt.purchaseUnavailable') }}
          </p>
        </div>

        <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
          <label class="input-label" for="experimental-prompt-redeem-code">
            {{ t('profile.experimentalPrompt.redeemLabel') }}
          </label>
          <div class="flex flex-col gap-2 sm:flex-row">
            <input
              data-testid="experimental-prompt-redeem-input"
              id="experimental-prompt-redeem-code"
              v-model="redeemCode"
              type="text"
              class="input min-w-0 flex-1 font-mono text-sm uppercase"
              :placeholder="t('profile.experimentalPrompt.redeemPlaceholder')"
              :disabled="redeeming"
              @keydown.enter.prevent="redeemFeature"
            />
            <button
              data-testid="experimental-prompt-redeem"
              type="button"
              class="btn btn-secondary btn-sm w-full shrink-0 sm:w-auto"
              :disabled="redeeming || !redeemCode.trim()"
              @click="redeemFeature"
            >
              {{ redeeming ? t('common.processing') : t('profile.experimentalPrompt.redeem') }}
            </button>
          </div>
        </div>
      </template>
    </div>

    <div v-else class="p-5">
      <button type="button" class="btn btn-secondary btn-sm w-full" @click="loadStatus">
        <Icon name="refresh" size="sm" />
        {{ t('common.retry') }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { redeemAPI } from '@/api'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { OpenAIExperimentalPromptStatus } from '@/types'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const status = ref<OpenAIExperimentalPromptStatus | null>(null)
const loading = ref(true)
const purchasing = ref(false)
const redeeming = ref(false)
const redeemCode = ref('')

const formattedPrice = computed(() => {
  const amount = Math.max(0, Number(status.value?.price_cents || 0)) / 100
  return t('profile.experimentalPrompt.priceValue', { amount: amount.toFixed(2) })
})

const purchaseShortfall = computed(() => {
  const price = Math.max(0, Number(status.value?.price_cents || 0)) / 100
  const balance = Math.max(0, Number(authStore.user?.balance || 0))
  return Math.max(0, price - balance)
})

async function loadStatus() {
  loading.value = true
  try {
    status.value = await redeemAPI.getOpenAIExperimentalPromptStatus()
  } catch (error: unknown) {
    status.value = null
    appStore.showError(
      extractApiErrorMessage(error, t('profile.experimentalPrompt.loadFailed')),
    )
  } finally {
    loading.value = false
  }
}

async function refreshAfterUnlock(nextStatus?: OpenAIExperimentalPromptStatus) {
  if (nextStatus) {
    status.value = nextStatus
  } else {
    status.value = await redeemAPI.getOpenAIExperimentalPromptStatus()
  }
  await authStore.refreshUser()
}

async function purchase() {
  if (!status.value?.configured || status.value.unlocked) return

  purchasing.value = true
  try {
    const nextStatus = await redeemAPI.purchaseOpenAIExperimentalPrompt()
    await refreshAfterUnlock(nextStatus)
    appStore.showSuccess(t('profile.experimentalPrompt.purchaseSuccess'))
  } catch (error: unknown) {
    if (extractApiErrorCode(error) === 'INSUFFICIENT_BALANCE') {
      try {
        await authStore.refreshUser()
      } catch {
        // Keep the purchase error visible even if the balance refresh fails.
      }
      appStore.showError(t('profile.experimentalPrompt.insufficientBalance', {
        amount: purchaseShortfall.value.toFixed(2),
      }))
    } else {
      appStore.showError(
        extractApiErrorMessage(error, t('profile.experimentalPrompt.purchaseFailed')),
      )
    }
  } finally {
    purchasing.value = false
  }
}

async function redeemFeature() {
  const code = redeemCode.value.trim()
  if (!code || status.value?.unlocked) return

  redeeming.value = true
  try {
    const result = await redeemAPI.redeem(code)
    if (result.type !== 'feature') {
      appStore.showError(t('profile.experimentalPrompt.wrongCodeType'))
      await authStore.refreshUser()
      return
    }
    redeemCode.value = ''
    await refreshAfterUnlock()
    appStore.showSuccess(t('profile.experimentalPrompt.redeemSuccess'))
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('profile.experimentalPrompt.redeemFailed')),
    )
  } finally {
    redeeming.value = false
  }
}

onMounted(loadStatus)
</script>
