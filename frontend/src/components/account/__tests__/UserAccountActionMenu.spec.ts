import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UserAccountActionMenu from '../UserAccountActionMenu.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

function makeAccount(overrides: Partial<Account> = {}): Account {
  return {
    id: 1,
    name: 'owned-account',
    platform: 'openai',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-07-22T00:00:00Z',
    updated_at: '2026-07-22T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides
  }
}

describe('UserAccountActionMenu', () => {
  it('offers owner-scoped state recovery for a rate-limited account', async () => {
    const account = makeAccount({ rate_limit_reset_at: '2099-01-01T00:00:00Z' })
    const wrapper = mount(UserAccountActionMenu, {
      props: {
        show: true,
        account,
        position: { top: 10, left: 10 }
      },
      global: {
        stubs: {
          Teleport: true,
          Icon: true
        }
      }
    })

    const recoverButton = wrapper.findAll('button').find((button) =>
      button.text().includes('admin.accounts.recoverState')
    )
    expect(recoverButton).toBeDefined()

    await recoverButton!.trigger('click')

    expect(wrapper.emitted('recover-state')).toEqual([[account]])
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('does not show state recovery for a healthy account', () => {
    const wrapper = mount(UserAccountActionMenu, {
      props: {
        show: true,
        account: makeAccount(),
        position: { top: 10, left: 10 }
      },
      global: {
        stubs: {
          Teleport: true,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).not.toContain('admin.accounts.recoverState')
  })
})
