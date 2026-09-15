export function applyInterceptWarmup(
  credentials: Record<string, unknown>,
  enabled: boolean,
  mode: 'create' | 'edit'
): void {
  if (enabled) {
    credentials.intercept_warmup_requests = true
  } else if (mode === 'edit') {
    delete credentials.intercept_warmup_requests
  }
}

export const ANTIGRAVITY_PROJECT_ID_CREDENTIAL_KEY = 'antigravity_project_id'

export function applyAntigravityProjectID(
  credentials: Record<string, unknown>,
  projectId: string,
  mode: 'create' | 'edit'
): void {
  const trimmed = projectId.trim()
  if (trimmed) {
    credentials[ANTIGRAVITY_PROJECT_ID_CREDENTIAL_KEY] = trimmed
  } else if (mode === 'edit') {
    delete credentials[ANTIGRAVITY_PROJECT_ID_CREDENTIAL_KEY]
  }
}

export const HEADER_OVERRIDE_ENABLED_CREDENTIAL_KEY = 'header_override_enabled'
export const HEADER_OVERRIDES_CREDENTIAL_KEY = 'header_overrides'

export interface HeaderOverrideRow {
  name: string
  value: string
}

export function isHeaderOverridePlatform(platform: string): boolean {
  return platform === 'anthropic' || platform === 'openai'
}

export function isHeaderOverrideCapable(platform: string, type: string): boolean {
  if (
    platform === 'anthropic' ||
    platform === 'openai' ||
    platform === 'kimi' ||
    platform === 'zhipu' ||
    platform === 'deepseek' ||
    platform === 'minimax' ||
    platform === 'opencode_go'
  ) {
    return type === 'apikey'
  }
  if (platform === 'grok') {
    return type === 'apikey' || type === 'oauth'
  }
  return false
}

const HEADER_OVERRIDE_BLOCKED_NAMES = new Set([
  'host',
  'content-length',
  'content-type',
  'transfer-encoding',
  'connection',
  'keep-alive',
  'proxy-authenticate',
  'proxy-authorization',
  'proxy-connection',
  'te',
  'trailer',
  'upgrade',
  'authorization',
  'x-api-key',
  'x-goog-api-key',
  'cookie',
  'accept-encoding',
  'sec-websocket-key',
  'sec-websocket-version',
  'sec-websocket-extensions',
  'sec-websocket-protocol',
  'sec-websocket-accept',
  'session_id',
  'conversation_id',
  'x-codex-turn-state',
  'x-codex-turn-metadata',
  'chatgpt-account-id',
  'x-claude-code-session-id',
  'x-client-request-id',
  'x-grok-conv-id'
])

const HEADER_NAME_PATTERN = /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/
const HEADER_OVERRIDE_MAX_ENTRIES = 64
const HEADER_OVERRIDE_MAX_NAME_LENGTH = 200
const HEADER_OVERRIDE_MAX_VALUE_LENGTH = 8192
// eslint-disable-next-line no-control-regex
const HEADER_VALUE_INVALID_PATTERN = /[\x00-\x08\x0a-\x1f\x7f]/
const HEADER_TEXT_ENCODER = new TextEncoder()

const ANTHROPIC_HEADER_OVERRIDE_TEMPLATE = [
  'user-agent',
  'x-app',
  'anthropic-beta',
  'anthropic-version',
  'anthropic-dangerous-direct-browser-access',
  'x-stainless-lang',
  'x-stainless-package-version',
  'x-stainless-os',
  'x-stainless-arch',
  'x-stainless-runtime',
  'x-stainless-runtime-version',
  'x-stainless-retry-count',
  'x-stainless-timeout'
]

const OPENAI_HEADER_OVERRIDE_TEMPLATE = [
  'user-agent',
  'originator',
  'openai-beta',
  'version',
  'accept',
  'accept-language'
]

export function getHeaderOverrideTemplate(platform: string): HeaderOverrideRow[] {
  const names =
    platform === 'openai' ? OPENAI_HEADER_OVERRIDE_TEMPLATE : ANTHROPIC_HEADER_OVERRIDE_TEMPLATE
  return names.map((name) => ({ name, value: '' }))
}

function utf8ByteLength(value: string): number {
  return HEADER_TEXT_ENCODER.encode(value).length
}

