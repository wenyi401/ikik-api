<template>
  <AppLayout>
    <UiPage width="wide">
      <section class="store-summary">
        <UiMetricStrip :style="{ '--metric-columns': authStore.isAuthenticated ? 4 : 2 }">
          <UiMetric :label="t('store.productCount')" :value="products.length" />
          <UiMetric :label="t('store.availableStock')" :value="totalStock" />
          <UiMetric
            v-if="authStore.isAuthenticated"
            :label="t('payment.currentBalance')"
            :value="`$${currentBalance.toFixed(2)}`"
          />
          <UiMetric
            v-if="authStore.isAuthenticated"
            :label="t('store.currentPoints')"
            :value="formatPoints(currentPoints)"
          />
        </UiMetricStrip>
      </section>

      <div v-if="loading" class="flex justify-center py-16">
        <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      </div>

      <template v-else-if="paymentPhase === 'paying'">
        <PaymentStatusPanel
          :order-id="paymentState.orderId"
          :qr-code="paymentState.qrCode"
          :expires-at="paymentState.expiresAt"
          :payment-type="paymentState.paymentType"
          :pay-url="paymentState.payUrl"
          :order-type="paymentState.orderType"
          @done="resetPayment"
          @success="handlePaymentSuccess"
          @settled="handlePaymentSettled"
        />
      </template>

      <template v-else>
        <nav class="store-categories" :aria-label="t('store.title')">
          <button
            type="button"
            class="store-filter"
            :class="{ 'store-filter-active': selectedCategoryId === 0 }"
            @click="selectedCategoryId = 0"
          >
            {{ t('common.all') }}
          </button>
          <button
            v-for="category in activeCategories"
            :key="category.id"
            type="button"
            class="store-filter"
            :class="{ 'store-filter-active': selectedCategoryId === category.id }"
            @click="selectedCategoryId = category.id"
          >
            {{ category.name }}
          </button>
        </nav>

        <EmptyState v-if="filteredProducts.length === 0" :title="t('store.empty')" />

        <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <article
            v-for="product in filteredProducts"
            :key="product.id"
            class="store-product-card"
          >
            <div class="flex min-w-0 flex-1 flex-col">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="truncate text-xs font-medium text-[var(--ui-text-tertiary)]">
                    {{ product.category?.name || categoryName(product.category_id) }}
                  </p>
                  <h2 class="mt-1 line-clamp-2 text-lg font-semibold text-gray-900 dark:text-white">
                    {{ product.name }}
                  </h2>
                </div>
                <span class="shrink-0 text-xs font-medium text-[var(--ui-text-tertiary)]">
                  {{ product.stock_unlimited ? t('store.drawProductBadge') : t('store.stock', { count: product.stock }) }}
                </span>
              </div>
              <p class="mt-3 line-clamp-3 min-h-[3.75rem] text-sm leading-5 text-[var(--ui-text-secondary)]">
                {{ product.description || t('store.noDescription') }}
              </p>
              <div v-if="isDrawProduct(product)" class="store-draw-progress">
                <div class="flex justify-between gap-3">
                  <span class="text-[var(--ui-text-tertiary)]">{{ t('store.drawProgress') }}</span>
                  <span class="font-semibold text-[var(--ui-text)]">{{ drawProgressText(product) }}</span>
                </div>
              </div>
              <div class="mt-4 flex items-end justify-between gap-3">
                <div>
                  <span v-if="product.original_price" class="text-sm text-[var(--ui-text-tertiary)] line-through">
                    ¥{{ product.original_price.toFixed(2) }}
                  </span>
                  <div class="text-2xl font-semibold text-[var(--ui-text)]">¥{{ product.price.toFixed(2) }}</div>
                </div>
                <button
                  type="button"
                  class="btn btn-primary min-h-[44px] px-4"
                  :disabled="!isProductPurchasable(product)"
                  @click="startCheckout(product)"
                >
                  {{ isProductPurchasable(product) ? t('store.buyNow') : t('store.soldOut') }}
                </button>
              </div>
            </div>
          </article>
        </div>
      </template>
    </UiPage>

    <Teleport to="body">
      <Transition name="modal">
        <div v-if="checkoutProduct" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="closeCheckout">
          <div class="store-dialog w-full max-w-lg p-5">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ checkoutProduct.name }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('store.checkoutTitle') }}</p>
              </div>
              <button type="button" class="store-close-button" :title="t('common.close')" @click="closeCheckout">
                <span class="sr-only">{{ t('common.close') }}</span>
                <Icon name="x" size="md" />
              </button>
            </div>

            <div class="mt-5 space-y-4">
              <div>
                <label class="input-label">{{ t('store.quantity') }}</label>
                <input v-model.number="quantity" type="number" :min="checkoutMinQuantity" :max="checkoutMaxQuantity" class="input" />
              </div>

              <div class="store-order-summary">
                <div class="flex justify-between">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('store.unitPrice') }}</span>
                  <span class="font-medium text-gray-900 dark:text-white">¥{{ checkoutProduct.price.toFixed(2) }}</span>
                </div>
                <div class="mt-2 flex justify-between">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('store.totalAmount') }}</span>
                  <span class="text-lg font-semibold text-[var(--ui-text)]">¥{{ checkoutAmount.toFixed(2) }}</span>
                </div>
                <div v-if="isCheckoutDrawProduct" class="mt-2 flex justify-between gap-3">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('store.drawRewardRange') }}</span>
                  <span class="text-right font-medium text-gray-900 dark:text-white">
                    {{ drawRewardRangeText }}
                  </span>
                </div>
                <div v-if="isCheckoutDrawProduct" class="mt-2 flex justify-between gap-3">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('store.drawProgress') }}</span>
                  <span class="text-right font-medium text-gray-900 dark:text-white">
                    {{ drawProgressText(checkoutProduct) }}
                  </span>
                </div>
                <div v-if="authStore.isAuthenticated && checkoutProduct.allow_balance_payment !== false" class="mt-2 flex justify-between">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('payment.currentBalance') }}</span>
                  <span class="font-medium text-gray-900 dark:text-white">${{ currentBalance.toFixed(2) }}</span>
                </div>
                <div v-if="authStore.isAuthenticated && checkoutProduct.allow_points_payment" class="mt-2 flex justify-between">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('store.currentPoints') }}</span>
                  <span class="font-medium text-gray-900 dark:text-white">{{ currentPoints.toFixed(10).replace(/\.?0+$/, '') || '0' }}</span>
                </div>
              </div>

              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('store.payMethod') }}
                </label>
                <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
                  <button
                    v-if="isBalancePaymentAllowed"
                    type="button"
                    class="store-pay-option"
                    :class="{ 'store-pay-option-active': payMethod === 'balance' }"
                    :disabled="!canPayByBalance"
                    @click="payMethod = 'balance'"
                  >
                    <span class="font-semibold">{{ t('store.balancePay') }}</span>
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ balancePayHint }}</span>
                  </button>
                  <button
                    v-if="isPointsPaymentAllowed"
                    type="button"
                    class="store-pay-option"
                    :class="{ 'store-pay-option-active': payMethod === 'points' }"
                    :disabled="!canPayByPoints"
                    @click="payMethod = 'points'"
                  >
                    <span class="font-semibold">{{ t('store.pointsPay') }}</span>
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ pointsPayHint }}</span>
                  </button>
                  <button
                    v-if="isPlatformPaymentAllowed"
                    type="button"
                    class="store-pay-option"
                    :class="{ 'store-pay-option-active': payMethod === 'payment' }"
                    :disabled="methodOptions.length === 0"
                    @click="payMethod = 'payment'"
                  >
                    <span class="font-semibold">{{ t('store.gatewayPay') }}</span>
                    <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('store.gatewayPayHint') }}</span>
                  </button>
                </div>
              </div>

              <PaymentMethodSelector
                v-if="payMethod === 'payment' && methodOptions.length > 0"
                :methods="methodOptions"
                :selected="selectedMethod"
                @select="selectedMethod = $event"
              />

              <button
                type="button"
                class="btn btn-primary w-full min-h-[44px]"
                :disabled="!canSubmitCheckout || submitting"
                @click="submitCheckout"
              >
                {{ submitting ? t('common.processing') : t('store.confirmBuy') }}
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <Transition name="modal">
        <div v-if="completedOrder" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="closeCompletedOrder">
          <div class="store-dialog w-full max-w-lg p-5">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('store.purchaseSuccess') }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  {{ t('store.deliveryReady', { orderNo: completedOrder.order_no }) }}
                </p>
              </div>
              <button type="button" class="store-close-button" :title="t('common.close')" @click="closeCompletedOrder">
                <span class="sr-only">{{ t('common.close') }}</span>
                <Icon name="x" size="md" />
              </button>
            </div>

            <div class="mt-5 space-y-4">
              <div class="store-order-summary">
                <div class="flex justify-between gap-3">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('store.product') }}</span>
                  <span class="text-right font-medium text-gray-900 dark:text-white">{{ completedOrder.product_name }}</span>
                </div>
                <div class="mt-2 flex justify-between gap-3">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('store.quantity') }}</span>
                  <span class="font-medium text-gray-900 dark:text-white">{{ completedOrder.quantity }}</span>
                </div>
                <div v-if="completedOrder.draw_reward_amount !== null && completedOrder.draw_reward_amount !== undefined" class="mt-2 flex justify-between gap-3">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('store.drawReward') }}</span>
                  <span class="font-medium text-emerald-600 dark:text-emerald-300">
                    {{ formatDrawReward(completedOrder) }}
                  </span>
                </div>
              </div>

              <div v-if="completedOrder.draw_reward_amount === null || completedOrder.draw_reward_amount === undefined">
                <div class="mb-2 flex items-center justify-between gap-3">
                  <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('store.deliveredCards') }}</label>
                  <button
                    v-if="completedOrder.delivered_cards.length > 0"
                    type="button"
                    class="btn btn-secondary btn-sm"
                    @click="copyDeliveredCards"
                  >
                    {{ t('common.copy') }}
                  </button>
                </div>
                <div v-if="completedOrder.delivered_cards.length > 0" class="max-h-72 space-y-2 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
                  <code
                    v-for="(card, index) in completedOrder.delivered_cards"
                    :key="index"
                    class="block break-all rounded-md bg-white px-3 py-2 font-mono text-xs text-gray-900 dark:bg-dark-900 dark:text-dark-100"
                  >
                    {{ card }}
                  </code>
                </div>
                <p v-else class="rounded-lg bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                  {{ t('store.deliveryPending') }}
                </p>
              </div>

              <DeliveredFilesList
                v-if="completedOrder.delivered_files.length > 0"
                :order-id="completedOrder.id"
                :files="completedOrder.delivered_files"
              />

              <button type="button" class="btn btn-primary w-full min-h-[44px]" @click="closeCompletedOrder">
                {{ t('common.confirm') }}
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { paymentAPI } from '@/api/payment'
import { storeAPI } from '@/api/store'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useClipboard } from '@/composables/useClipboard'
import { isMobileDevice } from '@/utils/device'
import AppLayout from '@/components/layout/AppLayout.vue'
import DeliveredFilesList from '@/components/store/DeliveredFilesList.vue'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { UiMetric, UiMetricStrip, UiPage } from '@/ui'
import { METHOD_ORDER, getPaymentPopupFeatures } from '@/components/payment/providerConfig'
import { decidePaymentLaunch, getVisibleMethods, normalizeVisibleMethod, type PaymentRecoverySnapshot } from '@/components/payment/paymentFlow'
import type { CheckoutInfoResponse, CreateOrderResult } from '@/types/payment'
import type { PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'
import { formatStoreDrawReward } from '@/utils/storeRewards'
import type { StoreCategory, StoreDrawConfig, StoreOrder, StorePayMethod, StoreProduct } from '@/types/store'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const submitting = ref(false)
const categories = ref<StoreCategory[]>([])
const products = ref<StoreProduct[]>([])
const checkout = ref<CheckoutInfoResponse | null>(null)
const selectedCategoryId = ref(0)
const checkoutProduct = ref<StoreProduct | null>(null)
const completedOrder = ref<StoreOrder | null>(null)
const quantity = ref(1)
const payMethod = ref<StorePayMethod>('balance')
const selectedMethod = ref('')
const paymentPhase = ref<'select' | 'paying'>('select')
const paymentState = ref<PaymentRecoverySnapshot>(emptyPaymentState())
const activeShopOrderId = ref(0)
const checkoutIdempotencyKey = ref('')

const activeCategories = computed(() => categories.value.filter(category => category.enabled))
const filteredProducts = computed(() => products.value.filter((product) =>
  product.enabled && (selectedCategoryId.value === 0 || product.category_id === selectedCategoryId.value),
))
const totalStock = computed(() => products.value.reduce((sum, product) => sum + (product.stock_unlimited ? 0 : Math.max(0, product.stock)), 0))
const currentBalance = computed(() => Number(authStore.user?.balance || 0))
const currentPoints = computed(() => Number(authStore.user?.points_balance || 0))
const checkoutMinQuantity = computed(() => Math.max(1, checkoutProduct.value?.min_purchase || 1))
const checkoutMaxQuantity = computed(() => {
  const product = checkoutProduct.value
  if (!product) return 1
  if (isDrawProduct(product)) return 1
  const maxPurchase = product.max_purchase > 0 ? product.max_purchase : product.stock
  return Math.max(checkoutMinQuantity.value, Math.min(product.stock, maxPurchase))
})
const checkoutAmount = computed(() => {
  const product = checkoutProduct.value
  if (!product) return 0
  return Math.round(product.price * Math.max(checkoutMinQuantity.value, quantity.value || checkoutMinQuantity.value) * 100) / 100
})
const visibleMethods = computed(() => getVisibleMethods(checkout.value?.methods || {}))
const isCheckoutDrawProduct = computed(() => isDrawProduct(checkoutProduct.value || null))
const drawRewardRangeText = computed(() => {
  const config = checkoutProduct.value?.draw_config
  if (!config) return ''
  return formatDrawRewardRange(checkoutProduct.value, config)
})
const enabledMethods = computed(() => Object.keys(visibleMethods.value).sort((a, b) => {
  const ai = METHOD_ORDER.indexOf(a as typeof METHOD_ORDER[number])
  const bi = METHOD_ORDER.indexOf(b as typeof METHOD_ORDER[number])
  return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
}))
const methodOptions = computed<PaymentMethodOption[]>(() => enabledMethods.value.map((type) => ({
  type,
  fee_rate: visibleMethods.value[type]?.fee_rate ?? 0,
  available: visibleMethods.value[type]?.available !== false && amountFitsMethod(checkoutAmount.value, type),
})))
const isBalancePaymentAllowed = computed(() => checkoutProduct.value?.allow_balance_payment !== false)
const isPointsPaymentAllowed = computed(() => checkoutProduct.value?.allow_points_payment === true)
const isPlatformPaymentAllowed = computed(() => checkoutProduct.value?.allow_platform_payment !== false)
const canPayByBalance = computed(() => isBalancePaymentAllowed.value && currentBalance.value >= checkoutAmount.value && checkoutAmount.value > 0)
const canPayByPoints = computed(() => isPointsPaymentAllowed.value && currentPoints.value >= checkoutAmount.value && checkoutAmount.value > 0)
const balancePayHint = computed(() => {
  if (!isBalancePaymentAllowed.value) return t('store.paymentMethodUnavailable')
  return canPayByBalance.value ? t('store.balanceEnough') : t('store.balanceNotEnough')
})
const pointsPayHint = computed(() => canPayByPoints.value ? t('store.pointsEnough') : t('store.pointsNotEnough'))
const canSubmitCheckout = computed(() => {
  if (!checkoutProduct.value || quantity.value < checkoutMinQuantity.value || quantity.value > checkoutMaxQuantity.value) return false
  if (payMethod.value === 'balance') return canPayByBalance.value
  if (payMethod.value === 'points') return canPayByPoints.value
  return isPlatformPaymentAllowed.value && !!selectedMethod.value && amountFitsMethod(checkoutAmount.value, selectedMethod.value)
})

function formatPoints(value: number): string {
  return value.toFixed(10).replace(/\.?0+$/, '') || '0'
}

function emptyPaymentState(): PaymentRecoverySnapshot {
  return {
    orderId: 0,
    amount: 0,
    qrCode: '',
    expiresAt: '',
    paymentType: '',
    payUrl: '',
    outTradeNo: '',
    clientSecret: '',
    intentId: '',
    currency: '',
    countryCode: '',
    paymentEnv: '',
    payAmount: 0,
    orderType: '',
    paymentMode: '',
    resumeToken: '',
    createdAt: 0,
  }
}

function categoryName(id?: number | null): string {
  if (!id) return t('store.uncategorized')
  return categories.value.find(category => category.id === id)?.name || t('store.uncategorized')
}

function amountFitsMethod(amount: number, method: string): boolean {
  const limit = visibleMethods.value[method]
  if (!limit || limit.available === false || amount <= 0) return false
  if (limit.single_min > 0 && amount < limit.single_min) return false
  if (limit.single_max > 0 && amount > limit.single_max) return false
  return true
}

function isProductPurchasable(product: StoreProduct): boolean {
  return isDrawProduct(product) || product.stock > 0
}

function isDrawProduct(product: StoreProduct | null): boolean {
  return product?.product_type === 'balance_draw' || product?.product_type === 'points_draw'
}

function drawRewardUnit(productType?: string | null): string {
  return productType === 'points_draw' ? '' : '$'
}

function formatDrawReward(order: StoreOrder): string {
  return formatStoreDrawReward(order)
}

function formatDrawRewardRange(product: StoreProduct | null, config: StoreDrawConfig): string {
  if (product?.product_type === 'points_draw') {
    const min = config.min_amount.toFixed(10).replace(/\.?0+$/, '') || '0'
    const max = config.max_amount.toFixed(10).replace(/\.?0+$/, '') || '0'
    return `${min} - ${max}`
  }
  return `${drawRewardUnit(product?.product_type)}${config.min_amount.toFixed(2)} - ${drawRewardUnit(product?.product_type)}${config.max_amount.toFixed(2)}`
}

function drawProgressText(product: StoreProduct | null): string {
  if (!product?.draw_config) return '0/0'
  const progress = product.draw_progress
  const drawn = Math.max(0, progress?.drawn_count ?? 0)
  const total = Math.max(0, progress?.guarantee_count ?? product.draw_config.guarantee_count)
  return `${drawn}/${total}`
}

function createCheckoutIdempotencyKey(): string {
  const crypto = globalThis.crypto
  if (crypto?.randomUUID) {
    return crypto.randomUUID()
  }
  if (!crypto?.getRandomValues) {
    throw new Error('crypto.getRandomValues is required to create store orders')
  }

  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, '0'))
  return [
    hex.slice(0, 4).join(''),
    hex.slice(4, 6).join(''),
    hex.slice(6, 8).join(''),
    hex.slice(8, 10).join(''),
    hex.slice(10, 16).join(''),
  ].join('-')
}

