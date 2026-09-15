import axios from 'axios'
import type { AuthResponse } from '@/types'
import { getAPIBaseURL } from './url'

export type RefreshOutcome =
  | { kind: 'authenticated'; bundle: AuthResponse }
  | { kind: 'anonymous' }
  | { kind: 'transient_error'; error: unknown }
  | { kind: 'out_of_sync'; code?: string }

export type AuthBootstrapState = 'idle' | 'checking' | 'complete'

export interface AuthTokenRotation {
  access_token: string
  token_type: 'Bearer'
  access_expires_at: number
  session: AuthResponse['session']
}

type AuthSessionListener = (bundle: AuthResponse | null, state: AuthBootstrapState) => void

const refreshRaceDelays = [80, 200, 500] as const
const legacyAuthKeys = ['auth_token', 'refresh_token', 'auth_user', 'token_expires_at'] as const
const channelName = 'ikik:auth-session'
const storageKey = 'ikik:auth-session:event'
const lockName = 'ikik:auth-refresh'

function randomIdentifier(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID()
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

const authSyncSource = randomIdentifier()
let authSyncPublisher: BroadcastChannel | null = null

let currentBundle: AuthResponse | null = null
let bootstrapState: AuthBootstrapState = 'idle'
let refreshPromise: Promise<RefreshOutcome> | null = null
let authEpoch = 0
const listeners = new Set<AuthSessionListener>()

function clearLegacyTokenStorage(): void {
  try {
    for (const key of legacyAuthKeys) localStorage.removeItem(key)
  } catch {
    // The cookie-backed session must keep working when Web Storage is unavailable.
  }
}

if (typeof window !== 'undefined') clearLegacyTokenStorage()

function notify(): void {
  for (const listener of listeners) listener(currentBundle, bootstrapState)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object'
}

export function isAuthBundle(value: unknown): value is AuthResponse {
  if (!isRecord(value) || !isRecord(value.user) || !isRecord(value.session)) return false
  return (
    typeof value.access_token === 'string' && value.access_token.length > 0 &&
    typeof value.token_type === 'string' &&
    typeof value.access_expires_at === 'number' && value.access_expires_at > 0 &&
    typeof value.session.sid === 'string' && value.session.sid.length > 0 &&
    value.session.current === true &&
    typeof value.user.id === 'number' && value.user.id > 0
  )
}

interface AuthSessionSyncEvent {
  kind: 'authenticated' | 'signed_out'
  sid: string
  source: string
  nonce: string
  timestamp: number
}

function isAuthSessionSyncEvent(value: unknown): value is AuthSessionSyncEvent {
  if (!isRecord(value)) return false
  return (
    (value.kind === 'authenticated' || value.kind === 'signed_out') &&
    typeof value.sid === 'string' && value.sid.length > 0 &&
    typeof value.source === 'string' &&
    typeof value.nonce === 'string' &&
    typeof value.timestamp === 'number'
  )
}

function publish(kind: AuthSessionSyncEvent['kind'], sid: string): void {
  if (!sid || typeof window === 'undefined') return
  const payload: AuthSessionSyncEvent = {
    kind,
    sid,
    source: authSyncSource,
    nonce: randomIdentifier(),
    timestamp: Date.now()
  }
  if (typeof window.BroadcastChannel !== 'undefined') {
    authSyncPublisher ??= new window.BroadcastChannel(channelName)
    authSyncPublisher.postMessage(payload)
    return
  }
  try {
    localStorage.setItem(storageKey, JSON.stringify(payload))
    localStorage.removeItem(storageKey)
  } catch {
    // Cross-tab synchronization is best-effort when storage is unavailable.
  }
}

function handlePeerEvent(value: unknown): void {
  if (!isAuthSessionSyncEvent(value) || value.source === authSyncSource ||
      Math.abs(Date.now() - value.timestamp) >= 60_000) return
  const localSID = currentBundle?.session.sid
  if (value.kind === 'signed_out' && localSID === value.sid) {
    clearAuthentication(false)
  } else if (value.kind === 'authenticated' && localSID !== value.sid) {
    void refreshAuthentication()
  }
}

if (typeof window !== 'undefined') {
  if (typeof window.BroadcastChannel !== 'undefined') {
    const channel = new window.BroadcastChannel(channelName)
    channel.addEventListener('message', (event) => handlePeerEvent(event.data))
  } else {
    window.addEventListener('storage', (event) => {
      if (event.key !== storageKey || !event.newValue) return
      try { handlePeerEvent(JSON.parse(event.newValue)) } catch { /* ignore malformed peer events */ }
    })
  }
}

export function subscribeAuthSession(listener: AuthSessionListener): () => void {
  listeners.add(listener)
  listener(currentBundle, bootstrapState)
  return () => listeners.delete(listener)
}

export function acceptAuthBundle(bundle: AuthResponse, synchronizeTabs = true): void {
  if (!isAuthBundle(bundle)) throw new Error('Invalid authentication response')
  const previousSID = currentBundle?.session.sid
  authEpoch += 1
  currentBundle = bundle
  bootstrapState = 'complete'
  notify()
  if (synchronizeTabs && previousSID !== bundle.session.sid) publish('authenticated', bundle.session.sid)
}

export function applyAuthRotation(rotation: AuthTokenRotation): void {
  if (!currentBundle || !isRecord(rotation) || !isRecord(rotation.session) ||
      typeof rotation.access_token !== 'string' || rotation.token_type !== 'Bearer' ||
      typeof rotation.access_expires_at !== 'number' ||
      rotation.session.sid !== currentBundle.session.sid) {
    throw new Error('Invalid authentication rotation response')
  }
  acceptAuthBundle({
    ...currentBundle,
    access_token: rotation.access_token,
    token_type: rotation.token_type,
    access_expires_at: rotation.access_expires_at,
    expires_in: Math.max(0, rotation.access_expires_at - Math.floor(Date.now() / 1000)),
    session: rotation.session
  }, false)
}

export function clearAuthentication(synchronizeTabs = true, nextState: AuthBootstrapState = 'complete'): void {
  const sid = currentBundle?.session.sid
  authEpoch += 1
  currentBundle = null
  bootstrapState = nextState
  notify()
  if (synchronizeTabs && sid) publish('signed_out', sid)
}

export function getAccessToken(): string | null {
  return currentBundle?.access_token ?? null
}

export function getCurrentAuthBundle(): AuthResponse | null {
  return currentBundle
}

export function getAuthBootstrapState(): AuthBootstrapState {
  return bootstrapState
}

async function requestRefresh(expectedSID?: string): Promise<{ status: number; data?: unknown; error?: unknown }> {
  try {
    const response = await axios.post(
      `${getAPIBaseURL()}/auth/refresh`,
      undefined,
      {
        withCredentials: true,
        timeout: 30_000,
        headers: expectedSID ? { 'X-Auth-Session': expectedSID } : undefined
      }
    )
    return { status: response.status, data: response.data }
  } catch (error) {
    if (!axios.isAxiosError(error)) return { status: 0, error }
    return { status: error.response?.status ?? 0, data: error.response?.data, error }
  }
}

function unwrapRefreshResponse(value: unknown): { bundle: AuthResponse | null; code?: string } {
  if (!isRecord(value)) return { bundle: null }
  const code = typeof value.reason === 'string'
    ? value.reason
    : typeof value.code === 'string'
      ? value.code
      : undefined
  const payload = value.code === 0 ? value.data : value.success === true ? value.data : undefined
  return { bundle: isAuthBundle(payload) ? payload : null, code }
}

function wait(delay: number): Promise<void> {
  return new Promise((resolve) => globalThis.setTimeout(resolve, delay))
}

async function runRefresh(refreshEpoch: number, raceAttempt = 0, allowMismatchRetry = true): Promise<RefreshOutcome> {
  if (refreshEpoch !== authEpoch) return { kind: 'out_of_sync', code: 'AUTH_SESSION_CHANGED' }
  const response = await requestRefresh(currentBundle?.session.sid)
  if (refreshEpoch !== authEpoch) return { kind: 'out_of_sync', code: 'AUTH_SESSION_CHANGED' }
  const { bundle, code } = unwrapRefreshResponse(response.data)
  if (bundle) {
    acceptAuthBundle(bundle, false)
    return { kind: 'authenticated', bundle }
  }
  if (response.status === 409 && code === 'AUTH_REFRESH_RACE') {
    const delay = refreshRaceDelays[raceAttempt]
    if (delay !== undefined) {
      await wait(delay)
      return runRefresh(refreshEpoch, raceAttempt + 1, allowMismatchRetry)
    }
    clearAuthentication(false)
    return { kind: 'out_of_sync', code }
  }
  if (response.status === 409 && code === 'AUTH_SESSION_MISMATCH') {
    if (allowMismatchRetry) {
      clearAuthentication(false, 'idle')
      return runRefresh(authEpoch, 0, false)
    }
    clearAuthentication(false)
    return { kind: 'out_of_sync', code }
  }
  if (response.status === 401) {
    clearAuthentication(true)
    return { kind: 'anonymous' }
  }
  if (!response.status || response.status >= 500 || response.status === 429) {
    bootstrapState = 'idle'
    notify()
    return { kind: 'transient_error', error: response.error ?? response.data }
  }
  clearAuthentication(false)
  return { kind: 'out_of_sync', code: code ?? 'AUTH_INVALID_REFRESH_RESPONSE' }
}

async function performRefreshWithBrowserLock(refreshEpoch: number): Promise<RefreshOutcome> {
  try {
    if (typeof navigator === 'undefined' || !navigator.locks) return runRefresh(refreshEpoch)
    return navigator.locks.request(lockName, { mode: 'exclusive' }, () => runRefresh(refreshEpoch))
  } catch (error) {
    bootstrapState = 'idle'
    notify()
    return { kind: 'transient_error', error }
  }
}

export function refreshAuthentication(): Promise<RefreshOutcome> {
  clearLegacyTokenStorage()
  if (!refreshPromise) {
    const refreshEpoch = authEpoch
    refreshPromise = performRefreshWithBrowserLock(refreshEpoch).finally(() => { refreshPromise = null })
  }
  return refreshPromise
}

export async function bootstrapAuthentication(): Promise<RefreshOutcome> {
  clearLegacyTokenStorage()
  if (currentBundle && currentBundle.access_expires_at > Math.floor(Date.now() / 1000)) {
    bootstrapState = 'complete'
    notify()
    return { kind: 'authenticated', bundle: currentBundle }
  }
  if (bootstrapState === 'complete' && !currentBundle) return { kind: 'anonymous' }
  bootstrapState = 'checking'
  notify()
  return refreshAuthentication()
}
