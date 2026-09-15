/**
 * Axios HTTP Client Configuration
 * Base client with interceptors for authentication, token refresh, and error handling
 */

import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig, AxiosResponse } from 'axios'
import type { ApiResponse } from '@/types'
import { getLocale } from '@/i18n'
import { getAccessToken, refreshAuthentication } from './authSession'
import {
  ADMIN_UI_REQUEST_HEADER,
  USER_UI_REQUEST_HEADER,
  shouldMarkAdminUIRequest,
  shouldMarkUserUIRequest
} from './adminUIRequest'
import { getAPIBaseURL } from './url'
export { buildApiUrl, buildGatewayUrl } from './url'

// ==================== Axios Instance Configuration ====================

export const apiClient: AxiosInstance = axios.create({
  baseURL: getAPIBaseURL(),
  withCredentials: true,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// ==================== Request Interceptor ====================

// UI markers let the backend distinguish admin-console traffic from user traffic
// (used by compliance auditing and Server-Timing allowlists).
function applyUIRequestHeaders(config: InternalAxiosRequestConfig): void {
  if (!config.headers) return
  const requestURL = String(config.url || '')
  if (shouldMarkAdminUIRequest(requestURL)) {
    config.headers[ADMIN_UI_REQUEST_HEADER] = '1'
  }
  if (shouldMarkUserUIRequest(requestURL)) {
    config.headers[USER_UI_REQUEST_HEADER] = '1'
  }
}

// Get user's timezone
const getUserTimezone = (): string => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    applyUIRequestHeaders(config)

  const token = getAccessToken()
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`
    }

    // Attach locale for backend translations
    if (config.headers) {
      config.headers['Accept-Language'] = getLocale()
    }

    // Attach timezone for all GET requests (backend may use it for default date ranges)
    if (config.method === 'get') {
      if (!config.params) {
        config.params = {}
      }
      config.params.timezone = getUserTimezone()
    }

    return config
  },
  (error: AxiosError) => {
    return Promise.reject(error)
  }
)

// ==================== Response Interceptor ====================

apiClient.interceptors.response.use(
  (response: AxiosResponse) => {
    // Unwrap standard API response format { code, message, data }
    const apiResponse = response.data as ApiResponse<unknown>
    if (apiResponse && typeof apiResponse === 'object' && 'code' in apiResponse) {
      if (apiResponse.code === 0) {
        // Success - return the data portion
        response.data = apiResponse.data
      } else {
        // API error
        const resp = apiResponse as unknown as Record<string, unknown>
        return Promise.reject({
          status: response.status,
          code: apiResponse.code,
          message: apiResponse.message || 'Unknown error',
          reason: resp.reason,
          metadata: resp.metadata,
        })
      }
    }
    return response
  },
  async (error: AxiosError<ApiResponse<unknown>>) => {
    // Request cancellation: keep the original axios cancellation error so callers can ignore it.
    // Otherwise we'd misclassify it as a generic "network error".
    if (error.code === 'ERR_CANCELED' || axios.isCancel(error)) {
      return Promise.reject(error)
    }

    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean }

    // Handle common errors
    if (error.response) {
      const { status, data } = error.response
      const url = String(error.config?.url || '')

      // Validate `data` shape to avoid HTML error pages breaking our error handling.
      const apiData = (typeof data === 'object' && data !== null ? data : {}) as Record<string, any>

      // Ops monitoring disabled: treat as feature-flagged 404, and proactively redirect away
      // from ops pages to avoid broken UI states.
      if (status === 404 && apiData.message === 'Ops monitoring is disabled') {
        try {
          localStorage.setItem('ops_monitoring_enabled_cached', 'false')
        } catch {
          // ignore localStorage failures
        }
        try {
          window.dispatchEvent(new CustomEvent('ops-monitoring-disabled'))
        } catch {
          // ignore event failures
        }

        if (window.location.pathname.startsWith('/admin/ops')) {
          window.location.href = '/admin/settings'
        }

        return Promise.reject({
          status,
          code: 'OPS_DISABLED',
          message: apiData.message || error.message,
          url
        })
      }

      if (status === 423 && apiData.code === 'ADMIN_COMPLIANCE_ACK_REQUIRED') {
        try {
          window.dispatchEvent(
            new CustomEvent('admin-compliance-required', {
              detail: apiData.metadata || {},
            })
          )
        } catch {
          // Ignore event failures and preserve the structured API error below.
        }

        return Promise.reject({
          status,
          code: apiData.code,
          message: apiData.message || error.message,
          metadata: apiData.metadata,
        })
      }

      // A confirmed refresh-cookie 401 is the only condition that signs the user out.
      if (status === 401 && !originalRequest._retry) {
        const isAuthEndpoint =
          url.includes('/auth/login') ||
          url.includes('/auth/register') ||
          url.includes('/auth/refresh') ||
          url.includes('/auth/logout')
        if (!isAuthEndpoint) {
          originalRequest._retry = true
          const outcome = await refreshAuthentication()
          if (outcome.kind === 'authenticated') {
            if (originalRequest.headers) {
              originalRequest.headers.Authorization = `Bearer ${outcome.bundle.access_token}`
            }
            return apiClient(originalRequest)
          }
          if (outcome.kind === 'transient_error') {
            return Promise.reject({
              status: 0,
              code: 'TOKEN_REFRESH_DEFERRED',
              message: 'Authentication is temporarily unavailable. Please retry shortly.'
            })
          }
          if (outcome.kind === 'out_of_sync') {
            // 刷新期间会话已被切换（换号/他标签页登录）：旧请求必须失败，
            // 但不能清除新会话。
            return Promise.reject({
              status: 0,
              code: 'AUTH_SESSION_CHANGED',
              message: 'Authentication session changed during refresh.'
            })
          }
          if (outcome.kind === 'anonymous') {
            sessionStorage.setItem('auth_expired', '1')
            if (!window.location.pathname.includes('/login')) {
              window.location.href = '/login'
            }
          }
        }
      }

      // Return structured error
      return Promise.reject({
        status,
        code: apiData.code,
        reason: apiData.reason,
        error: apiData.error,
        message: apiData.message || apiData.detail || error.message,
        metadata: apiData.metadata,
      })
    }

    if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') {
      return Promise.reject({
        status: 0,
        code: error.code,
        message: 'Request timed out. Please try again later.'
      })
    }

    // Network error
    return Promise.reject({
      status: 0,
      message: 'Network error. Please check your connection.'
    })
  }
)

export default apiClient