async function startCheckout(product: StoreProduct) {
  if (!authStore.isAuthenticated) {
    router.push({ path: '/login', query: { redirect: '/store' } })
    return
  }
  try {
    await ensureCheckoutInfo()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'store.errors', t('common.error')))
    return
  }
  if (checkoutProduct.value?.id !== product.id || !checkoutIdempotencyKey.value) {
    checkoutIdempotencyKey.value = createCheckoutIdempotencyKey()
  }
  checkoutProduct.value = product
  quantity.value = Math.max(1, product.min_purchase || 1)
  if (canPayByBalance.value) {
    payMethod.value = 'balance'
  } else if (product.allow_points_payment && canPayByPoints.value) {
    payMethod.value = 'points'
  } else if (product.allow_platform_payment !== false) {
    payMethod.value = 'payment'
  } else if (product.allow_balance_payment !== false) {
    payMethod.value = 'balance'
  } else {
    payMethod.value = 'points'
  }
  if (!selectedMethod.value && enabledMethods.value.length > 0) {
    selectedMethod.value = enabledMethods.value[0]
  }
}

function closeCheckout() {
  if (submitting.value) return
  checkoutProduct.value = null
  checkoutIdempotencyKey.value = ''
}

function closeCompletedOrder() {
  completedOrder.value = null
}

