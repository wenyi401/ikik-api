import { apiClient } from '../client'

export type ModerationMode = 'off' | 'observe' | 'pre_block' | 'adaptive'
export type KeywordBlockingMode = 'keyword_only' | 'keyword_and_api' | 'api_only'
export type ContentModerationModelFilterType = 'all' | 'include' | 'exclude'
export type ContentModerationProvider = 'openai' | 'aliyun_guardrails' | 'model_classifier'

export type ContentModerationEnforcementMode = 'shadow' | 'notify' | 'enforce'
export type ContentModerationRiskLevel = 'new' | 'normal' | 'trusted' | 'watch' | 'high' | 'critical'
export type ContentModerationManualLevel = 'auto' | 'trusted' | 'watch' | 'high' | 'critical'

export interface ContentModerationAdaptivePolicy {
  enforcement_mode: ContentModerationEnforcementMode
  full_audit_requests: number
  ramp_audit_requests: number
  ramp_sample_rate: number
  trusted_sample_rate: number
  watch_sample_rate: number
  high_risk_sample_rate: number
  daily_decay_percent: number
  low_risk_weight: number
  medium_risk_weight: number
  severe_risk_weight: number
  watch_threshold: number
  high_risk_threshold: number
  critical_threshold: number
  notification_cooldown_hours: number
}

export interface ContentModerationGroupPenaltyPolicy {
  enabled: boolean
  target_group_ids: number[]
  categories: string[]
  category_thresholds: Record<string, number>
  first_block_hours: number
  second_block_hours: number
}

export interface ContentModerationGroupPenaltyCategoryOption {
  category: string
  label_zh: string
  label_en: string
}

export interface ContentModerationModelFilter {
  type: ContentModerationModelFilterType
  models: string[]
}

export interface ContentModerationConfig {
  enabled: boolean
  mode: ModerationMode
  moderation_provider: ContentModerationProvider
  base_url: string
  model: string
  classifier_group_id: number
  classifier_models: string[]
  classifier_prompt: string
  classifier_prompt_default: string
  aliyun_region_id: string
  aliyun_endpoint: string
  aliyun_service: string
  api_key_configured: boolean
  api_key_masked: string
  api_key_count: number
  api_key_masks: string[]
  api_key_statuses: ContentModerationAPIKeyStatus[]
  timeout_ms: number
  sample_rate: number
  all_groups: boolean
  group_ids: number[]
  record_non_hits: boolean
  thresholds: Record<string, number>
  worker_count: number
  queue_size: number
  block_status: number
  block_message: string
  email_on_hit: boolean
  auto_ban_enabled: boolean
  ban_threshold: number
  violation_window_hours: number
  retry_count: number
  hit_retention_days: number
  non_hit_retention_days: number
  pre_hash_check_enabled: boolean
  blocked_keywords: string[]
  keyword_blocking_mode: KeywordBlockingMode
  model_filter: ContentModerationModelFilter
  adaptive_policy: ContentModerationAdaptivePolicy
  group_penalty?: ContentModerationGroupPenaltyPolicy
  group_penalty_category_options?: ContentModerationGroupPenaltyCategoryOption[]
  cyber_policy_exclude_from_ban_count: boolean
}

export type ContentModerationAPIKeyStatusValue = 'unknown' | 'ok' | 'error' | 'frozen'

export interface ContentModerationAPIKeyStatus {
  index: number
  key_hash: string
  masked: string
  status: ContentModerationAPIKeyStatusValue
  failure_count: number
  success_count: number
  last_error: string
  last_checked_at?: string
  frozen_until?: string
  last_latency_ms: number
  last_http_status: number
  last_tested: boolean
  configured: boolean
}

export interface TestContentModerationAPIKeysPayload {
  api_keys?: string[]
  moderation_provider?: ContentModerationProvider
  base_url?: string
  model?: string
  classifier_group_id?: number
  classifier_models?: string[]
  classifier_prompt?: string
  aliyun_region_id?: string
  aliyun_endpoint?: string
  aliyun_service?: string
  timeout_ms?: number
  prompt?: string
  images?: string[]
}

