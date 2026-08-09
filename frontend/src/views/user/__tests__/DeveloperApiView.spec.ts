import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import DeveloperApiView from '@/views/user/DeveloperApiView.vue'

const { authState, refreshUser } = vi.hoisted(() => ({
  authState: {
    user: null as Record<string, unknown> | null
  },
  refreshUser: vi.fn()
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get user() {
      return authState.user
    },
    refreshUser
  })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const global = {
  stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    UiPage: { template: '<main><slot /></main>' },
    ProfileDeveloperTokensCard: { template: '<section data-testid="developer-tokens-card" />' },
    Icon: true
  }
}

describe('DeveloperApiView', () => {
  beforeEach(() => {
    authState.user = { id: 1, developer_api_enabled: false }
    refreshUser.mockReset()
    refreshUser.mockResolvedValue(undefined)
  })

  it('keeps the page available and explains when access has not been granted', async () => {
    const wrapper = mount(DeveloperApiView, { global })
    await flushPromises()

    expect(refreshUser).toHaveBeenCalledOnce()
    expect(wrapper.get('[data-testid="developer-api-access-disabled"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="developer-tokens-card"]').exists()).toBe(false)
  })

  it('renders token management for an authorized user', async () => {
    authState.user = { id: 1, developer_api_enabled: true }
    const wrapper = mount(DeveloperApiView, { global })
    await flushPromises()

    expect(wrapper.get('[data-testid="developer-tokens-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="developer-api-access-disabled"]').exists()).toBe(false)
  })
})
