import type { AuthResponse } from '@/types'
import { refreshAuthentication } from './authSession'

export type RefreshTokenResponse = AuthResponse

export interface RefreshAuthTokensOptions {
  failedAccessToken?: string | null
}

export async function refreshAuthTokens(
  _options: RefreshAuthTokensOptions = {}
): Promise<RefreshTokenResponse> {
  const outcome = await refreshAuthentication()
  if (outcome.kind === 'authenticated') return outcome.bundle
  if (outcome.kind === 'transient_error') throw outcome.error
  throw new Error('Session expired')
}