export interface TestContentModerationAPIKeysResponse {
  items: ContentModerationAPIKeyStatus[]
  audit_result?: ContentModerationTestAuditResult
  classifier_trace?: ContentModerationClassifierTrace
  image_count: number
}

export interface ContentModerationClassifierAttempt {
  model: string
  status_code: number
  latency_ms: number
  success: boolean
  error?: string
}

export interface ContentModerationClassifierTrace {
  group_id: number
  group_name?: string
  attempts: ContentModerationClassifierAttempt[]
  error?: string
}

export interface ContentModerationTestAuditResult {
  flagged: boolean
  highest_category: string
  highest_score: number
  composite_score: number
  category_scores: Record<string, number>
  thresholds: Record<string, number>
  classifier_model?: string
}

export interface UpdateContentModerationConfig {
  enabled?: boolean
  mode?: ModerationMode
  moderation_provider?: ContentModerationProvider
  base_url?: string
  model?: string
  classifier_group_id?: number
  classifier_models?: string[]
  classifier_prompt?: string
  aliyun_region_id?: string
  aliyun_endpoint?: string
  aliyun_service?: string
  api_key?: string
  api_keys?: string[]
  api_keys_mode?: 'append' | 'replace'
  delete_api_key_hashes?: string[]
  clear_api_key?: boolean
  timeout_ms?: number
  sample_rate?: number
  all_groups?: boolean
  group_ids?: number[]
  record_non_hits?: boolean
  thresholds?: Record<string, number>
  worker_count?: number
  queue_size?: number
  block_status?: number
  block_message?: string
  email_on_hit?: boolean
  auto_ban_enabled?: boolean
  ban_threshold?: number
  violation_window_hours?: number
  retry_count?: number
  hit_retention_days?: number
  non_hit_retention_days?: number
  pre_hash_check_enabled?: boolean
  blocked_keywords?: string[]
  keyword_blocking_mode?: KeywordBlockingMode
  model_filter?: ContentModerationModelFilter
  adaptive_policy?: ContentModerationAdaptivePolicy
  group_penalty?: ContentModerationGroupPenaltyPolicy
  cyber_policy_exclude_from_ban_count?: boolean
}

export interface ContentModerationRuntimeStatus {
  enabled: boolean
  risk_control_enabled: boolean
  mode: ModerationMode
  worker_count: number
  max_workers: number
  active_workers: number
  idle_workers: number
  queue_size: number
  queue_length: number
  queue_usage_percent: number
  enqueued: number
  dropped: number
  processed: number
  errors: number
  pre_block_active: number
  pre_block_checked: number
  pre_block_allowed: number
  pre_block_blocked: number
  pre_block_errors: number
  pre_block_avg_latency_ms: number
  pre_block_api_key_active: number
  pre_block_api_key_available_count: number
  pre_block_api_key_total_calls: number
  pre_block_api_key_loads: ContentModerationAPIKeyLoad[]
  api_key_statuses: ContentModerationAPIKeyStatus[]
  flagged_hash_count: number
  last_cleanup_at?: string
  last_cleanup_deleted_hit: number
  last_cleanup_deleted_non_hit: number
}

export interface ContentModerationAPIKeyLoad {
  index: number
  key_hash: string
  masked: string
  status: ContentModerationAPIKeyStatusValue
  active: number
  total: number
  success: number
  errors: number
  avg_latency_ms: number
  last_latency_ms: number
  last_http_status: number
}

export interface ContentModerationLog {
  id: number
  request_id: string
  user_id: number | null
  user_email: string
  api_key_id: number | null
  api_key_name: string
  group_id: number | null
  group_name: string
  endpoint: string
  provider: string
  model: string
  mode: string
  action: string
  flagged: boolean
  highest_category: string
  highest_score: number
  matched_keyword: string
  category_scores: Record<string, number>
  threshold_snapshot: Record<string, number>
  input_excerpt: string
  input_content?: string
  upstream_latency_ms: number | null
  error: string
  violation_count: number
  auto_banned: boolean
  email_sent: boolean
  user_status: string
  queue_delay_ms: number | null
  created_at: string
}

