import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { post }
}))

import { probeModelList, probeModels } from '@/api/accounts'

describe('user account model probe API', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({ data: { models: [], results: [] } })
  })

  it('uses authenticated user routes instead of admin routes', async () => {
    const listPayload = { platform: 'openai', base_url: '', api_key: 'sk-test' }
    const testPayload = { ...listPayload, mode: 'responses', models: ['gpt-5.4'] }

    await probeModelList(listPayload)
    await probeModels(testPayload)

    expect(post).toHaveBeenNthCalledWith(1, '/accounts/model-probe/list', listPayload, { timeout: 45000 })
    expect(post).toHaveBeenNthCalledWith(2, '/accounts/model-probe/test', testPayload, { timeout: 90000 })
  })
})