function showCompletedOrder(order: StoreOrder) {
  completedOrder.value = order
}

async function copyDeliveredCards() {
  const cards = completedOrder.value?.delivered_cards || []
  if (cards.length === 0) return
  await copyToClipboard(cards.join('\n'), t('store.cardsCopied'))
}

async function submitCheckout() {
  if (!checkoutProduct.value || !canSubmitCheckout.value || submitting.value) return
  submitting.value = true
  try {
    const orderResponse = await storeAPI.createOrder({
      product_id: checkoutProduct.value.id,
      quantity: quantity.value,
      payment_method: payMethod.value === 'payment' ? selectedMethod.value : payMethod.value,
      return_url: `${window.location.origin}/payment/result`,
      payment_source: selectedMethod.value === 'wxpay' && /MicroMessenger/i.test(window.navigator.userAgent)
        ? 'wechat_in_app_resume'
        : 'hosted_redirect',
      is_mobile: isMobileDevice(),
    }, checkoutIdempotencyKey.value)
    const storeOrder = orderResponse.data
    if (payMethod.value === 'balance' || payMethod.value === 'points') {
      if (authStore.user) {
        const drawReward = Number(storeOrder.draw_reward_amount || 0)
        const rewardIsPoints = storeOrder.draw_reward_type === 'points' || storeOrder.product_type === 'points_draw'
        if (payMethod.value === 'balance') {
          authStore.user.balance = Math.max(0, currentBalance.value - storeOrder.total_amount + (rewardIsPoints ? 0 : drawReward))
          if (rewardIsPoints && drawReward > 0) {
            authStore.user.points_balance = currentPoints.value + drawReward
          }
        } else {
          authStore.user.points_balance = Math.max(0, currentPoints.value - storeOrder.total_amount + (rewardIsPoints ? drawReward : 0))
          if (!rewardIsPoints && drawReward > 0) {
            authStore.user.balance = currentBalance.value + drawReward
          }
        }
      }
      appStore.showSuccess(t('store.purchaseSuccess'))
      checkoutProduct.value = null
      checkoutIdempotencyKey.value = ''
      showCompletedOrder(storeOrder)
      await loadStore()
      return
    }

    launchPayment(storeOrder)
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'store.errors', t('common.error')))
  } finally {
    submitting.value = false
  }
}

