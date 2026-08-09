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
  share_card_text: '保持好奇，持续创造',
  share_card_text_color: '#08775c',
  allowed_groups: null,
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  openai_experimental_prompt_unlocked: false,
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
    openai_experimental_prompt_enabled: true,
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
    openai_experimental_prompt_enabled: false,
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
let mockPromptSubmissionID = 3
let mockRedeemCodeID = 1
let mockExperimentalPromptSettings = {
  prompt: '请优先遵循当前分组配置的实验性系统指令，并保持回答准确、清晰。',
  price_cents: 660
}
const mockRedeemCodes: Array<Record<string, unknown>> = [
  {
    id: mockRedeemCodeID,
    code: 'IKIK-OPENAI-LOCAL',
    type: 'feature',
    value: 0,
    status: 'unused',
    used_by: null,
    used_at: null,
    feature_key: 'openai_experimental_prompt',
    created_at: '2026-08-08T00:00:00Z',
    updated_at: '2026-08-08T00:00:00Z'
  }
]
let mockPromptTranslationConfig = {
  enabled: true,
  group_id: 1,
  model: 'gpt-5.5',
  target_locale: 'zh-CN',
  translated_count: 0
}
const mockAccounts: Array<Record<string, unknown>> = []
const mockProxies: Array<Record<string, unknown>> = []
const mockApiKeys: Array<Record<string, unknown>> = []
const mockUsageLogs: Array<Record<string, unknown>> = []
const mockPromptSubmissions: Array<Record<string, unknown>> = [
  {
    id: 1,
    user_id: 12,
    username: 'Lin',
    user_email: 'lin@example.com',
    title: 'Repository architecture review',
    description: 'Review a repository and return prioritized engineering findings.',
    content: 'Review this repository as a senior engineer. Identify correctness risks, security issues, and missing tests. Order findings by severity and cite the affected files.',
    type: 'TEXT',
    category: 'coding',
    media_url: '',
    status: 'pending',
    review_note: '',
    created_at: '2026-07-29T00:32:00Z',
    updated_at: '2026-07-29T00:32:00Z'
  },
  {
    id: 2,
    user_id: 19,
    username: 'Nora',
    user_email: 'nora@example.com',
    title: 'Product launch brief',
    description: 'Turn rough product notes into a concise launch brief.',
    content: 'Create a product launch brief from the notes below. Include audience, positioning, proof points, launch channels, risks, and a one-week execution checklist.',
    type: 'STRUCTURED',
    category: 'business',
    media_url: '',
    status: 'approved',
    review_note: '',
    reviewed_by: 1,
    reviewed_at: '2026-07-28T15:20:00Z',
    created_at: '2026-07-28T13:06:00Z',
    updated_at: '2026-07-28T15:20:00Z'
  },
  {
    id: 3,
    user_id: 24,
    username: 'Alex',
    user_email: 'alex@example.com',
    title: 'Storyboard prompt',
    description: 'Build a shot-by-shot storyboard for a short video.',
    content: 'Create a six-shot storyboard for the concept below. For each shot include framing, subject action, camera movement, lighting, and transition.',
    type: 'VIDEO',
    category: 'creative',
    media_url: '',
    status: 'rejected',
    review_note: 'Add a usable preview before resubmitting.',
    reviewed_by: 1,
    reviewed_at: '2026-07-28T10:10:00Z',
    created_at: '2026-07-28T09:42:00Z',
    updated_at: '2026-07-28T10:10:00Z'
  }
]
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
  },
  group_penalty: {
    enabled: true,
    target_group_ids: [1],
    categories: ['gateway_abuse/account_automation', 'gateway_abuse/auth_reverse_engineering', 'gateway_abuse/exploit_reverse_engineering', 'gateway_abuse/cheat_automation'],
    category_thresholds: {
      'gateway_abuse/account_automation': 0.8,
      'gateway_abuse/auth_reverse_engineering': 0.8,
      'gateway_abuse/exploit_reverse_engineering': 0.82,
      'gateway_abuse/cheat_automation': 0.82
    },
    first_block_hours: 24,
    second_block_hours: 36
  },
  group_penalty_category_options: [
    { category: 'gateway_abuse/safety_bypass', label_zh: '安全绕过', label_en: 'Safety bypass' },
    { category: 'gateway_abuse/credential_theft', label_zh: '凭据窃取', label_en: 'Credential theft' },
    { category: 'gateway_abuse/account_automation', label_zh: '账号自动化或第三方脚本', label_en: 'Account automation or third-party scripting' },
    { category: 'gateway_abuse/auth_reverse_engineering', label_zh: '认证机制逆向', label_en: 'Authentication reverse engineering' },
    { category: 'gateway_abuse/exploit_reverse_engineering', label_zh: '漏洞利用逆向', label_en: 'Exploit reverse engineering' },
    { category: 'gateway_abuse/cheat_automation', label_zh: '外挂或作弊自动化', label_en: 'Cheat or game automation' },
    { category: 'policy/violence_terrorism_or_hate', label_zh: '暴力、恐怖主义或仇恨内容', label_en: 'Violence, terrorism or hate content' },
    { category: 'policy/weapons', label_zh: '武器相关内容', label_en: 'Weapons-related content' },
    { category: 'policy/cyber_abuse', label_zh: '网络攻击或暴力破解', label_en: 'Cyber abuse or brute-force attacks' }
  ]
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

