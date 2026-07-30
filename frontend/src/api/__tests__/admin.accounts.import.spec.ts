import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { post }
}))

import { importCredentialContents, importData } from '@/api/admin/accounts'

describe('admin account import API', () => {
  beforeEach(() => {
    post.mockReset()
    vi.spyOn(globalThis.crypto, 'randomUUID').mockReturnValue('44444444-4444-4444-8444-444444444444')
  })

  it('sends idempotency keys for data and credential imports', async () => {
    post
      .mockResolvedValueOnce({ data: { account_created: 0, account_failed: 0 } })
      .mockResolvedValueOnce({ data: { total: 1, created: 1, failed: 0, errors: [] } })

    await importData({
      data: { accounts: [], proxies: [], exported_at: '2026-07-22T00:00:00Z' },
      compatibility_mode: true
    })
    await importCredentialContents({
      contents: ['session-key'],
      claude_web_import: true,
      proxy_id: 17
    })

    expect(post).toHaveBeenNthCalledWith(1, '/admin/accounts/data', {
      data: { accounts: [], proxies: [], exported_at: '2026-07-22T00:00:00Z' },
      source_url: undefined,
      skip_default_group_bind: undefined,
      group_ids: undefined,
      compatibility_mode: true,
      account_defaults: undefined
    }, {
      headers: {
        'Idempotency-Key': 'admin-account-import-data-44444444-4444-4444-8444-444444444444'
      }
    })
    expect(post).toHaveBeenNthCalledWith(2, '/admin/accounts/import-credentials', {
      contents: ['session-key'],
      claude_web_import: true,
      proxy_id: 17
    }, {
      headers: {
        'Idempotency-Key': 'admin-account-import-credentials-44444444-4444-4444-8444-444444444444'
      }
    })
  })
})
