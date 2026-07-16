/**
 * Admin Affiliate API endpoints
 * Manage per-user affiliate (邀请返利) configurations:
 * exclusive invite codes (overrides aff_code) and exclusive rebate rates.
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface AffiliateAdminEntry {
  user_id: number
  email: string
  username: string
  aff_code: string
  aff_code_custom: boolean
  aff_rebate_rate_percent?: number | null
  aff_code_usage_limit?: number | null
  aff_code_expires_at?: string | null
  aff_signup_bonus_balance: number
  aff_auto_group_id?: number | null
  aff_auto_group_name?: string
  aff_count: number
}

export interface ListAffiliateUsersParams {
  page?: number
  page_size?: number
  search?: string
}

export interface UpdateAffiliateUserRequest {
  aff_code?: string
  aff_rebate_rate_percent?: number | null
  aff_code_usage_limit?: number | null
  aff_code_expires_at?: string | null
  aff_signup_bonus_balance?: number
  aff_auto_group_id?: number | null
  /** Set true to explicitly clear the per-user rate (sets it to NULL). */
  clear_rebate_rate?: boolean
  clear_aff_code_usage_limit?: boolean
  clear_aff_code_expires_at?: boolean
  clear_aff_auto_group_id?: boolean
}

export interface BatchSetRateRequest {
  user_ids: number[]
  aff_rebate_rate_percent?: number | null
  /** Set true to clear rates instead of setting. */
  clear?: boolean
}

export interface BindInviterRequest {
  inviter_user_id: number
  reset_validity: boolean
}

export interface ExtendInviteRewardsRequest {
  scope: 'site' | 'inviter'
  inviter_user_id?: number
  all_invitees?: boolean
  invitee_user_ids?: number[]
  extend_days: number
}

export interface ExtendInviteRewardsResponse {
  affected: number
}

export interface SimpleUser {
  id: number
  email: string
  username: string
}

export async function listUsers(
  params: ListAffiliateUsersParams = {},
): Promise<PaginatedResponse<AffiliateAdminEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateAdminEntry>>(
    '/admin/affiliates/users',
    {
      params: {
        page: params.page ?? 1,
        page_size: params.page_size ?? 20,
        search: params.search ?? '',
      },
    },
  )
  return data
}

export async function lookupUsers(q: string): Promise<SimpleUser[]> {
  const { data } = await apiClient.get<SimpleUser[]>(
    '/admin/affiliates/users/lookup',
    { params: { q } },
  )
  return data
}

export async function updateUserSettings(
  userId: number,
  payload: UpdateAffiliateUserRequest,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.put<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
    payload,
  )
  return data
}

export async function clearUserSettings(
  userId: number,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.delete<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
  )
  return data
}

export async function batchSetRate(
  payload: BatchSetRateRequest,
): Promise<{ affected: number }> {
  const { data } = await apiClient.post<{ affected: number }>(
    '/admin/affiliates/users/batch-rate',
    payload,
  )
  return data
}

export async function bindInviter(
  userId: number,
  payload: BindInviterRequest,
): Promise<{ user_id: number; inviter_id?: number | null; inviter_bound_at?: string | null; invite_reward_expires_at?: string | null }> {
  const { data } = await apiClient.post<{ user_id: number; inviter_id?: number | null; inviter_bound_at?: string | null; invite_reward_expires_at?: string | null }>(
    `/admin/affiliates/users/${userId}/inviter`,
    payload,
  )
  return data
}

export async function extendInviteRewards(
  payload: ExtendInviteRewardsRequest,
): Promise<ExtendInviteRewardsResponse> {
  const { data } = await apiClient.post<ExtendInviteRewardsResponse>(
    '/admin/affiliates/invite-rewards/extend',
    payload,
  )
  return data
}

export const affiliatesAPI = {
  listUsers,
  lookupUsers,
  updateUserSettings,
  clearUserSettings,
  batchSetRate,
  bindInviter,
  extendInviteRewards,
}

export default affiliatesAPI
