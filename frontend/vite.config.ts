import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import checker from 'vite-plugin-checker'
import { resolve } from 'path'
import type { Plugin } from 'vite'
import type { IncomingMessage, ServerResponse } from 'http'

const mockUser = {
  id: 1,
  username: 'local-admin',
  email: 'admin@local.test',
  role: 'admin',
  balance: 1000,
  recharge_balance: 1000,
  invite_income_balance: 0,
  share_income_balance: 0,
  points_balance: 0,
  prefer_points_billing: false,
  concurrency: 10,
  rpm_limit: 0,
  status: 'active',
  allowed_groups: null,
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  run_mode: 'standard'
}

const mockGroups = [
  {
    id: 1,
    name: 'OpenAI Local',
    description: 'Local mock OpenAI group',
    platform: 'openai',
    rate_multiplier: 1,
    rpm_limit: 0,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'standard',
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    image_price_1k: null,
    image_price_2k: null,
    image_price_4k: null,
    claude_code_only: false,
    fallback_group_id: null,
    fallback_group_id_on_invalid_request: null,
    default_mapped_model: 'gpt-5.5',
    models_list_config: {
      enabled: true,
      models: ['gpt-5.5', 'gpt-5.4-mini', 'ik-auto-pro']
    },
    require_oauth_only: false,
    require_privacy_set: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
  },
  {
    id: 2,
    name: 'Claude Local',
    description: 'Local mock Claude group',
    platform: 'anthropic',
    rate_multiplier: 1,
    rpm_limit: 0,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'standard',
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    image_price_1k: null,
    image_price_2k: null,
    image_price_4k: null,
    claude_code_only: false,
    fallback_group_id: null,
    fallback_group_id_on_invalid_request: null,
    default_mapped_model: 'claude-sonnet-4.6',
    models_list_config: {
      enabled: true,
      models: ['claude-sonnet-4.6', 'claude-opus-4.7']
    },
    require_oauth_only: false,
    require_privacy_set: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
  }
]

const mockPublicSettings = {
  registration_enabled: true,
  email_verify_enabled: false,
  force_email_on_third_party_signup: false,
  registration_email_suffix_whitelist: [],
  promo_code_enabled: true,
  password_reset_enabled: true,
  invitation_code_enabled: false,
  turnstile_enabled: false,
  turnstile_site_key: '',
  site_name: 'ikik-api Local',
  site_logo: '',
  site_subtitle: 'Local mock preview',
  api_base_url: '',
  contact_info: '',
  doc_url: '',
  home_content: '',
  hide_ccs_import_button: false,
  payment_enabled: true,
  purchase_subscription_enabled: false,
  purchase_subscription_url: '',
  risk_control_enabled: true,
  table_default_page_size: 20,
  table_page_size_options: [10, 20, 50, 100, 1000],
  custom_menu_items: [],
  custom_endpoints: [],
  linuxdo_oauth_enabled: false,
  wechat_oauth_enabled: false,
  wechat_oauth_open_enabled: false,
  wechat_oauth_mp_enabled: false,
  wechat_oauth_mobile_enabled: false,
  oidc_oauth_enabled: false,
  oidc_oauth_provider_name: 'OIDC',
  github_oauth_enabled: true,
  google_oauth_enabled: true,
  backend_mode_enabled: false,
  version: '1.0.2-local',
  balance_low_notify_enabled: false,
  account_quota_notify_enabled: false,
  balance_low_notify_threshold: 0,
  channel_monitor_enabled: true,
  free_models_enabled: true,
  channel_monitor_default_interval_seconds: 60,
  available_channels_enabled: true,
  carpool_enabled: true,
  carpool_base_service_fee_usd: 75,
  carpool_system_proxy_fee_usd: 10,
  carpool_risk_control_fee_usd: 15,
  affiliate_enabled: true
}

let mockAccountID = 1000
let mockProxyID = 100
let mockBatchTaskID = 1
let mockApiKeyID = 2000
let mockUsageLogID = 3000
const mockAccounts: Array<Record<string, unknown>> = []
const mockProxies: Array<Record<string, unknown>> = []
const mockApiKeys: Array<Record<string, unknown>> = []
const mockUsageLogs: Array<Record<string, unknown>> = []
let mockRiskConfig: Record<string, unknown> = {
  enabled: true,
  mode: 'adaptive',
  moderation_provider: 'model_classifier',
  base_url: 'https://api.example.com',
  model: 'gpt-5.3-codex-spark',
  classifier_prompt: '',
  classifier_prompt_default: 'Evaluate the latest end-user content against safety, privacy, minor safety, fraud, high-stakes automation, cyber abuse, and gateway abuse policies. Use context and authorization; do not classify by isolated keywords.',
  aliyun_region_id: 'cn-shanghai',
  aliyun_endpoint: 'green-cip.cn-shanghai.aliyuncs.com',
  aliyun_service: 'query_security_check_cb',
  api_key_configured: true,
  api_key_masked: 'sk-****local',
  api_key_count: 1,
  api_key_masks: ['sk-****local'],
  api_key_statuses: [],
  timeout_ms: 5000,
  sample_rate: 100,
  all_groups: true,
  group_ids: [],
  record_non_hits: false,
  thresholds: {
    'gateway_abuse/safety_bypass': 0.8,
    'gateway_abuse/credential_theft': 0.8,
    'gateway_abuse/account_automation': 0.8,
    'gateway_abuse/auth_reverse_engineering': 0.8,
    'gateway_abuse/other': 0.85
  },
  worker_count: 4,
  queue_size: 32768,
  block_status: 403,
  block_message: 'Content policy violation',
  email_on_hit: true,
  auto_ban_enabled: false,
  ban_threshold: 10,
  violation_window_hours: 720,
  retry_count: 1,
  hit_retention_days: 180,
  non_hit_retention_days: 3,
  pre_hash_check_enabled: true,
  blocked_keywords: [],
  keyword_blocking_mode: 'api_only',
  model_filter: { type: 'all', models: [] },
  adaptive_policy: {
    enforcement_mode: 'shadow',
    full_audit_requests: 100,
    ramp_audit_requests: 300,
    ramp_sample_rate: 30,
    trusted_sample_rate: 5,
    watch_sample_rate: 50,
    high_risk_sample_rate: 100,
    daily_decay_percent: 10,
    low_risk_weight: 4,
    medium_risk_weight: 12,
    severe_risk_weight: 30,
    watch_threshold: 40,
    high_risk_threshold: 60,
    critical_threshold: 80,
    notification_cooldown_hours: 24
  }
}

const mockRiskProfiles: Array<Record<string, unknown>> = [
  {
    user_id: 42, user_email: 'research@example.com', user_status: 'active', total_requests: 486,
    audited_requests: 122, flagged_requests: 0, risk_score: 2.4, risk_level: 'trusted', manual_level: 'auto',
    current_sample_rate: 5, last_category: '', last_score_delta: 0, last_audited_at: new Date(Date.now() - 36 * 60 * 1000).toISOString(),
    score_updated_at: nowISO(), created_at: nowISO(), updated_at: nowISO()
  },
  {
    user_id: 83, user_email: 'new-user@example.com', user_status: 'active', total_requests: 64,
    audited_requests: 64, flagged_requests: 1, risk_score: 12, risk_level: 'new', manual_level: 'auto',
    current_sample_rate: 100, last_category: 'gateway_abuse/other', last_score_delta: 4,
    last_hit_at: new Date(Date.now() - 4 * 60 * 60 * 1000).toISOString(), last_audited_at: nowISO(),
    score_updated_at: nowISO(), created_at: nowISO(), updated_at: nowISO()
  },
  {
    user_id: 109, user_email: 'watch@example.com', user_status: 'active', total_requests: 822,
    audited_requests: 248, flagged_requests: 4, risk_score: 47.6, risk_level: 'watch', manual_level: 'auto',
    current_sample_rate: 50, last_category: 'gateway_abuse/auth_reverse_engineering', last_score_delta: 12,
    last_hit_at: new Date(Date.now() - 70 * 60 * 1000).toISOString(), last_audited_at: nowISO(),
    score_updated_at: nowISO(), created_at: nowISO(), updated_at: nowISO()
  },
  {
    user_id: 128, user_email: 'critical@example.com', user_status: 'active', total_requests: 315,
    audited_requests: 191, flagged_requests: 7, risk_score: 86.2, risk_level: 'critical', manual_level: 'auto',
    current_sample_rate: 100, last_category: 'gateway_abuse/credential_theft', last_score_delta: 30,
    last_hit_at: new Date(Date.now() - 12 * 60 * 1000).toISOString(), last_audited_at: nowISO(),
    score_updated_at: nowISO(), created_at: nowISO(), updated_at: nowISO()
  }
]

function nowISO(): string {
  return new Date().toISOString()
}

function localOrigin(req: IncomingMessage): string {
  const host = req.headers.host || '127.0.0.1:3000'
  const proto = req.headers['x-forwarded-proto'] || 'http'
  return `${proto}://${host}`
}

function localPublicSettings(req: IncomingMessage): Record<string, unknown> {
  return {
    ...mockPublicSettings,
    api_base_url: localOrigin(req)
  }
}

function parseJsonBody<T extends Record<string, unknown>>(body: string): T {
  try {
    return JSON.parse(body || '{}') as T
  } catch {
    return {} as T
  }
}

