import { apiClient } from '../client'
import type {
  ImportStoreCardsRequest,
  StoreCard,
  StoreFileCardStorageConfig,
  StoreCardStatus,
  StoreCategory,
  StoreListParams,
  StoreOrder,
  StorePaginatedResponse,
  StoreProduct,
  UpdateStoreFileCardStorageConfigRequest,
  UpsertStoreCategoryRequest,
  UpsertStoreProductRequest,
} from '@/types/store'

function categoryToView(category: StoreCategory): StoreCategory {
  return { ...category, status: category.enabled ? 'active' : 'inactive' }
}

function productToView(product: StoreProduct): StoreProduct {
  return {
    ...product,
    image_url: product.cover_url,
    purchase_limit: product.max_purchase,
    product_type: product.product_type || 'card_key',
    balance_only: product.balance_only === true,
    allow_balance_payment: product.allow_balance_payment !== false,
    allow_points_payment: product.allow_points_payment === true,
    allow_platform_payment: product.allow_platform_payment !== false,
    stock_unlimited: product.stock_unlimited === true,
    status: product.enabled ? 'active' : 'inactive',
  }
}

function cardToView(card: StoreCard): StoreCard {
  return {
    ...card,
    card_type: card.card_type || 'text',
    status: card.status === 'available' ? 'unused' : card.status,
  }
}

function normalizeOrder(order: StoreOrder): StoreOrder {
  return {
    ...order,
    product_type: order.product_type || 'card_key',
    draw_reward_type: order.draw_reward_type || null,
    points_amount: Number(order.points_amount || 0),
    delivered_cards: order.delivered_cards || [],
    delivered_files: order.delivered_files || [],
  }
}

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function cardStatusToAPI(status?: string): StoreCardStatus | undefined {
  if (!status) return undefined
  return status === 'unused' ? 'available' : status as StoreCardStatus
}

function categoryPayload(data: UpsertStoreCategoryRequest) {
  return {
    name: data.name,
    icon: data.icon ?? null,
    description: data.description ?? null,
    sort_order: data.sort_order,
    enabled: data.enabled ?? data.status !== 'inactive',
  }
}

function productPayload(data: UpsertStoreProductRequest) {
  const maxPurchase = data.max_purchase ?? data.purchase_limit ?? 1
  const payload: UpsertStoreProductRequest = {
    category_id: data.category_id || null,
    clear_category: data.clear_category === true ? true : undefined,
    name: data.name,
    cover_url: data.cover_url ?? data.image_url ?? null,
    description: data.description ?? null,
    price: data.price,
    original_price: typeof data.original_price === 'number' && Number.isFinite(data.original_price)
      ? data.original_price
      : undefined,
    clear_original_price: data.clear_original_price === true ? true : undefined,
    enabled: data.enabled ?? data.status !== 'inactive',
    sort_order: data.sort_order,
    min_purchase: data.min_purchase ?? 1,
    max_purchase: maxPurchase > 0 ? maxPurchase : 1,
    auto_delivery: data.auto_delivery ?? true,
    product_type: data.product_type || 'card_key',
    balance_only: data.balance_only === true,
    allow_balance_payment: data.allow_balance_payment !== false,
    allow_points_payment: data.allow_points_payment === true,
    allow_platform_payment: data.allow_platform_payment !== false,
    draw_config: data.draw_config ?? null,
  }
  return payload
}