export function validateHeaderOverrideRows(
  rows: HeaderOverrideRow[]
): 'invalidName' | 'blockedName' | 'duplicateName' | 'invalidValue' | 'tooManyEntries' | null {
  const seen = new Set<string>()
  for (const row of rows) {
    const name = row.name.trim()
    const value = row.value.trim()
    if (!name) {
      if (value) return 'invalidName'
      continue
    }
    if (!HEADER_NAME_PATTERN.test(name) || name.length > HEADER_OVERRIDE_MAX_NAME_LENGTH) {
      return 'invalidName'
    }
    const lower = name.toLowerCase()
    if (HEADER_OVERRIDE_BLOCKED_NAMES.has(lower)) return 'blockedName'
    if (seen.has(lower)) return 'duplicateName'
    if (
      HEADER_VALUE_INVALID_PATTERN.test(value) ||
      utf8ByteLength(value) > HEADER_OVERRIDE_MAX_VALUE_LENGTH
    ) {
      return 'invalidValue'
    }
    seen.add(lower)
  }
  if (seen.size > HEADER_OVERRIDE_MAX_ENTRIES) return 'tooManyEntries'
  return null
}

export function buildHeaderOverridesObject(rows: HeaderOverrideRow[]): Record<string, string> {
  // 名称小写化、值裁剪；仅丢弃空名称行（空值保留，与上游语义一致）。
  const result: Record<string, string> = {}
  for (const row of rows) {
    const name = row.name.trim().toLowerCase()
    if (!name) continue
    result[name] = row.value.trim()
  }
  return result
}

export function splitHeaderOverridesObject(record: unknown): HeaderOverrideRow[] {
  if (!record || typeof record !== 'object' || Array.isArray(record)) return []
  return Object.entries(record as Record<string, unknown>)
    .filter(([, value]) => typeof value === 'string')
    .map(([name, value]) => ({ name, value: value as string }))
    .sort((a, b) => a.name.localeCompare(b.name))
}

export function parseHeaderOverridesJson(text: string): HeaderOverrideRow[] | null {
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    return null
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null

  const rows: HeaderOverrideRow[] = []
  for (const [rawName, rawValue] of Object.entries(parsed as Record<string, unknown>)) {
    const name = rawName.trim()
    if (!name) continue
    if (
      typeof rawValue !== 'string' &&
      typeof rawValue !== 'number' &&
      typeof rawValue !== 'boolean'
    ) {
      return null
    }
    rows.push({ name, value: String(rawValue).trim() })
  }
  return rows.sort((a, b) => a.name.localeCompare(b.name))
}

export function serializeHeaderOverrideRows(rows: HeaderOverrideRow[]): string {
  const record: Record<string, string> = {}
  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    record[name] = row.value.trim()
  }
  return JSON.stringify(record, null, 2)
}

export function applyHeaderOverride(
  credentials: Record<string, unknown>,
  enabled: boolean,
  rows: HeaderOverrideRow[],
  mode: 'create' | 'edit'
): void {
  if (enabled) {
    credentials[HEADER_OVERRIDE_ENABLED_CREDENTIAL_KEY] = true
    credentials[HEADER_OVERRIDES_CREDENTIAL_KEY] = buildHeaderOverridesObject(rows)
  } else if (mode === 'edit') {
    delete credentials[HEADER_OVERRIDE_ENABLED_CREDENTIAL_KEY]
    delete credentials[HEADER_OVERRIDES_CREDENTIAL_KEY]
  }
}

const GROK_DEFAULT_GATEWAY_HOST = 'cli-chat-proxy.grok.com'

export function isCustomGrokBaseUrl(value: unknown): boolean {
  if (typeof value !== 'string') return false
  const trimmed = value.trim()
  if (!trimmed) return false
  try {
    return new URL(trimmed).hostname.toLowerCase() !== GROK_DEFAULT_GATEWAY_HOST
  } catch {
    return false
  }
}

export interface GrokBaseUrlPreset {
  labelKey?: 'cli' | 'official'
  label?: string
  url: string
}

export const GROK_BASE_URL_PRESETS: GrokBaseUrlPreset[] = [
  { labelKey: 'cli', url: 'https://cli-chat-proxy.grok.com/v1' },
  { labelKey: 'official', url: 'https://api.x.ai/v1' },
  { label: 'us-east-1', url: 'https://us-east-1.api.x.ai/v1' },
  { label: 'us-west-2', url: 'https://us-west-2.api.x.ai/v1' },
  { label: 'eu-west-1', url: 'https://eu-west-1.api.x.ai/v1' }
]

export type CnAccountMode = 'payg' | 'coding'
export type OpenCodeAccountMode = 'zen' | 'go'
export type CnProviderPlatform = 'kimi' | 'zhipu' | 'deepseek' | 'minimax'