function paginateItems<T>(items: T[], url: URL): Record<string, unknown> {
  const page = Math.max(1, Number(url.searchParams.get('page') || 1))
  const pageSize = Math.max(1, Number(url.searchParams.get('page_size') || 20))
  const start = (page - 1) * pageSize
  const sliced = items.slice(start, start + pageSize)
  return {
    items: sliced,
    total: items.length,
    page,
    page_size: pageSize,
    pages: Math.max(1, Math.ceil(items.length / pageSize))
  }
}

function createMockAccount(payload: Record<string, unknown>): Record<string, unknown> {
  const createdAt = nowISO()
  const platform = String(payload.platform || 'openai')
  const type = String(payload.type || 'apikey')
  const account = {
    id: ++mockAccountID,
    name: String(payload.name || `${platform} local account`),
    notes: payload.notes ?? null,
    platform,
    account_level: payload.account_level || 'free',
    type,
    credentials: payload.credentials || {},
    extra: payload.extra || {},
    proxy_id: payload.proxy_id ?? null,
    proxy_fallback_origin_id: null,
    proxy_fallback_origin_name: null,
    owner_user_id: mockUser.id,
    share_mode: payload.share_mode || 'private',
    share_status: 'none',
    share_policy_id: null,
    concurrency: payload.concurrency || 1,
    load_factor: payload.load_factor ?? null,
    current_concurrency: 0,
    priority: payload.priority || 50,
    rate_multiplier: payload.rate_multiplier ?? 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: payload.expires_at ?? null,
    auto_pause_on_expired: payload.auto_pause_on_expired ?? true,
    created_at: createdAt,
    updated_at: createdAt,
    proxy: null,
    group_ids: [],
    groups: [],
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    window_cost_limit: null,
    window_cost_sticky_reserve: null,
    max_sessions: null,
    session_idle_timeout_minutes: null,
    base_rpm: null,
    rpm_strategy: null,
    rpm_sticky_buffer: null,
    user_msg_queue_mode: null,
    enable_tls_fingerprint: null,
    tls_fingerprint_profile_id: null,
    session_id_masking_enabled: null,
    cache_ttl_override_enabled: null
  }
  mockAccounts.unshift(account)
  return account
}

function resolveMockGroup(groupId: unknown): Record<string, unknown> | null {
  if (groupId === null || groupId === undefined || groupId === '') {
    return null
  }
  return mockGroups.find((group) => group.id === Number(groupId)) || null
}

function createMockApiKey(payload: Record<string, unknown> = {}): Record<string, unknown> {
  const createdAt = nowISO()
  const group = resolveMockGroup(payload.group_id ?? 1)
  const key = {
    id: ++mockApiKeyID,
    user_id: mockUser.id,
    key: String(payload.custom_key || `sk-local-${mockApiKeyID.toString(16)}${'a'.repeat(42)}`),
    name: String(payload.name || `Local Key ${mockApiKeyID}`),
    group_id: group ? group.id : null,
    group_routes: payload.group_routes || (group ? [{
      group_id: group.id,
      priority: 100,
      weight: 1,
      enabled: true,
      cooldown_seconds: 0,
      group
    }] : []),
    status: payload.status || 'active',
    ip_whitelist: payload.ip_whitelist || [],
    ip_blacklist: payload.ip_blacklist || [],
    last_used_at: payload.last_used_at || new Date(Date.now() - 36 * 60 * 1000).toISOString(),
    quota: payload.quota ?? 100,
    quota_used: payload.quota_used ?? 18.42,
    expires_at: payload.expires_at ?? null,
    created_at: createdAt,
    updated_at: createdAt,
    current_concurrency: payload.current_concurrency ?? 0,
    group,
    rate_limit_5h: payload.rate_limit_5h ?? 0,
    rate_limit_1d: payload.rate_limit_1d ?? 0,
    rate_limit_7d: payload.rate_limit_7d ?? 0,
    usage_5h: payload.usage_5h ?? 0,
    usage_1d: payload.usage_1d ?? 0,
    usage_7d: payload.usage_7d ?? 0,
    window_5h_start: null,
    window_1d_start: null,
    window_7d_start: null,
    reset_5h_at: null,
    reset_1d_at: null,
    reset_7d_at: null
  }
  mockApiKeys.unshift(key)
  return key
}

function seedMockApiKeys(): void {
  if (mockApiKeys.length > 0) return
  createMockApiKey({
    name: 'Production Gateway',
    group_id: 1,
    quota: 300,
    quota_used: 42.35,
    current_concurrency: 2,
    rate_limit_5h: 50,
    rate_limit_7d: 200,
    usage_5h: 12.4,
    usage_7d: 64.8
  })
  createMockApiKey({
    name: 'Claude Workspace',
    group_id: 2,
    quota: 0,
    quota_used: 0,
    ip_whitelist: ['203.0.113.12'],
    current_concurrency: 1
  })
  createMockApiKey({
    name: 'Mobile Test Key',
    group_id: null,
    status: 'inactive',
    quota: 50,
    quota_used: 6.7
  })
}

function createMockUsageLog(payload: Record<string, unknown>): void {
  const apiKey = mockApiKeys.find((key) => Number(key.id) === Number(payload.api_key_id)) || mockApiKeys[0]
  const inputTokens = Number(payload.input_tokens || 0)
  const outputTokens = Number(payload.output_tokens || 0)
  const cacheCreationTokens = Number(payload.cache_creation_tokens || 0)
  const cacheReadTokens = Number(payload.cache_read_tokens || 0)
  const totalCost = Number(payload.total_cost || 0)
  const actualCost = Number(payload.actual_cost ?? totalCost)

  mockUsageLogs.push({
    id: ++mockUsageLogID,
    user_id: mockUser.id,
    api_key_id: Number(apiKey?.id || 0),
    account_id: Number(payload.account_id || 6459),
    request_id: String(payload.request_id || `req-local-${mockUsageLogID}`),
    model: String(payload.model || 'gpt-5.5'),
    service_tier: payload.service_tier || 'default',
    reasoning_effort: payload.reasoning_effort ?? null,
    inbound_endpoint: payload.inbound_endpoint || '/v1/responses',
    upstream_endpoint: payload.upstream_endpoint || 'https://api.openai.com/v1/responses',
    group_id: payload.group_id ?? apiKey?.group_id ?? null,
    subscription_id: payload.subscription_id ?? null,
    input_tokens: inputTokens,
    output_tokens: outputTokens,
    cache_creation_tokens: cacheCreationTokens,
    cache_read_tokens: cacheReadTokens,
    cache_creation_5m_tokens: Number(payload.cache_creation_5m_tokens || 0),
    cache_creation_1h_tokens: Number(payload.cache_creation_1h_tokens || 0),
    reasoning_tokens: Number(payload.reasoning_tokens || 0),
    input_cost: Number(payload.input_cost || 0),
    output_cost: Number(payload.output_cost || 0),
    cache_creation_cost: Number(payload.cache_creation_cost || 0),
    cache_read_cost: Number(payload.cache_read_cost || 0),
    total_cost: totalCost,
    actual_cost: actualCost,
    rate_multiplier: Number(payload.rate_multiplier || 1),
    points_deducted: Number(payload.points_deducted || 0),
    balance_deducted: Number(payload.balance_deducted ?? actualCost),
    billing_wallet_type: payload.billing_wallet_type || 'balance',
    billing_type: Number(payload.billing_type || 0),
    request_type: payload.request_type || (payload.stream === false ? 'sync' : 'stream'),
    stream: payload.stream !== false,
    openai_ws_mode: Boolean(payload.openai_ws_mode),
    duration_ms: Number(payload.duration_ms || 4200),
    first_token_ms: payload.first_token_ms ?? 860,
    image_count: Number(payload.image_count || 0),
    image_size: payload.image_size ?? null,
    image_output_tokens: Number(payload.image_output_tokens || 0),
    image_output_cost: Number(payload.image_output_cost || 0),
    user_agent: payload.user_agent || 'codex-cli/0.1.0',
    cache_ttl_overridden: Boolean(payload.cache_ttl_overridden),
    billing_mode: payload.billing_mode || 'token',
    created_at: payload.created_at || nowISO(),
    api_key: apiKey,
    group: apiKey?.group || null,
    subscription: null
  })
}