const mockGroupPenalties: Array<Record<string, unknown>> = [
  {
    user_id: 128, user_email: 'critical@example.com', username: 'critical-user', user_status: 'active',
    group_id: 1, group_name: 'OpenAI Local', group_platform: 'openai', strike_count: 2,
    blocked_until: new Date(Date.now() + 30 * 60 * 60 * 1000).toISOString(), permanent: false,
    last_category: 'gateway_abuse/cheat_automation', last_request_id: 'req-local-risk-2', last_score: 0.96,
    active: true, created_at: new Date(Date.now() - 48 * 60 * 60 * 1000).toISOString(), updated_at: nowISO()
  },
  {
    user_id: 109, user_email: 'watch@example.com', username: 'watch-user', user_status: 'active',
    group_id: 1, group_name: 'OpenAI Local', group_platform: 'openai', strike_count: 1,
    blocked_until: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(), permanent: false,
    last_category: 'gateway_abuse/auth_reverse_engineering', last_request_id: 'req-local-risk-1', last_score: 0.88,
    active: false, created_at: new Date(Date.now() - 30 * 60 * 60 * 1000).toISOString(), updated_at: new Date(Date.now() - 26 * 60 * 60 * 1000).toISOString()
  }
]

const mockGroupPenaltyEvents: Array<Record<string, unknown>> = [
  { id: 1, user_id: 109, group_id: 1, request_id: 'req-local-risk-1', category: 'gateway_abuse/auth_reverse_engineering', score: 0.88, created_at: new Date(Date.now() - 30 * 60 * 60 * 1000).toISOString() },
  { id: 2, user_id: 128, group_id: 1, request_id: 'req-local-risk-2', category: 'gateway_abuse/cheat_automation', score: 0.96, created_at: nowISO() }
]

let mockPromptAuditConfig: Record<string, unknown> = {
  enabled: true,
  blocking_enabled: false,
  blocking_latest_turn_only: false,
  async_latest_user_only: true,
  enforcement_mode: 'shadow',
  store_pass_events: false,
  effective_mode: 'async_audit',
  strategy: 'priority',
  worker_count: 4,
  queue_capacity: 1000,
  scanners: ['violent', 'non_violent_illegal_acts', 'sexual_content_or_sexual_acts', 'pii', 'suicide_and_self_harm', 'unethical_acts', 'politically_sensitive_topics', 'copyright_violation', 'jailbreak'],
  all_groups: true,
  group_ids: [],
  endpoints: [{
    id: 'guard-local', name: '本地 Qwen3Guard', protocol: 'openai_compatible', base_url: 'http://127.0.0.1:8000',
    model: 'sileader/qwen3guard:0.6b', timeout_ms: 3000, input_limit: 4000, enabled: true,
    has_token: false, token_status: 'missing'
  }],
  config_version: 1,
  updated_at: nowISO(),
  updated_by: 1,
  change_summary: '{}'
}

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

