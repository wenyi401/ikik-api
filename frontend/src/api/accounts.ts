/**
 * User-owned account management endpoints.
 * These routes are scoped to the current authenticated user.
 */

import { apiClient } from './client'
import type { Account, AccountUsageInfo, AccountUsageStatsResponse, AdminDataPayload, CreateAccountRequest, CreateProxyRequest, PaginatedResponse, Proxy, ProxyQualityCheckResult, UpdateAccountRequest, UpdateProxyRequest, UserAccountQuotaPoolDashboard, WindowStats } from '@/types'
import type { KiroIDCAuthUrlResponse, KiroTokenInfo } from '@/api/admin/kiro'

const USER_ACCOUNT_BULK_OPERATION_TIMEOUT_MS = 120000

export interface UserAccountListFilters {
  platform?: string
  type?: string
  status?: string
  group_id?: number | string
  search?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: UserAccountListFilters,
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<Account>> {
  const { data } = await apiClient.get<PaginatedResponse<Account>>('/accounts', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    signal: options?.signal
  })
  return data
}

export async function getById(id: number): Promise<Account> {
  const { data } = await apiClient.get<Account>(`/accounts/${id}`)
  return data
}

export async function getQuotaDashboard(options?: {
  signal?: AbortSignal
}): Promise<UserAccountQuotaPoolDashboard> {
  const { data } = await apiClient.get<UserAccountQuotaPoolDashboard>('/accounts/quota-dashboard', {
    signal: options?.signal
  })
  return data
}

export async function create(accountData: CreateAccountRequest): Promise<Account> {
  const { data } = await apiClient.post<Account>('/accounts', accountData)
  return data
}

export async function importAccount(accountData: CreateAccountRequest): Promise<Account> {
  const { data } = await apiClient.post<Account>('/accounts/import', accountData)
  return data
}

export interface ImportCredentialContentsRequest {
  contents: string[]
  kiro_config_import?: boolean
  share_mode?: 'private' | 'public'
  concurrency?: number
  load_factor?: number | null
  priority?: number
  group_ids?: number[]
  expires_at?: number | null
  auto_pause_on_expired?: boolean
}

export interface ImportCredentialError {
  index: number
  kind?: string
  name?: string
  message: string
}

export interface ImportCredentialContentsResponse {
  total: number
  created: number
  failed: number
  errors: ImportCredentialError[]
}

export async function importCredentialContents(
  request: ImportCredentialContentsRequest
): Promise<ImportCredentialContentsResponse> {
  const { data } = await apiClient.post<ImportCredentialContentsResponse>(
    '/accounts/import-credentials',
    request
  )
  return data
}

export async function exportData(options?: {
  ids?: number[]
  filters?: UserAccountListFilters
}): Promise<AdminDataPayload> {
  const params: Record<string, string> = {}
  if (options?.ids && options.ids.length > 0) {
    params.ids = options.ids.join(',')
  } else if (options?.filters) {
    const { platform, type, status, group_id, search, sort_by, sort_order } = options.filters
    if (platform) params.platform = platform
    if (type) params.type = type
    if (status) params.status = status
    if (group_id !== undefined && group_id !== '') params.group_id = String(group_id)
    if (search) params.search = search
    if (sort_by) params.sort_by = sort_by
    if (sort_order) params.sort_order = sort_order
  }
  const { data } = await apiClient.get<AdminDataPayload>('/accounts/data', { params })
  return data
}

export async function update(id: number, updates: UpdateAccountRequest): Promise<Account> {
  const { data } = await apiClient.put<Account>(`/accounts/${id}`, updates)
  return data
}

export async function revalidatePublicShare(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    `/accounts/${id}/revalidate-public-share`,
    undefined,
    { timeout: 75000 }
  )
  return data
}