function seedMockUsageLogs(): void {
  if (mockUsageLogs.length > 0) return
  seedMockApiKeys()
  const productionKey = mockApiKeys.find((key) => key.name === 'Production Gateway')
  const claudeKey = mockApiKeys.find((key) => key.name === 'Claude Workspace')
  const mobileKey = mockApiKeys.find((key) => key.name === 'Mobile Test Key')

  createMockUsageLog({
    api_key_id: productionKey?.id,
    model: 'gpt-5.5',
    reasoning_effort: 'high',
    reasoning_tokens: 18432,
    input_tokens: 48260,
    output_tokens: 9214,
    cache_read_tokens: 28640,
    total_cost: 0.3842,
    actual_cost: 0.2689,
    duration_ms: 12400,
    first_token_ms: 780,
    billing_wallet_type: 'subscription',
    billing_type: 1,
    balance_deducted: 0,
    user_agent: 'codex-cli/0.1.0',
    created_at: new Date(Date.now() - 7 * 60 * 1000).toISOString()
  })
  createMockUsageLog({
    api_key_id: claudeKey?.id,
    model: 'claude-sonnet-4.6',
    input_tokens: 32680,
    output_tokens: 6840,
    cache_creation_tokens: 4096,
    cache_creation_5m_tokens: 4096,
    cache_read_tokens: 16384,
    total_cost: 0.2921,
    actual_cost: 0.2045,
    duration_ms: 8900,
    first_token_ms: 620,
    inbound_endpoint: '/v1/messages',
    upstream_endpoint: 'https://api.anthropic.com/v1/messages',
    billing_wallet_type: 'balance',
    created_at: new Date(Date.now() - 54 * 60 * 1000).toISOString()
  })
  createMockUsageLog({
    api_key_id: productionKey?.id,
    model: 'gpt-5.4-mini',
    reasoning_effort: 'low',
    reasoning_tokens: 2048,
    input_tokens: 12400,
    output_tokens: 2310,
    cache_read_tokens: 8192,
    total_cost: 0.0648,
    actual_cost: 0.0454,
    duration_ms: 3100,
    first_token_ms: 410,
    openai_ws_mode: true,
    request_type: 'ws_v2',
    created_at: new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString()
  })
  createMockUsageLog({
    api_key_id: mobileKey?.id,
    model: 'gemini-3.1-pro',
    input_tokens: 22600,
    output_tokens: 5180,
    total_cost: 0.146,
    actual_cost: 0.1022,
    duration_ms: 7100,
    first_token_ms: 940,
    inbound_endpoint: '/v1beta/models/gemini-3.1-pro:streamGenerateContent',
    upstream_endpoint: 'https://generativelanguage.googleapis.com/v1beta/models/gemini-3.1-pro:streamGenerateContent',
    user_agent: 'GeminiCLI/1.4.2',
    created_at: new Date(Date.now() - 22 * 60 * 60 * 1000).toISOString()
  })
  createMockUsageLog({
    api_key_id: claudeKey?.id,
    model: 'claude-opus-4.7',
    input_tokens: 58600,
    output_tokens: 11200,
    cache_creation_tokens: 12288,
    cache_creation_1h_tokens: 12288,
    cache_read_tokens: 32768,
    total_cost: 0.824,
    actual_cost: 0.5768,
    duration_ms: 18100,
    first_token_ms: 1260,
    inbound_endpoint: '/v1/messages',
    billing_wallet_type: 'mixed',
    points_deducted: 0.2,
    balance_deducted: 0.3768,
    created_at: new Date(Date.now() - 28 * 60 * 60 * 1000).toISOString()
  })
}

function filteredMockUsageLogs(url: URL): Array<Record<string, unknown>> {
  seedMockUsageLogs()
  const apiKeyID = Number(url.searchParams.get('api_key_id') || 0)
  const filtered = apiKeyID > 0
    ? mockUsageLogs.filter((log) => Number(log.api_key_id) === apiKeyID)
    : mockUsageLogs
  const sortOrder = url.searchParams.get('sort_order') === 'asc' ? 1 : -1
  return [...filtered].sort((a, b) => (
    String(a.created_at).localeCompare(String(b.created_at)) * sortOrder
  ))
}

function mockUsageStats(url: URL): Record<string, unknown> {
  const logs = filteredMockUsageLogs(url)
  const sum = (key: string) => logs.reduce((total, log) => total + Number(log[key] || 0), 0)
  const models = logs.reduce<Record<string, number>>((result, log) => {
    const model = String(log.model || 'unknown')
    result[model] = (result[model] || 0) + 1
    return result
  }, {})
  return {
    total_requests: logs.length,
    total_input_tokens: sum('input_tokens'),
    total_output_tokens: sum('output_tokens'),
    total_cache_tokens: sum('cache_creation_tokens') + sum('cache_read_tokens'),
    total_tokens: sum('input_tokens') + sum('output_tokens') + sum('cache_creation_tokens') + sum('cache_read_tokens'),
    total_cost: sum('total_cost'),
    total_actual_cost: sum('actual_cost'),
    average_duration_ms: logs.length ? sum('duration_ms') / logs.length : 0,
    models
  }
}

function mockUsageInfo(): Record<string, unknown> {
  return {
    source: 'passive',
    updated_at: nowISO(),
    five_hour: {
      used: 0,
      limit: 100,
      percentage: 0,
      reset_at: new Date(Date.now() + 5 * 60 * 60 * 1000).toISOString()
    },
    seven_day: {
      used: 0,
      limit: 100,
      percentage: 0,
      reset_at: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString()
    },
    seven_day_sonnet: null
  }
}

function mockDashboardStats(): Record<string, unknown> {
  return {
    total_users: 128,
    today_new_users: 6,
    active_users: 42,
    hourly_active_users: 12,
    stats_updated_at: nowISO(),
    stats_stale: false,
    total_api_keys: 86,
    active_api_keys: 79,
    total_accounts: Math.max(mockAccounts.length, 12),
    normal_accounts: Math.max(mockAccounts.length, 10),
    error_accounts: 1,
    ratelimit_accounts: 1,
    overload_accounts: 0,
    total_requests: 128000,
    total_input_tokens: 184000000,
    total_output_tokens: 92000000,
    total_cache_creation_tokens: 12000000,
    total_cache_read_tokens: 38000000,
    total_tokens: 326000000,
    total_cost: 860,
    total_actual_cost: 512,
    total_account_cost: 384,
    today_requests: 1280,
    today_input_tokens: 1840000,
    today_output_tokens: 920000,
    today_cache_creation_tokens: 120000,
    today_cache_read_tokens: 380000,
    today_tokens: 3260000,
    today_cost: 8.6,
    today_actual_cost: 5.12,
    today_account_cost: 3.84,
    average_duration_ms: 5400,
    uptime: 86400,
    rpm: 18,
    tpm: 52000
  }
}

function mockTrend(granularity: string): Array<Record<string, unknown>> {
  const now = new Date()
  return Array.from({ length: granularity === 'hour' ? 12 : 7 }, (_, index) => {
    const date = new Date(now)
    if (granularity === 'hour') {
      date.setHours(now.getHours() - (11 - index), 0, 0, 0)
    } else {
      date.setDate(now.getDate() - (6 - index))
    }
    const requests = 120 + index * 18
    return {
      date: granularity === 'hour' ? date.toISOString() : date.toISOString().slice(0, 10),
      requests,
      input_tokens: requests * 900,
      output_tokens: requests * 460,
      cache_creation_tokens: requests * 80,
      cache_read_tokens: requests * 220,
      total_tokens: requests * 1660,
      cost: Number((requests * 0.006).toFixed(4)),
      actual_cost: Number((requests * 0.004).toFixed(4))
    }
  })
}

function mockModelStats(): Array<Record<string, unknown>> {
  return [
    {
      model: 'gpt-5.5',
      requests: 620,
      input_tokens: 920000,
      output_tokens: 410000,
      cache_creation_tokens: 72000,
      cache_read_tokens: 180000,
      total_tokens: 1582000,
      cost: 4.2,
      actual_cost: 2.8,
      account_cost: 2.1
    },
    {
      model: 'claude-sonnet-4.6',
      requests: 420,
      input_tokens: 680000,
      output_tokens: 330000,
      cache_creation_tokens: 48000,
      cache_read_tokens: 120000,
      total_tokens: 1178000,
      cost: 3.1,
      actual_cost: 1.9,
      account_cost: 1.4
    }
  ]
}

function mockUserTrend(granularity: string): Array<Record<string, unknown>> {
  return mockTrend(granularity).flatMap((point, index) => [
    {
      date: point.date,
      user_id: 1,
      email: 'admin@local.test',
      username: 'local-admin',
      requests: Number(point.requests),
      tokens: Number(point.total_tokens),
      cost: Number(point.cost),
      actual_cost: Number(point.actual_cost)
    },
    {
      date: point.date,
      user_id: 2,
      email: 'user@local.test',
      username: 'local-user',
      requests: Math.max(20, Number(point.requests) - 45 - index * 3),
      tokens: Math.max(20000, Number(point.total_tokens) - 46000 - index * 5000),
      cost: Math.max(0.1, Number(point.cost) - 0.18),
      actual_cost: Math.max(0.08, Number(point.actual_cost) - 0.12)
    }
  ])
}

function mockAccountSharingDashboard(url: URL): Record<string, unknown> {
  const page = Math.max(1, Number(url.searchParams.get('account_page') || 1))
  const pageSize = Math.max(1, Number(url.searchParams.get('account_page_size') || 20))
  return {
    summary: {
      owned_accounts: mockAccounts.length,
      private_accounts: mockAccounts.length,
      public_pending_accounts: 0,
      public_approved_accounts: 0,
      public_suspended_accounts: 0,
      self_requests: 0,
      self_tokens: 0,
      self_actual_cost: 0,
      self_account_cost: 0,
      external_requests: 0,
      external_consumer_charge: 0,
      external_account_cost: 0,
      external_owner_credit: 0,
      external_platform_fee: 0,
      total_account_cost: 0,
      balance_net_change: 0
    },
    accounts: [],
    accounts_pagination: {
      total: 0,
      page,
      page_size: pageSize,
      pages: 1
    },
    trend: [],
    start_date: url.searchParams.get('start_date') || '',
    end_date: url.searchParams.get('end_date') || '',
    granularity: url.searchParams.get('granularity') || 'day'
  }
}

function sendJson(res: ServerResponse, status: number, payload: unknown): void {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json; charset=utf-8')
  res.end(JSON.stringify(payload))
}

function success(data: unknown): Record<string, unknown> {
  return { code: 0, message: 'success', data }
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let body = ''
    req.on('data', (chunk) => {
      body += chunk
    })
    req.on('end', () => resolve(body))
    req.on('error', reject)
  })
}

