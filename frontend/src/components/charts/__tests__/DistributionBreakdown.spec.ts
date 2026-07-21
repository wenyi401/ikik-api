import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import { getUserBreakdown } from '@/api/admin/dashboard'
import EndpointDistributionChart from '../EndpointDistributionChart.vue'
import GroupDistributionChart from '../GroupDistributionChart.vue'
import ModelDistributionChart from '../ModelDistributionChart.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('vue-chartjs', () => ({
  Doughnut: {
    props: ['data'],
    template: '<div />',
  },
}))

vi.mock('@/api/admin/dashboard', () => ({
  getUserBreakdown: vi.fn(),
}))

afterEach(() => {
  vi.clearAllMocks()
})

describe('distribution chart breakdown controls', () => {
  it('keeps endpoint rows inert when breakdown is disabled', async () => {
    const wrapper = mount(EndpointDistributionChart, {
      props: {
        endpointStats: [{ endpoint: '/v1/messages', requests: 1, total_tokens: 10, cost: 1, actual_cost: 1 }],
        enableBreakdown: false,
      },
    })

    const row = wrapper.find('tbody tr')
    await row.trigger('click')

    expect(row.classes()).not.toContain('cursor-pointer')
    expect(row.find('svg').exists()).toBe(false)
    expect(getUserBreakdown).not.toHaveBeenCalled()
  })

  it('keeps group rows inert when breakdown is disabled', async () => {
    const wrapper = mount(GroupDistributionChart, {
      props: {
        groupStats: [{ group_id: 1, group_name: 'group-a', requests: 1, total_tokens: 10, cost: 1, actual_cost: 1 }],
        enableBreakdown: false,
      },
    })

    const row = wrapper.find('tbody tr')
    await row.trigger('click')

    expect(row.classes()).not.toContain('cursor-pointer')
    expect(row.find('svg').exists()).toBe(false)
    expect(getUserBreakdown).not.toHaveBeenCalled()
  })

  it('keeps model rows inert when breakdown is disabled', async () => {
    const wrapper = mount(ModelDistributionChart, {
      props: {
        modelStats: [{
          model: 'model-a',
          requests: 1,
          input_tokens: 5,
          output_tokens: 5,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          total_tokens: 10,
          cost: 1,
          actual_cost: 1,
        }],
        enableBreakdown: false,
      },
    })

    const row = wrapper.find('tbody tr')
    await row.trigger('click')

    expect(row.classes()).not.toContain('cursor-pointer')
    expect(row.find('svg').exists()).toBe(false)
    expect(getUserBreakdown).not.toHaveBeenCalled()
  })
})
