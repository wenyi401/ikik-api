import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountStatusIndicator from '../AccountStatusIndicator.vue'
import type { Account, Proxy } from '@/types'
import zh from '@/i18n/locales/zh'
import en from '@/i18n/locales/en'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/utils/format', async () => {
  const actual = await vi.importActual<typeof import('@/utils/format')>('@/utils/format')
  return {
    ...actual,
    formatCountdown: () => '10m',
    formatCountdownWithSuffix: () => '10m'
  }
})

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'account',
    platform: 'antigravity',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-03-15T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides,
  }
}

function makeProxy(overrides: Partial<Proxy> = {}): Proxy {
  return {
    id: 9,
    name: 'private-proxy',
    protocol: 'http',
    host: '127.0.0.1',
    port: 8080,
    username: null,
    status: 'active',
    expires_at: null,
    created_at: '2026-03-15T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
    ...overrides,
  }
}

describe('AccountStatusIndicator', () => {
  it('defines admin disabled status translations', () => {
    expect((zh as any).admin.accounts.status.disabled).toBeTruthy()
    expect((zh as any).admin.accounts.status.disabled).not.toBe('admin.accounts.status.disabled')
    expect((en as any).admin.accounts.status.disabled).toBe('Disabled')
  })

  it('shows an expired bound proxy instead of an active account status', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          proxy_id: 9,
          proxy: makeProxy({ status: 'expired' })
        })
      },
      global: { stubs: { Icon: true } }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.proxyExpired')
    expect(wrapper.text()).not.toContain('admin.accounts.status.active')
  })

  it('shows an unavailable status when a bound proxy is inactive', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          proxy_id: 9,
          proxy: makeProxy({ status: 'inactive' })
        })
      },
      global: { stubs: { Icon: true } }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.proxyUnavailable')
  })

  it('switches to proxy expired when its deadline passes', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-17T00:00:00Z'))
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          proxy_id: 9,
          proxy: makeProxy({ expires_at: '2026-03-17T00:00:10Z' })
        })
      },
      global: { stubs: { Icon: true } }
    })

    try {
      expect(wrapper.text()).toContain('admin.accounts.status.active')
      await vi.advanceTimersByTimeAsync(10_001)
      expect(wrapper.text()).toContain('admin.accounts.status.proxyExpired')
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })

  it('renders disabled account status through the admin status key', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          status: 'disabled'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.disabled')
  })

  it('模型限流 + overages 启用 + 无 AICredits key → 显示 ⚡ (credits_active)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 1,
          name: 'ag-1',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('⚡')
    expect(wrapper.text()).toContain('CSon45')
  })

  it('模型限流 + overages 未启用 → 普通限流样式（无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 2,
          name: 'ag-2',
          extra: {
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
  })

  it('AICredits key 生效 → 显示积分已用尽 (credits_exhausted)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 3,
          name: 'ag-3',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })

  it('模型限流 + overages 启用 + AICredits key 生效 → 普通限流样式（积分耗尽，无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 4,
          name: 'ag-4',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              },
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    // 模型限流 + 积分耗尽 → 不应显示 ⚡
    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
    // AICredits 积分耗尽状态应显示
    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })

  it('到达最近重置时间后自动移除账号和模型限流状态', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-17T00:00:00Z'))
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          rate_limit_reset_at: '2026-03-17T00:00:10Z',
          extra: {
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-16T23:59:00Z',
                rate_limit_reset_at: '2026-03-17T00:00:10Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    try {
      expect(wrapper.text()).toContain('admin.accounts.status.rateLimited')
      expect(wrapper.text()).toContain('CSon45')

      await vi.advanceTimersByTimeAsync(10_001)

      expect(wrapper.text()).not.toContain('admin.accounts.status.rateLimited')
      expect(wrapper.text()).not.toContain('CSon45')
      expect(wrapper.text()).toContain('admin.accounts.status.active')
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })
  it('Claude 5 模型限流时显示 Opus 和 Sonnet 的短别名', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          extra: {
            model_rate_limits: {
              'claude-opus-5': {
                rate_limited_at: '2026-07-28T00:00:00Z',
                rate_limit_reset_at: '2099-07-28T00:00:00Z'
              },
              'claude-sonnet-5': {
                rate_limited_at: '2026-07-28T00:00:00Z',
                rate_limit_reset_at: '2099-07-28T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('COpus5')
    expect(wrapper.text()).toContain('CSon5')
    expect(wrapper.text()).not.toContain('claude-sonnet-5')
  })


  it('Grok 账号额度限流时显示自动恢复时间而非临时不可调度', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 5,
          name: 'grok-free-1',
          platform: 'grok',
          rate_limited_at: '2026-07-11T12:00:00Z',
          rate_limit_reset_at: '2099-07-11T13:00:00Z',
          temp_unschedulable_until: '2099-07-11T12:30:00Z',
          temp_unschedulable_reason: 'legacy grok rate limited'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.find('.badge-warning').text()).toBe('admin.accounts.status.rateLimited')
    expect(wrapper.text()).toContain('admin.accounts.status.rateLimitedAutoResume')
    expect(wrapper.text()).not.toContain('admin.accounts.status.tempUnschedulable')
  })

})

