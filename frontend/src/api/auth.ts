/**
 * Authentication API endpoints
 * Handles user login, registration, and logout operations
 */

import { apiClient } from './client'
import {
  clearAuthentication,
  getAccessToken,
  getCurrentAuthBundle,
  refreshAuthentication
} from './authSession'
import type {
  LoginRequest,
  RegisterRequest,
  AuthResponse,
  CurrentUserResponse,
  SendVerifyCodeRequest,
  SendVerifyCodeResponse,
  PublicSettings,
  ActionCaptchaRequestProof,
  TotpLoginResponse,
  TotpLogin2FARequest,
  LoginSession
} from '@/types'

/**
 * Login response type - can be either full auth or 2FA required
 */
export type LoginResponse = AuthResponse | TotpLoginResponse
export type RefreshTokenResponse = AuthResponse

export type OAuthLoginProvider =
  | 'github'
  | 'google'
  | 'linuxdo'
  | 'dingtalk'
  | 'wechat'
  | 'oidc'

export interface OAuthLoginStart {
  provider: OAuthLoginProvider
  params: Record<string, string>
}

export interface OAuthLoginStartResponse {
  authorize_url: string
}

export function buildOAuthLoginStartURL(request: OAuthLoginStart): string {
  const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) || '/api/v1'
  const normalized = apiBase.replace(/\/$/, '')
  const query = new URLSearchParams(request.params).toString()
  const path = `${normalized}/auth/oauth/${request.provider}/start`
  return query ? `${path}?${query}` : path
}

export async function startOAuthLogin(
  request: OAuthLoginStart,
  proof: ActionCaptchaRequestProof
): Promise<OAuthLoginStartResponse> {
  const { data } = await apiClient.post<OAuthLoginStartResponse>(
    `/auth/oauth/${request.provider}/start`,
    proof,
    { params: request.params }
  )
  return data
}

/**
 * Type guard to check if login response requires 2FA
 */
export function isTotp2FARequired(response: LoginResponse): response is TotpLoginResponse {
  return 'requires_2fa' in response && response.requires_2fa === true
}

/**
 * Store authentication token in localStorage
 */
export function setAuthToken(token: string): void {
  void token
}

/**
 * Store refresh token in localStorage
 */
export function setRefreshToken(token: string): void {
  void token
}

/**
 * Store token expiration timestamp in localStorage
 * Converts expires_in (seconds) to absolute timestamp (milliseconds)
 */
export function setTokenExpiresAt(expiresIn: number): void {
  void expiresIn
}

/**
 * Get authentication token from localStorage
 */
export function getAuthToken(): string | null {
  return getAccessToken()
}

/**
 * Get refresh token from localStorage
 */
export function getRefreshToken(): string | null {
  return null
}

/**
 * Get token expiration timestamp from localStorage
 */
export function getTokenExpiresAt(): number | null {
  return null
}

/**
 * Clear authentication token from localStorage
 */
export function clearAuthToken(): void {
  clearAuthentication(false)
}

/**
 * User login
 * @param credentials - Email and password
 * @returns Authentication response with token and user data, or 2FA required response
 */
export async function login(credentials: LoginRequest): Promise<LoginResponse> {
  const { data } = await apiClient.post<LoginResponse>('/auth/login', credentials)

  return data
}

/**
 * Complete login with 2FA code
 * @param request - Temp token and TOTP code
 * @returns Authentication response with token and user data
 */
export async function login2FA(request: TotpLogin2FARequest): Promise<AuthResponse> {
  const { data } = await apiClient.post<AuthResponse>('/auth/login/2fa', request)

  return data
}

/**
 * User registration
 * @param userData - Registration data (username, email, password)
 * @returns Authentication response with token and user data
 */
export async function register(userData: RegisterRequest): Promise<AuthResponse> {
  const { data } = await apiClient.post<AuthResponse>('/auth/register', userData)

  return data
}

/**
 * Get current authenticated user
 * @returns User profile data
 */
export async function getCurrentUser() {
  return apiClient.get<CurrentUserResponse>('/auth/me')
}

/**
 * Resume website login with the HttpOnly session cookie.
 */
export async function resumeSession(): Promise<AuthResponse> {
  const outcome = await refreshAuthentication()
  if (outcome.kind === 'authenticated') return outcome.bundle
  if (outcome.kind === 'transient_error') throw outcome.error
  throw new Error('Session expired')
}

