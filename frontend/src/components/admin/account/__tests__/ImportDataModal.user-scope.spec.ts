import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ImportDataModal from '../ImportDataModal.vue'

const { importUserData, importUserCredentials, getAvailable, importAdminData, listAdminGroups } = vi.hoisted(() => ({
  importUserData: vi.fn(),
  importUserCredentials: vi.fn(),
  getAvailable: vi.fn(),
  importAdminData: vi.fn(),
  listAdminGroups: vi.fn()
}))

vi.mock('@/api/accounts', () => ({
  accountsAPI: {
    importData: importUserData,
    importCredentialContents: importUserCredentials
  },
  userGroupsAPI: {
    getAvailable
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  importData: importAdminData,
  importCredentialContents: vi.fn()
}))

vi.mock('@/api/admin/groups', () => ({
  list: listAdminGroups
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showWarning: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('ImportDataModal user scope', () => {
  beforeEach(() => {
    importUserData.mockReset()
    importUserCredentials.mockReset()
    getAvailable.mockReset()
    importAdminData.mockReset()
    listAdminGroups.mockReset()
    getAvailable.mockResolvedValue([])
    importUserData.mockResolvedValue({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_failed: 0,
      errors: []
    })
    importUserCredentials.mockResolvedValue({
      total: 1,
      created: 1,
      failed: 0,
      errors: []
    })
  })

  it('keeps local backup upload, hides remote URL and calls only the owner API', async () => {
    const wrapper = mount(ImportDataModal, {
      props: {
        show: true,
        accountScope: 'user'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          }
        }
      }
    })
    await flushPromises()

    expect(getAvailable).not.toHaveBeenCalled()
    expect(listAdminGroups).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('userAccounts.dataImportHint')
    expect(wrapper.text()).toContain('userAccounts.dataImportWarning')
    expect(wrapper.text()).not.toContain('admin.accounts.dataImportWarning')
    expect(wrapper.find('input[type="file"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('admin.accounts.dataImportURL')
    expect(wrapper.text()).toContain('admin.accounts.concurrency')
    expect(wrapper.text()).toContain('admin.accounts.priority')
    expect(wrapper.text()).not.toContain('admin.accounts.dataImportRateMultiplier')

    const payload = {
      type: 'sub2api-data',
      version: 1,
      exported_at: '2026-07-22T00:00:00Z',
      proxies: [],
      accounts: []
    }
    const envelope = { data: payload }
    const file = new File([JSON.stringify(envelope)], 'backup.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: vi.fn().mockResolvedValue(JSON.stringify(envelope))
    })
    const fileInput = wrapper.find('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      configurable: true,
      value: [file]
    })
    await fileInput.trigger('change')
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(importUserData).toHaveBeenCalledWith({ data: payload })
    expect(importAdminData).not.toHaveBeenCalled()
    expect(wrapper.emitted('imported')).toEqual([[{ close: true }]])
  })

  it('routes Codex credential JSON through the owner credential importer', async () => {
    const wrapper = mount(ImportDataModal, {
      props: {
        show: true,
        accountScope: 'user'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          }
        }
      }
    })
    await flushPromises()

    const codexPayload = {
      type: 'codex',
      refresh_token: 'test-refresh-token'
    }
    const contents = JSON.stringify(codexPayload)
    const file = new File([contents], 'codex.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: vi.fn().mockResolvedValue(contents)
    })
    const fileInput = wrapper.find('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      configurable: true,
      value: [file]
    })
    await fileInput.trigger('change')
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(importUserCredentials).toHaveBeenCalledWith({
      contents: [contents],
      share_mode: 'private',
      priority: 50,
      group_ids: [],
      auto_pause_on_expired: true
    })
    expect(importUserData).not.toHaveBeenCalled()
  })

  it('refreshes created accounts without closing when the backend reports partial success', async () => {
    importUserData.mockResolvedValueOnce({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_failed: 1,
      errors: [{ kind: 'account', name: 'bad-account', message: 'invalid credentials' }]
    })
    const wrapper = mount(ImportDataModal, {
      props: { show: true, accountScope: 'user' },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })
    const payload = {
      type: 'sub2api-data',
      version: 1,
      exported_at: '2026-07-22T00:00:00Z',
      proxies: [],
      accounts: []
    }
    const file = new File([JSON.stringify(payload)], 'partial.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: vi.fn().mockResolvedValue(JSON.stringify(payload))
    })
    const fileInput = wrapper.find('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', { configurable: true, value: [file] })

    await fileInput.trigger('change')
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.emitted('imported')).toEqual([[{ close: false }]])
    expect(wrapper.text()).toContain('admin.accounts.dataImportResult')
    expect(wrapper.text()).toContain('bad-account')
  })

  it('keeps earlier import results and refreshes them when a later file fails', async () => {
    importUserData
      .mockResolvedValueOnce({
        proxy_created: 0,
        proxy_reused: 0,
        proxy_failed: 0,
        account_created: 1,
        account_failed: 0,
        errors: []
      })
      .mockRejectedValueOnce(new Error('second file failed'))
    const wrapper = mount(ImportDataModal, {
      props: { show: true, accountScope: 'user' },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })
    const payload = {
      type: 'sub2api-data',
      version: 1,
      exported_at: '2026-07-22T00:00:00Z',
      proxies: [],
      accounts: []
    }
    const contents = JSON.stringify(payload)
    const files = ['first.json', 'second.json'].map((name) => {
      const file = new File([contents], name, { type: 'application/json' })
      Object.defineProperty(file, 'text', { value: vi.fn().mockResolvedValue(contents) })
      return file
    })
    const fileInput = wrapper.find('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', { configurable: true, value: files })

    await fileInput.trigger('change')
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(importUserData).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('imported')).toEqual([[{ close: false }]])
    expect(wrapper.text()).toContain('admin.accounts.dataImportResult')
  })
})
