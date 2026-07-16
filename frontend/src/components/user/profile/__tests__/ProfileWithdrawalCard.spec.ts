import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileWithdrawalCard from '@/components/user/profile/ProfileWithdrawalCard.vue'

const {
  getReceiptCodeMock,
  listWithdrawalsMock,
  authState,
  showErrorMock,
  showSuccessMock
} = vi.hoisted(() => ({
  getReceiptCodeMock: vi.fn(),
  listWithdrawalsMock: vi.fn(),
  authState: {
    user: { balance: 120.5, share_income_balance: 59.07 },
    refreshUser: vi.fn()
  },
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn()
}))

vi.mock('@/api', () => ({
  userAPI: {
    getReceiptCode: getReceiptCodeMock,
    listWithdrawals: listWithdrawalsMock,
    uploadReceiptCode: vi.fn(),
    deleteReceiptCode: vi.fn(),
    submitWithdrawal: vi.fn(),
    cancelWithdrawal: vi.fn()
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock
  })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('ProfileWithdrawalCard', () => {
  beforeEach(() => {
    getReceiptCodeMock.mockReset()
    listWithdrawalsMock.mockReset()
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
    authState.user = { balance: 120.5, share_income_balance: 59.07 }
    authState.refreshUser.mockReset()

    getReceiptCodeMock.mockResolvedValue(null)
    listWithdrawalsMock.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 5, pages: 1 })
  })

  it('keeps the withdrawal card mounted when amount is typed', async () => {
    const wrapper = mount(ProfileWithdrawalCard, {
      global: {
        stubs: {
          Icon: true
        }
      }
    })
    await flushPromises()

    const amountInput = wrapper.get('input[inputmode="decimal"]')
    await amountInput.setValue('1.23')

    expect(wrapper.text()).toContain('余额提现与收款码')
    expect(wrapper.text()).toContain('$1.23')
    expect(showErrorMock).not.toHaveBeenCalled()
  })
})