async function executeLogout(allowMismatchRecovery = true): Promise<void> {
  const sid = getCurrentAuthBundle()?.session.sid
  try {
    await apiClient.post('/auth/logout', undefined, {
      headers: sid ? { 'X-Auth-Session': sid } : undefined
    })
  } catch (error: unknown) {
    const apiError = error as { status?: number; code?: string; reason?: string }
    const code = apiError.reason || apiError.code
    if (allowMismatchRecovery && apiError.status === 409 && code === 'AUTH_SESSION_MISMATCH') {
      const outcome = await refreshAuthentication()
      if (outcome.kind === 'authenticated') return executeLogout(false)
      if (outcome.kind === 'anonymous') return
    }
    throw error
  }
}

/** Revoke the current server-controlled browser session. */
export async function logout(): Promise<void> {
  await executeLogout()
}

/**
 * Refresh token response
 */
export interface OAuthTokenResponse {
  access_token: string
  refresh_token?: string
  expires_in?: number
  access_expires_at?: number
  token_type?: string
}

export interface PendingOAuthBindLoginResponse extends Partial<OAuthTokenResponse> {
  auth_result?: string
  redirect?: string
  error?: string
  requires_2fa?: boolean
  temp_token?: string
  user_email_masked?: string
  adoption_required?: boolean
  suggested_display_name?: string
  suggested_avatar_url?: string
}

export type PendingOAuthExchangeResponse = PendingOAuthBindLoginResponse

export interface PendingOAuthCreateAccountResponse extends OAuthTokenResponse {
  auth_result?: string
}

export interface PendingOAuthSendVerifyCodeResponse extends SendVerifyCodeResponse {
  auth_result?: string
  provider?: string
  redirect?: string
}

export type OAuthCompletionKind = 'login' | 'bind'

export interface OAuthAdoptionDecision {
  adoptDisplayName?: boolean
  adoptAvatar?: boolean
}

function serializeOAuthAdoptionDecision(
  decision?: OAuthAdoptionDecision
): Record<string, boolean> {
  const payload: Record<string, boolean> = {}

  if (typeof decision?.adoptDisplayName === 'boolean') {
    payload.adopt_display_name = decision.adoptDisplayName
  }
  if (typeof decision?.adoptAvatar === 'boolean') {
    payload.adopt_avatar = decision.adoptAvatar
  }

  return payload
}

export function isOAuthLoginCompletion(
  completion: Partial<OAuthTokenResponse>
): completion is OAuthTokenResponse {
  return typeof completion.access_token === 'string' && completion.access_token.trim().length > 0
}

export function getOAuthCompletionKind(
  completion: Partial<OAuthTokenResponse>
): OAuthCompletionKind {
  return isOAuthLoginCompletion(completion) ? 'login' : 'bind'
}

export function getPendingOAuthBindLoginKind(
  completion: PendingOAuthBindLoginResponse
): OAuthCompletionKind {
  return getOAuthCompletionKind(completion)
}

export function isPendingOAuthCreateAccountRequired(
  completion: Pick<PendingOAuthBindLoginResponse, 'error'>
): boolean {
  return completion.error === 'invitation_required'
}

export function hasPendingOAuthSuggestedProfile(
  completion: Pick<
    PendingOAuthBindLoginResponse,
    'suggested_display_name' | 'suggested_avatar_url'
  >
): boolean {
  return Boolean(completion.suggested_display_name || completion.suggested_avatar_url)
}

export function persistOAuthTokenContext(tokens: Partial<OAuthTokenResponse>): void {
  void tokens
}

export async function prepareOAuthBindAccessTokenCookie(): Promise<void> {
  if (!getAuthToken()) {
    return
  }
  await apiClient.post('/auth/oauth/bind-token')
}

/**
 * Refresh the access token using the refresh token
 * @returns New token pair
 */
export async function refreshToken(): Promise<RefreshTokenResponse> {
  return resumeSession()
}

/**
 * Revoke all sessions for the current user
 * @returns Response with message
 */
export async function revokeAllSessions(): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/auth/revoke-all-sessions')
  return data
}

export async function listLoginSessions(): Promise<LoginSession[]> {
  const { data } = await apiClient.get<LoginSession[]>('/auth/sessions')
  return data
}

