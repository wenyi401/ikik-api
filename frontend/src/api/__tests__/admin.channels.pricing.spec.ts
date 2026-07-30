import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('../client', () => ({
  apiClient: {
    get,
  },
}))

import { syncPricingModels } from '@/api/admin/channels'

describe('admin channel pricing API', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('fetches official pricing models for the selected platform', async () => {
    get.mockResolvedValue({ data: { models: ['gpt-5.6'] } })

    const result = await syncPricingModels('openai')

    expect(get).toHaveBeenCalledWith('/admin/channels/pricing/sync-models', {
      params: { platform: 'openai' },
    })
    expect(result.models).toEqual(['gpt-5.6'])
  })
})
