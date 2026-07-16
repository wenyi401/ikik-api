<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <Select v-model="filters.product_id" :options="productOptionsWithAll" class="w-full sm:w-56" @change="loadCards" />
          <Select v-model="filters.status" :options="statusOptionsWithAll" class="w-full sm:w-40" @change="loadCards" />
          <div class="flex flex-1 justify-end gap-2">
            <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="loadCards">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </UiIconButton>
            <button class="btn btn-primary" :disabled="productOptions.length === 0" @click="openImport">
              <Icon name="plus" size="sm" />
              {{ t('admin.store.importCards') }}
            </button>
          </div>
        </div>
      </template>
      <template #table>
        <DataTable :columns="columns" :data="cards" :loading="loading" row-key="id">
          <template #cell-product_id="{ value, row }">{{ row.product || productName(value) }}</template>
          <template #cell-card_type="{ value }">
            <span :class="['badge', value === 'file' ? 'badge-primary' : 'badge-gray']">{{ t(`admin.store.cardType.${value || 'text'}`) }}</span>
          </template>
          <template #cell-content="{ value, row }">
            <div v-if="row.card_type === 'file'" class="space-y-1">
              <div class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ row.original_filename || '-' }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ formatBytes(row.byte_size || 0) }}</div>
            </div>
            <code v-else class="break-all font-mono text-xs">{{ value }}</code>
          </template>
          <template #cell-status="{ value }">
            <span :class="['badge', value === 'unused' ? 'badge-success' : value === 'sold' ? 'badge-warning' : 'badge-gray']">{{ t(`admin.store.cardStatus.${value}`) }}</span>
          </template>
          <template #cell-order_no="{ value }">
            <span v-if="value" class="font-mono text-xs">{{ value }}</span>
            <span v-else>-</span>
          </template>
          <template #cell-sold_at="{ value }">{{ formatSoldAt(value) }}</template>
          <template #cell-actions="{ row }">
            <UiIconButton v-if="row.status === 'unused'" :label="t('common.delete')" size="sm" tone="danger" @click="deleteCard(row)">
              <Icon name="trash" size="sm" />
            </UiIconButton>
            <span v-else>-</span>
          </template>
        </DataTable>
      </template>
      <template #pagination>
        <Pagination v-if="pagination.total > 0" :page="pagination.page" :page-size="pagination.page_size" :total="pagination.total" @update:page="setPage" @update:pageSize="setPageSize" />
      </template>
    </TablePageLayout>

    <Teleport to="body">
      <div v-if="dialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="dialogOpen = false">
        <form class="w-full max-w-xl rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900" @submit.prevent="submitImport">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.store.importCards') }}</h2>
          <div class="mt-5 space-y-4">
            <div>
              <label class="input-label">{{ t('admin.store.product') }}</label>
              <Select v-model="importForm.product_id" :options="productOptions" />
            </div>
            <div class="grid grid-cols-2 gap-2 rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
              <button type="button" :class="['rounded-md px-3 py-2 text-sm font-medium', importMode === 'text' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-600 dark:text-gray-300']" @click="importMode = 'text'">
                {{ t('admin.store.textCards') }}
              </button>
              <button type="button" :class="['rounded-md px-3 py-2 text-sm font-medium', importMode === 'file' ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-600 dark:text-gray-300']" @click="importMode = 'file'">
                {{ t('admin.store.fileCards') }}
              </button>
            </div>
            <div>
              <template v-if="importMode === 'text'">
                <label class="input-label">{{ t('admin.store.cardContents') }}</label>
                <textarea v-model="importForm.contents" class="input min-h-48 font-mono text-sm" :placeholder="t('admin.store.cardContentsPlaceholder')" :required="importMode === 'text'"></textarea>
              </template>
              <template v-else>
                <label class="input-label">{{ t('admin.store.cardFiles') }}</label>
                <input class="input" type="file" multiple @change="handleFileChange" />
                <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.store.fileCardSizeHint') }}</p>
                <div v-if="selectedFiles.length > 0" class="mt-3 max-h-40 space-y-2 overflow-y-auto rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                  <div v-for="file in selectedFiles" :key="`${file.name}-${file.size}-${file.lastModified}`" class="flex items-center justify-between gap-3 text-sm">
                    <span class="min-w-0 break-all text-gray-700 dark:text-gray-200">{{ file.name }}</span>
                    <span class="shrink-0 text-xs text-gray-500">{{ formatBytes(file.size) }}</span>
                  </div>
                </div>
              </template>
            </div>
          </div>
          <div class="mt-5 flex justify-end gap-2">
            <button type="button" class="btn btn-secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.processing') : t('common.import') }}</button>
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
import { formatDateTime } from '@/utils/format'
import type { StoreCard, StoreProduct } from '@/types/store'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { UiIconButton } from '@/ui'