export async function revokeLoginSession(sid: string): Promise<{ revoked_sid: string; current: boolean }> {
  const { data } = await apiClient.delete<{ revoked_sid: string; current: boolean }>(`/auth/sessions/${encodeURIComponent(sid)}`)
  if (data.current) clearAuthentication(true)
  return data
}

export async function revokeOtherLoginSessions(): Promise<{ revoked_count: number }> {
  const { data } = await apiClient.post<{ revoked_count: number }>('/auth/sessions/revoke-others')
  return data
}

/**
 * Check if user is authenticated
 * @returns True if user has valid token
 */
export function isAuthenticated(): boolean {
  return getAccessToken() !== null
}

/**
 * Get public settings (no auth required)
 * @returns Public settings including registration and Turnstile config
 */
export async function getPublicSettings(): Promise<PublicSettings> {
  const { data } = await apiClient.get<PublicSettings>('/settings/public')
  return data
}

export type WeChatOAuthMode = 'open' | 'mp'
export type WeChatOAuthUnavailableReason =
  | 'not_configured'
  | 'capability_unknown'
  | 'external_browser_required'
  | 'wechat_browser_required'
  | 'native_app_required'

export interface ResolvedWeChatOAuthStart {
  mode: WeChatOAuthMode | null
  openEnabled: boolean
  mpEnabled: boolean
  mobileEnabled: boolean
  isWeChatBrowser: boolean
  unavailableReason: WeChatOAuthUnavailableReason | null
}

export type WeChatOAuthPublicSettings = {
  wechat_oauth_enabled?: boolean
  wechat_oauth_open_enabled?: boolean
  wechat_oauth_mp_enabled?: boolean
  wechat_oauth_mobile_enabled?: boolean
}

export function isWeChatWebOAuthEnabled(
  settings: WeChatOAuthPublicSettings | null | undefined,
): boolean {
  const legacyEnabled = settings?.wechat_oauth_enabled ?? false
  const hasExplicitCapabilities =
    typeof settings?.wechat_oauth_open_enabled === 'boolean' ||
    typeof settings?.wechat_oauth_mp_enabled === 'boolean'

  if (!hasExplicitCapabilities) {
    return legacyEnabled
  }

  return settings?.wechat_oauth_open_enabled === true || settings?.wechat_oauth_mp_enabled === true
}

export function hasExplicitWeChatOAuthCapabilities(
  settings: WeChatOAuthPublicSettings | null | undefined,
): settings is WeChatOAuthPublicSettings & {
  wechat_oauth_open_enabled: boolean
  wechat_oauth_mp_enabled: boolean
} {
  return typeof settings?.wechat_oauth_open_enabled === 'boolean'
    && typeof settings?.wechat_oauth_mp_enabled === 'boolean'
}

export function resolveWeChatOAuthStart(
  settings: WeChatOAuthPublicSettings | null | undefined,
  userAgent?: string
): ResolvedWeChatOAuthStart {
  const normalizedUserAgent = (userAgent
    ?? (typeof navigator !== 'undefined' ? navigator.userAgent : '')
    ?? '').trim()
  const isWeChatBrowser = /MicroMessenger/i.test(normalizedUserAgent)
  const legacyEnabled = settings?.wechat_oauth_enabled ?? false
  const openEnabled = typeof settings?.wechat_oauth_open_enabled === 'boolean'
    ? settings.wechat_oauth_open_enabled
    : legacyEnabled
  const mpEnabled = typeof settings?.wechat_oauth_mp_enabled === 'boolean'
    ? settings.wechat_oauth_mp_enabled
    : legacyEnabled
  const mobileEnabled = typeof settings?.wechat_oauth_mobile_enabled === 'boolean'
    ? settings.wechat_oauth_mobile_enabled
    : false

  if (isWeChatBrowser) {
    if (mpEnabled) {
      return { mode: 'mp', openEnabled, mpEnabled, mobileEnabled, isWeChatBrowser, unavailableReason: null }
    }
    if (openEnabled) {
      return { mode: null, openEnabled, mpEnabled, mobileEnabled, isWeChatBrowser, unavailableReason: 'external_browser_required' }
    }
    return { mode: null, openEnabled, mpEnabled, mobileEnabled, isWeChatBrowser, unavailableReason: 'not_configured' }
  }

  if (openEnabled) {
    return { mode: 'open', openEnabled, mpEnabled, mobileEnabled, isWeChatBrowser, unavailableReason: null }
  }
  if (mpEnabled) {
    return { mode: null, openEnabled, mpEnabled, mobileEnabled, isWeChatBrowser, unavailableReason: 'wechat_browser_required' }
  }
  return { mode: null, openEnabled, mpEnabled, mobileEnabled, isWeChatBrowser, unavailableReason: 'not_configured' }
}

