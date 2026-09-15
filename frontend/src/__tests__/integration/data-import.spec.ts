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

const mountModal = () =>
  mount(ImportDataModal, {
    props: { show: true },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
      }
    }
  })

const makeJsonFile = (name: string, content: string, type = 'application/json') => {
  const file = new File([content], name, { type })
  Object.defineProperty(file, 'text', {
    value: () => Promise.resolve(content)
  })
  return file
}

const setInputFiles = (element: Element, files: File[]) => {
  Object.defineProperty(element, 'files', {
    value: files,
    configurable: true
  })
}

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
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    // 选择阶段即按文件名报错（上游契约）
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportParseFailedFile')
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
    await flushPromises()
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
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(importAdminData).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportTargetGroupMixedPlatforms')
  })
  it('invalid JSON is rejected per file name before importing', async () => {
    const wrapper = mountModal()
    const input = wrapper.find('input[type="file"]')

    setInputFiles(input.element, [makeJsonFile('data.json', 'invalid json')])

    await input.trigger('change')
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    // fork 在选择阶段就按文件名报错，并且不会发起导入
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportParseFailedFile')
    expect(importAdminData).not.toHaveBeenCalled()
  })

  it('a JSON that is not an export file is rejected by name', async () => {
    const wrapper = mountModal()
    const input = wrapper.find('input[type="file"]')

    setInputFiles(input.element, [makeJsonFile('random.json', JSON.stringify({ name: 'test' }))])

    await input.trigger('change')
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportInvalidFile')
    expect(importAdminData).not.toHaveBeenCalled()
  })

  it('a rejected selection keeps the previously chosen files', async () => {
    const wrapper = mountModal()
    const input = wrapper.find('input[type="file"]')

    const valid = makeJsonFile(
      'valid.json',
      JSON.stringify({ exported_at: '2026-07-05T00:00:00Z', proxies: [], accounts: [{ name: 'a' }] })
    )
    setInputFiles(input.element, [valid])
    await input.trigger('change')

    setInputFiles(input.element, [makeJsonFile('broken.json', 'not json')])
    await input.trigger('change')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportParseFailedFile')

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(importAdminData).toHaveBeenCalledWith({
      data: expect.objectContaining({
        accounts: [{ name: 'a' }]
      }),
      skip_default_group_bind: true
    })
  })

  it('imports every selected JSON file and merges the results', async () => {
    vi.mocked(importAdminData).mockResolvedValue({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_failed: 0
    })

    const wrapper = mountModal()
    const input = wrapper.find('input[type="file"]')

    setInputFiles(input.element, [
      makeJsonFile(
        'first.json',
        JSON.stringify({ exported_at: '2026-07-05T00:00:00Z', proxies: [], accounts: [{ name: 'a' }] })
      ),
      makeJsonFile(
        'second.json',
        JSON.stringify({
          exported_at: '2026-07-05T00:00:01Z',
          proxies: [{ proxy_key: 'p' }],
          accounts: [{ name: 'b' }]
        })
      )
    ])

    await input.trigger('change')
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    // fork 逐个文件调用导入接口，再在本地合并结果
    expect(importAdminData).toHaveBeenCalledTimes(2)
    const importedAccounts = vi
      .mocked(importAdminData)
      .mock.calls.flatMap((call) => {
        const payload = call[0] as { data?: { accounts?: Array<{ name: string }> } }
        return payload.data?.accounts?.map((account) => account.name) ?? []
      })
    expect(importedAccounts).toEqual(['a', 'b'])
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.dataImportSuccess')
  })

  it('notifies the parent when the modal is closed after a partial import', async () => {
    vi.mocked(importAdminData).mockResolvedValue({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_failed: 1
    })

    const wrapper = mountModal()
    const input = wrapper.find('input[type="file"]')
    setInputFiles(input.element, [
      makeJsonFile(
        'mixed.json',
        JSON.stringify({
          exported_at: '2026-07-05T00:00:00Z',
          proxies: [],
          accounts: [{ name: 'a' }, { name: 'b' }]
        })
      )
    ])

    await input.trigger('change')
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    // fork 用 warning + imported({close:false}) 通知父组件刷新，而不是直接关闭
    expect(showWarning).toHaveBeenCalledWith('admin.accounts.dataImportCompletedWithErrors')
    expect(wrapper.emitted('imported')?.[0]?.[0]).toEqual({ close: false })
  })

})
