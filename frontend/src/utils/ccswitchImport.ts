import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.5'
export const OPENAI_CC_SWITCH_REASONING_EFFORT = 'xhigh'

export type CcSwitchClientType = 'claude' | 'gemini'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
}

function encodeBase64Utf8(value: string): string {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary)
}

function toCodexProviderId(providerName: string): string {
  const normalized = providerName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')

  return normalized || 'pixel_api'
}

function tomlString(value: string): string {
  return JSON.stringify(value)
}

function buildOpenAICodexConfig(baseUrl: string, providerName: string): string {
  const providerId = toCodexProviderId(providerName)

  return `model_provider = ${tomlString(providerId)}
model = ${tomlString(OPENAI_CC_SWITCH_CODEX_MODEL)}
model_reasoning_effort = ${tomlString(OPENAI_CC_SWITCH_REASONING_EFFORT)}
disable_response_storage = true

[model_providers.${providerId}]
name = ${tomlString(providerId)}
base_url = ${tomlString(baseUrl)}
wire_api = "responses"
requires_openai_auth = true`
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string
): CcSwitchImportConfig {
  switch (platform || 'anthropic') {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : 'claude',
        endpoint: `${baseUrl}/antigravity`
      }
    case 'openai':
      return {
        app: 'codex',
        endpoint: baseUrl,
        model: OPENAI_CC_SWITCH_CODEX_MODEL
      }
    case 'gemini':
      return {
        app: 'gemini',
        endpoint: baseUrl
      }
    default:
      return {
        app: 'claude',
        endpoint: baseUrl
      }
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(input.platform, input.clientType, input.baseUrl)
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', input.baseUrl],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', encodeBase64Utf8(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  if (config.model) {
    entries.splice(2, 0, ['model', config.model])
  }

  if ((input.platform || 'anthropic') === 'openai') {
    entries.push([
      'config',
      encodeBase64Utf8(JSON.stringify({
        auth: {
          OPENAI_API_KEY: input.apiKey
        },
        config: buildOpenAICodexConfig(config.endpoint, input.providerName)
      }))
    ])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