export function resolveWeChatOAuthStartStrict(
  settings: WeChatOAuthPublicSettings | null | undefined,
  userAgent?: string,
): ResolvedWeChatOAuthStart {
  const normalizedUserAgent = (userAgent
    ?? (typeof navigator !== 'undefined' ? navigator.userAgent : '')
    ?? '').trim()
  const isWeChatBrowser = /MicroMessenger/i.test(normalizedUserAgent)

  if (!hasExplicitWeChatOAuthCapabilities(settings)) {
    return {
      mode: null,
      openEnabled: false,
      mpEnabled: false,
      mobileEnabled: false,
      isWeChatBrowser,
      unavailableReason: 'capability_unknown',
    }
  }

  return resolveWeChatOAuthStart(settings, normalizedUserAgent)
}

/**
 * Send verification code to email
 * @param request - Email and optional Turnstile token
 * @returns Response with countdown seconds
 */
export async function sendVerifyCode(
  request: SendVerifyCodeRequest
): Promise<SendVerifyCodeResponse> {
  const { data } = await apiClient.post<SendVerifyCodeResponse>('/auth/send-verify-code', request)
  return data
}

export async function sendPendingOAuthVerifyCode(
  request: SendVerifyCodeRequest
): Promise<PendingOAuthSendVerifyCodeResponse> {
  const { data } = await apiClient.post<PendingOAuthSendVerifyCodeResponse>(
    '/auth/oauth/pending/send-verify-code',
    request
  )
  return data
}

/**
 * Validate promo code response
 */
export interface ValidatePromoCodeResponse {
  valid: boolean
  bonus_amount?: number
  error_code?: string
  message?: string
}

/**
 * Validate promo code (public endpoint, no auth required)
 * @param code - Promo code to validate
 * @returns Validation result with bonus amount if valid
 */
export async function validatePromoCode(code: string): Promise<ValidatePromoCodeResponse> {
  const { data } = await apiClient.post<ValidatePromoCodeResponse>('/auth/validate-promo-code', { code })
  return data
}

/**
 * Validate invitation code response
 */
export interface ValidateInvitationCodeResponse {
  valid: boolean
  error_code?: string
}

/**
 * Validate invitation code (public endpoint, no auth required)
 * @param code - Invitation code to validate
 * @returns Validation result
 */
export async function validateInvitationCode(code: string): Promise<ValidateInvitationCodeResponse> {
  const { data } = await apiClient.post<ValidateInvitationCodeResponse>('/auth/validate-invitation-code', { code })
  return data
}

/**
 * Forgot password request
 */
export interface ForgotPasswordRequest {
  email: string
  turnstile_token?: string
  tencent_captcha_ticket?: string
  tencent_captcha_randstr?: string
}

/**
 * Forgot password response
 */
export interface ForgotPasswordResponse {
  message: string
}

/**
 * Request password reset link
 * @param request - Email and optional Turnstile token
 * @returns Response with message
 */
export async function forgotPassword(request: ForgotPasswordRequest): Promise<ForgotPasswordResponse> {
  const { data } = await apiClient.post<ForgotPasswordResponse>('/auth/forgot-password', request)
  return data
}

/**
 * Reset password request
 */
export interface ResetPasswordRequest {
  email: string
  token: string
  new_password: string
}

/**
 * Reset password response
 */
export interface ResetPasswordResponse {
  message: string
}

/**
 * Reset password with token
 * @param request - Email, token, and new password
 * @returns Response with message
 */
export async function resetPassword(request: ResetPasswordRequest): Promise<ResetPasswordResponse> {
  const { data } = await apiClient.post<ResetPasswordResponse>('/auth/reset-password', request)
  return data
}

/**
 * Complete LinuxDo OAuth registration by supplying an invitation code
 * @param invitationCode - Invitation code entered by the user
 * @returns Token pair on success
 */
