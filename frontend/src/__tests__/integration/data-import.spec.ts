import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'

const {
  importAdminData,
  importAdminCredentialContents,
  listAdminGroups
} = vi.hoisted(() => ({
  importAdminData: vi.fn(),
  importAdminCredentialContents: vi.fn(),
  listAdminGroups: vi.fn()
}))

const showError = vi.fn()
const showSuccess = vi.fn()
const showWarning = vi.fn()

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showWarning
  })
}))

vi.mock('@/api/accounts', () => ({
  accountsAPI: {
    importData: vi.fn(),
    importCredentialContents: vi.fn()
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  importData: importAdminData,
  importCredentialContents: importAdminCredentialContents
}))

vi.mock('@/api/admin/groups', () => ({
  list: listAdminGroups
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

describe('ImportDataModal', () => {
  beforeEach(() => {
    showError.mockReset()
    showSuccess.mockReset()
    showWarning.mockReset()
    importAdminData.mockReset()
    importAdminCredentialContents.mockReset()
    listAdminGroups.mockReset()
    listAdminGroups.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 1000,
      pages: 1
    } as any)
  })

  it('未选择文件时提示错误', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    await wrapper.find('form').trigger('submit')
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportSelectFile')
  })

  it('无效 JSON 时提示解析失败', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const file = new File(['invalid json'], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve('invalid json')
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportParseFailed')
  })

  it('选择导入目标分组后提交请求携带 group_ids 且不修改数据文件', async () => {
    listAdminGroups.mockResolvedValue({
      items: [
        { id: 1, name: 'OpenAI 1', platform: 'openai', status: 'active', rate_multiplier: 1, is_exclusive: false, subscription_type: 'standard', daily_limit_usd: null, weekly_limit_usd: null, monthly_limit_usd: null, image_price_1k: null, image_price_2k: null, image_price_4k: null, claude_code_only: false, fallback_group_id: null, fallback_group_id_on_invalid_request: null, require_oauth_only: false, require_privacy_set: false, created_at: '', updated_at: '' },
        { id: 2, name: 'OpenAI 2', platform: 'openai', status: 'active', rate_multiplier: 1, is_exclusive: false, subscription_type: 'standard', daily_limit_usd: null, weekly_limit_usd: null, monthly_limit_usd: null, image_price_1k: null, image_price_2k: null, image_price_4k: null, claude_code_only: false, fallback_group_id: null, fallback_group_id_on_invalid_request: null, require_oauth_only: false, require_privacy_set: false, created_at: '', updated_at: '' }
      ],
      total: 2,
      page: 1,
      page_size: 1000,
      pages: 1
    } as any)
    importAdminData.mockResolvedValue({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_failed: 0,
      errors: []
    })

    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })
    await flushPromises()

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[0].setValue(true)
    await checkboxes[1].setValue(true)

    const dataPayload = {
      type: 'ikik-api-data',
      version: 1,
      proxies: [],
      accounts: [
        {
          name: 'acc',
          platform: 'openai',
          type: 'oauth',
          credentials: { token: 'x' },
          concurrency: 3,
          priority: 50
        }
      ]
    }
    const input = wrapper.find('input[type="file"]')
    const file = new File([JSON.stringify(dataPayload)], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve(JSON.stringify(dataPayload))
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(importAdminData).toHaveBeenCalledWith({
      data: dataPayload,
      skip_default_group_bind: true,
      group_ids: [1, 2]
    })
    expect(dataPayload).not.toHaveProperty('group_ids')
  })

  it('目标分组跨平台时阻止提交', async () => {
    listAdminGroups.mockResolvedValue({
      items: [
        { id: 1, name: 'OpenAI', platform: 'openai', status: 'active', rate_multiplier: 1, is_exclusive: false, subscription_type: 'standard', daily_limit_usd: null, weekly_limit_usd: null, monthly_limit_usd: null, image_price_1k: null, image_price_2k: null, image_price_4k: null, claude_code_only: false, fallback_group_id: null, fallback_group_id_on_invalid_request: null, require_oauth_only: false, require_privacy_set: false, created_at: '', updated_at: '' },
        { id: 2, name: 'Claude', platform: 'anthropic', status: 'active', rate_multiplier: 1, is_exclusive: false, subscription_type: 'standard', daily_limit_usd: null, weekly_limit_usd: null, monthly_limit_usd: null, image_price_1k: null, image_price_2k: null, image_price_4k: null, claude_code_only: false, fallback_group_id: null, fallback_group_id_on_invalid_request: null, require_oauth_only: false, require_privacy_set: false, created_at: '', updated_at: '' }
      ],
      total: 2,
      page: 1,
      page_size: 1000,
      pages: 1
    } as any)

    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })
    await flushPromises()

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[0].setValue(true)
    await checkboxes[1].setValue(true)

    const input = wrapper.find('input[type="file"]')
    const file = new File([JSON.stringify({ type: 'ikik-api-data', version: 1, proxies: [], accounts: [] })], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve(JSON.stringify({ type: 'ikik-api-data', version: 1, proxies: [], accounts: [] }))
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(importAdminData).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportTargetGroupMixedPlatforms')
  })
})
