import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { list, create, revoke, showError, showSuccess, copyToClipboard } = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  revoke: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  copyToClipboard: vi.fn()
}))

vi.mock('@/api/developerTokens', () => ({
  DEVELOPER_TOKEN_SCOPES: ['accounts:read', 'accounts:write', 'accounts:share', 'bot:access'],
  developerTokensAPI: { list, create, revoke }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

vi.mock('@/composables/useClipboard', async () => {
  const { ref } = await import('vue')
  return {
    useClipboard: () => ({ copied: ref(false), copyToClipboard })
  }
})

vi.mock('@/utils/format', () => ({
  formatDateTimeToMinute: (value: string) => value
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

import ProfileDeveloperTokensCard from '@/components/user/profile/ProfileDeveloperTokensCard.vue'

const dialogStub = {
  props: ['show'],
  template: '<div v-if="show" class="dialog-stub"><slot /><slot name="footer" /></div>'
}

describe('ProfileDeveloperTokensCard', () => {
  beforeEach(() => {
    list.mockReset()
    create.mockReset()
    revoke.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    copyToClipboard.mockReset()
    list.mockResolvedValue([])
  })

  it('creates a scoped token and displays its plaintext once', async () => {
    const created = {
      developer_token: {
        id: 8,
        user_id: 3,
        name: 'account sync',
        token_prefix: 'ikd_created',
        scopes: ['accounts:read', 'accounts:write'],
        status: 'active',
        created_at: '2026-07-31T00:00:00Z',
        updated_at: '2026-07-31T00:00:00Z'
      },
      token: 'ikd_one_time_plaintext'
    }
    create.mockResolvedValue(created)

    const wrapper = mount(ProfileDeveloperTokensCard, {
      global: {
        stubs: {
          BaseDialog: dialogStub,
          ConfirmDialog: true,
          Icon: true
        }
      }
    })
    await flushPromises()

    await wrapper.get('button.btn-primary').trigger('click')
    await wrapper.get('#developer-token-name').setValue(' account sync ')
    await wrapper.get('#developer-token-create-form').trigger('submit')
    await flushPromises()

    expect(create).toHaveBeenCalledWith({
      name: 'account sync',
      scopes: ['accounts:read', 'accounts:write']
    })
    expect(wrapper.text()).toContain('ikd_one_time_plaintext')
    expect(wrapper.text()).toContain('account sync')

    const storedButton = wrapper.findAll('button').find(button => button.text() === 'profile.developerTokens.savedToken')
    expect(storedButton).toBeTruthy()
    await storedButton!.trigger('click')
    expect(wrapper.text()).not.toContain('ikd_one_time_plaintext')
  })
})
