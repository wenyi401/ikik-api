import { beforeEach, describe, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import ImportAccountsModal from '../ImportAccountsModal.vue'
import CredentialImportModal from '@/components/account/CredentialImportModal.vue'
import type { Proxy } from '@/types'

const { importCredentialContents } = vi.hoisted(() => ({
  importCredentialContents: vi.fn()
}))

vi.mock('@/api', () => ({
  accountsAPI: {
    importCredentialContents
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('ImportAccountsModal', () => {
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

  beforeEach(() => {
    importCredentialContents.mockReset()
    importCredentialContents.mockResolvedValue({
      total: 1,
      created: 1,
      failed: 0,
      errors: []
    })
  })

  it('enables Claude Web import and forwards its mode to the owner API', async () => {
    const wrapper = shallowMount(ImportAccountsModal, {
      props: { show: true, proxies }
    })
    const modal = wrapper.findComponent(CredentialImportModal)

    expect(modal.props('allowClaudeWebImport')).toBe(true)
    expect(modal.props('allowProxy')).toBe(true)
    expect(modal.props('proxies')).toEqual(proxies)
    expect(modal.props('proxyScope')).toBe('user')
    await modal.props('importer')(['session-key'], {
      claudeWebImport: true,
      claudeWebAuthMode: 'full_cookie',
      proxyId: 17
    })

    expect(importCredentialContents).toHaveBeenCalledWith(expect.objectContaining({
      contents: ['session-key'],
      claude_web_import: true,
      claude_web_auth_mode: 'full_cookie',
      share_mode: 'private',
      proxy_id: 17,
      group_ids: []
    }))
  })
})
