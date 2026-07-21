import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import UsageView from '../UsageView.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'

const {
  query,
  getStatsByDateRange,
  getDashboardModels,
  getDashboardSnapshotV2,
  list,
  showError,
  showWarning,
  showSuccess,
  showInfo,
} = vi.hoisted(() => ({
  query: vi.fn(),
  getStatsByDateRange: vi.fn(),
  getDashboardModels: vi.fn(),
  getDashboardSnapshotV2: vi.fn(),
  list: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
}))

const messages: Record<string, string> = {
  'usage.costDetails': 'Cost Breakdown',
  'admin.usage.inputCost': 'Input Cost',
  'admin.usage.outputCost': 'Output Cost',
  'admin.usage.cacheCreationCost': 'Cache Creation Cost',
  'admin.usage.cacheReadCost': 'Cache Read Cost',
  'usage.inputTokenPrice': 'Input price',
  'usage.outputTokenPrice': 'Output price',
  'usage.perMillionTokens': '/ 1M tokens',
  'usage.serviceTier': 'Service tier',
  'usage.serviceTierPriority': 'Fast',
  'usage.serviceTierFlex': 'Flex',
  'usage.serviceTierStandard': 'Standard',
  'usage.rate': 'Rate',
  'usage.original': 'Original',
  'usage.billed': 'Billed',
  'usage.allApiKeys': 'All API Keys',
  'usage.apiKeyFilter': 'API Key',
  'usage.model': 'Model',
  'usage.reasoningEffort': 'Reasoning Effort',
  'usage.type': 'Type',
  'usage.tokens': 'Tokens',
  'usage.cost': 'Cost',
  'usage.firstToken': 'First Token',
  'usage.duration': 'Duration',
  'usage.time': 'Time',
  'usage.userAgent': 'User Agent',
}

