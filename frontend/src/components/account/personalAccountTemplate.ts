import { getModelsByPlatform, getPresetMappingsByPlatform } from '@/composables/useModelWhitelist'
import { OPENAI_WS_MODE_OFF } from '@/utils/openaiWsMode'
import type { AccountPlatform, OpenAICompactMode } from '@/types'

export const PERSONAL_ACCOUNT_DEFAULT_CONCURRENCY = 3
export const PERSONAL_ACCOUNT_DEFAULT_PRIORITY = 1
export const PERSONAL_ACCOUNT_DEFAULT_AUTO_PAUSE_ON_EXPIRED = true

export const PERSONAL_ACCOUNT_DEFAULT_OPENAI_COMPACT_MODE: OpenAICompactMode = 'force_on'
export const PERSONAL_ACCOUNT_DEFAULT_OPENAI_WS_MODE = OPENAI_WS_MODE_OFF

export function buildPersonalAccountModelMapping(platform: AccountPlatform | string): Record<string, string> {
  const mapping: Record<string, string> = {}
  if (platform === 'kiro') {
    for (const { from, to } of getPresetMappingsByPlatform('kiro')) {
      if (from && to) {
        mapping[from] = to
      }
    }
    return mapping
  }
  for (const model of getModelsByPlatform(platform)) {
    if (!model.includes('*')) {
      mapping[model] = model
    }
  }
  return mapping
}

export function applyPersonalAccountTemplate(
  platform: AccountPlatform | string,
  credentials: Record<string, unknown>,
  extra?: Record<string, unknown>
): { credentials: Record<string, unknown>; extra?: Record<string, unknown> } {
  const nextCredentials: Record<string, unknown> = {
    ...credentials,
    model_mapping: buildPersonalAccountModelMapping(platform)
  }

  const nextExtra: Record<string, unknown> = { ...(extra || {}) }
  if (platform === 'openai') {
    nextExtra.openai_oauth_responses_websockets_v2_mode = PERSONAL_ACCOUNT_DEFAULT_OPENAI_WS_MODE
    nextExtra.openai_oauth_responses_websockets_v2_enabled = false
    nextExtra.openai_passthrough = false
    nextExtra.openai_oauth_passthrough = false
    nextExtra.codex_cli_only = false
    nextExtra.openai_compact_mode = PERSONAL_ACCOUNT_DEFAULT_OPENAI_COMPACT_MODE
    delete nextCredentials.compact_model_mapping
  }
  if (platform === 'kiro') {
    nextExtra.openai_responses_supported = false
  }

  return {
    credentials: nextCredentials,
    extra: Object.keys(nextExtra).length > 0 ? nextExtra : undefined
  }
}
