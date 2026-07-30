import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminUser, Group } from '@/types'
import UserAllowedGroupsModal from '../UserAllowedGroupsModal.vue'

const { listGroups, updateUser, showSuccess } = vi.hoisted(() => ({
  listGroups: vi.fn(),
  updateUser: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: { list: listGroups },
    users: { update: updateUser },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

const groups: Group[] = [
  {
    id: 1,
    name: 'Public',
    platform: 'openai',
    rate_multiplier: 1,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'standard',
  } as Group,
  {
    id: 2,
    name: 'Exclusive',
    platform: 'openai',
    rate_multiplier: 1,
    is_exclusive: true,
    status: 'active',
    subscription_type: 'standard',
  } as Group,
]

const user = {
  id: 7,
  email: 'blocked@example.com',
  allowed_groups: [2],
  blocked_groups: [1, 99],
  group_rates: {},
} as AdminUser

describe('UserAllowedGroupsModal', () => {
  beforeEach(() => {
    listGroups.mockReset().mockResolvedValue({ items: groups })
    updateUser.mockReset().mockResolvedValue({})
    showSuccess.mockReset()
  })

  it('saves explicit group blocks and removes conflicting grants', async () => {
    const wrapper = mount(UserAllowedGroupsModal, {
      props: { show: false, user },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          PlatformIcon: true,
          Icon: true,
        },
      },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect((wrapper.get('[data-test="block-group-1"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('[data-test="block-group-1"]').setValue(false)
    await wrapper.get('[data-test="block-group-2"]').setValue(true)
    await wrapper.get('[data-test="save-group-config"]').trigger('click')
    await flushPromises()

    expect(updateUser).toHaveBeenCalledWith(7, expect.objectContaining({
      allowed_groups: [],
      blocked_groups: [99, 2],
    }))
  })
})
