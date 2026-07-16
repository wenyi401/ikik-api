<template>
  <section class="card border border-gray-100 bg-white/90 p-5 dark:border-dark-700 dark:bg-dark-900/50 md:p-6">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('profile.receiptCode.title') }}
        </h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('profile.receiptCode.description') }}
        </p>
      </div>

      <div class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
        <button
          v-for="method in paymentMethods"
          :key="method"
          type="button"
          class="rounded-md px-3 py-1.5 text-sm font-medium transition"
          :class="selectedMethod === method
            ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
            : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'"
          @click="selectMethod(method)"
        >
          {{ methodLabel(method) }}
        </button>
      </div>
    </div>

    <div class="mt-5 grid gap-5 lg:grid-cols-[220px,1fr]">
      <div
        class="flex aspect-square w-full max-w-[220px] items-center justify-center overflow-hidden rounded-xl border border-dashed border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-900/60"
      >
        <img
          v-if="previewUrl"
          :src="previewUrl"
          :alt="methodLabel(selectedMethod)"
          class="h-full w-full object-contain"
        >
        <div v-else class="flex flex-col items-center gap-2 text-gray-400 dark:text-gray-500">
          <Icon name="creditCard" size="xl" />
          <span class="text-sm">{{ t('profile.receiptCode.empty') }}</span>
        </div>
      </div>

      <div class="flex min-w-0 flex-col justify-between gap-4 rounded-xl border border-gray-100 bg-gray-50/70 p-5 dark:border-dark-700 dark:bg-dark-900/30">
        <div class="space-y-2 text-sm text-gray-600 dark:text-gray-300">
          <p>
            {{ currentCode ? t('profile.receiptCode.savedAt', { time: formatDateTime(currentCode.updated_at) }) : t('profile.receiptCode.notUploaded') }}
          </p>
          <p v-if="currentCode" class="break-all text-xs text-gray-500 dark:text-gray-400">
            SHA256: {{ currentCode.sha256 }}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('profile.receiptCode.hint') }}
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <label class="btn btn-secondary btn-sm cursor-pointer">
            <input
              type="file"
              accept="image/png,image/jpeg,image/gif,image/webp"
              class="hidden"
              @change="handleFileChange"
            >
            <Icon name="upload" size="sm" class="mr-1.5" />
            {{ t('profile.receiptCode.uploadAction') }}
          </label>

          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="saving || !draftFile"
            @click="handleSave"
          >
            {{ saving ? t('common.loading') : t('common.save') }}
          </button>

          <button
            type="button"
            class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
            :disabled="saving || (!currentCode && !draftFile)"
            @click="handleDelete"
          >
            <Icon name="trash" size="sm" class="mr-1.5" />
            {{ t('common.delete') }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { userAPI } from '@/api'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import type { ReceiptCode, ReceiptCodePaymentMethod } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const paymentMethods: ReceiptCodePaymentMethod[] = ['alipay', 'wechat']
const selectedMethod = ref<ReceiptCodePaymentMethod>('alipay')
const receiptCodes = ref<Partial<Record<ReceiptCodePaymentMethod, ReceiptCode | null>>>({})
const draftFile = ref<File | null>(null)
const draftPreviewUrl = ref('')
const loading = ref(false)
const saving = ref(false)

const currentCode = computed(() => receiptCodes.value[selectedMethod.value] ?? null)
const previewUrl = computed(() => draftPreviewUrl.value || currentCode.value?.url?.trim() || '')

onMounted(() => {
  void loadCurrent()
})

onBeforeUnmount(() => {
  revokeDraftPreview()
})

function methodLabel(method: ReceiptCodePaymentMethod): string {
  return t(`profile.receiptCode.methods.${method}`)
}

async function selectMethod(method: ReceiptCodePaymentMethod) {
  if (selectedMethod.value === method) {
    return
  }
  selectedMethod.value = method
  clearDraft()
  await loadCurrent()
}

async function loadCurrent() {
  const method = selectedMethod.value
  loading.value = true
  try {
    receiptCodes.value[method] = await userAPI.getReceiptCode(method)
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('profile.receiptCode.loadFailed')))
  } finally {
    loading.value = false
  }
}

function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  if (input) {
    input.value = ''
  }
  if (!file) {
    return
  }
  if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(file.type)) {
    appStore.showError(t('profile.receiptCode.invalidType'))
    return
  }
  if (file.size > 1024 * 1024) {
    appStore.showError(t('profile.receiptCode.tooLarge'))
    return
  }
  revokeDraftPreview()
  draftFile.value = file
  draftPreviewUrl.value = URL.createObjectURL(file)
}

async function handleSave() {
  if (!draftFile.value) {
    appStore.showError(t('profile.receiptCode.uploadRequired'))
    return
  }

  const method = selectedMethod.value
  saving.value = true
  try {
    receiptCodes.value[method] = await userAPI.uploadReceiptCode(method, draftFile.value)
    clearDraft()
    appStore.showSuccess(t('profile.receiptCode.saveSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('profile.receiptCode.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  clearDraft()
  if (!currentCode.value) {
    return
  }

  const method = selectedMethod.value
  saving.value = true
  try {
    await userAPI.deleteReceiptCode(method)
    receiptCodes.value[method] = null
    appStore.showSuccess(t('profile.receiptCode.deleteSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('profile.receiptCode.deleteFailed')))
  } finally {
    saving.value = false
  }
}

function clearDraft() {
  revokeDraftPreview()
  draftFile.value = null
}

function revokeDraftPreview() {
  if (draftPreviewUrl.value) {
    URL.revokeObjectURL(draftPreviewUrl.value)
    draftPreviewUrl.value = ''
  }
}

function formatDateTime(raw: string): string {
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}
</script>