/** deepseek / kimi / minimax 支持原生 responses；adaptive 会按入站协议选择原生端点。 */
export type CnApiProtocol = 'adaptive' | 'chat_completions' | 'anthropic' | 'responses'
export type CnNativeApiProtocol = Exclude<CnApiProtocol, 'adaptive'>

export function isCNProviderPlatform(platform: string): platform is CnProviderPlatform {
  return platform === 'kimi' || platform === 'zhipu' || platform === 'deepseek' || platform === 'minimax'
}

/** DeepSeek、Kimi 与 MiniMax 提供原生 Responses 端点。 */
export function cnSupportsNativeResponses(platform: string): boolean {
  return platform === 'deepseek' || platform === 'kimi' || platform === 'minimax' || platform === 'opencode_go'
}

export const OPENCODE_GO_BASE_URL = 'https://opencode.ai/zen/go/v1'
export const OPENCODE_GO_ANTHROPIC_BASE_URL = 'https://opencode.ai/zen/go'
export const OPENCODE_ZEN_BASE_URL = 'https://opencode.ai/zen/v1'
export const OPENCODE_ZEN_ANTHROPIC_BASE_URL = 'https://opencode.ai/zen'

export function isOpenCodeGoPlatform(platform: string): boolean {
  return platform === 'opencode_go'
}

export const OPENCODE_GO_PROTOCOL_RULES_KEY = 'protocol_rules'

export interface OpenCodeGoProtocolRule {
  pattern: string
  protocol: CnNativeApiProtocol
}

export const DEFAULT_OPENCODE_GO_PROTOCOL_RULES: OpenCodeGoProtocolRule[] = [
  { pattern: 'grok-*', protocol: 'responses' },
  { pattern: 'gpt-*', protocol: 'responses' },
  { pattern: 'muse-spark-*', protocol: 'responses' },
  { pattern: 'minimax-*', protocol: 'anthropic' },
  { pattern: 'qwen*', protocol: 'anthropic' }
]

export const DEFAULT_OPENCODE_ZEN_PROTOCOL_RULES: OpenCodeGoProtocolRule[] = [
  { pattern: 'grok-*', protocol: 'responses' },
  { pattern: 'gpt-*', protocol: 'responses' },
  { pattern: 'muse-spark-*', protocol: 'responses' },
  { pattern: 'claude-*', protocol: 'anthropic' },
  { pattern: 'qwen*', protocol: 'anthropic' }
]

export function resolveOpenCodeAccountMode(value: unknown): OpenCodeAccountMode {
  return value === 'zen' ? 'zen' : 'go'
}

export function defaultOpenCodeProtocolRules(mode: OpenCodeAccountMode = 'go'): OpenCodeGoProtocolRule[] {
  return mode === 'zen' ? DEFAULT_OPENCODE_ZEN_PROTOCOL_RULES : DEFAULT_OPENCODE_GO_PROTOCOL_RULES
}

export function cloneOpenCodeGoProtocolRules(
  rules: OpenCodeGoProtocolRule[] = DEFAULT_OPENCODE_GO_PROTOCOL_RULES
): OpenCodeGoProtocolRule[] {
  return rules.map(rule => ({ pattern: rule.pattern, protocol: rule.protocol }))
}

function isNativeOpenCodeGoProtocol(value: unknown): value is CnNativeApiProtocol {
  return value === 'chat_completions' || value === 'anthropic' || value === 'responses'
}

export function parseOpenCodeGoProtocolRules(raw: unknown): OpenCodeGoProtocolRule[] | null {
  if (raw == null) return null
  if (!Array.isArray(raw)) return cloneOpenCodeGoProtocolRules()
  const rules: OpenCodeGoProtocolRule[] = []
  for (const item of raw) {
    if (!item || typeof item !== 'object') continue
    const pattern = typeof (item as { pattern?: unknown }).pattern === 'string'
      ? (item as { pattern: string }).pattern.trim()
      : ''
    const protocol = (item as { protocol?: unknown }).protocol
    if (!pattern || !isNativeOpenCodeGoProtocol(protocol)) continue
    rules.push({ pattern, protocol })
  }
  return rules
}

export function applyOpenCodeGoProtocolRules(
  credentials: Record<string, unknown>,
  rules: OpenCodeGoProtocolRule[],
  mode: 'create' | 'edit'
): void {
  const serialized = rules
    .map(rule => ({
      pattern: rule.pattern.trim().toLowerCase(),
      protocol: rule.protocol
    }))
    .filter(rule => rule.pattern.length > 0 && isNativeOpenCodeGoProtocol(rule.protocol))
  if (serialized.length > 0 || mode === 'edit') {
    credentials[OPENCODE_GO_PROTOCOL_RULES_KEY] = serialized
  }
}