export async function deleteAccount(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/accounts/${id}`)
  return data
}

export interface UserBulkAccountResult {
  account_id: number
  success: boolean
  error?: string
}

export interface UserBulkAccountOperationResponse {
  async?: boolean
  task?: AccountBatchTask
  success: number
  failed: number
  success_ids?: number[]
  failed_ids?: number[]
  results: UserBulkAccountResult[]
}

export type AccountBatchTaskStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'canceled'

export interface AccountBatchTaskItem {
  id: number
  task_id: number
  account_id: number
  status: AccountBatchTaskStatus
  error_message?: string
  result?: Record<string, unknown>
}

export interface AccountBatchTask {
  id: number
  scope: 'admin' | 'user'
  operation: string
  status: AccountBatchTaskStatus
  total: number
  processed: number
  success: number
  failed: number
  created_by: number
  owner_user_id?: number
  error_message?: string
  items?: AccountBatchTaskItem[]
}

export async function toggleStatus(id: number, status: 'active' | 'disabled'): Promise<Account> {
  return update(id, { status })
}

export async function bulkUpdate(
  accountIds: number[],
  updates: Partial<UpdateAccountRequest>
): Promise<UserBulkAccountOperationResponse> {
  const { data } = await apiClient.post<UserBulkAccountOperationResponse>('/accounts/bulk-update', {
    account_ids: accountIds,
    ...updates
  }, {
    timeout: USER_ACCOUNT_BULK_OPERATION_TIMEOUT_MS
  })
  return data
}

export async function bulkDelete(accountIds: number[]): Promise<UserBulkAccountOperationResponse> {
  const { data } = await apiClient.post<UserBulkAccountOperationResponse>('/accounts/bulk-delete', {
    account_ids: accountIds
  }, {
    timeout: USER_ACCOUNT_BULK_OPERATION_TIMEOUT_MS
  })
  return data
}

export async function createBatchRefreshTask(accountIds: number[]): Promise<AccountBatchTask> {
  const { data } = await apiClient.post<AccountBatchTask>('/accounts/batch-refresh/async', {
    account_ids: accountIds
  })
  return data
}

export async function createBatchRevalidatePublicShareTask(accountIds: number[]): Promise<AccountBatchTask> {
  const { data } = await apiClient.post<AccountBatchTask>('/accounts/batch-revalidate-public-share/async', {
    account_ids: accountIds
  })
  return data
}

export async function getBatchTask(taskId: number): Promise<AccountBatchTask> {
  const { data } = await apiClient.get<AccountBatchTask>(`/accounts/batch-tasks/${taskId}`)
  return data
}

export async function getUsage(id: number, source?: 'passive' | 'active'): Promise<AccountUsageInfo> {
  const { data } = await apiClient.get<AccountUsageInfo>(`/accounts/${id}/usage`, {
    params: source ? { source } : undefined
  })
  return data
}

export async function getStats(id: number, days: number = 30): Promise<AccountUsageStatsResponse> {
  const { data } = await apiClient.get<AccountUsageStatsResponse>(`/accounts/${id}/stats`, {
    params: { days }
  })
  return data
}

export async function getTodayStats(id: number): Promise<WindowStats> {
  const { data } = await apiClient.get<WindowStats>(`/accounts/${id}/today-stats`)
  return data
}

export async function testAccount(
  id: number,
  modelId?: string,
  prompt?: string,
  mode?: string
): Promise<{
  status: 'success' | 'error'
  message: string
  response?: string
  latency?: number
}> {
  const { data } = await apiClient.post<{
    status: 'success' | 'error'
    message: string
    response?: string
    latency?: number
  }>(`/accounts/${id}/test`, {
    model_id: modelId,
    prompt,
    mode
  })
  return data
}

export interface RefreshCredentialsResponse {
  account: Account
  warning?: string
  message?: string
}

function normalizeRefreshCredentialsResponse(data: Account | RefreshCredentialsResponse): RefreshCredentialsResponse {
  if (data && typeof data === 'object' && 'account' in data) {
    return data as RefreshCredentialsResponse
  }
  return { account: data as Account }
}

export async function refreshCredentials(id: number): Promise<RefreshCredentialsResponse> {
  const { data } = await apiClient.post<Account | RefreshCredentialsResponse>(`/accounts/${id}/refresh`)
  return normalizeRefreshCredentialsResponse(data)
}

export async function refreshCredentialsAccount(id: number): Promise<Account> {
  const data = await refreshCredentials(id)
  return data.account
}

export async function setPrivacy(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/accounts/${id}/set-privacy`)
  return data
}

export interface UserBatchTodayStatsResponse {
  stats: Record<string, WindowStats>
}

export async function getBatchTodayStats(accountIds: number[]): Promise<UserBatchTodayStatsResponse> {
  const { data } = await apiClient.post<UserBatchTodayStatsResponse>('/accounts/today-stats/batch', {
    account_ids: accountIds
  })
  return data
}

export interface ProxyTestResult {
  success: boolean
  message: string
  latency_ms?: number
  ip_address?: string
  city?: string
  region?: string
  country?: string
  country_code?: string
}

export async function listProxies(): Promise<Proxy[]> {
  const { data } = await apiClient.get<Proxy[]>('/account-proxies')
  return data
}

export async function createProxy(payload: CreateProxyRequest): Promise<Proxy> {
  const { data } = await apiClient.post<Proxy>('/account-proxies', payload)
  return data
}

export async function updateProxy(id: number, payload: UpdateProxyRequest): Promise<Proxy> {
  const { data } = await apiClient.put<Proxy>(`/account-proxies/${id}`, payload)
  return data
}

