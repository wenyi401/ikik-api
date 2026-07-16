<template>
  <div class="flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-dark-900">
    <div class="w-full max-w-md space-y-6">
      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      </div>
      <template v-else>
        <!-- Status Icon -->
        <div class="text-center">
          <div v-if="isSuccess"
            class="mx-auto flex h-20 w-20 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
            <svg class="h-10 w-10 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"
              stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
            </svg>
          </div>
          <div v-else-if="isPending"
            class="mx-auto flex h-20 w-20 items-center justify-center rounded-full bg-yellow-100 dark:bg-yellow-900/30">
            <div class="h-10 w-10 animate-spin rounded-full border-4 border-yellow-500 border-t-transparent"></div>
          </div>
          <div v-else
            class="mx-auto flex h-20 w-20 items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
            <svg class="h-10 w-10 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </div>
          <h2 class="mt-4 text-2xl font-bold text-gray-900 dark:text-white">
            {{ statusTitle }}
          </h2>
          <p v-if="isPending" class="mt-2 text-sm text-gray-500 dark:text-gray-400">
            {{ t('payment.result.processingHint') }}
          </p>
        </div>
        <!-- Order Info -->
        <div v-if="order" class="rounded-xl bg-white p-5 shadow-sm dark:bg-dark-800">
          <div class="space-y-3 text-sm">
            <div v-if="hasOrderId(order)" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
              <span class="font-medium text-gray-900 dark:text-white">#{{ order.id }}</span>
            </div>
            <div v-if="order.out_trade_no" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.paymentOrderNo') }}</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ order.out_trade_no }}</span>
            </div>
            <div v-if="hasAmountFields(order)" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.baseAmount') }}</span>
              <span class="font-medium text-gray-900 dark:text-white">&#165;{{ baseAmount.toFixed(2) }}</span>
            </div>
            <div v-if="hasAmountFields(order) && order.fee_rate > 0" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.fee') }} ({{ order.fee_rate }}%)</span>
              <span class="font-medium text-gray-900 dark:text-white">&#165;{{ feeAmount.toFixed(2) }}</span>
            </div>
            <div v-if="hasAmountFields(order)" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</span>
              <span class="font-bold text-primary-600 dark:text-primary-400">&#165;{{ order.pay_amount.toFixed(2) }}</span>
            </div>
            <div v-if="hasAmountFields(order) && order.amount !== order.pay_amount" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.creditedAmount') }}</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ order.order_type === 'balance' ? '$' : '¥' }}{{ order.amount.toFixed(2) }}</span>
            </div>
            <div v-if="hasPaymentType(order)" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.paymentMethod') }}</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ t(paymentMethodI18nKey(order.payment_type), normalizedOrderPaymentType(order.payment_type)) }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.status') }}</span>
              <OrderStatusBadge :status="displayOrderStatus(order.status)" />
            </div>
          </div>
        </div>
        <!-- Store Delivery -->
        <div
          v-if="isShopPayment && (shopOrder || loadingShopOrder)"
          class="rounded-xl bg-white p-5 shadow-sm dark:bg-dark-800"
        >
          <div v-if="loadingShopOrder && !shopOrder" class="flex items-center justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
          </div>
          <div v-else-if="shopOrder" class="space-y-4">
            <div class="space-y-2 text-sm">
              <div class="flex justify-between gap-3">
                <span class="text-gray-500 dark:text-gray-400">{{ t('store.product') }}</span>
                <span class="text-right font-medium text-gray-900 dark:text-white">{{ shopOrder.product_name }}</span>
              </div>
              <div class="flex justify-between gap-3">
                <span class="text-gray-500 dark:text-gray-400">{{ t('store.quantity') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ shopOrder.quantity }}</span>
              </div>
            </div>

            <div v-if="shopOrder.draw_reward_amount !== null && shopOrder.draw_reward_amount !== undefined" class="rounded-lg bg-emerald-50 p-4 text-sm dark:bg-emerald-950/30">
              <div class="flex justify-between gap-3">
                <span class="text-emerald-700 dark:text-emerald-300">{{ t('store.drawReward') }}</span>
                <span class="font-semibold text-emerald-800 dark:text-emerald-200">{{ formatStoreDrawReward(shopOrder) }}</span>
              </div>
            </div>

            <div v-if="shopOrder.delivered_cards.length > 0 && (shopOrder.draw_reward_amount === null || shopOrder.draw_reward_amount === undefined)">
              <div class="mb-2 flex items-center justify-between gap-3">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('store.deliveredCards') }}</span>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  @click="copyShopDeliveredCards"
                >
                  {{ t('common.copy') }}
                </button>
              </div>
              <div class="max-h-72 space-y-2 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900">
                <code
                  v-for="(card, index) in shopOrder.delivered_cards"
                  :key="index"
                  class="block break-all rounded-md bg-white px-3 py-2 font-mono text-xs text-gray-900 dark:bg-dark-800 dark:text-dark-100"
                >
                  {{ card }}
                </code>
              </div>
            </div>
            <DeliveredFilesList
              v-if="shopOrder.delivered_files.length > 0"
              :order-id="shopOrder.id"
              :files="shopOrder.delivered_files"
            />
            <p v-if="shopOrder.delivered_cards.length === 0 && shopOrder.delivered_files.length === 0 && (shopOrder.draw_reward_amount === null || shopOrder.draw_reward_amount === undefined)" class="rounded-lg bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-900 dark:text-dark-400">
              {{ t('store.deliveryPending') }}
            </p>
          </div>
        </div>
        <!-- EasyPay return info (when no order loaded) -->
        <div v-else-if="returnInfo" class="rounded-xl bg-white p-5 shadow-sm dark:bg-dark-800">
          <div class="space-y-3 text-sm">
            <div v-if="returnInfo.outTradeNo" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ returnInfo.outTradeNo }}</span>
            </div>
            <div v-if="returnInfo.money" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.payAmount') }}</span>
              <span class="font-medium text-gray-900 dark:text-white">&#165;{{ returnInfo.money }}</span>
            </div>
            <div v-if="returnInfo.type" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.paymentMethod') }}</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ t(paymentMethodI18nKey(returnInfo.type), normalizedOrderPaymentType(returnInfo.type)) }}</span>
            </div>
          </div>
        </div>
        <!-- Actions -->
        <div class="flex gap-3">
          <button class="btn btn-secondary flex-1" @click="router.push(resultBackPath)">{{ resultBackLabel }}</button>
          <button class="btn btn-primary flex-1" @click="router.push('/orders')">{{ t('payment.result.viewOrders') }}</button>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import OrderStatusBadge from '@/components/payment/OrderStatusBadge.vue'