export function isMultiProtocolApiKeyPlatform(platform: string): boolean {
  return platform === 'kimi' || platform === 'zhipu' || platform === 'deepseek' || platform === 'minimax' || platform === 'opencode_go'
}

export interface CnBaseUrlPreset {
  mode: CnAccountMode
  protocol: CnApiProtocol
  label: string
  url: string
}

/** 各供应商按账号类型 × API 协议分档的快捷端点（点击快速填充，输入框仍可自由填写）。 */
export const CN_BASE_URL_PRESETS: Record<CnProviderPlatform, CnBaseUrlPreset[]> = {
  kimi: [
    { mode: 'payg', protocol: 'chat_completions', label: 'Moonshot', url: 'https://api.moonshot.cn/v1' },
    { mode: 'payg', protocol: 'anthropic', label: 'Moonshot Anthropic', url: 'https://api.moonshot.cn/anthropic' },
    { mode: 'payg', protocol: 'responses', label: 'Moonshot Responses', url: 'https://api.moonshot.cn/v1' },
    { mode: 'coding', protocol: 'chat_completions', label: 'Kimi For Coding', url: 'https://api.kimi.com/coding/v1' },
    { mode: 'coding', protocol: 'anthropic', label: 'Kimi Coding Anthropic', url: 'https://api.kimi.com/coding' },
    { mode: 'coding', protocol: 'responses', label: 'Kimi Coding Responses', url: 'https://api.kimi.com/coding/v1' }
  ],
  zhipu: [
    { mode: 'payg', protocol: 'chat_completions', label: 'GLM PaaS', url: 'https://open.bigmodel.cn/api/paas/v4' },
    { mode: 'payg', protocol: 'anthropic', label: 'GLM Anthropic', url: 'https://open.bigmodel.cn/api/anthropic' },
    { mode: 'coding', protocol: 'chat_completions', label: 'GLM Coding', url: 'https://open.bigmodel.cn/api/coding/paas/v4' },
    { mode: 'coding', protocol: 'anthropic', label: 'GLM Coding Anthropic', url: 'https://open.bigmodel.cn/api/anthropic' }
  ],
  deepseek: [
    { mode: 'payg', protocol: 'chat_completions', label: 'DeepSeek', url: 'https://api.deepseek.com' },
    { mode: 'payg', protocol: 'anthropic', label: 'DeepSeek Anthropic', url: 'https://api.deepseek.com/anthropic' },
    { mode: 'payg', protocol: 'responses', label: 'DeepSeek Responses', url: 'https://api.deepseek.com' }
  ],
  minimax: [
    { mode: 'payg', protocol: 'chat_completions', label: 'MiniMax CN', url: 'https://api.minimaxi.com/v1' },
    { mode: 'payg', protocol: 'anthropic', label: 'MiniMax CN Anthropic', url: 'https://api.minimaxi.com/anthropic' },
    { mode: 'payg', protocol: 'responses', label: 'MiniMax CN Responses', url: 'https://api.minimaxi.com/v1' },
    { mode: 'payg', protocol: 'chat_completions', label: 'MiniMax Intl', url: 'https://api.minimax.io/v1' },
    { mode: 'payg', protocol: 'anthropic', label: 'MiniMax Intl Anthropic', url: 'https://api.minimax.io/anthropic' },
    { mode: 'payg', protocol: 'responses', label: 'MiniMax Intl Responses', url: 'https://api.minimax.io/v1' },
    { mode: 'coding', protocol: 'chat_completions', label: 'MiniMax Coding CN', url: 'https://api.minimaxi.com/v1' },
    { mode: 'coding', protocol: 'anthropic', label: 'MiniMax Coding CN Anthropic', url: 'https://api.minimaxi.com/anthropic' },
    { mode: 'coding', protocol: 'responses', label: 'MiniMax Coding CN Responses', url: 'https://api.minimaxi.com/v1' },
    { mode: 'coding', protocol: 'chat_completions', label: 'MiniMax Coding Intl', url: 'https://api.minimax.io/v1' },
    { mode: 'coding', protocol: 'anthropic', label: 'MiniMax Coding Intl Anthropic', url: 'https://api.minimax.io/anthropic' },
    { mode: 'coding', protocol: 'responses', label: 'MiniMax Coding Intl Responses', url: 'https://api.minimax.io/v1' }
  ]
}