export async function completeLinuxDoOAuthRegistration(
  invitationCode: string,
  decision?: OAuthAdoptionDecision,
  affiliateCode?: string
): Promise<OAuthTokenResponse> {
  return createPendingLinuxDoOAuthAccount(invitationCode, decision, affiliateCode)
}

/**
 * Complete OIDC OAuth registration by supplying an invitation code
 * @param invitationCode - Invitation code entered by the user
 * @returns Token pair on success
 */
export async function completeOIDCOAuthRegistration(
  invitationCode: string,
  decision?: OAuthAdoptionDecision,
  affiliateCode?: string
): Promise<OAuthTokenResponse> {
  return createPendingOIDCOAuthAccount(invitationCode, decision, affiliateCode)
}

export async function completeWeChatOAuthRegistration(
  invitationCode: string,
  decision?: OAuthAdoptionDecision,
  affiliateCode?: string
): Promise<OAuthTokenResponse> {
  return createPendingWeChatOAuthAccount(invitationCode, decision, affiliateCode)
}

async function createPendingOAuthAccount(
  provider: 'linuxdo' | 'oidc' | 'wechat',
  invitationCode: string,
  decision?: OAuthAdoptionDecision,
  affiliateCode?: string
): Promise<PendingOAuthCreateAccountResponse> {
  const normalizedAffiliateCode = affiliateCode?.trim()
  const { data } = await apiClient.post<PendingOAuthCreateAccountResponse>(
    `/auth/oauth/${provider}/complete-registration`,
    {
      invitation_code: invitationCode,
      ...(normalizedAffiliateCode ? { aff_code: normalizedAffiliateCode } : {}),
      ...serializeOAuthAdoptionDecision(decision)
    }
  )
  return data
}

export async function createPendingLinuxDoOAuthAccount(
  invitationCode: string,
  decision?: OAuthAdoptionDecision,
  affiliateCode?: string
): Promise<PendingOAuthCreateAccountResponse> {
  return createPendingOAuthAccount('linuxdo', invitationCode, decision, affiliateCode)
}

export async function createPendingOIDCOAuthAccount(
  invitationCode: string,
  decision?: OAuthAdoptionDecision,
  affiliateCode?: string
): Promise<PendingOAuthCreateAccountResponse> {
  return createPendingOAuthAccount('oidc', invitationCode, decision, affiliateCode)
}

export async function createPendingWeChatOAuthAccount(
  invitationCode: string,
  decision?: OAuthAdoptionDecision,
  affiliateCode?: string
): Promise<PendingOAuthCreateAccountResponse> {
  return createPendingOAuthAccount('wechat', invitationCode, decision, affiliateCode)
}

export async function completePendingOAuthBindLogin(
  decision?: OAuthAdoptionDecision
): Promise<PendingOAuthBindLoginResponse> {
  const { data } = await apiClient.post<PendingOAuthBindLoginResponse>(
    '/auth/oauth/pending/exchange',
    serializeOAuthAdoptionDecision(decision)
  )
  return data
}

export async function exchangePendingOAuthCompletion(
  decision?: OAuthAdoptionDecision
): Promise<PendingOAuthExchangeResponse> {
  return completePendingOAuthBindLogin(decision)
}

export const authAPI = {
  login,
  login2FA,
  isTotp2FARequired,
  register,
  getCurrentUser,
  logout,
  isAuthenticated,
  setAuthToken,
  setRefreshToken,
  setTokenExpiresAt,
  getAuthToken,
  getRefreshToken,
  getTokenExpiresAt,
  clearAuthToken,
  getPublicSettings,
  sendVerifyCode,
  sendPendingOAuthVerifyCode,
  validatePromoCode,
  validateInvitationCode,
  forgotPassword,
  resetPassword,
  refreshToken,
  resumeSession,
  revokeAllSessions,
  listLoginSessions,
  revokeLoginSession,
  revokeOtherLoginSessions,
  getPendingOAuthBindLoginKind,
  isPendingOAuthCreateAccountRequired,
  hasPendingOAuthSuggestedProfile,
  completePendingOAuthBindLogin,
  createPendingLinuxDoOAuthAccount,
  createPendingOIDCOAuthAccount,
  createPendingWeChatOAuthAccount,
  exchangePendingOAuthCompletion,
  completeLinuxDoOAuthRegistration,
  completeOIDCOAuthRegistration,
  completeWeChatOAuthRegistration
}

export default authAPI
