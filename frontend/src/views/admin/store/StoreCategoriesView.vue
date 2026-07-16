<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <input v-model="keyword" class="input flex-1 sm:max-w-72" :placeholder="t('admin.store.searchCategories')" @input="handleSearch" />
          <div class="flex flex-1 justify-end gap-2">
            <UiIconButton :label="t('common.refresh')" :disabled="loading" @click="loadCategories">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </UiIconButton>
            <button class="btn btn-primary" @click="openCreate">
              <Icon name="plus" size="sm" />
              {{ t('admin.store.createCategory') }}
            </button>
          </div>
        </div>
      </template>
      <template #table>
        <DataTable :columns="columns" :data="categories" :loading="loading" row-key="id">
          <template #cell-status="{ value }">
            <span :class="['badge', value === 'active' ? 'badge-success' : 'badge-gray']">
              {{ t(`admin.store.status.${value}`) }}
            </span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex justify-end gap-2">
              <UiIconButton :label="t('common.edit')" size="sm" @click="openEdit(row)">
                <Icon name="edit" size="sm" />
              </UiIconButton>
              <UiIconButton :label="t('common.delete')" size="sm" tone="danger" @click="deleteCategory(row)">
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
        <form class="w-full max-w-md rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900" @submit.prevent="submitForm">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ editingCategory ? t('admin.store.editCategory') : t('admin.store.createCategory') }}
          </h2>
          <div class="mt-5 space-y-4">
            <div>
              <label class="input-label">{{ t('common.name') }}</label>
              <input v-model.trim="form.name" class="input" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.store.description') }}</label>
              <textarea v-model.trim="form.description" class="input min-h-24"></textarea>
            </div>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label class="input-label">{{ t('common.status') }}</label>
                <Select v-model="form.status" :options="statusOptions" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.store.sortOrder') }}</label>
                <input v-model.number="form.sort_order" class="input" type="number" />
              </div>
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
import type { StoreCategory, StoreCategoryStatus } from '@/types/store'
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

const categories = ref<StoreCategory[]>([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const dialogOpen = ref(false)
const editingCategory = ref<StoreCategory | null>(null)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const form = reactive({ name: '', description: '', status: 'active' as StoreCategoryStatus, sort_order: 0 })
let searchTimer: ReturnType<typeof setTimeout> | undefined

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('common.name') },
  { key: 'description', label: t('admin.store.description') },
  { key: 'product_count', label: t('admin.store.productCount') },
  { key: 'sort_order', label: t('admin.store.sortOrder') },
  { key: 'status', label: t('common.status') },
  { key: 'actions', label: t('common.actions') },
])
const statusOptions = computed(() => [
  { value: 'active', label: t('admin.store.status.active') },
  { value: 'inactive', label: t('admin.store.status.inactive') },
])

function resetForm() {
  form.name = ''
  form.description = ''
  form.status = 'active'
  form.sort_order = 0
}

function openCreate() {
  editingCategory.value = null
  resetForm()
  dialogOpen.value = true
}

function openEdit(category: StoreCategory) {
  editingCategory.value = category
  form.name = category.name
  form.description = category.description || ''
  form.status = category.status ?? (category.enabled ? 'active' : 'inactive')
  form.sort_order = category.sort_order
  dialogOpen.value = true
}

async function loadCategories() {
  loading.value = true
  try {
    const { data } = await adminStoreAPI.listCategories({
      page: pagination.page,
      page_size: pagination.page_size,
      keyword: keyword.value || undefined,
    })
    const keywordText = keyword.value.trim().toLowerCase()
    const filtered = keywordText
      ? data.filter(category => category.name.toLowerCase().includes(keywordText) || (category.description || '').toLowerCase().includes(keywordText))
      : data
    pagination.total = filtered.length
    categories.value = filtered.slice((pagination.page - 1) * pagination.page_size, pagination.page * pagination.page_size)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.store.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function submitForm() {
  saving.value = true
  try {
    const payload = { ...form, description: form.description || null }
    if (editingCategory.value) await adminStoreAPI.updateCategory(editingCategory.value.id, payload)
    else await adminStoreAPI.createCategory(payload)
    appStore.showSuccess(t('common.saved'))
    dialogOpen.value = false
    await loadCategories()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    saving.value = false
  }
}

async function deleteCategory(category: StoreCategory) {
  if (!window.confirm(t('admin.store.deleteCategoryConfirm'))) return
  try {
    await adminStoreAPI.deleteCategory(category.id)
    appStore.showSuccess(t('common.deleted'))
    await loadCategories()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

function handleSearch() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    pagination.page = 1
    loadCategories()
  }, 300)
}
function setPage(page: number) { pagination.page = page; loadCategories() }
function setPageSize(pageSize: number) { pagination.page_size = pageSize; pagination.page = 1; loadCategories() }

onMounted(loadCategories)
</script>