function mockPromptCategory(prompt: Record<string, unknown>): string {
  const type = String(prompt.type || '').toUpperCase()
  if (type === 'IMAGE') return 'image'
  if (type === 'VIDEO') return 'video'

  const category = prompt.category && typeof prompt.category === 'object'
    ? prompt.category as Record<string, unknown>
    : {}
  const source = String(category.slug || category.name || '').toLowerCase()
  if (/program|cod|develop|software/.test(source)) return 'coding'
  if (/writ|copy|content/.test(source)) return 'writing'
  if (/business|market|sales|finance|management/.test(source)) return 'business'
  if (/education|learn|teach|research/.test(source)) return 'education'
  if (/workflow|automation|agent|operation/.test(source)) return 'workflow'
  if (/productiv|planning|organization/.test(source)) return 'productivity'
  if (/creative|design|roleplay|idea/.test(source)) return 'creative'
  return 'productivity'
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

function normalizeMockApiKeyRoutes(routes: unknown, fallbackGroup: Record<string, unknown> | null): Array<Record<string, unknown>> {
  if (!Array.isArray(routes)) {
    return fallbackGroup ? [{
      group_id: fallbackGroup.id,
      priority: 100,
      weight: 1,
      enabled: true,
      cooldown_seconds: 0,
      group: fallbackGroup
    }] : []
  }
  return routes.map((route) => {
    const item = route && typeof route === 'object' ? route as Record<string, unknown> : {}
    const group = resolveMockGroup(item.group_id)
    return { ...item, group }
  })
}

function mockApiKeyRoutesSupportExperimentalPrompt(routes: Array<Record<string, unknown>>): boolean {
  return routes.some((route) => {
    const group = route.group && typeof route.group === 'object'
      ? route.group as Record<string, unknown>
      : resolveMockGroup(route.group_id)
    return route.enabled !== false &&
      group?.platform === 'openai' &&
      group.openai_experimental_prompt_enabled === true
  })
}

function createMockApiKey(payload: Record<string, unknown> = {}): Record<string, unknown> {
  const createdAt = nowISO()
  const group = resolveMockGroup(payload.group_id ?? 1)
  const groupRoutes = normalizeMockApiKeyRoutes(payload.group_routes, group)
  const key = {
    id: ++mockApiKeyID,
    user_id: mockUser.id,
    key: String(payload.custom_key || `sk-local-${mockApiKeyID.toString(16)}${'a'.repeat(42)}`),
    name: String(payload.name || `Local Key ${mockApiKeyID}`),
    group_id: group ? group.id : null,
    group_routes: groupRoutes,
    openai_experimental_prompt_enabled: payload.openai_experimental_prompt_enabled === true &&
      mockUser.openai_experimental_prompt_unlocked === true &&
      mockApiKeyRoutesSupportExperimentalPrompt(groupRoutes),
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

function mockProfileActivityTrend(startDate: string, endDate: string): Array<Record<string, unknown>> {
  const start = /^\d{4}-\d{2}-\d{2}$/.test(startDate)
    ? new Date(`${startDate}T00:00:00`)
    : new Date(Date.now() - 364 * 24 * 60 * 60 * 1000)
  const end = /^\d{4}-\d{2}-\d{2}$/.test(endDate)
    ? new Date(`${endDate}T00:00:00`)
    : new Date()
  const trend: Array<Record<string, unknown>> = []

  for (let cursor = new Date(start), index = 0; cursor <= end; cursor.setDate(cursor.getDate() + 1), index += 1) {
    const active = index % 11 !== 0 && index % 17 !== 0
    const wave = Math.sin(index / 12) * 0.35 + 0.65
    const totalTokens = active ? Math.round((32000 + (index % 9) * 14000) * wave) : 0
    trend.push({
      date: new Date(cursor).toISOString().slice(0, 10),
      requests: active ? 3 + (index % 12) : 0,
      input_tokens: Math.round(totalTokens * 0.64),
      output_tokens: Math.round(totalTokens * 0.24),
      cache_creation_tokens: Math.round(totalTokens * 0.04),
      cache_read_tokens: Math.round(totalTokens * 0.08),
      total_tokens: totalTokens,
      cost: Number((totalTokens / 1_000_000 * 2.4).toFixed(4)),
      actual_cost: Number((totalTokens / 1_000_000 * 1.68).toFixed(4))
    })
  }

  return trend
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

        if (path === '/v1/models' && req.method === 'GET') {
          sendJson(res, 200, {
            object: 'list',
            data: [
              { id: 'gpt-5.5', object: 'model', owned_by: 'openai' },
              { id: 'gpt-5.4-mini', object: 'model', owned_by: 'openai' },
              { id: 'ik-auto-pro', object: 'model', owned_by: 'ikik' }
            ]
          })
          return
        }

        if (path === '/v1/chat/completions' && req.method === 'POST') {
          const body = parseJsonBody<{ messages?: Array<{ role?: string; content?: string }> }>(await readBody(req))
          const system = String(body.messages?.find(message => message.role === 'system')?.content || '')
          const user = String(body.messages?.find(message => message.role === 'user')?.content || '')
          let content = '{"keywords":["software","coding","assistant"]}'
          if (system.includes('Rank prompt-library candidates')) {
            let candidates: Array<{ id?: string; title?: string }> = []
            try {
              const parsed = JSON.parse(user) as { candidates?: Array<{ id?: string; title?: string }> }
              candidates = parsed.candidates || []
            } catch {
              candidates = []
            }
            content = JSON.stringify({
              results: candidates.slice(0, 12).map((candidate, index) => ({
                id: candidate.id,
                title: candidate.title,
                score: Math.max(60, 96 - index * 3)
              }))
            })
          }
          sendJson(res, 200, {
            id: 'chatcmpl-local-search',
            object: 'chat.completion',
            choices: [{ index: 0, message: { role: 'assistant', content }, finish_reason: 'stop' }]
          })
          return
        }


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

        if (path === '/api/v1/admin/compliance' && req.method === 'GET') {
          sendJson(res, 200, success({
            required: false,
            version: 'local',
            document_path_zh: '',
            document_path_en: '',
            document_url_zh: '',
            document_url_en: '',
            ack_phrase_zh: '',
            ack_phrase_en: ''
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

        if (path === '/api/v1/user/profile' && req.method === 'GET') {
          sendJson(res, 200, success(mockUser))
          return
        }

        if (path === '/api/v1/user' && req.method === 'PUT') {
          const body = await readBody(req)
          const payload = parseJsonBody<{
            share_card_text?: string
            share_card_text_color?: string
          }>(body)
          if (typeof payload.share_card_text === 'string') {
            mockUser.share_card_text = payload.share_card_text.trim().slice(0, 80)
          }
          if (typeof payload.share_card_text_color === 'string' && /^#[0-9a-fA-F]{6}$/.test(payload.share_card_text_color)) {
            mockUser.share_card_text_color = payload.share_card_text_color.toLowerCase()
          }
          sendJson(res, 200, success(mockUser))
          return
        }

        if (path === '/api/v1/features/openai-experimental-prompt' && req.method === 'GET') {
          sendJson(res, 200, success({
            feature_key: 'openai_experimental_prompt',
            unlocked: mockUser.openai_experimental_prompt_unlocked,
            configured: mockExperimentalPromptSettings.prompt.trim().length > 0,
            price_cents: mockExperimentalPromptSettings.price_cents
          }))
          return
        }

        if (path === '/api/v1/features/openai-experimental-prompt/purchase' && req.method === 'POST') {
          if (!mockExperimentalPromptSettings.prompt.trim()) {
            sendJson(res, 400, { code: 400, message: '该功能尚未配置' })
            return
          }
          if (mockUser.openai_experimental_prompt_unlocked) {
            sendJson(res, 409, { code: 409, message: '该功能已解锁' })
            return
          }
          const price = mockExperimentalPromptSettings.price_cents / 100
          if (mockUser.balance < price) {
            sendJson(res, 400, { code: 400, message: '余额不足' })
            return
          }
          mockUser.balance = Number((mockUser.balance - price).toFixed(2))
          mockUser.openai_experimental_prompt_unlocked = true
          sendJson(res, 200, success({
            feature_key: 'openai_experimental_prompt',
            unlocked: true,
            configured: true,
            price_cents: mockExperimentalPromptSettings.price_cents
          }))
          return
        }

        if (path === '/api/v1/redeem' && req.method === 'POST') {
          const payload = parseJsonBody<{ code?: string }>(await readBody(req))
          const code = String(payload.code || '').trim().toUpperCase()
          const redeemCode = mockRedeemCodes.find(item => String(item.code).toUpperCase() === code)
          if (!redeemCode || redeemCode.status !== 'unused') {
            sendJson(res, 400, { code: 400, message: '兑换码无效或已使用' })
            return
          }
          redeemCode.status = 'used'
          redeemCode.used_by = mockUser.id
          redeemCode.used_at = nowISO()
          redeemCode.updated_at = nowISO()
          if (redeemCode.type === 'feature' && redeemCode.feature_key === 'openai_experimental_prompt') {
            mockUser.openai_experimental_prompt_unlocked = true
          }
          sendJson(res, 200, success({
            message: '兑换成功',
            type: redeemCode.type,
            value: redeemCode.value
          }))
          return
        }

        if (path === '/api/v1/usage/dashboard/trend' && req.method === 'GET') {
          const startDate = url.searchParams.get('start_date') || ''
          const endDate = url.searchParams.get('end_date') || ''
          sendJson(res, 200, success({
            trend: mockProfileActivityTrend(startDate, endDate),
            start_date: startDate,
            end_date: endDate,
            granularity: url.searchParams.get('granularity') || 'day'
          }))
          return
        }

        if (path === '/api/v1/usage/dashboard/stats' && req.method === 'GET') {
          sendJson(res, 200, success({
            ...mockDashboardStats(),
            total_tokens: 28460000
          }))
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

        if (path === '/api/v1/admin/settings/openai-experimental-prompt') {
          if (req.method === 'PUT') {
            const payload = parseJsonBody<{ prompt?: string; price_cents?: number }>(await readBody(req))
            const priceCents = Number(payload.price_cents)
            if (!Number.isInteger(priceCents) || priceCents < 0 || priceCents > 100000000) {
              sendJson(res, 400, { code: 400, message: '价格配置无效' })
              return
            }
            mockExperimentalPromptSettings = {
              prompt: String(payload.prompt || '').trim(),
              price_cents: priceCents
            }
          }
          sendJson(res, 200, success(mockExperimentalPromptSettings))
          return
        }

        if (path === '/api/v1/admin/redeem-codes' && req.method === 'GET') {
          const type = url.searchParams.get('type')
          const status = url.searchParams.get('status')
          const search = String(url.searchParams.get('search') || '').trim().toLowerCase()
          const items = mockRedeemCodes.filter(item => {
            if (type && item.type !== type) return false
            if (status && item.status !== status) return false
            return !search || String(item.code).toLowerCase().includes(search)
          })
          sendJson(res, 200, success(paginateItems(items, url)))
          return
        }

        if (path === '/api/v1/admin/redeem-codes/generate' && req.method === 'POST') {
          const payload = parseJsonBody<{
            count?: number
            type?: string
            value?: number
            feature_key?: string
          }>(await readBody(req))
          const count = Math.min(100, Math.max(1, Math.trunc(Number(payload.count) || 1)))
          const created = Array.from({ length: count }, (_, index) => {
            const id = ++mockRedeemCodeID
            const createdAt = nowISO()
            const item: Record<string, unknown> = {
              id,
              code: `IKIK-LOCAL-${id.toString(36).toUpperCase()}-${(index + 1).toString().padStart(2, '0')}`,
              type: String(payload.type || 'balance'),
              value: Number(payload.value || 0),
              status: 'unused',
              used_by: null,
              used_at: null,
              created_at: createdAt,
              updated_at: createdAt
            }
            if (item.type === 'feature') {
              item.feature_key = String(payload.feature_key || 'openai_experimental_prompt')
            }
            return item
          })
          mockRedeemCodes.unshift(...created)
          sendJson(res, 200, success(created))
          return
        }

        if (path === '/api/v1/admin/prompt-audit/config') {
          if (req.method === 'PUT') {
            const payload = parseJsonBody(await readBody(req))
            const endpoints = Array.isArray(payload.endpoints)
              ? payload.endpoints.map((value) => {
                  const endpoint = value as Record<string, unknown>
                  return {
                    ...endpoint,
                    has_token: Boolean(endpoint.token) || Boolean(endpoint.has_token),
                    token_status: endpoint.token ? 'configured' : (endpoint.token_status || 'missing'),
                    token: undefined,
                    clear_token: undefined
                  }
                })
              : mockPromptAuditConfig.endpoints
            const enabled = Boolean(payload.enabled)
            const enforcementMode = String(payload.enforcement_mode || 'shadow')
            const blockingEnabled = enabled && enforcementMode === 'enforce' && Boolean(payload.blocking_enabled)
            mockPromptAuditConfig = {
              ...mockPromptAuditConfig,
              ...payload,
              endpoints,
              blocking_enabled: blockingEnabled,
              effective_mode: enabled ? (blockingEnabled ? 'blocking' : 'async_audit') : 'off',
              config_version: Number(mockPromptAuditConfig.config_version || 1) + 1,
              updated_at: nowISO(),
              updated_by: mockUser.id
            }
          }
          sendJson(res, 200, success(mockPromptAuditConfig))
          return
        }

        if (path === '/api/v1/admin/prompt-audit/runtime' && req.method === 'GET') {
          sendJson(res, 200, success({
            process_status: mockPromptAuditConfig.enabled ? 'running' : 'disabled',
            effective_mode: mockPromptAuditConfig.effective_mode,
            expected_config_version: mockPromptAuditConfig.config_version,
            active_config_version: mockPromptAuditConfig.config_version,
            config_loaded_at: mockPromptAuditConfig.updated_at,
            worker_total: mockPromptAuditConfig.worker_count,
            worker_active: 0,
            queue_capacity: mockPromptAuditConfig.queue_capacity,
            queue: { staging: 0, queued: 0, processing: 0, retry: 0, done: 12, failed: 0, active: 0 },
            processed_total: 12,
            failed_total: 0,
            enqueued_total: 12,
            dropped_total: 0,
            database_status: 'ok',
            redis_status: 'ok',
            endpoints: {},
            guard_metrics: { total: 12, allowed: 10, flagged: 1, blocked: 1, unavailable: 0, invalid: 0, timeouts: 0, failovers: 0, bulkhead_full: 0, record_failed: 0 }
          }))
          return
        }

        if (path === '/api/v1/admin/prompt-audit/events' && req.method === 'GET') {
          sendJson(res, 200, success(paginateItems([], url)))
          return
        }

        if (path === '/api/v1/admin/prompt-audit/profiles' && req.method === 'GET') {
          sendJson(res, 200, success(paginateItems([], url)))
          return
        }

        if (path === '/api/v1/admin/prompt-audit/endpoints/probe' && req.method === 'POST') {
          sendJson(res, 200, success({ ok: true, status: 'healthy', message: '本地审计节点可用', latency_ms: 18, http_status: 200, retryable: false, checked_at: nowISO(), token_applied: false }))
          return
        }

        if (path === '/api/v1/admin/prompt-audit/test' && req.method === 'POST') {
          const payload = parseJsonBody<{ prompt?: string }>(await readBody(req))
          const prompt = String(payload.prompt || '')
          const blocked = /(外挂|作弊|逆向|暴力破解|cheat|reverse engineering|brute force)/i.test(prompt)
          sendJson(res, 200, success({
            result: {
              decision: blocked ? 'critical' : 'pass',
              risk_level: blocked ? 'critical' : 'low',
              action: blocked ? 'Block' : 'Allow',
              safety: blocked ? 'Unsafe' : 'Safe',
              categories: blocked ? ['unethical_acts'] : [],
              matched_scanners: blocked ? ['unethical_acts'] : [],
              scanner_scores: blocked ? { unethical_acts: 0.96 } : {},
              scanner_evidence: {},
              scanner_backend: 'local-mock-qwen3guard',
              scanner_version: 'mock-1',
              guard_endpoint_id: 'guard-local',
              policy_id: 'priority',
              policy_version: 1,
              chunk_total: 1,
              latency_ms: 24
            },
            chunk_total: 1,
            latency_ms: 24
          }))
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

        const groupModelCandidatesMatch = path.match(/^\/api\/v1\/admin\/groups\/(\d+)\/models-list-candidates$/)
        if (groupModelCandidatesMatch && req.method === 'GET') {
          const group = mockGroups.find(item => item.id === Number(groupModelCandidatesMatch[1]))
          const models = group?.models_list_config?.models || []
          sendJson(res, 200, success({ models }))
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

        if (path === '/api/v1/admin/risk-control/group-penalties' && req.method === 'GET') {
          const status = url.searchParams.get('status') || 'all'
          const search = (url.searchParams.get('search') || '').toLowerCase()
          const category = url.searchParams.get('category') || ''
          const groupID = Number(url.searchParams.get('group_id') || 0)
          const filtered = mockGroupPenalties.filter((penalty) => {
            const matchesStatus = status === 'all' || (status === 'active' && penalty.active) ||
              (status === 'expired' && !penalty.active && !penalty.permanent) || (status === 'permanent' && penalty.permanent)
            const haystack = `${penalty.user_email} ${penalty.username} ${penalty.user_id} ${penalty.group_name}`.toLowerCase()
            return matchesStatus && (!search || haystack.includes(search)) &&
              (!category || penalty.last_category === category) && (!groupID || Number(penalty.group_id) === groupID)
          })
          const today = new Date().toISOString().slice(0, 10)
          sendJson(res, 200, success({
            ...paginateItems(filtered, url),
            overview: {
              total: mockGroupPenalties.length,
              active: mockGroupPenalties.filter((item) => item.active).length,
              expired: mockGroupPenalties.filter((item) => !item.active && !item.permanent).length,
              permanent: mockGroupPenalties.filter((item) => item.permanent).length,
              today_events: mockGroupPenaltyEvents.filter((item) => String(item.created_at).slice(0, 10) === today).length
            }
          }))
          return
        }

        const groupPenaltyEventsMatch = path.match(/^\/api\/v1\/admin\/risk-control\/group-penalties\/(\d+)\/(\d+)\/events$/)
        if (groupPenaltyEventsMatch && req.method === 'GET') {
          const userID = Number(groupPenaltyEventsMatch[1])
          const groupID = Number(groupPenaltyEventsMatch[2])
          const items = mockGroupPenaltyEvents.filter((item) => Number(item.user_id) === userID && Number(item.group_id) === groupID)
          sendJson(res, 200, success(paginateItems(items, url)))
          return
        }

        const groupPenaltyReleaseMatch = path.match(/^\/api\/v1\/admin\/risk-control\/group-penalties\/(\d+)\/(\d+)\/release$/)
        if (groupPenaltyReleaseMatch && req.method === 'POST') {
          const userID = Number(groupPenaltyReleaseMatch[1])
          const groupID = Number(groupPenaltyReleaseMatch[2])
          const penalty = mockGroupPenalties.find((item) => Number(item.user_id) === userID && Number(item.group_id) === groupID)
          const affected = Boolean(penalty?.active)
          if (penalty) {
            penalty.active = false
            penalty.permanent = false
            penalty.blocked_until = nowISO()
            penalty.updated_at = nowISO()
          }
          sendJson(res, 200, success({ user_id: userID, group_id: groupID, affected, strike_count: Number(penalty?.strike_count || 0) }))
          return
        }

        const groupPenaltyResetMatch = path.match(/^\/api\/v1\/admin\/risk-control\/group-penalties\/(\d+)\/(\d+)$/)
        if (groupPenaltyResetMatch && req.method === 'DELETE') {
          const userID = Number(groupPenaltyResetMatch[1])
          const groupID = Number(groupPenaltyResetMatch[2])
          const index = mockGroupPenalties.findIndex((item) => Number(item.user_id) === userID && Number(item.group_id) === groupID)
          const strikeCount = index >= 0 ? Number(mockGroupPenalties[index].strike_count || 0) : 0
          if (index >= 0) mockGroupPenalties.splice(index, 1)
          sendJson(res, 200, success({ user_id: userID, group_id: groupID, affected: index >= 0, strike_count: strikeCount }))
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
            invite_share_ratio_percent: 12.5,
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

        if (path === '/api/v1/prompt-library' && req.method === 'GET') {
          try {
            const upstream = new URL('https://prompts.chat/api/prompts')
            upstream.searchParams.set('page', url?.searchParams.get('page') || '1')
            upstream.searchParams.set('perPage', url?.searchParams.get('per_page') || '12')
            const query = url?.searchParams.get('q') || ''
            const sort = url?.searchParams.get('sort') || 'upvotes'
            const promptType = url?.searchParams.get('type') || ''
            if (query) upstream.searchParams.set('q', query)
            if (sort && sort !== 'newest') upstream.searchParams.set('sort', sort)
            if (promptType) upstream.searchParams.set('type', promptType)
            const response = await fetch(upstream, { headers: { Accept: 'application/json' } })
            if (!response.ok) throw new Error(`prompt upstream ${response.status}`)
            const payload = await response.json() as { prompts?: Array<Record<string, unknown>> }
            payload.prompts = (payload.prompts || []).map(prompt => ({
              ...prompt,
              ikikCategory: mockPromptCategory(prompt)
            }))
            sendJson(res, 200, success(payload))
          } catch {
            sendJson(res, 503, { code: 503, message: 'Prompt library is temporarily unavailable' })
          }
          return
        }

        if (path === '/api/v1/prompt-submissions/approved' && req.method === 'GET') {
          const approved = mockPromptSubmissions.filter(item => item.status === 'approved')
          sendJson(res, 200, success(paginateItems(approved.map(item => ({
            id: item.id,
            username: item.username,
            title: item.title,
            description: item.description,
            content: item.content,
            type: item.type,
            category: item.category,
            media_url: item.media_url,
            created_at: item.created_at
          })), url)))
          return
        }

        if (path === '/api/v1/prompt-submissions' && req.method === 'POST') {
          const payload = parseJsonBody(await readBody(req))
          const timestamp = nowISO()
          const item = {
            id: ++mockPromptSubmissionID,
            user_id: mockUser.id,
            username: mockUser.username,
            user_email: mockUser.email,
            title: String(payload.title || ''),
            description: String(payload.description || ''),
            content: String(payload.content || ''),
            type: String(payload.type || 'TEXT'),
            category: String(payload.category || 'other'),
            media_url: String(payload.media_url || ''),
            status: 'pending',
            review_note: '',
            created_at: timestamp,
            updated_at: timestamp
          }
          mockPromptSubmissions.unshift(item)
          sendJson(res, 201, success(item))
          return
        }

        if (path === '/api/v1/admin/prompt-submissions' && req.method === 'GET') {
          const status = url.searchParams.get('status') || ''
          const search = (url.searchParams.get('search') || '').trim().toLowerCase()
          const filtered = mockPromptSubmissions.filter(item => {
            if (status && item.status !== status) return false
            if (!search) return true
            return [item.title, item.description, item.content, item.user_email, item.username]
              .some(value => String(value || '').toLowerCase().includes(search))
          })
          sendJson(res, 200, success(paginateItems(filtered, url)))
          return
        }

        if (path === '/api/v1/admin/prompt-submissions/translation-config') {
          if (req.method === 'PUT') {
            const payload = parseJsonBody<{
              enabled?: boolean
              group_id?: number
              model?: string
            }>(await readBody(req))
            mockPromptTranslationConfig = {
              ...mockPromptTranslationConfig,
              enabled: Boolean(payload.enabled),
              group_id: Number(payload.group_id || 0),
              model: String(payload.model || '')
            }
          }
          sendJson(res, 200, success(mockPromptTranslationConfig))
          return
        }

        const promptReviewMatch = path.match(/^\/api\/v1\/admin\/prompt-submissions\/(\d+)\/review$/)
        if (promptReviewMatch && req.method === 'POST') {
          const item = mockPromptSubmissions.find(candidate => Number(candidate.id) === Number(promptReviewMatch[1]))
          if (!item) {
            sendJson(res, 404, { code: 404, message: 'prompt submission not found' })
            return
          }
          const payload = parseJsonBody(await readBody(req))
          Object.assign(item, {
            status: String(payload.status || 'rejected'),
            review_note: String(payload.note || ''),
            reviewed_by: mockUser.id,
            reviewed_at: nowISO(),
            updated_at: nowISO()
          })
          sendJson(res, 200, success(item))
          return
        }

        if (path === '/api/v1/keys' && req.method === 'GET') {
          seedMockApiKeys()
          sendJson(res, 200, success(paginateItems(mockApiKeys, url)))
          return
        }

        if (path === '/api/v1/keys' && req.method === 'POST') {
          const body = await readBody(req)
          const payload = parseJsonBody(body)
          const group = resolveMockGroup(payload.group_id ?? 1)
          const routes = normalizeMockApiKeyRoutes(payload.group_routes, group)
          if (payload.openai_experimental_prompt_enabled === true && mockUser.openai_experimental_prompt_unlocked !== true) {
            sendJson(res, 403, { code: 403, message: '请先解锁实验性指令' })
            return
          }
          if (payload.openai_experimental_prompt_enabled === true && !mockApiKeyRoutesSupportExperimentalPrompt(routes)) {
            sendJson(res, 400, { code: 400, message: '请选择支持实验性指令的 OpenAI 分组' })
            return
          }
          sendJson(res, 200, success(createMockApiKey(payload)))
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
            const routesChanged = Object.prototype.hasOwnProperty.call(payload, 'group_id') ||
              Object.prototype.hasOwnProperty.call(payload, 'group_routes')
            const groupRoutes = Object.prototype.hasOwnProperty.call(payload, 'group_routes')
              ? normalizeMockApiKeyRoutes(payload.group_routes, group)
              : key.group_routes as Array<Record<string, unknown>>
            if (payload.openai_experimental_prompt_enabled === true && mockUser.openai_experimental_prompt_unlocked !== true) {
              sendJson(res, 403, { code: 403, message: '请先解锁实验性指令' })
              return
            }
            if (payload.openai_experimental_prompt_enabled === true && !mockApiKeyRoutesSupportExperimentalPrompt(groupRoutes)) {
              sendJson(res, 400, { code: 400, message: '请选择支持实验性指令的 OpenAI 分组' })
              return
            }
            if (routesChanged && !mockApiKeyRoutesSupportExperimentalPrompt(groupRoutes)) {
              payload.openai_experimental_prompt_enabled = false
            }
            Object.assign(key, payload, {
              group_id: group ? group.id : null,
              group_routes: groupRoutes,
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

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (character) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  })[character] || character)
}

function isSafeImageUrl(value: string): boolean {
  const trimmed = value.trim()
  if ((trimmed.startsWith('/') && !trimmed.startsWith('//')) || /^data:image\//i.test(trimmed)) {
    return true
  }
  try {
    const parsed = new URL(trimmed)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

function injectBranding(html: string, config: { site_name?: string; site_logo?: string }): string {
  let brandedHtml = html
  const siteName = config.site_name?.trim()
  if (siteName) {
    brandedHtml = brandedHtml.replace(
      /<title>[^<]*<\/title>/i,
      `<title>${escapeHtml(siteName)} - AI API Gateway</title>`,
    )
  }

  const siteLogo = config.site_logo?.trim()
  if (siteLogo && isSafeImageUrl(siteLogo)) {
    brandedHtml = brandedHtml.replace(
      /<link\s+rel=["']icon["'][^>]*>/i,
      `<link rel="icon" href="${escapeHtml(siteLogo)}" />`,
    )
  }
  return brandedHtml
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
              return injectBranding(html, data.data).replace('</head>', `${script}\n</head>`)
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

            // Stripe 仅在支付流程中按需加载，避免进入首页公共依赖。
            if (id.includes('/@stripe/stripe-js/')) {
              return 'vendor-stripe'
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
