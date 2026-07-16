<template>
  <AppLayout>
    <div class="space-y-4">
      <!-- Filters -->
      <div class="pb-1">
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-64">
            <input v-model="orderSearch" type="text" :placeholder="t('payment.admin.searchOrders')" class="input" @input="debounceLoadOrders" />
          </div>
          <Select v-model="orderFilters.status" :options="statusFilterOptions" class="w-36" @change="loadOrders" />
          <Select v-model="orderFilters.payment_type" :options="paymentTypeFilterOptions" class="w-40" @change="loadOrders" />
          <Select v-model="orderFilters.order_type" :options="orderTypeFilterOptions" class="w-36" @change="loadOrders" />
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <UiIconButton :label="t('common.refresh')" @click="loadOrders" :disabled="ordersLoading">
              <Icon name="refresh" size="md" :class="ordersLoading ? 'animate-spin' : ''" />
            </UiIconButton>
          </div>
        </div>
      </div>

      <!-- Table -->
      <OrderTable :orders="orders" :loading="ordersLoading" show-user>
        <template #actions="{ row }">
          <div class="flex items-center gap-1">
            <button @click="showOrderDetail(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-600">
              <Icon name="eye" size="sm" />
              {{ t('common.view') }}
            </button>
            <button v-if="canViewShopDelivery(row)" @click="showOrderDetail(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-primary-600 hover:bg-primary-50 dark:text-primary-400 dark:hover:bg-primary-900/20">
              <Icon name="key" size="sm" />
              {{ t('payment.orders.viewCards') }}
            </button>
            <button v-if="row.status === 'PENDING'" @click="handleCancelOrder(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-yellow-600 hover:bg-yellow-50 dark:text-yellow-400 dark:hover:bg-yellow-900/20">
              <Icon name="x" size="sm" />
              {{ t('payment.orders.cancel') }}
            </button>
            <button v-if="canRetryFulfillment(row)" @click="handleRetryOrder(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/20">
              <Icon name="refresh" size="sm" />
              {{ t('payment.admin.retry') }}
            </button>
            <button v-if="canManualFulfill(row)" @click="openManualFulfillDialog(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-emerald-600 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-900/20">
              <Icon name="plus" size="sm" />
              {{ t('payment.admin.manualFulfill') }}
            </button>
            <template v-if="row.status === 'REFUND_REQUESTED'">
              <span v-if="row.refund_amount" class="rounded-full bg-purple-100 px-1.5 py-0.5 text-xs font-medium text-purple-700 dark:bg-purple-900/30 dark:text-purple-300">{{ row.order_type === 'balance' ? '$' : '¥' }}{{ row.refund_amount.toFixed(2) }}</span>
              <button @click="openRefundDialog(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-purple-600 hover:bg-purple-50 dark:text-purple-400 dark:hover:bg-purple-900/20">
                <Icon name="check" size="sm" />
                {{ t('payment.admin.approveRefund') }}
              </button>
            </template>
            <button v-else-if="row.status === 'REFUND_FAILED'" @click="openRefundDialog(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-purple-600 hover:bg-purple-50 dark:text-purple-400 dark:hover:bg-purple-900/20">
              <Icon name="refresh" size="sm" />
              {{ t('payment.admin.retryRefund') }}
            </button>
            <button v-else-if="row.status === 'REFUND_PENDING'" @click="handleQueryRefund(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-orange-600 hover:bg-orange-50 dark:text-orange-400 dark:hover:bg-orange-900/20">
              <Icon name="refresh" size="sm" />
              {{ t('payment.admin.queryRefund') }}
            </button>
            <button v-else-if="row.status === 'COMPLETED' || row.status === 'PARTIALLY_REFUNDED'" @click="openRefundDialog(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20">
              <Icon name="dollar" size="sm" />
              {{ t('payment.admin.refund') }}
            </button>
          </div>
        </template>
      </OrderTable>
      <Pagination v-if="orderPagination.total > 0" :page="orderPagination.page" :total="orderPagination.total" :page-size="orderPagination.page_size" @update:page="handleOrderPageChange" @update:pageSize="handleOrderPageSizeChange" />
    </div>

    <!-- Order Detail Dialog -->
    <BaseDialog :show="showDetailDialog" :title="t('payment.admin.orderDetail')" width="wide" @close="showDetailDialog = false">
      <div v-if="selectedOrder" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</p><p class="font-mono text-sm font-medium text-gray-900 dark:text-white">#{{ selectedOrder.id }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.paymentOrderNo') }}</p><p class="text-sm font-medium text-gray-900 dark:text-white">{{ selectedOrder.out_trade_no }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.status') }}</p><OrderStatusBadge :status="selectedOrder.status" /></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.amount') }}</p><p class="text-sm font-medium text-gray-900 dark:text-white">{{ selectedOrder.order_type === 'balance' ? '$' : '¥' }}{{ selectedOrder.amount.toFixed(2) }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</p><p class="text-sm font-medium text-gray-900 dark:text-white">¥{{ selectedOrder.pay_amount.toFixed(2) }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.paymentMethod') }}</p><p class="text-sm text-gray-700 dark:text-gray-300">{{ t('payment.methods.' + selectedOrder.payment_type, selectedOrder.payment_type) }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.feeRate') }}</p><p class="text-sm text-gray-700 dark:text-gray-300">{{ selectedOrder.fee_rate }}%</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.createdAt') }}</p><p class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(selectedOrder.created_at) }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.expiresAt') }}</p><p class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(selectedOrder.expires_at) }}</p></div>
          <div v-if="selectedOrder.paid_at"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.paidAt') }}</p><p class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(selectedOrder.paid_at) }}</p></div>
          <div v-if="selectedOrder.completed_at"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.completedAt') }}</p><p class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(selectedOrder.completed_at) }}</p></div>
          <div v-if="selectedOrder.failed_reason" class="col-span-2"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.failedReason') }}</p><p class="text-sm text-red-600 dark:text-red-400">{{ selectedOrder.failed_reason }}</p></div>
          <div v-if="selectedOrder.refund_amount"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.refundAmount') }}</p><p class="text-sm font-medium text-red-600 dark:text-red-400">{{ selectedOrder.order_type === 'balance' ? '$' : '¥' }}{{ selectedOrder.refund_amount.toFixed(2) }}</p></div>
          <div v-if="selectedOrder.refund_reason" class="col-span-2"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.refundReason') }}</p><p class="text-sm text-gray-700 dark:text-gray-300">{{ selectedOrder.refund_reason }}</p></div>
          <!-- Refund request info -->
          <div v-if="selectedOrder.refund_requested_at" class="col-span-2 border-t border-gray-200 pt-3 dark:border-dark-600">
            <p class="mb-2 text-xs font-medium text-purple-600 dark:text-purple-400">{{ t('payment.admin.refundRequestInfo') }}</p>
            <div class="grid grid-cols-2 gap-4">
              <div>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.refundRequestedAt') }}</p>
                <p class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(selectedOrder.refund_requested_at) }}</p>
              </div>
              <div>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.refundRequestedBy') }}</p>
                <p class="text-sm text-gray-700 dark:text-gray-300">#{{ selectedOrder.refund_requested_by }}</p>
              </div>
              <div class="col-span-2">
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.refundRequestReason') }}</p>
                <p class="text-sm text-gray-700 dark:text-gray-300">{{ selectedOrder.refund_request_reason }}</p>
              </div>
            </div>
          </div>
        </div>
        <!-- Store Delivery -->
        <div v-if="selectedShopOrder" class="border-t border-gray-200 pt-4 dark:border-dark-600">
          <div class="mb-3 grid gap-3 rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800 sm:grid-cols-2">
            <div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.shopOrderNo') }}</p>
              <p class="mt-1 break-all font-mono text-gray-900 dark:text-white">{{ selectedShopOrder.order_no }}</p>
            </div>
            <div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('store.product') }}</p>
              <p class="mt-1 text-gray-900 dark:text-white">{{ selectedShopOrder.product_name }} x {{ selectedShopOrder.quantity }}</p>
            </div>
          </div>
          <div v-if="selectedShopOrder.draw_reward_amount !== null && selectedShopOrder.draw_reward_amount !== undefined" class="mb-4 rounded-lg bg-emerald-50 p-4 text-sm dark:bg-emerald-950/30">
            <div class="flex justify-between gap-3">
              <span class="text-emerald-700 dark:text-emerald-300">{{ t('store.drawReward') }}</span>
              <span class="font-semibold text-emerald-800 dark:text-emerald-200">{{ formatStoreDrawReward(selectedShopOrder) }}</span>
            </div>
          </div>

          <div v-if="selectedShopOrder.delivered_cards.length > 0 && (selectedShopOrder.draw_reward_amount === null || selectedShopOrder.draw_reward_amount === undefined)" class="mb-4">
            <div class="mb-2 flex flex-wrap items-center justify-between gap-3">
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('store.deliveredCards') }}</label>
              <button type="button" class="btn btn-secondary btn-sm min-h-[2.25rem]" @click="copySelectedShopCards">
                <Icon name="copy" size="sm" />
                <span>{{ t('common.copy') }}</span>
              </button>
            </div>
            <div class="max-h-72 space-y-2 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
              <code
                v-for="(card, index) in selectedShopOrder.delivered_cards"
                :key="index"
                class="block break-all rounded-md bg-white px-3 py-2 font-mono text-xs text-gray-900 dark:bg-dark-900 dark:text-dark-100"
              >
                {{ card }}
              </code>
            </div>
          </div>
          <DeliveredFilesList
            v-if="selectedShopOrder.delivered_files.length > 0"
            :order-id="selectedShopOrder.id"
            :files="selectedShopOrder.delivered_files"
            :download-file="downloadAdminStoreFile"
            :download-all-files="downloadAdminStoreFilesZip"
          />
          <p v-if="selectedShopOrder.delivered_cards.length === 0 && selectedShopOrder.delivered_files.length === 0 && (selectedShopOrder.draw_reward_amount === null || selectedShopOrder.draw_reward_amount === undefined)" class="rounded-lg bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-800 dark:text-dark-400">
            {{ t('store.deliveryPending') }}
          </p>
        </div>
        <!-- Audit Logs -->
        <div v-if="orderAuditLogs.length > 0" class="border-t border-gray-200 pt-4 dark:border-dark-600">
          <p class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('payment.admin.auditLogs') }}</p>
          <div class="max-h-48 space-y-2 overflow-y-auto">
            <div v-for="log in orderAuditLogs" :key="log.id" class="rounded-lg border border-gray-100 bg-gray-50 p-2.5 dark:border-dark-600 dark:bg-dark-800">
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{{ log.action }}</span>
                <span class="text-xs text-gray-400">{{ formatDateTime(log.created_at) }}</span>
              </div>
              <div v-if="log.detail" class="mt-1 break-all text-xs text-gray-500 dark:text-gray-400">{{ log.detail }}</div>
              <div v-if="log.operator" class="mt-1 text-xs text-gray-400">{{ t('payment.admin.operator') }}: {{ log.operator }}</div>
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="showManualFulfillDialog" :title="t('payment.admin.manualFulfillOrder')" width="normal" @close="closeManualFulfillDialog">
      <form id="manual-fulfill-form" class="space-y-4" @submit.prevent="handleManualFulfill">
        <div v-if="manualFulfillTarget" class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800">
          <div class="flex justify-between gap-3">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">#{{ manualFulfillTarget.id }}</span>
          </div>
          <div class="mt-1 flex justify-between gap-3">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</span>
            <span class="font-medium text-gray-900 dark:text-white">¥{{ manualFulfillTarget.pay_amount.toFixed(2) }}</span>
          </div>
          <div class="mt-1 flex justify-between gap-3">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.status') }}</span>
            <OrderStatusBadge :status="manualFulfillTarget.status" />
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('payment.admin.manualPaidAmount') }}</label>
          <input v-model="manualFulfillForm.paid_amount" type="number" step="0.01" min="0.01" class="input" :placeholder="manualFulfillTarget ? manualFulfillTarget.pay_amount.toFixed(2) : ''" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.admin.manualTradeNo') }}</label>
          <input v-model.trim="manualFulfillForm.trade_no" type="text" maxlength="128" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.admin.manualReason') }}</label>
          <textarea v-model.trim="manualFulfillForm.reason" rows="3" maxlength="500" class="input" required></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeManualFulfillDialog">{{ t('common.cancel') }}</button>
          <button type="submit" form="manual-fulfill-form" class="btn btn-primary" :disabled="manualFulfillSubmitting || !manualFulfillForm.reason.trim() || !manualPaidAmountValid">
            {{ manualFulfillSubmitting ? t('common.processing') : t('payment.admin.confirmManualFulfill') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <AdminRefundDialog :show="showRefundDialog" :order="selectedOrder" :submitting="refundSubmitting" @confirm="handleRefund" @cancel="showRefundDialog = false" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { adminStoreAPI } from '@/api/admin/store'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatStoreDrawReward } from '@/utils/storeRewards'
import { formatOrderDateTime } from '@/components/payment/orderUtils'
import { useClipboard } from '@/composables/useClipboard'
import type { AdminPaymentOrderDetail, PaymentOrder, PaymentOrderAuditLog } from '@/types/payment'
import type { StoreOrder } from '@/types/store'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import AdminRefundDialog from '@/components/admin/payment/AdminRefundDialog.vue'
import OrderStatusBadge from '@/components/payment/OrderStatusBadge.vue'
import OrderTable from '@/components/payment/OrderTable.vue'
import DeliveredFilesList from '@/components/store/DeliveredFilesList.vue'
import { UiIconButton } from '@/ui'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const ordersLoading = ref(false)
const orders = ref<PaymentOrder[]>([])
const orderSearch = ref('')
const orderFilters = reactive({ status: '', payment_type: '', order_type: '' })
const orderPagination = reactive({ page: 1, page_size: 20, total: 0 })
const selectedOrder = ref<PaymentOrder | null>(null)
const selectedShopOrder = ref<StoreOrder | null>(null)
const showDetailDialog = ref(false)
const showRefundDialog = ref(false)
const refundSubmitting = ref(false)
const orderAuditLogs = ref<PaymentOrderAuditLog[]>([])
const showManualFulfillDialog = ref(false)
const manualFulfillSubmitting = ref(false)
const manualFulfillTarget = ref<PaymentOrder | null>(null)
const manualFulfillForm = reactive({ paid_amount: '', trade_no: '', reason: '' })

let debounceTimer: ReturnType<typeof setTimeout> | null = null
function debounceLoadOrders() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => loadOrders(), 300)
}

