import { onBeforeUnmount, reactive, ref } from 'vue'
import type { ServiceEndpointTarget } from '@/utils/serviceStatus'
import { median } from '@/utils/serviceStatus'

export type EndpointDiagnosticStatus = 'idle' | 'testing' | 'online' | 'offline'

export interface EndpointDiagnosticResult {
  status: EndpointDiagnosticStatus
  latencyMs: number | null
  checkedAt: string | null
  error: string
}

const ATTEMPTS = 3
const TIMEOUT_MS = 6000

export function useEndpointDiagnostics() {
  const results = reactive<Record<string, EndpointDiagnosticResult>>({})
  const testingAll = ref(false)
  const controllers = new Map<string, AbortController>()

  function resultFor(id: string): EndpointDiagnosticResult {
    if (!results[id]) {
      results[id] = { status: 'idle', latencyMs: null, checkedAt: null, error: '' }
    }
    return results[id]
  }

  async function ping(url: string, signal: AbortSignal): Promise<number> {
    const startedAt = performance.now()
    const requestUrl = new URL(`${url}${url.includes('?') ? '&' : '?'}_=${Date.now()}`)
    const crossOrigin = requestUrl.origin !== window.location.origin
    const response = await fetch(requestUrl, {
      method: 'GET',
      credentials: 'omit',
      cache: 'no-store',
      mode: crossOrigin ? 'no-cors' : 'same-origin',
      signal,
    })
    // Cross-origin health checks return an opaque response unless that route
    // exposes CORS headers. An opaque response still proves DNS/TLS/HTTP reachability.
    if (response.type !== 'opaque' && !response.ok) throw new Error(`HTTP ${response.status}`)
    return performance.now() - startedAt
  }

  async function test(target: ServiceEndpointTarget): Promise<void> {
    controllers.get(target.id)?.abort()
    const controller = new AbortController()
    controllers.set(target.id, controller)
    const result = resultFor(target.id)
    result.status = 'testing'
    result.error = ''

    const latencies: number[] = []
    let lastError = ''
    for (let attempt = 0; attempt < ATTEMPTS; attempt += 1) {
      const timeout = window.setTimeout(() => controller.abort('timeout'), TIMEOUT_MS)
      try {
        latencies.push(await ping(target.healthUrl, controller.signal))
      } catch (error) {
        if (controller.signal.aborted) {
          lastError = controller.signal.reason === 'timeout' ? 'timeout' : 'cancelled'
          break
        }
        lastError = error instanceof Error ? error.message : 'request_failed'
      } finally {
        window.clearTimeout(timeout)
      }
    }

    if (controllers.get(target.id) !== controller) return
    controllers.delete(target.id)
    result.checkedAt = new Date().toISOString()
    const latency = median(latencies)
    if (latency !== null) {
      result.status = 'online'
      result.latencyMs = Math.round(latency)
      result.error = ''
    } else {
      result.status = 'offline'
      result.latencyMs = null
      result.error = lastError || 'request_failed'
    }
  }

  async function testAll(targets: ServiceEndpointTarget[]): Promise<void> {
    testingAll.value = true
    try {
      const queue = [...targets]
      const workers = Array.from({ length: Math.min(3, queue.length) }, async () => {
        while (queue.length > 0) {
          const target = queue.shift()
          if (target) await test(target)
        }
      })
      await Promise.all(workers)
    } finally {
      testingAll.value = false
    }
  }

  onBeforeUnmount(() => {
    for (const controller of controllers.values()) controller.abort('cancelled')
    controllers.clear()
  })

  return { results, resultFor, test, testAll, testingAll }
}