function launchPayment(storeOrder: StoreOrder) {
  const result = storeOrder.payment as CreateOrderResult | null | undefined
  if (!result) {
    throw new Error(t('store.errors.UNHANDLED_PAYMENT_SCENARIO'))
  }
  activeShopOrderId.value = storeOrder.id
  const visibleMethod = normalizeVisibleMethod(result.payment_type || selectedMethod.value) || selectedMethod.value
  const stripeRouteUrl = result.client_secret
    ? router.resolve({
      path: '/payment/stripe',
      query: {
        order_id: String(result.order_id),
        client_secret: result.client_secret,
        method: visibleMethod === 'stripe' ? undefined : visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay',
        resume_token: result.resume_token || undefined,
      },
    }).href
    : ''
  const decision = decidePaymentLaunch(result, {
    visibleMethod,
    orderType: 'shop',
    isMobile: isMobileDevice(),
    isWechatBrowser: /MicroMessenger/i.test(window.navigator.userAgent),
    stripePopupUrl: stripeRouteUrl,
    stripeRouteUrl,
  })
  if (decision.kind === 'wechat_oauth' && decision.oauth?.authorize_url) {
    window.location.href = decision.oauth.authorize_url
    return
  }
  if (decision.kind === 'unhandled') {
    throw new Error(t('store.errors.UNHANDLED_PAYMENT_SCENARIO'))
  }
  paymentState.value = decision.paymentState
  paymentPhase.value = 'paying'
  checkoutProduct.value = null
  checkoutIdempotencyKey.value = ''
  if (decision.kind === 'stripe_popup' || decision.kind === 'redirect_waiting') {
    const url = decision.paymentState.payUrl
    if (url) {
      const win = window.open(url, 'paymentPopup', getPaymentPopupFeatures())
      if (!win || win.closed) window.location.href = url
    }
  }
  if (decision.kind === 'stripe_route') {
    window.location.href = decision.paymentState.payUrl
  }
}

