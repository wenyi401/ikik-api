<template>
  <AppLayout>
    <UiPage width="wide" density="compact">
      <section class="orders-panel">
        <div class="orders-toolbar">
          <Select v-model="currentFilter" :options="statusFilters" class="w-36" @change="fetchOrders" />
          <div class="flex flex-1 items-center justify-end gap-2">
            <button @click="fetchOrders" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="router.push('/purchase')">{{ t('payment.result.backToRecharge') }}</button>
          </div>
        </div>

        <div class="orders-table">
          <OrderTable :orders="orders" :loading="loading">
            <template #actions="{ row }">
              <div class="flex flex-wrap items-center gap-2">
                <button v-if="canViewStoreCards(row)" @click="openStoreOrderDialog(row)" class="inline-flex min-h-[2.25rem] items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-[var(--ui-text-secondary)] hover:bg-[var(--ui-surface-subtle)] hover:text-[var(--ui-text)]">
                  <Icon name="key" size="sm" />
                  <span>{{ t('payment.orders.viewCards') }}</span>
                </button>
                <button v-if="row.status === 'PENDING'" @click="handleCancel(row.id)" class="inline-flex min-h-[2.25rem] items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-[var(--ui-warning)] hover:bg-[var(--ui-warning-soft)]">
                  <Icon name="x" size="sm" />
                  <span>{{ t('payment.orders.cancel') }}</span>
                </button>
                <button v-if="canRequestRefund(row)" @click="openRefundDialog(row)" class="inline-flex min-h-[2.25rem] items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-[var(--ui-text-secondary)] hover:bg-[var(--ui-surface-subtle)] hover:text-[var(--ui-text)]">
                  <Icon name="dollar" size="sm" />
                  <span>{{ t('payment.orders.requestRefund') }}</span>
                </button>
              </div>
            </template>
          </OrderTable>
        </div>
      </section>

      <!-- Pagination -->
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </UiPage>

    <!-- Cancel Confirm Dialog -->
    <BaseDialog :show="!!cancelTargetId" :title="t('payment.orders.cancel')" width="narrow" @close="cancelTargetId = null">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('payment.confirmCancel') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="cancelTargetId = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger" :disabled="actionLoading" @click="confirmCancel">{{ actionLoading ? t('common.processing') : t('payment.orders.cancel') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Store Delivery Dialog -->
    <BaseDialog :show="!!storeOrderTarget" :title="t('payment.orders.storeDelivery')" width="wide" @close="closeStoreOrderDialog">
      <div class="space-y-4">
        <div v-if="storeOrderLoading" class="flex items-center justify-center py-8 text-sm text-gray-500 dark:text-gray-400">
          <Icon name="refresh" size="md" class="mr-2 animate-spin" />
          <span>{{ t('payment.orders.loadingCards') }}</span>
        </div>
        <template v-else-if="storeOrderDetail">
          <div class="rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800">
            <div class="grid gap-3 sm:grid-cols-2">
              <div>
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.shopOrderNo') }}</div>
                <div class="mt-1 break-all font-mono text-gray-900 dark:text-white">{{ storeOrderDetail.order_no }}</div>
              </div>
              <div>
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.status') }}</div>
                <div class="mt-1 text-gray-900 dark:text-white">{{ storeOrderDetail.status }}</div>
              </div>
              <div>
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('store.product') }}</div>
                <div class="mt-1 text-gray-900 dark:text-white">{{ storeOrderDetail.product_name }}</div>
              </div>
              <div>
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('store.quantity') }}</div>
                <div class="mt-1 text-gray-900 dark:text-white">{{ storeOrderDetail.quantity }}</div>
              </div>
            </div>
          </div>

          <div v-if="storeOrderDetail.draw_reward_amount !== null && storeOrderDetail.draw_reward_amount !== undefined" class="rounded-lg bg-emerald-50 p-4 text-sm dark:bg-emerald-950/30">
            <div class="flex justify-between gap-3">
              <span class="text-emerald-700 dark:text-emerald-300">{{ t('store.drawReward') }}</span>
              <span class="font-semibold text-emerald-800 dark:text-emerald-200">{{ formatStoreDrawReward(storeOrderDetail) }}</span>
            </div>
          </div>

          <div v-if="storeOrderDetail.delivered_cards.length > 0 && (storeOrderDetail.draw_reward_amount === null || storeOrderDetail.draw_reward_amount === undefined)">
            <div class="mb-2 flex flex-wrap items-center justify-between gap-3">
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('store.deliveredCards') }}</label>
              <button
                type="button"
                class="btn btn-secondary btn-sm min-h-[2.25rem]"
                @click="copyStoreCards"
              >
                <Icon name="copy" size="sm" />
                <span>{{ t('common.copy') }}</span>
              </button>
            </div>
            <div class="max-h-72 space-y-2 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
              <code
                v-for="(card, index) in storeOrderDetail.delivered_cards"
                :key="index"
                class="block break-all rounded-md bg-white px-3 py-2 font-mono text-xs text-gray-900 dark:bg-dark-900 dark:text-dark-100"
              >
                {{ card }}
              </code>
            </div>
          </div>
          <DeliveredFilesList
            v-if="storeOrderDetail.delivered_files.length > 0"
            :order-id="storeOrderDetail.id"
            :files="storeOrderDetail.delivered_files"
          />
          <p v-if="storeOrderDetail.delivered_cards.length === 0 && storeOrderDetail.delivered_files.length === 0 && (storeOrderDetail.draw_reward_amount === null || storeOrderDetail.draw_reward_amount === undefined)" class="rounded-lg bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-800 dark:text-dark-400">
            {{ t('store.deliveryPending') }}
          </p>
        </template>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button class="btn btn-primary min-h-[2.75rem]" @click="closeStoreOrderDialog">{{ t('common.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Refund Dialog -->
    <BaseDialog :show="!!refundTarget" :title="t('payment.orders.requestRefund')" @close="refundTarget = null">
      <div v-if="refundTarget" class="space-y-4">
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">#{{ refundTarget.id }}</span>
          </div>
          <div class="mt-2 flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.amount') }}</span>
            <span class="text-gray-900 dark:text-white">${{ refundTarget.amount.toFixed(2) }}</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('payment.refundReason') }}</label>
          <textarea v-model="refundReason" rows="3" class="input mt-1 w-full" :placeholder="t('payment.refundReasonPlaceholder')" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="refundTarget = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="actionLoading || !refundReason.trim()" @click="confirmRefund">{{ actionLoading ? t('common.processing') : t('payment.orders.requestRefund') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { storeAPI } from '@/api/store'
import { useClipboard } from '@/composables/useClipboard'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatStoreDrawReward } from '@/utils/storeRewards'
import type { PaymentOrder } from '@/types/payment'
import type { StoreOrder } from '@/types/store'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderTable from '@/components/payment/OrderTable.vue'
import DeliveredFilesList from '@/components/store/DeliveredFilesList.vue'
import { UiPage } from '@/ui'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const actionLoading = ref(false)
const orders = ref<PaymentOrder[]>([])
const refundEligibleProviders = ref<Set<string>>(new Set())
const currentFilter = ref('')
const cancelTargetId = ref<number | null>(null)
const refundTarget = ref<PaymentOrder | null>(null)
const refundReason = ref('')
const storeOrderTarget = ref<PaymentOrder | null>(null)
const storeOrderDetail = ref<StoreOrder | null>(null)
const storeOrderLoading = ref(false)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const statusFilters = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'FAILED', label: t('payment.status.failed') },
  { value: 'REFUNDED', label: t('payment.status.refunded') },
])