import DeliveredFilesList from '@/components/store/DeliveredFilesList.vue'
import {
  PAYMENT_RECOVERY_STORAGE_KEY,
  clearPaymentRecoverySnapshot,
  readPaymentRecoverySnapshotFromStorage,
} from '@/components/payment/paymentFlow'
import { usePaymentStore } from '@/stores/payment'
import { paymentAPI, type PublicOrderVerifyResult } from '@/api/payment'
import { storeAPI } from '@/api/store'
import { formatStoreDrawReward } from '@/utils/storeRewards'
import type { OrderStatus, PaymentOrder } from '@/types/payment'
import type { StoreOrder } from '@/types/store'
import { normalizePaymentMethodForDisplay, paymentMethodI18nKey } from './paymentUx'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const paymentStore = usePaymentStore()

type ResolvedOrder = PaymentOrder | PublicOrderVerifyResult

const order = ref<ResolvedOrder | null>(null)
const shopOrder = ref<StoreOrder | null>(null)
const loading = ref(true)
const loadingShopOrder = ref(false)

interface ReturnInfo {
  outTradeNo: string
  money: string
  type: string
  tradeStatus: string
}
const returnInfo = ref<ReturnInfo | null>(null)

const SUCCESS_STATUSES = new Set(['COMPLETED', 'PAID', 'RECHARGING'])
const PENDING_STATUSES = new Set(['PENDING', 'CREATED', 'WAITING', 'PROCESSING'])
const STATUS_REFRESH_INTERVAL_MS = 2000
const STATUS_REFRESH_MAX_ATTEMPTS = 15

