import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type RevenueGranularity = 'day' | 'hour'

export interface RevenueSummaryParams {
  start_date?: string
  end_date?: string
  granularity?: RevenueGranularity
  top_limit?: number
  user_id?: number
}

export interface RevenueShareSettlementParams {
  page?: number
  page_size?: number
  start_date?: string
  end_date?: string
  search?: string
  status?: 'all' | 'applied' | 'reversed' | 'frozen'
}

export interface RevenueCashStats {
  paid_amount: number
  balance_paid_amount: number
  subscription_paid_amount: number
  redeem_balance_amount: number
  refund_amount: number
  net_paid_amount: number
  pending_amount: number
  paid_order_count: number
  redeem_balance_count: number
  refund_order_count: number
  pending_order_count: number
}

export interface RevenueUsageStats {
  requests: number
  total_tokens: number
  standard_cost: number
  consumed_revenue: number
  balance_consumed_amount: number
  points_consumed_amount: number
  points_issued_amount: number
  account_cost: number
}

export interface RevenueAdjustmentStats {
  affiliate_rebate: number
  affiliate_transfer: number
  affiliate_rebate_count: number
  private_group_commission: number
  share_consumer_charge: number
  share_account_cost: number
  share_owner_credit: number
  share_platform_fee: number
  share_net_profit: number
  share_settlement_count: number
}

export interface RevenueProfitStats {
  usage_gross_profit: number
  usage_gross_margin: number
  estimated_net_profit: number
  estimated_net_margin: number
}

export interface RevenueTrendPoint {
  date: string
  paid_amount: number
  redeem_balance_amount: number
  refund_amount: number
  net_paid_amount: number
  requests: number
  consumed_revenue: number
  balance_consumed_amount: number
  points_consumed_amount: number
  points_issued_amount: number
  account_cost: number
  usage_gross_profit: number
  affiliate_rebate: number
  private_group_commission: number
  share_owner_credit: number
  share_platform_fee: number
  estimated_net_profit: number
}

export interface RevenueBreakdownItem {
  id?: number
  name: string
  secondary?: string
  requests: number
  total_tokens: number
  consumed_revenue: number
  account_cost: number
  share_owner_credit: number
  gross_profit: number
  gross_margin: number
  net_profit: number
  net_margin: number
}

export interface RevenueShareOwnerBreakdownItem {
  id?: number
  name: string
  secondary?: string
  requests: number
  total_tokens: number
  consumer_charge: number
  account_cost: number
  owner_credit: number
  platform_fee: number
  owner_share_ratio: number
}

export interface RevenueShareSettlementItem {
  id: number
  usage_log_id?: number
  request_id: string
  api_key_id: number
  api_key_name: string
  consumer_user_id: number
  consumer_email: string
  consumer_username?: string
  owner_user_id: number
  owner_email: string
  owner_username?: string
  inviter_user_id?: number
  inviter_email?: string
  inviter_username?: string
  account_id: number
  account_name: string
  account_platform: string
  group_id?: number
  group_name?: string
  policy_id?: number
  policy_version: number
  share_mode_snapshot: string
  share_status_snapshot: string
  consumer_charge: number
  account_cost: number
  owner_share_ratio: number
  owner_credit: number
  invite_bound_at?: string
  invite_expires_at?: string
  invite_share_ratio: number
  invite_credit: number
  platform_share_ratio: number
  platform_fee: number
  platform_net_profit: number
  status: string
  created_at: string
}

export interface RevenueSummary {
  generated_at: string
  start_date: string
  end_date: string
  granularity: RevenueGranularity
  cash: RevenueCashStats
  usage: RevenueUsageStats
  adjustments: RevenueAdjustmentStats
  profit: RevenueProfitStats
  trend: RevenueTrendPoint[]
  top_users: RevenueBreakdownItem[]
  top_groups: RevenueBreakdownItem[]
  top_accounts: RevenueBreakdownItem[]
  top_models: RevenueBreakdownItem[]
  top_share_owners: RevenueShareOwnerBreakdownItem[]
}

export const revenueAPI = {
  getSummary(params?: RevenueSummaryParams) {
    return apiClient.get<RevenueSummary>('/admin/revenue/summary', { params })
  },

  async listShareSettlements(params?: RevenueShareSettlementParams): Promise<PaginatedResponse<RevenueShareSettlementItem>> {
    const { data } = await apiClient.get<PaginatedResponse<RevenueShareSettlementItem>>('/admin/revenue/share-settlements', { params })
    return data
  }
}

export default revenueAPI