export interface ListContentModerationLogsParams {
  page?: number
  page_size?: number
  result?: string
  group_id?: number
  endpoint?: string
  search?: string
  from?: string
  to?: string
}

export interface ContentModerationLogsResponse {
  items: ContentModerationLog[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface ContentModerationUnbanUserResponse {
  user_id: number
  status: string
  released_group_penalties: number
}

export interface ContentModerationGroupPenalty {
  user_id: number
  user_email: string
  username: string
  user_status: string
  group_id: number
  group_name: string
  group_platform: string
  strike_count: number
  blocked_until?: string
  permanent: boolean
  last_category: string
  last_request_id: string
  last_score: number
  active: boolean
  created_at: string
  updated_at: string
}

export interface ContentModerationGroupPenaltyEvent {
  id: number
  user_id: number
  group_id: number
  request_id: string
  category: string
  score: number
  created_at: string
}

export interface ContentModerationGroupPenaltyOverview {
  total: number
  active: number
  expired: number
  permanent: number
  today_events: number
}

export interface ContentModerationGroupPenaltiesResponse {
  items: ContentModerationGroupPenalty[]
  overview: ContentModerationGroupPenaltyOverview
  total: number
  page: number
  page_size: number
  pages: number
}

export interface ContentModerationGroupPenaltyEventsResponse {
  items: ContentModerationGroupPenaltyEvent[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface ContentModerationGroupPenaltyActionResponse {
  user_id: number
  group_id: number
  affected: boolean
  strike_count: number
}

export interface ListContentModerationGroupPenaltiesParams {
  page?: number
  page_size?: number
  status?: 'all' | 'active' | 'expired' | 'permanent'
  search?: string
  category?: string
  group_id?: number
}

export interface ContentModerationRiskProfile {
  user_id: number
  user_email: string
  user_status: string
  total_requests: number
  audited_requests: number
  flagged_requests: number
  risk_score: number
  risk_level: ContentModerationRiskLevel
  manual_level: ContentModerationManualLevel
  current_sample_rate: number
  last_category: string
  last_score_delta: number
  last_hit_at?: string
  last_audited_at?: string
  last_notified_at?: string
  score_updated_at: string
  created_at: string
  updated_at: string
}

export interface ContentModerationRiskOverview {
  total_profiles: number
  new_profiles: number
  trusted_profiles: number
  watch_profiles: number
  high_profiles: number
  critical_profiles: number
  audited_requests: number
  flagged_requests: number
  average_risk_score: number
}

export interface ContentModerationRiskProfilesResponse {
  items: ContentModerationRiskProfile[]
  overview: ContentModerationRiskOverview
  total: number
  page: number
  page_size: number
  pages: number
}

export interface ListContentModerationRiskProfilesParams {
  page?: number
  page_size?: number
  level?: string
  search?: string
}

export interface UpdateContentModerationRiskProfilePayload {
  manual_level?: ContentModerationManualLevel
  reset_score?: boolean
}

export interface DeleteFlaggedHashResponse {
  input_hash: string
  deleted: boolean
}

export interface ClearFlaggedHashesResponse {
  deleted: number
}

export async function getConfig(): Promise<ContentModerationConfig> {
  const { data } = await apiClient.get<ContentModerationConfig>('/admin/risk-control/config')
  return data
}

export async function updateConfig(
  payload: UpdateContentModerationConfig
): Promise<ContentModerationConfig> {
  const { data } = await apiClient.put<ContentModerationConfig>('/admin/risk-control/config', payload)
  return data
}

export async function getStatus(): Promise<ContentModerationRuntimeStatus> {
  const { data } = await apiClient.get<ContentModerationRuntimeStatus>('/admin/risk-control/status')
  return data
}

export async function testAPIKeys(
  payload: TestContentModerationAPIKeysPayload = {}
): Promise<TestContentModerationAPIKeysResponse> {
  const { data } = await apiClient.post<TestContentModerationAPIKeysResponse>('/admin/risk-control/api-keys/test', payload)
  return data
}

export async function listLogs(
  params: ListContentModerationLogsParams = {}
): Promise<ContentModerationLogsResponse> {
  const { data } = await apiClient.get<ContentModerationLogsResponse>('/admin/risk-control/logs', {
    params,
  })
  return data
}

export async function getLog(logID: number): Promise<ContentModerationLog> {
  const { data } = await apiClient.get<ContentModerationLog>(`/admin/risk-control/logs/${logID}`)
  return data
}

export async function unbanUser(userID: number): Promise<ContentModerationUnbanUserResponse> {
  const { data } = await apiClient.post<ContentModerationUnbanUserResponse>(
    `/admin/risk-control/users/${userID}/unban`
  )
  return data
}

export async function listGroupPenalties(
  params: ListContentModerationGroupPenaltiesParams = {}
): Promise<ContentModerationGroupPenaltiesResponse> {
  const { data } = await apiClient.get<ContentModerationGroupPenaltiesResponse>(
    '/admin/risk-control/group-penalties',
    { params }
  )
  return data
}

export async function listGroupPenaltyEvents(
  userID: number,
  groupID: number,
  page = 1,
  pageSize = 20
): Promise<ContentModerationGroupPenaltyEventsResponse> {
  const { data } = await apiClient.get<ContentModerationGroupPenaltyEventsResponse>(
    `/admin/risk-control/group-penalties/${userID}/${groupID}/events`,
    { params: { page, page_size: pageSize } }
  )
  return data
}

export async function releaseGroupPenalty(
  userID: number,
  groupID: number
): Promise<ContentModerationGroupPenaltyActionResponse> {
  const { data } = await apiClient.post<ContentModerationGroupPenaltyActionResponse>(
    `/admin/risk-control/group-penalties/${userID}/${groupID}/release`
  )
  return data
}

export async function resetGroupPenalty(
  userID: number,
  groupID: number
): Promise<ContentModerationGroupPenaltyActionResponse> {
  const { data } = await apiClient.delete<ContentModerationGroupPenaltyActionResponse>(
    `/admin/risk-control/group-penalties/${userID}/${groupID}`
  )
  return data
}

export async function listRiskProfiles(
  params: ListContentModerationRiskProfilesParams = {}
): Promise<ContentModerationRiskProfilesResponse> {
  const { data } = await apiClient.get<ContentModerationRiskProfilesResponse>('/admin/risk-control/risk-profiles', {
    params,
  })
  return data
}

export async function updateRiskProfile(
  userID: number,
  payload: UpdateContentModerationRiskProfilePayload
): Promise<ContentModerationRiskProfile> {
  const { data } = await apiClient.patch<ContentModerationRiskProfile>(
    `/admin/risk-control/risk-profiles/${userID}`,
    payload
  )
  return data
}

export async function deleteFlaggedHash(inputHash: string): Promise<DeleteFlaggedHashResponse> {
  const { data } = await apiClient.delete<DeleteFlaggedHashResponse>('/admin/risk-control/hashes', {
    data: { input_hash: inputHash },
  })
  return data
}

export async function clearFlaggedHashes(): Promise<ClearFlaggedHashesResponse> {
  const { data } = await apiClient.delete<ClearFlaggedHashesResponse>('/admin/risk-control/hashes/all')
  return data
}

export const riskControlAPI = {
  getConfig,
  updateConfig,
  getStatus,
  testAPIKeys,
  listLogs,
  getLog,
  listRiskProfiles,
  updateRiskProfile,
  unbanUser,
  listGroupPenalties,
  listGroupPenaltyEvents,
  releaseGroupPenalty,
  resetGroupPenalty,
  deleteFlaggedHash,
  clearFlaggedHashes,
}

export default riskControlAPI
