import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CredentialImportModal from '../CredentialImportModal.vue'
import type { Proxy } from '@/types'

const { showError, showSuccess, showWarning } = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess, showWarning })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const BaseDialogStub = defineComponent({
  template: '<div><slot /><slot name="footer" /></div>'
})

const ProxySelectorStub = defineComponent({
  name: 'ProxySelector',
  props: {
    modelValue: { type: Number, default: null },
    proxies: { type: Array, default: () => [] },
    scope: { type: String, default: 'admin' }
  },
  emits: ['update:modelValue'],
  template: '<button type="button" data-test="proxy-selector" @click="$emit(\'update:modelValue\', 17)">proxy</button>'
})

describe('CredentialImportModal', () => {
  it('forwards the selected user proxy to each credential import call', async () => {
    const importer = vi.fn().mockResolvedValue({
      total: 1,
      created: 1,
      failed: 0,
      errors: []
    })
    const proxies: Proxy[] = [{
      id: 17,
      name: 'Private proxy',
      protocol: 'http',
      host: '127.0.0.1',
      port: 8080,
      username: null,
      status: 'active',
      created_at: '2026-07-22T00:00:00Z',
      updated_at: '2026-07-22T00:00:00Z'
    }]
    const wrapper = mount(CredentialImportModal, {
      props: {
        show: true,
        title: 'Import',
        hint: 'Hint',
        warning: 'Warning',
        allowProxy: true,
        proxies,
        proxyScope: 'user',
        importer
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ProxySelector: ProxySelectorStub,
          Icon: true
        }
      }
    })

    const proxySelector = wrapper.findComponent(ProxySelectorStub)
    expect(proxySelector.props('proxies')).toEqual(proxies)
    expect(proxySelector.props('scope')).toBe('user')
    await proxySelector.trigger('click')
    await wrapper.find('textarea').setValue('session-key')
    await wrapper.find('form').trigger('submit')

    await vi.waitFor(() => {
      expect(importer).toHaveBeenCalledWith(['session-key'], expect.objectContaining({
        proxyId: 17
      }))
    })
  })

  it('keeps the proxy selector hidden by default for existing admin callers', () => {
    const wrapper = mount(CredentialImportModal, {
      props: {
        show: true,
        title: 'Import',
        hint: 'Hint',
        warning: 'Warning',
        importer: vi.fn()
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ProxySelector: ProxySelectorStub,
          Icon: true
        }
      }
    })

    expect(wrapper.find('[data-test="proxy-selector"]').exists()).toBe(false)
  })
})
