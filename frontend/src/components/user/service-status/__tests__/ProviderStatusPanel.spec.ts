import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import ProviderStatusPanel from '../ProviderStatusPanel.vue'
import type { ProviderStatusSnapshot } from '@/api/serviceStatus'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const snapshot: ProviderStatusSnapshot = {
  fetched_at: '2026-07-29T02:00:00Z',
  stale: false,
  providers: [
    {
      id: 'claude',
      status: 'operational',
      source_updated_at: '2026-07-29T01:59:00Z',
      stale: false,
      components: [{ id: 'claude_api', name: 'Claude API', status: 'operational' }],
      active_incidents: [],
    },
    {
      id: 'grok',
      status: 'degraded',
      source_updated_at: '2026-07-29T01:58:00Z',
      stale: false,
      components: [{ id: 'xai_api', name: 'xAI API', status: 'degraded' }],
      active_incidents: [{
        id: 'INC1',
        name: 'Elevated Grok API errors',
        status: 'investigating',
        impact: 'degraded',
        updated_at: '2026-07-29T01:58:00Z',
        latest_body: 'We are investigating the issue.',
      }],
    },
    {
      id: 'gemini',
      status: 'unknown',
      source_updated_at: '',
      stale: false,
      components: [{ id: 'gemini_api', name: 'Gemini API', status: 'unknown' }],
      active_incidents: [],
    },
  ],
}

describe('ProviderStatusPanel', () => {
  it('shows provider health and incidents without external links', () => {
    const wrapper = mount(ProviderStatusPanel, {
      props: { snapshot, loading: false, error: false },
      global: {
        stubs: {
          Icon: { template: '<span />' },
          PlatformIcon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('Claude')
    expect(wrapper.text()).toContain('Grok')
    expect(wrapper.text()).toContain('Gemini')
    expect(wrapper.text()).toContain('Elevated Grok API errors')
    expect(wrapper.findAll('a')).toHaveLength(0)
    expect(wrapper.findAll('.official-card')).toHaveLength(3)
  })

  it('keeps all providers visible when an older response contains null arrays', () => {
    const malformedSnapshot = {
      ...snapshot,
      providers: snapshot.providers.map((provider) => ({
        ...provider,
        components: provider.id === 'claude' ? null : provider.components,
        active_incidents: null,
      })),
    } as unknown as ProviderStatusSnapshot

    const wrapper = mount(ProviderStatusPanel, {
      props: { snapshot: malformedSnapshot, loading: false, error: false },
      global: {
        stubs: {
          Icon: { template: '<span />' },
          PlatformIcon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.findAll('.official-card')).toHaveLength(3)
    expect(wrapper.text()).toContain('Claude')
    expect(wrapper.text()).toContain('Grok')
    expect(wrapper.text()).toContain('Gemini')
  })
})
