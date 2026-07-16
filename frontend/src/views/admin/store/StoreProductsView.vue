<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <input v-model="keyword" class="input flex-1 sm:max-w-72" :placeholder="t('admin.store.searchProducts')" @input="handleSearch" />
          <div class="flex flex-1 justify-end gap-2">
            <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="loadProducts">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </UiIconButton>
            <button class="btn btn-primary" @click="openCreate">
              <Icon name="plus" size="sm" />
              {{ t('admin.store.createProduct') }}
            </button>
          </div>
        </div>
      </template>
      <template #table>
        <DataTable :columns="columns" :data="products" :loading="loading" row-key="id">
          <template #cell-price="{ value }">¥{{ Number(value || 0).toFixed(2) }}</template>
          <template #cell-category_id="{ value, row }">{{ row.category?.name || categoryName(value) }}</template>
          <template #cell-product_type="{ value }">{{ productTypeLabel(value) }}</template>
          <template #cell-payment_methods="{ row }">{{ paymentMethodsText(row) }}</template>
          <template #cell-stock="{ value, row }">{{ row.stock_unlimited ? t('admin.store.unlimitedStock') : value }}</template>
          <template #cell-status="{ value }">
            <span :class="['badge', value === 'active' ? 'badge-success' : 'badge-gray']">{{ t(`admin.store.status.${value}`) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex justify-end gap-2">
              <UiIconButton :label="t('common.edit')" size="sm" @click="openEdit(row)">
                <Icon name="edit" size="sm" />
              </UiIconButton>
              <UiIconButton :label="t('common.delete')" size="sm" tone="danger" @click="deleteProduct(row)">
                <Icon name="trash" size="sm" />
              </UiIconButton>
            </div>
          </template>
        </DataTable>
      </template>
      <template #pagination>
        <Pagination v-if="pagination.total > 0" :page="pagination.page" :page-size="pagination.page_size" :total="pagination.total" @update:page="setPage" @update:pageSize="setPageSize" />
      </template>
    </TablePageLayout>

    <Teleport to="body">
      <div v-if="dialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="dialogOpen = false">
        <form class="w-full max-w-2xl rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900" @submit.prevent="submitForm">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ editingProduct ? t('admin.store.editProduct') : t('admin.store.createProduct') }}</h2>
          <div class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('common.name') }}</label>
              <input v-model.trim="form.name" class="input" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.category') }}</label>
              <Select v-model="form.category_id" :options="categoryOptions" />
            </div>
            <div>
              <label class="input-label">{{ t('common.status') }}</label>
              <Select v-model="form.status" :options="statusOptions" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.productType') }}</label>
              <Select v-model="form.product_type" :options="productTypeOptions" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.price') }}</label>
              <input v-model.number="form.price" class="input" type="number" min="0.01" step="0.01" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.originalPrice') }}</label>
              <input v-model.number="form.original_price" class="input" type="number" min="0" step="0.01" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.sortOrder') }}</label>
              <input v-model.number="form.sort_order" class="input" type="number" />
            </div>
            <div class="sm:col-span-2 grid gap-3 sm:grid-cols-3">
              <label class="payment-toggle-card">
                <span>
                  <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.store.allowBalancePayment') }}</span>
                  <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.store.allowBalancePaymentHint') }}</span>
                </span>
                <Toggle :model-value="form.allow_balance_payment" @update:modelValue="setPaymentMethod('balance', $event)" />
              </label>
              <label class="payment-toggle-card">
                <span>
                  <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.store.allowPointsPayment') }}</span>
                  <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.store.allowPointsPaymentHint') }}</span>
                </span>
                <Toggle :model-value="form.allow_points_payment" @update:modelValue="setPaymentMethod('points', $event)" />
              </label>
              <label class="payment-toggle-card">
                <span>
                  <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.store.allowPlatformPayment') }}</span>
                  <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.store.allowPlatformPaymentHint') }}</span>
                </span>
                <Toggle :model-value="form.allow_platform_payment" @update:modelValue="setPaymentMethod('platform', $event)" />
              </label>
            </div>
            <p v-if="paymentMethodError" class="sm:col-span-2 text-sm text-red-600 dark:text-red-400">{{ paymentMethodError }}</p>
            <div>
              <label class="input-label">{{ t('admin.store.purchaseLimit') }}</label>
              <input v-model.number="form.purchase_limit" class="input" type="number" min="0" :disabled="isDrawProduct" />
            </div>
            <div v-if="isDrawProduct">
              <label class="input-label">{{ t('admin.store.drawMinAmount') }}</label>
              <input v-model.number="form.draw_config.min_amount" class="input" type="number" min="0.01" step="0.01" required />
            </div>
            <div v-if="isDrawProduct">
              <label class="input-label">{{ t('admin.store.drawMaxAmount') }}</label>
              <input v-model.number="form.draw_config.max_amount" class="input" type="number" min="0" step="0.01" required />
            </div>
            <div v-if="isDrawProduct">
              <label class="input-label">{{ t('admin.store.drawGuaranteeCount') }}</label>
              <input v-model.number="form.draw_config.guarantee_count" class="input" type="number" min="1" step="1" required />
            </div>
            <div v-if="isDrawProduct">
              <label class="input-label">{{ t('admin.store.drawReturnRate') }}</label>
              <input v-model.number="form.draw_config.return_rate" class="input" type="number" min="0.0001" step="0.0001" required />
            </div>
            <div v-if="isDrawProduct" class="sm:col-span-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200">
              {{ t('admin.store.drawConfigHint', { amount: drawTargetAmountText, unit: drawRewardUnitLabel }) }}
            </div>
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('admin.store.imageUrl') }}</label>
              <input v-model.trim="form.image_url" class="input" />
            </div>
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('admin.store.description') }}</label>
              <textarea v-model.trim="form.description" class="input min-h-24"></textarea>
            </div>
          </div>
          <div class="mt-5 flex justify-end gap-2">
            <button type="button" class="btn btn-secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button>
          </div>
        </form>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminStoreAPI } from '@/api/admin/store'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { StoreCategory, StoreProduct, StoreProductStatus, StoreProductType } from '@/types/store'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { UiIconButton } from '@/ui'

