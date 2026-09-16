import { describe, expect, it } from 'vitest'
import {
  COMPOSITE_ROUTE_TARGET_OPTIONS,
  CONCRETE_PLATFORM_OPTIONS,
  GROUP_PLATFORM_OPTIONS
} from '@/constants/platforms'

const concretePlatforms = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  'kimi',
  'zhipu',
  'deepseek',
  'minimax',
  'opencode_go'
]

describe('platform option catalogs', () => {
  it('exposes every concrete account platform', () => {
    expect(CONCRETE_PLATFORM_OPTIONS.map((option) => option.value)).toEqual([
      ...concretePlatforms,
      'kiro',
      'custom'
    ])
  })

  it('adds composite for group-backed filters', () => {
    expect(GROUP_PLATFORM_OPTIONS.map((option) => option.value)).toEqual([
      ...concretePlatforms,
      'kiro',
      'custom',
      'composite'
    ])
  })

  it('exposes exactly the platforms a composite group can route to', () => {
    // 与后端 isConcreteRequestPlatform 对齐：kiro / custom 不能作为复合路由目标。
    expect(COMPOSITE_ROUTE_TARGET_OPTIONS.map((option) => option.value)).toEqual(concretePlatforms)
    expect(COMPOSITE_ROUTE_TARGET_OPTIONS.find((option) => option.value === 'opencode_go')?.label).toBe(
      'OpenCode'
    )
  })
})