async function fetchOrders() {
  loading.value = true
  try {
    const res = await paymentAPI.getMyOrders({
      page: pagination.page,
      page_size: pagination.page_size,
      status: currentFilter.value || undefined,
    })
    orders.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) { pagination.page = page; fetchOrders() }
function handlePageSizeChange(size: number) { pagination.page_size = size; pagination.page = 1; fetchOrders() }

function handleCancel(orderId: number) { cancelTargetId.value = orderId }

function canViewStoreCards(order: PaymentOrder): order is PaymentOrder & { shop_order_id: number } {
  return order.order_type === 'shop' && typeof order.shop_order_id === 'number' && order.shop_order_id > 0
}

async function openStoreOrderDialog(order: PaymentOrder): Promise<void> {
  if (!canViewStoreCards(order)) return
  storeOrderTarget.value = order
  storeOrderDetail.value = null
  storeOrderLoading.value = true
  try {
    const res = await storeAPI.getOrder(order.shop_order_id)
    storeOrderDetail.value = res.data
  } catch (err: unknown) {
    storeOrderTarget.value = null
    appStore.showError(extractI18nErrorMessage(err, t, 'store.errors', t('common.error')))
  } finally {
    storeOrderLoading.value = false
  }
}

function closeStoreOrderDialog() {
  storeOrderTarget.value = null
  storeOrderDetail.value = null
  storeOrderLoading.value = false
}

async function copyStoreCards(): Promise<void> {
  const cards = storeOrderDetail.value?.delivered_cards || []
  if (cards.length === 0) return
  await copyToClipboard(cards.join('\n'), t('store.cardsCopied'))
}

async function confirmCancel() {
  if (!cancelTargetId.value) return
  actionLoading.value = true
  try {
    await paymentAPI.cancelOrder(cancelTargetId.value)
    appStore.showSuccess(t('common.success'))
    cancelTargetId.value = null
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

function openRefundDialog(order: PaymentOrder) { refundTarget.value = order; refundReason.value = '' }

async function confirmRefund() {
  if (!refundTarget.value || !refundReason.value.trim()) return
  actionLoading.value = true
  try {
    await paymentAPI.requestRefund(refundTarget.value.id, { reason: refundReason.value.trim() })
    appStore.showSuccess(t('common.success'))
    refundTarget.value = null
    refundReason.value = ''
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

function canRequestRefund(order: PaymentOrder): boolean {
  if (order.status !== 'COMPLETED') return false
  if (!order.provider_instance_id) return false
  return refundEligibleProviders.value.has(order.provider_instance_id)
}

async function loadRefundEligibility() {
  try {
    const res = await paymentAPI.getRefundEligibleProviders()
    refundEligibleProviders.value = new Set(res.data.provider_instance_ids || [])
  } catch { /* ignore — default to hiding refund button */ }
}

onMounted(() => { fetchOrders(); loadRefundEligibility() })
</script>

<style scoped>
.orders-panel {
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.orders-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--ui-border);
}

.orders-table :deep(.table-wrapper) {
  border: 0;
  border-radius: 0;
}

@media (max-width: 640px) {
  .orders-toolbar {
    padding: 0.75rem;
  }

  .orders-toolbar > div {
    width: auto;
  }

  .orders-table {
    padding: 0.75rem;
  }

  .orders-table :deep(.mobile-data-card) {
    padding: 0.875rem 0;
    border: 0;
    border-bottom: 1px solid var(--ui-border);
    border-radius: 0;
    background: transparent;
    box-shadow: none;
  }

  .orders-table :deep(.mobile-data-card:last-child) {
    border-bottom: 0;
  }
}
</style>
