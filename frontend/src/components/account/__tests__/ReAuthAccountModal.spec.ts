import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { adminGenerateKiroAuthUrlMock, userGenerateKiroAuthUrlMock } = vi.hoisted(() => ({
  adminGenerateKiroAuthUrlMock: vi.fn(),
  userGenerateKiroAuthUrlMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {},
    kiro: {
      generateAuthUrl: adminGenerateKiroAuthUrlMock
    }
  }
}))

vi.mock('@/api/accounts', () => ({
  accountsAPI: {
    generateKiroOAuthUrl: userGenerateKiroAuthUrlMock
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

import ReAuthAccountModal from '../ReAuthAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const OAuthAuthorizationFlowStub = defineComponent({
  name: 'OAuthAuthorizationFlow',
  emits: ['generate-url'],
  template: '<button data-testid="generate-url" @click="$emit(\'generate-url\')">generate</button>'
})

function buildKiroAccount() {
  return {
    id: 17,
    name: 'Kiro account',
    platform: 'kiro',
    type: 'oauth',
    credentials: {
      provider: 'Github'
    },
    proxy_id: 9,
    status: 'active'
  } as any
}

describe('ReAuthAccountModal owner scope', () => {
  beforeEach(() => {
    adminGenerateKiroAuthUrlMock.mockReset()
    userGenerateKiroAuthUrlMock.mockReset().mockResolvedValue({
      auth_url: 'https://example.com/authorize',
      session_id: 'session-id',
      state: 'state'
    })
  })

  it('uses the user Kiro OAuth endpoint when reauthorizing an owned account', async () => {
    const wrapper = mount(ReAuthAccountModal, {
      props: {
        show: true,
        account: buildKiroAccount(),
        accountScope: 'user'
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
          Icon: true
        }
      }
    })

    await wrapper.get('[data-testid="generate-url"]').trigger('click')
    await flushPromises()

    expect(userGenerateKiroAuthUrlMock).toHaveBeenCalledWith({
      proxy_id: 9,
      provider: 'Github'
    })
    expect(adminGenerateKiroAuthUrlMock).not.toHaveBeenCalled()
  })
})
