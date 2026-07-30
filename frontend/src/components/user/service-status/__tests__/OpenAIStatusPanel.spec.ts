import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import OpenAIStatusPanel from '../OpenAIStatusPanel.vue'
import type { OpenAIStatusSnapshot } from '@/api/serviceStatus'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const snapshot: OpenAIStatusSnapshot = {
  products: [
    { id: 'chatgpt', status: 'degraded', active_incidents: 0 },
    { id: 'codex', status: 'operational', active_incidents: 0 },
  ],
  active_incidents: [{
    id: 'global-incident',
    name: 'Elevated Error Rates',
    status: 'investigating',
    impact: 'minor',
    products: [],
    created_at: '2026-07-24T10:00:00Z',
    updated_at: '2026-07-24T10:06:05Z',
    latest_body: 'We are investigating the issue for the listed services.',
    source_url: 'https://status.openai.com/',
  }],
  recent_resolved: [],
  source_updated_at: '2026-07-24T10:06:05Z',
  fetched_at: '2026-07-24T10:07:00Z',
  stale: false,
}

describe('OpenAIStatusPanel', () => {
  it('shows product health without a contradictory per-product incident label', () => {
    const wrapper = mount(OpenAIStatusPanel, {
      props: { snapshot, loading: false, error: false },
      global: {
        stubs: {
          Icon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('serviceStatus.status.degraded')
    expect(wrapper.text()).not.toContain('serviceStatus.official.noActive')
    expect(wrapper.text()).not.toContain('serviceStatus.official.activeCount')
    expect(wrapper.get('.incident-count').text()).toBe('1')
    expect(wrapper.get('.incident-row').text()).toContain('Elevated Error Rates')
  })
})
