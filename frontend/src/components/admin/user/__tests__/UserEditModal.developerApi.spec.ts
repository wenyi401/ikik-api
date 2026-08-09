import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { update, showError, showSuccess } = vi.hoisted(() => ({
  update: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { update },
    userAttributes: { updateUserAttributeValues: vi.fn() }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

import UserEditModal from '@/components/admin/user/UserEditModal.vue'

describe('UserEditModal developer API gate', () => {
  beforeEach(() => {
    update.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    update.mockResolvedValue({})
  })

  it('submits the explicit per-user developer API setting', async () => {
    const wrapper = mount(UserEditModal, {
      props: {
        show: true,
        user: {
          id: 5,
          email: 'user@example.com',
          username: 'user',
          notes: '',
          role: 'user',
          balance: 0,
          concurrency: 1,
          status: 'active',
          developer_api_enabled: false,
          allowed_groups: null,
          balance_notify_enabled: false,
          balance_notify_threshold: null,
          balance_notify_extra_emails: [],
          created_at: '2026-07-31T00:00:00Z',
          updated_at: '2026-07-31T00:00:00Z'
        }
      },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          UserAttributeForm: true,
          Icon: true
        }
      }
    })

    await wrapper.get('[role="switch"]').trigger('click')
    await wrapper.get('#edit-user-form').trigger('submit')
    await flushPromises()

    expect(update).toHaveBeenCalledWith(5, expect.objectContaining({
      developer_api_enabled: true
    }))
  })
})