function resetPayment() {
  paymentState.value = emptyPaymentState()
  paymentPhase.value = 'select'
  activeShopOrderId.value = 0
}

async function handlePaymentSuccess() {
  appStore.showSuccess(t('store.purchaseSuccess'))
  if (activeShopOrderId.value > 0) {
    try {
      const order = await waitForDeliveredOrder(activeShopOrderId.value)
      showCompletedOrder(order)
    } catch (err: unknown) {
      appStore.showError(extractI18nErrorMessage(err, t, 'store.errors', t('common.error')))
    }
  }
  await loadStore()
}

function handlePaymentSettled(outcome: string) {
  if (outcome !== 'success') {
    loadStore()
  }
}

async function loadStore() {
  const [categoriesResp, productsResp] = await Promise.all([storeAPI.getCategories(), storeAPI.getProducts()])
  categories.value = categoriesResp.data || []
  products.value = productsResp.data.items || []
  if (authStore.isAuthenticated) {
    await ensureCheckoutInfo()
    const { data } = await storeAPI.getDrawProgress()
    products.value = products.value.map(product => ({
      ...product,
      draw_progress: data?.[product.id] || product.draw_progress || null,
    }))
  }
  if (!selectedMethod.value && enabledMethods.value.length > 0) {
    selectedMethod.value = enabledMethods.value[0]
  }
}

