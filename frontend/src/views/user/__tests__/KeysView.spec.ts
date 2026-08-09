import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'

import type { ApiKey } from '@/types'
import KeysView from '../KeysView.vue'

const {
  listKeys,
  createKey,
  updateKey,
  getPublicSettings,
  getDashboardApiKeysUsage,
  getAvailableGroups,
  getUserGroupRates,
  showError,
  showSuccess,
  copyToClipboard,
  isCurrentStep,
  nextStep,
  completeMission,
  setMissionPanelOpen,
  routerPush,
  routerReplace,
  startPageTutorial,
  authUser,
} = vi.hoisted(() => ({
  listKeys: vi.fn(),
  createKey: vi.fn(),
  updateKey: vi.fn(),
  getPublicSettings: vi.fn(),
  getDashboardApiKeysUsage: vi.fn(),
  getAvailableGroups: vi.fn(),
  getUserGroupRates: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  copyToClipboard: vi.fn(),
  isCurrentStep: vi.fn(),
  nextStep: vi.fn(),
  completeMission: vi.fn(),
  setMissionPanelOpen: vi.fn(),
  routerPush: vi.fn(),
  routerReplace: vi.fn(),
  startPageTutorial: vi.fn(),
  authUser: {
    id: 1,
    onboarding_mode: 'beginner',
    openai_experimental_prompt_unlocked: false,
  },
}))

const messages: Record<string, string> = {
  'common.actions': 'Actions',
  'common.name': 'Name',
  'common.refresh': 'Refresh',
  'common.status': 'Status',
  'keys.apiKey': 'API Key',
  'keys.allGroups': 'All Groups',
  'keys.allStatus': 'All Status',
  'keys.columnSettings': 'Column Settings',
  'keys.createKey': 'Create API Key',
  'keys.created': 'Created',
  'keys.expiresAt': 'Expires',
  'keys.group': 'Group',
  'keys.imageGeneration': 'Create image',
  'keys.id': 'ID',
  'keys.currentConcurrency': 'Current Concurrency',
  'keys.lastUsedAt': 'Last Used',
  'keys.lastUsedIP': 'Last Used IP',
  'keys.rateLimitColumn': 'Rate Limit',
  'keys.searchPlaceholder': 'Search name or key...',
  'keys.status.active': 'Active',
  'keys.status.expired': 'Expired',
  'keys.status.inactive': 'Inactive',
  'keys.status.quota_exhausted': 'Quota exhausted',
  'keys.usage': 'Usage',
}

vi.mock('@/api', () => ({
  keysAPI: {
    list: listKeys,
    create: createKey,
    update: updateKey,
    delete: vi.fn(),
    toggleStatus: vi.fn(),
  },
  authAPI: {
    getPublicSettings,
  },
  usageAPI: {
    getDashboardApiKeysUsage,
  },
  userGroupsAPI: {
    getAvailable: getAvailableGroups,
    getUserGroupRates,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep,
    nextStep,
    completeMission,
    setMissionPanelOpen,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: authUser,
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/keys', query: {} }),
  useRouter: () => ({ push: routerPush, replace: routerReplace }),
}))

vi.mock('@/composables/usePageTutorial', () => ({
  usePageTutorial: () => ({ startPageTutorial }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
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

const createApiKey = (): ApiKey => ({
  id: 1,
  user_id: 1,
  key: 'sk-test-key',
  name: 'test-key',
  group_id: null,
  openai_experimental_prompt_enabled: false,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  last_used_ip: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-06-27T00:00:00Z',
  updated_at: '2026-06-27T00:00:00Z',
  current_concurrency: 3,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
})

const AppLayoutStub = {
  template: '<div><slot /></div>',
}

const TablePageLayoutStub = {
  template: `
    <div>
      <slot name="filters" />
      <slot name="actions" />
      <slot name="table" />
      <slot name="pagination" />
    </div>
  `,
}

const DataTableStub = {
  name: 'DataTable',
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map((col) => col.key).join(',') }}</div>
      <div data-test="columns-meta">{{ JSON.stringify(columns.map((col) => ({ key: col.key, sortable: !!col.sortable }))) }}</div>
      <button data-test="sort-current-concurrency" @click="$emit('sort', 'current_concurrency', 'asc')">
        Sort Current Concurrency
      </button>
      <div v-for="row in data" :key="row.id">
        <div
          v-if="columns.some((col) => col.key === 'id')"
          data-test="key-id"
        >
          <slot name="cell-id" :value="row.id" :row="row" />
        </div>
        <slot name="cell-name" :value="row.name" :row="row" />
        <div data-test="current-concurrency">
          <slot name="cell-current_concurrency" :value="row.current_concurrency" :row="row" />
        </div>
        <div data-test="key-actions">
          <slot name="cell-actions" :row="row" />
        </div>
        <div
          v-if="columns.some((col) => col.key === 'last_used_ip')"
          data-test="last-used-ip"
        >
          <slot name="cell-last_used_ip" :value="row.last_used_ip" :row="row" />
        </div>
      </div>
      <slot name="empty" />
    </div>
  `,
}

const SelectStub = {
  name: 'Select',
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"></select>',
}

const SearchInputStub = {
  name: 'SearchInput',
  props: ['modelValue'],
  emits: ['update:modelValue', 'search'],
  template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
}

const PaginationStub = {
  name: 'Pagination',
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div>
      <button data-test="page-size-50" @click="$emit('update:pageSize', 50)">50</button>
    </div>
  `,
}

const IconStub = {
  props: ['name'],
  template: '<span data-test="icon">{{ name }}</span>',
}

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const mountView = async () => {
  const wrapper = mount(KeysView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: PaginationStub,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: SelectStub,
        SearchInput: SearchInputStub,
        Icon: IconStub,
        UseKeyModal: true,
        EndpointPopover: true,
        GroupBadge: true,
        GroupOptionItem: true,
        Teleport: true,
      },
    },
  })
  await flushPromises()
  await nextTick()
  return wrapper
}

