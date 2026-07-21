import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import WithdrawalsView from '@/views/user/WithdrawalsView.vue'

describe('WithdrawalsView', () => {
  it('renders withdrawal management as a standalone personal-center page', () => {
    const wrapper = mount(WithdrawalsView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          UiPage: { template: '<section data-testid="withdrawals-page"><slot /></section>' },
          ProfileWithdrawalCard: { template: '<div data-testid="profile-withdrawal-card" />' },
        },
      },
    })

    expect(wrapper.get('[data-testid="withdrawals-page"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-withdrawal-card"]').exists()).toBe(true)
  })
})
