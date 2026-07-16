import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type AccountSharePolicyScopeType = 'global'

export interface AccountSharePolicy {
  id: number
  scope_type: AccountSharePolicyScopeType
  scope_id?: number
  platform?: string
  owner_share_ratio: number
  invite_share_ratio: number
  version: number
  enabled: boolean
  effective_at: string
  created_by_admin_id?: number
  created_at: string
  updated_at: string
}

export interface AccountSharePolicyFilters {
  scope_type?: AccountSharePolicyScopeType
  platform?: string
  enabled?: boolean
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

export interface CreateAccountSharePolicyRequest {
  scope_type?: AccountSharePolicyScopeType
  scope_id?: number
  platform?: string
  owner_share_ratio: number
  invite_share_ratio?: number
  enabled?: boolean
  effective_at?: string
}

export type UpdateAccountSharePolicyRequest = Partial<CreateAccountSharePolicyRequest>

export const accountSharePoliciesAPI = {
  async list(
    page = 1,
    pageSize = 20,
    filters?: AccountSharePolicyFilters
  ): Promise<PaginatedResponse<AccountSharePolicy>> {
    const { data } = await apiClient.get<PaginatedResponse<AccountSharePolicy>>('/admin/account-share-policies', {
      params: {
        page,
        page_size: pageSize,
        ...filters
      }
    })
    return data
  },

  async create(payload: CreateAccountSharePolicyRequest): Promise<AccountSharePolicy> {
    const { data } = await apiClient.post<AccountSharePolicy>('/admin/account-share-policies', payload)
    return data
  },

  async update(id: number, payload: UpdateAccountSharePolicyRequest): Promise<AccountSharePolicy> {
    const { data } = await apiClient.put<AccountSharePolicy>(`/admin/account-share-policies/${id}`, payload)
    return data
  }
}

export default accountSharePoliciesAPI
