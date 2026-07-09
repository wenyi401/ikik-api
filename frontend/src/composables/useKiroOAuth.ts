import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { accountsAPI } from '@/api/accounts'
import type { KiroTokenInfo } from '@/api/admin/kiro'
import type { AccountApiScope } from '@/composables/useAccountOAuth'

export type KiroAuthMode = 'oauth' | 'idc' | 'import'
export type KiroSocialProvider = 'Google' | 'Github'

export function useKiroOAuth(scope: AccountApiScope = 'admin') {
  const appStore = useAppStore()
  const { t } = useI18n()

  const authUrl = ref('')
  const sessionId = ref('')
  const state = ref('')
  const loading = ref(false)
  const error = ref('')

  const resetState = () => {
    authUrl.value = ''
    sessionId.value = ''
    state.value = ''
    loading.value = false
    error.value = ''
  }

  const generateAuthUrl = async (
    proxyId: number | null | undefined,
    provider: KiroSocialProvider = 'Google'
  ): Promise<boolean> => {
    loading.value = true
    error.value = ''
    authUrl.value = ''
    sessionId.value = ''
    state.value = ''

    try {
      const payload = {
        proxy_id: proxyId || undefined,
        provider
      }
      const response =
        scope === 'user'
          ? await accountsAPI.generateKiroOAuthUrl(payload)
          : await adminAPI.kiro.generateAuthUrl(payload)
      authUrl.value = response.auth_url
      sessionId.value = response.session_id
      state.value = response.state || ''
      return true
    } catch (err: any) {
      error.value = err.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(error.value)
      return false
    } finally {
      loading.value = false
    }
  }

  const generateIDCAuthUrl = async (params: {
    proxyId?: number | null
    startUrl?: string
    region?: string
  }): Promise<boolean> => {
    loading.value = true
    error.value = ''
    authUrl.value = ''
    sessionId.value = ''
    state.value = ''

    try {
      const payload = {
        proxy_id: params.proxyId || undefined,
        start_url: params.startUrl,
        region: params.region
      }
      const response =
        scope === 'user'
          ? await accountsAPI.generateKiroIDCAuthUrl(payload)
          : await adminAPI.kiro.generateIDCAuthUrl(payload)
      authUrl.value = response.auth_url
      sessionId.value = response.session_id
      state.value = response.state
      return true
    } catch (err: any) {
      error.value = err.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(error.value)
      return false
    } finally {
      loading.value = false
    }
  }

  const exchangeAuthCode = async (params: {
    code: string
    sessionId: string
    state: string
    callbackPath?: string
    loginOption?: string
    proxyId?: number | null
  }): Promise<KiroTokenInfo | null> => {
    loading.value = true
    error.value = ''

    try {
      const payload = {
        session_id: params.sessionId,
        state: params.state,
        code: params.code.trim(),
        callback_path: params.callbackPath,
        login_option: params.loginOption,
        proxy_id: params.proxyId || undefined
      }
      return scope === 'user'
        ? await accountsAPI.exchangeKiroOAuthCode(payload)
        : await adminAPI.kiro.exchangeCode(payload)
    } catch (err: any) {
      error.value = err.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const validateRefreshToken = async (input: {
    refreshToken: string
    authMethod?: string
    provider?: string
    clientId?: string
    clientSecret?: string
    startUrl?: string
    region?: string
    profileArn?: string
    proxyId?: number | null
  }): Promise<KiroTokenInfo | null> => {
    loading.value = true
    error.value = ''

    try {
      const payload = {
        refresh_token: input.refreshToken.trim(),
        auth_method: input.authMethod,
        provider: input.provider,
        client_id: input.clientId,
        client_secret: input.clientSecret,
        start_url: input.startUrl,
        region: input.region,
        profile_arn: input.profileArn,
        proxy_id: input.proxyId || undefined
      }
      return scope === 'user'
        ? await accountsAPI.refreshKiroToken(payload)
        : await adminAPI.kiro.refreshToken(payload)
    } catch (err: any) {
      error.value = err.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      return null
    } finally {
      loading.value = false
    }
  }

  const importToken = async (
    tokenJSON: string,
    deviceRegistrationJSON?: string
  ): Promise<KiroTokenInfo | null> => {
    loading.value = true
    error.value = ''

    try {
      const payload = {
        token_json: tokenJSON,
        device_registration_json: deviceRegistrationJSON
      }
      return scope === 'user'
        ? await accountsAPI.importKiroToken(payload)
        : await adminAPI.kiro.importToken(payload)
    } catch (err: any) {
      error.value = err.response?.data?.detail || t('admin.accounts.oauth.authFailed')
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const buildCredentials = (tokenInfo: KiroTokenInfo): Record<string, unknown> => {
    const credentials: Record<string, unknown> = {
      access_token: tokenInfo.access_token,
      refresh_token: tokenInfo.refresh_token,
      profile_arn: tokenInfo.profile_arn,
      expires_at: tokenInfo.expires_at,
      auth_method: tokenInfo.auth_method,
      provider: tokenInfo.provider,
      client_id: tokenInfo.client_id,
      client_secret: tokenInfo.client_secret,
      client_id_hash: tokenInfo.client_id_hash,
      email: tokenInfo.email,
      start_url: tokenInfo.start_url,
      region: tokenInfo.region
    }
    return Object.fromEntries(
      Object.entries(credentials).filter(([, value]) => value !== undefined && value !== '')
    )
  }

  const buildExtraInfo = (tokenInfo: KiroTokenInfo): Record<string, unknown> => {
    const extra: Record<string, unknown> = {
      openai_responses_supported: false
    }
    if (tokenInfo.email) extra.email = tokenInfo.email
    if (tokenInfo.profile_arn) extra.profile_arn = tokenInfo.profile_arn
    return extra
  }

  return {
    authUrl,
    sessionId,
    state,
    loading,
    error,
    resetState,
    generateAuthUrl,
    generateIDCAuthUrl,
    exchangeAuthCode,
    validateRefreshToken,
    importToken,
    buildCredentials,
    buildExtraInfo
  }
}