async function ensureCheckoutInfo() {
  if (checkout.value) return
  const { data } = await paymentAPI.getCheckoutInfo()
  checkout.value = data
}

async function waitForDeliveredOrder(orderId: number): Promise<StoreOrder> {
  let latest: StoreOrder | null = null
  for (let attempt = 0; attempt < 5; attempt += 1) {
    const { data } = await storeAPI.getOrder(orderId)
    latest = data
    if (data.status === 'completed' && (data.delivered_cards.length > 0 || data.delivered_files.length > 0)) {
      return data
    }
    await new Promise(resolve => setTimeout(resolve, 700))
  }
  if (!latest) {
    throw new Error(t('store.errors.SHOP_ORDER_NOT_FOUND'))
  }
  return latest
}

onMounted(async () => {
  try {
    await loadStore()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'store.errors', t('common.error')))
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.store-filter {
  min-height: 2.25rem;
  flex: 0 0 auto;
  padding: 0 0.15rem;
  border-bottom: 2px solid transparent;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--ui-text-secondary);
}

.store-filter-active {
  border-bottom-color: var(--ui-text);
  color: var(--ui-text);
}

.store-summary,
.store-product-card,
.store-dialog {
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
}

.store-summary {
  overflow: hidden;
}

.store-summary :deep(.ui-metric + .ui-metric) {
  border-left: 1px solid var(--ui-border);
}

