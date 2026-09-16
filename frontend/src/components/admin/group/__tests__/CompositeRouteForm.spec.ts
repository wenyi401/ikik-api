import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CompositeRouteForm from '../CompositeRouteForm.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

// select 顺序：match_type → endpoint → target_platform。
const targetPlatformOptions = (wrapper: ReturnType<typeof mount>) =>
  wrapper.findAll('select')[2].findAll('option').map((option) => option.text())

describe('CompositeRouteForm target platforms', () => {
  it('offers OpenCode and the CN platforms as route targets', () => {
    const wrapper = mount(CompositeRouteForm, { global: { stubs: { Icon: true } } })
    const options = targetPlatformOptions(wrapper)

    expect(options).toContain('OpenCode')
    expect(options).toContain('Kimi')
    expect(options).toContain('Zhipu GLM')
    expect(options).toContain('DeepSeek')
    expect(options).toContain('MiniMax')
  })

  it('does not offer platforms the backend rejects as route targets', () => {
    const wrapper = mount(CompositeRouteForm, { global: { stubs: { Icon: true } } })
    const options = targetPlatformOptions(wrapper)

    expect(options).not.toContain('Kiro')
    expect(options).not.toContain('Custom')
  })

  it('edits an existing OpenCode route without losing its target platform', async () => {
    const wrapper = mount(CompositeRouteForm, {
      props: {
        route: {
          id: 1,
          group_id: 2,
          public_model: 'oc-model',
          match_type: 'exact',
          target_platform: 'opencode_go',
          upstream_model: 'oc-model',
          endpoint: 'any',
          priority: 100,
          enabled: true,
          notes: ''
        } as any
      },
      global: { stubs: { Icon: true } }
    })

    const targetSelect = wrapper.findAll('select')[2]
    expect((targetSelect.element as HTMLSelectElement).value).toBe('opencode_go')
  })
})
