import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

const { getDashboardTrend, getDashboardStats, showError } = vi.hoisted(() => ({
  getDashboardTrend: vi.fn(),
  getDashboardStats: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardTrend,
    getDashboardStats
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (key === 'profile.activity.githubTitle') return '过去一年 Token 使用'
        if (key === 'profile.activity.mobileMeta') return `累计 ${params?.total} · 连续 ${params?.streak} 天`
        if (key === 'profile.activity.loadFailed') return '加载 Token 活动失败'
        if (key === 'common.retry') return '重试'
        return key
      }
    })
  }
})

import ProfileTokenActivityHeatmap from '@/components/user/profile/ProfileTokenActivityHeatmap.vue'

describe('ProfileTokenActivityHeatmap', () => {
  afterEach(() => {
    document.documentElement.classList.remove('dark')
  })

  it('uses a stable loading skeleton before rendering the activity grid', async () => {
    let resolveTrend: (value: { trend: [] }) => void = () => undefined
    getDashboardTrend.mockReturnValue(new Promise((resolve) => { resolveTrend = resolve }))
    getDashboardStats.mockResolvedValue({ total_tokens: 0 })

    const wrapper = mount(ProfileTokenActivityHeatmap)
    await nextTick()
    expect(wrapper.get('[data-testid="profile-token-activity-loading"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('common.loading')

    resolveTrend({ trend: [] })
    await flushPromises()

    expect(wrapper.find('[data-testid="profile-token-activity-loading"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('过去一年 Token 使用')
    expect(wrapper.get('[data-testid="profile-token-activity-grid"]').attributes('style')).toContain('minmax(0, 1fr)')
  })

  it('shows a retry action when activity loading fails', async () => {
    getDashboardTrend.mockRejectedValue(new Error('network error'))
    getDashboardStats.mockResolvedValue({ total_tokens: 0 })

    const wrapper = mount(ProfileTokenActivityHeatmap)
    await flushPromises()

    expect(wrapper.get('[data-testid="profile-token-activity-error"]').text()).toContain('加载 Token 活动失败')
    expect(wrapper.get('[data-testid="profile-token-activity-error"]').text()).toContain('重试')
  })

  it('uses a muted empty-cell color in dark mode', async () => {
    document.documentElement.classList.add('dark')
    getDashboardTrend.mockResolvedValue({ trend: [] })
    getDashboardStats.mockResolvedValue({ total_tokens: 0 })

    const wrapper = mount(ProfileTokenActivityHeatmap)
    await flushPromises()

    const firstCell = wrapper.get('[data-testid="profile-token-activity-grid"] button')
    expect(firstCell.attributes('style')).toContain('rgb(58, 58, 58)')

    wrapper.unmount()
  })
})