async function loadOrders() {
  ordersLoading.value = true
  try {
    const res = await adminPaymentAPI.getOrders({
      page: orderPagination.page, page_size: orderPagination.page_size,
      keyword: orderSearch.value || undefined, status: orderFilters.status || undefined,
      payment_type: orderFilters.payment_type || undefined, order_type: orderFilters.order_type || undefined,
    })
    orders.value = res.data.items || []
    orderPagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally { ordersLoading.value = false }
}

function handleOrderPageChange(page: number) { orderPagination.page = page; loadOrders() }
function handleOrderPageSizeChange(size: number) { orderPagination.page_size = size; orderPagination.page = 1; loadOrders() }

const statusFilterOptions = computed(() => [
  { value: '', label: t('payment.admin.allStatuses') },
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'PAID', label: t('payment.status.paid') },
  { value: 'RECHARGING', label: t('payment.status.recharging') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'EXPIRED', label: t('payment.status.expired') },
  { value: 'CANCELLED', label: t('payment.status.cancelled') },
  { value: 'FAILED', label: t('payment.status.failed') },
  { value: 'REFUND_REQUESTED', label: t('payment.status.refund_requested') },
  { value: 'REFUNDING', label: t('payment.status.refunding') },
  { value: 'REFUND_PENDING', label: t('payment.status.refund_pending') },
  { value: 'PARTIALLY_REFUNDED', label: t('payment.status.partially_refunded') },
  { value: 'REFUNDED', label: t('payment.status.refunded') },
  { value: 'REFUND_FAILED', label: t('payment.status.refund_failed') },
])