let statusRefreshTimer: ReturnType<typeof setTimeout> | null = null
const refreshAttempts = ref(0)

/** 充值金额 = pay_amount / (1 + fee_rate/100)，fee_rate=0 时等于 pay_amount */
const baseAmount = computed(() => {
  if (!hasAmountFields(order.value) || order.value.fee_rate <= 0) return hasAmountFields(order.value) ? order.value.pay_amount : 0
  return Math.round((order.value.pay_amount / (1 + order.value.fee_rate / 100)) * 100) / 100
})

/** 手续费 = pay_amount - baseAmount */
const feeAmount = computed(() => {
  if (!hasAmountFields(order.value) || order.value.fee_rate <= 0) return 0
  return Math.round((order.value.pay_amount - baseAmount.value) * 100) / 100
})

const isSuccess = computed(() => {
  return isSuccessStatus(order.value?.status)
})

const isPending = computed(() => {
  return isPendingStatus(order.value?.status)
})

const statusTitle = computed(() => {
  if (isSuccess.value) {
    return t('payment.result.success')
  }
  if (isPending.value) {
    return t('payment.result.processing')
  }
  return t('payment.result.failed')
})
const isShopPayment = computed(() => hasOrderType(order.value) && order.value.order_type === 'shop')
const resultBackPath = computed(() => isShopPayment.value ? '/store' : '/purchase')
const resultBackLabel = computed(() => isShopPayment.value ? t('nav.store') : t('payment.result.backToRecharge'))

function normalizedOrderPaymentType(paymentType: string): string {
  return normalizePaymentMethodForDisplay(paymentType || '') || paymentType || ''
}

function hasOrderId(nextOrder: ResolvedOrder | null): nextOrder is PaymentOrder {
  return !!nextOrder && 'id' in nextOrder && typeof nextOrder.id === 'number'
}

function hasAmountFields(nextOrder: ResolvedOrder | null): nextOrder is PaymentOrder {
  return !!nextOrder
    && 'pay_amount' in nextOrder
    && typeof nextOrder.pay_amount === 'number'
    && 'amount' in nextOrder
    && typeof nextOrder.amount === 'number'
}

function hasPaymentType(nextOrder: ResolvedOrder | null): nextOrder is PaymentOrder {
  return !!nextOrder && 'payment_type' in nextOrder && typeof nextOrder.payment_type === 'string' && nextOrder.payment_type.trim() !== ''
}

function hasOrderType(nextOrder: ResolvedOrder | null): nextOrder is PaymentOrder {
  return !!nextOrder && 'order_type' in nextOrder && typeof nextOrder.order_type === 'string'
}

function normalizeOrderStatus(status: string | null | undefined): string {
  return String(status || '').trim().toUpperCase()
}

function displayOrderStatus(status: string): OrderStatus {
  return normalizeOrderStatus(status) as OrderStatus
}

function isSuccessStatus(status: string | null | undefined): boolean {
  return SUCCESS_STATUSES.has(normalizeOrderStatus(status))
}

function isPendingStatus(status: string | null | undefined): boolean {
  return PENDING_STATUSES.has(normalizeOrderStatus(status))
}

function hasLocalAuthToken(): boolean {
  if (typeof window === 'undefined') return false
  return !!window.localStorage.getItem('auth_token')
}

function shouldLoadShopOrder(paymentOrder: PaymentOrder | null): paymentOrder is PaymentOrder & { shop_order_id: number } {
  return hasLocalAuthToken()
    && paymentOrder?.order_type === 'shop'
    && typeof paymentOrder.shop_order_id === 'number'
    && paymentOrder.shop_order_id > 0
}

function shouldRefreshShopDelivery(): boolean {
  const currentOrder = hasOrderId(order.value) ? order.value : null
  if (!shouldLoadShopOrder(currentOrder) || !isSuccessStatus(currentOrder.status)) {
    return false
  }
  if (!shopOrder.value) {
    return true
  }
  return (shopOrder.value.status === 'pending' || shopOrder.value.status === 'paid')
    && shopOrder.value.delivered_cards.length === 0
    && shopOrder.value.delivered_files.length === 0
}

