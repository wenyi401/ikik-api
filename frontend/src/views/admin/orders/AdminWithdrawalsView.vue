<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="pb-1">
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-72">
            <input v-model="keyword" type="text" placeholder="搜索邮箱或提现单号" class="input" @input="debounceLoad" />
          </div>
          <Select v-model="filters.status" :options="statusOptions" class="w-36" @change="load" />
          <Select v-model="filters.payment_method" :options="methodOptions" class="w-36" @change="load" />
          <div class="ml-auto flex items-center gap-2">
            <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="load">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </UiIconButton>
          </div>
        </div>
      </div>

      <DataTable :columns="columns" :data="items" :loading="loading">
        <template #cell-id="{ value }">
          <span class="font-mono text-sm">#{{ value }}</span>
        </template>
        <template #cell-user_email="{ value, row }">
          <div class="text-sm">
            <p class="font-medium text-gray-900 dark:text-white">{{ value }}</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">UID {{ row.user_id }}</p>
          </div>
        </template>
        <template #cell-amount="{ value, row }">
          <div class="text-sm">
            <p class="font-semibold text-gray-900 dark:text-white">${{ value.toFixed(2) }}</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">扣除 ${{ row.total_deducted.toFixed(2) }}</p>
          </div>
        </template>
        <template #cell-payment_method="{ value }">
          <span class="text-sm text-gray-700 dark:text-gray-300">{{ methodLabel(value) }}</span>
        </template>
        <template #cell-receipt_code_url="{ row }">
          <button class="btn btn-secondary btn-sm" @click="openReceipt(row)">查看收款码</button>
        </template>
        <template #cell-status="{ value }">
          <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(value)">
            {{ statusLabel(value) }}
          </span>
        </template>
        <template #cell-created_at="{ value }">
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDate(value) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex flex-wrap items-center gap-1">
            <button class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-600" @click="openDetail(row)">
              <Icon name="eye" size="sm" />
              查看
            </button>
            <button v-if="row.status === 'PENDING'" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-green-600 hover:bg-green-50 dark:text-green-400 dark:hover:bg-green-900/20" @click="openProcess(row, 'settle')">
              <Icon name="check" size="sm" />
              确认打款
            </button>
            <button v-if="row.status === 'PENDING'" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20" @click="openProcess(row, 'reject')">
              <Icon name="x" size="sm" />
              拒绝
            </button>
          </div>
        </template>
      </DataTable>

      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>

    <BaseDialog :show="!!receiptTarget" title="收款码快照" width="narrow" @close="closeReceipt">
      <div v-if="receiptTarget" class="space-y-4">
        <div class="flex min-h-64 items-center justify-center overflow-hidden rounded-xl border border-gray-100 bg-gray-50 dark:border-dark-700 dark:bg-dark-900/50">
          <div v-if="receiptLoading" class="flex flex-col items-center gap-2 text-gray-400 dark:text-gray-500">
            <Icon name="refresh" size="lg" class="animate-spin" />
            <span class="text-sm">图片加载中</span>
          </div>
          <img
            v-else-if="receiptImageUrl && !receiptImageFailed"
            :src="receiptImageUrl"
            class="h-auto max-h-[70vh] w-full object-contain"
            alt="收款码"
            @error="receiptImageFailed = true"
          >
          <div v-else class="flex flex-col items-center gap-2 px-6 py-10 text-center text-gray-400 dark:text-gray-500">
            <Icon name="exclamationCircle" size="lg" />
            <span class="text-sm">收款码图片暂不可访问</span>
          </div>
        </div>
        <div class="text-xs text-gray-500 dark:text-gray-400">
          <p>方式：{{ methodLabel(receiptTarget.payment_method) }}</p>
          <p class="break-all">SHA256：{{ receiptTarget.receipt_code_sha256 }}</p>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="!!detailTarget" title="提现详情" width="wide" @close="detailTarget = null">
      <div v-if="detailTarget" class="grid gap-4 sm:grid-cols-2">
        <InfoItem label="提现单号" :value="'#' + detailTarget.id" />
        <InfoItem label="状态" :value="statusLabel(detailTarget.status)" />
        <InfoItem label="用户邮箱" :value="detailTarget.user_email" />
        <InfoItem label="用户ID" :value="String(detailTarget.user_id)" />
        <InfoItem label="提现金额" :value="'$' + detailTarget.amount.toFixed(2)" />
        <InfoItem label="手续费" :value="'$' + detailTarget.fee_amount.toFixed(2)" />
        <InfoItem label="申请前余额" :value="'$' + detailTarget.balance_before.toFixed(2)" />
        <InfoItem label="申请后余额" :value="'$' + detailTarget.balance_after.toFixed(2)" />
        <InfoItem label="申请时间" :value="formatDate(detailTarget.created_at)" />
        <InfoItem label="处理时间" :value="detailTarget.processed_at ? formatDate(detailTarget.processed_at) : '-'" />
        <InfoItem class="sm:col-span-2" label="备注" :value="detailTarget.admin_note || detailTarget.user_cancel_reason || '-'" />
      </div>
    </BaseDialog>

    <BaseDialog :show="!!processTarget" :title="processAction === 'settle' ? '确认已打款' : '拒绝提现'" width="narrow" @close="processTarget = null">
      <div v-if="processTarget" class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-300">
          {{ processAction === 'settle' ? '确认已向用户收款码完成打款。' : '拒绝后会自动退回本次扣除金额。' }}
        </p>
        <textarea v-model="processNote" rows="3" class="input" placeholder="备注"></textarea>
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" @click="processTarget = null">取消</button>
          <button class="btn" :class="processAction === 'settle' ? 'btn-primary' : 'btn-danger'" :disabled="processing" @click="submitProcess">
            {{ processing ? '处理中' : '确认' }}
          </button>
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { ReceiptCodePaymentMethod, WithdrawalRequest, WithdrawalStatus } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import { UiIconButton } from '@/ui'

