import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'

const { showSuccessMock, showInfoMock } = vi.hoisted(() => {
  Object.defineProperty(globalThis, 'localStorage', {
    value: {
      getItem: vi.fn(),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn()
    },
    configurable: true
  })

  return {
    showSuccessMock: vi.fn(),
    showInfoMock: vi.fn()
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: showSuccessMock,
    showInfo: showInfoMock
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      probeModelList: vi.fn(),
      probeModels: vi.fn()
    }
  }
}))

vi.mock('@/api/admin/index', () => ({
  adminAPI: {
    accounts: {
      probeModelList: vi.fn(),
      probeModels: vi.fn()
    }
  }
}))

vi.mock('@/api/admin/index.ts', () => ({
  adminAPI: {
    accounts: {
      probeModelList: vi.fn(),
      probeModels: vi.fn()
    }
  }
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_err: unknown, fallback: string) => fallback
}))

vi.mock('@/components/account/ModelProbeModal.vue', () => ({
  default: {
    name: 'ModelProbeModal',
    props: ['show', 'defaultPlatform', 'accountScope'],
    emits: ['close', 'apply'],
    template: `
      <div v-if="show" data-test="probe-modal">
        <button type="button" @click="$emit('apply', ['gpt-5.4-openai-compact'])">apply-probed</button>
      </div>
    `
  }
}))

vi.mock('../ModelProbeModal.vue', () => ({
  default: {
    name: 'ModelProbeModal',
    props: ['show', 'defaultPlatform', 'accountScope'],
    emits: ['close', 'apply'],
    template: `
      <div v-if="show" data-test="probe-modal">
        <button type="button" @click="$emit('apply', ['gpt-5.4-openai-compact'])">apply-probed</button>
      </div>
    `
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params?.count !== undefined ? `${key}:${params.count}` : key
    })
  }
})

describe('ModelWhitelistSelector', () => {
  it('把用户范围传给探测弹窗并隐藏管理员同步入口', async () => {
    const wrapper = mount(ModelWhitelistSelector, {
      props: {
        modelValue: [],
        platform: 'openai',
        accountScope: 'user',
        syncCredentials: {
          platform: 'openai',
          type: 'apikey',
          api_key: 'sk-test'
        }
      },
      global: {
        stubs: {
          ModelIcon: true,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).not.toContain('admin.accounts.syncUpstreamModels')
    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.modelProbe.openButton'))!.trigger('click')
    expect(wrapper.getComponent({ name: 'ModelProbeModal' }).props('accountScope')).toBe('user')
  })

  it('通过探测弹窗把模型合并到白名单', async () => {
    const wrapper = mount(ModelWhitelistSelector, {
      props: {
        modelValue: ['gpt-5.4'],
        platform: 'openai'
      },
      global: {
        stubs: {
          ModelIcon: true,
          Icon: true
        }
      }
    })

    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.modelProbe.openButton'))!.trigger('click')
    await wrapper.get('[data-test="probe-modal"] button').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([['gpt-5.4', 'gpt-5.4-openai-compact']])
    expect(showSuccessMock).toHaveBeenCalledWith('admin.accounts.modelProbe.addedModels:1')
  })

  it('切换平台后清空本地探测候选模型', async () => {
    const wrapper = mount(ModelWhitelistSelector, {
      props: {
        modelValue: [],
        platform: 'openai'
      },
      global: {
        stubs: {
          ModelIcon: true,
          Icon: true
        }
      }
    })

    await wrapper.findAll('button').find(button => button.text().includes('admin.accounts.modelProbe.openButton'))!.trigger('click')
    await wrapper.get('[data-test="probe-modal"] button').trigger('click')
    await wrapper.setProps({ modelValue: ['gpt-5.4-openai-compact'] })

    expect(wrapper.text()).toContain('gpt-5.4-openai-compact')

    await wrapper.setProps({
      modelValue: [],
      platform: 'anthropic'
    })
    await wrapper.find('.cursor-pointer').trigger('click')
    await wrapper.find('input').setValue('gpt-5.4-openai-compact')

    expect(wrapper.text()).toContain('admin.accounts.noMatchingModels')
  })

  it('warns when model IDs sync but capability metadata is incomplete', async () => {
    syncUpstreamModels.mockResolvedValue({
      models: ['x-preview-f-free'],
      warnings: [
        {
          code: 'upstream_model_metadata_incomplete',
          message: 'Model IDs were synced, but capability metadata could not be updated.'
        }
      ]
    })
    const wrapper = mount(ModelWhitelistSelector, {
      props: {
        modelValue: [],
        platform: 'openai',
        accountId: 46
      },
      global: {
        stubs: {
          ModelIcon: true
        }
      }
    })

    const syncButton = wrapper
      .findAll('button')
      .find(button => button.text() === 'admin.accounts.syncUpstreamModels')
    expect(syncButton).toBeDefined()
    await syncButton!.trigger('click')
    await flushPromises()

    expect(wrapper.emitted('update:modelValue')).toEqual([[['x-preview-f-free']]])
    expect(showWarning).toHaveBeenCalledWith('admin.accounts.syncUpstreamModelsMetadataIncomplete')
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('reports a successful preview so account creation can persist metadata', async () => {
    syncUpstreamModelsPreview.mockResolvedValue({
      models: ['x-preview-f-free'],
      metadata: {
        'x-preview-f-free': {
          id: 'x-preview-f-free',
          reasoning: true,
          supported_reasoning_levels: ['low', 'high', 'max'],
        },
      },
    })
    const wrapper = mountSelector({
      syncCredentials: {
        platform: 'openai',
        type: 'apikey',
        base_url: 'https://opencode.ai/zen/v1',
        api_key: 'test-key',
      },
    })
    const syncButton = wrapper
      .findAll('button')
      .find(button => button.text() === 'admin.accounts.syncUpstreamModels')

    expect(syncButton).toBeDefined()
    await syncButton?.trigger('click')
    await flushPromises()

    expect(syncUpstreamModelsPreview).toHaveBeenCalledOnce()
    expect(wrapper.emitted('upstream-synced')).toEqual([[]])
    expect(wrapper.emitted('update:modelValue')).toEqual([[['x-preview-f-free']]])
  })
})