const visibleColumnKeys = (wrapper: VueWrapper) =>
  wrapper.get('[data-test="columns"]').text().split(',').filter(Boolean)

const visibleColumnMeta = (wrapper: VueWrapper): Array<{ key: string; sortable: boolean }> =>
  JSON.parse(wrapper.get('[data-test="columns-meta"]').text())

const getButtonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) {
    throw new Error(`Button not found: ${text}`)
  }
  return button
}

describe('user KeysView column settings', () => {
  beforeEach(() => {
    localStorage.clear()

    listKeys.mockReset()
    createKey.mockReset()
    updateKey.mockReset()
    getPublicSettings.mockReset()
    getDashboardApiKeysUsage.mockReset()
    getAvailableGroups.mockReset()
    getUserGroupRates.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    copyToClipboard.mockReset()
    isCurrentStep.mockReset()
    nextStep.mockReset()
    completeMission.mockReset()
    setMissionPanelOpen.mockReset()
    routerPush.mockReset()
    routerReplace.mockReset()
    startPageTutorial.mockReset()
    authUser.openai_experimental_prompt_unlocked = false

    listKeys.mockResolvedValue({
      items: [createApiKey()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getPublicSettings.mockResolvedValue({})
    getDashboardApiKeysUsage.mockResolvedValue({ stats: {} })
    getAvailableGroups.mockResolvedValue([])
    getUserGroupRates.mockResolvedValue({})
    createKey.mockResolvedValue(createApiKey())
    updateKey.mockResolvedValue(createApiKey())
    isCurrentStep.mockReturnValue(false)
  })

  it('uses the default API key columns with low-frequency columns hidden', async () => {
    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'group',
      'current_concurrency',
      'usage',
      'expires_at',
      'status',
      'created_at',
      'actions',
    ])
    expect(visibleColumnKeys(wrapper)).not.toContain('rate_limit')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_at')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_ip')
    expect(visibleColumnKeys(wrapper)).not.toContain('id')
  })

  it('shows a hidden column when toggled and persists the preference', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Rate Limit').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('rate_limit')
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['id', 'last_used_at', 'last_used_ip'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('shows the API key ID column when toggled', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'ID').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('id')
    expect(wrapper.get('[data-test="key-id"]').text()).toBe('#1')
    expect(visibleColumnMeta(wrapper).find((column) => column.key === 'id')?.sortable).toBe(true)
  })

  it('shows the last used IP column when toggled', async () => {
    listKeys.mockResolvedValueOnce({
      items: [{ ...createApiKey(), last_used_ip: '203.0.113.10' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Last Used IP').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('last_used_ip')
    expect(wrapper.get('[data-test="last-used-ip"]').text()).toBe('203.0.113.10')
  })

  it('restores column preferences from localStorage on mount', async () => {
    localStorage.setItem('api-key-hidden-columns', JSON.stringify(['group', 'created_at']))
    localStorage.setItem('api-key-column-settings-version', '1')

    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'current_concurrency',
      'usage',
      'rate_limit',
      'expires_at',
      'status',
      'last_used_at',
      'actions',
    ])
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['group', 'created_at', 'last_used_ip', 'id'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('does not include always-visible columns in the toggleable menu', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await nextTick()

    const columnMenuText = wrapper.text()
    expect(columnMenuText).toContain('API Key')
    expect(columnMenuText).toContain('ID')
    expect(columnMenuText).toContain('Current Concurrency')
    expect(columnMenuText).toContain('Rate Limit')
    expect(columnMenuText).toContain('Last Used IP')
    expect(columnMenuText).not.toContain('Name')
    expect(columnMenuText).not.toContain('Actions')
  })

  it('renders the current concurrency value', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-test="current-concurrency"]').text()).toBe('3')
  })

  it('always shows image generation for an API key', async () => {
    listKeys.mockResolvedValueOnce({
      items: [createApiKey()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = await mountView()

    expect(wrapper.get('[data-test="key-actions"]').text()).toContain('Create image')
  })

  it('marks current concurrency as sortable', async () => {
    const wrapper = await mountView()

    const currentConcurrencyColumn = visibleColumnMeta(wrapper).find(
      (column) => column.key === 'current_concurrency'
    )
    expect(currentConcurrencyColumn?.sortable).toBe(true)
  })

  it('keeps filters and selected page size when sorting by current concurrency', async () => {
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'OpenAI' }])
    const wrapper = await mountView()

    await wrapper.get('[data-test="page-size-50"]').trigger('click')
    await flushPromises()

    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('update:modelValue', 'target')
    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
    await flushPromises()

    const selects = wrapper.findAllComponents({ name: 'Select' })
    await selects[0].vm.$emit('update:modelValue', 42)
    await flushPromises()
    await selects[1].vm.$emit('update:modelValue', 'active')
    await flushPromises()

    listKeys.mockClear()

    await wrapper.get('[data-test="sort-current-concurrency"]').trigger('click')
    await flushPromises()

    expect(listKeys).toHaveBeenLastCalledWith(
      1,
      50,
      {
        search: 'target',
        status: 'active',
        group_id: 42,
        sort_by: 'current_concurrency',
        sort_order: 'asc',
      },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })

  it('collapses platform-specific private groups into one private router option', async () => {
    getAvailableGroups.mockResolvedValue([
      {
        id: 31,
        name: 'Kiro Private',
        platform: 'kiro',
        scope: 'user_private',
        subscription_type: 'subscription',
        rate_multiplier: 1,
      },
      {
        id: 11,
        name: 'OpenAI Private',
        platform: 'openai',
        scope: 'user_private',
        subscription_type: 'subscription',
        rate_multiplier: 1,
      },
      {
        id: 21,
        name: 'Anthropic Private',
        platform: 'anthropic',
        scope: 'user_private',
        subscription_type: 'subscription',
        rate_multiplier: 1,
      },
      {
        id: 99,
        name: 'Public Group',
        platform: 'openai',
        scope: 'public',
        subscription_type: 'standard',
        rate_multiplier: 1,
      },
    ])

    const wrapper = await mountView()
    const vm = wrapper.vm as unknown as {
      groupOptions: Array<{ value: number; label: string; scope: string }>
      privateRouterRouteForms: () => Array<{ group_id: number; priority: number }>
      showSpecificPrivateGroups: boolean
      isPrivateRouterRoutes: (routes: Array<{ group_id: number }>) => boolean
    }

    expect(vm.groupOptions.map((option) => option.label)).toEqual([
      'keys.privateRouter.title',
      'Public Group',
    ])
    expect(vm.groupOptions.filter((option) => option.scope === 'user_private')).toHaveLength(1)
    expect(vm.privateRouterRouteForms().map((route) => [route.group_id, route.priority])).toEqual([
      [21, 100],
      [11, 101],
      [31, 102],
    ])

    vm.showSpecificPrivateGroups = true
    await nextTick()
    expect(vm.groupOptions.map((option) => option.label)).toEqual([
      'keys.privateRouter.title',
      'Anthropic Private',
      'OpenAI Private',
      'Kiro Private',
      'Public Group',
    ])
    expect(vm.isPrivateRouterRoutes([{ group_id: 11 }])).toBe(false)
    expect(vm.isPrivateRouterRoutes([{ group_id: 21 }, { group_id: 11 }, { group_id: 31 }])).toBe(true)
  })

  it('defaults new keys to the PLUS shared pool even when assigned PRO is available', async () => {
    getAvailableGroups.mockResolvedValue([
      {
        id: 16,
        name: 'gpt pro shared pool',
        platform: 'openai',
        scope: 'public',
        subscription_type: 'standard',
        rate_multiplier: 1,
        is_exclusive: true,
        is_shared_pool: true,
        required_account_level: 'pro',
      },
      {
        id: 6,
        name: 'gpt plus shared pool',
        platform: 'openai',
        scope: 'public',
        subscription_type: 'standard',
        rate_multiplier: 1,
        is_exclusive: false,
        is_shared_pool: true,
        required_account_level: 'plus',
      },
    ])

    const wrapper = await mountView()
    const vm = wrapper.vm as unknown as {
      recommendedGroup: { id: number; name: string } | null
      groupOptions: Array<{ value: number; recommended?: boolean }>
      formData: { group_id: number | null }
      openCreateModal: () => void
    }

    expect(vm.recommendedGroup?.id).toBe(6)
    expect(vm.groupOptions[0]).toMatchObject({ value: 6, recommended: true })

    vm.openCreateModal()
    await nextTick()
    expect(vm.formData.group_id).toBe(6)
  })

  it('shows an unlock action instead of a toggle for a locked eligible group', async () => {
    getAvailableGroups.mockResolvedValue([{
      id: 1,
      name: 'OpenAI Pro',
      description: '',
      platform: 'openai',
      scope: 'public',
      subscription_type: 'standard',
      rate_multiplier: 1,
      is_shared_pool: true,
      required_account_level: 'pro',
      openai_experimental_prompt_enabled: true,
    }])
    const wrapper = await mountView()
    const vm = wrapper.vm as unknown as {
      openCreateModal: () => void
      formData: { group_id: number | null }
    }
    vm.openCreateModal()
    vm.formData.group_id = 1
    await nextTick()

    expect(wrapper.find('[data-test="openai-experimental-prompt-setting"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="openai-experimental-prompt-unlock"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="openai-experimental-prompt-toggle"]').exists()).toBe(false)
  })

  it('lets an unlocked user opt in per key and clears it for an unsupported route', async () => {
    authUser.openai_experimental_prompt_unlocked = true
    getAvailableGroups.mockResolvedValue([
      {
        id: 1,
        name: 'OpenAI Pro',
        description: '',
        platform: 'openai',
        scope: 'public',
        subscription_type: 'standard',
        rate_multiplier: 1,
        is_shared_pool: true,
        required_account_level: 'pro',
        openai_experimental_prompt_enabled: true,
      },
      {
        id: 2,
        name: 'Claude',
        description: '',
        platform: 'anthropic',
        scope: 'public',
        subscription_type: 'standard',
        rate_multiplier: 1,
        openai_experimental_prompt_enabled: false,
      },
    ])
    const wrapper = await mountView()
    const vm = wrapper.vm as unknown as {
      openCreateModal: () => void
      formData: {
        group_id: number | null
        name: string
        openai_experimental_prompt_enabled: boolean
      }
      handleSubmit: () => Promise<void>
    }
    vm.openCreateModal()
    vm.formData.group_id = 1
    await nextTick()

    await wrapper.get('[data-test="openai-experimental-prompt-toggle"]').trigger('click')
    expect(vm.formData.openai_experimental_prompt_enabled).toBe(true)

    vm.formData.name = 'Experimental key'
    await vm.handleSubmit()
    expect(createKey).toHaveBeenLastCalledWith(
      'Experimental key',
      1,
      undefined,
      [],
      [],
      0,
      undefined,
      { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 },
      expect.any(Array),
      true,
    )

    vm.openCreateModal()
    vm.formData.group_id = 1
    await nextTick()
    vm.formData.openai_experimental_prompt_enabled = true
    vm.formData.group_id = 2
    await nextTick()
    expect(vm.formData.openai_experimental_prompt_enabled).toBe(false)
    expect(wrapper.find('[data-test="openai-experimental-prompt-setting"]').exists()).toBe(false)
  })
})