const InfoItem = defineComponent({
  props: { label: { type: String, required: true }, value: { type: String, required: true } },
  setup(props) {
    return () => h('div', [
      h('p', { class: 'text-xs text-gray-500 dark:text-gray-400' }, props.label),
      h('p', { class: 'mt-1 break-words text-sm font-medium text-gray-900 dark:text-white' }, props.value),
    ])
  },
})

const { t } = useI18n()
const appStore = useAppStore()
const items = ref<WithdrawalRequest[]>([])
const loading = ref(false)
const processing = ref(false)
const keyword = ref('')
const filters = reactive({ status: '', payment_method: '' as ReceiptCodePaymentMethod | '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const receiptTarget = ref<WithdrawalRequest | null>(null)
const receiptLoading = ref(false)
const receiptImageFailed = ref(false)
const detailTarget = ref<WithdrawalRequest | null>(null)
const processTarget = ref<WithdrawalRequest | null>(null)
const processAction = ref<'settle' | 'reject'>('settle')
const processNote = ref('')
let debounceTimer: ReturnType<typeof setTimeout> | null = null
let receiptRequestSeq = 0

const columns = computed<Column[]>(() => [
  { key: 'id', label: '提现单号' },
  { key: 'user_email', label: '用户' },
  { key: 'amount', label: '金额' },
  { key: 'payment_method', label: '收款方式' },
  { key: 'receipt_code_url', label: '收款码' },
  { key: 'status', label: '状态' },
  { key: 'created_at', label: '申请时间' },
  { key: 'actions', label: '操作' },
])

const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'PENDING', label: '待结算' },
  { value: 'SETTLED', label: '已结算' },
  { value: 'CANCELLED', label: '已取消' },
  { value: 'REJECTED', label: '已拒绝' },
]

const methodOptions = [
  { value: '', label: '全部方式' },
  { value: 'alipay', label: '支付宝' },
  { value: 'wechat', label: '微信' },
]

const receiptImageUrl = computed(() => receiptTarget.value?.receipt_code_url?.trim() || '')

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => load(), 300)
}

async function load() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getWithdrawals({
      page: pagination.page,
      page_size: pagination.page_size,
      keyword: keyword.value || undefined,
      status: filters.status || undefined,
      payment_method: filters.payment_method || undefined,
    })
    items.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '提现列表加载失败'))
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  void load()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void load()
}

async function openReceipt(row: WithdrawalRequest) {
  const requestSeq = ++receiptRequestSeq
  receiptTarget.value = row
  receiptImageFailed.value = false
  receiptLoading.value = true
  try {
    const res = await adminPaymentAPI.getWithdrawal(row.id)
    if (requestSeq !== receiptRequestSeq) {
      return
    }
    receiptTarget.value = res.data
    const index = items.value.findIndex(item => item.id === res.data.id)
    if (index >= 0) {
      items.value.splice(index, 1, res.data)
    }
  } catch (error: unknown) {
    if (requestSeq === receiptRequestSeq) {
      appStore.showError(extractApiErrorMessage(error, '收款码快照加载失败'))
    }
  } finally {
    if (requestSeq === receiptRequestSeq) {
      receiptLoading.value = false
    }
  }
}

function closeReceipt() {
  receiptRequestSeq++
  receiptTarget.value = null
  receiptLoading.value = false
  receiptImageFailed.value = false
}

function openDetail(row: WithdrawalRequest) {
  detailTarget.value = row
}

function openProcess(row: WithdrawalRequest, action: 'settle' | 'reject') {
  processTarget.value = row
  processAction.value = action
  processNote.value = ''
}

async function submitProcess() {
  if (!processTarget.value) return
  processing.value = true
  try {
    if (processAction.value === 'settle') {
      await adminPaymentAPI.settleWithdrawal(processTarget.value.id, { note: processNote.value })
      appStore.showSuccess('提现已结算')
    } else {
      await adminPaymentAPI.rejectWithdrawal(processTarget.value.id, { note: processNote.value })
      appStore.showSuccess('提现已拒绝并退回余额')
    }
    processTarget.value = null
    await load()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '提现处理失败'))
  } finally {
    processing.value = false
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

function methodLabel(method: ReceiptCodePaymentMethod): string {
  return method === 'alipay' ? '支付宝' : '微信'
}

function formatDate(raw: string): string {
  return new Date(raw).toLocaleString()
}

void load()
</script>
