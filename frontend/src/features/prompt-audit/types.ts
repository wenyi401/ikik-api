export type PromptAuditMode = 'off' | 'async_audit' | 'blocking'
export type PromptAuditEnforcementMode = 'shadow' | 'enforce'
export type PromptDecision = 'pass' | 'flag' | 'critical'
export type PromptRiskLevel = 'low' | 'medium' | 'high' | 'critical'

export interface PromptAuditEndpoint {
  id: string
  name: string
  protocol: 'openai_compatible'
  base_url: string
  model: string
  timeout_ms: number
  input_limit: number
  enabled: boolean
  has_token: boolean
  token_status: 'configured' | 'missing' | string
}

export interface PromptAuditEndpointDraft extends PromptAuditEndpoint {
  token: string
  clear_token: boolean
}

export interface PromptAuditConfig {
  enabled: boolean
  blocking_enabled: boolean
  blocking_latest_turn_only: boolean
  async_latest_user_only: boolean
  enforcement_mode: PromptAuditEnforcementMode
  store_pass_events: boolean
  effective_mode: PromptAuditMode
  strategy: 'priority'
  worker_count: number
  queue_capacity: number
  scanners: string[]
  all_groups: boolean
  group_ids: number[]
  endpoints: PromptAuditEndpoint[]
  config_version: number
  updated_at: string
  updated_by: number
  change_summary: string
}

export interface PromptAuditDraft extends Omit<PromptAuditConfig, 'endpoints'> {
  endpoints: PromptAuditEndpointDraft[]
}

export interface PromptAuditUpdateRequest {
  expected_config_version: number
  enabled: boolean
  blocking_enabled: boolean
  blocking_latest_turn_only: boolean
  async_latest_user_only: boolean
  enforcement_mode: PromptAuditEnforcementMode
  store_pass_events: boolean
  strategy: 'priority'
  worker_count: number
  queue_capacity: number
  scanners: string[]
  all_groups: boolean
  group_ids: number[]
  endpoints: Array<{
    id: string
    name: string
    protocol: 'openai_compatible'
    base_url: string
    model: string
    token?: string
    clear_token: boolean
    timeout_ms: number
    input_limit: number
    enabled: boolean
  }>
}

export interface PromptProbeResult {
  ok: boolean
  status: string
  error_code?: string
  message: string
  latency_ms: number
  http_status: number
  retryable: boolean
  checked_at: string
  token_applied: boolean
}

export interface PromptAuditTestResult {
  result: {
    decision: PromptDecision
    risk_level: PromptRiskLevel
    action: 'Allow' | 'Warn' | 'Block'
    safety: string
    categories: string[]
    matched_scanners: string[]
    scanner_scores: Record<string, number>
    scanner_evidence: Record<string, string>
    scanner_backend: string
    scanner_version: string
    guard_endpoint_id: string
    policy_id: string
    policy_version: number
    chunk_total: number
    latency_ms: number
  }
  chunk_total: number
  latency_ms: number
}

export interface PromptQueueStats {
  staging: number
  queued: number
  processing: number
  retry: number
  done: number
  failed: number
  active: number
}

export interface PromptGuardMetrics {
  total: number
  allowed: number
  flagged: number
  blocked: number
  unavailable: number
  invalid: number
  timeouts: number
  failovers: number
  bulkhead_full: number
  record_failed: number
  latency_avg_ms?: number
  latency_p50_ms?: number
  latency_p95_ms?: number
  latency_p99_ms?: number
  latency_max_ms?: number
}

export interface PromptAuditRuntime {
  process_status: 'disabled' | 'running' | 'degraded' | 'error' | string
  effective_mode: PromptAuditMode
  expected_config_version: number
  active_config_version: number
  config_loaded_at?: string
  config_load_error?: string
  worker_total: number
  worker_active: number
  worker_heartbeat_at?: string
  queue_capacity: number
  queue: PromptQueueStats
  processed_total: number
  failed_total: number
  enqueued_total: number
  dropped_total: number
  last_processed_at?: string
  last_error_code?: string
  last_error_message?: string
  database_status: string
  redis_status: string
  endpoints: Record<string, PromptProbeResult>
  guard_metrics: PromptGuardMetrics
}

export interface PromptSnapshot {
  request_id: string
  user_id: number
  username: string
  user_email: string
  api_key_id: number
  api_key_name: string
  group_id?: number
  group_name: string
  provider: string
  endpoint: string
  protocol: string
  model: string
  prompt_hash: string
  redacted_preview: string
  full_prompt: string
  prompt_length: number
  message_count: number
  stage: string
}

export interface PromptIssueSummary {
  category: string
  scanner_id: string
  title: string
  description: string
  severity: string
  severity_label: string
  action: string
  action_label: string
  code: string
  score: number
  evidence: string
  evidence_hash: string
  start_rune?: number
  end_rune?: number
}

export interface PromptAuditEvent {
  id: number
  job_id: number
  snapshot: PromptSnapshot
  decision: PromptDecision
  risk_level: PromptRiskLevel
  action: 'Allow' | 'Warn' | 'Block' | string
  categories: string[]
  matched_scanners: string[]
  scanner_scores: Record<string, number>
  scanner_evidence: Record<string, string>
  scanner_backend: string
  scanner_version: string
  guard_endpoint_id: string
  policy_id: string
  policy_version: number
  config_version: number
  chunk_total: number
  latency_ms: number
  issue_summaries: PromptIssueSummary[]
  created_at: string
}

export interface PromptEventFilters {
  decision: string
  risk_level: string
  endpoint: string
  group_id: string
  user_id: string
  api_key_id: string
  request_id: string
  prompt_hash: string
  keyword: string
  start_at: string
  end_at: string
}