const { t } = useI18n()
const appStore = useAppStore()
const products = ref<StoreProduct[]>([])
const categories = ref<StoreCategory[]>([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const dialogOpen = ref(false)
const editingProduct = ref<StoreProduct | null>(null)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const form = reactive({
  category_id: null as number | null,
  name: '',
  description: '',
  price: 0,
  original_price: null as number | null,
  status: 'active' as StoreProductStatus,
  sort_order: 0,
  image_url: '',
  purchase_limit: null as number | null,
  allow_balance_payment: true,
  allow_points_payment: false,
  allow_platform_payment: true,
  product_type: 'card_key' as StoreProductType,
  balance_only: false,
  draw_config: {
    enabled: false,
    min_amount: 1,
    max_amount: 5,
    guarantee_count: 20,
    return_rate: 1,
  },
})
let searchTimer: ReturnType<typeof setTimeout> | undefined

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('common.name') },
  { key: 'category_id', label: t('admin.store.category') },
  { key: 'product_type', label: t('admin.store.productType') },
  { key: 'price', label: t('admin.store.price') },
  { key: 'payment_methods', label: t('admin.store.paymentMethods') },
  { key: 'stock', label: t('admin.store.stockLabel') },
  { key: 'status', label: t('common.status') },
  { key: 'actions', label: t('common.actions') },
])
const categoryOptions = computed(() => [
  { value: null, label: t('common.uncategorized') },
  ...categories.value.map(category => ({ value: category.id, label: category.name })),
])
const statusOptions = computed(() => [
  { value: 'active', label: t('admin.store.status.active') },
  { value: 'inactive', label: t('admin.store.status.inactive') },
])
const productTypeOptions = computed(() => [
  { value: 'card_key', label: t('admin.store.productTypes.cardKey') },
  { value: 'balance_draw', label: t('admin.store.productTypes.balanceDraw') },
  { value: 'points_draw', label: t('admin.store.productTypes.pointsDraw') },
])
const isDrawProduct = computed(() => form.product_type === 'balance_draw' || form.product_type === 'points_draw')
const drawTargetAmount = computed(() => Math.round(form.price * form.draw_config.guarantee_count * form.draw_config.return_rate * 100) / 100)
const isPointsDrawProduct = computed(() => form.product_type === 'points_draw')
const drawTargetAmountText = computed(() => isPointsDrawProduct.value
  ? drawTargetAmount.value.toFixed(10).replace(/\.?0+$/, '') || '0'
  : drawTargetAmount.value.toFixed(2))
const drawRewardUnitLabel = computed(() => isPointsDrawProduct.value ? t('common.points') : t('common.balance'))
const enabledPaymentMethodCount = computed(() =>
  Number(form.allow_balance_payment) + Number(form.allow_points_payment) + Number(form.allow_platform_payment),
)
const paymentMethodError = computed(() => enabledPaymentMethodCount.value <= 0 ? t('admin.store.paymentMethodRequired') : '')

