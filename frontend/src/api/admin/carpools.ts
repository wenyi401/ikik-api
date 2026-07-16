import { apiClient } from '../client'
import type {
  CarpoolPoolDetail,
  CarpoolPoolSummary,
  CarpoolPoolStatus,
  GroupPlatform,
  PaginatedResponse,
} from '@/types'

export interface AdminCarpoolPoolSummary extends CarpoolPoolSummary {
  owner_email: string
  owner_username: string
}

export interface AdminCarpoolFilters {
  search?: string
  platform?: GroupPlatform | string
  status?: CarpoolPoolStatus | string
  owner_user_id?: number
}

export async function list(
  page = 1,
  pageSize = 20,
  filters: AdminCarpoolFilters = {},
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<AdminCarpoolPoolSummary>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminCarpoolPoolSummary>>('/admin/carpools', {
    params: {
      page,
      page_size: pageSize,
      ...filters,
    },
    signal: options?.signal,
  })
  return data
}

export async function get(poolId: number): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.get<CarpoolPoolDetail>(`/admin/carpools/${poolId}`)
  return data
}

export async function close(poolId: number): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.post<CarpoolPoolDetail>(`/admin/carpools/${poolId}/close`)
  return data
}

export async function deletePool(poolId: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/carpools/${poolId}`)
  return data
}

export const adminCarpoolsAPI = {
  list,
  get,
  close,
  deletePool,
}

export default adminCarpoolsAPI