export const adminStoreAPI = {
  listCategories(params?: StoreListParams) {
    void params
    return apiClient.get<StoreCategory[]>('/admin/shop/categories').then((response) => {
      response.data = (response.data || []).map(categoryToView)
      return response
    })
  },

  createCategory(data: UpsertStoreCategoryRequest) {
    return apiClient.post<StoreCategory>('/admin/shop/categories', categoryPayload(data)).then((response) => {
      response.data = categoryToView(response.data)
      return response
    })
  },

  updateCategory(id: number, data: UpsertStoreCategoryRequest) {
    return apiClient.put<StoreCategory>(`/admin/shop/categories/${id}`, categoryPayload(data)).then((response) => {
      response.data = categoryToView(response.data)
      return response
    })
  },

  deleteCategory(id: number) {
    return apiClient.delete(`/admin/shop/categories/${id}`)
  },

  listProducts(params?: StoreListParams) {
    return apiClient.get<StorePaginatedResponse<StoreProduct>>('/admin/shop/products', { params }).then((response) => {
      response.data.items = (response.data.items || []).map(productToView)
      return response
    })
  },

  createProduct(data: UpsertStoreProductRequest) {
    return apiClient.post<StoreProduct>('/admin/shop/products', productPayload(data)).then((response) => {
      response.data = productToView(response.data)
      return response
    })
  },

  updateProduct(id: number, data: UpsertStoreProductRequest) {
    return apiClient.put<StoreProduct>(`/admin/shop/products/${id}`, productPayload(data)).then((response) => {
      response.data = productToView(response.data)
      return response
    })
  },

  deleteProduct(id: number) {
    return apiClient.delete(`/admin/shop/products/${id}`)
  },

  listCards(params?: StoreListParams) {
    const nextParams = { ...params, status: cardStatusToAPI(params?.status) }
    return apiClient.get<StorePaginatedResponse<StoreCard>>('/admin/shop/card-keys', { params: nextParams }).then((response) => {
      response.data.items = (response.data.items || []).map(cardToView)
      return response
    })
  },

  importCards(data: ImportStoreCardsRequest) {
    return apiClient.post<StoreCard[]>('/admin/shop/card-keys/import', data).then((response) => {
      response.data = (response.data || []).map(cardToView)
      return response
    })
  },

  importFileCards(productId: number, files: File[]) {
    const form = new FormData()
    form.append('product_id', String(productId))
    files.forEach(file => form.append('files', file))
    return apiClient.post<StoreCard[]>('/admin/shop/card-keys/import-files', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then((response) => {
      response.data = (response.data || []).map(cardToView)
      return response
    })
  },

  createCard(data: { product_id: number; content: string; status?: string }) {
    return apiClient.post<StoreCard>('/admin/shop/card-keys', { ...data, status: cardStatusToAPI(data.status) }).then((response) => {
      response.data = cardToView(response.data)
      return response
    })
  },

  updateCard(id: number, data: { product_id?: number; content?: string; status?: string }) {
    return apiClient.put<StoreCard>(`/admin/shop/card-keys/${id}`, { ...data, status: cardStatusToAPI(data.status) }).then((response) => {
      response.data = cardToView(response.data)
      return response
    })
  },

  deleteCard(id: number) {
    return apiClient.delete(`/admin/shop/card-keys/${id}`)
  },

  getOrder(orderId: number) {
    return apiClient.get<StoreOrder>(`/admin/shop/orders/${orderId}`).then((response) => {
      response.data = normalizeOrder(response.data)
      return response
    })
  },

  async downloadOrderFile(orderId: number, cardId: number, filename: string) {
    const { data } = await apiClient.get<Blob>(`/admin/shop/orders/${orderId}/files/${cardId}/download`, { responseType: 'blob' })
    downloadBlob(data, filename || `shop-file-${cardId}`)
  },

  async downloadOrderFilesZip(orderId: number, filename?: string) {
    const { data } = await apiClient.get<Blob>(`/admin/shop/orders/${orderId}/files/download.zip`, { responseType: 'blob' })
    downloadBlob(data, filename || `shop-order-${orderId}-files.zip`)
  },

  getFileCardStorage() {
    return apiClient.get<StoreFileCardStorageConfig>('/admin/shop/file-card-storage')
  },

  updateFileCardStorage(data: UpdateStoreFileCardStorageConfigRequest) {
    return apiClient.put<StoreFileCardStorageConfig>('/admin/shop/file-card-storage', data)
  },

  testFileCardStorage(data: UpdateStoreFileCardStorageConfigRequest) {
    return apiClient.post<{ message: string }>('/admin/shop/file-card-storage/test', data)
  },
}

export default adminStoreAPI