function categoryName(id?: number | null) {
  if (!id) return t('common.uncategorized')
  return categories.value.find(category => category.id === id)?.name || `#${id}`
}
function productTypeLabel(value?: string) {
  if (value === 'balance_draw') return t('admin.store.productTypes.balanceDraw')
  if (value === 'points_draw') return t('admin.store.productTypes.pointsDraw')
  return t('admin.store.productTypes.cardKey')
}
function paymentMethodsText(product: StoreProduct) {
  const methods: string[] = []
  if (product.allow_balance_payment !== false) methods.push(t('common.balance'))
  if (product.allow_points_payment === true) methods.push(t('common.points'))
  if (product.allow_platform_payment !== false) methods.push(t('admin.store.platformPayment'))
  return methods.length > 0 ? methods.join(' / ') : t('common.disabled')
}
function setPaymentMethod(method: 'balance' | 'points' | 'platform', enabled: boolean) {
  if (!enabled && enabledPaymentMethodCount.value <= 1) {
    appStore.showError(t('admin.store.paymentMethodRequired'))
    return
  }
  if (method === 'balance') form.allow_balance_payment = enabled
  else if (method === 'points') form.allow_points_payment = enabled
  else form.allow_platform_payment = enabled
}
function nullableNumber(value: number | null) {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}
function resetForm() {
  form.category_id = null
  form.name = ''
  form.description = ''
  form.price = 0
  form.original_price = null
  form.status = 'active'
  form.sort_order = 0
  form.image_url = ''
  form.purchase_limit = null
  form.allow_balance_payment = true
  form.allow_points_payment = false
  form.allow_platform_payment = true
  form.product_type = 'card_key'
  form.balance_only = false
  form.draw_config.enabled = false
  form.draw_config.min_amount = 1
  form.draw_config.max_amount = 5
  form.draw_config.guarantee_count = 20
  form.draw_config.return_rate = 1
}
function openCreate() { editingProduct.value = null; resetForm(); dialogOpen.value = true }
function openEdit(product: StoreProduct) {
  editingProduct.value = product
  form.category_id = product.category_id ?? null
  form.name = product.name
  form.description = product.description || ''
  form.price = product.price
  form.original_price = product.original_price ?? null
  form.status = product.status ?? (product.enabled ? 'active' : 'inactive')
  form.sort_order = product.sort_order
  form.image_url = product.cover_url || product.image_url || ''
  form.purchase_limit = product.purchase_limit ?? null
  form.product_type = product.product_type || 'card_key'
  form.balance_only = product.balance_only === true
  form.allow_balance_payment = product.allow_balance_payment !== false
  form.allow_points_payment = product.allow_points_payment === true
  form.allow_platform_payment = product.allow_platform_payment !== false
  form.draw_config.enabled = product.draw_config?.enabled ?? (form.product_type === 'balance_draw' || form.product_type === 'points_draw')
  form.draw_config.min_amount = product.draw_config?.min_amount ?? 1
  form.draw_config.max_amount = product.draw_config?.max_amount ?? 5
  form.draw_config.guarantee_count = product.draw_config?.guarantee_count ?? 20
  form.draw_config.return_rate = product.draw_config?.return_rate ?? 1
  dialogOpen.value = true
}

async function loadCategories() {
  const { data } = await adminStoreAPI.listCategories({ page: 1, page_size: 1000, status: 'active' })
  categories.value = data.filter(category => category.enabled)
}
async function loadProducts() {
  loading.value = true
  try {
    const { data } = await adminStoreAPI.listProducts({ page: pagination.page, page_size: pagination.page_size, keyword: keyword.value || undefined })
    products.value = data.items
    pagination.total = data.total
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.store.loadFailed')))
  } finally {
    loading.value = false
  }
}
async function submitForm() {
  if (paymentMethodError.value) {
    appStore.showError(paymentMethodError.value)
    return
  }
  saving.value = true
  try {
    const payload = {
      ...form,
      description: form.description || null,
      image_url: form.image_url || null,
      category_id: form.category_id || null,
      clear_category: !!editingProduct.value && !!editingProduct.value.category_id && !form.category_id,
      original_price: nullableNumber(form.original_price),
      clear_original_price: !!editingProduct.value && typeof editingProduct.value.original_price === 'number' && nullableNumber(form.original_price) === null,
      purchase_limit: isDrawProduct.value ? 1 : nullableNumber(form.purchase_limit),
      min_purchase: 1,
      max_purchase: isDrawProduct.value ? 1 : nullableNumber(form.purchase_limit) || 1,
      auto_delivery: true,
      allow_balance_payment: form.allow_balance_payment,
      allow_points_payment: form.allow_points_payment,
      allow_platform_payment: form.allow_platform_payment,
      product_type: form.product_type,
      balance_only: isDrawProduct.value,
      draw_config: isDrawProduct.value
        ? { ...form.draw_config, enabled: true }
        : { enabled: false, min_amount: 0, max_amount: 0, guarantee_count: 0, return_rate: 1 },
    }
    if (editingProduct.value) await adminStoreAPI.updateProduct(editingProduct.value.id, payload)
    else await adminStoreAPI.createProduct(payload)
    appStore.showSuccess(t('common.saved'))
    dialogOpen.value = false
    await loadProducts()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    saving.value = false
  }
}
async function deleteProduct(product: StoreProduct) {
  if (!window.confirm(t('admin.store.deleteProductConfirm'))) return
  try {
    await adminStoreAPI.deleteProduct(product.id)
    appStore.showSuccess(t('common.deleted'))
    await loadProducts()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}
function handleSearch() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { pagination.page = 1; loadProducts() }, 300)
}
function setPage(page: number) { pagination.page = page; loadProducts() }
function setPageSize(pageSize: number) { pagination.page_size = pageSize; pagination.page = 1; loadProducts() }
onMounted(async () => { await loadCategories(); await loadProducts() })
</script>

<style scoped>
.payment-toggle-card {
  display: flex;
  min-height: 5.75rem;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(229 231 235);
  background: rgb(249 250 251);
  padding: 0.75rem;
}

.dark .payment-toggle-card {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55);
}
</style>