export interface PromptEventPage {
  items: PromptAuditEvent[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface PromptDeleteResult {
  deleted_events: number
  deleted_jobs: number
}

export interface PromptDeletePreview {
  matched_count: number
  filter_summary: Record<string, unknown>
  snapshot_max_id: number
  filter_hash: string
  confirmation_token: string
  expires_at: string
}

export interface PromptAuditGroup {
  id: number
  name: string
  status: 'active' | 'inactive'
  platform: string
}

export interface PromptAuditUserProfile {
	user_id: number
	user_email: string
	total_requests: number
	remote_audits: number
	flagged_requests: number
	risk_score: number
	risk_level: 'new' | 'normal' | 'trusted' | 'watch' | 'high' | 'critical' | string
	blocked: boolean
	current_sample_rate: number
	last_category: string
	last_hit_at?: string
	last_audited_at?: string
	blocked_at?: string
	updated_at: string
}

export interface PromptAuditUserProfilePage {
	items: PromptAuditUserProfile[]
	total: number
	page: number
	page_size: number
	pages: number
}

export interface PromptLoadErrors {
  config: string
  runtime: string
  groups: string
  events: string
	profiles: string
}

export type RiskKnowledgeTopic =
  | 'cheat_development' | 'cheat_usage' | 'reverse_engineering' | 'license_cracking'
  | 'detection_bypass' | 'account_automation' | 'third_party_scripts'
  | 'credential_abuse' | 'benign_research'
export type RiskDomainCategory =
  | 'none' | 'cheat_automation' | 'auth_reverse_engineering' | 'exploit_reverse_engineering'
  | 'credential_theft' | 'safety_bypass' | 'account_automation' | 'cyber_abuse'
export type RiskKnowledgeDisposition = 'safe' | 'review' | 'risk'
export type RiskKnowledgeIntent = 'neutral' | 'educational' | 'defensive' | 'operational' | 'evasion' | 'unknown'
export type RiskKnowledgeActionability = 'none' | 'low' | 'medium' | 'high'
export type RiskKnowledgeAuthorization = 'authorized' | 'unauthorized' | 'unknown'
export type RiskObservationReviewStatus = 'unreviewed' | 'confirmed' | 'false_positive' | 'inconclusive'

export interface RiskKnowledgeEntry {
  id: number
  entry_key: string
  topic: RiskKnowledgeTopic
  category: RiskDomainCategory
  disposition: RiskKnowledgeDisposition
  intent: RiskKnowledgeIntent
  actionability: RiskKnowledgeActionability
  authorization: RiskKnowledgeAuthorization
  language: 'zh' | 'en' | 'multilingual'
  title: string
  example_text: string
  aliases: string[]
  rationale: string
  enabled: boolean
  source_type: 'seed' | 'admin' | 'audit_review'
  source_event_id?: number
  revision: number
  created_by?: number
  updated_by?: number
  created_at: string
  updated_at: string
}

export type RiskKnowledgeWriteInput = Omit<RiskKnowledgeEntry,
  'id' | 'entry_key' | 'revision' | 'created_by' | 'updated_by' | 'created_at' | 'updated_at'>

export interface RiskKnowledgeSummary {
  version: number
  total: number
  enabled: number
  risk: number
  safe: number
  review: number
  unreviewed_observations: number
  topic_counts: Record<string, number>
  last_updated?: string
}

export interface RiskKnowledgeEntryPage {
  items: RiskKnowledgeEntry[]
  total: number
  page: number
  page_size: number
  pages: number
  version: number
}

export interface RiskKnowledgeMatch {
  entry_id: number
  entry_key: string
  topic: RiskKnowledgeTopic
  category: RiskDomainCategory
  disposition: RiskKnowledgeDisposition
  intent: RiskKnowledgeIntent
  actionability: RiskKnowledgeActionability
  authorization: RiskKnowledgeAuthorization
  score: number
  evidence: string
  title: string
}

export interface RiskObservation {
  id: number
  request_id: string
  user_id?: number
  group_id?: number
  incident_fingerprint: string
  candidate: {
    review: boolean
    signals: string[]
    confidence: number
    knowledge_version: number
    knowledge_matches: RiskKnowledgeMatch[]
  }
  adjudication: {
    schema_version: number
    verdict: 'safe' | 'review' | 'confirmed' | 'abstain'
    category: RiskDomainCategory
    intent: RiskKnowledgeIntent
    actionability: RiskKnowledgeActionability
    authorization: RiskKnowledgeAuthorization
    confidence: number
    evidence: Array<{ quote: string; signal: string }>
    reason_code: string
    model?: string
  }
  recommendation: string
  would_protect: boolean
  would_strike: boolean
  reason_code: string
  policy_version: number
  adjudicator_model: string
  knowledge_version: number
  knowledge_match_ids: number[]
  mode: 'shadow'
  review_status: RiskObservationReviewStatus
  review_label?: {
    topic?: RiskKnowledgeTopic
    category?: RiskDomainCategory
    disposition?: RiskKnowledgeDisposition
  }
  reviewed_by?: number
  reviewed_at?: string
  review_note: string
  observed_at: string
  created_at: string
  audit: {
    event_id?: number
    username: string
    group_name: string
    model: string
    redacted_preview: string
    full_prompt: string
    guard_decision: string
    guard_risk_level: string
  }
}

export interface RiskObservationPage {
  items: RiskObservation[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface RiskObservationReviewInput {
  status: Exclude<RiskObservationReviewStatus, 'unreviewed'>
  topic?: RiskKnowledgeTopic
  category?: RiskDomainCategory
  note: string
}
