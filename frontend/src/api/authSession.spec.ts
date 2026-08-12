// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AuthResponse } from '@/types'

const post = vi.fn()

Object.defineProperty(window, 'BroadcastChannel', { value: undefined, configurable: true })

vi.mock('axios', () => ({
  default: {
    post,
    isAxiosError: (error: unknown) => Boolean((error as { isAxiosError?: boolean })?.isAxiosError)
  }
}))

function bundle(sid = '11111111-1111-4111-8111-111111111111'): AuthResponse {
  return {
    access_token: `access-${sid}`,
    expires_in: 900,
    access_expires_at: Math.floor(Date.now() / 1000) + 900,
    token_type: 'Bearer',
    session: {
      sid,
      current: true,
      login_method: 'password',
      ip: '203.0.113.1',
      user_agent: 'test',
      created_at: 1,
      last_active_at: 1,
      expires_at: Math.floor(Date.now() / 1000) + 30 * 86400
    },
    user: { id: 1, role: 'user', status: 'active', email: 'user@example.com' }
  } as AuthResponse
}

function axiosFailure(status: number, reason?: string): unknown {
  return {
    isAxiosError: true,
    response: { status, data: { code: status, reason, message: reason } }
  }
}

describe('authSession', () => {
  beforeEach(async () => {
    post.mockReset()
    localStorage.clear()
    const auth = await import('./authSession')
    auth.clearAuthentication(false, 'idle')
  })

  it('keeps access and refresh tokens out of Web Storage', async () => {
    const auth = await import('./authSession')
    auth.acceptAuthBundle(bundle(), false)
    expect(auth.getAccessToken()).toContain('access-')
    expect(localStorage.getItem('auth_token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
    expect(localStorage.getItem('auth_user')).toBeNull()
  })

  it('keeps cookie-backed authentication usable when Web Storage is unavailable', async () => {
    const auth = await import('./authSession')
    auth.acceptAuthBundle(bundle(), false)
    const removeItem = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new DOMException('Storage blocked', 'SecurityError')
    })
    post.mockRejectedValueOnce(axiosFailure(503))
    try {
      const outcome = await auth.refreshAuthentication()
      expect(outcome.kind).toBe('transient_error')
      expect(auth.getAccessToken()).not.toBeNull()
    } finally {
      removeItem.mockRestore()
    }
  })

  it('applies a password-change token rotation without replacing the refresh cookie', async () => {
    const auth = await import('./authSession')
    const initial = bundle()
    auth.acceptAuthBundle(initial, false)
    auth.applyAuthRotation({
      access_token: 'rotated-access',
      token_type: 'Bearer',
      access_expires_at: Math.floor(Date.now() / 1000) + 900,
      session: { ...initial.session, last_active_at: initial.session.last_active_at + 1 }
    })
    expect(auth.getAccessToken()).toBe('rotated-access')
    expect(auth.getCurrentAuthBundle()?.user).toEqual(initial.user)
    expect(localStorage.getItem('refresh_token')).toBeNull()
  })

  it('retains the current login on network, rate-limit and server failures', async () => {
    const auth = await import('./authSession')
    auth.acceptAuthBundle(bundle(), false)
    for (const status of [0, 429, 502, 503]) {
      post.mockRejectedValueOnce(axiosFailure(status))
      const outcome = await auth.refreshAuthentication()
      expect(outcome.kind).toBe('transient_error')
      expect(auth.getAccessToken()).not.toBeNull()
    }
  })

  it('clears authentication only after a confirmed refresh 401', async () => {
    const auth = await import('./authSession')
    auth.acceptAuthBundle(bundle(), false)
    post.mockRejectedValueOnce(axiosFailure(401, 'AUTH_SESSION_REVOKED'))
    const outcome = await auth.refreshAuthentication()
    expect(outcome.kind).toBe('anonymous')
    expect(auth.getAccessToken()).toBeNull()
  })

  it('retries the server refresh-race response and adopts the winner', async () => {
    const auth = await import('./authSession')
    const first = bundle()
    const winner = bundle(first.session.sid)
    winner.access_token = 'race-winner'
    auth.acceptAuthBundle(first, false)
    post
      .mockRejectedValueOnce(axiosFailure(409, 'AUTH_REFRESH_RACE'))
      .mockResolvedValueOnce({ status: 200, data: { code: 0, data: winner } })
    const outcome = await auth.refreshAuthentication()
    expect(outcome.kind).toBe('authenticated')
    expect(auth.getAccessToken()).toBe('race-winner')
  })
})
