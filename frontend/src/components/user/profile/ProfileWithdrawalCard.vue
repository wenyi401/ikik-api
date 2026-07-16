<template>
  <section class="card min-w-0 overflow-hidden border border-gray-100 bg-white/90 p-5 dark:border-dark-700 dark:bg-dark-900/50 md:p-6">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
      <div class="min-w-0">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">余额提现与收款码</h3>
        <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
          用户账户余额可提现，最低 1.00 元，金额最多保留两位小数。首次提交提现申请额外扣除 0.10 元。
        </p>
      </div>

      <div class="grid grid-cols-1 gap-2 sm:min-w-[18rem] sm:grid-cols-2">
        <div class="rounded-lg bg-primary-50 px-4 py-3 dark:bg-primary-900/20">
          <p class="text-xs text-primary-600 dark:text-primary-300">当前余额</p>
          <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">${{ balance.toFixed(2) }}</p>
        </div>
        <div class="rounded-lg bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
          <p class="text-xs text-gray-500 dark:text-gray-400">提现状态</p>
          <p class="mt-1 text-sm font-semibold" :class="hasPendingWithdrawal ? 'text-yellow-700 dark:text-yellow-300' : 'text-green-700 dark:text-green-300'">
            {{ hasPendingWithdrawal ? '已有待结算' : '可提交' }}
          </p>
        </div>
      </div>
    </div>

    <div class="mt-5 grid min-w-0 items-start gap-5 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1fr)_minmax(0,1.05fr)]">
      <div class="min-w-0 rounded-lg border border-gray-100 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-900/30">
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-sm font-semibold text-gray-900 dark:text-white">提交提现</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">同一用户只能保留一笔待结算提现。</p>
          </div>
          <Icon name="dollar" size="lg" class="text-primary-500" />
        </div>

        <label class="mt-4 block text-sm font-medium text-gray-700 dark:text-gray-300">提现金额</label>
        <input
          v-model="amountText"
          type="text"
          inputmode="decimal"
          pattern="^\d+(\.\d{1,2})?$"
          class="input mt-2"
          placeholder="1.00"
        >

        <div class="mt-4">
          <p class="text-sm font-medium text-gray-700 dark:text-gray-300">本次收款码</p>
          <div class="mt-2 grid grid-cols-2 gap-2">
            <button
              v-for="method in paymentMethods"
              :key="method"
              type="button"
              class="min-h-11 rounded-lg border px-3 py-2 text-sm font-medium transition"
              :class="selectedMethod === method
                ? 'border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-700 dark:bg-primary-900/30 dark:text-primary-200'
                : 'border-gray-200 bg-white text-gray-600 hover:border-gray-300 dark:border-dark-700 dark:bg-dark-900/60 dark:text-gray-300 dark:hover:border-dark-600'"
              @click="selectMethod(method)"
            >
              {{ methodLabel(method) }}
            </button>
          </div>
        </div>

        <div class="mt-4 rounded-lg border border-gray-100 bg-white p-3 text-sm dark:border-dark-700 dark:bg-dark-900/60">
          <div class="flex justify-between gap-3">
            <span class="text-gray-500 dark:text-gray-400">提现金额</span>
            <span class="font-medium text-gray-900 dark:text-white">${{ normalizedAmount.toFixed(2) }}</span>
          </div>
          <div class="mt-2 flex justify-between gap-3">
            <span class="text-gray-500 dark:text-gray-400">首次手续费</span>
            <span class="font-medium text-gray-900 dark:text-white">${{ feeAmount.toFixed(2) }}</span>
          </div>
          <div class="mt-2 flex justify-between gap-3 border-t border-gray-100 pt-2 dark:border-dark-700">
            <span class="text-gray-500 dark:text-gray-400">本次扣除</span>
            <span class="font-semibold text-gray-900 dark:text-white">${{ totalDeducted.toFixed(2) }}</span>
          </div>
        </div>

        <p v-if="submitHint" class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ submitHint }}</p>

        <button
          type="button"
          class="btn btn-primary mt-4 w-full"
          :disabled="submitting || !canSubmit"
          @click="submit"
        >
          {{ submitting ? t('common.processing') : '提交提现申请' }}
        </button>
      </div>

      <div class="min-w-0 rounded-lg border border-gray-100 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-900/30">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-sm font-semibold text-gray-900 dark:text-white">收款码管理</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">上传后保存，提交提现时会保存本次收款码快照。</p>
          </div>
          <button class="btn btn-secondary btn-sm" :disabled="loading" @click="load">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>

        <div
          class="mt-4 flex aspect-square w-full items-center justify-center overflow-hidden rounded-lg border border-dashed border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900/60"
        >
          <img
            v-if="previewUrl"
            :src="previewUrl"
            :alt="methodLabel(selectedMethod)"
            class="h-full w-full object-contain"
          >
          <div v-else class="flex flex-col items-center gap-2 text-gray-400 dark:text-gray-500">
            <Icon name="creditCard" size="xl" />
            <span class="text-sm">未上传{{ methodLabel(selectedMethod) }}收款码</span>
          </div>
        </div>

        <div class="mt-4 min-h-12 text-sm text-gray-600 dark:text-gray-300">
          <p>
            {{ currentReceiptCode ? `已保存于 ${formatDateTime(currentReceiptCode.updated_at)}` : '当前方式还未保存收款码' }}
          </p>
          <p v-if="draftFile" class="mt-1 text-xs text-primary-600 dark:text-primary-300">
            已选择新图片，保存后才能用于提现。
          </p>
          <p v-else-if="currentReceiptCode" class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
            SHA256: {{ currentReceiptCode.sha256 }}
          </p>
        </div>

        <div class="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-3">
          <label class="btn btn-secondary btn-sm min-h-11 cursor-pointer justify-center">
            <input
              type="file"
              accept="image/png,image/jpeg,image/gif,image/webp"
              class="hidden"
              @change="handleFileChange"
            >
            <Icon name="upload" size="sm" class="mr-1.5" />
            上传
          </label>

          <button
            type="button"
            class="btn btn-primary btn-sm min-h-11"
            :disabled="saving || !draftFile"
            @click="handleSave"
          >
            {{ saving ? t('common.loading') : t('common.save') }}
          </button>

          <button
            type="button"
            class="btn btn-secondary btn-sm min-h-11 text-red-600 hover:text-red-700 dark:text-red-400"
            :disabled="saving || (!currentReceiptCode && !draftFile)"
            @click="handleDelete"
          >
            <Icon name="trash" size="sm" class="mr-1.5" />
            {{ draftFile ? '清除' : '删除' }}
          </button>
        </div>
      </div>

      <div class="min-w-0 rounded-lg border border-gray-100 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-900/30">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-sm font-semibold text-gray-900 dark:text-white">提现记录</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">待结算申请可取消，取消后金额退回余额。</p>
          </div>
          <span class="rounded-full bg-white px-2.5 py-1 text-xs text-gray-500 ring-1 ring-gray-100 dark:bg-dark-900 dark:text-gray-400 dark:ring-dark-700">
            最近 {{ withdrawals.length }} 笔
          </span>
        </div>

        <div v-if="withdrawals.length" class="mt-4 max-h-[22rem] space-y-3 overflow-y-auto pr-1">
          <div
            v-for="item in withdrawals"
            :key="item.id"
            class="rounded-lg border border-gray-100 bg-white p-3 dark:border-dark-700 dark:bg-dark-900/60"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span class="font-mono text-sm text-gray-700 dark:text-gray-300">#{{ item.id }}</span>
              <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(item.status)">
                {{ statusLabel(item.status) }}
              </span>
            </div>
            <div class="mt-3 grid grid-cols-2 gap-3 text-sm">
              <div>
                <p class="text-xs text-gray-500 dark:text-gray-400">提现金额</p>
                <p class="font-medium text-gray-900 dark:text-white">${{ item.amount.toFixed(2) }}</p>
              </div>
              <div>
                <p class="text-xs text-gray-500 dark:text-gray-400">扣除</p>
                <p class="font-medium text-gray-900 dark:text-white">${{ item.total_deducted.toFixed(2) }}</p>
              </div>
            </div>
            <div class="mt-3 flex flex-wrap items-center justify-between gap-2">
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDate(item.created_at) }}</span>
              <button
                v-if="item.status === 'PENDING'"
                type="button"
                class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                :disabled="actionLoading"
                @click="cancel(item.id)"
              >
                取消提现
              </button>
            </div>
          </div>
        </div>
        <p v-else class="mt-4 rounded-lg border border-dashed border-gray-200 bg-white p-4 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-900/60 dark:text-gray-400">
          暂无提现记录
        </p>
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
import { useAuthStore } from '@/stores/auth'
import type { ReceiptCode, ReceiptCodePaymentMethod, WithdrawalRequest, WithdrawalStatus } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const paymentMethods: ReceiptCodePaymentMethod[] = ['alipay', 'wechat']
const selectedMethod = ref<ReceiptCodePaymentMethod>('alipay')
const receiptCodes = ref<Partial<Record<ReceiptCodePaymentMethod, ReceiptCode | null>>>({})
const withdrawals = ref<WithdrawalRequest[]>([])
const amountText = ref<string | number>('')
const draftFile = ref<File | null>(null)
const draftPreviewUrl = ref('')
const loading = ref(false)
const saving = ref(false)
const submitting = ref(false)
const actionLoading = ref(false)

