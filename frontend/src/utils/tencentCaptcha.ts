// Tencent captcha has separate China and international scripts. Their app IDs
// and constructor signatures are not interchangeable.
export type TencentCaptchaRegion = 'cn' | 'intl'

export interface TencentCaptchaProof {
  ticket: string
  randstr: string
}

export interface TencentCaptchaResult {
  ret: number
  ticket?: string | null
  randstr?: string | null
  errorCode?: number
  errorMessage?: string
}

interface TencentCaptchaInstance {
  show(): void
  destroy(): void
}

type TencentCaptchaCallback = (result: TencentCaptchaResult) => void

type TencentCaptchaConstructor = {
  new (
    appId: string,
    callback: TencentCaptchaCallback,
    options?: Record<string, unknown>
  ): TencentCaptchaInstance
  new (
    element: HTMLElement,
    appId: string,
    callback: TencentCaptchaCallback,
    options?: Record<string, unknown>
  ): TencentCaptchaInstance
}

declare global {
  interface Window {
    TencentCaptcha?: TencentCaptchaConstructor
    TCaptchaGlobal?: boolean
  }
}

const scriptSources: Record<TencentCaptchaRegion, string> = {
  cn: 'https://turing.captcha.qcloud.com/TJCaptcha.js',
  intl: 'https://ca.turing.captcha.qcloud.com/TJNCaptcha-global.js'
}

export function normalizeTencentCaptchaRegion(value?: string | null): TencentCaptchaRegion {
  return value === 'intl' ? 'intl' : 'cn'
}

let scriptPromise: Promise<TencentCaptchaConstructor> | null = null
let loadedRegion: TencentCaptchaRegion | null = null

function existingGlobalRegion(): TencentCaptchaRegion {
  return window.TCaptchaGlobal === true ? 'intl' : 'cn'
}

export function loadTencentCaptcha(
  region: TencentCaptchaRegion = 'cn'
): Promise<TencentCaptchaConstructor> {
  const globalRegion = window.TencentCaptcha ? existingGlobalRegion() : null
  if (window.TencentCaptcha && (loadedRegion === region || globalRegion === region)) {
    return Promise.resolve(window.TencentCaptcha)
  }
  if (window.TencentCaptcha && loadedRegion !== null && loadedRegion !== region) {
    return Promise.reject(new Error('Tencent Captcha region changed; reload the page to apply it'))
  }
  if (scriptPromise && loadedRegion === region) return scriptPromise

  loadedRegion = region
  scriptPromise = new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = scriptSources[region]
    script.async = true
    script.onload = () => {
      if (window.TencentCaptcha && existingGlobalRegion() === region) {
        resolve(window.TencentCaptcha)
        return
      }
      scriptPromise = null
      loadedRegion = null
      reject(new Error('Tencent Captcha SDK is unavailable'))
    }
    script.onerror = () => {
      scriptPromise = null
      loadedRegion = null
      reject(new Error('Failed to load Tencent Captcha SDK'))
    }
    document.head.appendChild(script)
  })

  return scriptPromise
}

export function resetTencentCaptchaLoaderForTest(): void {
  scriptPromise = null
  loadedRegion = null
}
