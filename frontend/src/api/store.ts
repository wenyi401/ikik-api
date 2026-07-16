import { apiClient } from './client'
import type {
  CreateStoreOrderRequest,
  StoreCategory,
  StoreListParams,
  StoreOrder,
  StoreProduct,
} from '@/types/store'
import type { BasePaginationResponse } from '@/types'

function normalizeCategory<T extends StoreCategory>(category: T): T {
  return {
    ...category,
    status: category.enabled ? 'active' : 'inactive',
  }
}

function normalizeProduct<T extends StoreProduct>(product: T): T {
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

function normalizeOrder(order: StoreOrder): StoreOrder {
  return {
    ...order,
    product_type: order.product_type || 'card_key',
    points_amount: Number(order.points_amount || 0),
    draw_reward_type: order.draw_reward_type || null,
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

export const storeAPI = {
  getCategories() {
    return apiClient.get<StoreCategory[]>('/shop/categories').then((response) => {
      response.data = (response.data || []).map(normalizeCategory)
      return response
    })
  },

  getProducts(params?: Pick<StoreListParams, 'category_id' | 'keyword'>) {
    return apiClient.get<BasePaginationResponse<StoreProduct>>('/shop/products', { params }).then((response) => {
      response.data.items = (response.data.items || []).map(normalizeProduct)
      return response
    })
  },

  getDrawProgress() {
    return apiClient.get<Record<number, StoreProduct['draw_progress']>>('/shop/draw-progress')
  },

  createOrder(data: CreateStoreOrderRequest, idempotencyKey?: string) {
    return apiClient.post<StoreOrder>('/shop/orders', data, {
      headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined,
    }).then((response) => {
      response.data = normalizeOrder(response.data)
      return response
    })
  },

  getOrder(orderId: number) {
    return apiClient.get<StoreOrder>(`/shop/orders/${orderId}`).then((response) => {
      response.data = normalizeOrder(response.data)
      return response
    })
  },

  async downloadOrderFile(orderId: number, cardId: number, filename: string) {
    const { data } = await apiClient.get<Blob>(`/shop/orders/${orderId}/files/${cardId}/download`, { responseType: 'blob' })
    downloadBlob(data, filename || `shop-file-${cardId}`)
  },

  async downloadOrderFilesZip(orderId: number, filename?: string) {
    const { data } = await apiClient.get<Blob>(`/shop/orders/${orderId}/files/download.zip`, { responseType: 'blob' })
    downloadBlob(data, filename || `shop-order-${orderId}-files.zip`)
  },
}

export default storeAPI