const { t } = useI18n()
const appStore = useAppStore()
const cards = ref<StoreCard[]>([])
const products = ref<StoreProduct[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogOpen = ref(false)
const importMode = ref<'text' | 'file'>('text')
const selectedFiles = ref<File[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const filters = reactive({ product_id: '', status: '' })
const importForm = reactive({ product_id: 0, contents: '' })

const columns = computed<Column[]>(() => [
  { key: 'product_id', label: t('admin.store.product') },
  { key: 'card_type', label: t('admin.store.cardTypeLabel') },
  { key: 'content', label: t('admin.store.cardContent') },
  { key: 'status', label: t('common.status') },
  { key: 'order_no', label: t('admin.store.relatedOrderNo') },
  { key: 'sold_at', label: t('admin.store.soldAt') },
  { key: 'actions', label: t('common.actions') },
])
const productOptions = computed(() => products.value.map(product => ({ value: product.id, label: product.name })))
const productOptionsWithAll = computed(() => [{ value: '', label: t('admin.store.allProducts') }, ...productOptions.value])
const statusOptionsWithAll = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'unused', label: t('admin.store.cardStatus.unused') },
  { value: 'sold', label: t('admin.store.cardStatus.sold') },
  { value: 'disabled', label: t('admin.store.cardStatus.disabled') },
])

function productName(id: number) {
  return products.value.find(product => product.id === id)?.name || `#${id}`
}
function openImport() {
  importForm.product_id = products.value[0]?.id || 0
  importForm.contents = ''
  importMode.value = 'text'
  selectedFiles.value = []
  dialogOpen.value = true
}
async function loadProducts() {
  const { data } = await adminStoreAPI.listProducts({ page: 1, page_size: 1000, status: 'active' })
  products.value = data.items
}
async function loadCards() {
  loading.value = true
  try {
    const { data } = await adminStoreAPI.listCards({
      page: pagination.page,
      page_size: pagination.page_size,
      product_id: filters.product_id || undefined,
      status: filters.status || undefined,
    })
    cards.value = data.items
    pagination.total = data.total
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.store.loadFailed')))
  } finally {
    loading.value = false
  }
}
async function submitImport() {
  const contents = importForm.contents.split(/\r?\n/).map(line => line.trim()).filter(Boolean)
  if (importMode.value === 'text' && contents.length === 0) return
  if (importMode.value === 'file' && selectedFiles.value.length === 0) return
  saving.value = true
  try {
    if (importMode.value === 'file') {
      const oversized = selectedFiles.value.find(file => file.size > 200 * 1024)
      if (oversized) {
        appStore.showError(t('admin.store.fileCardTooLarge', { name: oversized.name }))
        return
      }
      await adminStoreAPI.importFileCards(importForm.product_id, selectedFiles.value)
    } else {
      await adminStoreAPI.importCards({ product_id: importForm.product_id, contents })
    }
    appStore.showSuccess(t('admin.store.importSuccess'))
    dialogOpen.value = false
    await loadCards()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    saving.value = false
  }
}
function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  selectedFiles.value = Array.from(input.files || [])
}
function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  return `${(bytes / 1024).toFixed(1)} KB`
}
function formatSoldAt(value: string | null | undefined) {
  return formatDateTime(value) || '-'
}
async function deleteCard(card: StoreCard) {
  if (!window.confirm(t('admin.store.deleteCardConfirm'))) return
  try {
    await adminStoreAPI.deleteCard(card.id)
    appStore.showSuccess(t('common.deleted'))
    await loadCards()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}
function setPage(page: number) { pagination.page = page; loadCards() }
function setPageSize(pageSize: number) { pagination.page_size = pageSize; pagination.page = 1; loadCards() }
onMounted(async () => { await loadProducts(); await loadCards() })
</script>