const paymentTypeFilterOptions = computed(() => [
  { value: '', label: t('payment.admin.allPaymentTypes') },
  { value: 'alipay', label: t('payment.methods.alipay') },
  { value: 'wxpay', label: t('payment.methods.wxpay') },
  { value: 'stripe', label: t('payment.methods.stripe') },
])

const orderTypeFilterOptions = computed(() => [
  { value: '', label: t('payment.admin.allOrderTypes') },
  { value: 'balance', label: t('payment.admin.balanceOrder') },
  { value: 'subscription', label: t('payment.admin.subscriptionOrder') },
  { value: 'shop', label: t('payment.admin.shopOrder') },
])

async function showOrderDetail(order: PaymentOrder) {
  selectedOrder.value = order
  selectedShopOrder.value = null
  orderAuditLogs.value = []
  showDetailDialog.value = true
  try {
    const res = await adminPaymentAPI.getOrder(order.id)
    const data = res.data as AdminPaymentOrderDetail
    if (data.order) selectedOrder.value = data.order
    selectedShopOrder.value = data.shop_order || null
    orderAuditLogs.value = data.auditLogs || data.audit_logs || []
  } catch (_err: unknown) { /* keep cached order data */ }
}

async function handleCancelOrder(order: PaymentOrder) {
  try { await adminPaymentAPI.cancelOrder(order.id); appStore.showSuccess(t('payment.admin.orderCancelled')); loadOrders() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

async function handleRetryOrder(order: PaymentOrder) {
  try { await adminPaymentAPI.retryRecharge(order.id); appStore.showSuccess(t('payment.admin.retrySuccess')); loadOrders() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

function canRetryFulfillment(order: PaymentOrder): boolean {
  return order.status === 'PAID' || (order.status === 'FAILED' && !!order.paid_at)
}

function canManualFulfill(order: PaymentOrder): boolean {
  return ['PENDING', 'EXPIRED', 'CANCELLED', 'FAILED', 'RECHARGING'].includes(order.status)
}

function canViewShopDelivery(order: PaymentOrder): boolean {
  return order.order_type === 'shop' && typeof order.shop_order_id === 'number' && order.shop_order_id > 0
}

function openManualFulfillDialog(order: PaymentOrder) {
  manualFulfillTarget.value = order
  manualFulfillForm.paid_amount = ''
  manualFulfillForm.trade_no = ''
  manualFulfillForm.reason = ''
  showManualFulfillDialog.value = true
}

function closeManualFulfillDialog() {
  showManualFulfillDialog.value = false
  manualFulfillTarget.value = null
}

const manualPaidAmountValid = computed(() => {
  const raw = manualFulfillForm.paid_amount.trim()
  if (!raw) return true
  const amount = Number(raw)
  return Number.isFinite(amount) && amount > 0
})

async function handleManualFulfill() {
  if (!manualFulfillTarget.value || !manualFulfillForm.reason.trim() || !manualPaidAmountValid.value) return
  manualFulfillSubmitting.value = true
  try {
    const rawAmount = manualFulfillForm.paid_amount.trim()
    await adminPaymentAPI.manualFulfillOrder(manualFulfillTarget.value.id, {
      reason: manualFulfillForm.reason.trim(),
      paid_amount: rawAmount ? Number(rawAmount) : undefined,
      trade_no: manualFulfillForm.trade_no.trim() || undefined,
    })
    appStore.showSuccess(t('payment.admin.manualFulfillSuccess'))
    closeManualFulfillDialog()
    await loadOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    manualFulfillSubmitting.value = false
  }
}

async function copySelectedShopCards(): Promise<void> {
  const cards = selectedShopOrder.value?.delivered_cards || []
  if (cards.length === 0) return
  await copyToClipboard(cards.join('\n'), t('store.cardsCopied'))
}

async function downloadAdminStoreFile(orderId: number, cardId: number, filename: string): Promise<void> {
  await adminStoreAPI.downloadOrderFile(orderId, cardId, filename)
}

async function downloadAdminStoreFilesZip(orderId: number): Promise<void> {
  await adminStoreAPI.downloadOrderFilesZip(orderId)
}

function openRefundDialog(order: PaymentOrder) { selectedOrder.value = order; showRefundDialog.value = true }

async function handleRefund(data: { amount: number; reason: string; deduct_balance: boolean; force: boolean }) {
  if (!selectedOrder.value) return
  refundSubmitting.value = true
  try {
    await adminPaymentAPI.refundOrder(selectedOrder.value.id, { amount: data.amount, reason: data.reason, deduct_balance: data.deduct_balance, force: data.force })
    appStore.showSuccess(t('payment.admin.refundSuccess')); showRefundDialog.value = false; loadOrders()
  } catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { refundSubmitting.value = false }
}

async function handleQueryRefund(order: PaymentOrder) {
  try {
    await adminPaymentAPI.queryRefund(order.id)
    appStore.showSuccess(t('payment.admin.queryRefundSuccess'))
    loadOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

function formatDateTime(dateStr: string): string { return formatOrderDateTime(dateStr) }

onMounted(() => loadOrders())
</script>
