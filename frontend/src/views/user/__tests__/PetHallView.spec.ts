import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PetHallView from '@/views/user/PetHallView.vue'

const { petState, getAvailable, showError, showSuccess } = vi.hoisted(() => {
  const preferences = {
    selected_asset_id: 'pet-1',
    assistant_group_id: null as number | null,
    assistant_model: '',
    enabled: true,
    size: 'medium' as const,
    anchor: 'bottom-right' as const,
    reduced_motion: false,
    activity_reactions: true,
    position_x: null,
    position_y: null,
  }
  const assets = [
    {
      id: 'pet-1', pet_key: 'shinobu', display_name: '蝴蝶忍', description: '',
      sprite_version: 1 as const, sha256: 'a', size_bytes: 1, width: 1536,
      height: 1872, license: 'Personal non-commercial use', created_at: '',
      asset_url: '/pets/shinobu.webp', is_builtin: true,
    },
    {
      id: 'pet-2', pet_key: 'custom', display_name: '自定义宠物', description: '',
      sprite_version: 2 as const, sha256: 'b', size_bytes: 1, width: 1536,
      height: 2288, license: '', created_at: '', asset_url: '/pet/custom', is_builtin: false,
    },
  ]
  return {
    petState: {
      preferences,
      assets,
      selectedAsset: assets[0],
      initialized: true,
      loading: false,
      initialize: vi.fn(),
      savePreferences: vi.fn(async (next: typeof preferences) => {
        Object.assign(preferences, next)
        return next
      }),
      importAsset: vi.fn(),
      removeAsset: vi.fn(),
    },
    getAvailable: vi.fn(),
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }
})

vi.mock('@/stores/pet', () => ({ usePetStore: () => petState }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/api/groups', () => ({ default: { getAvailable } }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('PetHallView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    petState.preferences.assistant_group_id = null
    petState.preferences.assistant_model = 'stale-model'
    getAvailable.mockResolvedValue([
      { id: 7, name: 'GPT 兜底分组', platform: 'openai', status: 'active' },
    ])
  })

  it('renders a grid and persists the selected billed assistant group', async () => {
    const wrapper = mount(PetHallView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          PetHallPreview: { template: '<div data-testid="pet-preview" />' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.findAll('[data-testid="pet-card"]')).toHaveLength(2)
    expect(wrapper.text()).toContain('蝴蝶忍')
    expect(wrapper.text()).not.toContain('Personal non-commercial use')
    await wrapper.get('[data-testid="assistant-group-select"]').setValue('7')
    await flushPromises()

    expect(petState.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      assistant_group_id: 7,
      assistant_model: '',
    }))
  })
})