export async function deleteProxy(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/account-proxies/${id}`)
  return data
}

export async function testProxy(id: number): Promise<ProxyTestResult> {
  const { data } = await apiClient.post<ProxyTestResult>(`/account-proxies/${id}/test`, undefined, {
    timeout: 45000
  })
  return data
}

export async function checkProxyQuality(id: number): Promise<ProxyQualityCheckResult> {
  const { data } = await apiClient.post<ProxyQualityCheckResult>(`/account-proxies/${id}/quality-check`, undefined, {
    timeout: 90000
  })
  return data
}

export interface UserOAuthAuthUrlResponse {
  auth_url: string
  session_id: string
  state?: string
}

export interface UserOAuthProxyPayload {
  proxy_id?: number
}

export interface UserOAuthExchangeCodePayload {
  session_id: string
  code: string
  state?: string
  proxy_id?: number
  redirect_uri?: string
  oauth_type?: 'code_assist' | 'google_one' | 'ai_studio'
  tier_id?: string
}

export interface UserKiroAuthUrlPayload extends UserOAuthProxyPayload {
  provider?: string
}

export interface UserKiroIDCAuthUrlPayload extends UserOAuthProxyPayload {
  start_url?: string
  region?: string
}

export interface UserKiroExchangeCodePayload extends UserOAuthExchangeCodePayload {
  callback_path?: string
  login_option?: string
}

export interface UserGeminiAuthUrlPayload extends UserOAuthProxyPayload {
  project_id?: string
  oauth_type?: 'code_assist' | 'google_one' | 'ai_studio'
  tier_id?: string
}

export interface UserGeminiOAuthCapabilities {
  ai_studio_oauth_enabled: boolean
  required_redirect_uris: string[]
}

function compactPayload<T extends object>(payload?: T): Partial<T> {
  if (!payload) return {}
  return Object.fromEntries(
    Object.entries(payload as Record<string, unknown>).filter(([, value]) => value !== undefined && value !== null && value !== '')
  ) as Partial<T>
}

export async function generateAnthropicOAuthUrl(
  payload?: UserOAuthProxyPayload
): Promise<UserOAuthAuthUrlResponse> {
  const { data } = await apiClient.post<UserOAuthAuthUrlResponse>(
    '/account-oauth/anthropic/auth-url',
    compactPayload(payload)
  )
  return data
}

export async function exchangeAnthropicOAuthCode(
  payload: UserOAuthExchangeCodePayload
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>(
    '/account-oauth/anthropic/exchange-code',
    compactPayload(payload)
  )
  return data
}

export async function generateAnthropicSetupTokenUrl(
  payload?: UserOAuthProxyPayload
): Promise<UserOAuthAuthUrlResponse> {
  const { data } = await apiClient.post<UserOAuthAuthUrlResponse>(
    '/account-oauth/anthropic/setup-token/auth-url',
    compactPayload(payload)
  )
  return data
}

export async function exchangeAnthropicSetupTokenCode(
  payload: UserOAuthExchangeCodePayload
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>(
    '/account-oauth/anthropic/setup-token/exchange-code',
    compactPayload(payload)
  )
  return data
}

export async function anthropicCookieAuth(
  payload: { code: string; proxy_id?: number }
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>(
    '/account-oauth/anthropic/cookie-auth',
    compactPayload(payload)
  )
  return data
}

export async function anthropicSetupTokenCookieAuth(
  payload: { code: string; proxy_id?: number }
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>(
    '/account-oauth/anthropic/setup-token-cookie-auth',
    compactPayload(payload)
  )
  return data
}

export async function generateOpenAIOAuthUrl(
  payload?: UserOAuthProxyPayload & { redirect_uri?: string }
): Promise<UserOAuthAuthUrlResponse> {
  const { data } = await apiClient.post<UserOAuthAuthUrlResponse>(
    '/account-oauth/openai/auth-url',
    compactPayload(payload)
  )
  return data
}

export async function exchangeOpenAIOAuthCode(
  payload: UserOAuthExchangeCodePayload
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>(
    '/account-oauth/openai/exchange-code',
    compactPayload(payload)
  )
  return data
}

export async function refreshOpenAIToken(
  refreshToken: string,
  proxyId?: number | null,
  clientId?: string
): Promise<Record<string, unknown>> {
  const payload: { refresh_token: string; proxy_id?: number; client_id?: string } = {
    refresh_token: refreshToken
  }
  if (proxyId) payload.proxy_id = proxyId
  if (clientId) payload.client_id = clientId
  const { data } = await apiClient.post<Record<string, unknown>>(
    '/account-oauth/openai/refresh-token',
    compactPayload(payload)
  )
  return data
}

export async function getGeminiOAuthCapabilities(): Promise<UserGeminiOAuthCapabilities> {
  const { data } = await apiClient.get<UserGeminiOAuthCapabilities>(
    '/account-oauth/gemini/capabilities'
  )
  return data
}

export async function generateGeminiOAuthUrl(
  payload?: UserGeminiAuthUrlPayload
): Promise<UserOAuthAuthUrlResponse> {
  const { data } = await apiClient.post<UserOAuthAuthUrlResponse>(
    '/account-oauth/gemini/auth-url',
    compactPayload(payload)
  )
  return data
}

export async function exchangeGeminiOAuthCode(
  payload: UserOAuthExchangeCodePayload
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>(
    '/account-oauth/gemini/exchange-code',
    compactPayload(payload)
  )
  return data
}

export async function generateAntigravityOAuthUrl(
  payload?: UserOAuthProxyPayload
): Promise<UserOAuthAuthUrlResponse> {
  const { data } = await apiClient.post<UserOAuthAuthUrlResponse>(
    '/account-oauth/antigravity/auth-url',
    compactPayload(payload)
  )
  return data
}

export async function exchangeAntigravityOAuthCode(
  payload: UserOAuthExchangeCodePayload
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>(
    '/account-oauth/antigravity/exchange-code',
    compactPayload(payload)
  )
  return data
}

export async function refreshAntigravityToken(
  refreshToken: string,
  proxyId?: number | null
): Promise<Record<string, unknown>> {
  const payload: { refresh_token: string; proxy_id?: number } = { refresh_token: refreshToken }
  if (proxyId) payload.proxy_id = proxyId
  const { data } = await apiClient.post<Record<string, unknown>>(
    '/account-oauth/antigravity/refresh-token',
    compactPayload(payload)
  )
  return data
}

export async function generateKiroOAuthUrl(
  payload?: UserKiroAuthUrlPayload
): Promise<UserOAuthAuthUrlResponse> {
  const { data } = await apiClient.post<UserOAuthAuthUrlResponse>(
    '/account-oauth/kiro/auth-url',
    compactPayload(payload)
  )
  return data
}

export async function generateKiroIDCAuthUrl(
  payload?: UserKiroIDCAuthUrlPayload
): Promise<KiroIDCAuthUrlResponse> {
  const { data } = await apiClient.post<KiroIDCAuthUrlResponse>(
    '/account-oauth/kiro/idc-auth-url',
    compactPayload(payload)
  )
  return data
}

export async function exchangeKiroOAuthCode(
  payload: UserKiroExchangeCodePayload
): Promise<KiroTokenInfo> {
  const { data } = await apiClient.post<KiroTokenInfo>(
    '/account-oauth/kiro/exchange-code',
    compactPayload(payload)
  )
  return data
}

export async function refreshKiroToken(payload: {
  refresh_token: string
  auth_method?: string
  provider?: string
  client_id?: string
  client_secret?: string
  start_url?: string
  region?: string
  profile_arn?: string
  proxy_id?: number
}): Promise<KiroTokenInfo> {
  const { data } = await apiClient.post<KiroTokenInfo>(
    '/account-oauth/kiro/refresh-token',
    compactPayload(payload)
  )
  return data
}

export async function importKiroToken(payload: {
  token_json: string
  device_registration_json?: string
}): Promise<KiroTokenInfo> {
  const { data } = await apiClient.post<KiroTokenInfo>(
    '/account-oauth/kiro/import-token',
    compactPayload(payload)
  )
  return data
}

export const accountsAPI = {
  list,
  getById,
  getQuotaDashboard,
  create,
  importAccount,
  importCredentialContents,
  exportData,
  update,
  revalidatePublicShare,
  delete: deleteAccount,
  toggleStatus,
  bulkUpdate,
  bulkDelete,
  createBatchRefreshTask,
  createBatchRevalidatePublicShareTask,
  getBatchTask,
  getUsage,
  getStats,
  getTodayStats,
  getBatchTodayStats,
  listProxies,
  createProxy,
  updateProxy,
  deleteProxy,
  testProxy,
  checkProxyQuality,
  testAccount,
  refreshCredentials,
  setPrivacy,
  generateAnthropicOAuthUrl,
  exchangeAnthropicOAuthCode,
  generateAnthropicSetupTokenUrl,
  exchangeAnthropicSetupTokenCode,
  anthropicCookieAuth,
  anthropicSetupTokenCookieAuth,
  generateOpenAIOAuthUrl,
  exchangeOpenAIOAuthCode,
  refreshOpenAIToken,
  getGeminiOAuthCapabilities,
  generateGeminiOAuthUrl,
  exchangeGeminiOAuthCode,
  generateAntigravityOAuthUrl,
  exchangeAntigravityOAuthCode,
  refreshAntigravityToken,
  generateKiroOAuthUrl,
  generateKiroIDCAuthUrl,
  exchangeKiroOAuthCode,
  refreshKiroToken,
  importKiroToken
}

export default accountsAPI
