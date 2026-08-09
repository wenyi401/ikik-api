import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import type { User } from '@/types'

vi.mock('vue-router', () => ({
  useRoute: () => ({
    fullPath: '/profile'
  })
}))

const mocks = vi.hoisted(() => ({
  updateProfile: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/user', () => ({
  userAPI: {
    updateProfile: mocks.updateProfile
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: null })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: mocks.showError,
    showSuccess: mocks.showSuccess
  })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'profile.accountBalance') return 'Account Balance'
        if (key === 'profile.concurrencyLimit') return 'Concurrency Limit'
        if (key === 'profile.memberSince') return 'Member Since'
        if (key === 'profile.administrator') return 'Administrator'
        if (key === 'profile.user') return 'User'
        if (key === 'profile.share.action') return 'Share'
        if (key === 'profile.authBindings.providers.email') return 'Email'
        if (key === 'profile.authBindings.providers.linuxdo') return 'LinuxDo'
        if (key === 'profile.authBindings.providers.wechat') return 'WeChat'
        if (key === 'profile.authBindings.providers.oidc') return params?.providerName || 'OIDC'
        if (key === 'profile.authBindings.source.avatar') {
          return `Avatar synced from ${params?.providerName || 'provider'}`
        }
        if (key === 'profile.authBindings.source.username') {
          return `Username synced from ${params?.providerName || 'provider'}`
        }
        return key
      }
    })
  }
})

function createUser(overrides: Partial<User> = {}): User {
  return {
    id: 5,
    username: 'alice',
    email: 'alice@example.com',
    avatar_url: null,
    role: 'user',
    balance: 10,
    concurrency: 2,
    status: 'active',
    allowed_groups: null,
    balance_notify_enabled: true,
    balance_notify_threshold: null,
    balance_notify_extra_emails: [],
    created_at: '2026-04-20T00:00:00Z',
    updated_at: '2026-04-20T00:00:00Z',
    ...overrides
  }
}

