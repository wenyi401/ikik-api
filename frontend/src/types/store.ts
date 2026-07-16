import type { BasePaginationResponse } from '@/types'
import type { CreateOrderResult } from '@/types/payment'

export type StoreCardStatus = 'available' | 'locked' | 'sold' | 'disabled'
export type StoreCardViewStatus = StoreCardStatus | 'unused'
export type StoreCardType = 'text' | 'file'
export type StoreOrderStatus = 'pending' | 'paid' | 'completed' | 'cancelled' | 'failed'
export type StorePayMethod = 'balance' | 'points' | 'payment'
export type StoreCategoryStatus = 'active' | 'inactive'
export type StoreProductStatus = 'active' | 'inactive'
export type StoreProductType = 'card_key' | 'balance_draw' | 'points_draw'

export interface StoreDrawConfig {
  enabled: boolean
  min_amount: number
  max_amount: number
  guarantee_count: number
  return_rate: number
}

export interface StoreDrawProgress {
  drawn_count: number
  guarantee_count: number
}

export interface StoreCategory {
  id: number
  name: string
  icon?: string | null
  description?: string | null
  enabled: boolean
  status?: StoreCategoryStatus
  sort_order: number
  product_count?: number
  created_at?: string
  updated_at?: string
}

export interface StoreProduct {
  id: number
  category_id?: number | null
  category?: StoreCategory | null
  name: string
  cover_url?: string | null
  image_url?: string | null
  description?: string | null
  price: number
  original_price?: number | null
  stock: number
  enabled: boolean
  status?: StoreProductStatus
  sort_order: number
  min_purchase: number
  max_purchase: number
  purchase_limit?: number | null
  auto_delivery: boolean
  product_type: StoreProductType
  balance_only: boolean
  allow_balance_payment: boolean
  allow_points_payment: boolean
  allow_platform_payment: boolean
  draw_config?: StoreDrawConfig | null
  draw_progress?: StoreDrawProgress | null
  stock_unlimited?: boolean
  created_at?: string
  updated_at?: string
}

export interface StoreCard {
  id: number
  product_id: number
  product?: string | null
  content: string
  card_type: StoreCardType
  storage_provider?: string | null
  original_filename?: string | null
  content_type?: string | null
  byte_size?: number | null
  sha256?: string | null
  status: StoreCardViewStatus
  order_id?: number | null
  order_no?: string | null
  sold_at?: string | null
  created_at?: string
  updated_at?: string
}

export interface StoreDeliveredFile {
  id: number
  filename: string
  content_type: string
  byte_size: number
  sha256: string
}

export interface StoreOrder {
  id: number
  order_no: string
  user_id: number
  product_id: number
  product_name: string
  product_cover_url?: string | null
  product_description?: string | null
  product_type: StoreProductType
  unit_price: number
  quantity: number
  total_amount: number
  points_amount: number
  payment_method: string
  payment_order_id?: number | null
  status: StoreOrderStatus
  delivered_cards: string[]
  delivered_files: StoreDeliveredFile[]
  draw_reward_amount?: number | null
  draw_reward_type?: 'balance' | 'points' | null
  draw_cycle_id?: number | null
  draw_cycle_index?: number | null
  paid_at?: string | null
  completed_at?: string | null
  cancelled_at?: string | null
  failed_reason?: string | null
  created_at?: string
  updated_at?: string
  payment?: CreateOrderResult | null
}

export interface CreateStoreOrderRequest {
  product_id: number
  quantity: number
  payment_method: string
  openid?: string
  wechat_resume_token?: string
  return_url?: string
  payment_source?: string
  is_mobile?: boolean
}

export interface StoreListParams {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
  category_id?: number | string
  product_id?: number | string
}

export type StorePaginatedResponse<T> = BasePaginationResponse<T>

export interface UpsertStoreCategoryRequest {
  name: string
  icon?: string | null
  description?: string | null
  enabled?: boolean
  status?: StoreCategoryStatus
  sort_order: number
}

export interface UpsertStoreProductRequest {
  category_id?: number | null
  clear_category?: boolean
  name: string
  cover_url?: string | null
  image_url?: string | null
  description?: string | null
  price: number
  original_price?: number | null
  clear_original_price?: boolean
  enabled?: boolean
  status?: StoreProductStatus
  sort_order: number
  min_purchase?: number
  max_purchase?: number
  purchase_limit?: number | null
  auto_delivery?: boolean
  product_type?: StoreProductType
  balance_only?: boolean
  allow_balance_payment?: boolean
  allow_points_payment?: boolean
  allow_platform_payment?: boolean
  draw_config?: StoreDrawConfig | null
}

export interface ImportStoreCardsRequest {
  product_id: number
  contents: string[]
}

export interface StoreFileCardStorageConfig {
  enabled: boolean
  endpoint: string
  region: string
  bucket: string
  access_key_id: string
  secret_access_key?: string
  secret_access_key_configured: boolean
  prefix: string
  force_path_style: boolean
  max_size_bytes: number
}

export interface UpdateStoreFileCardStorageConfigRequest {
  enabled?: boolean
  endpoint?: string
  region?: string
  bucket?: string
  access_key_id?: string
  secret_access_key?: string
  prefix?: string
  force_path_style?: boolean
}
