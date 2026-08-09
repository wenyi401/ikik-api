import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, deleteRequest } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  deleteRequest: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
    delete: deleteRequest
  }
}))

import {
  createDeveloperToken,
  listDeveloperTokens,
  revokeDeveloperToken
} from '@/api/developerTokens'

describe('developer tokens api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    deleteRequest.mockReset()
  })

  it('uses the JWT-authenticated token management routes', async () => {
    const token = {
      id: 1,
      user_id: 7,
      name: 'sync',
      token_prefix: 'ikd_example',
      scopes: ['accounts:read'],
      status: 'active',
      created_at: '2026-07-31T00:00:00Z',
      updated_at: '2026-07-31T00:00:00Z'
    }
    get.mockResolvedValue({ data: [token] })
    post.mockResolvedValue({ data: { developer_token: token, token: 'ikd_plaintext' } })
    deleteRequest.mockResolvedValue({ data: undefined })

    await expect(listDeveloperTokens()).resolves.toEqual([token])
    await expect(createDeveloperToken({ name: 'sync', scopes: ['accounts:read'] })).resolves.toEqual({
      developer_token: token,
      token: 'ikd_plaintext'
    })
    await revokeDeveloperToken(1)

    expect(get).toHaveBeenCalledWith('/developer-tokens')
    expect(post).toHaveBeenCalledWith('/developer-tokens', {
      name: 'sync',
      scopes: ['accounts:read']
    })
    expect(deleteRequest).toHaveBeenCalledWith('/developer-tokens/1')
  })
})
