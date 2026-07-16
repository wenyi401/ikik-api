import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => {
      const messages: Record<string, string> = {
        'admin.accounts.oauth.openai.failedToExchangeCode': 'OpenAI 授权码兑换失败',
        'admin.accounts.oauth.openai.errors.OPENAI_OAUTH_PROXY_REQUIRED':
          '未设置代理，当前服务器无法直连 OpenAI，导致 OpenAI OAuth 请求失败。请先选择可访问 OpenAI 的代理后重试；如果授权码已失效，请重新生成授权链接。'
      }
      return messages[key] ?? key
    }
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      generateAuthUrl: vi.fn(),
      exchangeCode: vi.fn(),
      refreshOpenAIToken: vi.fn()
    }
  }
}))

vi.mock('@/api/accounts', () => ({
  accountsAPI: {
    generateOpenAIOAuthUrl: vi.fn(),
    exchangeOpenAIOAuthCode: vi.fn(),
    refreshOpenAIToken: vi.fn()
  }
}))

import { useOpenAIOAuth } from '@/composables/useOpenAIOAuth'
import { adminAPI } from '@/api/admin'
import { accountsAPI } from '@/api/accounts'

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useOpenAIOAuth.generateAuthUrl', () => {
  it('uses the user-scoped OpenAI auth-url endpoint for personal accounts', async () => {
    vi.mocked(accountsAPI.generateOpenAIOAuthUrl).mockResolvedValueOnce({
      auth_url: 'https://example.com/oauth?state=user-state',
      session_id: 'user-session'
    })

    const oauth = useOpenAIOAuth('user')

    const ok = await oauth.generateAuthUrl(7, 'http://localhost:3000/auth/callback')

    expect(ok).toBe(true)
    expect(accountsAPI.generateOpenAIOAuthUrl).toHaveBeenCalledWith({
      proxy_id: 7,
      redirect_uri: 'http://localhost:3000/auth/callback'
    })
    expect(adminAPI.accounts.generateAuthUrl).not.toHaveBeenCalled()
    expect(oauth.authUrl.value).toBe('https://example.com/oauth?state=user-state')
    expect(oauth.sessionId.value).toBe('user-session')
    expect(oauth.oauthState.value).toBe('user-state')
  })

  it('keeps the admin OpenAI generate-auth-url endpoint unchanged', async () => {
    vi.mocked(adminAPI.accounts.generateAuthUrl).mockResolvedValueOnce({
      auth_url: 'https://example.com/oauth?state=admin-state',
      session_id: 'admin-session'
    })

    const oauth = useOpenAIOAuth('admin')

    const ok = await oauth.generateAuthUrl(9)

    expect(ok).toBe(true)
    expect(adminAPI.accounts.generateAuthUrl).toHaveBeenCalledWith(
      '/admin/openai/generate-auth-url',
      { proxy_id: 9 }
    )
    expect(accountsAPI.generateOpenAIOAuthUrl).not.toHaveBeenCalled()
  })
})

describe('useOpenAIOAuth.buildCredentials', () => {
  it('should keep client_id when token response contains it', () => {
    const oauth = useOpenAIOAuth()
    const creds = oauth.buildCredentials({
      access_token: 'at',
      refresh_token: 'rt',
      client_id: 'app_test_client',
      expires_at: 1700000000
    })

    expect(creds.client_id).toBe('app_test_client')
    expect(creds.access_token).toBe('at')
    expect(creds.refresh_token).toBe('rt')
  })

  it('should keep legacy behavior when client_id is missing', () => {
    const oauth = useOpenAIOAuth()
    const creds = oauth.buildCredentials({
      access_token: 'at',
      refresh_token: 'rt',
      expires_at: 1700000000
    })

    expect(Object.prototype.hasOwnProperty.call(creds, 'client_id')).toBe(false)
    expect(creds.access_token).toBe('at')
    expect(creds.refresh_token).toBe('rt')
  })
})

describe('useOpenAIOAuth.exchangeAuthCode', () => {
  it('shows a clear proxy hint when code exchange fails without a proxy', async () => {
    vi.mocked(adminAPI.accounts.exchangeCode).mockRejectedValueOnce({
      status: 502,
      reason: 'OPENAI_OAUTH_PROXY_REQUIRED',
      message: 'OpenAI OAuth token exchange failed: no proxy is configured.'
    })
    const oauth = useOpenAIOAuth()

    const tokenInfo = await oauth.exchangeAuthCode('code', 'session-id', 'state')

    expect(tokenInfo).toBeNull()
    expect(oauth.error.value).toBe(
      '未设置代理，当前服务器无法直连 OpenAI，导致 OpenAI OAuth 请求失败。请先选择可访问 OpenAI 的代理后重试；如果授权码已失效，请重新生成授权链接。'
    )
  })
})
