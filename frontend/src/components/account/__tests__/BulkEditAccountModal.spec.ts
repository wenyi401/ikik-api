import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BulkEditAccountModal from '../BulkEditAccountModal.vue'
import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'
import { adminAPI } from '@/api/admin'
import { accountsAPI } from '@/api/accounts'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      bulkUpdate: vi.fn(),
      checkMixedChannelRisk: vi.fn()
    }
  }
}))

vi.mock('@/api/accounts', () => ({
  accountsAPI: {
    bulkUpdate: vi.fn()
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

function makeGroup(overrides: Record<string, unknown>) {
  return {
    id: 1,
    name: 'group',
    description: null,
    platform: 'openai',
    rate_multiplier: 1,
    rpm_limit: 0,
    is_exclusive: false,
    status: 'active',
    owner_user_id: null,
    scope: 'user_private',
    subscription_type: 'standard',
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    image_price_1k: null,
    image_price_2k: null,
    image_price_4k: null,
    claude_code_only: false,
    fallback_group_id: null,
    fallback_group_id_on_invalid_request: null,
    allow_messages_dispatch: false,
    require_oauth_only: false,
    require_privacy_set: false,
    created_at: '',
    updated_at: '',
    model_routing: null,
    model_routing_enabled: false,
    mcp_xml_inject: false,
    supported_model_scopes: [],
    account_count: 0,
    active_account_count: 0,
    rate_limited_account_count: 0,
    sort_order: 0,
    ...overrides
  }
}

function mountModal(extraProps: Record<string, unknown> = {}, extraStubs: Record<string, unknown> = {}) {
  return mount(BulkEditAccountModal, {
    props: {
      show: true,
      accountIds: [1, 2],
      selectedPlatforms: ['antigravity'],
      selectedTypes: ['apikey'],
      proxies: [],
      groups: [],
      ...extraProps
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true,
        Select: {
          props: ['modelValue', 'options'],
          emits: ['update:modelValue'],
          template: `
            <select
              v-bind="$attrs"
              :value="modelValue"
              @change="$emit('update:modelValue', $event.target.value)"
            >
              <option v-for="option in options" :key="option.value" :value="option.value" :disabled="option.disabled">
                {{ option.label }}
              </option>
            </select>
          `
        },
        ProxySelector: true,
        GroupSelector: true,
        Icon: true,
        ...extraStubs
      }
    }
  })
}

describe('BulkEditAccountModal', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.accounts.bulkUpdate).mockReset()
    vi.mocked(adminAPI.accounts.checkMixedChannelRisk).mockReset()
    vi.mocked(accountsAPI.bulkUpdate).mockReset()

    vi.mocked(adminAPI.accounts.bulkUpdate).mockResolvedValue({
      success: 2,
      failed: 0,
      results: []
    } as any)
    vi.mocked(adminAPI.accounts.checkMixedChannelRisk).mockResolvedValue({
      has_risk: false
    } as any)
    vi.mocked(accountsAPI.bulkUpdate).mockResolvedValue({
      success: 2,
      failed: 0,
      results: []
    } as any)
  })

  it('antigravity 白名单包含 Gemini 图片模型且过滤掉普通 GPT 模型', async () => {
    const wrapper = mountModal()
    const selector = wrapper.findComponent(ModelWhitelistSelector)
    expect(selector.exists()).toBe(true)

    await selector.find('div.cursor-pointer').trigger('click')

    expect(wrapper.text()).toContain('gemini-3.1-flash-image')
    expect(wrapper.text()).toContain('gemini-2.5-flash-image')
    expect(wrapper.text()).not.toContain('gpt-5.3-codex')
  })

  it('antigravity 映射预设包含图片映射并过滤 OpenAI 预设', async () => {
    const wrapper = mountModal()

    const mappingTab = wrapper.findAll('button').find((btn) => btn.text().includes('admin.accounts.modelMapping'))
    expect(mappingTab).toBeTruthy()
    await mappingTab!.trigger('click')

    expect(wrapper.text()).toContain('Flash-Image')
    expect(wrapper.text()).toContain('Pro-Image')
    expect(wrapper.text()).not.toContain('GPT-5.3 Codex Spark')
  })

  it('仅勾选模型限制且白名单留空时，应提交空 model_mapping 以支持所有模型', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['anthropic'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-model-restriction-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      credentials: {
        model_mapping: {}
      }
    })
  })

  it('全部目标为 Grok OAuth 时，官方主机 base_url 作为手动端点切换正常提交', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['grok'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-base-url-enabled').setValue(true)
    await wrapper.get('#bulk-edit-base-url').setValue('https://api.x.ai/v1')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      credentials: {
        base_url: 'https://api.x.ai/v1'
      }
    })
  })

  it('所选全为 grok 时展示快捷端点，点击后填入并自动勾选 base_url', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['grok'],
      selectedTypes: ['oauth']
    })

    const presets = wrapper.findAll('[data-testid="grok-base-url-preset"]')
    expect(presets.length).toBe(5)

    // 第三个预设为区域 API (us-east-1.api.x.ai/v1)
    await presets[2].trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      credentials: {
        base_url: 'https://us-east-1.api.x.ai/v1'
      }
    })
  })

  it('所选含非 grok 平台时不展示快捷端点', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['grok', 'anthropic'],
      selectedTypes: ['apikey']
    })

    expect(wrapper.findAll('[data-testid="grok-base-url-preset"]').length).toBe(0)
  })

  it('全部目标为 Grok OAuth 时，第三方 base_url 正常提交', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['grok'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-base-url-enabled').setValue(true)
    await wrapper.get('#bulk-edit-base-url').setValue('https://relay.example.com/v1')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      credentials: {
        base_url: 'https://relay.example.com/v1'
      }
    })
  })

  it('混合类型选择（含 apikey）时官方主机 base_url 不拦截', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['grok'],
      selectedTypes: ['apikey', 'oauth']
    })

    await wrapper.get('#bulk-edit-base-url-enabled').setValue(true)
    await wrapper.get('#bulk-edit-base-url').setValue('https://api.x.ai/v1')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      credentials: {
        base_url: 'https://api.x.ai/v1'
      }
    })
  })

  it('OpenAI 账号批量编辑可开启自动透传', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-passthrough-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: true
      }
    })
  })

  it('OpenAI OAuth 批量编辑应提交 OAuth 专属 WS mode 字段', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-ws-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-ws-mode-select"]').setValue('passthrough')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_oauth_responses_websockets_v2_mode: 'passthrough',
        openai_oauth_responses_websockets_v2_enabled: true
      }
    })
  })

  it('OpenAI API Key 批量编辑不显示 WS mode 入口', () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    expect(wrapper.find('#bulk-edit-openai-ws-mode-enabled').exists()).toBe(false)
  })

  it('OpenAI OAuth 批量编辑应提交 codex_cli_only 字段', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-codex-cli-only-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-cli-only-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        codex_cli_only: true
      }
    })
  })

  it('OpenAI API Key 批量编辑应提交 API Key 专属 WS mode 字段', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-openai-apikey-ws-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-apikey-ws-mode-select"]').setValue('ctx_pool')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_apikey_responses_websockets_v2_mode: 'ctx_pool',
        openai_apikey_responses_websockets_v2_enabled: true
      }
    })
  })

  it('OpenAI 账号批量编辑可关闭自动透传', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: false,
        openai_oauth_passthrough: false
      }
    })
  })

  it('开启 OpenAI 自动透传时不再同时提交模型限制', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-passthrough-toggle').trigger('click')
    await wrapper.get('#bulk-edit-model-restriction-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: true
      }
    })
    expect(wrapper.text()).toContain('admin.accounts.openai.modelRestrictionDisabledByPassthrough')
  })

  it('filtered-results 模式下应提交 filters 而不是 account_ids', async () => {
    const wrapper = mountModal({
      accountIds: [],
      target: {
        mode: 'filtered',
        filters: {
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          group: '12',
          search: 'bulk-target',
          privacy_mode: 'training_set_cf_blocked'
        },
        previewCount: 5,
        selectedPlatforms: ['openai'],
        selectedTypes: ['oauth']
      }
    })

    await wrapper.get('#bulk-edit-status-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith({
      filters: {
        platform: 'openai',
        type: 'oauth',
        status: 'active',
        group: '12',
        search: 'bulk-target',
        privacy_mode: 'training_set_cf_blocked'
      },
      status: 'active'
    })
  })

  it('用户作用域批量编辑分组只展示当前账号平台兼容分组', async () => {
    const wrapper = mountModal({
      accountScope: 'user',
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      groups: [
        makeGroup({ id: 1, name: 'private-u9-openai', platform: 'openai' }),
        makeGroup({ id: 2, name: 'private-u9-anthropic', platform: 'anthropic' }),
        makeGroup({ id: 3, name: 'private-u9-gemini', platform: 'gemini' }),
        makeGroup({ id: 4, name: 'Codex OAuth Only', platform: 'openai', require_oauth_only: true })
      ]
    }, {
      GroupSelector: {
        props: ['groups'],
        template: `
          <div>
            <span v-for="group in groups" :key="group.id" class="group-option">{{ group.name }}</span>
          </div>
        `
      }
    })

    expect(wrapper.find('#bulk-edit-share-mode-enabled').exists()).toBe(true)
    expect(wrapper.find('#bulk-edit-groups-enabled').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('private-u9-openai')
  })

  it('用户作用域提交分组更新时调用用户接口', async () => {
    const wrapper = mountModal({
      accountScope: 'user',
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth'],
      groups: [
        makeGroup({ id: 1, name: 'private-u9-openai', platform: 'openai' }),
        makeGroup({ id: 2, name: 'private-u9-anthropic', platform: 'anthropic' })
      ]
    }, {
      GroupSelector: {
        props: ['groups'],
        emits: ['update:modelValue'],
        template: `
          <div>
            <button
              v-for="group in groups"
              :key="group.id"
              type="button"
              class="group-option"
              @click="$emit('update:modelValue', [group.id])"
            >
              {{ group.name }}
            </button>
          </div>
        `
      }
    })

    await wrapper.get('#bulk-edit-share-mode-enabled').setValue(true)
    await wrapper.get('select[aria-labelledby="bulk-edit-share-mode-label"]').setValue('public')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(accountsAPI.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      share_mode: 'public'
    })
    expect(adminAPI.accounts.bulkUpdate).not.toHaveBeenCalled()
  })

  it('allows a user to apply a proxy and public sharing together', async () => {
    const wrapper = mountModal({
      accountScope: 'user',
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth'],
      proxies: [{ id: 7, name: 'Residential', status: 'active' }]
    }, {
      ProxySelector: {
        props: ['modelValue'],
        emits: ['update:modelValue'],
        template: `
          <button type="button" data-testid="select-proxy" @click="$emit('update:modelValue', 7)">
            select proxy
          </button>
        `
      }
    })

    await wrapper.get('#bulk-edit-proxy-enabled').setValue(true)
    await wrapper.get('[data-testid="select-proxy"]').trigger('click')
    await flushPromises()
    await wrapper.get('#bulk-edit-share-mode-enabled').setValue(true)

    const shareModeSelect = wrapper.get('select[aria-labelledby="bulk-edit-share-mode-label"]')
    expect(shareModeSelect.get('option[value="public"]').attributes('disabled')).toBeUndefined()
    await shareModeSelect.setValue('public')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(accountsAPI.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      proxy_id: 7,
      share_mode: 'public'
    })
  })

  it('用户作用域批量改为公共共享时支持后台任务响应', async () => {
    vi.mocked(accountsAPI.bulkUpdate).mockResolvedValueOnce({
      async: true,
      task: {
        id: 77,
        scope: 'user',
        operation: 'user_set_public_share',
        status: 'pending',
        total: 2,
        processed: 0,
        success: 0,
        failed: 0,
        created_by: 9,
      },
      success: 0,
      failed: 0,
      results: []
    } as any)
    const wrapper = mountModal({
      accountScope: 'user',
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-share-mode-enabled').setValue(true)
    await wrapper.get('select[aria-labelledby="bulk-edit-share-mode-label"]').setValue('public')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(accountsAPI.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      share_mode: 'public'
    })
    expect(wrapper.emitted('updated')?.[0]).toEqual([
      expect.objectContaining({
        async: true,
        task: expect.objectContaining({ id: 77, operation: 'user_set_public_share' })
      })
    ])
  })

  it('admin OpenAI bulk edit submits account_level', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-account-level-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-account-level-select"]').setValue('plus')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      account_level: 'plus'
    })
  })
})
