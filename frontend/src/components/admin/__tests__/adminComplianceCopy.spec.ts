import { describe, expect, it } from 'vitest'
import { resolveAdminComplianceCopy } from '../adminComplianceCopy'

describe('resolveAdminComplianceCopy', () => {
  it('uses the translated text when the locale key exists', () => {
    expect(resolveAdminComplianceCopy('zh', 'title', '已翻译标题')).toBe('已翻译标题')
  })

  it('never exposes a missing locale key to Chinese users', () => {
    expect(resolveAdminComplianceCopy('zh-CN', 'title', 'adminCompliance.title')).toBe(
      '管理员合规确认',
    )
  })

  it('falls back to English for non-Chinese locales', () => {
    expect(resolveAdminComplianceCopy('en-US', 'accept')).toBe('Acknowledge and continue')
  })
})
