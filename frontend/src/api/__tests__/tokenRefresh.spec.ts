import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AuthResponse } from '@/types'

const refreshAuthentication = vi.fn()

vi.mock('@/api/authSession', () => ({ refreshAuthentication }))

function bundle(): AuthResponse {
  return {
    access_token: 'new-access',
    expires_in: 900,
    access_expires_at: Math.floor(Date.now() / 1000) + 900,
    token_type: 'Bearer',
    session: {
      sid: '11111111-1111-4111-8111-111111111111',
      current: true,
      login_method: 'password',
      ip: '127.0.0.1',
      user_agent: 'vitest',
      created_at: 1,
      last_active_at: 1,
      expires_at: Math.floor(Date.now() / 1000) + 30 * 86400
    },
    user: { id: 7, email: 'admin@example.com', role: 'admin', status: 'active' }
  } as AuthResponse
}

describe('refreshAuthTokens compatibility wrapper', () => {
  beforeEach(() => refreshAuthentication.mockReset())

  it('returns the memory-only bundle produced by the shared refresh state machine', async () => {
    const expected = bundle()
    refreshAuthentication.mockResolvedValue({ kind: 'authenticated', bundle: expected })
    const { refreshAuthTokens } = await import('@/api/tokenRefresh')

    await expect(refreshAuthTokens({ failedAccessToken: 'expired' })).resolves.toEqual(expected)
    expect(refreshAuthentication).toHaveBeenCalledTimes(1)
  })

  it('preserves and surfaces transient refresh failures', async () => {
    const failure = new Error('temporarily unavailable')
    refreshAuthentication.mockResolvedValue({ kind: 'transient_error', error: failure })
    const { refreshAuthTokens } = await import('@/api/tokenRefresh')

    await expect(refreshAuthTokens()).rejects.toBe(failure)
  })

  it.each(['anonymous', 'out_of_sync'] as const)('rejects %s outcomes as expired sessions', async (kind) => {
    refreshAuthentication.mockResolvedValue({ kind })
    const { refreshAuthTokens } = await import('@/api/tokenRefresh')

    await expect(refreshAuthTokens()).rejects.toThrow('Session expired')
  })
})
