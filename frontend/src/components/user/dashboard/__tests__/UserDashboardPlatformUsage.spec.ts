import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UserDashboardPlatformUsage from '../UserDashboardPlatformUsage.vue'
import type { UserDashboardStats } from '@/api/usage'

const messages: Record<string, string> = {
  'dashboard.todayPlatformUsage': 'Today platform usage',
  'dashboard.requests': 'Requests',
  'dashboard.input': 'Input',
  'dashboard.output': 'Output',
  'dashboard.actual': 'Actual',
  'dashboard.noDataAvailable': 'No data available'
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

describe('UserDashboardPlatformUsage', () => {
  it('renders a real today_platforms payload instead of the empty state', () => {
    const stats: UserDashboardStats = {
      total_api_keys: 1,
      active_api_keys: 1,
      total_requests: 7,
      total_input_tokens: 5000,
      total_output_tokens: 1200,
      total_cache_creation_tokens: 400,
      total_cache_read_tokens: 600,
      total_tokens: 7200,
      total_cost: 0.25,
      total_actual_cost: 0.125,
      today_requests: 7,
      today_input_tokens: 5000,
      today_output_tokens: 1200,
      today_cache_creation_tokens: 400,
      today_cache_read_tokens: 600,
      today_tokens: 7200,
      today_cost: 0.25,
      today_actual_cost: 0.125,
      today_platforms: [{
        platform: 'openai',
        requests: 7,
        input_tokens: 5000,
        output_tokens: 1200,
        cache_creation_tokens: 400,
        cache_read_tokens: 600,
        total_tokens: 7200,
        cost: 0.25,
        actual_cost: 0.125
      }],
      average_duration_ms: 120,
      rpm: 2,
      tpm: 1440
    }

    const wrapper = mount(UserDashboardPlatformUsage, {
      props: { stats },
      global: {
        stubs: {
          UiSection: {
            props: ['title'],
            template: '<section><h2>{{ title }}</h2><slot name="actions" /><slot /></section>'
          }
        }
      }
    })

    expect(wrapper.find('.platform-empty').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('No data available')

    const row = wrapper.get('.platform-row')
    expect(row.text()).toContain('OpenAI')
    expect(row.text()).toContain('7 Requests')
    expect(row.text()).toContain('7.2K')
    expect(row.text()).toContain('$0.1250')
  })
})
