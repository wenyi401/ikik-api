import { apiClient } from './client'
import type {
  AccountPlatform,
  CarpoolJoinRequest,
  CarpoolMineOverview,
  CarpoolPoolDetail,
  CarpoolPoolSummary,
  CarpoolPoolVisibility,
} from '@/types'

export interface CreateCarpoolPoolRequest {
  name: string
  platform: AccountPlatform
  visibility: CarpoolPoolVisibility
  target_seats: number
  duration_days: number
  seat_price: number
  extra_fee: number
  extra_fee_description?: string
  system_proxy_enabled?: boolean
  risk_control_enabled?: boolean
  notes?: string
}

export interface BindCarpoolAccountsRequest {
  account_ids: number[]
}

export interface ApplyCarpoolPoolRequest {
  note?: string
}

export interface ReviewCarpoolJoinRequestPayload {
  review_note?: string
}

export interface UpdateCarpoolMemberAllocationsRequest {
  allocations: Array<{
    member_id: number
    quota_share_ratio: number
  }>
}

export async function listMine(): Promise<CarpoolMineOverview> {
  const { data } = await apiClient.get<CarpoolMineOverview>('/accounts/carpools')
  return data
}

export async function listHall(): Promise<CarpoolPoolSummary[]> {
  const { data } = await apiClient.get<CarpoolPoolSummary[]>('/accounts/carpools/hall')
  return data
}

export async function getDetail(poolId: number): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.get<CarpoolPoolDetail>(`/accounts/carpools/${poolId}`)
  return data
}

export async function getByInviteCode(inviteCode: string): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.get<CarpoolPoolDetail>(`/accounts/carpools/invite/${encodeURIComponent(inviteCode)}`)
  return data
}

export async function createPool(payload: CreateCarpoolPoolRequest): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.post<CarpoolPoolDetail>('/accounts/carpools', payload)
  return data
}

export async function deletePool(poolId: number): Promise<void> {
  await apiClient.delete(`/accounts/carpools/${poolId}`)
}

export async function bindAccounts(
  poolId: number,
  payload: BindCarpoolAccountsRequest
): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.put<CarpoolPoolDetail>(`/accounts/carpools/${poolId}/accounts`, payload)
  return data
}

export async function resetAccountLocalLimit(
  poolId: number,
  accountId: number
): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.post<CarpoolPoolDetail>(
    `/accounts/carpools/${poolId}/accounts/${accountId}/reset-local-limit`
  )
  return data
}

export async function applyToPool(
  poolId: number,
  payload: ApplyCarpoolPoolRequest
): Promise<CarpoolJoinRequest> {
  const { data } = await apiClient.post<CarpoolJoinRequest>(`/accounts/carpools/${poolId}/apply`, payload)
  return data
}

export async function applyByInviteCode(
  inviteCode: string,
  payload: ApplyCarpoolPoolRequest
): Promise<CarpoolJoinRequest> {
  const { data } = await apiClient.post<CarpoolJoinRequest>(
    `/accounts/carpools/invite/${encodeURIComponent(inviteCode)}/apply`,
    payload
  )
  return data
}

export async function approveJoinRequest(
  poolId: number,
  requestId: number,
  payload: ReviewCarpoolJoinRequestPayload
): Promise<CarpoolJoinRequest> {
  const { data } = await apiClient.post<CarpoolJoinRequest>(
    `/accounts/carpools/${poolId}/requests/${requestId}/approve`,
    payload
  )
  return data
}

export async function rejectJoinRequest(
  poolId: number,
  requestId: number,
  payload: ReviewCarpoolJoinRequestPayload
): Promise<CarpoolJoinRequest> {
  const { data } = await apiClient.post<CarpoolJoinRequest>(
    `/accounts/carpools/${poolId}/requests/${requestId}/reject`,
    payload
  )
  return data
}

export async function confirmJoinPaid(poolId: number, requestId: number): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.post<CarpoolPoolDetail>(
    `/accounts/carpools/${poolId}/requests/${requestId}/confirm-paid`
  )
  return data
}

export async function removeMember(poolId: number, memberId: number): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.post<CarpoolPoolDetail>(
    `/accounts/carpools/${poolId}/members/${memberId}/remove`
  )
  return data
}

export async function updateMemberAllocations(
  poolId: number,
  payload: UpdateCarpoolMemberAllocationsRequest
): Promise<CarpoolPoolDetail> {
  const { data } = await apiClient.put<CarpoolPoolDetail>(
    `/accounts/carpools/${poolId}/members/allocation`,
    payload
  )
  return data
}

export const carpoolsAPI = {
  listMine,
  listHall,
  getDetail,
  getByInviteCode,
  createPool,
  deletePool,
  bindAccounts,
  resetAccountLocalLimit,
  applyToPool,
  applyByInviteCode,
  approveJoinRequest,
  rejectJoinRequest,
  confirmJoinPaid,
  removeMember,
  updateMemberAllocations,
}

export default carpoolsAPI