vi.mock('@/api', () => ({
  usageAPI: {
    query,
    getStatsByDateRange,
    getDashboardModels,
    getDashboardSnapshotV2,
  },
  keysAPI: {
    list,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess, showInfo }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<div><slot name="actions" /><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
}

describe('user UsageView tooltip', () => {
  beforeEach(() => {
    query.mockReset()
    getStatsByDateRange.mockReset()
    getDashboardModels.mockReset()
    getDashboardSnapshotV2.mockReset()
    list.mockReset()
    showError.mockReset()
    showWarning.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()

    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: vi.fn().mockImplementation((media: string) => ({
        matches: true,
        media,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })

    getDashboardModels.mockResolvedValue({ models: [], start_date: '', end_date: '' })
    getDashboardSnapshotV2.mockResolvedValue({
      generated_at: '',
      start_date: '',
      end_date: '',
      granularity: 'day',
      groups: [],
    })

    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      top: 20,
      left: 20,
      right: 120,
      bottom: 40,
      width: 100,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)

    ;(globalThis as any).ResizeObserver = class {
      observe() {}
      disconnect() {}
    }
  })

  it('loads responsive user analytics with safe distribution-chart controls', async () => {
    const statsResponse = {
      total_requests: 3,
      total_input_tokens: 10,
      total_output_tokens: 5,
      total_cache_tokens: 0,
      total_cache_read_tokens: 0,
      total_cache_creation_tokens: 0,
      total_tokens: 15,
      total_cost: 1,
      total_actual_cost: 0.8,
      average_duration_ms: 120,
      endpoints: [
        { endpoint: '/v1/messages', requests: 3, total_tokens: 15, cost: 1, actual_cost: 0.8 },
      ],
    }
    const modelResponse = {
      models: [{
        model: 'claude-sonnet-4',
        requests: 3,
        input_tokens: 10,
        output_tokens: 5,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        total_tokens: 15,
        cost: 1,
        actual_cost: 0.8,
      }],
      start_date: '2026-03-01',
      end_date: '2026-03-07',
    }
    const snapshotResponse = {
      generated_at: '2026-03-08T00:00:00Z',
      start_date: '2026-03-01',
      end_date: '2026-03-07',
      granularity: 'day',
      groups: [{ group_id: 1, group_name: 'Primary', requests: 3, total_tokens: 15, cost: 1, actual_cost: 0.8 }],
    }

    let resolveStats!: (value: typeof statsResponse) => void
    let resolveModels!: (value: typeof modelResponse) => void
    let resolveSnapshot!: (value: typeof snapshotResponse) => void
    getStatsByDateRange.mockResolvedValue(statsResponse)
    getStatsByDateRange.mockReturnValueOnce(new Promise(resolve => { resolveStats = resolve }))
    getDashboardModels.mockReturnValueOnce(new Promise(resolve => { resolveModels = resolve }))
    getDashboardSnapshotV2.mockReturnValueOnce(new Promise(resolve => { resolveSnapshot = resolve }))
    query.mockResolvedValue({ items: [], total: 0, pages: 0 })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await nextTick()

    const modelChart = wrapper.getComponent(ModelDistributionChart)
    const groupChart = wrapper.getComponent(GroupDistributionChart)
    const endpointChart = wrapper.getComponent(EndpointDistributionChart)
    expect(modelChart.props('loading')).toBe(true)
    expect(groupChart.props('loading')).toBe(true)
    expect(endpointChart.props('loading')).toBe(true)
    expect(wrapper.get('[data-testid="usage-analytics"]').exists()).toBe(true)
    expect(wrapper.get('.usage-analytics-panel--wide').exists()).toBe(true)

    resolveStats(statsResponse)
    resolveModels(modelResponse)
    resolveSnapshot(snapshotResponse)
    await flushPromises()

    expect(modelChart.props('modelStats')).toEqual(modelResponse.models)
    expect(modelChart.props('enableBreakdown')).toBe(false)
    expect(modelChart.props('showAccountCost')).toBe(false)
    expect(modelChart.props('loading')).toBe(false)
    expect(groupChart.props('groupStats')).toEqual(snapshotResponse.groups)
    expect(groupChart.props('enableBreakdown')).toBe(false)
    expect(groupChart.props('showAccountCost')).toBe(false)
    expect(groupChart.props('loading')).toBe(false)
    expect(endpointChart.props('endpointStats')).toEqual(statsResponse.endpoints)
    expect(endpointChart.props('enableBreakdown')).toBe(false)
    expect(endpointChart.props('loading')).toBe(false)

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.filters.api_key_id = 42
    setupState.applyFilters()
    await flushPromises()

    expect(getStatsByDateRange).toHaveBeenLastCalledWith(expect.any(String), expect.any(String), 42)
    expect(getDashboardModels).toHaveBeenLastCalledWith(expect.objectContaining({
      api_key_id: 42,
      model_source: 'requested',
    }))
    expect(getDashboardSnapshotV2).toHaveBeenLastCalledWith(expect.objectContaining({
      api_key_id: 42,
      include_trend: false,
      include_model_stats: false,
      include_group_stats: true,
    }))
  })

  it('shows fast service tier and unit prices in user tooltip', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-1',
          actual_cost: 0.092883,
          total_cost: 0.092883,
          rate_multiplier: 1,
          service_tier: 'priority',
          input_cost: 0.020285,
          output_cost: 0.00303,
          cache_creation_cost: 0,
          cache_read_cost: 0.069568,
          input_tokens: 4057,
          output_tokens: 101,
          cache_creation_tokens: 0,
          cache_read_tokens: 278272,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-1',
      actual_cost: 0.092883,
      total_cost: 0.092883,
      rate_multiplier: 1,
      service_tier: 'priority',
      input_cost: 0.020285,
      output_cost: 0.00303,
      cache_creation_cost: 0,
      cache_read_cost: 0.069568,
      input_tokens: 4057,
      output_tokens: 101,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Service tier')
    expect(text).toContain('Fast')
    expect(text).toContain('Rate')
    expect(text).toContain('1.00x')
    expect(text).toContain('Billed')
    expect(text).toContain('$0.092883')
    expect(text).toContain('$5.0000 / 1M tokens')
    expect(text).toContain('$30.0000 / 1M tokens')
  })

  it('exports csv with input and output unit price columns', async () => {
    const exportedLogs = [
      {
        request_id: 'req-user-export',
        actual_cost: 0.092883,
        total_cost: 0.092883,
        rate_multiplier: 1,
        service_tier: 'priority',
        input_cost: 0.020285,
        output_cost: 0.00303,
        cache_creation_cost: 0.000001,
        cache_read_cost: 0.069568,
        input_tokens: 4057,
        output_tokens: 101,
        cache_creation_tokens: 4,
        cache_read_tokens: 278272,
        cache_creation_5m_tokens: 0,
        cache_creation_1h_tokens: 0,
        image_count: 0,
        image_size: null,
        first_token_ms: 12,
        duration_ms: 345,
        created_at: '2026-03-08T00:00:00Z',
        model: 'gpt-5.4',
        reasoning_effort: null,
        api_key: { name: 'demo-key' },
      },
    ]

    query.mockResolvedValue({
      items: exportedLogs,
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    let exportedBlob: Blob | null = null
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    window.URL.createObjectURL = vi.fn((blob: Blob | MediaSource) => {
      exportedBlob = blob as Blob
      return 'blob:usage-export'
    }) as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = vi.fn(() => {}) as typeof window.URL.revokeObjectURL
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState
    await setupState.exportToCSV()

    expect(exportedBlob).not.toBeNull()
    const hasSortedExportQuery = query.mock.calls.some((call) => {
      const params = call[0] as Record<string, unknown> | undefined
      const config = call[1]
      return (
        params?.page_size === 100 &&
        params?.sort_by === 'created_at' &&
        params?.sort_order === 'desc' &&
        config === undefined
      )
    })
    expect(hasSortedExportQuery).toBe(true)
    expect(clickSpy).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    clickSpy.mockRestore()
  })
})