async function loadShopOrder(paymentOrder: PaymentOrder | null = hasOrderId(order.value) ? order.value : null): Promise<void> {
  if (!shouldLoadShopOrder(paymentOrder)) {
    shopOrder.value = null
    return
  }

  loadingShopOrder.value = true
  try {
    const { data } = await storeAPI.getOrder(paymentOrder.shop_order_id)
    shopOrder.value = data
  } catch (_err: unknown) {
    shopOrder.value = null
  } finally {
    loadingShopOrder.value = false
  }
}

async function copyShopDeliveredCards(): Promise<void> {
  const cards = shopOrder.value?.delivered_cards || []
  if (cards.length === 0) return
  await copyText(cards.join('\n'))
}

async function copyText(text: string): Promise<void> {
  if (!text || typeof window === 'undefined') return
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return
    } catch {
      // Continue with the DOM copy path below.
    }
  }

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.cssText = 'position:fixed;left:-9999px;top:-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  try {
    document.execCommand('copy')
  } finally {
    document.body.removeChild(textarea)
  }
}

function readRouteQueryString(key: string): string {
  const value = route.query[key]
  if (Array.isArray(value)) {
    return typeof value[0] === 'string' ? value[0] : ''
  }
  return typeof value === 'string' ? value : ''
}

function restoreRecoverySnapshot(context: {
  resumeToken: string
  routeOrderId: number
  routeOutTradeNo: string
}) {
  if (typeof window === 'undefined') {
    return null
  }

  if (context.resumeToken) {
    return readPaymentRecoverySnapshotFromStorage(window.localStorage, {
      resumeToken: context.resumeToken,
    }, PAYMENT_RECOVERY_STORAGE_KEY)
  }

  if (!context.routeOrderId && !context.routeOutTradeNo) {
    return null
  }

  const restored = readPaymentRecoverySnapshotFromStorage(window.localStorage, {
    orderId: context.routeOrderId,
    outTradeNo: context.routeOutTradeNo,
  }, PAYMENT_RECOVERY_STORAGE_KEY)
  if (!restored) {
    return null
  }

  if (context.routeOrderId > 0 && restored.orderId !== context.routeOrderId) {
    return null
  }

  if (context.routeOutTradeNo && restored.outTradeNo !== context.routeOutTradeNo) {
    return null
  }

  return restored
}

async function resolveOrderFromResumeToken(resumeToken: string): Promise<ResolvedOrder | null> {
  try {
    const result = await paymentAPI.resolveOrderPublicByResumeToken(resumeToken)
    return result.data
  } catch (_err: unknown) {
    return null
  }
}

async function resolveOrderFromOutTradeNo(outTradeNo: string): Promise<ResolvedOrder | null> {
  try {
    const result = await paymentAPI.verifyOrderPublic(outTradeNo)
    return result.data
  } catch (_err: unknown) {
    return null
  }
}

function clearStatusRefreshTimer(): void {
  if (statusRefreshTimer !== null) {
    clearTimeout(statusRefreshTimer)
    statusRefreshTimer = null
  }
}

function clearRecoverySnapshot(): void {
  if (typeof window === 'undefined') return
  const routeOrderId = Number(readRouteQueryString('order_id')) || 0
  clearPaymentRecoverySnapshot(window.localStorage, PAYMENT_RECOVERY_STORAGE_KEY, {
    resumeToken: readRouteQueryString('resume_token'),
    orderId: hasOrderId(order.value) ? order.value.id : routeOrderId,
    outTradeNo: order.value?.out_trade_no || returnInfo.value?.outTradeNo || readRouteQueryString('out_trade_no'),
  })
}

function clearRecoverySnapshotForTerminalStatus(status: string | null | undefined): void {
  if (!status) return
  if (!isPendingStatus(status)) {
    clearRecoverySnapshot()
  }
}

