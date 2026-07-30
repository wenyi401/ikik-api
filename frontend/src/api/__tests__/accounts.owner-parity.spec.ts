import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put, delete: del }
}))

import {
  anthropicCookieAuth,
  anthropicSetupTokenCookieAuth,
  bulkRecoverState,
  exchangeAnthropicOAuthCode,
  exchangeAnthropicSetupTokenCode,
  generateAnthropicOAuthUrl,
  generateAnthropicSetupTokenUrl,
  importCredentialContents,
  importData,
  deleteOllamaCloudUsageSession,
  getOllamaCloudUsage,
  queryOpenAIQuota,
  refreshOllamaCloudUsage,
  recoverState,
  resetOpenAIQuota,
  saveOllamaCloudUsageSession,
  setOllamaCloudUsageAutoRefresh
} from '@/api/accounts'

describe('user-owned account parity API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    del.mockReset()
    vi.spyOn(globalThis.crypto, 'randomUUID').mockReturnValue('11111111-1111-4111-8111-111111111111')
  })

  it('uses owner-scoped state recovery endpoints', async () => {
    post
      .mockResolvedValueOnce({ data: { id: 7 } })
      .mockResolvedValueOnce({ data: { success: 2, failed: 0, results: [] } })

    await recoverState(7)
    await bulkRecoverState([7, 8])

    expect(post).toHaveBeenNthCalledWith(1, '/accounts/7/recover-state')
    expect(post).toHaveBeenNthCalledWith(
      2,
      '/accounts/batch-recover-state',
      { account_ids: [7, 8] },
      { timeout: 120000 }
    )
  })

  it('uses owner-scoped data import and preserves Claude Web options', async () => {
    post
      .mockResolvedValueOnce({ data: { account_created: 1, account_failed: 0 } })
      .mockResolvedValueOnce({ data: { total: 1, created: 1, failed: 0, errors: [] } })

    const importPayload = {
      data: { accounts: [], proxies: [], exported_at: '2026-07-22T00:00:00Z' },
      account_defaults: { concurrency: 3, priority: 5, auto_pause_on_expired: true }
    }
    await importData(importPayload)
    await importCredentialContents({
      contents: ['session-key'],
      claude_web_import: true,
      claude_web_auth_mode: 'session_key',
      share_mode: 'private',
      proxy_id: 17
    })

    expect(post).toHaveBeenNthCalledWith(1, '/accounts/data', importPayload, {
      headers: {
        'Idempotency-Key': 'user-account-import-data-11111111-1111-4111-8111-111111111111'
      }
    })
    expect(post).toHaveBeenNthCalledWith(2, '/accounts/import-credentials', {
      contents: ['session-key'],
      claude_web_import: true,
      claude_web_auth_mode: 'session_key',
      share_mode: 'private',
      proxy_id: 17
    }, {
      headers: {
        'Idempotency-Key': 'user-account-import-credentials-11111111-1111-4111-8111-111111111111'
      }
    })
  })

  it('creates a new idempotency key for each logical import call', async () => {
    vi.mocked(globalThis.crypto.randomUUID)
      .mockReturnValueOnce('22222222-2222-4222-8222-222222222222')
      .mockReturnValueOnce('33333333-3333-4333-8333-333333333333')
    post.mockResolvedValue({ data: { account_created: 0, account_failed: 0 } })

    const payload = { data: { accounts: [], proxies: [], exported_at: '2026-07-22T00:00:00Z' } }
    await importData(payload)
    await importData(payload)

    expect(post.mock.calls[0][2].headers['Idempotency-Key']).toBe(
      'user-account-import-data-22222222-2222-4222-8222-222222222222'
    )
    expect(post.mock.calls[1][2].headers['Idempotency-Key']).toBe(
      'user-account-import-data-33333333-3333-4333-8333-333333333333'
    )
  })

  it('uses owner-scoped OpenAI quota endpoints', async () => {
    get.mockResolvedValueOnce({ data: { fetched_at: 1 } })
    post.mockResolvedValueOnce({ data: { code: 'ok', windows_reset: 1 } })

    await queryOpenAIQuota(11)
    await resetOpenAIQuota(11)

    expect(get).toHaveBeenCalledWith('/accounts/11/quota')
    expect(post).toHaveBeenCalledWith('/accounts/11/reset-quota')
  })

  it('uses owner-scoped Claude OAuth and Setup Token endpoints', async () => {
    post.mockResolvedValue({ data: { auth_url: 'https://claude.ai/oauth', session_id: 'session-1' } })

    await generateAnthropicOAuthUrl({ proxy_id: 17 })
    await generateAnthropicSetupTokenUrl({ proxy_id: 18 })
    await exchangeAnthropicOAuthCode({ session_id: 'session-1', code: 'oauth-code', proxy_id: 17 })
    await exchangeAnthropicSetupTokenCode({ session_id: 'session-2', code: 'setup-code', proxy_id: 18 })
    await anthropicCookieAuth({ code: 'session-key-1', proxy_id: 17 })
    await anthropicSetupTokenCookieAuth({ code: 'session-key-2', proxy_id: 18 })

    expect(post).toHaveBeenNthCalledWith(1, '/account-oauth/anthropic/auth-url', { proxy_id: 17 })
    expect(post).toHaveBeenNthCalledWith(2, '/account-oauth/anthropic/setup-token/auth-url', { proxy_id: 18 })
    expect(post).toHaveBeenNthCalledWith(3, '/account-oauth/anthropic/exchange-code', {
      session_id: 'session-1',
      code: 'oauth-code',
      proxy_id: 17
    })
    expect(post).toHaveBeenNthCalledWith(4, '/account-oauth/anthropic/setup-token/exchange-code', {
      session_id: 'session-2',
      code: 'setup-code',
      proxy_id: 18
    })
    expect(post).toHaveBeenNthCalledWith(5, '/account-oauth/anthropic/cookie-auth', {
      code: 'session-key-1',
      proxy_id: 17
    })
    expect(post).toHaveBeenNthCalledWith(6, '/account-oauth/anthropic/setup-token-cookie-auth', {
      code: 'session-key-2',
      proxy_id: 18
    })
  })

  it('uses owner-scoped Ollama Cloud usage endpoints', async () => {
    const state = { account_id: 11, eligible: true, configured: true }
    get.mockResolvedValueOnce({ data: state })
    put.mockResolvedValue({ data: state })
    del.mockResolvedValueOnce({ data: state })
    post.mockResolvedValueOnce({ data: state })

    await getOllamaCloudUsage(11)
    await saveOllamaCloudUsageSession(11, 'wos-session=value')
    await setOllamaCloudUsageAutoRefresh(11, true)
    await deleteOllamaCloudUsageSession(11)
    await refreshOllamaCloudUsage(11)

    expect(get).toHaveBeenCalledWith('/accounts/11/ollama-cloud-usage')
    expect(put).toHaveBeenNthCalledWith(1, '/accounts/11/ollama-cloud-usage/session', {
      session: 'wos-session=value'
    })
    expect(put).toHaveBeenNthCalledWith(2, '/accounts/11/ollama-cloud-usage/auto-refresh', {
      enabled: true
    })
    expect(del).toHaveBeenCalledWith('/accounts/11/ollama-cloud-usage/session')
    expect(post).toHaveBeenCalledWith('/accounts/11/ollama-cloud-usage/refresh')
  })
})