const balance = computed(() => Number(authStore.user?.share_income_balance || 0))
const currentReceiptCode = computed(() => receiptCodes.value[selectedMethod.value] ?? null)
const previewUrl = computed(() => draftPreviewUrl.value || currentReceiptCode.value?.url?.trim() || '')
const hasAnyWithdrawal = computed(() => withdrawals.value.length > 0)
const hasPendingWithdrawal = computed(() => withdrawals.value.some(item => item.status === 'PENDING'))
const feeAmount = computed(() => hasAnyWithdrawal.value ? 0 : 0.1)
const amountRawText = computed(() => String(amountText.value).trim())
const normalizedAmount = computed(() => {
  const value = Number(amountRawText.value)
  return Number.isFinite(value) ? Math.round(value * 100) / 100 : 0
})
const amountIsValid = computed(() => /^\d+(\.\d{1,2})?$/.test(amountRawText.value) && normalizedAmount.value >= 1)
const totalDeducted = computed(() => normalizedAmount.value + feeAmount.value)
const canSubmit = computed(() => {
  return amountIsValid.value
    && !!currentReceiptCode.value
    && !draftFile.value
    && !hasPendingWithdrawal.value
    && balance.value + 1e-9 >= totalDeducted.value
})
const submitHint = computed(() => {
  if (hasPendingWithdrawal.value) return '已有待结算提现，处理完成或取消后才能再次提交。'
  if (draftFile.value) return '新收款码需要先保存，保存后本次提现才会使用它。'
  if (!currentReceiptCode.value) return '请先上传并保存本次要使用的收款码。'
  if (amountRawText.value && !amountIsValid.value) return '提现金额最低 1.00 元，且最多两位小数。'
  if (amountIsValid.value && balance.value + 1e-9 < totalDeducted.value) return '余额不足以覆盖提现金额和手续费。'
  return ''
})