function scheduleStatusRefresh(refreshOrder: (() => Promise<ResolvedOrder | null>) | null): void {
  clearStatusRefreshTimer()
  if (!refreshOrder || (!isPending.value && !shouldRefreshShopDelivery()) || refreshAttempts.value >= STATUS_REFRESH_MAX_ATTEMPTS) {
    return
  }

  statusRefreshTimer = setTimeout(async () => {
    refreshAttempts.value += 1
    const refreshedOrder = await refreshOrder()
    if (refreshedOrder) {
      order.value = refreshedOrder
      await loadShopOrder(hasOrderId(refreshedOrder) ? refreshedOrder : null)
      clearRecoverySnapshotForTerminalStatus(refreshedOrder.status)
    }

    if (isPendingStatus(order.value?.status) || shouldRefreshShopDelivery()) {
      scheduleStatusRefresh(refreshOrder)
    }
  }, STATUS_REFRESH_INTERVAL_MS)
}

onMounted(async () => {
  const resumeToken = readRouteQueryString('resume_token')
  const routeOrderId = Number(readRouteQueryString('order_id')) || 0
  let outTradeNo = readRouteQueryString('out_trade_no')
  let orderId = 0
  let resumeTokenLookupFailed = false

  const restored = restoreRecoverySnapshot({
    resumeToken,
    routeOrderId,
    routeOutTradeNo: outTradeNo,
  })
  if (restored?.orderId) {
    orderId = restored.orderId
  }
  if (!outTradeNo && restored?.outTradeNo) {
    outTradeNo = restored.outTradeNo
  }

  if (resumeToken) {
    const resolvedOrder = await resolveOrderFromResumeToken(resumeToken)
    if (resolvedOrder) {
      order.value = resolvedOrder
      if (!orderId) {
        orderId = hasOrderId(resolvedOrder) ? resolvedOrder.id : 0
      }
    } else if (routeOrderId > 0) {
      resumeTokenLookupFailed = true
      orderId = routeOrderId
    } else {
      resumeTokenLookupFailed = true
    }
  } else if (routeOrderId > 0) {
    orderId = routeOrderId
  }

  const hasLegacyFallbackContext = readRouteQueryString('trade_status').trim() !== ''
  const shouldUsePublicOutTradeNo = outTradeNo !== '' && (hasLegacyFallbackContext || routeOrderId > 0 || orderId > 0)

  if (!order.value && orderId && (!resumeToken || routeOrderId > 0)) {
    try {
      order.value = await paymentStore.pollOrderStatus(orderId)
    } catch (_err: unknown) {
      // Order lookup failed, will try legacy fallback below when possible.
    }
  }

  if (!order.value && shouldUsePublicOutTradeNo && (!resumeToken || resumeTokenLookupFailed)) {
    const legacyOrder = await resolveOrderFromOutTradeNo(outTradeNo)
    if (legacyOrder) {
      order.value = legacyOrder
      if (!orderId) {
        orderId = hasOrderId(legacyOrder) ? legacyOrder.id : 0
      }
    }
  }

  if (!order.value && !orderId && outTradeNo && hasLegacyFallbackContext) {
    returnInfo.value = {
      outTradeNo,
      money: String(route.query.money || ''),
      type: String(route.query.type || ''),
      tradeStatus: String(route.query.trade_status || ''),
    }
  }

  const refreshOrder = async (): Promise<ResolvedOrder | null> => {
    if (resumeToken) {
      const resolvedOrder = await resolveOrderFromResumeToken(resumeToken)
      if (resolvedOrder) {
        return resolvedOrder
      }
    }

    if (orderId) {
      try {
        return await paymentStore.pollOrderStatus(orderId)
      } catch (_err: unknown) {
        // Fall through to legacy public verification when order polling is unavailable.
      }
    }

    if (shouldUsePublicOutTradeNo) {
      return await resolveOrderFromOutTradeNo(outTradeNo)
    }

    return null
  }

  await loadShopOrder(hasOrderId(order.value) ? order.value : null)
  if (isPendingStatus(order.value?.status) || shouldRefreshShopDelivery()) {
    scheduleStatusRefresh(refreshOrder)
  } else if (order.value) {
    clearRecoverySnapshotForTerminalStatus(order.value.status)
  } else if (returnInfo.value) {
    clearRecoverySnapshot()
  }
  loading.value = false
})

onBeforeUnmount(() => {
  clearStatusRefreshTimer()
})
</script>