describe('ProfileInfoCard', () => {
  beforeEach(() => {
    mocks.updateProfile.mockReset()
    mocks.showError.mockReset()
    mocks.showSuccess.mockReset()
  })

  it('renders basic account information inside the new overview shell', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser()
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true
        }
      }
    })

    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).toContain('alice')
    expect(wrapper.text()).toContain('User')
    expect(wrapper.get('[data-testid="profile-share-action"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-edit-action"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-auth-bindings-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="profile-main-column"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="profile-content-grid"]').classes()).toContain('grid-cols-1')
  })

  it('uses the two-column desktop layout only when the main column has content', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser(),
        hasMainContent: true
      },
      slots: {
        'main-after': '<div data-testid="main-content">Main content</div>'
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true
        }
      }
    })

    expect(wrapper.get('[data-testid="profile-main-column"]').text()).toContain('Main content')
    expect(wrapper.get('[data-testid="profile-content-grid"]').classes()).toContain('xl:grid-cols-[minmax(0,1.65fr)_minmax(0,0.85fr)]')
  })

  it('omits the email address from the image sharing preview', async () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser()
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true,
          Teleport: { template: '<div><slot /></div>' }
        }
      }
    })

    await wrapper.get('[data-testid="profile-share-action"]').trigger('click')

    const preview = wrapper.get('[data-testid="profile-share-preview"]')
    expect(preview.text()).toContain('alice')
    expect(preview.text()).not.toContain('alice@example.com')
  })

  it('shows the saved custom export text and color without exposing the email', async () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          share_card_text: '保持好奇，持续创造',
          share_card_text_color: '#be123c'
        })
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true,
          Teleport: { template: '<div><slot /></div>' }
        }
      }
    })

    await wrapper.get('[data-testid="profile-share-action"]').trigger('click')

    const preview = wrapper.get('[data-testid="profile-share-preview"]')
    const customText = wrapper.get('[data-testid="profile-share-custom-text"]')
    expect(customText.text()).toBe('保持好奇，持续创造')
    expect(customText.attributes('style')).toContain('color: rgb(190, 18, 60)')
    expect(preview.text()).not.toContain('alice@example.com')
    expect(wrapper.get('[data-testid="profile-share-heatmap-grid"]').findAll('span')).toHaveLength(371)
  })

  it('saves custom export text and color to the current user profile', async () => {
    const updatedUser = createUser({
      share_card_text: '今天也在认真创造',
      share_card_text_color: '#0b6bcb'
    })
    mocks.updateProfile.mockResolvedValue(updatedUser)

    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser()
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true,
          Teleport: { template: '<div><slot /></div>' }
        }
      }
    })

    await wrapper.get('[data-testid="profile-share-action"]').trigger('click')
    await wrapper.get('[data-testid="profile-share-text-input"]').setValue('  今天也在认真创造  ')
    await wrapper.get('[data-testid="profile-share-color-input"]').setValue('#0b6bcb')
    await wrapper.get('[data-testid="profile-share-settings-save"]').trigger('click')
    await flushPromises()

    expect(mocks.updateProfile).toHaveBeenCalledWith({
      share_card_text: '今天也在认真创造',
      share_card_text_color: '#0b6bcb'
    })
    expect(mocks.showSuccess).toHaveBeenCalledWith('profile.share.settingsSaved')
  })

  it('keeps an unsaved export draft when the session refreshes the user', async () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          share_card_text: '原来的文案',
          share_card_text_color: '#08775c'
        })
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true,
          Teleport: { template: '<div><slot /></div>' }
        }
      }
    })

    await wrapper.get('[data-testid="profile-share-action"]').trigger('click')
    await wrapper.get('[data-testid="profile-share-text-input"]').setValue('尚未保存的新文案')
    await wrapper.setProps({
      user: createUser({
        share_card_text: '原来的文案',
        share_card_text_color: '#08775c',
        updated_at: '2026-08-02T00:00:00Z'
      })
    })

    expect(wrapper.get('[data-testid="profile-share-text-input"]').element).toHaveProperty(
      'value',
      '尚未保存的新文案'
    )
    expect(wrapper.get('[data-testid="profile-share-custom-text"]').text()).toBe('尚未保存的新文案')
  })

  it('renders third-party source hints from profile sources', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          avatar_url: 'https://cdn.example.com/linuxdo.png',
          profile_sources: {
            avatar: { provider: 'linuxdo', source: 'linuxdo' },
            username: { provider: 'linuxdo', source: 'linuxdo' }
          }
        })
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true
        }
      }
    })

    expect(wrapper.text()).toContain('Avatar synced from LinuxDo')
    expect(wrapper.text()).toContain('Username synced from LinuxDo')
  })

  it('uses the configured OIDC provider name in source hints', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          profile_sources: {
            username: { provider: 'oidc', source: 'oidc' }
          }
        }),
        oidcProviderName: 'ExampleID'
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true
        }
      }
    })

    expect(wrapper.text()).toContain('Username synced from ExampleID')
  })

  it('does not display synthetic oauth-only emails as a real bound email', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          email: 'legacy-user@oidc-connect.invalid',
          email_bound: false,
          auth_bindings: {
            email: { bound: false }
          }
        })
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true
        }
      }
    })

    expect(wrapper.text()).not.toContain('legacy-user@oidc-connect.invalid')
  })

  it('does not display synthetic oauth-only emails when only legacy identity bindings mark email as unbound', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          email: 'legacy-user@wechat-connect.invalid',
          identity_bindings: {
            email: { bound: false }
          }
        })
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true
        }
      }
    })

    expect(wrapper.text()).not.toContain('legacy-user@wechat-connect.invalid')
  })

  it('renders the approved overview hero and two-column content shell', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser(),
        hasMainContent: true
      },
      global: {
        stubs: {
          Icon: true,
          ProfileTokenActivityHeatmap: true
        }
      }
    })

    expect(wrapper.get('[data-testid="profile-overview-hero"]').text()).toContain('alice@example.com')
    expect(wrapper.get('[data-testid="profile-overview-metric-balance"]').text()).toContain('Account Balance')
    expect(wrapper.get('[data-testid="profile-overview-metric-concurrency"]').text()).toContain('Concurrency Limit')
    expect(wrapper.get('[data-testid="profile-overview-metric-member-since"]').text()).toContain('Member Since')
    expect(wrapper.find('[data-testid="profile-info-summary-grid"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'ProfileTokenActivityHeatmap' }).exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-main-column"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-side-column"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-auth-bindings-panel"]').exists()).toBe(true)
  })
})