export function defaultCNBaseUrl(
  platform: string,
  mode: CnAccountMode | OpenCodeAccountMode,
  protocol: CnApiProtocol = 'chat_completions'
): string {
  if (protocol === 'anthropic') {
    switch (platform) {
      case 'kimi':
        return mode === 'coding' ? 'https://api.kimi.com/coding' : 'https://api.moonshot.cn/anthropic'
      case 'zhipu':
        return 'https://open.bigmodel.cn/api/anthropic'
      case 'deepseek':
        return 'https://api.deepseek.com/anthropic'
      case 'minimax':
        return 'https://api.minimaxi.com/anthropic'
      case 'opencode_go':
        return mode === 'zen' ? OPENCODE_ZEN_ANTHROPIC_BASE_URL : OPENCODE_GO_ANTHROPIC_BASE_URL
      default:
        return ''
    }
  }
  // responses：Kimi / DeepSeek / MiniMax 的 base 与 chat_completions 相同（端点路径差异由后端处理）。
  switch (platform) {
    case 'kimi':
      return mode === 'coding' ? 'https://api.kimi.com/coding/v1' : 'https://api.moonshot.cn/v1'
    case 'zhipu':
      return mode === 'coding'
        ? 'https://open.bigmodel.cn/api/coding/paas/v4'
        : 'https://open.bigmodel.cn/api/paas/v4'
    case 'deepseek':
      return 'https://api.deepseek.com'
    case 'minimax':
      return 'https://api.minimaxi.com/v1'
    case 'opencode_go':
      return mode === 'zen' ? OPENCODE_ZEN_BASE_URL : OPENCODE_GO_BASE_URL
    default:
      return ''
  }
  if (platform === 'kimi') return mode === 'coding' ? 'https://api.kimi.com/coding/v1' : 'https://api.moonshot.cn/v1'
  if (platform === 'zhipu') return mode === 'coding'
    ? 'https://open.bigmodel.cn/api/coding/paas/v4'
    : 'https://open.bigmodel.cn/api/paas/v4'
  if (platform === 'deepseek') return 'https://api.deepseek.com'
  return ''
}

export function defaultCNAdaptiveBaseUrls(
  platform: CnProviderPlatform | 'opencode_go',
  mode: CnAccountMode | OpenCodeAccountMode
): Record<CnNativeApiProtocol, string> {
  return {
    chat_completions: defaultCNBaseUrl(platform, mode, 'chat_completions'),
    anthropic: defaultCNBaseUrl(platform, mode, 'anthropic'),
    responses: cnSupportsNativeResponses(platform) ? defaultCNBaseUrl(platform, mode, 'responses') : ''
  }
}

export function cnQuotaCellVisible(platform: string, accountMode: string): boolean {
  if (platform === 'opencode_go') return accountMode !== 'zen'
  return (platform === 'kimi' || platform === 'zhipu' || platform === 'minimax') && accountMode === 'coding'
}

export function cnBalanceCellVisible(platform: string, accountMode: string): boolean {
  return (platform === 'kimi' || platform === 'deepseek') && accountMode !== 'coding'
}

export interface PlanTypeOption {
  value: string
  label: string
  [key: string]: unknown
}

export function planTypeDisplayLabel(value: string): string {
  switch (value.trim().toLowerCase()) {
    case 'plus':
      return 'Plus'
    case 'pro':
    case 'chatgptpro':
      return 'Pro'
    case 'free':
      return 'Free'
    case 'team':
      return 'Team'
    default:
      return value
  }
}

export function readPlanType(credentials: Record<string, unknown> | undefined | null): string {
  const value = credentials?.plan_type
  return typeof value === 'string' ? value : ''
}

export function buildPlanTypeOptions(current: string, clearLabel: string): PlanTypeOption[] {
  const value = (current || '').trim()
  const currentLabel = value ? planTypeDisplayLabel(value) : ''
  const presets: PlanTypeOption[] = [
    { value: 'plus', label: 'Plus' },
    { value: 'pro', label: 'Pro' },
    { value: 'free', label: 'Free' }
  ]
  const options: PlanTypeOption[] = [{ value: '', label: clearLabel }]
  for (const preset of presets) {
    if (value && preset.value !== value.toLowerCase() && preset.label === currentLabel) {
      options.push({ value, label: preset.label })
    } else {
      options.push(preset)
    }
  }
  if (value && !options.some((option) => option.value.toLowerCase() === value.toLowerCase())) {
    options.push({ value, label: planTypeDisplayLabel(value) })
  }
  return options
}

export function applyPlanType(
  credentials: Record<string, unknown>,
  planType: string
): Record<string, unknown> {
  const value = (planType || '').trim()
  if (value) {
    credentials.plan_type = value
  } else {
    delete credentials.plan_type
  }
  return credentials
}