onMounted(() => {
  void load()
})

onBeforeUnmount(() => {
  revokeDraftPreview()
})

async function load() {
  loading.value = true
  try {
    const [alipay, wechat, list] = await Promise.all([
      userAPI.getReceiptCode('alipay'),
      userAPI.getReceiptCode('wechat'),
      userAPI.listWithdrawals({ page: 1, page_size: 5 }),
    ])
    receiptCodes.value.alipay = alipay
    receiptCodes.value.wechat = wechat
    withdrawals.value = list.items || []
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '提现信息加载失败'))
  } finally {
    loading.value = false
  }
}

function selectMethod(method: ReceiptCodePaymentMethod) {
  if (selectedMethod.value === method) {
    return
  }
  selectedMethod.value = method
  clearDraft()
}

function methodLabel(method: ReceiptCodePaymentMethod): string {
  return method === 'alipay' ? '支付宝' : '微信'
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
    appStore.showError('收款码必须是 PNG、JPEG、GIF 或 WebP 图片')
    return
  }
  if (file.size > 1024 * 1024) {
    appStore.showError('收款码图片不能超过 1MB')
    return
  }
  revokeDraftPreview()
  draftFile.value = file
  draftPreviewUrl.value = URL.createObjectURL(file)
}

async function handleSave() {
  if (!draftFile.value) {
    appStore.showError('请先选择收款码图片')
    return
  }

  const method = selectedMethod.value
  saving.value = true
  try {
    receiptCodes.value[method] = await userAPI.uploadReceiptCode(method, draftFile.value)
    clearDraft()
    appStore.showSuccess('收款码已保存')
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '收款码保存失败'))
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  if (draftFile.value) {
    clearDraft()
    return
  }
  if (!currentReceiptCode.value) {
    return
  }

  const method = selectedMethod.value
  saving.value = true
  try {
    await userAPI.deleteReceiptCode(method)
    receiptCodes.value[method] = null
    appStore.showSuccess('收款码已删除')
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '收款码删除失败'))
  } finally {
    saving.value = false
  }
}

async function submit() {
  if (!canSubmit.value) {
    appStore.showError(submitHint.value || '请确认金额、余额和收款码后再提交')
    return
  }
  submitting.value = true
  try {
    await userAPI.submitWithdrawal({
      amount: normalizedAmount.value,
      payment_method: selectedMethod.value,
    })
    amountText.value = ''
    await Promise.all([load(), authStore.refreshUser()])
    appStore.showSuccess('提现申请已提交')
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '提现申请提交失败'))
  } finally {
    submitting.value = false
  }
}

async function cancel(id: number) {
  actionLoading.value = true
  try {
    await userAPI.cancelWithdrawal(id)
    await Promise.all([load(), authStore.refreshUser()])
    appStore.showSuccess('提现申请已取消')
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '取消提现失败'))
  } finally {
    actionLoading.value = false
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

function statusLabel(status: WithdrawalStatus): string {
  const map: Record<WithdrawalStatus, string> = {
    PENDING: '待结算',
    SETTLED: '已结算',
    CANCELLED: '已取消',
    REJECTED: '已拒绝',
  }
  return map[status]
}

function statusClass(status: WithdrawalStatus): string {
  const map: Record<WithdrawalStatus, string> = {
    PENDING: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300',
    SETTLED: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300',
    CANCELLED: 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300',
    REJECTED: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300',
  }
  return map[status]
}

function formatDate(raw: string): string {
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }
  return date.toLocaleString()
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