.store-categories {
  display: flex;
  gap: 1.25rem;
  overflow-x: auto;
  border-bottom: 1px solid var(--ui-border);
  scrollbar-width: none;
}

.store-categories::-webkit-scrollbar {
  display: none;
}

.store-product-card {
  display: flex;
  min-width: 0;
  padding: 1.125rem;
}

.store-draw-progress {
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--ui-border);
  font-size: 0.875rem;
}

.store-pay-option {
  min-height: 4rem;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 0.25rem;
  border-radius: var(--ui-radius-md);
  border: 1px solid var(--ui-border);
  background: var(--ui-surface);
  padding: 0.75rem;
  text-align: left;
}

.store-pay-option:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.store-pay-option-active {
  border-color: var(--ui-text);
  background: var(--ui-surface-subtle);
}

.store-dialog {
  max-height: min(90vh, 760px);
  overflow-y: auto;
}

.store-close-button {
  display: inline-flex;
  width: 2.25rem;
  height: 2.25rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border-radius: var(--ui-radius-md);
  color: var(--ui-text-tertiary);
}

.store-close-button:hover {
  background: var(--ui-surface-subtle);
  color: var(--ui-text);
}

.store-order-summary {
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
  padding: 0.875rem 0;
  border-block: 1px solid var(--ui-border);
  font-size: 0.875rem;
}

@media (max-width: 640px) {
  .store-summary :deep(.ui-metric + .ui-metric) {
    border-left: 0;
  }

  .store-summary :deep(.ui-metric:nth-child(even)) {
    border-left: 1px solid var(--ui-border);
  }

  .store-summary :deep(.ui-metric:nth-child(n + 3)) {
    border-top: 1px solid var(--ui-border);
  }
}
</style>
