import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import type { AxiosInstance } from 'axios'
import type { AuthResponse } from '@/types'

// 需要在导入 client 之前设置 mock
vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))

function authBundle(accessToken = 'my-jwt-token'): AuthResponse {
  const now = Math.floor(Date.now() / 1000)
  return {
    access_token: accessToken, expires_in: 900, access_expires_at: now + 900, token_type: 'Bearer',
    session: { sid: '11111111-1111-4111-8111-111111111111', current: true, login_method: 'password', ip: '', user_agent: '', created_at: now, last_active_at: now, expires_at: now + 30 * 86400 },
    user: { id: 1, email: 'user@example.com', role: 'user', status: 'active' }
  } as AuthResponse
}

describe('API Client', () => {
  let apiClient: AxiosInstance

  beforeEach(async () => {
    localStorage.clear()
    // 每次测试重新导入以获取干净的模块状态
    vi.resetModules()
    const mod = await import('@/api/client')
    apiClient = mod.apiClient
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  // --- 请求拦截器 ---

  describe('请求拦截器', () => {
    it('自动附加 Authorization 头', async () => {
      const auth = await import('@/api/authSession')
      auth.acceptAuthBundle(authBundle(), false)

      // 拦截实际请求
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('Authorization')).toBe('Bearer my-jwt-token')
    })

    it('无 token 时不附加 Authorization 头', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('Authorization')).toBeFalsy()
    })

    it('GET 请求自动附加 timezone 参数', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.params).toHaveProperty('timezone')
    })

    it('POST 请求不附加 timezone 参数', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.post('/test', { foo: 'bar' })

      const config = adapter.mock.calls[0][0]
      expect(config.params?.timezone).toBeUndefined()
    })

    it('请求默认带 withCredentials 以支持跨域 cookie', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.post('/auth/oauth/bind-token')

      const config = adapter.mock.calls[0][0]
      expect(config.withCredentials).toBe(true)
    })
  })

  // --- 响应拦截器 ---

  describe('响应拦截器', () => {
    it('code=0 时解包 data 字段', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: { name: 'test' }, message: 'ok' },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      const response = await apiClient.get('/test')
      expect(response.data).toEqual({ name: 'test' })
    })

    it('code!=0 时拒绝并返回结构化错误', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 1001, message: '参数错误', data: null },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          code: 1001,
          message: '参数错误',
        })
      )
    })

    it('部署与运营合规未确认时广播事件且保留登录态', async () => {
      localStorage.setItem('auth_token', 'admin-token')
      const listener = vi.fn()
      window.addEventListener('admin-compliance-required', listener)

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 423,
          data: {
            code: 'ADMIN_COMPLIANCE_ACK_REQUIRED',
            message: 'administrator compliance acknowledgement is required',
            metadata: {
              version: 'v2026.07.18',
              document_path_zh: 'docs/legal/admin-compliance.zh.md',
              document_path_en: 'docs/legal/admin-compliance.en.md',
            },
          },
        },
        config: {
          url: '/admin/users',
          headers: { Authorization: 'Bearer admin-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/admin/users')).rejects.toEqual(
        expect.objectContaining({
          status: 423,
          code: 'ADMIN_COMPLIANCE_ACK_REQUIRED',
          metadata: expect.objectContaining({
            version: 'v2026.07.18',
          }),
        })
      )

      expect(listener).toHaveBeenCalledTimes(1)
      expect((listener.mock.calls[0][0] as CustomEvent).detail).toEqual(
        expect.objectContaining({
          version: 'v2026.07.18',
        })
      )
      expect(localStorage.getItem('auth_token')).toBe('admin-token')

      window.removeEventListener('admin-compliance-required', listener)
    })
  })

  // --- 401 Token 刷新 ---

  describe('401 Token 刷新', () => {
    it('refresh cookie 确认 401 时清除旧 Web Storage', async () => {
      localStorage.setItem('auth_token', 'expired-token')
      vi.spyOn(axios, 'post').mockRejectedValue({
        isAxiosError: true,
        response: { status: 401, data: { reason: 'AUTH_UNAUTHORIZED' } }
      })

      // Mock window.location
      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/dashboard', href: '/dashboard' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'TOKEN_EXPIRED', message: 'Token expired' },
        },
        config: {
          url: '/test',
          headers: { Authorization: 'Bearer expired-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toBeDefined()

      expect(localStorage.getItem('auth_token')).toBeNull()

      // 恢复 location
      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })

    it('refresh cookie 轮换后用新的内存 access token 重试请求', async () => {
      const auth = await import('@/api/authSession')
      auth.acceptAuthBundle(authBundle('expired-token'), false)
      vi.spyOn(axios, 'post').mockResolvedValue({
        status: 200,
        data: { code: 0, data: authBundle('new-token') }
      })

      const adapter = vi
        .fn()
        .mockRejectedValueOnce({
          response: {
            status: 401,
            data: { code: 'TOKEN_EXPIRED', message: 'Token expired' },
          },
          config: {
            url: '/test',
            headers: { Authorization: 'Bearer expired-token' },
          },
          code: 'ERR_BAD_REQUEST',
        })
        .mockResolvedValueOnce({
          status: 200,
          data: { code: 0, data: { ok: true } },
          headers: {},
          config: {},
          statusText: 'OK',
        })
      apiClient.defaults.adapter = adapter

      const response = await apiClient.get('/test')

      expect(response.data).toEqual({ ok: true })
      expect(auth.getAccessToken()).toBe('new-token')
      expect(localStorage.getItem('auth_token')).toBeNull()
      expect(localStorage.getItem('refresh_token')).toBeNull()
      expect(adapter).toHaveBeenCalledTimes(2)
      const retryConfig = adapter.mock.calls[1][0]
      expect(retryConfig.headers.get('Authorization')).toBe('Bearer new-token')
    })

    it('refresh 被限流时保留本地登录态', async () => {
      const auth = await import('@/api/authSession')
      auth.acceptAuthBundle(authBundle('expired-token'), false)

      vi.spyOn(axios, 'post').mockRejectedValue({
        isAxiosError: true,
        response: {
          status: 429,
          data: { message: 'rate limit exceeded' },
        },
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'TOKEN_EXPIRED', message: 'Token expired' },
        },
        config: {
          url: '/test',
          headers: { Authorization: 'Bearer expired-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          status: 0,
          code: 'TOKEN_REFRESH_DEFERRED',
        })
      )

      expect(auth.getAccessToken()).toBe('expired-token')
      expect(localStorage.getItem('auth_token')).toBeNull()
      expect(localStorage.getItem('refresh_token')).toBeNull()
    })

    it('/auth/logout 401 时不递归刷新或强制跳转登录页', async () => {
      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/', href: '/' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'UNAUTHORIZED', message: 'User session is not available' },
        },
        config: {
          url: '/auth/logout',
          headers: {},
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.post('/auth/logout')).rejects.toEqual(
        expect.objectContaining({
          status: 401,
          code: 'UNAUTHORIZED',
        })
      )
      expect(window.location.href).toBe('/')

      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })
  })

  // --- 网络错误 ---

  describe('网络错误', () => {
    it('网络错误返回 status 0 的错误', async () => {
      const adapter = vi.fn().mockRejectedValue({
        code: 'ERR_NETWORK',
        message: 'Network Error',
        config: { url: '/test' },
        // 没有 response
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          status: 0,
          message: 'Network error. Please check your connection.',
        })
      )
    })

    it('请求超时返回明确的超时错误', async () => {
      const adapter = vi.fn().mockRejectedValue({
        code: 'ECONNABORTED',
        message: 'timeout of 30000ms exceeded',
        config: { url: '/test' },
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          status: 0,
          code: 'ECONNABORTED',
          message: 'Request timed out. Please try again later.',
        })
      )
    })
  })

  // --- 请求取消 ---

  describe('请求取消', () => {
    it('取消的请求保持原始取消错误', async () => {
      const source = axios.CancelToken.source()

      const adapter = vi.fn().mockRejectedValue(
        new axios.Cancel('Operation canceled')
      )
      apiClient.defaults.adapter = adapter

      await expect(
        apiClient.get('/test', { cancelToken: source.token })
      ).rejects.toBeDefined()
    })
  })
})