function localMockApiPlugin(enabled: boolean): Plugin {
  return {
    name: 'ikik-local-mock-api',
    enforce: 'pre',
    apply: 'serve',
    configureServer(server) {
      if (!enabled) return
      console.info('[vite] Local mock API enabled')

      server.middlewares.use(async (req, res, next) => {
        const url = req.url ? new URL(req.url, 'http://local.test') : null
        const path = url?.pathname || ''


        if (path === '/setup/status') {
          sendJson(res, 200, success({ needs_setup: false, step: 'done' }))
          return
        }

        if (path === '/api/v1/settings/public') {
          sendJson(res, 200, success(localPublicSettings(req)))
          return
        }

        if (path === '/api/v1/auth/login' && req.method === 'POST') {
          const body = await readBody(req)
          const credentials = parseJsonBody<{ email?: string; password?: string }>(body)
          if (credentials.email !== 'admin@local.test' || credentials.password !== 'admin123456') {
            sendJson(res, 401, { code: 401, message: 'invalid email or password' })
            return
          }
          sendJson(res, 200, success({
            access_token: 'local-mock-access-token',
            refresh_token: 'local-mock-refresh-token',
            expires_in: 2592000,
            token_type: 'Bearer',
            user: mockUser
          }))
          return
        }

        if (path === '/api/v1/auth/register' && req.method === 'POST') {
          const body = await readBody(req)
          const payload = parseJsonBody<{ email?: string; username?: string }>(body)
          const user = {
            ...mockUser,
            id: 2,
            username: payload.username || 'local-user',
            email: payload.email || 'user@local.test',
            role: 'user'
          }
          sendJson(res, 200, success({
            access_token: 'local-mock-access-token',
            refresh_token: 'local-mock-refresh-token',
            expires_in: 2592000,
            token_type: 'Bearer',
            user
          }))
          return
        }

        if (path === '/api/v1/auth/logout' && req.method === 'POST') {
          sendJson(res, 200, success({ message: 'logged out' }))
          return
        }

        if (path === '/api/v1/auth/me') {
          sendJson(res, 200, success(mockUser))
          return
        }

        if (path === '/api/v1/auth/session') {
          sendJson(res, 401, { code: 401, message: 'no local mock session' })
          return
        }

        if (path === '/api/v1/auth/refresh' && req.method === 'POST') {
          sendJson(res, 200, success({
            access_token: 'local-mock-access-token',
            refresh_token: 'local-mock-refresh-token',
            expires_in: 2592000,
            token_type: 'Bearer'
          }))
          return
        }

        if (path === '/api/v1/public/usage/today') {
          sendJson(res, 200, success({
            today_requests: 1280,
            today_tokens: 2480000,
            success_count: 1270,
            error_count: 10,
            success_rate: 99.2,
            average_duration_ms: 5400,
            average_first_token_ms: 1200,
            timezone: 'Asia/Shanghai'
          }))
          return
        }

        if (path === '/api/v1/admin/system/check-updates') {
          sendJson(res, 200, success({
            current_version: '1.0.2-local',
            latest_version: '1.0.2-local',
            update_available: false
          }))
          return
        }

        if (path === '/api/v1/admin/settings') {
          sendJson(res, 200, success(localPublicSettings(req)))
          return
        }

        if (path === '/api/v1/admin/risk-control/config') {
          if (req.method === 'PUT') {
            const body = await readBody(req)
            const payload = parseJsonBody(body)
            mockRiskConfig = { ...mockRiskConfig, ...payload, api_key_configured: true, api_key_count: 1 }
          }
          sendJson(res, 200, success(mockRiskConfig))
          return
        }

        if (path === '/api/v1/admin/groups/all' && req.method === 'GET') {
          sendJson(res, 200, success(mockGroups))
          return
        }

        if (path === '/api/v1/admin/risk-control/status') {
          sendJson(res, 200, success({
            enabled: true,
            risk_control_enabled: true,
            mode: mockRiskConfig.mode,
            worker_count: 4,
            max_workers: 32,
            active_workers: 1,
            idle_workers: 3,
            queue_size: 32768,
            queue_length: 7,
            queue_usage_percent: 0.02,
            enqueued: 1842,
            dropped: 0,
            processed: 1835,
            errors: 2,
            pre_block_active: 0,
            pre_block_checked: 0,
            pre_block_allowed: 0,
            pre_block_blocked: 0,
            pre_block_errors: 0,
            pre_block_avg_latency_ms: 0,
            pre_block_api_key_active: 0,
            pre_block_api_key_available_count: 1,
            pre_block_api_key_total_calls: 0,
            pre_block_api_key_loads: [],
            api_key_statuses: [],
            flagged_hash_count: 3,
            last_cleanup_at: nowISO(),
            last_cleanup_deleted_hit: 0,
            last_cleanup_deleted_non_hit: 12
          }))
          return
        }

        if (path === '/api/v1/admin/risk-control/logs') {
          sendJson(res, 200, success(paginateItems([], url)))
          return
        }

        if (path === '/api/v1/admin/risk-control/risk-profiles') {
          const search = (url.searchParams.get('search') || '').toLowerCase()
          const level = url.searchParams.get('level') || ''
          const filtered = mockRiskProfiles.filter((profile) => {
            const matchesSearch = !search || String(profile.user_email).toLowerCase().includes(search) || String(profile.user_id).includes(search)
            const matchesLevel = !level || profile.risk_level === level
            return matchesSearch && matchesLevel
          })
          const page = paginateItems(filtered, url)
          sendJson(res, 200, success({
            ...page,
            overview: {
              total_profiles: mockRiskProfiles.length,
              new_profiles: mockRiskProfiles.filter((item) => item.risk_level === 'new').length,
              trusted_profiles: mockRiskProfiles.filter((item) => item.risk_level === 'trusted').length,
              watch_profiles: mockRiskProfiles.filter((item) => item.risk_level === 'watch').length,
              high_profiles: mockRiskProfiles.filter((item) => item.risk_level === 'high').length,
              critical_profiles: mockRiskProfiles.filter((item) => item.risk_level === 'critical').length,
              audited_requests: mockRiskProfiles.reduce((sum, item) => sum + Number(item.audited_requests || 0), 0),
              flagged_requests: mockRiskProfiles.reduce((sum, item) => sum + Number(item.flagged_requests || 0), 0),
              average_risk_score: mockRiskProfiles.reduce((sum, item) => sum + Number(item.risk_score || 0), 0) / mockRiskProfiles.length
            }
          }))
          return
        }

        const riskProfileMatch = path.match(/^\/api\/v1\/admin\/risk-control\/risk-profiles\/(\d+)$/)
        if (riskProfileMatch && req.method === 'PATCH') {
          const profile = mockRiskProfiles.find((item) => Number(item.user_id) === Number(riskProfileMatch[1]))
          if (!profile) {
            sendJson(res, 404, { code: 404, message: 'profile not found' })
            return
          }
          const body = await readBody(req)
          const payload = parseJsonBody(body)
          if (payload.manual_level) profile.manual_level = payload.manual_level
          if (payload.reset_score) profile.risk_score = 0
          profile.updated_at = nowISO()
          sendJson(res, 200, success(profile))
          return
        }

        if (path === '/api/v1/admin/risk-control/api-keys/test' && req.method === 'POST') {
          sendJson(res, 200, success({ items: [], image_count: 0 }))
          return
        }

        if (path === '/api/v1/admin/payment/config') {
          sendJson(res, 200, success({
            enabled: true,
            min_amount: 1,
            max_amount: 10000,
            daily_limit: 50000,
            order_timeout_minutes: 15,
            max_pending_orders: 5,
            enabled_payment_types: ['balance'],
            balance_disabled: false,
            balance_recharge_multiplier: 1,
            subscription_usd_to_cny_rate: 7.2,
            recharge_fee_rate: 0,
            load_balance_strategy: 'round_robin',
            product_name_prefix: 'ikik-api',
            product_name_suffix: '',
            help_image_url: '',
            help_text: ''
          }))
          return
        }

        if (path === '/api/v1/admin/dashboard/snapshot-v2') {
          const startDate = url.searchParams.get('start_date') || ''
          const endDate = url.searchParams.get('end_date') || ''
          const granularity = url.searchParams.get('granularity') || 'day'
          sendJson(res, 200, success({
            generated_at: nowISO(),
            start_date: startDate,
            end_date: endDate,
            granularity,
            stats: mockDashboardStats(),
            trend: mockTrend(granularity),
            models: mockModelStats(),
            groups: [],
            users_trend: []
          }))
          return
        }

        if (path === '/api/v1/admin/dashboard/users-trend') {
          const startDate = url.searchParams.get('start_date') || ''
          const endDate = url.searchParams.get('end_date') || ''
          const granularity = url.searchParams.get('granularity') || 'day'
          sendJson(res, 200, success({
            trend: mockUserTrend(granularity),
            start_date: startDate,
            end_date: endDate,
            granularity
          }))
          return
        }

        if (path === '/api/v1/admin/dashboard/users-ranking') {
          sendJson(res, 200, success({
            ranking: [
              {
                user_id: 1,
                email: 'admin@local.test',
                actual_cost: 3.42,
                requests: 860,
                tokens: 2140000
              },
              {
                user_id: 2,
                email: 'user@local.test',
                actual_cost: 1.7,
                requests: 420,
                tokens: 1120000
              }
            ],
            total_actual_cost: 5.12,
            total_requests: 1280,
            total_tokens: 3260000,
            start_date: url.searchParams.get('start_date') || '',
            end_date: url.searchParams.get('end_date') || ''
          }))
          return
        }

        if (path === '/api/v1/admin/dashboard/models') {
          sendJson(res, 200, success({
            models: mockModelStats(),
            start_date: url.searchParams.get('start_date') || '',
            end_date: url.searchParams.get('end_date') || ''
          }))
          return
        }

        if (path === '/api/v1/admin/dashboard/user-breakdown') {
          sendJson(res, 200, success({
            users: [
              {
                user_id: 1,
                email: 'admin@local.test',
                requests: 860,
                total_tokens: 2140000,
                cost: 4.8,
                actual_cost: 3.42,
                account_cost: 2.6
              },
              {
                user_id: 2,
                email: 'user@local.test',
                requests: 420,
                total_tokens: 1120000,
                cost: 2.1,
                actual_cost: 1.7,
                account_cost: 1.24
              }
            ],
            start_date: url.searchParams.get('start_date') || '',
            end_date: url.searchParams.get('end_date') || ''
          }))
          return
        }

        if (path === '/api/v1/usage/dashboard/stats') {
          const stats = mockDashboardStats()
          sendJson(res, 200, success({
            total_api_keys: stats.total_api_keys,
            active_api_keys: stats.active_api_keys,
            total_requests: stats.total_requests,
            total_input_tokens: stats.total_input_tokens,
            total_output_tokens: stats.total_output_tokens,
            total_cache_creation_tokens: stats.total_cache_creation_tokens,
            total_cache_read_tokens: stats.total_cache_read_tokens,
            total_tokens: stats.total_tokens,
            total_cost: stats.total_cost,
            total_actual_cost: stats.total_actual_cost,
            today_requests: stats.today_requests,
            today_input_tokens: stats.today_input_tokens,
            today_output_tokens: stats.today_output_tokens,
            today_cache_creation_tokens: stats.today_cache_creation_tokens,
            today_cache_read_tokens: stats.today_cache_read_tokens,
            today_tokens: stats.today_tokens,
            today_cost: stats.today_cost,
            today_actual_cost: stats.today_actual_cost,
            today_platforms: [
              { platform: 'openai', requests: 860, input_tokens: 1200000, output_tokens: 620000, cache_creation_tokens: 80000, cache_read_tokens: 260000, total_tokens: 2160000, cost: 5.8, actual_cost: 3.42 },
              { platform: 'anthropic', requests: 420, input_tokens: 640000, output_tokens: 300000, cache_creation_tokens: 40000, cache_read_tokens: 120000, total_tokens: 1100000, cost: 2.8, actual_cost: 1.7 }
            ],
            average_duration_ms: stats.average_duration_ms,
            rpm: stats.rpm,
            tpm: stats.tpm
          }))
          return
        }

        if (path === '/api/v1/usage/dashboard/trend') {
          const startDate = url.searchParams.get('start_date') || ''
          const endDate = url.searchParams.get('end_date') || ''
          const granularity = url.searchParams.get('granularity') || 'day'
          sendJson(res, 200, success({
            trend: mockTrend(granularity),
            start_date: startDate,
            end_date: endDate,
            granularity
          }))
          return
        }

        if (path === '/api/v1/usage/dashboard/models') {
          sendJson(res, 200, success({
            models: mockModelStats(),
            start_date: url.searchParams.get('start_date') || '',
            end_date: url.searchParams.get('end_date') || ''
          }))
          return
        }

        if (path === '/api/v1/usage/dashboard/account-sharing') {
          sendJson(res, 200, success(mockAccountSharingDashboard(url)))
          return
        }

        if (path === '/api/v1/usage/dashboard/api-keys-usage' && req.method === 'POST') {
          const body = await readBody(req)
          const payload = parseJsonBody<{ api_key_ids?: unknown[] }>(body)
          const ids = Array.isArray(payload.api_key_ids) ? payload.api_key_ids.map(Number) : []
          const stats = Object.fromEntries(ids.map((id) => [
            String(id),
            {
              api_key_id: id,
              today_actual_cost: Number((id % 7 * 0.127 + 0.42).toFixed(4)),
              total_actual_cost: Number((id % 11 * 1.37 + 8.6).toFixed(4))
            }
          ]))
          sendJson(res, 200, success({ stats }))
          return
        }

        if (path === '/api/v1/usage/stats') {
          sendJson(res, 200, success(mockUsageStats(url)))
          return
        }

        if (path === '/api/v1/usage') {
          sendJson(res, 200, success(paginateItems(filteredMockUsageLogs(url), url)))
          return
        }

        if (path === '/api/v1/payment/orders/my') {
          const orders = [
            {
              id: 2026071001,
              user_id: 1,
              amount: 100,
              pay_amount: 100,
              currency: 'CNY',
              fee_rate: 0,
              payment_type: 'alipay',
              out_trade_no: 'IK202607100001',
              status: 'COMPLETED',
              order_type: 'balance',
              created_at: '2026-07-10T04:20:00Z',
              expires_at: '2026-07-10T04:50:00Z',
              paid_at: '2026-07-10T04:21:00Z',
              completed_at: '2026-07-10T04:21:00Z',
              refund_amount: 0,
              provider_instance_id: 'alipay-local'
            },
            {
              id: 2026070902,
              user_id: 1,
              amount: 199,
              pay_amount: 199,
              currency: 'CNY',
              fee_rate: 0,
              payment_type: 'alipay',
              out_trade_no: 'IK202607090002',
              status: 'COMPLETED',
              order_type: 'subscription',
              created_at: '2026-07-09T08:12:00Z',
              expires_at: '2026-07-09T08:42:00Z',
              paid_at: '2026-07-09T08:13:00Z',
              completed_at: '2026-07-09T08:13:00Z',
              refund_amount: 0,
              plan_id: 2,
              provider_instance_id: 'alipay-local'
            },
            {
              id: 2026070803,
              user_id: 1,
              amount: 50,
              pay_amount: 50,
              currency: 'CNY',
              fee_rate: 0,
              payment_type: 'alipay',
              out_trade_no: 'IK202607080003',
              status: 'PENDING',
              order_type: 'balance',
              created_at: '2026-07-08T13:45:00Z',
              expires_at: '2026-07-08T14:15:00Z',
              refund_amount: 0,
              provider_instance_id: 'alipay-local'
            }
          ]
          const status = url.searchParams.get('status')
          const filtered = status ? orders.filter((order) => order.status === status) : orders
          sendJson(res, 200, success(paginateItems(filtered, url)))
          return
        }

        if (path === '/api/v1/payment/orders/refund-eligible-providers') {
          sendJson(res, 200, success({ provider_instance_ids: [] }))
          return
        }

        if (path === '/api/v1/shop/categories') {
          sendJson(res, 200, success([
            { id: 1, name: '订阅与额度', description: '', enabled: true, sort_order: 1, product_count: 2 },
            { id: 2, name: '活动', description: '', enabled: true, sort_order: 2, product_count: 1 }
          ]))
          return
        }

        if (path === '/api/v1/shop/products') {
          const categories = {
            1: { id: 1, name: '订阅与额度', description: '', enabled: true, sort_order: 1 },
            2: { id: 2, name: '活动', description: '', enabled: true, sort_order: 2 }
          }
          const products = [
            {
              id: 11,
              category_id: 1,
              category: categories[1],
              name: 'OpenAI Pro 30 天',
              cover_url: null,
              description: '适用于持续开发与高频模型调用',
              price: 99,
              original_price: 129,
              stock: 36,
              enabled: true,
              sort_order: 1,
              min_purchase: 1,
              max_purchase: 3,
              auto_delivery: true,
              product_type: 'card_key',
              balance_only: false,
              allow_balance_payment: true,
              allow_points_payment: false,
              allow_platform_payment: true,
              draw_config: null,
              draw_progress: null,
              stock_unlimited: false
            },
            {
              id: 12,
              category_id: 1,
              category: categories[1],
              name: 'Claude Max 30 天',
              cover_url: null,
              description: '适用于 Sonnet、Opus 和长上下文任务',
              price: 199,
              original_price: 239,
              stock: 18,
              enabled: true,
              sort_order: 2,
              min_purchase: 1,
              max_purchase: 2,
              auto_delivery: true,
              product_type: 'card_key',
              balance_only: false,
              allow_balance_payment: true,
              allow_points_payment: true,
              allow_platform_payment: true,
              draw_config: null,
              draw_progress: null,
              stock_unlimited: false
            },
            {
              id: 13,
              category_id: 2,
              category: categories[2],
              name: '余额随机包',
              cover_url: null,
              description: '购买后立即获得随机余额',
              price: 10,
              original_price: null,
              stock: 0,
              enabled: true,
              sort_order: 3,
              min_purchase: 1,
              max_purchase: 1,
              auto_delivery: true,
              product_type: 'balance_draw',
              balance_only: true,
              allow_balance_payment: true,
              allow_points_payment: false,
              allow_platform_payment: false,
              draw_config: { enabled: true, min_amount: 2, max_amount: 20, guarantee_count: 10, return_rate: 1 },
              draw_progress: { drawn_count: 4, guarantee_count: 10 },
              stock_unlimited: true
            }
          ]
          sendJson(res, 200, success(paginateItems(products, url)))
          return
        }

        if (path === '/api/v1/shop/draw-progress') {
          sendJson(res, 200, success({ 13: { drawn_count: 4, guarantee_count: 10 } }))
          return
        }

        if (path === '/api/v1/payment/checkout-info') {
          sendJson(res, 200, success({
            methods: {
              alipay: {
                currency: 'CNY',
                daily_limit: 5000,
                daily_used: 0,
                daily_remaining: 5000,
                single_min: 5,
                single_max: 1000,
                fee_rate: 0,
                available: true
              }
            },
            global_min: 5,
            global_max: 1000,
            min_amount: 5,
            max_amount: 1000,
            plans: [{
              id: 1,
              group_id: 1,
              group_platform: 'openai',
              group_name: 'OpenAI Pro',
              rate_multiplier: 1,
              daily_limit_usd: null,
              weekly_limit_usd: 50,
              monthly_limit_usd: null,
              supported_model_scopes: ['gpt-5.5', 'gpt-5.4-mini'],
              name: 'Pro',
              description: '适合持续开发与日常高频调用，包含 GPT-5.5 与 Codex 模型。',
              price: 99,
              original_price: 129,
              validity_days: 30,
              validity_unit: 'day',
              features: ['GPT-5.5', 'Codex', '50 USD weekly'],
              for_sale: true,
              sort_order: 1
            }, {
              id: 2,
              group_id: 2,
              group_platform: 'anthropic',
              group_name: 'Claude Max',
              rate_multiplier: 1,
              daily_limit_usd: null,
              weekly_limit_usd: 110,
              monthly_limit_usd: null,
              supported_model_scopes: ['claude-sonnet-4.6', 'claude-opus-4.7'],
              name: 'Max',
              description: '面向重度推理与长上下文任务，支持 Sonnet、Opus 与 1M 上下文模型。',
              price: 199,
              original_price: 239,
              validity_days: 30,
              validity_unit: 'day',
              features: ['Sonnet 4.6', 'Opus 4.7', '110 USD weekly'],
              for_sale: true,
              sort_order: 2
            }, {
              id: 3,
              group_id: 3,
              group_platform: 'custom',
              group_name: 'All Models',
              rate_multiplier: 1,
              daily_limit_usd: null,
              weekly_limit_usd: 220,
              monthly_limit_usd: null,
              supported_model_scopes: ['gpt-5.5', 'claude-opus-4.7', 'gemini-3.1-pro'],
              name: 'Ultra',
              description: '全模型开放，适合多模型工作流与高强度使用。',
              price: 329,
              validity_days: 30,
              validity_unit: 'day',
              features: ['All models', '220 USD weekly', 'Priority routing'],
              for_sale: true,
              sort_order: 3
            }],
            balance_disabled: false,
            balance_recharge_multiplier: 1,
            balance_pricing_tiers: [],
            recharge_fee_rate: 0,
            help_text: '',
            help_image_url: '',
            stripe_publishable_key: ''
          }))
          return
        }

        if (path === '/api/v1/subscriptions/active') {
          sendJson(res, 200, success([]))
          return
        }

        if (path === '/api/v1/subscriptions') {
          sendJson(res, 200, success([
            {
              id: 101,
              user_id: 1,
              group_id: 1,
              status: 'active',
              daily_usage_usd: 0,
              weekly_usage_usd: 18.42,
              monthly_usage_usd: 0,
              daily_window_start: null,
              weekly_window_start: '2026-07-07T00:00:00Z',
              monthly_window_start: null,
              created_at: '2026-07-01T00:00:00Z',
              updated_at: nowISO(),
              expires_at: '2026-08-01T00:00:00Z',
              group: {
                ...mockGroups[0],
                name: 'OpenAI Pro',
                description: 'GPT-5.5 and Codex',
                weekly_limit_usd: 50
              }
            },
            {
              id: 102,
              user_id: 1,
              group_id: 2,
              status: 'active',
              daily_usage_usd: 12.8,
              weekly_usage_usd: 76.35,
              monthly_usage_usd: 0,
              daily_window_start: '2026-07-10T00:00:00Z',
              weekly_window_start: '2026-07-07T00:00:00Z',
              monthly_window_start: null,
              created_at: '2026-07-03T00:00:00Z',
              updated_at: nowISO(),
              expires_at: '2026-08-03T00:00:00Z',
              group: {
                ...mockGroups[1],
                name: 'Claude Max',
                description: 'Sonnet and Opus',
                daily_limit_usd: 25,
                weekly_limit_usd: 110
              }
            }
          ]))
          return
        }

        if (path === '/api/v1/user/aff') {
          sendJson(res, 200, success({
            user_id: 1,
            aff_code: 'IKIK2026',
            inviter_id: null,
            inviter_bound_at: null,
            invite_reward_expires_at: null,
            aff_count: 3,
            aff_quota: 18.72,
            aff_frozen_quota: 0,
            aff_history_quota: 68.45,
            period_start_at: url.searchParams.get('period_start_at'),
            period_end_at: url.searchParams.get('period_end_at'),
            period_rebate: 6.84,
            effective_rebate_rate_percent: 12.5,
            invitees: [
              {
                user_id: 2,
                email: 'lin***@example.com',
                username: 'Lin',
                created_at: '2026-07-08T09:20:00Z',
                invite_bind_source: 'registration',
                status: 'active',
                period_consumption: 24.6,
                period_rebate: 3.08,
                history_consumption: 186.2,
                total_rebate: 23.28
              },
              {
                user_id: 3,
                email: 'nt***@example.com',
                username: 'NTTD',
                created_at: '2026-07-06T12:10:00Z',
                invite_bind_source: 'registration',
                status: 'active',
                period_consumption: 18.4,
                period_rebate: 2.3,
                history_consumption: 142.8,
                total_rebate: 17.85
              },
              {
                user_id: 4,
                email: 'dev***@example.com',
                username: 'Developer',
                created_at: '2026-07-02T03:45:00Z',
                invite_bind_source: 'admin',
                status: 'active',
                period_consumption: 11.7,
                period_rebate: 1.46,
                history_consumption: 98.6,
                total_rebate: 12.32
              }
            ]
          }))
          return
        }

        if (path === '/api/v1/announcements') {
          sendJson(res, 200, success([]))
          return
        }

        if (path === '/api/v1/groups/available') {
          sendJson(res, 200, success(mockGroups))
          return
        }

        if (path === '/api/v1/groups/rates') {
          sendJson(res, 200, success({}))
          return
        }

        if (path === '/api/v1/keys' && req.method === 'GET') {
          seedMockApiKeys()
          sendJson(res, 200, success(paginateItems(mockApiKeys, url)))
          return
        }

        if (path === '/api/v1/keys' && req.method === 'POST') {
          const body = await readBody(req)
          sendJson(res, 200, success(createMockApiKey(parseJsonBody(body))))
          return
        }

        if (path.startsWith('/api/v1/keys/')) {
          seedMockApiKeys()
          const id = Number(path.split('/').pop() || 0)
          const key = mockApiKeys.find((item) => Number(item.id) === id)
          if (!key) {
            sendJson(res, 404, { code: 404, message: 'key not found' })
            return
          }

          if (req.method === 'GET') {
            sendJson(res, 200, success(key))
            return
          }

          if (req.method === 'PUT') {
            const body = await readBody(req)
            const payload = parseJsonBody(body)
            const group = Object.prototype.hasOwnProperty.call(payload, 'group_id')
              ? resolveMockGroup(payload.group_id)
              : resolveMockGroup(key.group_id)
            Object.assign(key, payload, {
              group_id: group ? group.id : null,
              group,
              updated_at: nowISO()
            })
            sendJson(res, 200, success(key))
            return
          }

          if (req.method === 'DELETE') {
            mockApiKeys.splice(mockApiKeys.indexOf(key), 1)
            sendJson(res, 200, success({ message: 'deleted' }))
            return
          }
        }

        if (path === '/api/v1/channels/available') {
          sendJson(res, 200, success([
            {
              name: 'Local Mock Gateway',
              description: 'Local data for playground UI preview',
              platforms: [
                {
                  platform: 'openai',
                  groups: [mockGroups[0]],
                  supported_models: ['gpt-5.5', 'gpt-5.4-mini', 'ik-auto-pro'].map((name) => ({
                    name,
                    platform: 'openai',
                    pricing: {
                      billing_mode: 'token',
                      input_price: 0.000002,
                      output_price: 0.000008,
                      cache_write_price: 0.0000005,
                      cache_read_price: 0.0000001,
                      image_output_price: null,
                      per_request_price: null,
                      intervals: []
                    }
                  }))
                },
                {
                  platform: 'anthropic',
                  groups: [mockGroups[1]],
                  supported_models: ['claude-sonnet-4.6', 'claude-opus-4.7'].map((name) => ({
                    name,
                    platform: 'anthropic',
                    pricing: {
                      billing_mode: 'token',
                      input_price: 0.000003,
                      output_price: 0.000015,
                      cache_write_price: 0.00000375,
                      cache_read_price: 0.0000003,
                      image_output_price: null,
                      per_request_price: null,
                      intervals: []
                    }
                  }))
                }
              ]
            }
          ]))
          return
        }

        if (path === '/api/v1/accounts/quota-dashboard') {
          const quotaDimension = {
            enabled_account_count: 0,
            exhausted_account_count: 0,
            limit: 0,
            used: 0,
            remaining: 0,
            utilization: 0
          }
          const makeQuotaDashboard = (
            accountCount: number,
            schedulableCount: number,
            limitedCount: number,
            groupName: string,
            platform: string,
            fiveHourUsage: number,
            weeklyUsage: number
          ) => {
            const quotaProtectedCount = Math.min(1, Math.max(0, accountCount - schedulableCount - limitedCount))
            const errorCount = Math.max(0, accountCount - schedulableCount - limitedCount - quotaProtectedCount)
            const concurrencyCapacity = accountCount * 10
            const schedulableConcurrencyCapacity = schedulableCount * 10

            return {
              generated_at: nowISO(),
              summaries: [],
              totals: {
              platform: 'all',
              type: 'all',
              account_count: accountCount,
              active_account_count: accountCount,
              schedulable_account_count: schedulableCount,
              rate_limited_account_count: limitedCount,
              quota_protected_account_count: quotaProtectedCount,
              error_account_count: errorCount,
              disabled_account_count: 0,
              concurrency_capacity: concurrencyCapacity,
              schedulable_concurrency_capacity: schedulableConcurrencyCapacity,
              quota_account_count: accountCount,
              unlimited_account_count: 0,
              daily: quotaDimension,
              weekly: quotaDimension,
              total: quotaDimension,
              usage_windows: []
            },
            group_summaries: [{
              group_id: platform === 'openai' ? 1 : 2,
              group_name: groupName,
              group_status: 'active',
              platform,
              account_count: accountCount,
              active_account_count: accountCount,
              schedulable_account_count: schedulableCount,
              rate_limited_account_count: limitedCount,
              quota_protected_account_count: quotaProtectedCount,
              error_account_count: errorCount,
              disabled_account_count: 0,
              concurrency_capacity: concurrencyCapacity,
              schedulable_concurrency_capacity: schedulableConcurrencyCapacity,
              quota_account_count: accountCount,
              unlimited_account_count: 0,
              daily: quotaDimension,
              weekly: quotaDimension,
              total: quotaDimension,
              usage_windows: [{
                window: '5h',
                account_count: accountCount,
                known_account_count: accountCount,
                average_utilization: fiveHourUsage,
                remaining_capacity_percent: Math.max(0, 100 - fiveHourUsage)
              }, {
                window: '7d',
                account_count: accountCount,
                known_account_count: accountCount,
                average_utilization: weeklyUsage,
                remaining_capacity_percent: Math.max(0, 100 - weeklyUsage)
              }]
            }]
            }
          }
          sendJson(res, 200, success({
            generated_at: nowISO(),
            mine: makeQuotaDashboard(3, 2, 1, 'OpenAI Local', 'openai', 42.5, 61.8),
            platform: makeQuotaDashboard(8, 5, 1, 'Claude Local', 'anthropic', 35.2, 48.4)
          }))
          return
        }

        if (path === '/api/v1/channel-monitors') {
          const makeTimeline = (status: string, latency: number) => Array.from({ length: 18 }, (_, index) => ({
            status: index === 5 && status === 'operational' ? 'degraded' : status,
            latency_ms: latency + ((index % 4) * 35),
            ping_latency_ms: 95 + ((index % 3) * 12),
            checked_at: new Date(Date.now() - index * 60_000).toISOString()
          }))
          sendJson(res, 200, success({
            items: [{
              id: 1,
              name: 'OpenAI Primary',
              provider: 'openai',
              group_name: 'OpenAI Local',
              primary_model: 'gpt-5.5',
              primary_status: 'operational',
              primary_latency_ms: 820,
              primary_ping_latency_ms: 108,
              availability_7d: 99.96,
              extra_models: [{ model: 'gpt-5.4-mini', status: 'operational', latency_ms: 610 }],
              timeline: makeTimeline('operational', 820)
            }, {
              id: 2,
              name: 'Claude Primary',
              provider: 'anthropic',
              group_name: 'Claude Local',
              primary_model: 'claude-sonnet-4.6',
              primary_status: 'operational',
              primary_latency_ms: 940,
              primary_ping_latency_ms: 126,
              availability_7d: 99.82,
              extra_models: [{ model: 'claude-opus-4.7', status: 'operational', latency_ms: 1180 }],
              timeline: makeTimeline('operational', 940)
            }, {
              id: 3,
              name: 'Gemini Backup',
              provider: 'gemini',
              group_name: 'Gemini',
              primary_model: 'gemini-3.1-pro',
              primary_status: 'degraded',
              primary_latency_ms: 1640,
              primary_ping_latency_ms: 182,
              availability_7d: 97.42,
              extra_models: [],
              timeline: makeTimeline('degraded', 1640)
            }]
          }))
          return
        }

        const monitorStatusMatch = path.match(/^\/api\/v1\/channel-monitors\/(\d+)\/status$/)
        if (monitorStatusMatch) {
          const monitorID = Number(monitorStatusMatch[1])
          const model = monitorID === 1
            ? 'gpt-5.5'
            : monitorID === 2
              ? 'claude-sonnet-4.6'
              : 'gemini-3.1-pro'
          sendJson(res, 200, success({
            id: monitorID,
            name: model,
            provider: monitorID === 1 ? 'openai' : monitorID === 2 ? 'anthropic' : 'gemini',
            group_name: monitorID === 1 ? 'OpenAI Local' : monitorID === 2 ? 'Claude Local' : 'Gemini',
            models: [{
              model,
              latest_status: monitorID === 3 ? 'degraded' : 'operational',
              latest_latency_ms: monitorID === 3 ? 1640 : 880,
              availability_7d: monitorID === 3 ? 97.42 : 99.9,
              availability_15d: monitorID === 3 ? 97.8 : 99.85,
              availability_30d: monitorID === 3 ? 98.1 : 99.8,
              avg_latency_7d_ms: monitorID === 3 ? 1510 : 860
            }]
          }))
          return
        }

        if (path === '/api/v1/accounts/data') {
          sendJson(res, 200, success({
            type: 'ikik-api-data',
            version: 1,
            accounts: mockAccounts,
            proxies: mockProxies
          }))
          return
        }

        if (path === '/api/v1/accounts/import-credentials' && req.method === 'POST') {
          const body = await readBody(req)
          const payload = parseJsonBody<{ contents?: unknown[]; share_mode?: string }>(body)
          const contents = Array.isArray(payload.contents) ? payload.contents : []
          contents.forEach((content, index) => {
            createMockAccount({
              name: `Imported Local ${index + 1}`,
              platform: 'openai',
              type: 'oauth',
              share_mode: payload.share_mode || 'private',
              credentials: { imported: true, content }
            })
          })
          sendJson(res, 200, success({
            total: contents.length,
            created: contents.length,
            failed: 0,
            errors: []
          }))
          return
        }

        if (path === '/api/v1/accounts/import' && req.method === 'POST') {
          const body = await readBody(req)
          sendJson(res, 200, success(createMockAccount(parseJsonBody(body))))
          return
        }

        if (path === '/api/v1/accounts/today-stats/batch' && req.method === 'POST') {
          const body = await readBody(req)
          const payload = parseJsonBody<{ account_ids?: unknown[] }>(body)
          const stats = Object.fromEntries(
            (Array.isArray(payload.account_ids) ? payload.account_ids : []).map((id) => [
              String(id),
              { requests: 0, input_tokens: 0, output_tokens: 0, total_tokens: 0, cost: 0 }
            ])
          )
          sendJson(res, 200, success({ stats }))
          return
        }

        if (path === '/api/v1/accounts/bulk-update' && req.method === 'POST') {
          const body = await readBody(req)
          const payload = parseJsonBody<{ account_ids?: unknown[] }>(body)
          const ids = Array.isArray(payload.account_ids) ? payload.account_ids.map(Number) : []
          const successIDs: number[] = []
          mockAccounts.forEach((account) => {
            if (ids.includes(Number(account.id))) {
              Object.assign(account, payload, { updated_at: nowISO() })
              delete account.account_ids
              successIDs.push(Number(account.id))
            }
          })
          sendJson(res, 200, success({
            success: successIDs.length,
            failed: 0,
            success_ids: successIDs,
            failed_ids: [],
            results: successIDs.map((id) => ({ account_id: id, success: true }))
          }))
          return
        }

        if (path === '/api/v1/accounts/bulk-delete' && req.method === 'POST') {
          const body = await readBody(req)
          const payload = parseJsonBody<{ account_ids?: unknown[] }>(body)
          const ids = Array.isArray(payload.account_ids) ? payload.account_ids.map(Number) : []
          const before = mockAccounts.length
          for (let index = mockAccounts.length - 1; index >= 0; index -= 1) {
            if (ids.includes(Number(mockAccounts[index].id))) {
              mockAccounts.splice(index, 1)
            }
          }
          const deleted = before - mockAccounts.length
          sendJson(res, 200, success({
            success: deleted,
            failed: 0,
            success_ids: ids,
            failed_ids: [],
            results: ids.map((id) => ({ account_id: id, success: true }))
          }))
          return
        }

        if (path === '/api/v1/accounts/batch-refresh/async' && req.method === 'POST') {
          sendJson(res, 200, success({
            id: ++mockBatchTaskID,
            scope: 'user',
            operation: 'refresh',
            status: 'succeeded',
            total: 0,
            processed: 0,
            success: 0,
            failed: 0,
            created_by: mockUser.id,
            items: []
          }))
          return
        }

        if (path === '/api/v1/accounts/batch-revalidate-public-share/async' && req.method === 'POST') {
          sendJson(res, 200, success({
            id: ++mockBatchTaskID,
            scope: 'user',
            operation: 'revalidate-public-share',
            status: 'succeeded',
            total: 0,
            processed: 0,
            success: 0,
            failed: 0,
            created_by: mockUser.id,
            items: []
          }))
          return
        }

        if (path.startsWith('/api/v1/accounts/batch-tasks/')) {
          sendJson(res, 200, success({
            id: Number(path.split('/').pop() || 1),
            scope: 'user',
            operation: 'mock',
            status: 'succeeded',
            total: 0,
            processed: 0,
            success: 0,
            failed: 0,
            created_by: mockUser.id,
            items: []
          }))
          return
        }

        if (path === '/api/v1/accounts' && req.method === 'GET') {
          sendJson(res, 200, success(paginateItems(mockAccounts, url)))
          return
        }

        if (path === '/api/v1/accounts' && req.method === 'POST') {
          const body = await readBody(req)
          sendJson(res, 200, success(createMockAccount(parseJsonBody(body))))
          return
        }

        const accountMatch = path.match(/^\/api\/v1\/accounts\/(\d+)(?:\/([^/]+))?$/)
        if (accountMatch) {
          const id = Number(accountMatch[1])
          const action = accountMatch[2] || ''
          const account = mockAccounts.find((item) => Number(item.id) === id)
          if (!account) {
            sendJson(res, 404, { code: 404, message: 'account not found' })
            return
          }

          if (!action && req.method === 'GET') {
            sendJson(res, 200, success(account))
            return
          }

          if (!action && req.method === 'PUT') {
            const body = await readBody(req)
            Object.assign(account, parseJsonBody(body), { updated_at: nowISO() })
            sendJson(res, 200, success(account))
            return
          }

          if (!action && req.method === 'DELETE') {
            mockAccounts.splice(mockAccounts.indexOf(account), 1)
            sendJson(res, 200, success({ message: 'deleted' }))
            return
          }

          if (action === 'usage') {
            sendJson(res, 200, success(mockUsageInfo()))
            return
          }

          if (action === 'stats') {
            sendJson(res, 200, success({
              history: [],
              summary: {
                total_requests: 0,
                total_tokens: 0,
                total_cost: 0,
                avg_daily_requests: 0,
                avg_daily_tokens: 0,
                avg_daily_cost: 0
              }
            }))
            return
          }

          if (action === 'today-stats') {
            sendJson(res, 200, success({ requests: 0, input_tokens: 0, output_tokens: 0, total_tokens: 0, cost: 0 }))
            return
          }

          if (action === 'test' && req.method === 'POST') {
            sendJson(res, 200, success({
              status: 'success',
              message: 'Local mock test passed',
              response: 'pong',
              latency: 42
            }))
            return
          }

          if (action === 'refresh' && req.method === 'POST') {
            sendJson(res, 200, success({ account, message: 'Local mock refreshed' }))
            return
          }

          if (action === 'set-privacy' && req.method === 'POST') {
            account.share_mode = 'private'
            account.share_status = 'none'
            account.updated_at = nowISO()
            sendJson(res, 200, success(account))
            return
          }

          if (action === 'revalidate-public-share' && req.method === 'POST') {
            account.share_status = 'approved'
            account.updated_at = nowISO()
            sendJson(res, 200, success(account))
            return
          }
        }

        if (path === '/api/v1/account-proxies' && req.method === 'GET') {
          sendJson(res, 200, success(mockProxies))
          return
        }

        if (path === '/api/v1/account-proxies' && req.method === 'POST') {
          const body = await readBody(req)
          const payload = parseJsonBody(body)
          const proxy = {
            id: ++mockProxyID,
            ...payload,
            created_at: nowISO(),
            updated_at: nowISO()
          }
          mockProxies.unshift(proxy)
          sendJson(res, 200, success(proxy))
          return
        }

        const proxyMatch = path.match(/^\/api\/v1\/account-proxies\/(\d+)(?:\/([^/]+))?$/)
        if (proxyMatch) {
          const id = Number(proxyMatch[1])
          const action = proxyMatch[2] || ''
          const proxy = mockProxies.find((item) => Number(item.id) === id)
          if (!proxy) {
            sendJson(res, 404, { code: 404, message: 'proxy not found' })
            return
          }

          if (!action && req.method === 'PUT') {
            const body = await readBody(req)
            Object.assign(proxy, parseJsonBody(body), { updated_at: nowISO() })
            sendJson(res, 200, success(proxy))
            return
          }

          if (!action && req.method === 'DELETE') {
            mockProxies.splice(mockProxies.indexOf(proxy), 1)
            sendJson(res, 200, success({ message: 'deleted' }))
            return
          }

          if (action === 'test' && req.method === 'POST') {
            sendJson(res, 200, success({
              success: true,
              message: 'Local mock proxy test passed',
              latency_ms: 42,
              ip_address: '127.0.0.1',
              country: 'Local',
              country_code: 'LO'
            }))
            return
          }

          if (action === 'quality-check' && req.method === 'POST') {
            sendJson(res, 200, success({
              overall_score: 100,
              success: true,
              message: 'Local mock quality check passed'
            }))
            return
          }
        }

        if (path === '/api/v1/playground/chat/completions' && req.method === 'POST') {
          res.statusCode = 200
          res.setHeader('Content-Type', 'text/event-stream; charset=utf-8')
          res.setHeader('Cache-Control', 'no-cache')
          res.write(`data: ${JSON.stringify({ choices: [{ delta: { reasoning_content: '本地 mock 正在组织回答。' } }] })}\n\n`)
          res.write(`data: ${JSON.stringify({ choices: [{ delta: { content: '这是本地 mock 的流式回复。\\n\\n- 支持 **Markdown**\\n- 支持代码块\\n- 支持图片 Markdown 预览\\n\\n```ts\\nconsole.log(\"ikik playground\")\\n```' } }] })}\n\n`)
          res.write(`data: ${JSON.stringify({ usage: { prompt_tokens: 32, completion_tokens: 58, total_tokens: 90, reasoning_tokens: 12 }, choices: [{ delta: {} }] })}\n\n`)
          res.write('data: [DONE]\n\n')
          res.end()
          return
        }

        next()
      })
    }
  }
}

/**
 * Vite 插件：开发模式下注入公开配置到 index.html
 * 与生产模式的后端注入行为保持一致，消除闪烁
 */
function injectPublicSettings(backendUrl: string, localSettings?: unknown): Plugin {
  return {
    name: 'inject-public-settings',
    apply: 'serve',
    transformIndexHtml: {
      order: 'pre',
      async handler(html) {
        if (localSettings) {
          const settingsJson = JSON.stringify(localSettings).replace(/</g, '\\u003c')
          const script = `<script>window.__APP_CONFIG__=${settingsJson};window.__APP_CONFIG__.api_base_url=window.location.origin;</script>`
          return html.replace('</head>', `${script}\n</head>`)
        }

        try {
          const response = await fetch(`${backendUrl}/api/v1/settings/public`, {
            signal: AbortSignal.timeout(2000)
          })
          if (response.ok) {
            const data = await response.json()
            if (data.code === 0 && data.data) {
              const script = `<script>window.__APP_CONFIG__=${JSON.stringify(data.data)};</script>`
              return html.replace('</head>', `${script}\n</head>`)
            }
          }
        } catch (e) {
          console.warn('[vite] 无法获取公开配置，将回退到 API 调用:', (e as Error).message)
        }
        return html
      }
    }
  }
}

export default defineConfig(({ mode }) => {
  // 加载环境变量
  const env = loadEnv(mode, process.cwd(), '')
  const backendUrl = process.env.VITE_DEV_PROXY_TARGET || env.VITE_DEV_PROXY_TARGET || 'http://localhost:8080'
  const devPort = Number(process.env.VITE_DEV_PORT || env.VITE_DEV_PORT || 3000)
  const useLocalMocks = process.env.VITE_USE_LOCAL_MOCKS === 'true' || env.VITE_USE_LOCAL_MOCKS === 'true'

  return {
    plugins: [
      localMockApiPlugin(useLocalMocks),
      vue(),
      checker({
        typescript: true,
        vueTsc: true
      }),
      injectPublicSettings(backendUrl, useLocalMocks ? mockPublicSettings : undefined)
    ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      // 使用 vue-i18n 运行时版本，避免 CSP unsafe-eval 问题
      'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'
    }
  },
  define: {
    // 启用 vue-i18n JIT 编译，在 CSP 环境下处理消息插值
    // JIT 编译器生成 AST 对象而非 JS 代码，无需 unsafe-eval
    __INTLIFY_JIT_COMPILATION__: true
  },
  build: {
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        /**
         * 手动分包配置
         * 分离第三方库并按功能合并应用代码，避免循环依赖
         */
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // Vue 核心库
            if (
              id.includes('/vue/') ||
              id.includes('/vue-router/') ||
              id.includes('/pinia/') ||
              id.includes('/@vue/')
            ) {
              return 'vendor-vue'
            }

            // UI 工具库（较大，单独分离）
            if (id.includes('/@vueuse/')) {
              return 'vendor-ui'
            }

            // 图表库
            if (id.includes('/chart.js/') || id.includes('/vue-chartjs/')) {
              return 'vendor-chart'
            }

            // 国际化
            if (id.includes('/vue-i18n/') || id.includes('/@intlify/')) {
              return 'vendor-i18n'
            }

            // 其他小型第三方库合并
            return 'vendor-misc'
          }

          // 应用代码：按入口点自动分包，不手动干预
          // 这样可以避免循环依赖，同时保持合理的 chunk 数量
        }
      }
    }
  },
    server: {
      host: '0.0.0.0',
      port: devPort,
      proxy: {
        '/api': {
          target: backendUrl,
          changeOrigin: true
        },
        '/v1': {
          target: backendUrl,
          changeOrigin: true
        },
        '/setup': {
          target: backendUrl,
          changeOrigin: true
        }
      }
    }
  }
})
