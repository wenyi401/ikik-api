import { nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ChannelQuotaSummary from '../ChannelQuotaSummary.vue'
import type {
  AccountQuotaDashboard,
  AccountQuotaDimensionSummary,
  AccountQuotaGroupSummary,
  AccountQuotaSummary,
} from '@/types'

const mobileViewport = vi.hoisted(() => ({ value: false }))

vi.mock('@vueuse/core', () => ({
  useMediaQuery: () => mobileViewport,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => key === 'pagination.pageOf'
        ? `${params?.page} / ${params?.total}`
        : key,
    }),
  }
})

const dimension: AccountQuotaDimensionSummary = {
  enabled_account_count: 0,
  exhausted_account_count: 0,
  limit: 0,
  used: 0,
  remaining: 0,
  utilization: 0,
}

function makeGroup(index: number): AccountQuotaGroupSummary {
  return {
    group_id: index,
    group_name: `Group ${index}`,
    group_status: 'active',
    platform: 'openai',
    account_count: 10,
    active_account_count: 10,
    schedulable_account_count: 9,
    rate_limited_account_count: 1,
    quota_protected_account_count: 0,
    error_account_count: 0,
    disabled_account_count: 0,
    concurrency_capacity: 100,
    schedulable_concurrency_capacity: 90,
    quota_account_count: 10,
    unlimited_account_count: 0,
    total: dimension,
    daily: dimension,
    weekly: dimension,
    usage_windows: [{
      window: '5h',
      account_count: 10,
      known_account_count: 10,
      average_utilization: 35.2,
      remaining_capacity_percent: 64.8,
    }, {
      window: '7d',
      account_count: 10,
      known_account_count: 10,
      average_utilization: 48.4,
      remaining_capacity_percent: 51.6,
    }],
  }
}

function makeDashboard(groupCount: number): AccountQuotaDashboard {
  const groups = Array.from({ length: groupCount }, (_, index) => makeGroup(index + 1))
  const totalAccounts = groupCount * 10
  const totals: AccountQuotaSummary = {
    platform: 'all',
    type: 'all',
    account_count: totalAccounts,
    active_account_count: totalAccounts,
    schedulable_account_count: groupCount * 9,
    rate_limited_account_count: groupCount,
    quota_protected_account_count: 0,
    error_account_count: 0,
    disabled_account_count: 0,
    concurrency_capacity: totalAccounts * 10,
    schedulable_concurrency_capacity: groupCount * 90,
    quota_account_count: totalAccounts,
    unlimited_account_count: 0,
    total: dimension,
    daily: dimension,
    weekly: dimension,
    usage_windows: [],
  }

  return {
    generated_at: '2026-07-13T00:00:00Z',
    summaries: [],
    totals,
    group_summaries: groups,
  }
}

function mountSummary(
  groupCount: number,
  pageSizeProps: { desktopPageSize?: number; mobilePageSize?: number } = {},
) {
  return mount(ChannelQuotaSummary, {
    props: {
      dashboard: makeDashboard(groupCount),
      loading: false,
      error: false,
      title: 'Quota pool',
      emptyMessage: 'Empty',
      loadFailedMessage: 'Failed',
      ...pageSizeProps,
    },
    global: {
      stubs: {
        Icon: true,
        PlatformIcon: true,
        UiIconButton: {
          props: ['label'],
          template: '<button type="button" :aria-label="label"><slot /></button>',
        },
      },
    },
  })
}

describe('ChannelQuotaSummary pagination', () => {
  it('shows three groups per page and clamps the page when data shrinks', async () => {
    mobileViewport.value = false
    const wrapper = mountSummary(7)

    expect(wrapper.findAll('.quota-group-card')).toHaveLength(3)
    expect(wrapper.text()).toContain('Group 1')
    expect(wrapper.text()).not.toContain('Group 4')

    await wrapper.get('button[aria-label="pagination.next"]').trigger('click')
    expect(wrapper.text()).toContain('Group 4')
    expect(wrapper.text()).not.toContain('Group 1')

    await wrapper.get('button[aria-label="pagination.next"]').trigger('click')
    expect(wrapper.findAll('.quota-group-card')).toHaveLength(1)
    expect(wrapper.text()).toContain('Group 7')

    await wrapper.setProps({ dashboard: makeDashboard(1) })
    await nextTick()

    expect(wrapper.text()).toContain('Group 1')
    expect(wrapper.find('.quota-pagination').exists()).toBe(false)
  })

  it('shows one group per page on mobile', () => {
    mobileViewport.value = true
    const wrapper = mountSummary(4)

    expect(wrapper.findAll('.quota-group-card')).toHaveLength(1)
    expect(wrapper.text()).toContain('Group 1')
    expect(wrapper.text()).toContain('1 / 4')
  })

  it('keeps the shared-pool desktop page at three groups', () => {
    mobileViewport.value = false
    const wrapper = mountSummary(4, { desktopPageSize: 3 })

    expect(wrapper.findAll('.quota-group-card')).toHaveLength(3)
    expect(wrapper.text()).toContain('Group 3')
    expect(wrapper.text()).not.toContain('Group 4')
    expect(wrapper.text()).toContain('1 / 2')
  })

  it('supports three shared-pool groups per mobile page', () => {
    mobileViewport.value = true
    const wrapper = mountSummary(4, { mobilePageSize: 3 })

    expect(wrapper.findAll('.quota-group-card')).toHaveLength(3)
    expect(wrapper.text()).toContain('Group 3')
    expect(wrapper.text()).not.toContain('Group 4')
    expect(wrapper.text()).toContain('1 / 2')
  })

  it('renders the 5h and 7d windows in one combined panel', () => {
    mobileViewport.value = true
    const wrapper = mountSummary(1)

    expect(wrapper.findAll('.quota-windows')).toHaveLength(1)
    expect(wrapper.get('.quota-windows').findAll('.quota-window-card')).toHaveLength(2)
  })
})
