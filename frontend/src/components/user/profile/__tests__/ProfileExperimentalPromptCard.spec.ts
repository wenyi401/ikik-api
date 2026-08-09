import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileExperimentalPromptCard from '../ProfileExperimentalPromptCard.vue'

const {
  getStatus,
  purchaseFeature,
  redeem,
  refreshUser,
  showSuccess,
  showError,
  authUser,
} = vi.hoisted(() => ({
  getStatus: vi.fn(),
  purchaseFeature: vi.fn(),
  redeem: vi.fn(),
  refreshUser: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  authUser: { balance: 0 },
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getOpenAIExperimentalPromptStatus: getStatus,
    purchaseOpenAIExperimentalPrompt: purchaseFeature,
    redeem,
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: authUser, refreshUser }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorCode: (error: { reason?: string }) => error?.reason,
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, values?: Record<string, string | number>) =>
        values?.amount !== undefined ? `${key}:${values.amount}` : key,
    }),
  }
})

const lockedStatus = {
  feature_key: 'openai_experimental_prompt',
  unlocked: false,
  configured: true,
  price_cents: 660,
}

describe('ProfileExperimentalPromptCard', () => {
  beforeEach(() => {
    getStatus.mockReset()
    purchaseFeature.mockReset()
    redeem.mockReset()
    refreshUser.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    authUser.balance = 0

    getStatus.mockResolvedValue({ ...lockedStatus })
    refreshUser.mockResolvedValue(undefined)
  })

  it('purchases the entitlement and refreshes the user balance', async () => {
    purchaseFeature.mockResolvedValue({ ...lockedStatus, unlocked: true })
    const wrapper = mount(ProfileExperimentalPromptCard, {
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('profile.experimentalPrompt.priceValue:6.60')
    await wrapper.get('[data-testid="experimental-prompt-purchase"]').trigger('click')
    await flushPromises()

    expect(purchaseFeature).toHaveBeenCalledOnce()
    expect(refreshUser).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('profile.experimentalPrompt.unlockedHint')
    expect(showSuccess).toHaveBeenCalledWith('profile.experimentalPrompt.purchaseSuccess')
  })

  it('redeems a feature code and refreshes the entitlement', async () => {
    redeem.mockResolvedValue({ type: 'feature', value: 0 })
    getStatus
      .mockResolvedValueOnce({ ...lockedStatus })
      .mockResolvedValueOnce({ ...lockedStatus, unlocked: true })

    const wrapper = mount(ProfileExperimentalPromptCard, {
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    await wrapper.get('[data-testid="experimental-prompt-redeem-input"]').setValue('FEATURE-CODE')
    await wrapper.get('[data-testid="experimental-prompt-redeem"]').trigger('click')
    await flushPromises()

    expect(redeem).toHaveBeenCalledWith('FEATURE-CODE')
    expect(getStatus).toHaveBeenCalledTimes(2)
    expect(refreshUser).toHaveBeenCalledOnce()
    expect(showSuccess).toHaveBeenCalledWith('profile.experimentalPrompt.redeemSuccess')
  })

  it('shows the localized balance shortfall without unlocking', async () => {
    authUser.balance = 2.35
    purchaseFeature.mockRejectedValue({ reason: 'INSUFFICIENT_BALANCE' })
    const wrapper = mount(ProfileExperimentalPromptCard, {
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    await wrapper.get('[data-testid="experimental-prompt-purchase"]').trigger('click')
    await flushPromises()

    expect(refreshUser).toHaveBeenCalledOnce()
    expect(showError).toHaveBeenCalledWith(
      'profile.experimentalPrompt.insufficientBalance:4.25',
    )
    expect(wrapper.text()).toContain('profile.experimentalPrompt.locked')
  })

  it('disables balance purchase while the instruction is not configured', async () => {
    getStatus.mockResolvedValue({ ...lockedStatus, configured: false })
    const wrapper = mount(ProfileExperimentalPromptCard, {
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="experimental-prompt-purchase"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('profile.experimentalPrompt.purchaseUnavailable')
  })
})
