import { describe, expect, it, vi } from 'vitest'
import {
  ACCOUNT_TUTORIAL_QUERY,
  accountTutorialStorageKey,
  createAccountTutorialSteps,
  shouldAutoStartAccountTutorial
} from '../accountTutorial'

describe('account tutorial', () => {
  it('only auto-starts once for beginner users', () => {
    expect(shouldAutoStartAccountTutorial('beginner', false)).toBe(true)
    expect(shouldAutoStartAccountTutorial('beginner', true)).toBe(false)
    expect(shouldAutoStartAccountTutorial('expert', false)).toBe(false)
    expect(shouldAutoStartAccountTutorial('unset', false)).toBe(false)
  })

  it('uses a versioned per-user storage key', () => {
    expect(accountTutorialStorageKey(42)).toBe('account_tutorial_42_v2')
    expect(ACCOUNT_TUTORIAL_QUERY).toBe('accounts')
  })

  it('covers the real account controls and conditionally includes carpool', () => {
    const t = vi.fn((key: string) => key)
    const steps = createAccountTutorialSteps(t, true)

    expect(steps.map((step) => step.element)).toEqual([
      '[data-guide="accounts-list"]',
      '[data-guide="accounts-create"]',
      '[data-guide="account-share-mode"]',
      '[data-guide="account-proxy-field"]',
      '[data-guide="accounts-tools"] [role="menu"]',
      '[data-guide="accounts-list"]',
      '[data-guide="accounts-carpool"]'
    ])
    expect(createAccountTutorialSteps(t, false)).toHaveLength(6)
  })
})
